package resource_test

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/resource"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/mocks"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/lib/pq"
	"go.uber.org/mock/gomock"
)

// mockResult implements sql.Result for ExecContext returns.
type mockResult struct{}

func (r mockResult) LastInsertId() (int64, error) { return 0, nil }
func (r mockResult) RowsAffected() (int64, error) { return 0, nil }

// ---------------------------------------------------------------------------
// Helpers to build request/response objects
// ---------------------------------------------------------------------------

var timeoutsObjectType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"create": tftypes.String,
	"read":   tftypes.String,
	"update": tftypes.String,
	"delete": tftypes.String,
}}

func newCreateReqResp(ctx context.Context, s rschema.Schema, planVal tftypes.Value) (fwresource.CreateRequest, *fwresource.CreateResponse) {
	tfType := s.Type().TerraformType(ctx)
	req := fwresource.CreateRequest{
		Plan: tfsdk.Plan{Raw: planVal, Schema: s},
	}
	resp := &fwresource.CreateResponse{
		State: tfsdk.State{Raw: tftypes.NewValue(tfType, nil), Schema: s},
	}
	return req, resp
}

func newReadReqResp(ctx context.Context, s rschema.Schema, stateVal tftypes.Value) (fwresource.ReadRequest, *fwresource.ReadResponse) {
	req := fwresource.ReadRequest{
		State: tfsdk.State{Raw: stateVal, Schema: s},
	}
	resp := &fwresource.ReadResponse{
		State: tfsdk.State{Raw: stateVal, Schema: s},
	}
	return req, resp
}

func newUpdateReqResp(ctx context.Context, s rschema.Schema, planVal, stateVal tftypes.Value) (fwresource.UpdateRequest, *fwresource.UpdateResponse) {
	tfType := s.Type().TerraformType(ctx)
	req := fwresource.UpdateRequest{
		Plan:  tfsdk.Plan{Raw: planVal, Schema: s},
		State: tfsdk.State{Raw: stateVal, Schema: s},
	}
	resp := &fwresource.UpdateResponse{
		State: tfsdk.State{Raw: tftypes.NewValue(tfType, nil), Schema: s},
	}
	return req, resp
}

func newDeleteReqResp(_ context.Context, s rschema.Schema, stateVal tftypes.Value) (fwresource.DeleteRequest, *fwresource.DeleteResponse) {
	req := fwresource.DeleteRequest{
		State: tfsdk.State{Raw: stateVal, Schema: s},
	}
	resp := &fwresource.DeleteResponse{
		State: tfsdk.State{Raw: stateVal, Schema: s},
	}
	return req, resp
}

func newImportReqResp(ctx context.Context, s rschema.Schema, id string) (fwresource.ImportStateRequest, *fwresource.ImportStateResponse) {
	tfType := s.Type().TerraformType(ctx)
	req := fwresource.ImportStateRequest{ID: id}
	resp := &fwresource.ImportStateResponse{
		State: tfsdk.State{Raw: tftypes.NewValue(tfType, nil), Schema: s},
	}
	return req, resp
}

// ---------------------------------------------------------------------------
// databaseResource helpers
// ---------------------------------------------------------------------------

func databaseResourceSchema() rschema.Schema {
	r := &resource.DatabaseResource{}
	sresp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, sresp)
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
		"force_drop":        tftypes.NewValue(tftypes.Bool, false),
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
		"force_drop":        tftypes.NewValue(tftypes.Bool, false),
		"oid":               tftypes.NewValue(tftypes.Number, oid),
		"timeouts":          tftypes.NewValue(timeoutsObjectType, nil),
	})
}

// ---------------------------------------------------------------------------
// databaseResource tests
// ---------------------------------------------------------------------------

func TestDatabaseResource_Create_execError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, true, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.DatabaseResource{DB: mockDB}
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
}

func TestDatabaseResource_Create_readError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil)
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, true, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.DatabaseResource{DB: mockDB}
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
}

func TestDatabaseResource_Read_notFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sql.ErrNoRows)

	ctx := context.Background()
	s := databaseResourceSchema()
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.DatabaseResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed (null) when database not found")
	}
}

func TestDatabaseResource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection refused"))

	ctx := context.Background()
	s := databaseResourceSchema()
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.DatabaseResource{DB: mockDB}
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
}

func TestDatabaseResource_Update_ownerChangeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("role does not exist"))

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "newowner", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, true, false)
	state := databaseStateValue(ctx, s, "testdb", "oldowner", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.DatabaseResource{DB: mockDB}
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
}

func TestDatabaseResource_Update_tablespaceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// Owner is the same, so no owner ALTER. Tablespace change triggers error.
	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("tablespace not found"))

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "fast_ssd", -1, true, false)
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.DatabaseResource{DB: mockDB}
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
}

func TestDatabaseResource_Update_withOptsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// Owner same, tablespace same, connection_limit changes -> ALTER DATABASE ... WITH
	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("syntax error"))

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", 10, true, false)
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.DatabaseResource{DB: mockDB}
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
}

func TestDatabaseResource_Delete_templateUnsetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// is_template=true => ALTER DATABASE ... IS_TEMPLATE = false first
	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot alter"))

	ctx := context.Background()
	s := databaseResourceSchema()
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, true)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.DatabaseResource{DB: mockDB}
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
}

func TestDatabaseResource_Delete_dropError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// is_template=false so no ALTER needed, go straight to DROP
	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("database is being accessed"))

	ctx := context.Background()
	s := databaseResourceSchema()
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "en_US.UTF-8", "en_US.UTF-8", "pg_default", -1, 12345, true, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.DatabaseResource{DB: mockDB}
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
}

// ---------------------------------------------------------------------------
// roleResource helpers
// ---------------------------------------------------------------------------

func roleResourceSchema() rschema.Schema {
	r := &resource.RoleResource{}
	sresp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, sresp)
	return sresp.Schema
}

var privilegeObjectType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"privileges":  tftypes.Set{ElementType: tftypes.String},
	"object_type": tftypes.String,
	"schema":      tftypes.String,
	"database":    tftypes.String,
	"objects":     tftypes.List{ElementType: tftypes.String},
}}

var privilegeListType = tftypes.List{ElementType: privilegeObjectType}

func roleStateValue(ctx context.Context, s rschema.Schema, name string, superuser, createDB, createRole, replication bool, connLimit, oid int64) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":              tftypes.NewValue(tftypes.String, name),
		"superuser":         tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":   tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":       tftypes.NewValue(tftypes.Bool, createRole),
		"replication":       tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit":  tftypes.NewValue(tftypes.Number, connLimit),
		"privilege":         tftypes.NewValue(privilegeListType, []tftypes.Value{}),
		"reassign_owned_to": tftypes.NewValue(tftypes.String, nil),
		"oid":               tftypes.NewValue(tftypes.Number, oid),
		"timeouts":          tftypes.NewValue(timeoutsObjectType, nil),
	})
}

// ---------------------------------------------------------------------------
// roleResource tests
// ---------------------------------------------------------------------------

// switchingDB is a mock connection that hands out other mocks for other databases.
type switchingDB struct {
	*mocks.MockDBTX
	others map[string]common.DBTX
	err    error
}

func (s *switchingDB) ForDatabase(_ context.Context, database string) (common.DBTX, error) {
	if s.err != nil {
		return nil, s.err
	}
	if db, ok := s.others[database]; ok {
		return db, nil
	}
	return s, nil
}

func (s *switchingDB) ReleaseDatabase(string) error { return nil }

func rolePrivilegeBlock(objectType, database, schemaName string, privs []string) tftypes.Value {
	var privVals []tftypes.Value
	for _, p := range privs {
		privVals = append(privVals, tftypes.NewValue(tftypes.String, p))
	}
	var db, sch any
	if database != "" {
		db = database
	}
	if schemaName != "" {
		sch = schemaName
	}
	return tftypes.NewValue(privilegeObjectType, map[string]tftypes.Value{
		"privileges":  tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, privVals),
		"object_type": tftypes.NewValue(tftypes.String, objectType),
		"schema":      tftypes.NewValue(tftypes.String, sch),
		"database":    tftypes.NewValue(tftypes.String, db),
		"objects":     tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
	})
}

func roleValue(ctx context.Context, s rschema.Schema, name string, superuser bool, oid, reassignTo any, blocks ...tftypes.Value) tftypes.Value {
	if blocks == nil {
		blocks = []tftypes.Value{}
	}
	return tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
		"name":              tftypes.NewValue(tftypes.String, name),
		"superuser":         tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":   tftypes.NewValue(tftypes.Bool, false),
		"create_role":       tftypes.NewValue(tftypes.Bool, false),
		"replication":       tftypes.NewValue(tftypes.Bool, false),
		"connection_limit":  tftypes.NewValue(tftypes.Number, -1),
		"privilege":         tftypes.NewValue(privilegeListType, blocks),
		"reassign_owned_to": tftypes.NewValue(tftypes.String, reassignTo),
		"oid":               tftypes.NewValue(tftypes.Number, oid),
		"timeouts":          tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func expectReadRole(ctrl *gomock.Controller, db *mocks.MockDBTX, name string, oid int64) {
	scanner := mocks.NewMockScanner(ctrl)
	db.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), name).Return(scanner)
	scanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = oid
		*dest[6].(*int64) = -1
		return nil
	})
}

// expectTx expects one transaction running statements in order; failAt is the index that fails, -1 for none, len for a commit error.
func expectTx(ctrl *gomock.Controller, db *mocks.MockDBTX, statements []string, failAt int) {
	tx := mocks.NewMockTx(ctrl)
	db.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(tx, nil)
	var calls []any
	for i, stmt := range statements {
		if i == failAt {
			calls = append(calls, tx.EXPECT().ExecContext(gomock.Any(), stmt).Return(nil, fmt.Errorf("exec failed")))
			break
		}
		calls = append(calls, tx.EXPECT().ExecContext(gomock.Any(), stmt).Return(mockResult{}, nil))
	}
	switch {
	case failAt == len(statements):
		calls = append(calls, tx.EXPECT().Commit().Return(fmt.Errorf("commit failed")), tx.EXPECT().Rollback().Return(nil))
	case failAt >= 0:
		calls = append(calls, tx.EXPECT().Rollback().Return(nil))
	default:
		calls = append(calls, tx.EXPECT().Commit().Return(nil), tx.EXPECT().Rollback().Return(sql.ErrTxDone))
	}
	gomock.InOrder(calls...)
}

const (
	createTestRole = `CREATE ROLE "testrole" WITH NOLOGIN CONNECTION LIMIT -1`
	grantTestRole  = `GRANT SELECT ON ALL TABLES IN SCHEMA "public" TO "testrole"`
	revokeTestRole = `REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA "public" FROM "testrole"`
)

func TestRoleResource_Create_txErrors(t *testing.T) {
	tests := []struct {
		name   string
		failAt int
	}{
		{"create fails", 0},
		{"grant fails", 1},
		{"commit fails", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mocks.NewMockDBTX(ctrl)
			expectTx(ctrl, mockDB, []string{createTestRole, grantTestRole}, tc.failAt)

			ctx := context.Background()
			s := roleResourceSchema()
			plan := roleValue(ctx, s, "testrole", false, nil, nil, rolePrivilegeBlock("table", "", "public", []string{"select"}))
			req, resp := newCreateReqResp(ctx, s, plan)

			r := &resource.RoleResource{DB: mockDB}
			r.Create(ctx, req, resp)

			if !hasDiag(resp.Diagnostics, "Error creating role") {
				t.Fatalf("expected 'Error creating role', got %v", resp.Diagnostics.Errors())
			}
			if !resp.State.Raw.IsNull() {
				t.Error("expected no state when the transaction fails")
			}
		})
	}
}

func TestRoleResource_Create_beginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot begin"))

	ctx := context.Background()
	s := roleResourceSchema()
	plan := roleValue(ctx, s, "testrole", false, nil, nil)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.RoleResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error creating role") {
		t.Fatalf("expected 'Error creating role', got %v", resp.Diagnostics.Errors())
	}
}

func TestRoleResource_Create_invalidPrivilege(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	// No expectations: nothing runs before the privileges are validated.

	ctx := context.Background()
	s := roleResourceSchema()
	plan := roleValue(ctx, s, "testrole", false, nil, nil, rolePrivilegeBlock("table", "", "public", []string{"DROP"}))
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.RoleResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error preparing privileges") {
		t.Fatalf("expected 'Error preparing privileges', got %v", resp.Diagnostics.Errors())
	}
}

func TestRoleResource_Create_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	expectTx(ctrl, mockDB, []string{`CREATE ROLE "testrole" WITH NOLOGIN SUPERUSER CONNECTION LIMIT -1`, grantTestRole}, -1)
	expectReadRole(ctrl, mockDB, "testrole", 16384)

	ctx := context.Background()
	s := roleResourceSchema()
	plan := roleValue(ctx, s, "testrole", true, nil, nil, rolePrivilegeBlock("table", "", "public", []string{"SELECT"}))
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.RoleResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.RoleResourceModel
	resp.State.Get(ctx, &got)
	if got.OID.ValueInt64() != 16384 {
		t.Errorf("expected oid 16384, got %d", got.OID.ValueInt64())
	}
}

func TestRoleResource_Create_otherDatabaseGrantFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	mainDB := mocks.NewMockDBTX(ctrl)
	otherDB := mocks.NewMockDBTX(ctrl)
	db := &switchingDB{MockDBTX: mainDB, others: map[string]common.DBTX{"app": otherDB}}

	expectTx(ctrl, mainDB, []string{createTestRole}, -1)
	expectReadRole(ctrl, mainDB, "testrole", 16384)
	expectTx(ctrl, otherDB, []string{`GRANT USAGE ON SCHEMA "public" TO "testrole"`}, 0)

	ctx := context.Background()
	s := roleResourceSchema()
	plan := roleValue(ctx, s, "testrole", false, nil, nil, rolePrivilegeBlock("schema", "app", "public", []string{"USAGE"}))
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.RoleResource{DB: db}
	r.Create(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error granting privileges") {
		t.Fatalf("expected 'Error granting privileges', got %v", resp.Diagnostics.Errors())
	}
	// The role exists, so it stays tracked even though a grant failed.
	if resp.State.Raw.IsNull() {
		t.Error("expected the created role to be kept in state")
	}
}

func TestRoleResource_Read_notFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sql.ErrNoRows)

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleStateValue(ctx, s, "testrole", false, false, false, false, -1, 16384)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.RoleResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed (null) when role not found")
	}
}

func TestRoleResource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleStateValue(ctx, s, "testrole", false, false, false, false, -1, 16384)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.RoleResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error reading role") {
		t.Fatalf("expected 'Error reading role', got %v", resp.Diagnostics.Errors())
	}
}

func TestRoleResource_Read_privilegeDrift(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	expectReadRole(ctrl, mockDB, "testrole", 16384)
	gomock.InOrder(
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "testrole").Return(aclRowsMock(ctrl, aclRow{"public", "USAGE", false}, aclRow{"public", "CREATE", false}), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "n").Return(stringRowsMock(ctrl, "USAGE", "CREATE"), nil),
	)

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleValue(ctx, s, "testrole", false, 16384, nil, rolePrivilegeBlock("schema", "", "public", []string{"USAGE"}))
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.RoleResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.RoleResourceModel
	resp.State.Get(ctx, &got)
	privs := common.StringSetToSlice(ctx, got.Privileges[0].Privileges)
	slices.Sort(privs)
	if !slices.Equal(privs, []string{"CREATE", "USAGE"}) {
		t.Errorf("expected drift to [CREATE USAGE], got %v", privs)
	}
}

func TestRoleResource_Read_privilegeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	expectReadRole(ctrl, mockDB, "testrole", 16384)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "testrole").Return(nil, fmt.Errorf("boom"))

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleValue(ctx, s, "testrole", false, 16384, nil, rolePrivilegeBlock("schema", "", "public", []string{"USAGE"}))
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.RoleResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error reading privileges") {
		t.Fatalf("expected 'Error reading privileges', got %v", resp.Diagnostics.Errors())
	}
}

func TestRoleResource_Update_txErrors(t *testing.T) {
	statements := []string{
		`ALTER ROLE "oldrole" RENAME TO "testrole"`,
		`ALTER ROLE "testrole" WITH SUPERUSER CONNECTION LIMIT -1`,
		revokeTestRole,
		grantTestRole,
	}
	for failAt, name := range []string{"rename fails", "alter fails", "revoke fails", "grant fails", "commit fails"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mocks.NewMockDBTX(ctrl)
			expectTx(ctrl, mockDB, statements, failAt)

			ctx := context.Background()
			s := roleResourceSchema()
			block := rolePrivilegeBlock("table", "", "public", []string{"SELECT"})
			plan := roleValue(ctx, s, "testrole", true, 16384, nil, block)
			state := roleValue(ctx, s, "oldrole", false, 16384, nil, block)
			req, resp := newUpdateReqResp(ctx, s, plan, state)

			r := &resource.RoleResource{DB: mockDB}
			r.Update(ctx, req, resp)

			if !hasDiag(resp.Diagnostics, "Error updating role") {
				t.Fatalf("expected 'Error updating role', got %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestRoleResource_Update_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	expectTx(ctrl, mockDB, []string{`ALTER ROLE "testrole" WITH CONNECTION LIMIT -1`, revokeTestRole, grantTestRole}, -1)
	expectReadRole(ctrl, mockDB, "testrole", 16384)

	ctx := context.Background()
	s := roleResourceSchema()
	block := rolePrivilegeBlock("table", "", "public", []string{"SELECT"})
	plan := roleValue(ctx, s, "testrole", false, 16384, nil, block)
	req, resp := newUpdateReqResp(ctx, s, plan, plan)

	r := &resource.RoleResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestRoleResource_Update_otherDatabaseFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	mainDB := mocks.NewMockDBTX(ctrl)
	otherDB := mocks.NewMockDBTX(ctrl)
	db := &switchingDB{MockDBTX: mainDB, others: map[string]common.DBTX{"app": otherDB}}

	expectTx(ctrl, mainDB, []string{`ALTER ROLE "testrole" WITH CONNECTION LIMIT -1`}, -1)
	expectTx(ctrl, otherDB, []string{`REVOKE ALL PRIVILEGES ON SCHEMA "public" FROM "testrole"`}, 0)

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleValue(ctx, s, "testrole", false, 16384, nil, rolePrivilegeBlock("schema", "app", "public", []string{"USAGE"}))
	plan := roleValue(ctx, s, "testrole", false, 16384, nil)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.RoleResource{DB: db}
	r.Update(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error updating privileges") {
		t.Fatalf("expected 'Error updating privileges', got %v", resp.Diagnostics.Errors())
	}
}

func TestRoleResource_Update_invalidPrivilege(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	// No expectations: the REVOKE must not run before the GRANT is validated.

	ctx := context.Background()
	s := roleResourceSchema()
	plan := roleValue(ctx, s, "testrole", false, 16384, nil, rolePrivilegeBlock("table", "", "public", []string{"FOO"}))
	state := roleValue(ctx, s, "testrole", false, 16384, nil, rolePrivilegeBlock("table", "", "public", []string{"SELECT"}))
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.RoleResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error preparing privileges") {
		t.Fatalf("expected 'Error preparing privileges', got %v", resp.Diagnostics.Errors())
	}
}

func TestRoleResource_Delete_dropError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	tx := mocks.NewMockTx(ctrl)
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(tx, nil)
	tx.EXPECT().ExecContext(gomock.Any(), `DROP ROLE "testrole"`).Return(nil, &pq.Error{Code: "2BP01", Message: "role cannot be dropped"})
	tx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleValue(ctx, s, "testrole", false, 16384, nil)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.RoleResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error deleting role") {
		t.Fatalf("expected 'Error deleting role', got %v", resp.Diagnostics.Errors())
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "reassign_owned_to") {
		t.Errorf("expected the reassign_owned_to hint, got %q", resp.Diagnostics.Errors()[0].Detail())
	}
}

func TestRoleResource_Delete_reassignsAndRevokes(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	// A revoke on a dropped schema is skipped.
	mockDB.EXPECT().ExecContext(gomock.Any(), revokeTestRole).Return(nil, &pq.Error{Code: "3F000"})
	expectTx(ctrl, mockDB, []string{`REASSIGN OWNED BY "testrole" TO "postgres"`, `DROP OWNED BY "testrole"`, `DROP ROLE "testrole"`}, -1)

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleValue(ctx, s, "testrole", false, 16384, "postgres", rolePrivilegeBlock("table", "", "public", []string{"SELECT"}))
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.RoleResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestRoleResource_Delete_revokeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().ExecContext(gomock.Any(), revokeTestRole).Return(nil, &pq.Error{Code: "42501", Message: "permission denied"})

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleValue(ctx, s, "testrole", false, 16384, nil, rolePrivilegeBlock("table", "", "public", []string{"SELECT"}))
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.RoleResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error revoking privilege") {
		t.Fatalf("expected 'Error revoking privilege', got %v", resp.Diagnostics.Errors())
	}
}

func TestRoleResource_Delete_prepareError(t *testing.T) {
	db := &switchingDB{err: fmt.Errorf("refused")}

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleValue(ctx, s, "testrole", false, 16384, nil, rolePrivilegeBlock("schema", "app", "public", []string{"USAGE"}))
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.RoleResource{DB: db}
	r.Delete(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error preparing privileges") {
		t.Fatalf("expected 'Error preparing privileges', got %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// PrivilegeStatements / ReadPrivilegeBlocks tests
// ---------------------------------------------------------------------------

func privilegeModel(objectType, database, schemaName string, objects []string, privs ...string) resource.PrivilegeModel {
	m := resource.PrivilegeModel{
		ObjectType: types.StringValue(objectType),
		Database:   types.StringNull(),
		Schema:     types.StringNull(),
		Objects:    types.ListNull(types.StringType),
	}
	if database != "" {
		m.Database = types.StringValue(database)
	}
	if schemaName != "" {
		m.Schema = types.StringValue(schemaName)
	}
	if objects != nil {
		m.Objects, _ = types.ListValueFrom(context.Background(), types.StringType, objects)
	}
	m.Privileges, _ = types.SetValueFrom(context.Background(), types.StringType, privs)
	return m
}

func TestPrivilegeStatements_GrantAndRevoke(t *testing.T) {
	ctrl := gomock.NewController(t)
	mainDB := mocks.NewMockDBTX(ctrl)
	otherDB := mocks.NewMockDBTX(ctrl)
	db := &switchingDB{MockDBTX: mainDB, others: map[string]common.DBTX{"app": otherDB}}
	r := &resource.RoleResource{DB: db}
	blocks := []resource.PrivilegeModel{
		privilegeModel("database", "app", "", nil, "connect"),
		privilegeModel("table", "", "public", []string{"t1"}, "select", "insert"),
		privilegeModel("schema", "app", "public", nil, "usage"),
	}

	grants, err := r.PrivilegeStatements(context.Background(), nil, "r", blocks, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Database grants run on the provider's connection; the schema grant in app on app's.
	if len(grants) != 2 || grants[0].Conn != db || grants[1].Conn != otherDB {
		t.Fatalf("expected two groups (provider, app), got %+v", grants)
	}
	if !slices.Equal(grants[0].Statements, []string{`GRANT CONNECT ON DATABASE "app" TO "r"`, `GRANT SELECT, INSERT ON TABLE "public"."t1" TO "r"`}) {
		t.Errorf("unexpected provider statements %v", grants[0].Statements)
	}
	if !slices.Equal(grants[1].Statements, []string{`GRANT USAGE ON SCHEMA "public" TO "r"`}) {
		t.Errorf("unexpected app statements %v", grants[1].Statements)
	}

	all, err := r.PrivilegeStatements(context.Background(), grants, "r", blocks[:1], false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 2 || all[0].Statements[2] != `REVOKE ALL PRIVILEGES ON DATABASE "app" FROM "r"` {
		t.Errorf("expected the revoke appended to the provider's group, got %+v", all)
	}
}

func TestPrivilegeStatements_InvalidPrivilege(t *testing.T) {
	r := &resource.RoleResource{DB: mocks.NewMockDBTX(gomock.NewController(t))}
	_, err := r.PrivilegeStatements(context.Background(), nil, "r", []resource.PrivilegeModel{privilegeModel("schema", "", "public", nil, "DROP")}, true)
	if err == nil || !strings.Contains(err.Error(), "invalid privilege") {
		t.Fatalf("expected an invalid privilege error, got %v", err)
	}
}

func TestPrivilegeStatements_Connections(t *testing.T) {
	block := []resource.PrivilegeModel{privilegeModel("schema", "gone", "public", nil, "USAGE")}

	gone := &resource.RoleResource{DB: &switchingDB{err: &pq.Error{Code: "3D000"}}}
	groups, err := gone.PrivilegeStatements(context.Background(), nil, "r", block, false)
	if err != nil || len(groups) != 0 {
		t.Errorf("expected a revoke in a dropped database to be skipped, got %v, %v", groups, err)
	}
	if _, err := gone.PrivilegeStatements(context.Background(), nil, "r", block, true); err == nil {
		t.Error("expected a grant in a dropped database to fail")
	}
}

func TestPrivilegeStatements_Functions(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	r := &resource.RoleResource{DB: mockDB}
	block := []resource.PrivilegeModel{privilegeModel("function", "", "public", []string{"f"}, "EXECUTE")}

	gomock.InOrder(
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "").Return(stringRowsMock(ctrl, "public.f(integer)"), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "").Return(stringRowsMock(ctrl), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "").Return(nil, fmt.Errorf("boom")),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "").Return(nil, fmt.Errorf("boom")),
	)

	groups, err := r.PrivilegeStatements(context.Background(), nil, "r", block, true)
	if err != nil || !slices.Equal(groups[0].Statements, []string{`GRANT EXECUTE ON ROUTINE public.f(integer) TO "r"`}) {
		t.Errorf("expected the grant on the resolved signature, got %+v, %v", groups, err)
	}
	groups, err = r.PrivilegeStatements(context.Background(), nil, "r", block, false)
	if err != nil || len(groups) != 0 {
		t.Errorf("expected no revoke once the function is gone, got %+v, %v", groups, err)
	}
	if _, err := r.PrivilegeStatements(context.Background(), nil, "r", block, false); err == nil {
		t.Error("expected the resolve error on revoke")
	}
	if _, err := r.PrivilegeStatements(context.Background(), nil, "r", block, true); err == nil {
		t.Error("expected the resolve error on grant")
	}
}

func TestReadPrivilegeBlocks(t *testing.T) {
	ctx := context.Background()
	t.Run("keep matching spelling", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		gomock.InOrder(
			mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "r").Return(aclRowsMock(ctrl, aclRow{"public", "USAGE", false}, aclRow{"public", "CREATE", false}), nil),
			mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "n").Return(stringRowsMock(ctrl, "USAGE", "CREATE"), nil),
		)
		blocks := []resource.PrivilegeModel{privilegeModel("schema", "", "public", nil, "all")}
		r := &resource.RoleResource{DB: mockDB}
		if diags := r.ReadPrivilegeBlocks(ctx, "r", blocks); diags.HasError() {
			t.Fatalf("unexpected error: %v", diags)
		}
		if got := common.StringSetToSlice(ctx, blocks[0].Privileges); !slices.Equal(got, []string{"all"}) {
			t.Errorf("expected [all] kept, got %v", got)
		}
	})
	t.Run("replace on drift", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		gomock.InOrder(
			mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "r").Return(aclRowsMock(ctrl, aclRow{"public", "USAGE", false}), nil),
			mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "n").Return(stringRowsMock(ctrl, "USAGE", "CREATE"), nil),
		)
		blocks := []resource.PrivilegeModel{privilegeModel("schema", "", "public", nil, "ALL")}
		r := &resource.RoleResource{DB: mockDB}
		if diags := r.ReadPrivilegeBlocks(ctx, "r", blocks); diags.HasError() {
			t.Fatalf("unexpected error: %v", diags)
		}
		if got := common.StringSetToSlice(ctx, blocks[0].Privileges); !slices.Equal(got, []string{"USAGE"}) {
			t.Errorf("expected [USAGE], got %v", got)
		}
	})
	t.Run("empty schema kept", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		expectSchemaExists(ctrl, mockDB, "public", true)
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "r", gomock.Any()).Return(aclRowsMock(ctrl), nil)
		blocks := []resource.PrivilegeModel{privilegeModel("table", "", "public", nil, "SELECT")}
		r := &resource.RoleResource{DB: mockDB}
		if diags := r.ReadPrivilegeBlocks(ctx, "r", blocks); diags.HasError() {
			t.Fatalf("unexpected error: %v", diags)
		}
		if got := common.StringSetToSlice(ctx, blocks[0].Privileges); !slices.Equal(got, []string{"SELECT"}) {
			t.Errorf("expected [SELECT] kept, got %v", got)
		}
	})
	t.Run("target gone empties the block", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		expectSchemaExists(ctrl, mockDB, "gone", false)
		blocks := []resource.PrivilegeModel{privilegeModel("table", "", "gone", nil, "SELECT")}
		r := &resource.RoleResource{DB: mockDB}
		if diags := r.ReadPrivilegeBlocks(ctx, "r", blocks); diags.HasError() {
			t.Fatalf("unexpected error: %v", diags)
		}
		if got := common.StringSetToSlice(ctx, blocks[0].Privileges); len(got) != 0 || blocks[0].Privileges.IsNull() {
			t.Errorf("expected an empty set, got %v", blocks[0].Privileges)
		}
	})
	t.Run("database gone empties the block", func(t *testing.T) {
		blocks := []resource.PrivilegeModel{privilegeModel("schema", "gone", "public", nil, "USAGE")}
		r := &resource.RoleResource{DB: &switchingDB{err: &pq.Error{Code: "3D000"}}}
		if diags := r.ReadPrivilegeBlocks(ctx, "r", blocks); diags.HasError() {
			t.Fatalf("unexpected error: %v", diags)
		}
		if got := common.StringSetToSlice(ctx, blocks[0].Privileges); len(got) != 0 {
			t.Errorf("expected an empty set, got %v", got)
		}
	})
	t.Run("connect error", func(t *testing.T) {
		blocks := []resource.PrivilegeModel{privilegeModel("schema", "app", "public", nil, "USAGE")}
		r := &resource.RoleResource{DB: &switchingDB{err: fmt.Errorf("refused")}}
		if diags := r.ReadPrivilegeBlocks(ctx, "r", blocks); !hasDiag(diags, "Error connecting to database") {
			t.Errorf("expected 'Error connecting to database', got %v", diags)
		}
	})
	t.Run("full privileges error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		gomock.InOrder(
			mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "r").Return(aclRowsMock(ctrl, aclRow{"public", "USAGE", false}), nil),
			mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "n").Return(nil, fmt.Errorf("boom")),
		)
		blocks := []resource.PrivilegeModel{privilegeModel("schema", "", "public", nil, "USAGE")}
		r := &resource.RoleResource{DB: mockDB}
		if diags := r.ReadPrivilegeBlocks(ctx, "r", blocks); !hasDiag(diags, "Error reading privileges") {
			t.Errorf("expected 'Error reading privileges', got %v", diags)
		}
	})
}

// ---------------------------------------------------------------------------
// schemaResource helpers
// ---------------------------------------------------------------------------

func schemaResourceSchema() rschema.Schema {
	r := &resource.SchemaResource{}
	sresp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, sresp)
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
		"drop_cascade":  tftypes.NewValue(tftypes.Bool, false),
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
		"drop_cascade":  tftypes.NewValue(tftypes.Bool, false),
		"timeouts":      tftypes.NewValue(timeoutsObjectType, nil),
	})
}

// ---------------------------------------------------------------------------
// schemaResource tests
// ---------------------------------------------------------------------------

func TestSchemaResource_Create_execError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.SchemaResource{DB: mockDB}
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
}

func TestSchemaResource_Create_currentDBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil)
	// database is null, so it queries current_database()
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", nil, "postgres", false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.SchemaResource{DB: mockDB}
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
}

func TestSchemaResource_Create_readOwnerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil)
	// database is set, so no current_database query
	// Read owner query fails
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("schema vanished"))

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.SchemaResource{DB: mockDB}
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
}

func TestSchemaResource_Read_notFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any()).Return(sql.ErrNoRows)

	ctx := context.Background()
	s := schemaResourceSchema()
	state := schemaStateValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.SchemaResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed (null) when schema not found")
	}
}

func TestSchemaResource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := schemaResourceSchema()
	state := schemaStateValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.SchemaResource{DB: mockDB}
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
}

func TestSchemaResource_Read_currentDBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner1 := mocks.NewMockScanner(ctrl)
	mockScanner2 := mocks.NewMockScanner(ctrl)

	// Schema query succeeds
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner1)
	mockScanner1.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "postgres"
		return nil
	})
	// current_database() query fails (database is null in state)
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner2)
	mockScanner2.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("broken pipe"))

	ctx := context.Background()
	s := schemaResourceSchema()
	// Use null database in state
	tfType := s.Type().TerraformType(ctx)
	state := tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":          tftypes.NewValue(tftypes.String, "myschema"),
		"database":      tftypes.NewValue(tftypes.String, nil),
		"owner":         tftypes.NewValue(tftypes.String, "postgres"),
		"if_not_exists": tftypes.NewValue(tftypes.Bool, false),
		"drop_cascade":  tftypes.NewValue(tftypes.Bool, nil),
		"timeouts":      tftypes.NewValue(timeoutsObjectType, nil),
	})
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.SchemaResource{DB: mockDB}
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
}

func TestSchemaResource_Update_renameError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), `ALTER SCHEMA "oldschema" RENAME TO "newschema"`).Return(nil, fmt.Errorf("schema exists"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "newschema", "testdb", "postgres", false)
	state := schemaStateValue(ctx, s, "oldschema", "testdb", "postgres", false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.SchemaResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error updating schema" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error updating schema' diagnostic")
	}
}

func TestSchemaResource_Update_ownerChangeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockTx := mocks.NewMockTx(ctrl)

	// Same name, owner changes
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), `ALTER SCHEMA "myschema" OWNER TO "newowner"`).Return(nil, fmt.Errorf("role does not exist"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", "testdb", "newowner", false)
	state := schemaStateValue(ctx, s, "myschema", "testdb", "oldowner", false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.SchemaResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error updating schema" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error updating schema' diagnostic")
	}
}

func TestSchemaResource_Update_readBackError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	// Same name, same owner => no ALTER needed, goes straight to read-back
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", "testdb", "postgres", false)
	state := schemaStateValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.SchemaResource{DB: mockDB}
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
}

func TestSchemaResource_Delete_dropError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("schema has dependent objects"))

	ctx := context.Background()
	s := schemaResourceSchema()
	state := schemaStateValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.SchemaResource{DB: mockDB}
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
}

func TestSchemaResource_ImportState_withDatabase(t *testing.T) {
	ctx := context.Background()
	s := schemaResourceSchema()
	req, resp := newImportReqResp(ctx, s, "mydb/myschema")

	r := &resource.SchemaResource{}
	r.ImportState(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}

	var state resource.SchemaResourceModel
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

	r := &resource.SchemaResource{}
	r.ImportState(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}

	var state resource.SchemaResourceModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("error reading state: %v", resp.Diagnostics.Errors())
	}
	if state.Name.ValueString() != "myschema" {
		t.Errorf("expected name=myschema, got %s", state.Name.ValueString())
	}
}

func TestSchemaResource_Create_ifNotExistsSetsOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	// IF NOT EXISTS skips AUTHORIZATION on an existing schema, so the owner is set apart.
	gomock.InOrder(
		mockDB.EXPECT().ExecContext(gomock.Any(), `CREATE SCHEMA IF NOT EXISTS "myschema" AUTHORIZATION "app"`).Return(mockResult{}, nil),
		mockDB.EXPECT().ExecContext(gomock.Any(), `ALTER SCHEMA "myschema" OWNER TO "app"`).Return(mockResult{}, nil),
	)
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), "myschema").Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "app"
		return nil
	})

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", "testdb", "app", true)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.SchemaResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestSchemaResource_Create_ifNotExistsOwnerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	gomock.InOrder(
		mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil),
		mockDB.EXPECT().ExecContext(gomock.Any(), `ALTER SCHEMA "myschema" OWNER TO "app"`).Return(nil, fmt.Errorf("must be member of role")),
	)

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", "testdb", "app", true)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.SchemaResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error changing schema owner") {
		t.Fatalf("expected 'Error changing schema owner', got %v", resp.Diagnostics.Errors())
	}
}

func TestSchemaResource_Create_connectError(t *testing.T) {
	w := &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, fmt.Errorf("refused")
	}}

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "myschema", "otherdb", nil, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.SchemaResource{DB: w}
	r.Create(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error connecting to database") {
		t.Fatalf("expected 'Error connecting to database', got %v", resp.Diagnostics.Errors())
	}
}

func TestSchemaResource_ConnectErrors(t *testing.T) {
	w := &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, fmt.Errorf("refused")
	}}
	ctx := context.Background()
	s := schemaResourceSchema()
	state := schemaStateValue(ctx, s, "myschema", "otherdb", "postgres", false)
	r := &resource.SchemaResource{DB: w}

	readReq, readResp := newReadReqResp(ctx, s, state)
	r.Read(ctx, readReq, readResp)
	updReq, updResp := newUpdateReqResp(ctx, s, state, state)
	r.Update(ctx, updReq, updResp)
	delReq, delResp := newDeleteReqResp(ctx, s, state)
	r.Delete(ctx, delReq, delResp)

	for name, diags := range map[string]diag.Diagnostics{"read": readResp.Diagnostics, "update": updResp.Diagnostics, "delete": delResp.Diagnostics} {
		if !hasDiag(diags, "Error connecting to database") {
			t.Errorf("%s: expected 'Error connecting to database', got %v", name, diags.Errors())
		}
	}
}

func TestSchemaResource_Read_setsDropCascadeDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), "SELECT pg_get_userbyid(nspowner) FROM pg_catalog.pg_namespace WHERE nspname = $1", "myschema").Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "newowner"
		return nil
	})

	ctx := context.Background()
	s := schemaResourceSchema()
	state := tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
		"name":          tftypes.NewValue(tftypes.String, "myschema"),
		"database":      tftypes.NewValue(tftypes.String, "testdb"),
		"owner":         tftypes.NewValue(tftypes.String, "postgres"),
		"if_not_exists": tftypes.NewValue(tftypes.Bool, false),
		"drop_cascade":  tftypes.NewValue(tftypes.Bool, nil),
		"timeouts":      tftypes.NewValue(timeoutsObjectType, nil),
	})
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.SchemaResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.SchemaResourceModel
	resp.State.Get(ctx, &got)
	if got.Owner.ValueString() != "newowner" {
		t.Errorf("expected owner from pg_namespace, got %q", got.Owner.ValueString())
	}
	if got.DropCascade.IsNull() || got.DropCascade.ValueBool() {
		t.Errorf("expected drop_cascade false, got %v", got.DropCascade)
	}
}

func TestSchemaResource_Update_renameAndOwnerInOneTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	gomock.InOrder(
		mockTx.EXPECT().ExecContext(gomock.Any(), `ALTER SCHEMA "oldschema" RENAME TO "newschema"`).Return(mockResult{}, nil),
		mockTx.EXPECT().ExecContext(gomock.Any(), `ALTER SCHEMA "newschema" OWNER TO "newowner"`).Return(mockResult{}, nil),
		mockTx.EXPECT().Commit().Return(nil),
		mockTx.EXPECT().Rollback().Return(sql.ErrTxDone),
	)
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), "newschema").Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "newowner"
		return nil
	})

	ctx := context.Background()
	s := schemaResourceSchema()
	plan := schemaPlanValue(ctx, s, "newschema", "testdb", "newowner", false)
	state := schemaStateValue(ctx, s, "oldschema", "testdb", "oldowner", false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.SchemaResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestSchemaResource_Delete_statement(t *testing.T) {
	tests := []struct {
		name    string
		cascade bool
		want    string
	}{
		{"restrict", false, `DROP SCHEMA IF EXISTS "myschema"`},
		{"cascade", true, `DROP SCHEMA IF EXISTS "myschema" CASCADE`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mocks.NewMockDBTX(ctrl)
			mockDB.EXPECT().ExecContext(gomock.Any(), tc.want).Return(mockResult{}, nil)

			ctx := context.Background()
			s := schemaResourceSchema()
			state := tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
				"name":          tftypes.NewValue(tftypes.String, "myschema"),
				"database":      tftypes.NewValue(tftypes.String, "testdb"),
				"owner":         tftypes.NewValue(tftypes.String, "postgres"),
				"if_not_exists": tftypes.NewValue(tftypes.Bool, false),
				"drop_cascade":  tftypes.NewValue(tftypes.Bool, tc.cascade),
				"timeouts":      tftypes.NewValue(timeoutsObjectType, nil),
			})
			req, resp := newDeleteReqResp(ctx, s, state)

			r := &resource.SchemaResource{DB: mockDB}
			r.Delete(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestSchemaResource_Delete_dependentObjectsHint(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, &pq.Error{Code: "2BP01", Message: "cannot drop schema"})

	ctx := context.Background()
	s := schemaResourceSchema()
	state := schemaStateValue(ctx, s, "myschema", "testdb", "postgres", false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.SchemaResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error deleting schema") {
		t.Fatalf("expected 'Error deleting schema', got %v", resp.Diagnostics.Errors())
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "drop_cascade = true") {
		t.Errorf("expected the drop_cascade hint, got %q", resp.Diagnostics.Errors()[0].Detail())
	}
}

func TestDatabaseResource_Update_tablespaceSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), `ALTER DATABASE "testdb" SET TABLESPACE "fast"`).Return(mockResult{}, nil)
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), "testdb", "UTF8").Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 12345
		*dest[1].(*string) = "postgres"
		*dest[2].(*string) = "UTF8"
		*dest[3].(*bool) = true
		*dest[4].(*string) = "C"
		*dest[5].(*string) = "C"
		*dest[6].(*bool) = true
		*dest[7].(*int64) = -1
		*dest[8].(*bool) = false
		*dest[9].(*string) = "fast"
		return nil
	})

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "C", "C", "fast", -1, true, false)
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "C", "C", "pg_default", -1, 12345, true, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.DatabaseResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.DatabaseResourceModel
	resp.State.Get(ctx, &got)
	if got.TablespaceName.ValueString() != "fast" || got.OID.ValueInt64() != 12345 {
		t.Errorf("unexpected state after update: %+v", got)
	}
}

func TestDatabaseResource_Delete_statement(t *testing.T) {
	tests := []struct {
		name  string
		force bool
		want  string
	}{
		{"plain", false, `DROP DATABASE IF EXISTS "testdb"`},
		{"force", true, `DROP DATABASE IF EXISTS "testdb" WITH (FORCE)`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mocks.NewMockDBTX(ctrl)
			mockDB.EXPECT().ExecContext(gomock.Any(), tc.want).Return(mockResult{}, nil)

			ctx := context.Background()
			s := databaseResourceSchema()
			var state resource.DatabaseResourceModel
			raw := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "C", "C", "pg_default", -1, 1, true, false)
			req, resp := newDeleteReqResp(ctx, s, raw)
			req.State.Get(ctx, &state)
			state.ForceDrop = types.BoolValue(tc.force)
			req.State.Set(ctx, &state)

			r := &resource.DatabaseResource{DB: mockDB}
			r.Delete(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestDatabaseResource_Delete_inUseHint(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, &pq.Error{Code: "55006", Message: "database is being accessed by other users"})

	ctx := context.Background()
	s := databaseResourceSchema()
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "C", "C", "pg_default", -1, 1, true, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.DatabaseResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error deleting database") {
		t.Fatalf("expected 'Error deleting database', got %v", resp.Diagnostics.Errors())
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "force_drop = true") {
		t.Errorf("expected the force_drop hint, got %q", resp.Diagnostics.Errors()[0].Detail())
	}
}

// ReleaseDatabase must close the provider's own pools to the database before the drop.
func TestDatabaseResource_Delete_releasesPools(t *testing.T) {
	var opened int
	w := &common.DBWrapper{DB: fakeSQLDB(t), Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		opened++
		return fakeSQLDB(t), nil
	}}
	ctx := context.Background()
	first, err := w.ForDatabase(ctx, "testdb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := databaseResourceSchema()
	state := databaseStateValue(ctx, s, "testdb", "postgres", "template0", "UTF8", "C", "C", "pg_default", -1, 1, true, false)
	req, resp := newDeleteReqResp(ctx, s, state)
	r := &resource.DatabaseResource{DB: w}
	// The drop itself fails on the fake connection; only the release matters here.
	r.Delete(ctx, req, resp)

	again, err := w.ForDatabase(ctx, "testdb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if again == first || opened != 2 {
		t.Errorf("expected the pool to be reopened after the drop, opened %d times", opened)
	}
}

func fakeSQLDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", "host=__fake__ port=0 sslmode=disable connect_timeout=1")
	if err != nil {
		t.Fatalf("failed to create fake db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// ---------------------------------------------------------------------------
// grantResource helpers
// ---------------------------------------------------------------------------

func grantResourceSchema() rschema.Schema {
	r := &resource.GrantResource{}
	sresp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, sresp)
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

func grantValueWithObjects(ctx context.Context, s rschema.Schema, id any, role, schemaName, objectType string, objects, privileges []string, withGrantOption bool) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var privVals []tftypes.Value
	for _, p := range privileges {
		privVals = append(privVals, tftypes.NewValue(tftypes.String, p))
	}
	var objVals []tftypes.Value
	for _, o := range objects {
		objVals = append(objVals, tftypes.NewValue(tftypes.String, o))
	}
	objList := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)
	if objVals != nil {
		objList = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, objVals)
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"id":                tftypes.NewValue(tftypes.String, id),
		"role":              tftypes.NewValue(tftypes.String, role),
		"database":          tftypes.NewValue(tftypes.String, nil),
		"schema":            tftypes.NewValue(tftypes.String, schemaName),
		"object_type":       tftypes.NewValue(tftypes.String, objectType),
		"objects":           objList,
		"privileges":        tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, privVals),
		"with_grant_option": tftypes.NewValue(tftypes.Bool, withGrantOption),
		"timeouts":          tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func hasDiag(diags diag.Diagnostics, summary string) bool {
	for _, d := range diags.Errors() {
		if d.Summary() == summary {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// grantResource tests
// ---------------------------------------------------------------------------

func TestGrantResource_Create_invalidPrivilegeRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	// No expectations: the allowlist must block the statement before reaching the database.

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"DROP DATABASE"}, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.GrantResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error preparing GRANT") {
		t.Fatalf("expected 'Error preparing GRANT', got %v", resp.Diagnostics.Errors())
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "invalid privilege") {
		t.Errorf("expected an invalid privilege detail, got %q", resp.Diagnostics.Errors()[0].Detail())
	}
}

func TestGrantResource_Update_invalidPrivilegeRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	// No expectations: nothing, not even the REVOKE, may run before validation.

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"FOO"}, false)
	state := grantStateValue(ctx, s, "testrole/database/testdb", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error preparing GRANT") {
		t.Fatalf("expected 'Error preparing GRANT', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Create_beginError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("too many connections"))

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.GrantResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error executing GRANT") {
		t.Fatalf("expected 'Error executing GRANT', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Create_execError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), `GRANT CONNECT ON DATABASE "testdb" TO "testrole"`).Return(nil, fmt.Errorf("permission denied"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.GrantResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error executing GRANT") {
		t.Fatalf("expected 'Error executing GRANT', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Create_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	gomock.InOrder(
		mockTx.EXPECT().ExecContext(gomock.Any(), `GRANT CONNECT ON DATABASE "testdb" TO "testrole" WITH GRANT OPTION`).Return(mockResult{}, nil),
		mockTx.EXPECT().Commit().Return(nil),
		mockTx.EXPECT().Rollback().Return(sql.ErrTxDone),
	)

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"connect"}, true)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.GrantResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var state resource.GrantResourceModel
	resp.State.Get(ctx, &state)
	if state.ID.ValueString() != "testrole/database/testdb" {
		t.Errorf("unexpected id %q", state.ID.ValueString())
	}
}

func TestGrantResource_Create_functionSignatures(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "").Return(stringRowsMock(ctrl, "public.f(integer)", "public.f(text)"), nil)
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	gomock.InOrder(
		mockTx.EXPECT().ExecContext(gomock.Any(), `GRANT EXECUTE ON ROUTINE public.f(integer) TO "testrole"`).Return(mockResult{}, nil),
		mockTx.EXPECT().ExecContext(gomock.Any(), `GRANT EXECUTE ON ROUTINE public.f(text) TO "testrole"`).Return(mockResult{}, nil),
		mockTx.EXPECT().Commit().Return(nil),
		mockTx.EXPECT().Rollback().Return(sql.ErrTxDone),
	)

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantValueWithObjects(ctx, s, nil, "testrole", "public", "function", []string{"f"}, []string{"EXECUTE"}, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.GrantResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Create_missingFunction(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	// Resolved once for the GRANT, then once more to name the missing function.
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "nope", "").Return(stringRowsMock(ctrl), nil)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "nope", "").Return(stringRowsMock(ctrl), nil)

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantValueWithObjects(ctx, s, nil, "testrole", "public", "function", []string{"nope"}, []string{"EXECUTE"}, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.GrantResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error preparing GRANT") {
		t.Fatalf("expected 'Error preparing GRANT', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Create_connectError(t *testing.T) {
	w := &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, fmt.Errorf("refused")
	}}

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "otherdb", "public", "schema", []string{"USAGE"}, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.GrantResource{DB: w}
	r.Create(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error preparing GRANT") {
		t.Fatalf("expected 'Error preparing GRANT', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Read_privError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "testdb", "testrole").Return(nil, fmt.Errorf("relation does not exist"))

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/database/testdb", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error reading privileges") {
		t.Fatalf("expected 'Error reading privileges', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Read_targetGone(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "testdb", "testrole").Return(aclRowsMock(ctrl), nil)

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/database/testdb", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed when the database is gone")
	}
}

func TestGrantResource_Read_databaseGoneOnConnect(t *testing.T) {
	w := &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, &pq.Error{Code: "3D000"}
	}}

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/schema/gonedb/public", "testrole", "gonedb", "public", "schema", []string{"USAGE"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: w}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed when the database is gone")
	}
}

func TestGrantResource_Read_connectError(t *testing.T) {
	w := &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, fmt.Errorf("refused")
	}}

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/schema/otherdb/public", "testrole", "otherdb", "public", "schema", []string{"USAGE"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: w}
	r.Read(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error connecting to database") {
		t.Fatalf("expected 'Error connecting to database', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Read_successDatabase(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	gomock.InOrder(
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "testdb", "testrole").Return(aclRowsMock(ctrl, aclRow{"testdb", "CONNECT", false}), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "d").Return(stringRowsMock(ctrl, "CREATE", "TEMPORARY", "CONNECT"), nil),
	)

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/database/testdb", "testrole", "testdb", "", "database", []string{"connect"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.GrantResourceModel
	resp.State.Get(ctx, &got)
	// The configured spelling is kept when it means what the server holds.
	if privs := common.StringSetToSlice(ctx, got.Privileges); !slices.Equal(privs, []string{"connect"}) {
		t.Errorf("expected privileges [connect] kept, got %v", privs)
	}
	if got.WithGrantOption.ValueBool() {
		t.Error("expected with_grant_option false")
	}
}

func TestGrantResource_Read_drift(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	gomock.InOrder(
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "testdb", "testrole").Return(aclRowsMock(ctrl,
			aclRow{"testdb", "CONNECT", true}, aclRow{"testdb", "CREATE", true}), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "d").Return(stringRowsMock(ctrl, "CREATE", "TEMPORARY", "CONNECT"), nil),
	)

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/database/testdb", "testrole", "testdb", "", "database", []string{"ALL"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.GrantResourceModel
	resp.State.Get(ctx, &got)
	privs := common.StringSetToSlice(ctx, got.Privileges)
	slices.Sort(privs)
	if !slices.Equal(privs, []string{"CONNECT", "CREATE"}) {
		t.Errorf("expected drift to [CONNECT CREATE], got %v", privs)
	}
	if !got.WithGrantOption.ValueBool() {
		t.Error("expected with_grant_option true from the catalog")
	}
}

func TestGrantResource_Read_fullPrivilegesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	gomock.InOrder(
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "testdb", "testrole").Return(aclRowsMock(ctrl, aclRow{"testdb", "CONNECT", false}), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "d").Return(nil, fmt.Errorf("boom")),
	)

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/database/testdb", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error reading privileges") {
		t.Fatalf("expected 'Error reading privileges', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Read_emptySchemaKeepsState(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	expectSchemaExists(ctrl, mockDB, "public", true)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "testrole", gomock.Any()).Return(aclRowsMock(ctrl), nil)

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/table/testdb/public", "testrole", "testdb", "public", "table", []string{"SELECT"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.GrantResourceModel
	resp.State.Get(ctx, &got)
	if privs := common.StringSetToSlice(ctx, got.Privileges); !slices.Equal(privs, []string{"SELECT"}) {
		t.Errorf("expected privileges kept with no object to compare, got %v", privs)
	}
}

func TestGrantResource_Read_afterImport(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	gomock.InOrder(
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "testrole").Return(aclRowsMock(ctrl, aclRow{"public", "USAGE", false}), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "n").Return(stringRowsMock(ctrl, "USAGE", "CREATE"), nil),
	)

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantValueWithObjects(ctx, s, "testrole/schema//public", "testrole", "public", "schema", nil, nil, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.GrantResourceModel
	resp.State.Get(ctx, &got)
	if privs := common.StringSetToSlice(ctx, got.Privileges); !slices.Equal(privs, []string{"USAGE"}) {
		t.Errorf("expected imported privileges [USAGE], got %v", privs)
	}
}

func TestGrantResource_Update_revokeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), `REVOKE ALL PRIVILEGES ON DATABASE "testdb" FROM "testrole"`).Return(nil, fmt.Errorf("cannot revoke"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"CREATE"}, false)
	state := grantStateValue(ctx, s, "testrole/database/testdb", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error updating grant") {
		t.Fatalf("expected 'Error updating grant', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Update_grantErrorRollsBack(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	gomock.InOrder(
		mockTx.EXPECT().ExecContext(gomock.Any(), `REVOKE ALL PRIVILEGES ON DATABASE "testdb" FROM "testrole"`).Return(mockResult{}, nil),
		mockTx.EXPECT().ExecContext(gomock.Any(), `GRANT CREATE ON DATABASE "testdb" TO "testrole"`).Return(nil, fmt.Errorf("cannot grant")),
		mockTx.EXPECT().Rollback().Return(nil),
	)

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"CREATE"}, false)
	state := grantStateValue(ctx, s, "testrole/database/testdb", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error updating grant") {
		t.Fatalf("expected 'Error updating grant', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Update_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	// The function is resolved for the GRANT and again for the REVOKE.
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "(integer)").Return(stringRowsMock(ctrl, "public.f(integer)"), nil)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "(integer)").Return(stringRowsMock(ctrl, "public.f(integer)"), nil)
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	gomock.InOrder(
		mockTx.EXPECT().ExecContext(gomock.Any(), `REVOKE ALL PRIVILEGES ON ROUTINE public.f(integer) FROM "testrole"`).Return(mockResult{}, nil),
		mockTx.EXPECT().ExecContext(gomock.Any(), `GRANT EXECUTE ON ROUTINE public.f(integer) TO "testrole" WITH GRANT OPTION`).Return(mockResult{}, nil),
		mockTx.EXPECT().Commit().Return(nil),
		mockTx.EXPECT().Rollback().Return(sql.ErrTxDone),
	)

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantValueWithObjects(ctx, s, nil, "testrole", "public", "function", []string{"f(integer)"}, []string{"EXECUTE"}, true)
	state := grantValueWithObjects(ctx, s, "testrole/function//public/f(integer)", "testrole", "public", "function", []string{"f(integer)"}, []string{"EXECUTE"}, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.GrantResourceModel
	resp.State.Get(ctx, &got)
	if got.ID.ValueString() != "testrole/function//public/f(integer)" || !got.WithGrantOption.ValueBool() {
		t.Errorf("unexpected state %+v", got)
	}
}

func TestGrantResource_Delete_revokeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot revoke"))

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/database/testdb", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error executing REVOKE") {
		t.Fatalf("expected 'Error executing REVOKE', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Delete_targetGoneIgnored(t *testing.T) {
	for _, code := range []pq.ErrorCode{"3D000", "3F000", "42P01", "42883", "42704"} {
		t.Run(string(code), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mocks.NewMockDBTX(ctrl)
			mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, &pq.Error{Code: code}).Times(2)

			ctx := context.Background()
			s := grantResourceSchema()
			state := grantValueWithObjects(ctx, s, "testrole/table//public/t1,t2", "testrole", "public", "table", []string{"t1", "t2"}, []string{"SELECT"}, false)
			req, resp := newDeleteReqResp(ctx, s, state)

			r := &resource.GrantResource{DB: mockDB}
			r.Delete(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestGrantResource_Delete_otherPqErrorReported(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, &pq.Error{Code: "42501", Message: "permission denied"})

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/schema/testdb/public", "testrole", "testdb", "public", "schema", []string{"USAGE"}, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error executing REVOKE") {
		t.Fatalf("expected 'Error executing REVOKE', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Delete_databaseGone(t *testing.T) {
	w := &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, &pq.Error{Code: "3D000"}
	}}

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/schema/gonedb/public", "testrole", "gonedb", "public", "schema", []string{"USAGE"}, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: w}
	r.Delete(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Delete_connectError(t *testing.T) {
	w := &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, fmt.Errorf("refused")
	}}

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole/schema/otherdb/public", "testrole", "otherdb", "public", "schema", []string{"USAGE"}, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: w}
	r.Delete(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error connecting to database") {
		t.Fatalf("expected 'Error connecting to database', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Delete_functionsGone(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	// No ExecContext: with every function dropped there is nothing to revoke.
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "").Return(stringRowsMock(ctrl), nil)

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantValueWithObjects(ctx, s, "testrole/function//public/f", "testrole", "public", "function", []string{"f"}, []string{"EXECUTE"}, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Delete_functionResolveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "").Return(nil, fmt.Errorf("boom"))

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantValueWithObjects(ctx, s, "testrole/function//public/f", "testrole", "public", "function", []string{"f"}, []string{"EXECUTE"}, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error preparing REVOKE") {
		t.Fatalf("expected 'Error preparing REVOKE', got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Delete_functionSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "").Return(stringRowsMock(ctrl, "public.f(integer)"), nil)
	mockDB.EXPECT().ExecContext(gomock.Any(), `REVOKE ALL PRIVILEGES ON ROUTINE public.f(integer) FROM "testrole"`).Return(mockResult{}, nil)

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantValueWithObjects(ctx, s, "testrole/function//public/f", "testrole", "public", "function", []string{"f"}, []string{"EXECUTE"}, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_ImportState_invalidFormat(t *testing.T) {
	ctx := context.Background()
	s := grantResourceSchema()
	req, resp := newImportReqResp(ctx, s, "invalid")

	r := &resource.GrantResource{}
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

func TestGrantResource_ImportState_emptyString(t *testing.T) {
	ctx := context.Background()
	s := grantResourceSchema()
	req, resp := newImportReqResp(ctx, s, "")

	r := &resource.GrantResource{}
	r.ImportState(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for empty import ID")
	}
}

func TestGrantResource_ImportState_twoParts(t *testing.T) {
	ctx := context.Background()
	s := grantResourceSchema()
	req, resp := newImportReqResp(ctx, s, "role/database")

	r := &resource.GrantResource{}
	r.ImportState(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for 2-part import ID")
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

func TestGrantResource_ImportState_sixParts(t *testing.T) {
	ctx := context.Background()
	s := grantResourceSchema()
	req, resp := newImportReqResp(ctx, s, "role/table/db/schema/t1/extra")

	r := &resource.GrantResource{}
	r.ImportState(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Invalid Import ID") {
		t.Fatalf("expected 'Invalid Import ID' for a 6-part import ID, got %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_ImportState_5partsWithObjects(t *testing.T) {
	ctx := context.Background()
	s := grantResourceSchema()
	req, resp := newImportReqResp(ctx, s, "testrole/FUNCTION//public/f(integer),g")

	r := &resource.GrantResource{}
	r.ImportState(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var state resource.GrantResourceModel
	resp.State.Get(ctx, &state)
	if state.ObjectType.ValueString() != "function" {
		t.Errorf("expected object_type lower-cased to function, got %q", state.ObjectType.ValueString())
	}
	if !state.Database.IsNull() {
		t.Errorf("expected an empty database part to be null, got %q", state.Database.ValueString())
	}
	if objs := common.StringListToSlice(ctx, state.Objects); !slices.Equal(objs, []string{"f(integer)", "g"}) {
		t.Errorf("expected objects [f(integer) g], got %v", objs)
	}
	if !state.Privileges.IsNull() {
		t.Errorf("expected privileges null so Read fills them, got %v", state.Privileges)
	}
	if state.ID.ValueString() != "testrole/function//public/f(integer),g" {
		t.Errorf("unexpected id %q", state.ID.ValueString())
	}
}

func TestGrantResource_ImportState_3parts(t *testing.T) {
	ctx := context.Background()
	s := grantResourceSchema()
	req, resp := newImportReqResp(ctx, s, "testrole/database/testdb")

	r := &resource.GrantResource{}
	r.ImportState(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}

	var state resource.GrantResourceModel
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
	if state.ID.ValueString() != "testrole/database/testdb" {
		t.Errorf("expected id=testrole/database/testdb, got %s", state.ID.ValueString())
	}
}

func TestGrantResource_ImportState_4parts(t *testing.T) {
	ctx := context.Background()
	s := grantResourceSchema()
	req, resp := newImportReqResp(ctx, s, "testrole/table/testdb/public")

	r := &resource.GrantResource{}
	r.ImportState(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}

	var state resource.GrantResourceModel
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
	if state.ID.ValueString() != "testrole/table/testdb/public" {
		t.Errorf("expected id=testrole/table/testdb/public, got %s", state.ID.ValueString())
	}
}

// ---------------------------------------------------------------------------
// userResource helpers
// ---------------------------------------------------------------------------

func userResourceSchema() rschema.Schema {
	r := &resource.UserResource{}
	sresp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func userStateValue(ctx context.Context, s rschema.Schema, name string, superuser, createDB, createRole, replication bool, connLimit, oid int64) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":              tftypes.NewValue(tftypes.String, name),
		"password":          tftypes.NewValue(tftypes.String, nil),
		"superuser":         tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":   tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":       tftypes.NewValue(tftypes.Bool, createRole),
		"replication":       tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit":  tftypes.NewValue(tftypes.Number, connLimit),
		"valid_until":       tftypes.NewValue(tftypes.String, nil),
		"roles":             tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		"reassign_owned_to": tftypes.NewValue(tftypes.String, nil),
		"oid":               tftypes.NewValue(tftypes.Number, oid),
		"timeouts":          tftypes.NewValue(timeoutsObjectType, nil),
	})
}

// ---------------------------------------------------------------------------
// userResource tests
// ---------------------------------------------------------------------------

func userValue(ctx context.Context, s rschema.Schema, name string, superuser bool, password, validUntil, reassignTo, oid any, roles ...string) tftypes.Value {
	rolesVal := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)
	if roles != nil {
		var vals []tftypes.Value
		for _, r := range roles {
			vals = append(vals, tftypes.NewValue(tftypes.String, r))
		}
		rolesVal = tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, vals)
	}
	return tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
		"name":              tftypes.NewValue(tftypes.String, name),
		"password":          tftypes.NewValue(tftypes.String, password),
		"superuser":         tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":   tftypes.NewValue(tftypes.Bool, false),
		"create_role":       tftypes.NewValue(tftypes.Bool, false),
		"replication":       tftypes.NewValue(tftypes.Bool, false),
		"connection_limit":  tftypes.NewValue(tftypes.Number, -1),
		"valid_until":       tftypes.NewValue(tftypes.String, validUntil),
		"roles":             rolesVal,
		"reassign_owned_to": tftypes.NewValue(tftypes.String, reassignTo),
		"oid":               tftypes.NewValue(tftypes.Number, oid),
		"timeouts":          tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func expectReadUser(ctrl *gomock.Controller, db *mocks.MockDBTX, name string, oid int64, validUntil sql.NullString, memberOf ...string) {
	scanner := mocks.NewMockScanner(ctrl)
	db.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), name).Return(scanner)
	scanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = oid
		*dest[1].(*bool) = true
		*dest[6].(*int64) = -1
		*dest[7].(*sql.NullString) = validUntil
		return nil
	})
	db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), oid).Return(stringRowsMock(ctrl, memberOf...), nil)
}

func TestUserResource_Create_txErrors(t *testing.T) {
	statements := []string{`CREATE USER "testuser" WITH CONNECTION LIMIT -1`, `GRANT "admin" TO "testuser"`}
	for failAt, name := range []string{"create fails", "membership fails", "commit fails"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mocks.NewMockDBTX(ctrl)
			expectTx(ctrl, mockDB, statements, failAt)

			ctx := context.Background()
			s := userResourceSchema()
			plan := userValue(ctx, s, "testuser", false, nil, nil, nil, nil, "admin")
			req, resp := newCreateReqResp(ctx, s, plan)

			r := &resource.UserResource{DB: mockDB}
			r.Create(ctx, req, resp)

			if !hasDiag(resp.Diagnostics, "Error creating user") {
				t.Fatalf("expected 'Error creating user', got %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestUserResource_Create_beginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot begin"))

	ctx := context.Background()
	s := userResourceSchema()
	plan := userValue(ctx, s, "testuser", false, nil, nil, nil, nil)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.UserResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error creating user") {
		t.Fatalf("expected 'Error creating user', got %v", resp.Diagnostics.Errors())
	}
}

func TestUserResource_Create_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	expectTx(ctrl, mockDB, []string{
		`CREATE USER "testuser" WITH SUPERUSER CONNECTION LIMIT -1 PASSWORD 'pw' VALID UNTIL '2030-01-01T00:00:00Z'`,
		`GRANT "writers" TO "testuser"`,
		`GRANT "readers" TO "testuser"`,
	}, -1)
	expectReadUser(ctrl, mockDB, "testuser", 16384, sql.NullString{String: "2030-01-01 00:00:00+00", Valid: true}, "readers", "writers")

	ctx := context.Background()
	s := userResourceSchema()
	plan := userValue(ctx, s, "testuser", true, "pw", "2030-01-01T00:00:00Z", nil, nil, "writers", "readers")
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.UserResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.UserResourceModel
	resp.State.Get(ctx, &got)
	// The server spells the same instant and members differently; the configured values are kept.
	if got.ValidUntil.ValueString() != "2030-01-01T00:00:00Z" {
		t.Errorf("expected the configured valid_until kept, got %q", got.ValidUntil.ValueString())
	}
	if roles := common.StringListToSlice(ctx, got.Roles); !slices.Equal(roles, []string{"writers", "readers"}) {
		t.Errorf("expected the configured roles order kept, got %v", roles)
	}
}

func TestUserResource_Read_notFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sql.ErrNoRows)

	ctx := context.Background()
	s := userResourceSchema()
	state := userStateValue(ctx, s, "testuser", false, false, false, false, -1, 16384)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.UserResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed (null) when user not found")
	}
}

func TestUserResource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := userResourceSchema()
	state := userStateValue(ctx, s, "testuser", false, false, false, false, -1, 16384)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.UserResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error reading user") {
		t.Fatalf("expected 'Error reading user', got %v", resp.Diagnostics.Errors())
	}
}

func TestUserResource_Read_driftAndClears(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	// valid_until 'infinity' reads as NULL and the role was removed from every group.
	expectReadUser(ctrl, mockDB, "testuser", 16384, sql.NullString{})

	ctx := context.Background()
	s := userResourceSchema()
	state := userValue(ctx, s, "testuser", false, nil, "2030-01-01T00:00:00Z", nil, 16384, "admin")
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.UserResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.UserResourceModel
	resp.State.Get(ctx, &got)
	if !got.ValidUntil.IsNull() {
		t.Errorf("expected valid_until null, got %q", got.ValidUntil.ValueString())
	}
	if got.Roles.IsNull() || len(got.Roles.Elements()) != 0 {
		t.Errorf("expected an empty roles list, got %v", got.Roles)
	}
}

func TestUserResource_Update_txErrors(t *testing.T) {
	statements := []string{
		`ALTER USER "olduser" RENAME TO "testuser"`,
		`ALTER USER "testuser" WITH SUPERUSER CONNECTION LIMIT -1 PASSWORD NULL`,
		`GRANT "writers" TO "testuser"`,
		`REVOKE "readers" FROM "testuser"`,
	}
	for failAt, name := range []string{"rename fails", "alter fails", "grant fails", "revoke fails", "commit fails"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mocks.NewMockDBTX(ctrl)
			expectTx(ctrl, mockDB, statements, failAt)

			ctx := context.Background()
			s := userResourceSchema()
			plan := userValue(ctx, s, "testuser", true, nil, nil, nil, 16384, "writers")
			state := userValue(ctx, s, "olduser", false, "old", nil, nil, 16384, "readers")
			req, resp := newUpdateReqResp(ctx, s, plan, state)

			r := &resource.UserResource{DB: mockDB}
			r.Update(ctx, req, resp)

			if !hasDiag(resp.Diagnostics, "Error updating user") {
				t.Fatalf("expected 'Error updating user', got %v", resp.Diagnostics.Errors())
			}
		})
	}
}

func TestUserResource_Update_clearsExpiry(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	expectTx(ctrl, mockDB, []string{`ALTER USER "testuser" WITH CONNECTION LIMIT -1 VALID UNTIL 'infinity'`}, -1)
	expectReadUser(ctrl, mockDB, "testuser", 16384, sql.NullString{})

	ctx := context.Background()
	s := userResourceSchema()
	plan := userValue(ctx, s, "testuser", false, nil, nil, nil, 16384)
	state := userValue(ctx, s, "testuser", false, nil, "2030-01-01T00:00:00Z", nil, 16384)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.UserResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.UserResourceModel
	resp.State.Get(ctx, &got)
	if !got.ValidUntil.IsNull() || !got.Roles.IsNull() {
		t.Errorf("expected valid_until and roles null, got %v, %v", got.ValidUntil, got.Roles)
	}
}

func TestUserResource_Delete_dropError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	tx := mocks.NewMockTx(ctrl)
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(tx, nil)
	tx.EXPECT().ExecContext(gomock.Any(), `DROP USER "testuser"`).Return(nil, &pq.Error{Code: "2BP01", Message: "role cannot be dropped"})
	tx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := userResourceSchema()
	state := userStateValue(ctx, s, "testuser", false, false, false, false, -1, 16384)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.UserResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error deleting user") {
		t.Fatalf("expected 'Error deleting user', got %v", resp.Diagnostics.Errors())
	}
	if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "reassign_owned_to") {
		t.Errorf("expected the reassign_owned_to hint, got %q", resp.Diagnostics.Errors()[0].Detail())
	}
}

func TestUserResource_Delete_reassignsOwned(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	expectTx(ctrl, mockDB, []string{`REASSIGN OWNED BY "testuser" TO "postgres"`, `DROP OWNED BY "testuser"`, `DROP USER "testuser"`}, -1)

	ctx := context.Background()
	s := userResourceSchema()
	state := userValue(ctx, s, "testuser", false, nil, nil, "postgres", 16384)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.UserResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}
