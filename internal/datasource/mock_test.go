package datasource

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// newReadReqResp builds a datasource.ReadRequest and ReadResponse using the
// given schema and tftypes config value. The config is used both for Config
// and the initial (empty) State, which is required so that resp.State.Set
// can work after a successful Read.
func newReadReqResp(ctx context.Context, s dschema.Schema, configVal tftypes.Value) (datasource.ReadRequest, *datasource.ReadResponse) {
	tfType := s.Type().TerraformType(ctx)
	nullState := tftypes.NewValue(tfType, nil)

	req := datasource.ReadRequest{
		Config: tfsdk.Config{
			Raw:    configVal,
			Schema: s,
		},
	}
	resp := &datasource.ReadResponse{
		State: tfsdk.State{
			Raw:    nullState,
			Schema: s,
		},
	}
	return req, resp
}

// ---------------------------------------------------------------------------
// versionDataSource
// ---------------------------------------------------------------------------

func versionSchema() dschema.Schema {
	d := &versionDataSource{}
	sreq := datasource.SchemaRequest{}
	sresp := &datasource.SchemaResponse{}
	d.Schema(context.Background(), sreq, sresp)
	return sresp.Schema
}

func versionConfigValue(ctx context.Context, s dschema.Schema) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"version":            tftypes.NewValue(tftypes.String, nil),
		"major":              tftypes.NewValue(tftypes.Number, nil),
		"minor":              tftypes.NewValue(tftypes.Number, nil),
		"server_version_num": tftypes.NewValue(tftypes.Number, nil),
	})
}

func TestVersionDataSource_Read_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT version()").WillReturnError(fmt.Errorf("connection refused"))

	ctx := context.Background()
	s := versionSchema()
	req, resp := newReadReqResp(ctx, s, versionConfigValue(ctx, s))

	d := &versionDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying version" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error querying version' diagnostic")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %s", err)
	}
}

func TestVersionDataSource_Read_serverVersionNumQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// First query succeeds
	rows := sqlmock.NewRows([]string{"version"}).AddRow("PostgreSQL 16.2 on x86_64")
	mock.ExpectQuery("SELECT version()").WillReturnRows(rows)
	// Second query fails
	mock.ExpectQuery("SHOW server_version_num").WillReturnError(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := versionSchema()
	req, resp := newReadReqResp(ctx, s, versionConfigValue(ctx, s))

	d := &versionDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for server_version_num query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying server_version_num" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error querying server_version_num' diagnostic")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %s", err)
	}
}

func TestVersionDataSource_Read_serverVersionNumParseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows1 := sqlmock.NewRows([]string{"version"}).AddRow("PostgreSQL 16.2 on x86_64")
	mock.ExpectQuery("SELECT version()").WillReturnRows(rows1)
	rows2 := sqlmock.NewRows([]string{"server_version_num"}).AddRow("not_a_number")
	mock.ExpectQuery("SHOW server_version_num").WillReturnRows(rows2)

	ctx := context.Background()
	s := versionSchema()
	req, resp := newReadReqResp(ctx, s, versionConfigValue(ctx, s))

	d := &versionDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for parse failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error parsing server_version_num" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error parsing server_version_num' diagnostic")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %s", err)
	}
}

// ---------------------------------------------------------------------------
// extensionsDataSource
// ---------------------------------------------------------------------------

func extensionsSchema() dschema.Schema {
	d := &extensionsDataSource{}
	sresp := &datasource.SchemaResponse{}
	d.Schema(context.Background(), datasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func extensionsConfigValue(ctx context.Context, s dschema.Schema) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	// extensions is a list of objects - use nil for computed
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"database":   tftypes.NewValue(tftypes.String, nil),
		"extensions": tftypes.NewValue(tftypes.List{ElementType: extensionsListTfType()}, nil),
	})
}

func extensionsListTfType() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":        tftypes.String,
		"version":     tftypes.String,
		"schema":      tftypes.String,
		"description": tftypes.String,
	}}
}

func TestExtensionsDataSource_Read_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := extensionsSchema()
	req, resp := newReadReqResp(ctx, s, extensionsConfigValue(ctx, s))

	d := &extensionsDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying extensions" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error querying extensions' diagnostic, got: %v", resp.Diagnostics.Errors())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %s", err)
	}
}

func TestExtensionsDataSource_Read_scanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Return rows with wrong column count to trigger scan error
	rows := sqlmock.NewRows([]string{"name", "version"}).AddRow("plpgsql", "1.0")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	ctx := context.Background()
	s := extensionsSchema()
	req, resp := newReadReqResp(ctx, s, extensionsConfigValue(ctx, s))

	d := &extensionsDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for scan failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error scanning extension row" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error scanning extension row' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestExtensionsDataSource_Read_rowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"name", "version", "schema", "description"}).
		AddRow("plpgsql", "1.0", "pg_catalog", "PL/pgSQL").
		RowError(0, fmt.Errorf("row iteration error"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	ctx := context.Background()
	s := extensionsSchema()
	req, resp := newReadReqResp(ctx, s, extensionsConfigValue(ctx, s))

	d := &extensionsDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for rows.Err()")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error iterating extension rows" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error iterating extension rows' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// tablesDataSource
// ---------------------------------------------------------------------------

func tablesSchema() dschema.Schema {
	d := &tablesDataSource{}
	sresp := &datasource.SchemaResponse{}
	d.Schema(context.Background(), datasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func tablesConfigValue(ctx context.Context, s dschema.Schema) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	tablesObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":   tftypes.String,
		"schema": tftypes.String,
		"type":   tftypes.String,
		"owner":  tftypes.String,
	}}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"database":         tftypes.NewValue(tftypes.String, nil),
		"schema":           tftypes.NewValue(tftypes.String, nil),
		"like_pattern":     tftypes.NewValue(tftypes.String, nil),
		"not_like_pattern": tftypes.NewValue(tftypes.String, nil),
		"table_type":       tftypes.NewValue(tftypes.String, nil),
		"tables":           tftypes.NewValue(tftypes.List{ElementType: tablesObjType}, nil),
	})
}

func TestTablesDataSource_Read_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("relation does not exist"))

	ctx := context.Background()
	s := tablesSchema()
	req, resp := newReadReqResp(ctx, s, tablesConfigValue(ctx, s))

	d := &tablesDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying tables" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error querying tables' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestTablesDataSource_Read_scanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Wrong column count to trigger scan error
	rows := sqlmock.NewRows([]string{"name"}).AddRow("test_table")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	ctx := context.Background()
	s := tablesSchema()
	req, resp := newReadReqResp(ctx, s, tablesConfigValue(ctx, s))

	d := &tablesDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for scan failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error scanning table row" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error scanning table row' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestTablesDataSource_Read_rowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"table_name", "table_schema", "table_type", "tableowner"}).
		AddRow("t1", "public", "BASE TABLE", "postgres").
		RowError(0, fmt.Errorf("row iteration error"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	ctx := context.Background()
	s := tablesSchema()
	req, resp := newReadReqResp(ctx, s, tablesConfigValue(ctx, s))

	d := &tablesDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for rows.Err()")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error iterating table rows" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error iterating table rows' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// roleDataSource
// ---------------------------------------------------------------------------

func roleSchema() dschema.Schema {
	d := &roleDataSource{}
	sresp := &datasource.SchemaResponse{}
	d.Schema(context.Background(), datasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func roleConfigValue(ctx context.Context, s dschema.Schema, name string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"oid":              tftypes.NewValue(tftypes.Number, nil),
		"login":            tftypes.NewValue(tftypes.Bool, nil),
		"superuser":        tftypes.NewValue(tftypes.Bool, nil),
		"create_database":  tftypes.NewValue(tftypes.Bool, nil),
		"create_role":      tftypes.NewValue(tftypes.Bool, nil),
		"replication":      tftypes.NewValue(tftypes.Bool, nil),
		"connection_limit": tftypes.NewValue(tftypes.Number, nil),
		"valid_until":      tftypes.NewValue(tftypes.String, nil),
		"roles":            tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
	})
}

func TestRoleDataSource_Read_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT oid").WillReturnError(fmt.Errorf("role not found"))

	ctx := context.Background()
	s := roleSchema()
	req, resp := newReadReqResp(ctx, s, roleConfigValue(ctx, s, "nonexistent"))

	d := &roleDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error reading role" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error reading role' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestRoleDataSource_Read_membershipQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// First query succeeds
	rows := sqlmock.NewRows([]string{"oid", "rolcanlogin", "rolsuper", "rolcreatedb", "rolcreaterole", "rolreplication", "rolconnlimit", "rolvaliduntil"}).
		AddRow(16384, true, false, false, false, false, -1, nil)
	mock.ExpectQuery("SELECT oid").WillReturnRows(rows)

	// Membership query fails
	mock.ExpectQuery("SELECT r.rolname").WillReturnError(fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := roleSchema()
	req, resp := newReadReqResp(ctx, s, roleConfigValue(ctx, s, "testrole"))

	d := &roleDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for membership query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error reading role memberships" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error reading role memberships' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestRoleDataSource_Read_membershipScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows1 := sqlmock.NewRows([]string{"oid", "rolcanlogin", "rolsuper", "rolcreatedb", "rolcreaterole", "rolreplication", "rolconnlimit", "rolvaliduntil"}).
		AddRow(16384, true, false, false, false, false, -1, nil)
	mock.ExpectQuery("SELECT oid").WillReturnRows(rows1)

	// Membership query returns wrong columns
	rows2 := sqlmock.NewRows([]string{"a", "b"}).AddRow("role1", 42)
	mock.ExpectQuery("SELECT r.rolname").WillReturnRows(rows2)

	ctx := context.Background()
	s := roleSchema()
	req, resp := newReadReqResp(ctx, s, roleConfigValue(ctx, s, "testrole"))

	d := &roleDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for membership scan failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error scanning role membership" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error scanning role membership' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestRoleDataSource_Read_membershipRowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows1 := sqlmock.NewRows([]string{"oid", "rolcanlogin", "rolsuper", "rolcreatedb", "rolcreaterole", "rolreplication", "rolconnlimit", "rolvaliduntil"}).
		AddRow(16384, true, false, false, false, false, -1, nil)
	mock.ExpectQuery("SELECT oid").WillReturnRows(rows1)

	rows2 := sqlmock.NewRows([]string{"rolname"}).
		AddRow("admin").
		RowError(0, fmt.Errorf("row iteration failure"))
	mock.ExpectQuery("SELECT r.rolname").WillReturnRows(rows2)

	ctx := context.Background()
	s := roleSchema()
	req, resp := newReadReqResp(ctx, s, roleConfigValue(ctx, s, "testrole"))

	d := &roleDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for rows.Err()")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error iterating role memberships" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error iterating role memberships' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// rolesDataSource
// ---------------------------------------------------------------------------

func rolesSchema() dschema.Schema {
	d := &rolesDataSource{}
	sresp := &datasource.SchemaResponse{}
	d.Schema(context.Background(), datasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func rolesConfigValue(ctx context.Context, s dschema.Schema) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	rolesObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":             tftypes.String,
		"oid":              tftypes.Number,
		"login":            tftypes.Bool,
		"superuser":        tftypes.Bool,
		"create_database":  tftypes.Bool,
		"create_role":      tftypes.Bool,
		"replication":      tftypes.Bool,
		"connection_limit": tftypes.Number,
	}}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"like_pattern":     tftypes.NewValue(tftypes.String, nil),
		"not_like_pattern": tftypes.NewValue(tftypes.String, nil),
		"login_only":       tftypes.NewValue(tftypes.Bool, nil),
		"roles":            tftypes.NewValue(tftypes.List{ElementType: rolesObjType}, nil),
	})
}

func TestRolesDataSource_Read_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT rolname").WillReturnError(fmt.Errorf("access denied"))

	ctx := context.Background()
	s := rolesSchema()
	req, resp := newReadReqResp(ctx, s, rolesConfigValue(ctx, s))

	d := &rolesDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying roles" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error querying roles' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestRolesDataSource_Read_scanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Return wrong number of columns
	rows := sqlmock.NewRows([]string{"rolname"}).AddRow("test")
	mock.ExpectQuery("SELECT rolname").WillReturnRows(rows)

	ctx := context.Background()
	s := rolesSchema()
	req, resp := newReadReqResp(ctx, s, rolesConfigValue(ctx, s))

	d := &rolesDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for scan failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error scanning role row" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error scanning role row' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestRolesDataSource_Read_rowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"rolname", "oid", "rolcanlogin", "rolsuper", "rolcreatedb", "rolcreaterole", "rolreplication", "rolconnlimit"}).
		AddRow("postgres", 10, true, true, true, true, true, -1).
		RowError(0, fmt.Errorf("row iteration error"))
	mock.ExpectQuery("SELECT rolname").WillReturnRows(rows)

	ctx := context.Background()
	s := rolesSchema()
	req, resp := newReadReqResp(ctx, s, rolesConfigValue(ctx, s))

	d := &rolesDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for rows.Err()")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error iterating role rows" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error iterating role rows' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// schemasDataSource
// ---------------------------------------------------------------------------

func schemasSchemaFn() dschema.Schema {
	d := &schemasDataSource{}
	sresp := &datasource.SchemaResponse{}
	d.Schema(context.Background(), datasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func schemasConfigValue(ctx context.Context, s dschema.Schema) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	schemasObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":  tftypes.String,
		"owner": tftypes.String,
	}}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"database":               tftypes.NewValue(tftypes.String, nil),
		"like_pattern":           tftypes.NewValue(tftypes.String, nil),
		"not_like_pattern":       tftypes.NewValue(tftypes.String, nil),
		"include_system_schemas": tftypes.NewValue(tftypes.Bool, nil),
		"schemas":                tftypes.NewValue(tftypes.List{ElementType: schemasObjType}, nil),
	})
}

func TestSchemasDataSource_Read_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := schemasSchemaFn()
	req, resp := newReadReqResp(ctx, s, schemasConfigValue(ctx, s))

	d := &schemasDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying schemas" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error querying schemas' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestSchemasDataSource_Read_scanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"nspname"}).AddRow("public")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	ctx := context.Background()
	s := schemasSchemaFn()
	req, resp := newReadReqResp(ctx, s, schemasConfigValue(ctx, s))

	d := &schemasDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for scan failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error scanning schema row" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error scanning schema row' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestSchemasDataSource_Read_rowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"nspname", "rolname"}).
		AddRow("public", "postgres").
		RowError(0, fmt.Errorf("row iteration error"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	ctx := context.Background()
	s := schemasSchemaFn()
	req, resp := newReadReqResp(ctx, s, schemasConfigValue(ctx, s))

	d := &schemasDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for rows.Err()")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error iterating schema rows" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error iterating schema rows' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// databaseDataSource
// ---------------------------------------------------------------------------

func databaseSchema() dschema.Schema {
	d := &databaseDataSource{}
	sresp := &datasource.SchemaResponse{}
	d.Schema(context.Background(), datasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func databaseConfigValue(ctx context.Context, s dschema.Schema, name string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":              tftypes.NewValue(tftypes.String, name),
		"oid":               tftypes.NewValue(tftypes.Number, nil),
		"owner":             tftypes.NewValue(tftypes.String, nil),
		"encoding":          tftypes.NewValue(tftypes.String, nil),
		"lc_collate":        tftypes.NewValue(tftypes.String, nil),
		"lc_ctype":          tftypes.NewValue(tftypes.String, nil),
		"tablespace_name":   tftypes.NewValue(tftypes.String, nil),
		"connection_limit":  tftypes.NewValue(tftypes.Number, nil),
		"allow_connections": tftypes.NewValue(tftypes.Bool, nil),
		"is_template":       tftypes.NewValue(tftypes.Bool, nil),
	})
}

func TestDatabaseDataSource_Read_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT d.oid").WillReturnError(fmt.Errorf("database not found"))

	ctx := context.Background()
	s := databaseSchema()
	req, resp := newReadReqResp(ctx, s, databaseConfigValue(ctx, s, "nonexistent"))

	d := &databaseDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error reading database" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error reading database' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// queryDataSource
// ---------------------------------------------------------------------------

func querySchema() dschema.Schema {
	d := &queryDataSource{}
	sresp := &datasource.SchemaResponse{}
	d.Schema(context.Background(), datasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func queryConfigValue(ctx context.Context, s dschema.Schema, query, database string) tftypes.Value {
	return queryConfigValueWithDestructive(ctx, s, query, database, nil)
}

func queryConfigValueWithDestructive(ctx context.Context, s dschema.Schema, query, database string, allowDestructive *bool) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var destructiveVal tftypes.Value
	if allowDestructive != nil {
		destructiveVal = tftypes.NewValue(tftypes.Bool, *allowDestructive)
	} else {
		destructiveVal = tftypes.NewValue(tftypes.Bool, nil)
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"query":             tftypes.NewValue(tftypes.String, query),
		"database":          tftypes.NewValue(tftypes.String, database),
		"allow_destructive": destructiveVal,
		"rows":              tftypes.NewValue(tftypes.List{ElementType: tftypes.Map{ElementType: tftypes.String}}, nil),
	})
}

func TestQueryDataSource_Read_nonSelectQuery(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "INSERT INTO foo VALUES(1)", "postgres"))

	d := &queryDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for non-SELECT query")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Invalid Query" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Invalid Query' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestQueryDataSource_Read_beginTxError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin().WillReturnError(fmt.Errorf("cannot start transaction"))

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "SELECT 1", "postgres"))

	d := &queryDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for BeginTx failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error starting transaction" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error starting transaction' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestQueryDataSource_Read_queryExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("syntax error"))
	mock.ExpectRollback()

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "SELECT bad_syntax", "postgres"))

	d := &queryDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query execution failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error executing query" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error executing query' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestQueryDataSource_Read_scanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	// Return rows that will fail during scan by using CloseError
	rows := sqlmock.NewRows([]string{"col1"}).AddRow("val1").
		RowError(0, fmt.Errorf("scan failure during iteration"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	mock.ExpectRollback()

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "SELECT col1 FROM t", "postgres"))

	d := &queryDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for scan/rows error")
	}
}

func TestQueryDataSource_Read_withCTE(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"num"}).AddRow("1")
	mock.ExpectQuery("WITH").WillReturnRows(rows)
	mock.ExpectCommit()

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "WITH cte AS (SELECT 1 AS num) SELECT num FROM cte", "postgres"))

	d := &queryDataSource{db: db}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// allow_destructive tests
// ---------------------------------------------------------------------------

func TestQueryDataSource_Read_destructiveBlocked(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	s := querySchema()
	// DELETE without allow_destructive → should be rejected
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "DELETE FROM sessions WHERE expired = true", "postgres"))

	d := &queryDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for DELETE without allow_destructive")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Invalid Query" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Invalid Query' diagnostic")
	}
}

func TestQueryDataSource_Read_destructiveAllowed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"id"}).AddRow("1")
	mock.ExpectQuery("DELETE").WillReturnRows(rows)
	mock.ExpectCommit()

	ctx := context.Background()
	s := querySchema()
	allowDestructive := true
	req, resp := newReadReqResp(ctx, s, queryConfigValueWithDestructive(ctx, s, "DELETE FROM sessions WHERE expired = true RETURNING id", "postgres", &allowDestructive))

	d := &queryDataSource{db: db}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestQueryDataSource_Read_destructiveFalseBlocked(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	s := querySchema()
	// Explicit allow_destructive = false → still blocked
	allowDestructive := false
	req, resp := newReadReqResp(ctx, s, queryConfigValueWithDestructive(ctx, s, "DROP TABLE users", "postgres", &allowDestructive))

	d := &queryDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for DROP with allow_destructive=false")
	}
}

func TestQueryDataSource_Read_commitError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"num"}).AddRow("1")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	mock.ExpectCommit().WillReturnError(fmt.Errorf("commit failed"))
	mock.ExpectRollback()

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "SELECT 1 AS num", "postgres"))

	d := &queryDataSource{db: db}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for commit failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error committing transaction" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error committing transaction' diagnostic")
	}
}
