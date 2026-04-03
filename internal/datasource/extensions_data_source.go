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
	_ datasource.DataSource              = (*extensionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*extensionsDataSource)(nil)
)

type extensionsDataSource struct {
	db common.DBTX
}

type extensionsDataSourceModel struct {
	Database   types.String `tfsdk:"database"`
	Extensions types.List   `tfsdk:"extensions"`
}

var extensionObjectAttrTypes = map[string]attr.Type{
	"name":        types.StringType,
	"version":     types.StringType,
	"schema":      types.StringType,
	"description": types.StringType,
}

func NewExtensionsDataSource() datasource.DataSource {
	return &extensionsDataSource{}
}

func (d *extensionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_extensions"
}

func (d *extensionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists installed PostgreSQL extensions.",
		Attributes: map[string]schema.Attribute{
			"database": schema.StringAttribute{
				Description: "The database to query. Uses the provider default if not set.",
				Optional:    true,
			},
			"extensions": schema.ListNestedAttribute{
				Description: "List of installed extensions.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the extension.",
							Computed:    true,
						},
						"version": schema.StringAttribute{
							Description: "The installed version of the extension.",
							Computed:    true,
						},
						"schema": schema.StringAttribute{
							Description: "The schema the extension is installed in.",
							Computed:    true,
						},
						"description": schema.StringAttribute{
							Description: "A description of the extension.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *extensionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *extensionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state extensionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := `SELECT e.extname, e.extversion, n.nspname, COALESCE(a.comment, '')
		FROM pg_catalog.pg_extension e
		JOIN pg_catalog.pg_namespace n ON e.extnamespace = n.oid
		LEFT JOIN pg_catalog.pg_available_extensions a ON e.extname = a.name
		ORDER BY e.extname`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		resp.Diagnostics.AddError("Error querying extensions", fmt.Sprintf("Could not query extensions: %s", err.Error()))
		return
	}
	defer rows.Close() //nolint:errcheck

	var extObjects []attr.Value
	for rows.Next() {
		var name, version, schemaName, description string
		if err := rows.Scan(&name, &version, &schemaName, &description); err != nil {
			resp.Diagnostics.AddError("Error scanning extension row", err.Error())
			return
		}

		obj, diags := types.ObjectValue(extensionObjectAttrTypes, map[string]attr.Value{
			"name":        types.StringValue(name),
			"version":     types.StringValue(version),
			"schema":      types.StringValue(schemaName),
			"description": types.StringValue(description),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		extObjects = append(extObjects, obj)
	}
	if err := rows.Err(); err != nil {
		resp.Diagnostics.AddError("Error iterating extension rows", err.Error())
		return
	}

	extList, diags := types.ListValue(types.ObjectType{AttrTypes: extensionObjectAttrTypes}, extObjects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Extensions = extList

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
