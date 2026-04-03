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
	_ datasource.DataSource              = (*tablesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*tablesDataSource)(nil)
)

type tablesDataSource struct {
	db common.DBTX
}

type tablesDataSourceModel struct {
	Database       types.String `tfsdk:"database"`
	Schema         types.String `tfsdk:"schema"`
	LikePattern    types.String `tfsdk:"like_pattern"`
	NotLikePattern types.String `tfsdk:"not_like_pattern"`
	TableType      types.String `tfsdk:"table_type"`
	Tables         types.List   `tfsdk:"tables"`
}

var tableObjectAttrTypes = map[string]attr.Type{
	"name":   types.StringType,
	"schema": types.StringType,
	"type":   types.StringType,
	"owner":  types.StringType,
}

func NewTablesDataSource() datasource.DataSource {
	return &tablesDataSource{}
}

func (d *tablesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tables"
}

func (d *tablesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists PostgreSQL tables with optional filtering.",
		Attributes: map[string]schema.Attribute{
			"database": schema.StringAttribute{
				Description: "The database to query. Uses the provider default if not set.",
				Optional:    true,
			},
			"schema": schema.StringAttribute{
				Description: "Filter tables by schema name.",
				Optional:    true,
			},
			"like_pattern": schema.StringAttribute{
				Description: "A SQL LIKE pattern to filter table names.",
				Optional:    true,
			},
			"not_like_pattern": schema.StringAttribute{
				Description: "A SQL NOT LIKE pattern to exclude table names.",
				Optional:    true,
			},
			"table_type": schema.StringAttribute{
				Description: "Filter by table type: BASE TABLE, VIEW, or FOREIGN TABLE.",
				Optional:    true,
			},
			"tables": schema.ListNestedAttribute{
				Description: "List of tables matching the filters.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the table.",
							Computed:    true,
						},
						"schema": schema.StringAttribute{
							Description: "The schema the table belongs to.",
							Computed:    true,
						},
						"type": schema.StringAttribute{
							Description: "The type of the table (BASE TABLE, VIEW, FOREIGN TABLE).",
							Computed:    true,
						},
						"owner": schema.StringAttribute{
							Description: "The owner of the table.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *tablesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *tablesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state tablesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := `SELECT t.table_name, t.table_schema, t.table_type, COALESCE(pt.tableowner, '')
		FROM information_schema.tables t
		LEFT JOIN pg_catalog.pg_tables pt ON t.table_name = pt.tablename AND t.table_schema = pt.schemaname`
	var conditions []string
	var args []interface{}
	argIdx := 1

	if common.IsSet(state.Schema) {
		conditions = append(conditions, fmt.Sprintf(`t.table_schema = $%d`, argIdx))
		args = append(args, state.Schema.ValueString())
		argIdx++
	}

	if common.IsSet(state.LikePattern) {
		conditions = append(conditions, fmt.Sprintf(`t.table_name LIKE $%d`, argIdx))
		args = append(args, state.LikePattern.ValueString())
		argIdx++
	}

	if common.IsSet(state.NotLikePattern) {
		conditions = append(conditions, fmt.Sprintf(`t.table_name NOT LIKE $%d`, argIdx))
		args = append(args, state.NotLikePattern.ValueString())
		argIdx++
	}

	if common.IsSet(state.TableType) {
		conditions = append(conditions, fmt.Sprintf(`t.table_type = $%d`, argIdx))
		args = append(args, state.TableType.ValueString())
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

	query += " ORDER BY t.table_schema, t.table_name"

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		resp.Diagnostics.AddError("Error querying tables", fmt.Sprintf("Could not query tables: %s", err.Error()))
		return
	}
	defer rows.Close() //nolint:errcheck

	var tableObjects []attr.Value
	for rows.Next() {
		var name, schemaName, tableType, owner string
		if err := rows.Scan(&name, &schemaName, &tableType, &owner); err != nil {
			resp.Diagnostics.AddError("Error scanning table row", err.Error())
			return
		}

		obj, diags := types.ObjectValue(tableObjectAttrTypes, map[string]attr.Value{
			"name":   types.StringValue(name),
			"schema": types.StringValue(schemaName),
			"type":   types.StringValue(tableType),
			"owner":  types.StringValue(owner),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		tableObjects = append(tableObjects, obj)
	}
	if err := rows.Err(); err != nil {
		resp.Diagnostics.AddError("Error iterating table rows", err.Error())
		return
	}

	tablesList, diags := types.ListValue(types.ObjectType{AttrTypes: tableObjectAttrTypes}, tableObjects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Tables = tablesList

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
