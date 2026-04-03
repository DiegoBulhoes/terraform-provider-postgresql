package datasource

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*UserDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*UserDataSource)(nil)
)

type UserDataSource struct {
	DB common.DBTX
}

type UserDataSourceModel struct {
	Name            types.String `tfsdk:"name"`
	OID             types.Int64  `tfsdk:"oid"`
	Superuser       types.Bool   `tfsdk:"superuser"`
	CreateDatabase  types.Bool   `tfsdk:"create_database"`
	CreateRole      types.Bool   `tfsdk:"create_role"`
	Replication     types.Bool   `tfsdk:"replication"`
	ConnectionLimit types.Int64  `tfsdk:"connection_limit"`
	ValidUntil      types.String `tfsdk:"valid_until"`
	Roles           types.List   `tfsdk:"roles"`
}

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

func (d *UserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads information about a PostgreSQL user (a role with LOGIN privilege).",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the user.",
				Required:    true,
			},
			"oid": schema.Int64Attribute{
				Description: "The OID of the user.",
				Computed:    true,
			},
			"superuser": schema.BoolAttribute{
				Description: "Whether the user is a superuser.",
				Computed:    true,
			},
			"create_database": schema.BoolAttribute{
				Description: "Whether the user can create databases.",
				Computed:    true,
			},
			"create_role": schema.BoolAttribute{
				Description: "Whether the user can create other roles.",
				Computed:    true,
			},
			"replication": schema.BoolAttribute{
				Description: "Whether the user can initiate streaming replication.",
				Computed:    true,
			},
			"connection_limit": schema.Int64Attribute{
				Description: "Connection limit for the user. -1 means no limit.",
				Computed:    true,
			},
			"valid_until": schema.StringAttribute{
				Description: "Password expiry time. Empty if no expiry.",
				Computed:    true,
			},
			"roles": schema.ListAttribute{
				Description: "List of roles that this user is a member of.",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *UserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	db, err := common.ConfigureDB(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", err.Error())
		return
	}
	d.DB = db
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state UserDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := state.Name.ValueString()

	var oid int64
	var login, superuser, createDB, createRole, replication bool
	var connLimit int64
	var validUntil sql.NullString

	err := d.DB.QueryRowContext(ctx,
		`SELECT oid, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolconnlimit, rolvaliduntil
		 FROM pg_catalog.pg_roles WHERE rolname = $1`, name,
	).Scan(&oid, &login, &superuser, &createDB, &createRole, &replication, &connLimit, &validUntil)
	if err != nil {
		resp.Diagnostics.AddError("Error reading user", fmt.Sprintf("Could not read user %q: %s", name, err.Error()))
		return
	}

	if !login {
		resp.Diagnostics.AddWarning(
			"Role is not a user",
			fmt.Sprintf("Role %q does not have LOGIN privilege. Consider using the postgresql_role data source instead.", name),
		)
	}

	state.OID = types.Int64Value(oid)
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
	rows, err := d.DB.QueryContext(ctx,
		`SELECT r.rolname
		 FROM pg_catalog.pg_auth_members m
		 JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
		 WHERE m.member = (SELECT oid FROM pg_roles WHERE rolname = $1)`, name,
	)
	if err != nil {
		resp.Diagnostics.AddError("Error reading role memberships", fmt.Sprintf("Could not read memberships for user %q: %s", name, err.Error()))
		return
	}
	defer rows.Close() //nolint:errcheck

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
