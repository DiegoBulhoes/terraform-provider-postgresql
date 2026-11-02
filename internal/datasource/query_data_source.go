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
	_ datasource.DataSource              = (*QueryDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*QueryDataSource)(nil)
)

type QueryDataSource struct {
	DB common.DBTX
}

type QueryDataSourceModel struct {
	Query            types.String `tfsdk:"query"`
	Database         types.String `tfsdk:"database"`
	AllowDestructive types.Bool   `tfsdk:"allow_destructive"`
	Rows             types.List   `tfsdk:"rows"`
}

func NewQueryDataSource() datasource.DataSource {
	return &QueryDataSource{}
}

func (d *QueryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_query"
}

func (d *QueryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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

func (d *QueryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *QueryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state QueryDataSourceModel
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
	db, err := common.OnDatabase(ctx, d.DB, state.Database.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to database", err.Error())
		return
	}

	tx, err := db.BeginTx(ctx, txOpts)
	if err != nil {
		resp.Diagnostics.AddError("Error starting transaction", err.Error())
		return
	}
	defer common.LogRollback(ctx, tx)

	rows, err := runQuery(ctx, tx, queryStr, allowDestructive)
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
	// Rows are maps keyed by column name, so a repeated name would drop a value.
	seen := make(map[string]bool, len(columns))
	for _, col := range columns {
		if seen[col] {
			resp.Diagnostics.AddError("Duplicate column name",
				fmt.Sprintf("The query returns duplicate column name %q; give each column a distinct alias.", col))
			return
		}
		seen[col] = true
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

// runQuery prepares read-only queries: the simple protocol would run every
// statement in the string, and a COMMIT among them ends the read-only transaction.
func runQuery(ctx context.Context, tx common.Tx, query string, allowDestructive bool) (common.Rows, error) {
	if allowDestructive {
		return tx.QueryContext(ctx, query)
	}
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		_ = stmt.Close()
		return nil, err
	}
	return &stmtRows{Rows: rows, stmt: stmt}, nil
}

// stmtRows closes its statement after the rows; closing it first breaks the connection.
type stmtRows struct {
	common.Rows
	stmt common.Stmt
}

func (r *stmtRows) Close() error {
	err := r.Rows.Close()
	if cerr := r.stmt.Close(); err == nil {
		err = cerr
	}
	return err
}
