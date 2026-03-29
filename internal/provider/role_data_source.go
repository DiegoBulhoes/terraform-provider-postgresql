package provider

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*roleDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*roleDataSource)(nil)
)

type roleDataSource struct {
	db *sql.DB
}

type roleDataSourceModel struct {
	Name            types.String `tfsdk:"name"`
	OID             types.Int64  `tfsdk:"oid"`
	Login           types.Bool   `tfsdk:"login"`
	Superuser       types.Bool   `tfsdk:"superuser"`
	CreateDatabase  types.Bool   `tfsdk:"create_database"`
	CreateRole      types.Bool   `tfsdk:"create_role"`
	Replication     types.Bool   `tfsdk:"replication"`
	ConnectionLimit types.Int64  `tfsdk:"connection_limit"`
	ValidUntil      types.String `tfsdk:"valid_until"`
	Roles           types.List   `tfsdk:"roles"`
}

func NewRoleDataSource() datasource.DataSource {
	return &roleDataSource{}
}

func (d *roleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *roleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads information about a PostgreSQL role.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the role.",
				Required:    true,
			},
			"oid": schema.Int64Attribute{
				Description: "The OID of the role.",
				Computed:    true,
			},
			"login": schema.BoolAttribute{
				Description: "Whether the role can log in.",
				Computed:    true,
			},
			"superuser": schema.BoolAttribute{
				Description: "Whether the role is a superuser.",
				Computed:    true,
			},
			"create_database": schema.BoolAttribute{
				Description: "Whether the role can create databases.",
				Computed:    true,
			},
			"create_role": schema.BoolAttribute{
				Description: "Whether the role can create other roles.",
				Computed:    true,
			},
			"replication": schema.BoolAttribute{
				Description: "Whether the role can initiate streaming replication.",
				Computed:    true,
			},
			"connection_limit": schema.Int64Attribute{
				Description: "Connection limit for the role. -1 means no limit.",
				Computed:    true,
			},
			"valid_until": schema.StringAttribute{
				Description: "Password expiry time. Empty if no expiry.",
				Computed:    true,
			},
			"roles": schema.ListAttribute{
				Description: "List of roles that this role is a member of.",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *roleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	db, ok := req.ProviderData.(*sql.DB)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *sql.DB, got: %T.", req.ProviderData),
		)
		return
	}

	d.db = db
}

func (d *roleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state roleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := state.Name.ValueString()

	var oid int64
	var login, superuser, createDB, createRole, replication bool
	var connLimit int64
	var validUntil sql.NullString

	err := d.db.QueryRowContext(ctx,
		`SELECT oid, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolconnlimit, rolvaliduntil
		 FROM pg_catalog.pg_roles WHERE rolname = $1`, name,
	).Scan(&oid, &login, &superuser, &createDB, &createRole, &replication, &connLimit, &validUntil)
	if err != nil {
		resp.Diagnostics.AddError("Error reading role", fmt.Sprintf("Could not read role %q: %s", name, err.Error()))
		return
	}

	state.OID = types.Int64Value(oid)
	state.Login = types.BoolValue(login)
	state.Superuser = types.BoolValue(superuser)
	state.CreateDatabase = types.BoolValue(createDB)
	state.CreateRole = types.BoolValue(createRole)
	state.Replication = types.BoolValue(replication)
	state.ConnectionLimit = types.Int64Value(connLimit)

	if validUntil.Valid {
		state.ValidUntil = types.StringValue(validUntil.String)
	} else {
		state.ValidUntil = types.StringValue("")
	}

	// Query role memberships
	rows, err := d.db.QueryContext(ctx,
		`SELECT r.rolname
		 FROM pg_catalog.pg_auth_members m
		 JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
		 WHERE m.member = (SELECT oid FROM pg_roles WHERE rolname = $1)`, name,
	)
	if err != nil {
		resp.Diagnostics.AddError("Error reading role memberships", fmt.Sprintf("Could not read memberships for role %q: %s", name, err.Error()))
		return
	}
	defer rows.Close()

	var roleNames []attr.Value
	for rows.Next() {
		var roleName string
		if err := rows.Scan(&roleName); err != nil {
			resp.Diagnostics.AddError("Error scanning role membership", err.Error())
			return
		}
		roleNames = append(roleNames, types.StringValue(roleName))
	}
	if err := rows.Err(); err != nil {
		resp.Diagnostics.AddError("Error iterating role memberships", err.Error())
		return
	}

	rolesList, diags := types.ListValue(types.StringType, roleNames)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Roles = rolesList

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
