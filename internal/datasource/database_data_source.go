package datasource

import (
	"context"
	"fmt"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*databaseDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*databaseDataSource)(nil)
)

type databaseDataSource struct {
	db common.DBTX
}

type databaseDataSourceModel struct {
	Name             types.String `tfsdk:"name"`
	OID              types.Int64  `tfsdk:"oid"`
	Owner            types.String `tfsdk:"owner"`
	Encoding         types.String `tfsdk:"encoding"`
	LcCollate        types.String `tfsdk:"lc_collate"`
	LcCtype          types.String `tfsdk:"lc_ctype"`
	TablespaceName   types.String `tfsdk:"tablespace_name"`
	ConnectionLimit  types.Int64  `tfsdk:"connection_limit"`
	AllowConnections types.Bool   `tfsdk:"allow_connections"`
	IsTemplate       types.Bool   `tfsdk:"is_template"`
}

func NewDatabaseDataSource() datasource.DataSource {
	return &databaseDataSource{}
}

func (d *databaseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database"
}

func (d *databaseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads information about a PostgreSQL database.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the database.",
				Required:    true,
			},
			"oid": schema.Int64Attribute{
				Description: "The OID of the database.",
				Computed:    true,
			},
			"owner": schema.StringAttribute{
				Description: "The role that owns the database.",
				Computed:    true,
			},
			"encoding": schema.StringAttribute{
				Description: "Character set encoding of the database.",
				Computed:    true,
			},
			"lc_collate": schema.StringAttribute{
				Description: "LC_COLLATE setting of the database.",
				Computed:    true,
			},
			"lc_ctype": schema.StringAttribute{
				Description: "LC_CTYPE setting of the database.",
				Computed:    true,
			},
			"tablespace_name": schema.StringAttribute{
				Description: "The default tablespace for the database.",
				Computed:    true,
			},
			"connection_limit": schema.Int64Attribute{
				Description: "Connection limit for the database. -1 means no limit.",
				Computed:    true,
			},
			"allow_connections": schema.BoolAttribute{
				Description: "Whether connections are allowed to this database.",
				Computed:    true,
			},
			"is_template": schema.BoolAttribute{
				Description: "Whether this database is a template.",
				Computed:    true,
			},
		},
	}
}

func (d *databaseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	db, err := common.ConfigureDB(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", err.Error())
		return
	}
	d.db = db
}

func (d *databaseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state databaseDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := state.Name.ValueString()

	var oid int64
	var owner, encoding, lcCollate, lcCtype, tablespace string
	var connLimit int64
	var allowConn, isTemplate bool

	err := d.db.QueryRowContext(ctx,
		`SELECT d.oid, r.rolname as owner, pg_encoding_to_char(d.encoding) as encoding,
		        d.datcollate, d.datctype, t.spcname as tablespace, d.datconnlimit,
		        d.datallowconn, d.datistemplate
		 FROM pg_catalog.pg_database d
		 JOIN pg_catalog.pg_roles r ON d.datdba = r.oid
		 JOIN pg_catalog.pg_tablespace t ON d.dattablespace = t.oid
		 WHERE d.datname = $1`, name,
	).Scan(&oid, &owner, &encoding, &lcCollate, &lcCtype, &tablespace, &connLimit, &allowConn, &isTemplate)
	if err != nil {
		resp.Diagnostics.AddError("Error reading database", fmt.Sprintf("Could not read database %q: %s", name, err.Error()))
		return
	}

	state.OID = types.Int64Value(oid)
	state.Owner = types.StringValue(owner)
	state.Encoding = types.StringValue(encoding)
	state.LcCollate = types.StringValue(lcCollate)
	state.LcCtype = types.StringValue(lcCtype)
	state.TablespaceName = types.StringValue(tablespace)
	state.ConnectionLimit = types.Int64Value(connLimit)
	state.AllowConnections = types.BoolValue(allowConn)
	state.IsTemplate = types.BoolValue(isTemplate)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
