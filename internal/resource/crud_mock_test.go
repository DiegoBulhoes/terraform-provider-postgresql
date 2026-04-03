package resource

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// ---------------------------------------------------------------------------
// Helpers to build request/response objects
// ---------------------------------------------------------------------------

var timeoutsObjectType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"create": tftypes.String,
	"update": tftypes.String,
	"delete": tftypes.String,
}}

func newCreateReqResp(ctx context.Context, s rschema.Schema, planVal tftypes.Value) (resource.CreateRequest, *resource.CreateResponse) {
	tfType := s.Type().TerraformType(ctx)
	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Raw: planVal, Schema: s},
	}
	resp := &resource.CreateResponse{
		State: tfsdk.State{Raw: tftypes.NewValue(tfType, nil), Schema: s},
	}
	return req, resp
}

func newReadReqResp(ctx context.Context, s rschema.Schema, stateVal tftypes.Value) (resource.ReadRequest, *resource.ReadResponse) {
	req := resource.ReadRequest{
		State: tfsdk.State{Raw: stateVal, Schema: s},
	}
	resp := &resource.ReadResponse{
		State: tfsdk.State{Raw: stateVal, Schema: s},
	}
	return req, resp
}

func newUpdateReqResp(ctx context.Context, s rschema.Schema, planVal, stateVal tftypes.Value) (resource.UpdateRequest, *resource.UpdateResponse) {
	tfType := s.Type().TerraformType(ctx)
	req := resource.UpdateRequest{
		Plan:  tfsdk.Plan{Raw: planVal, Schema: s},
		State: tfsdk.State{Raw: stateVal, Schema: s},
	}
	resp := &resource.UpdateResponse{
		State: tfsdk.State{Raw: tftypes.NewValue(tfType, nil), Schema: s},
	}
	return req, resp
}

func newDeleteReqResp(_ context.Context, s rschema.Schema, stateVal tftypes.Value) (resource.DeleteRequest, *resource.DeleteResponse) {
	req := resource.DeleteRequest{
		State: tfsdk.State{Raw: stateVal, Schema: s},
	}
	resp := &resource.DeleteResponse{
		State: tfsdk.State{Raw: stateVal, Schema: s},
	}
	return req, resp
}

func newImportReqResp(ctx context.Context, s rschema.Schema, id string) (resource.ImportStateRequest, *resource.ImportStateResponse) {
	tfType := s.Type().TerraformType(ctx)
	req := resource.ImportStateRequest{ID: id}
	resp := &resource.ImportStateResponse{
		State: tfsdk.State{Raw: tftypes.NewValue(tfType, nil), Schema: s},
	}
	return req, resp
}

// hasDiagSummary checks whether resp diagnostics contain the given summary.
func hasDiagSummary(diags interface {
	Errors() []interface{ Summary() string }
}, summary string) bool {
	for _, d := range diags.Errors() {
		if d.Summary() == summary {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// databaseResource helpers
// ---------------------------------------------------------------------------

func databaseResourceSchema() rschema.Schema {
	r := &databaseResource{}
	sresp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func databasePlanValue(ctx context.Context, s rschema.Schema, name, owner, template, encoding, lcCollate, lcCtype, tablespace string, connLimit int64, allowConn, isTemplate bool) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":              tftypes.NewValue(tftypes.String, name),
		"owner":             tftypes.NewValue(tftypes.String, owner),
		"template":          tftypes.NewValue(tftypes.String, template),
		"encoding":          tftypes.NewValue(tftypes.String, encoding),
		"lc_collate":        tftypes.NewValue(tftypes.String, lcCollate),
		"lc_ctype":          tftypes.NewValue(tftypes.String, lcCtype),
		"tablespace_name":   tftypes.NewValue(tftypes.String, tablespace),
		"connection_limit":  tftypes.NewValue(tftypes.Number, connLimit),
		"allow_connections": tftypes.NewValue(tftypes.Bool, allowConn),
		"is_template":       tftypes.NewValue(tftypes.Bool, isTemplate),
		"oid":               tftypes.NewValue(tftypes.Number, nil),
		"timeouts":          tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func databaseStateValue(ctx context.Context, s rschema.Schema, name, owner, template, encoding, lcCollate, lcCtype, tablespace string, connLimit, oid int64, allowConn, isTemplate bool) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":              tftypes.NewValue(tftypes.String, name),
		"owner":             tftypes.NewValue(tftypes.String, owner),
		"template":          tftypes.NewValue(tftypes.String, template),
		"encoding":          tftypes.NewValue(tftypes.String, encoding),
		"lc_collate":        tftypes.NewValue(tftypes.String, lcCollate),
		"lc_ctype":          tftypes.NewValue(tftypes.String, lcCtype),
		"tablespace_name":   tftypes.NewValue(tftypes.String, tablespace),
		"connection_limit":  tftypes.NewValue(tftypes.Number, connLimit),
		"allow_connections": tftypes.NewValue(tftypes.Bool, allowConn),
		"is_template":       tftypes.NewValue(tftypes.Bool, isTemplate),
		"oid":               tftypes.NewValue(tftypes.Number, oid),
		"timeouts":          tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func databaseReadRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"oid", "owner", "encoding", "lc_collate", "lc_ctype",
		"allow_connections", "connection_limit", "is_template", "tablespace_name",
	})
}

// ---------------------------------------------------------------------------
// databaseResource tests
// ---------------------------------------------------------------------------

func TestDatabaseResource_Create_execError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE DATABASE").WillReturnError(fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, true, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &databaseResource{db: db}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error creating database" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error creating database' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDatabaseResource_Create_readError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE DATABASE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, true, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &databaseResource{db: db}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error reading database" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error reading database' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDatabaseResource_Read_notFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Return empty rows to trigger sql.ErrNoRows on QueryRowContext.Scan
	rows := databaseReadRows()
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	ctx := context.Background()
	s := databaseResourceSchema()
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &databaseResource{db: db}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed (null) when database not found")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDatabaseResource_Read_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("connection refused"))

	ctx := context.Background()
	s := databaseResourceSchema()
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &databaseResource{db: db}
	r.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error reading database" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error reading database' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDatabaseResource_Update_ownerChangeError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("ALTER DATABASE").WillReturnError(fmt.Errorf("role does not exist"))

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "newowner", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, true, false)
	state := databaseStateValue(ctx, s, "testdb", "oldowner", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &databaseResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error updating database owner" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error updating database owner' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDatabaseResource_Update_tablespaceError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Owner is the same, so no owner ALTER. Tablespace change triggers error.
	mock.ExpectExec("ALTER DATABASE.*SET TABLESPACE").WillReturnError(fmt.Errorf("tablespace not found"))

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "fast_ssd", -1, true, false)
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &databaseResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error updating database tablespace" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error updating database tablespace' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDatabaseResource_Update_withOptsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Owner same, tablespace same, connection_limit changes -> ALTER DATABASE ... WITH
	mock.ExpectExec("ALTER DATABASE.*WITH").WillReturnError(fmt.Errorf("syntax error"))

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", 10, true, false)
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &databaseResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error updating database" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error updating database' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDatabaseResource_Delete_templateUnsetError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// is_template=true => ALTER DATABASE ... IS_TEMPLATE = false first
	mock.ExpectExec("ALTER DATABASE").WillReturnError(fmt.Errorf("cannot alter"))

	ctx := context.Background()
	s := databaseResourceSchema()
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, true)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &databaseResource{db: db}
	r.Delete(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error disabling template on database" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error disabling template on database' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDatabaseResource_Delete_dropError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// is_template=false so no ALTER needed, go straight to DROP
	mock.ExpectExec("DROP DATABASE").WillReturnError(fmt.Errorf("database is being accessed"))

	ctx := context.Background()
	s := databaseResourceSchema()
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &databaseResource{db: db}
	r.Delete(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error deleting database" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error deleting database' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

// ---------------------------------------------------------------------------
// roleResource helpers
// ---------------------------------------------------------------------------

func roleResourceSchema() rschema.Schema {
	r := &roleResource{}
	sresp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func rolePlanValue(ctx context.Context, s rschema.Schema, name string, login, superuser, createDB, createRole, replication bool, connLimit int64) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"password":         tftypes.NewValue(tftypes.String, nil),
		"login":            tftypes.NewValue(tftypes.Bool, login),
		"superuser":        tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":  tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":      tftypes.NewValue(tftypes.Bool, createRole),
		"replication":      tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit": tftypes.NewValue(tftypes.Number, connLimit),
		"valid_until":      tftypes.NewValue(tftypes.String, nil),
		"roles":            tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		"oid":              tftypes.NewValue(tftypes.Number, nil),
		"timeouts":         tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func rolePlanValueWithRoles(ctx context.Context, s rschema.Schema, name string, login, superuser, createDB, createRole, replication bool, connLimit int64, roles []string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var roleVals []tftypes.Value
	for _, r := range roles {
		roleVals = append(roleVals, tftypes.NewValue(tftypes.String, r))
	}
	var rolesVal tftypes.Value
	if roleVals != nil {
		rolesVal = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, roleVals)
	} else {
		rolesVal = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{})
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"password":         tftypes.NewValue(tftypes.String, nil),
		"login":            tftypes.NewValue(tftypes.Bool, login),
		"superuser":        tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":  tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":      tftypes.NewValue(tftypes.Bool, createRole),
		"replication":      tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit": tftypes.NewValue(tftypes.Number, connLimit),
		"valid_until":      tftypes.NewValue(tftypes.String, nil),
		"roles":            rolesVal,
		"oid":              tftypes.NewValue(tftypes.Number, nil),
		"timeouts":         tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func roleStateValue(ctx context.Context, s rschema.Schema, name string, login, superuser, createDB, createRole, replication bool, connLimit, oid int64) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"password":         tftypes.NewValue(tftypes.String, nil),
		"login":            tftypes.NewValue(tftypes.Bool, login),
		"superuser":        tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":  tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":      tftypes.NewValue(tftypes.Bool, createRole),
		"replication":      tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit": tftypes.NewValue(tftypes.Number, connLimit),
		"valid_until":      tftypes.NewValue(tftypes.String, nil),
		"roles":            tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		"oid":              tftypes.NewValue(tftypes.Number, oid),
		"timeouts":         tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func roleStateValueWithRoles(ctx context.Context, s rschema.Schema, name string, login, superuser, createDB, createRole, replication bool, connLimit, oid int64, roles []string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var roleVals []tftypes.Value
	for _, r := range roles {
		roleVals = append(roleVals, tftypes.NewValue(tftypes.String, r))
	}
	var rolesVal tftypes.Value
	if roleVals != nil {
		rolesVal = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, roleVals)
	} else {
		rolesVal = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{})
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"password":         tftypes.NewValue(tftypes.String, nil),
		"login":            tftypes.NewValue(tftypes.Bool, login),
		"superuser":        tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":  tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":      tftypes.NewValue(tftypes.Bool, createRole),
		"replication":      tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit": tftypes.NewValue(tftypes.Number, connLimit),
		"valid_until":      tftypes.NewValue(tftypes.String, nil),
		"roles":            rolesVal,
		"oid":              tftypes.NewValue(tftypes.Number, oid),
		"timeouts":         tftypes.NewValue(timeoutsObjectType, nil),
	})
}

// ---------------------------------------------------------------------------
// roleResource tests
// ---------------------------------------------------------------------------

func TestRoleResource_Create_beginTxError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin().WillReturnError(fmt.Errorf("cannot begin"))

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValue(ctx, s, "testrole", false, false, false, false, false, -1)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &roleResource{db: db}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error starting transaction" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error starting transaction' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestRoleResource_Create_execError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("CREATE ROLE").WillReturnError(fmt.Errorf("role already exists"))
	mock.ExpectRollback()

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValue(ctx, s, "testrole", false, false, false, false, false, -1)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &roleResource{db: db}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error creating role" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error creating role' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestRoleResource_Create_grantMembershipError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("CREATE ROLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("GRANT").WillReturnError(fmt.Errorf("role admin does not exist"))
	mock.ExpectRollback()

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValueWithRoles(ctx, s, "testrole", false, false, false, false, false, -1, []string{"admin"})
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &roleResource{db: db}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error granting role membership" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error granting role membership' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestRoleResource_Create_commitError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("CREATE ROLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit().WillReturnError(fmt.Errorf("commit failed"))

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValue(ctx, s, "testrole", false, false, false, false, false, -1)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &roleResource{db: db}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error committing transaction" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error committing transaction' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestRoleResource_Read_notFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Return empty result set
	rows := sqlmock.NewRows([]string{"oid", "rolcanlogin", "rolsuper", "rolcreatedb", "rolcreaterole", "rolreplication", "rolconnlimit", "rolvaliduntil"})
	mock.ExpectQuery("SELECT oid").WillReturnRows(rows)

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleStateValue(ctx, s, "testrole", false, false, false, false, false, -1, 16384)
	req, resp := newReadReqResp(ctx, s, state)

	r := &roleResource{db: db}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed (null) when role not found")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestRoleResource_Read_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT oid").WillReturnError(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleStateValue(ctx, s, "testrole", false, false, false, false, false, -1, 16384)
	req, resp := newReadReqResp(ctx, s, state)

	r := &roleResource{db: db}
	r.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error reading role" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error reading role' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestRoleResource_Update_renameError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("ALTER ROLE.*RENAME TO").WillReturnError(fmt.Errorf("role exists"))

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValue(ctx, s, "newname", false, false, false, false, false, -1)
	state := roleStateValue(ctx, s, "oldname", false, false, false, false, false, -1, 16384)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &roleResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error renaming role" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error renaming role' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestRoleResource_Update_alterError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Same name, so no rename. ALTER ROLE fails.
	mock.ExpectExec("ALTER ROLE").WillReturnError(fmt.Errorf("insufficient privilege"))

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValue(ctx, s, "testrole", true, false, false, false, false, -1)
	state := roleStateValue(ctx, s, "testrole", false, false, false, false, false, -1, 16384)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &roleResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error updating role" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error updating role' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestRoleResource_Update_grantError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// ALTER ROLE succeeds, then GRANT new membership fails
	mock.ExpectExec("ALTER ROLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("GRANT").WillReturnError(fmt.Errorf("role admin does not exist"))

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValueWithRoles(ctx, s, "testrole", false, false, false, false, false, -1, []string{"admin"})
	state := roleStateValue(ctx, s, "testrole", false, false, false, false, false, -1, 16384)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &roleResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error granting role membership" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error granting role membership' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestRoleResource_Update_revokeError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// ALTER ROLE succeeds, then REVOKE membership fails
	mock.ExpectExec("ALTER ROLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("REVOKE").WillReturnError(fmt.Errorf("cannot revoke"))

	ctx := context.Background()
	s := roleResourceSchema()
	// Plan has no roles (removing "admin"), state has "admin"
	plan := rolePlanValue(ctx, s, "testrole", false, false, false, false, false, -1)
	state := roleStateValueWithRoles(ctx, s, "testrole", false, false, false, false, false, -1, 16384, []string{"admin"})
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &roleResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error revoking role membership" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error revoking role membership' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestRoleResource_Delete_dropError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("DROP ROLE").WillReturnError(fmt.Errorf("role has dependent objects"))

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleStateValue(ctx, s, "testrole", false, false, false, false, false, -1, 16384)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &roleResource{db: db}
	r.Delete(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error deleting role" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error deleting role' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

// ---------------------------------------------------------------------------
// schemaResource helpers
// ---------------------------------------------------------------------------

func schemaResourceSchema() rschema.Schema {
	r := &schemaResource{}
	sresp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func schemaPlanValue(ctx context.Context, s rschema.Schema, name string, database interface{}, owner interface{}, ifNotExists bool) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var dbVal, ownerVal tftypes.Value
	if database == nil {
		dbVal = tftypes.NewValue(tftypes.String, nil)
	} else {
		dbVal = tftypes.NewValue(tftypes.String, database)
	}
	if owner == nil {
		ownerVal = tftypes.NewValue(tftypes.String, nil)
	} else {
		ownerVal = tftypes.NewValue(tftypes.String, owner)
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":          tftypes.NewValue(tftypes.String, name),
		"database":      dbVal,
		"owner":         ownerVal,
		"if_not_exists": tftypes.NewValue(tftypes.Bool, ifNotExists),
		"timeouts":      tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func schemaStateValue(ctx context.Context, s rschema.Schema, name, database, owner string, ifNotExists bool) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":          tftypes.NewValue(tftypes.String, name),
		"database":      tftypes.NewValue(tftypes.String, database),
		"owner":         tftypes.NewValue(tftypes.String, owner),
		"if_not_exists": tftypes.NewValue(tftypes.Bool, ifNotExists),
		"timeouts":      tftypes.NewValue(timeoutsObjectType, nil),
	})
}

// ---------------------------------------------------------------------------
// schemaResource tests
// ---------------------------------------------------------------------------

func TestSchemaResource_Create_execError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE SCHEMA").WillReturnError(fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &schemaResource{db: db}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error creating schema" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error creating schema' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestSchemaResource_Create_currentDBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE SCHEMA").WillReturnResult(sqlmock.NewResult(0, 0))
	// database is null, so it queries current_database()
	mock.ExpectQuery("SELECT current_database").WillReturnError(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", nil, "postgres", false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &schemaResource{db: db}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error reading current database" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error reading current database' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestSchemaResource_Create_readOwnerError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE SCHEMA").WillReturnResult(sqlmock.NewResult(0, 0))
	// database is set, so no current_database query
	// Read owner query fails
	mock.ExpectQuery("SELECT schema_owner").WillReturnError(fmt.Errorf("schema vanished"))

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &schemaResource{db: db}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error reading schema after creation" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error reading schema after creation' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestSchemaResource_Read_notFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Empty result set triggers sql.ErrNoRows
	rows := sqlmock.NewRows([]string{"schema_owner"})
	mock.ExpectQuery("SELECT schema_owner").WillReturnRows(rows)

	ctx := context.Background()
	s := schemaResourceSchema()
	state := schemaStateValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &schemaResource{db: db}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed (null) when schema not found")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestSchemaResource_Read_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT schema_owner").WillReturnError(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := schemaResourceSchema()
	state := schemaStateValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &schemaResource{db: db}
	r.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error reading schema" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error reading schema' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestSchemaResource_Read_currentDBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Schema query succeeds
	rows := sqlmock.NewRows([]string{"schema_owner"}).AddRow("postgres")
	mock.ExpectQuery("SELECT schema_owner").WillReturnRows(rows)
	// current_database() query fails (database is null in state)
	mock.ExpectQuery("SELECT current_database").WillReturnError(fmt.Errorf("broken pipe"))

	ctx := context.Background()
	s := schemaResourceSchema()
	// Use null database in state
	tfType := s.Type().TerraformType(ctx)
	state := tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":          tftypes.NewValue(tftypes.String, "myschema"),
		"database":      tftypes.NewValue(tftypes.String, nil),
		"owner":         tftypes.NewValue(tftypes.String, "postgres"),
		"if_not_exists": tftypes.NewValue(tftypes.Bool, false),
		"timeouts":      tftypes.NewValue(timeoutsObjectType, nil),
	})
	req, resp := newReadReqResp(ctx, s, state)

	r := &schemaResource{db: db}
	r.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error reading current database" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error reading current database' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestSchemaResource_Update_renameError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("ALTER SCHEMA.*RENAME TO").WillReturnError(fmt.Errorf("schema exists"))

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "newschema", "testdb", "postgres", false)
	state := schemaStateValue(ctx, s, "oldschema", "testdb", "postgres", false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &schemaResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error renaming schema" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error renaming schema' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestSchemaResource_Update_ownerChangeError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Same name, owner changes
	mock.ExpectExec("ALTER SCHEMA.*OWNER TO").WillReturnError(fmt.Errorf("role does not exist"))

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", "testdb", "newowner", false)
	state := schemaStateValue(ctx, s, "myschema", "testdb", "oldowner", false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &schemaResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error changing schema owner" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error changing schema owner' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestSchemaResource_Update_readBackError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Same name, same owner => no ALTER needed, goes straight to read-back
	mock.ExpectQuery("SELECT schema_owner").WillReturnError(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", "testdb", "postgres", false)
	state := schemaStateValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &schemaResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error reading schema after update" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error reading schema after update' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestSchemaResource_Delete_dropError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("DROP SCHEMA").WillReturnError(fmt.Errorf("schema has dependent objects"))

	ctx := context.Background()
	s := schemaResourceSchema()
	state := schemaStateValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &schemaResource{db: db}
	r.Delete(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error deleting schema" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error deleting schema' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestSchemaResource_ImportState_withDatabase(t *testing.T) {
	ctx := context.Background()
	s := schemaResourceSchema()
	req, resp := newImportReqResp(ctx, s, "mydb/myschema")

	r := &schemaResource{}
	r.ImportState(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}

	var state schemaResourceModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("error reading state: %v", resp.Diagnostics.Errors())
	}
	if state.Database.ValueString() != "mydb" {
		t.Errorf("expected database=mydb, got %s", state.Database.ValueString())
	}
	if state.Name.ValueString() != "myschema" {
		t.Errorf("expected name=myschema, got %s", state.Name.ValueString())
	}
}

func TestSchemaResource_ImportState_schemaOnly(t *testing.T) {
	ctx := context.Background()
	s := schemaResourceSchema()
	req, resp := newImportReqResp(ctx, s, "myschema")

	r := &schemaResource{}
	r.ImportState(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}

	var state schemaResourceModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("error reading state: %v", resp.Diagnostics.Errors())
	}
	if state.Name.ValueString() != "myschema" {
		t.Errorf("expected name=myschema, got %s", state.Name.ValueString())
	}
}

// ---------------------------------------------------------------------------
// grantResource helpers
// ---------------------------------------------------------------------------

func grantResourceSchema() rschema.Schema {
	r := &grantResource{}
	sresp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func grantPlanValue(ctx context.Context, s rschema.Schema, role, database, schemaName, objectType string, privileges []string, withGrantOption bool) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var privVals []tftypes.Value
	for _, p := range privileges {
		privVals = append(privVals, tftypes.NewValue(tftypes.String, p))
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"id":                tftypes.NewValue(tftypes.String, nil),
		"role":              tftypes.NewValue(tftypes.String, role),
		"database":          tftypes.NewValue(tftypes.String, database),
		"schema":            tftypes.NewValue(tftypes.String, schemaName),
		"object_type":       tftypes.NewValue(tftypes.String, objectType),
		"objects":           tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		"privileges":        tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, privVals),
		"with_grant_option": tftypes.NewValue(tftypes.Bool, withGrantOption),
		"timeouts":          tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func grantStateValue(ctx context.Context, s rschema.Schema, id, role, database, schemaName, objectType string, privileges []string, withGrantOption bool) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var privVals []tftypes.Value
	for _, p := range privileges {
		privVals = append(privVals, tftypes.NewValue(tftypes.String, p))
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"id":                tftypes.NewValue(tftypes.String, id),
		"role":              tftypes.NewValue(tftypes.String, role),
		"database":          tftypes.NewValue(tftypes.String, database),
		"schema":            tftypes.NewValue(tftypes.String, schemaName),
		"object_type":       tftypes.NewValue(tftypes.String, objectType),
		"objects":           tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		"privileges":        tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, privVals),
		"with_grant_option": tftypes.NewValue(tftypes.Bool, withGrantOption),
		"timeouts":          tftypes.NewValue(timeoutsObjectType, nil),
	})
}

// ---------------------------------------------------------------------------
// grantResource tests
// ---------------------------------------------------------------------------

func TestGrantResource_Create_execError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("GRANT").WillReturnError(fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &grantResource{db: db}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error executing GRANT" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error executing GRANT' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestGrantResource_Read_privError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("relation does not exist"))

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole_database_testdb_", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &grantResource{db: db}
	r.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error reading privileges" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error reading privileges' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestGrantResource_Read_empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Return empty result
	rows := sqlmock.NewRows([]string{"privilege_type", "is_grantable"})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole_database_testdb_", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &grantResource{db: db}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed (null) when no privileges found")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestGrantResource_Update_revokeError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("REVOKE").WillReturnError(fmt.Errorf("cannot revoke"))

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"CREATE"}, false)
	state := grantStateValue(ctx, s, "testrole_database_testdb_", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &grantResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error executing REVOKE" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error executing REVOKE' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestGrantResource_Update_grantError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// REVOKE succeeds, GRANT fails
	mock.ExpectExec("REVOKE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("GRANT").WillReturnError(fmt.Errorf("cannot grant"))

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"CREATE"}, false)
	state := grantStateValue(ctx, s, "testrole_database_testdb_", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &grantResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error executing GRANT" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error executing GRANT' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestGrantResource_Delete_revokeError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("REVOKE").WillReturnError(fmt.Errorf("cannot revoke"))

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole_database_testdb_", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &grantResource{db: db}
	r.Delete(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error executing REVOKE" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error executing REVOKE' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestGrantResource_ImportState_invalidFormat(t *testing.T) {
	ctx := context.Background()
	s := grantResourceSchema()
	req, resp := newImportReqResp(ctx, s, "invalid")

	r := &grantResource{}
	r.ImportState(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Invalid Import ID" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Invalid Import ID' diagnostic")
	}
}

func TestGrantResource_ImportState_3parts(t *testing.T) {
	ctx := context.Background()
	s := grantResourceSchema()
	req, resp := newImportReqResp(ctx, s, "testrole/database/testdb")

	r := &grantResource{}
	r.ImportState(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}

	var state grantResourceModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("error reading state: %v", resp.Diagnostics.Errors())
	}
	if state.Role.ValueString() != "testrole" {
		t.Errorf("expected role=testrole, got %s", state.Role.ValueString())
	}
	if state.ObjectType.ValueString() != "database" {
		t.Errorf("expected object_type=database, got %s", state.ObjectType.ValueString())
	}
	if state.Database.ValueString() != "testdb" {
		t.Errorf("expected database=testdb, got %s", state.Database.ValueString())
	}
	if state.ID.ValueString() != "testrole_database_testdb_" {
		t.Errorf("expected id=testrole_database_testdb_, got %s", state.ID.ValueString())
	}
}

func TestGrantResource_ImportState_4parts(t *testing.T) {
	ctx := context.Background()
	s := grantResourceSchema()
	req, resp := newImportReqResp(ctx, s, "testrole/table/testdb/public")

	r := &grantResource{}
	r.ImportState(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}

	var state grantResourceModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("error reading state: %v", resp.Diagnostics.Errors())
	}
	if state.Role.ValueString() != "testrole" {
		t.Errorf("expected role=testrole, got %s", state.Role.ValueString())
	}
	if state.ObjectType.ValueString() != "table" {
		t.Errorf("expected object_type=table, got %s", state.ObjectType.ValueString())
	}
	if state.Database.ValueString() != "testdb" {
		t.Errorf("expected database=testdb, got %s", state.Database.ValueString())
	}
	if state.Schema.ValueString() != "public" {
		t.Errorf("expected schema=public, got %s", state.Schema.ValueString())
	}
	if state.ID.ValueString() != "testrole_table_testdb_public" {
		t.Errorf("expected id=testrole_table_testdb_public, got %s", state.ID.ValueString())
	}
}

// ---------------------------------------------------------------------------
// defaultPrivilegesResource helpers
// ---------------------------------------------------------------------------

func defaultPrivilegesResourceSchema() rschema.Schema {
	r := &defaultPrivilegesResource{}
	sresp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func defaultPrivPlanValue(ctx context.Context, s rschema.Schema, owner, role, database string, schemaName interface{}, objectType string, privileges []string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var privVals []tftypes.Value
	for _, p := range privileges {
		privVals = append(privVals, tftypes.NewValue(tftypes.String, p))
	}
	var schemaVal tftypes.Value
	if schemaName == nil {
		schemaVal = tftypes.NewValue(tftypes.String, nil)
	} else {
		schemaVal = tftypes.NewValue(tftypes.String, schemaName)
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"id":          tftypes.NewValue(tftypes.String, nil),
		"owner":       tftypes.NewValue(tftypes.String, owner),
		"role":        tftypes.NewValue(tftypes.String, role),
		"database":    tftypes.NewValue(tftypes.String, database),
		"schema":      schemaVal,
		"object_type": tftypes.NewValue(tftypes.String, objectType),
		"privileges":  tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, privVals),
		"timeouts":    tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func defaultPrivStateValue(ctx context.Context, s rschema.Schema, id, owner, role, database string, schemaName interface{}, objectType string, privileges []string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var privVals []tftypes.Value
	for _, p := range privileges {
		privVals = append(privVals, tftypes.NewValue(tftypes.String, p))
	}
	var schemaVal tftypes.Value
	if schemaName == nil {
		schemaVal = tftypes.NewValue(tftypes.String, nil)
	} else {
		schemaVal = tftypes.NewValue(tftypes.String, schemaName)
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"id":          tftypes.NewValue(tftypes.String, id),
		"owner":       tftypes.NewValue(tftypes.String, owner),
		"role":        tftypes.NewValue(tftypes.String, role),
		"database":    tftypes.NewValue(tftypes.String, database),
		"schema":      schemaVal,
		"object_type": tftypes.NewValue(tftypes.String, objectType),
		"privileges":  tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, privVals),
		"timeouts":    tftypes.NewValue(timeoutsObjectType, nil),
	})
}

// ---------------------------------------------------------------------------
// defaultPrivilegesResource tests
// ---------------------------------------------------------------------------

func TestDefaultPrivilegesResource_Create_execError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("ALTER DEFAULT PRIVILEGES").WillReturnError(fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := defaultPrivilegesResourceSchema()
	plan := defaultPrivPlanValue(ctx, s, "owner", "grantee", "testdb", "public", "table", []string{"SELECT"})
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &defaultPrivilegesResource{db: db}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error granting default privileges" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error granting default privileges' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDefaultPrivilegesResource_Read_queryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := defaultPrivilegesResourceSchema()
	state := defaultPrivStateValue(ctx, s, "owner_grantee_testdb_public_table", "owner", "grantee", "testdb", "public", "table", []string{"SELECT"})
	req, resp := newReadReqResp(ctx, s, state)

	r := &defaultPrivilegesResource{db: db}
	r.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error reading default privileges" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error reading default privileges' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDefaultPrivilegesResource_Read_scanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Return rows with wrong number of columns to trigger scan error
	rows := sqlmock.NewRows([]string{"privilege_type"}).AddRow("SELECT")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	ctx := context.Background()
	s := defaultPrivilegesResourceSchema()
	state := defaultPrivStateValue(ctx, s, "owner_grantee_testdb_public_table", "owner", "grantee", "testdb", "public", "table", []string{"SELECT"})
	req, resp := newReadReqResp(ctx, s, state)

	r := &defaultPrivilegesResource{db: db}
	r.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error scanning default privileges" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error scanning default privileges' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDefaultPrivilegesResource_Read_rowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"privilege_type", "is_grantable"}).
		AddRow("SELECT", false).
		RowError(0, fmt.Errorf("row iteration error"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	ctx := context.Background()
	s := defaultPrivilegesResourceSchema()
	state := defaultPrivStateValue(ctx, s, "owner_grantee_testdb_public_table", "owner", "grantee", "testdb", "public", "table", []string{"SELECT"})
	req, resp := newReadReqResp(ctx, s, state)

	r := &defaultPrivilegesResource{db: db}
	r.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error iterating default privileges" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error iterating default privileges' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDefaultPrivilegesResource_Read_noRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"privilege_type", "is_grantable"})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	ctx := context.Background()
	s := defaultPrivilegesResourceSchema()
	state := defaultPrivStateValue(ctx, s, "owner_grantee_testdb_public_table", "owner", "grantee", "testdb", "public", "table", []string{"SELECT"})
	req, resp := newReadReqResp(ctx, s, state)

	r := &defaultPrivilegesResource{db: db}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed (null) when no default privileges found")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDefaultPrivilegesResource_Update_revokeError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("ALTER DEFAULT PRIVILEGES").WillReturnError(fmt.Errorf("cannot revoke"))

	ctx := context.Background()
	s := defaultPrivilegesResourceSchema()
	plan := defaultPrivPlanValue(ctx, s, "owner", "grantee", "testdb", "public", "table", []string{"INSERT"})
	state := defaultPrivStateValue(ctx, s, "owner_grantee_testdb_public_table", "owner", "grantee", "testdb", "public", "table", []string{"SELECT"})
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &defaultPrivilegesResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error revoking old default privileges" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error revoking old default privileges' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDefaultPrivilegesResource_Update_grantError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// First REVOKE succeeds, then GRANT fails
	mock.ExpectExec("ALTER DEFAULT PRIVILEGES.*REVOKE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER DEFAULT PRIVILEGES.*GRANT").WillReturnError(fmt.Errorf("cannot grant"))

	ctx := context.Background()
	s := defaultPrivilegesResourceSchema()
	plan := defaultPrivPlanValue(ctx, s, "owner", "grantee", "testdb", "public", "table", []string{"INSERT"})
	state := defaultPrivStateValue(ctx, s, "owner_grantee_testdb_public_table", "owner", "grantee", "testdb", "public", "table", []string{"SELECT"})
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &defaultPrivilegesResource{db: db}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error granting new default privileges" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error granting new default privileges' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDefaultPrivilegesResource_Delete_revokeError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec("ALTER DEFAULT PRIVILEGES").WillReturnError(fmt.Errorf("cannot revoke"))

	ctx := context.Background()
	s := defaultPrivilegesResourceSchema()
	state := defaultPrivStateValue(ctx, s, "owner_grantee_testdb_public_table", "owner", "grantee", "testdb", "public", "table", []string{"SELECT"})
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &defaultPrivilegesResource{db: db}
	r.Delete(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error revoking default privileges" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error revoking default privileges' diagnostic")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %s", err)
	}
}

func TestDefaultPrivilegesResource_ImportState_invalidFormat(t *testing.T) {
	ctx := context.Background()
	s := defaultPrivilegesResourceSchema()
	req, resp := newImportReqResp(ctx, s, "only/two/parts")

	r := &defaultPrivilegesResource{}
	r.ImportState(ctx, req, resp)

	// 3 parts is too few (minimum is 4)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Invalid Import ID" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Invalid Import ID' diagnostic")
	}
}

func TestDefaultPrivilegesResource_ImportState_4parts(t *testing.T) {
	ctx := context.Background()
	s := defaultPrivilegesResourceSchema()
	req, resp := newImportReqResp(ctx, s, "owner/grantee/testdb/table")

	r := &defaultPrivilegesResource{}
	r.ImportState(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}

	var state defaultPrivilegesResourceModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("error reading state: %v", resp.Diagnostics.Errors())
	}
	if state.Owner.ValueString() != "owner" {
		t.Errorf("expected owner=owner, got %s", state.Owner.ValueString())
	}
	if state.Role.ValueString() != "grantee" {
		t.Errorf("expected role=grantee, got %s", state.Role.ValueString())
	}
	if state.Database.ValueString() != "testdb" {
		t.Errorf("expected database=testdb, got %s", state.Database.ValueString())
	}
	if state.ObjectType.ValueString() != "table" {
		t.Errorf("expected object_type=table, got %s", state.ObjectType.ValueString())
	}
}

func TestDefaultPrivilegesResource_ImportState_5parts(t *testing.T) {
	ctx := context.Background()
	s := defaultPrivilegesResourceSchema()
	req, resp := newImportReqResp(ctx, s, "owner/grantee/testdb/public/table")

	r := &defaultPrivilegesResource{}
	r.ImportState(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}

	var state defaultPrivilegesResourceModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("error reading state: %v", resp.Diagnostics.Errors())
	}
	if state.Owner.ValueString() != "owner" {
		t.Errorf("expected owner=owner, got %s", state.Owner.ValueString())
	}
	if state.Role.ValueString() != "grantee" {
		t.Errorf("expected role=grantee, got %s", state.Role.ValueString())
	}
	if state.Database.ValueString() != "testdb" {
		t.Errorf("expected database=testdb, got %s", state.Database.ValueString())
	}
	if state.Schema.ValueString() != "public" {
		t.Errorf("expected schema=public, got %s", state.Schema.ValueString())
	}
	if state.ObjectType.ValueString() != "table" {
		t.Errorf("expected object_type=table, got %s", state.ObjectType.ValueString())
	}
}
