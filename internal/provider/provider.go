package provider

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	_ "github.com/lib/pq"
)

var _ provider.Provider = (*PostgreSQLProvider)(nil)

type PostgreSQLProvider struct {
	version string
}

type PostgreSQLProviderModel struct {
	Host           types.String `tfsdk:"host"`
	Port           types.Int64  `tfsdk:"port"`
	Username       types.String `tfsdk:"username"`
	Password       types.String `tfsdk:"password"`
	Database       types.String `tfsdk:"database"`
	SSLMode        types.String `tfsdk:"sslmode"`
	ConnectTimeout types.Int64  `tfsdk:"connect_timeout"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &PostgreSQLProvider{
			version: version,
		}
	}
}

func (p *PostgreSQLProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "postgresql"
	resp.Version = p.version
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
				Description: "SSL mode (disable, require, verify-ca, verify-full). Default: prefer. Can also be set with the PGSSLMODE environment variable.",
				Optional:    true,
			},
			"connect_timeout": schema.Int64Attribute{
				Description: "Connection timeout in seconds. Default: 30.",
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

	host := envOrDefault(config.Host, "PGHOST", "localhost")
	port := envOrDefaultInt(config.Port, "PGPORT", 5432)
	username := envOrDefault(config.Username, "PGUSER", "postgres")
	password := envOrDefault(config.Password, "PGPASSWORD", "")
	database := envOrDefault(config.Database, "PGDATABASE", "postgres")
	sslmode := envOrDefault(config.SSLMode, "PGSSLMODE", "prefer")
	connectTimeout := envOrDefaultInt(config.ConnectTimeout, "", 30)

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		host, port, username, password, database, sslmode, connectTimeout,
	)

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

	err = db.PingContext(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect to PostgreSQL", err.Error())
		return
	}

	resp.DataSourceData = db
	resp.ResourceData = db
}

func (p *PostgreSQLProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRoleResource,
		NewDatabaseResource,
		NewSchemaResource,
		NewGrantResource,
		NewDefaultPrivilegesResource,
	}
}

func (p *PostgreSQLProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewRoleDataSource,
		NewDatabaseDataSource,
		NewSchemasDataSource,
		NewQueryDataSource,
	}
}

func envOrDefault(val types.String, envVar, defaultVal string) string {
	if !val.IsNull() && !val.IsUnknown() {
		return val.ValueString()
	}
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			return v
		}
	}
	return defaultVal
}

func envOrDefaultInt(val types.Int64, envVar string, defaultVal int) int {
	if !val.IsNull() && !val.IsUnknown() {
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
