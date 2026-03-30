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
	_ datasource.DataSource              = (*schemasDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*schemasDataSource)(nil)
)

type schemasDataSource struct {
	db *sql.DB
}

type schemasDataSourceModel struct {
	Database             types.String `tfsdk:"database"`
	LikePattern          types.String `tfsdk:"like_pattern"`
	NotLikePattern       types.String `tfsdk:"not_like_pattern"`
	IncludeSystemSchemas types.Bool   `tfsdk:"include_system_schemas"`
	Schemas              types.List   `tfsdk:"schemas"`
}

var schemaObjectAttrTypes = map[string]attr.Type{
	"name":  types.StringType,
	"owner": types.StringType,
}

func NewSchemasDataSource() datasource.DataSource {
	return &schemasDataSource{}
}

func (d *schemasDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schemas"
}

func (d *schemasDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists PostgreSQL schemas with optional filtering.",
		Attributes: map[string]schema.Attribute{
			"database": schema.StringAttribute{
				Description: "The database to list schemas from. Uses the provider default if not set.",
				Optional:    true,
			},
			"like_pattern": schema.StringAttribute{
				Description: "A SQL LIKE pattern to filter schema names.",
				Optional:    true,
			},
			"not_like_pattern": schema.StringAttribute{
				Description: "A SQL NOT LIKE pattern to exclude schema names.",
				Optional:    true,
			},
			"include_system_schemas": schema.BoolAttribute{
				Description: "Whether to include system schemas (pg_* and information_schema). Default: false.",
				Optional:    true,
			},
			"schemas": schema.ListNestedAttribute{
				Description: "List of schemas.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the schema.",
							Computed:    true,
						},
						"owner": schema.StringAttribute{
							Description: "The owner of the schema.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *schemasDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *schemasDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state schemasDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	includeSystem := false
	if common.IsSet(state.IncludeSystemSchemas) {
		includeSystem = state.IncludeSystemSchemas.ValueBool()
	}

	query := `SELECT s.nspname, r.rolname FROM pg_catalog.pg_namespace s JOIN pg_catalog.pg_roles r ON s.nspowner = r.oid`
	var conditions []string
	var args []interface{}
	argIdx := 1

	if !includeSystem {
		conditions = append(conditions, `s.nspname NOT LIKE 'pg\_%' AND s.nspname != 'information_schema'`)
	}

	if common.IsSet(state.LikePattern) {
		conditions = append(conditions, fmt.Sprintf(`s.nspname LIKE $%d`, argIdx))
		args = append(args, state.LikePattern.ValueString())
		argIdx++
	}

	if common.IsSet(state.NotLikePattern) {
		conditions = append(conditions, fmt.Sprintf(`s.nspname NOT LIKE $%d`, argIdx))
		args = append(args, state.NotLikePattern.ValueString())
		argIdx++
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

	query += " ORDER BY s.nspname"

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		resp.Diagnostics.AddError("Error querying schemas", fmt.Sprintf("Could not query schemas: %s", err.Error()))
		return
	}
	defer rows.Close()

	var schemaObjects []attr.Value
	for rows.Next() {
		var name, owner string
		if err := rows.Scan(&name, &owner); err != nil {
			resp.Diagnostics.AddError("Error scanning schema row", err.Error())
			return
		}

		obj, diags := types.ObjectValue(schemaObjectAttrTypes, map[string]attr.Value{
			"name":  types.StringValue(name),
			"owner": types.StringValue(owner),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		schemaObjects = append(schemaObjects, obj)
	}
	if err := rows.Err(); err != nil {
		resp.Diagnostics.AddError("Error iterating schema rows", err.Error())
		return
	}

	schemasList, diags := types.ListValue(types.ObjectType{AttrTypes: schemaObjectAttrTypes}, schemaObjects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Schemas = schemasList

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
