package datasource

import (
	"context"
	"fmt"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*RolesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*RolesDataSource)(nil)
)

type RolesDataSource struct {
	DB common.DBTX
}

type RolesDataSourceModel struct {
	LikePattern    types.String `tfsdk:"like_pattern"`
	NotLikePattern types.String `tfsdk:"not_like_pattern"`
	LoginOnly      types.Bool   `tfsdk:"login_only"`
	Roles          types.List   `tfsdk:"roles"`
}

var RoleObjectAttrTypes = map[string]attr.Type{
	"name":             types.StringType,
	"oid":              types.Int64Type,
	"login":            types.BoolType,
	"superuser":        types.BoolType,
	"create_database":  types.BoolType,
	"create_role":      types.BoolType,
	"replication":      types.BoolType,
	"connection_limit": types.Int64Type,
}

func NewRolesDataSource() datasource.DataSource {
	return &RolesDataSource{}
}

func (d *RolesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_roles"
}

func (d *RolesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists PostgreSQL roles with optional filtering.",
		Attributes: map[string]schema.Attribute{
			"like_pattern": schema.StringAttribute{
				Description: "A SQL LIKE pattern to filter role names.",
				Optional:    true,
			},
			"not_like_pattern": schema.StringAttribute{
				Description: "A SQL NOT LIKE pattern to exclude role names.",
				Optional:    true,
			},
			"login_only": schema.BoolAttribute{
				Description: "Whether to only include roles with LOGIN privilege. Default: false.",
				Optional:    true,
			},
			"roles": schema.ListNestedAttribute{
				Description: "List of roles matching the filters.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the role.",
							Computed:    true,
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
					},
				},
			},
		},
	}
}

func (d *RolesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state RolesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := `SELECT rolname, oid, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolconnlimit FROM pg_catalog.pg_roles`
	var conditions []string
	var args []interface{}
	argIdx := 1

	loginOnly := false
	if common.IsSet(state.LoginOnly) {
		loginOnly = state.LoginOnly.ValueBool()
	}

	if loginOnly {
		conditions = append(conditions, `rolcanlogin = true`)
	}

	if common.IsSet(state.LikePattern) {
		conditions = append(conditions, fmt.Sprintf(`rolname LIKE $%d`, argIdx))
		args = append(args, state.LikePattern.ValueString())
		argIdx++
	}

	if common.IsSet(state.NotLikePattern) {
		conditions = append(conditions, fmt.Sprintf(`rolname NOT LIKE $%d`, argIdx))
		args = append(args, state.NotLikePattern.ValueString())
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, cond := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += cond
		}
	}

	query += " ORDER BY rolname"

	rows, err := d.DB.QueryContext(ctx, query, args...)
	if err != nil {
		resp.Diagnostics.AddError("Error querying roles", fmt.Sprintf("Could not query roles: %s", err.Error()))
		return
	}
	defer rows.Close() //nolint:errcheck

	var roleObjects []attr.Value
	for rows.Next() {
		var name string
		var oid, connLimit int64
		var login, superuser, createDB, createRole, replication bool
		if err := rows.Scan(&name, &oid, &login, &superuser, &createDB, &createRole, &replication, &connLimit); err != nil {
			resp.Diagnostics.AddError("Error scanning role row", err.Error())
			return
		}

		obj, diags := types.ObjectValue(RoleObjectAttrTypes, map[string]attr.Value{
			"name":             types.StringValue(name),
			"oid":              types.Int64Value(oid),
			"login":            types.BoolValue(login),
			"superuser":        types.BoolValue(superuser),
			"create_database":  types.BoolValue(createDB),
			"create_role":      types.BoolValue(createRole),
			"replication":      types.BoolValue(replication),
			"connection_limit": types.Int64Value(connLimit),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		roleObjects = append(roleObjects, obj)
	}
	if err := rows.Err(); err != nil {
		resp.Diagnostics.AddError("Error iterating role rows", err.Error())
		return
	}

	rolesList, diags := types.ListValue(types.ObjectType{AttrTypes: RoleObjectAttrTypes}, roleObjects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Roles = rolesList

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
