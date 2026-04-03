package provider

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
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
	_ "github.com/lib/pq"
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
	Superuser          types.Bool   `tfsdk:"superuser"`
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
				Description: "Maximum number of open connections to the database. Default: 5.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"max_idle_connections": schema.Int64Attribute{
				Description: "Maximum number of idle connections in the pool. Default: 2.",
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
			"superuser": schema.BoolAttribute{
				Description: "Whether the provider connection user is a superuser. When false, the provider avoids operations that require superuser privileges. Useful for managed PostgreSQL services (RDS, Cloud SQL, Azure). Default: true.",
				Optional:    true,
			},
			"expected_version": schema.StringAttribute{
				Description: "Expected PostgreSQL server major version (e.g. \"16\", \"15\"). When set, the provider can skip features not available in older versions. If omitted, the provider detects the version automatically.",
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
	maxOpenConns := EnvOrDefaultInt(config.MaxOpenConnections, "", 2)
	maxIdleConns := EnvOrDefaultInt(config.MaxIdleConnections, "", 1)
	connMaxLifetime := EnvOrDefaultInt(config.ConnMaxLifetime, "", 0)
	connMaxIdleTime := EnvOrDefaultInt(config.ConnMaxIdleTime, "", 0)
	_ = EnvOrDefaultBool(config.Superuser, "", true)

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		host, port, username, password, database, sslmode, connectTimeout,
	)

	if sslcert != "" {
		connStr += fmt.Sprintf(" sslcert=%s", sslcert)
	}
	if sslkey != "" {
		connStr += fmt.Sprintf(" sslkey=%s", sslkey)
	}
	if sslrootcert != "" {
		connStr += fmt.Sprintf(" sslrootcert=%s", sslrootcert)
	}

	tflog.Debug(ctx, "Connecting to PostgreSQL", map[string]interface{}{
		"host":     host,
		"port":     port,
		"username": username,
		"database": database,
		"sslmode":  sslmode,
	})

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create PostgreSQL client", err.Error())
		return
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	if connMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(connMaxLifetime) * time.Second)
	}
	// Default to 5 seconds idle time to reclaim connections quickly.
	idleTime := 5
	if connMaxIdleTime > 0 {
		idleTime = connMaxIdleTime
	}
	db.SetConnMaxIdleTime(time.Duration(idleTime) * time.Second)

	err = db.PingContext(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to PostgreSQL", err.Error())
		return
	}

	wrapper := common.NewDBWrapper(db)
	resp.DataSourceData = wrapper
	resp.ResourceData = wrapper
}

func (p *PostgreSQLProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		pgresource.NewRoleResource,
		pgresource.NewUserResource,
		pgresource.NewDatabaseResource,
		pgresource.NewSchemaResource,
		pgresource.NewGrantResource,
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

// EnvOrDefaultBool returns the Terraform attribute value, or falls back to the
// given environment variable, or finally the default value.
func EnvOrDefaultBool(val types.Bool, envVar string, defaultVal bool) bool {
	if common.IsSet(val) {
		return val.ValueBool()
	}
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			if b, err := strconv.ParseBool(v); err == nil {
				return b
			}
		}
	}
	return defaultVal
}
