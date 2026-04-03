package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*queryDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*queryDataSource)(nil)
)

type queryDataSource struct {
	db common.DBTX
}

type queryDataSourceModel struct {
	Query            types.String `tfsdk:"query"`
	Database         types.String `tfsdk:"database"`
	AllowDestructive types.Bool   `tfsdk:"allow_destructive"`
	Rows             types.List   `tfsdk:"rows"`
}

func NewQueryDataSource() datasource.DataSource {
	return &queryDataSource{}
}

func (d *queryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_query"
}

func (d *queryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Executes a SQL query and returns the results. By default, only SELECT queries (including CTEs with WITH) are allowed and run inside a read-only transaction for safety. Set `allow_destructive = true` to permit DML/DDL statements.",
		Attributes: map[string]schema.Attribute{
			"query": schema.StringAttribute{
				Description: "The SQL query to execute. By default, must be a SELECT statement or a CTE starting with WITH. Set `allow_destructive = true` to allow other statements.",
				Required:    true,
			},
			"database": schema.StringAttribute{
				Description: "The database to execute the query against.",
				Required:    true,
			},
			"allow_destructive": schema.BoolAttribute{
				Description: "When true, allows DML (INSERT, UPDATE, DELETE) and DDL (CREATE, DROP, ALTER) queries. By default only SELECT is permitted. Use with caution.",
				Optional:    true,
			},
			"rows": schema.ListAttribute{
				Description: "The query result rows. Each row is a map of column name to string value.",
				Computed:    true,
				ElementType: types.MapType{ElemType: types.StringType},
			},
		},
	}
}

func (d *queryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *queryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state queryDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queryStr := state.Query.ValueString()
	allowDestructive := state.AllowDestructive.ValueBool() // false when null/unknown

	// Validate that the query looks like a SELECT statement (unless destructive is allowed).
	trimmed := strings.TrimSpace(queryStr)
	upper := strings.ToUpper(trimmed)
	if !allowDestructive &&
		!strings.HasPrefix(upper, "SELECT") &&
		!strings.HasPrefix(upper, "WITH") {
		resp.Diagnostics.AddError(
			"Invalid Query",
			"Only SELECT queries are allowed. The query must start with SELECT or WITH (CTE). Set `allow_destructive = true` to allow other statements.",
		)
		return
	}

	// Execute inside a transaction. Read-only unless destructive is explicitly allowed.
	txOpts := &sql.TxOptions{ReadOnly: !allowDestructive}
	tx, err := d.db.BeginTx(ctx, txOpts)
	if err != nil {
		resp.Diagnostics.AddError("Error starting transaction", err.Error())
		return
	}
	defer tx.Rollback() //nolint:errcheck

	rows, err := tx.QueryContext(ctx, queryStr)
	if err != nil {
		resp.Diagnostics.AddError("Error executing query", fmt.Sprintf("Could not execute query: %s", err.Error()))
		return
	}
	defer rows.Close() //nolint:errcheck

	columns, err := rows.Columns()
	if err != nil {
		resp.Diagnostics.AddError("Error reading columns", fmt.Sprintf("Could not read column names: %s", err.Error()))
		return
	}

	var resultRows []attr.Value

	for rows.Next() {
		// Create a slice of *sql.NullString to scan into
		scanArgs := make([]interface{}, len(columns))
		values := make([]*sql.NullString, len(columns))
		for i := range values {
			values[i] = &sql.NullString{}
			scanArgs[i] = values[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			resp.Diagnostics.AddError("Error scanning row", fmt.Sprintf("Could not scan row: %s", err.Error()))
			return
		}

		rowMap := make(map[string]attr.Value, len(columns))
		for i, col := range columns {
			if values[i].Valid {
				rowMap[col] = types.StringValue(values[i].String)
			} else {
				rowMap[col] = types.StringNull()
			}
		}

		mapVal, diags := types.MapValue(types.StringType, rowMap)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		resultRows = append(resultRows, mapVal)
	}
	if err := rows.Err(); err != nil {
		resp.Diagnostics.AddError("Error iterating rows", fmt.Sprintf("Error during row iteration: %s", err.Error()))
		return
	}

	rowsList, diags := types.ListValue(types.MapType{ElemType: types.StringType}, resultRows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Rows = rowsList

	// Commit the transaction so destructive queries take effect.
	// For read-only transactions this is a no-op.
	if err := tx.Commit(); err != nil {
		resp.Diagnostics.AddError("Error committing transaction", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
