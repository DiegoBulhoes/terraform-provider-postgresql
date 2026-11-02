package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	pgdatasource "github.com/DiegoBulhoes/terraform-provider-postgresql/internal/datasource"
	pgresource "github.com/DiegoBulhoes/terraform-provider-postgresql/internal/resource"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lib/pq"
)

var _ provider.Provider = (*PostgreSQLProvider)(nil)

type PostgreSQLProvider struct {
	Version string
}

type PostgreSQLProviderModel struct {
	Host               types.String `tfsdk:"host"`
	Port               types.Int64  `tfsdk:"port"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	Database           types.String `tfsdk:"database"`
	SSLMode            types.String `tfsdk:"sslmode"`
	SSLCert            types.String `tfsdk:"sslcert"`
	SSLKey             types.String `tfsdk:"sslkey"`
	SSLRootCert        types.String `tfsdk:"sslrootcert"`
	ConnectTimeout     types.Int64  `tfsdk:"connect_timeout"`
	MaxOpenConnections types.Int64  `tfsdk:"max_connections"`
	MaxIdleConnections types.Int64  `tfsdk:"max_idle_connections"`
	ConnMaxLifetime    types.Int64  `tfsdk:"conn_max_lifetime"`
	ConnMaxIdleTime    types.Int64  `tfsdk:"conn_max_idle_time"`
	ExpectedVersion    types.String `tfsdk:"expected_version"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &PostgreSQLProvider{
			Version: version,
		}
	}
}

func (p *PostgreSQLProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "postgresql"
	resp.Version = p.Version
}

func (p *PostgreSQLProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for managing PostgreSQL resources.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Description: "PostgreSQL server hostname. Can also be set with the PGHOST environment variable.",
				Optional:    true,
			},
			"port": schema.Int64Attribute{
				Description: "PostgreSQL server port. Default: 5432. Can also be set with the PGPORT environment variable.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.Between(1, 65535),
				},
			},
			"username": schema.StringAttribute{
				Description: "PostgreSQL user. Can also be set with the PGUSER environment variable.",
				Optional:    true,
			},
			"password": schema.StringAttribute{
				Description: "PostgreSQL password. Can also be set with the PGPASSWORD environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"database": schema.StringAttribute{
				Description: "Default database to connect to. Default: postgres. Can also be set with the PGDATABASE environment variable.",
				Optional:    true,
			},
			"sslmode": schema.StringAttribute{
				Description: "SSL mode (disable, allow, prefer, require, verify-ca, verify-full). Default: prefer. Can also be set with the PGSSLMODE environment variable.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("disable", "allow", "prefer", "require", "verify-ca", "verify-full"),
				},
			},
			"sslcert": schema.StringAttribute{
				Description: "Path to the SSL client certificate. Can also be set with the PGSSLCERT environment variable.",
				Optional:    true,
			},
			"sslkey": schema.StringAttribute{
				Description: "Path to the SSL client private key. Can also be set with the PGSSLKEY environment variable.",
				Optional:    true,
			},
			"sslrootcert": schema.StringAttribute{
				Description: "Path to the SSL root certificate authority. Can also be set with the PGSSLROOTCERT environment variable.",
				Optional:    true,
			},
			"connect_timeout": schema.Int64Attribute{
				Description: "Connection timeout in seconds. Default: 30.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"max_connections": schema.Int64Attribute{
				Description: "Maximum number of open connections to the database. Default: 10.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"max_idle_connections": schema.Int64Attribute{
				Description: "Maximum number of idle connections in the pool. Default: 5.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"conn_max_lifetime": schema.Int64Attribute{
				Description: "Maximum lifetime of a connection in seconds. Connections older than this are closed before reuse. 0 means no limit. Default: 0.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"conn_max_idle_time": schema.Int64Attribute{
				Description: "Maximum time in seconds a connection can be idle before being closed. 0 means no limit. Default: 0.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"expected_version": schema.StringAttribute{
				Description: "Expected PostgreSQL server major version (e.g. \"16\"). When set, the provider fails to configure if the server runs another major version.",
				Optional:    true,
			},
		},
	}
}

func (p *PostgreSQLProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config PostgreSQLProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if v := os.Getenv("PGPORT"); !common.IsSet(config.Port) && v != "" {
		if _, err := strconv.Atoi(v); err != nil {
			resp.Diagnostics.AddError("Invalid PGPORT", fmt.Sprintf("PGPORT must be a port number, got %q.", v))
			return
		}
	}

	host := EnvOrDefault(config.Host, "PGHOST", "localhost")
	port := EnvOrDefaultInt(config.Port, "PGPORT", 5432)
	username := EnvOrDefault(config.Username, "PGUSER", "postgres")
	password := EnvOrDefault(config.Password, "PGPASSWORD", "")
	database := EnvOrDefault(config.Database, "PGDATABASE", "postgres")
	sslmode := EnvOrDefault(config.SSLMode, "PGSSLMODE", "prefer")
	sslcert := EnvOrDefault(config.SSLCert, "PGSSLCERT", "")
	sslkey := EnvOrDefault(config.SSLKey, "PGSSLKEY", "")
	sslrootcert := EnvOrDefault(config.SSLRootCert, "PGSSLROOTCERT", "")
	connectTimeout := EnvOrDefaultInt(config.ConnectTimeout, "", 30)
	maxOpenConns := EnvOrDefaultInt(config.MaxOpenConnections, "", 10)
	maxIdleConns := EnvOrDefaultInt(config.MaxIdleConnections, "", 5)
	connMaxLifetime := EnvOrDefaultInt(config.ConnMaxLifetime, "", 0)
	connMaxIdleTime := EnvOrDefaultInt(config.ConnMaxIdleTime, "", 0)

	params := func(dbname string) []string {
		p := []string{
			"host=" + common.QuoteConnStringValue(host),
			fmt.Sprintf("port=%d", port),
			"user=" + common.QuoteConnStringValue(username),
			"password=" + common.QuoteConnStringValue(password),
			"dbname=" + common.QuoteConnStringValue(dbname),
			fmt.Sprintf("connect_timeout=%d", connectTimeout),
		}
		if sslcert != "" {
			p = append(p, "sslcert="+common.QuoteConnStringValue(sslcert))
		}
		if sslkey != "" {
			p = append(p, "sslkey="+common.QuoteConnStringValue(sslkey))
		}
		if sslrootcert != "" {
			p = append(p, "sslrootcert="+common.QuoteConnStringValue(sslrootcert))
		}
		return p
	}

	tflog.Debug(ctx, "Connecting to PostgreSQL", map[string]interface{}{
		"host":     host,
		"port":     port,
		"username": username,
		"database": database,
		"sslmode":  sslmode,
	})

	pool := PoolSettings{
		MaxOpen:        maxOpenConns,
		MaxIdle:        maxIdleConns,
		MaxLifetime:    time.Duration(connMaxLifetime) * time.Second,
		MaxIdleTime:    time.Duration(connMaxIdleTime) * time.Second,
		ConnectTimeout: time.Duration(connectTimeout) * time.Second,
	}
	key := fmt.Sprintf("%s|%s|%+v|%s", strings.Join(params(database), " "), sslmode, pool, config.ExpectedVersion.ValueString())
	if cached, ok := configured.Load(key); ok {
		resp.DataSourceData = cached
		resp.ResourceData = cached
		return
	}

	db, err := Connect(ctx, params(database), sslmode, pool)
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to PostgreSQL", err.Error())
		return
	}

	if common.IsSet(config.ExpectedVersion) {
		if err := CheckServerVersion(ctx, db, config.ExpectedVersion.ValueString()); err != nil {
			_ = db.Close()
			resp.Diagnostics.AddError("Unexpected PostgreSQL version", err.Error())
			return
		}
	}

	wrapper := common.NewDBWrapper(db)
	wrapper.Database = database
	wrapper.Open = func(ctx context.Context, dbname string) (*sql.DB, error) {
		return Connect(ctx, params(dbname), sslmode, pool)
	}
	if cached, loaded := configured.LoadOrStore(key, wrapper); loaded {
		_ = wrapper.Close()
		wrapper = cached.(*common.DBWrapper)
	}
	resp.DataSourceData = wrapper
	resp.ResourceData = wrapper
}

// configured keeps one connection wrapper per configuration. Terraform configures
// a provider once per process, but tests configure it for every command in one.
var configured sync.Map

func (p *PostgreSQLProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		pgresource.NewRoleResource,
		pgresource.NewUserResource,
		pgresource.NewDatabaseResource,
		pgresource.NewSchemaResource,
		pgresource.NewGrantResource,
		pgresource.NewRoleMembershipResource,
		pgresource.NewExtensionResource,
		pgresource.NewDefaultPrivilegesResource,
	}
}

func (p *PostgreSQLProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		pgdatasource.NewRoleDataSource,
		pgdatasource.NewUserDataSource,
		pgdatasource.NewDatabaseDataSource,
		pgdatasource.NewSchemasDataSource,
		pgdatasource.NewQueryDataSource,
		pgdatasource.NewTablesDataSource,
		pgdatasource.NewExtensionsDataSource,
		pgdatasource.NewRolesDataSource,
		pgdatasource.NewVersionDataSource,
	}
}

func EnvOrDefault(val types.String, envVar, defaultVal string) string {
	if common.IsSet(val) {
		return val.ValueString()
	}
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			return v
		}
	}
	return defaultVal
}

func EnvOrDefaultInt(val types.Int64, envVar string, defaultVal int) int {
	if common.IsSet(val) {
		return int(val.ValueInt64())
	}
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return defaultVal
}

// PoolSettings holds the connection pool options of the provider.
type PoolSettings struct {
	MaxOpen        int
	MaxIdle        int
	MaxLifetime    time.Duration
	MaxIdleTime    time.Duration
	ConnectTimeout time.Duration
}

// SSLModeAttempts returns the sslmode values to try in order. lib/pq has no
// "prefer" or "allow", so they are emulated as in libpq.
func SSLModeAttempts(sslmode string) []string {
	switch sslmode {
	case "prefer":
		return []string{"require", "disable"}
	case "allow":
		return []string{"disable", "require"}
	default:
		return []string{sslmode}
	}
}

// Connect opens and pings a pool, falling back to the next sslmode only when
// the server refused the previous one.
func Connect(ctx context.Context, params []string, sslmode string, pool PoolSettings) (*sql.DB, error) {
	attempts := SSLModeAttempts(sslmode)
	var lastErr error
	for i, mode := range attempts {
		db, err := openPool(ctx, append(append([]string{}, params...), "sslmode="+mode), pool)
		if err == nil {
			return db, nil
		}
		lastErr = err
		if i == len(attempts)-1 || !sslRefused(mode, err) {
			break
		}
		tflog.Debug(ctx, "Server refused sslmode, trying the next one", map[string]interface{}{"sslmode": mode})
	}
	return nil, lastErr
}

func sslRefused(mode string, err error) bool {
	if mode == "require" {
		return errors.Is(err, pq.ErrSSLNotSupported)
	}
	// pg_hba.conf rejects a plain connection with invalid_authorization_specification.
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "28000"
}

func openPool(ctx context.Context, params []string, pool PoolSettings) (*sql.DB, error) {
	db, err := sql.Open("postgres", strings.Join(params, " "))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(pool.MaxOpen)
	db.SetMaxIdleConns(pool.MaxIdle)
	if pool.MaxLifetime > 0 {
		db.SetConnMaxLifetime(pool.MaxLifetime)
	}
	// Default to 5 seconds idle time to reclaim connections quickly.
	idleTime := 5 * time.Second
	if pool.MaxIdleTime > 0 {
		idleTime = pool.MaxIdleTime
	}
	db.SetConnMaxIdleTime(idleTime)

	pingCtx, cancel := context.WithTimeout(ctx, pool.ConnectTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// CheckServerVersion fails when the server's major version is not expected
// (a major version such as "16", or "16.2" of which only 16 counts).
func CheckServerVersion(ctx context.Context, db *sql.DB, expected string) error {
	want, err := strconv.Atoi(strings.SplitN(strings.TrimSpace(expected), ".", 2)[0])
	if err != nil {
		return fmt.Errorf("expected_version %q is not a major version number", expected)
	}
	var num int
	if err := db.QueryRowContext(ctx, `SELECT current_setting('server_version_num')::int`).Scan(&num); err != nil {
		return err
	}
	if got := num / 10000; got != want {
		return fmt.Errorf("the server runs PostgreSQL %d, expected PostgreSQL %d", got, want)
	}
	return nil
}
