package resource_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/resource"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/mocks"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection lost"))

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

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sql.ErrNoRows)

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

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection refused"))

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

func rolePlanValue(ctx context.Context, s rschema.Schema, name string, superuser, createDB, createRole, replication bool, connLimit int64) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"superuser":        tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":  tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":      tftypes.NewValue(tftypes.Bool, createRole),
		"replication":      tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit": tftypes.NewValue(tftypes.Number, connLimit),
		"privilege":        tftypes.NewValue(privilegeListType, []tftypes.Value{}),
		"oid":              tftypes.NewValue(tftypes.Number, nil),
		"timeouts":         tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func rolePlanValueWithPrivilege(ctx context.Context, s rschema.Schema, name string, superuser, createDB, createRole, replication bool, connLimit int64, privObjectType, schemaName string, privs []string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var privVals []tftypes.Value
	for _, p := range privs {
		privVals = append(privVals, tftypes.NewValue(tftypes.String, p))
	}
	privBlock := tftypes.NewValue(privilegeObjectType, map[string]tftypes.Value{
		"privileges":  tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, privVals),
		"object_type": tftypes.NewValue(tftypes.String, privObjectType),
		"schema":      tftypes.NewValue(tftypes.String, schemaName),
		"database":    tftypes.NewValue(tftypes.String, nil),
		"objects":     tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
	})
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"superuser":        tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":  tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":      tftypes.NewValue(tftypes.Bool, createRole),
		"replication":      tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit": tftypes.NewValue(tftypes.Number, connLimit),
		"privilege":        tftypes.NewValue(privilegeListType, []tftypes.Value{privBlock}),
		"oid":              tftypes.NewValue(tftypes.Number, nil),
		"timeouts":         tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func roleStateValue(ctx context.Context, s rschema.Schema, name string, superuser, createDB, createRole, replication bool, connLimit, oid int64) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"superuser":        tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":  tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":      tftypes.NewValue(tftypes.Bool, createRole),
		"replication":      tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit": tftypes.NewValue(tftypes.Number, connLimit),
		"privilege":        tftypes.NewValue(privilegeListType, []tftypes.Value{}),
		"oid":              tftypes.NewValue(tftypes.Number, oid),
		"timeouts":         tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func roleStateValueWithPrivilege(ctx context.Context, s rschema.Schema, name string, superuser, createDB, createRole, replication bool, connLimit, oid int64, privObjectType, schemaName string, privs []string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var privVals []tftypes.Value
	for _, p := range privs {
		privVals = append(privVals, tftypes.NewValue(tftypes.String, p))
	}
	privBlock := tftypes.NewValue(privilegeObjectType, map[string]tftypes.Value{
		"privileges":  tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, privVals),
		"object_type": tftypes.NewValue(tftypes.String, privObjectType),
		"schema":      tftypes.NewValue(tftypes.String, schemaName),
		"database":    tftypes.NewValue(tftypes.String, nil),
		"objects":     tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
	})
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"superuser":        tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":  tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":      tftypes.NewValue(tftypes.Bool, createRole),
		"replication":      tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit": tftypes.NewValue(tftypes.Number, connLimit),
		"privilege":        tftypes.NewValue(privilegeListType, []tftypes.Value{privBlock}),
		"oid":              tftypes.NewValue(tftypes.Number, oid),
		"timeouts":         tftypes.NewValue(timeoutsObjectType, nil),
	})
}

// ---------------------------------------------------------------------------
// roleResource tests
// ---------------------------------------------------------------------------

func TestRoleResource_Create_beginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot begin"))

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValue(ctx, s, "testrole", false, false, false, false, -1)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.RoleResource{DB: mockDB}
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
}

func TestRoleResource_Create_execError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("role already exists"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValue(ctx, s, "testrole", false, false, false, false, -1)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.RoleResource{DB: mockDB}
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
}

func TestRoleResource_Create_grantPrivilegeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("role admin does not exist"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValueWithPrivilege(ctx, s, "testrole", false, false, false, false, -1, "table", "public", []string{"SELECT"})
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.RoleResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error granting privilege" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error granting privilege' diagnostic")
	}
}

func TestRoleResource_Create_commitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil)
	mockTx.EXPECT().Commit().Return(fmt.Errorf("commit failed"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValue(ctx, s, "testrole", false, false, false, false, -1)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.RoleResource{DB: mockDB}
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
}

func TestRoleResource_Read_notFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sql.ErrNoRows)

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

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleStateValue(ctx, s, "testrole", false, false, false, false, -1, 16384)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.RoleResource{DB: mockDB}
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
}

func TestRoleResource_Update_renameError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("role exists"))

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValue(ctx, s, "newname", false, false, false, false, -1)
	state := roleStateValue(ctx, s, "oldname", false, false, false, false, -1, 16384)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.RoleResource{DB: mockDB}
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
}

func TestRoleResource_Update_alterError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// Same name, so no rename. ALTER ROLE fails.
	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("insufficient privilege"))

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValue(ctx, s, "testrole", false, false, false, false, -1)
	state := roleStateValue(ctx, s, "testrole", false, false, false, false, -1, 16384)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.RoleResource{DB: mockDB}
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
}

func TestRoleResource_Update_grantPrivilegeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// ALTER ROLE succeeds, then GRANT privilege fails
	gomock.InOrder(
		mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil),
		mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("schema public does not exist")),
	)

	ctx := context.Background()
	s := roleResourceSchema()
	plan := rolePlanValueWithPrivilege(ctx, s, "testrole", false, false, false, false, -1, "table", "public", []string{"SELECT"})
	state := roleStateValue(ctx, s, "testrole", false, false, false, false, -1, 16384)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.RoleResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error granting privilege" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error granting privilege' diagnostic")
	}
}

func TestRoleResource_Update_revokePrivilegeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// ALTER ROLE succeeds, then REVOKE old privilege fails
	gomock.InOrder(
		mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil),
		mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot revoke")),
	)

	ctx := context.Background()
	s := roleResourceSchema()
	// Plan has no privileges, state has a privilege (revoking it)
	plan := rolePlanValue(ctx, s, "testrole", false, false, false, false, -1)
	state := roleStateValueWithPrivilege(ctx, s, "testrole", false, false, false, false, -1, 16384, "table", "public", []string{"SELECT"})
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.RoleResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error revoking privilege" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error revoking privilege' diagnostic")
	}
}

func TestRoleResource_Delete_dropError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("role has dependent objects"))

	ctx := context.Background()
	s := roleResourceSchema()
	state := roleStateValue(ctx, s, "testrole", false, false, false, false, -1, 16384)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.RoleResource{DB: mockDB}
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

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("schema exists"))

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
		if d.Summary() == "Error renaming schema" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error renaming schema' diagnostic")
	}
}

func TestSchemaResource_Update_ownerChangeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// Same name, owner changes
	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("role does not exist"))

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
		if d.Summary() == "Error changing schema owner" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error changing schema owner' diagnostic")
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

// ---------------------------------------------------------------------------
// grantResource tests
// ---------------------------------------------------------------------------

func TestGrantResource_Create_execError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.GrantResource{DB: mockDB}
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
}

func TestGrantResource_Read_privError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("relation does not exist"))

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole_database_testdb_", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
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
}

func TestGrantResource_Read_empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole_database_testdb_", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if !resp.State.Raw.IsNull() {
		t.Error("expected state to be removed (null) when no privileges found")
	}
}

func TestGrantResource_Update_revokeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot revoke"))

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"CREATE"}, false)
	state := grantStateValue(ctx, s, "testrole_database_testdb_", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.GrantResource{DB: mockDB}
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
}

func TestGrantResource_Update_grantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// REVOKE succeeds, GRANT fails
	gomock.InOrder(
		mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil),
		mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot grant")),
	)

	ctx := context.Background()
	s := grantResourceSchema()
	plan := grantPlanValue(ctx, s, "testrole", "testdb", "", "database", []string{"CREATE"}, false)
	state := grantStateValue(ctx, s, "testrole_database_testdb_", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.GrantResource{DB: mockDB}
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
}

func TestGrantResource_Delete_revokeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot revoke"))

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole_database_testdb_", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
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
	if state.ID.ValueString() != "testrole_database_testdb_" {
		t.Errorf("expected id=testrole_database_testdb_, got %s", state.ID.ValueString())
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
	if state.ID.ValueString() != "testrole_table_testdb_public" {
		t.Errorf("expected id=testrole_table_testdb_public, got %s", state.ID.ValueString())
	}
}

func TestGrantResource_Read_successDatabase(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	// database-level grant triggers drift detection via readPrivileges
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "CONNECT"
			*dest[1].(*bool) = false
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
		mockRows.EXPECT().Err().Return(nil),
		mockRows.EXPECT().Close().Return(nil),
	)

	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole_database_testdb_", "testrole", "testdb", "", "database", []string{"CONNECT"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Read_noDriftTableAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// table with no specific objects -> canDetectDrift=false, no queries
	ctx := context.Background()
	s := grantResourceSchema()
	state := grantStateValue(ctx, s, "testrole_table_testdb_public", "testrole", "testdb", "public", "table", []string{"SELECT"}, false)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.GrantResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
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

func userPlanValue(ctx context.Context, s rschema.Schema, name string, superuser, createDB, createRole, replication bool, connLimit int64) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"password":         tftypes.NewValue(tftypes.String, nil),
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

func userPlanValueWithRoles(ctx context.Context, s rschema.Schema, name string, superuser, createDB, createRole, replication bool, connLimit int64, roles []string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var roleVals []tftypes.Value
	for _, r := range roles {
		roleVals = append(roleVals, tftypes.NewValue(tftypes.String, r))
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"password":         tftypes.NewValue(tftypes.String, nil),
		"superuser":        tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":  tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":      tftypes.NewValue(tftypes.Bool, createRole),
		"replication":      tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit": tftypes.NewValue(tftypes.Number, connLimit),
		"valid_until":      tftypes.NewValue(tftypes.String, nil),
		"roles":            tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, roleVals),
		"oid":              tftypes.NewValue(tftypes.Number, nil),
		"timeouts":         tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func userStateValue(ctx context.Context, s rschema.Schema, name string, superuser, createDB, createRole, replication bool, connLimit, oid int64) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"password":         tftypes.NewValue(tftypes.String, nil),
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

func userStateValueWithRoles(ctx context.Context, s rschema.Schema, name string, superuser, createDB, createRole, replication bool, connLimit, oid int64, roles []string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var roleVals []tftypes.Value
	for _, r := range roles {
		roleVals = append(roleVals, tftypes.NewValue(tftypes.String, r))
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"password":         tftypes.NewValue(tftypes.String, nil),
		"superuser":        tftypes.NewValue(tftypes.Bool, superuser),
		"create_database":  tftypes.NewValue(tftypes.Bool, createDB),
		"create_role":      tftypes.NewValue(tftypes.Bool, createRole),
		"replication":      tftypes.NewValue(tftypes.Bool, replication),
		"connection_limit": tftypes.NewValue(tftypes.Number, connLimit),
		"valid_until":      tftypes.NewValue(tftypes.String, nil),
		"roles":            tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, roleVals),
		"oid":              tftypes.NewValue(tftypes.Number, oid),
		"timeouts":         tftypes.NewValue(timeoutsObjectType, nil),
	})
}

// ---------------------------------------------------------------------------
// userResource tests
// ---------------------------------------------------------------------------

func TestUserResource_Create_beginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot begin"))

	ctx := context.Background()
	s := userResourceSchema()
	plan := userPlanValue(ctx, s, "testuser", false, false, false, false, -1)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.UserResource{DB: mockDB}
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
}

func TestUserResource_Create_execError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("user already exists"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := userResourceSchema()
	plan := userPlanValue(ctx, s, "testuser", false, false, false, false, -1)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.UserResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error creating user" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error creating user' diagnostic")
	}
}

func TestUserResource_Create_grantMembershipError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("role admin does not exist"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := userResourceSchema()
	plan := userPlanValueWithRoles(ctx, s, "testuser", false, false, false, false, -1, []string{"admin"})
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.UserResource{DB: mockDB}
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
}

func TestUserResource_Create_commitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil)
	mockTx.EXPECT().Commit().Return(fmt.Errorf("commit failed"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := userResourceSchema()
	plan := userPlanValue(ctx, s, "testuser", false, false, false, false, -1)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.UserResource{DB: mockDB}
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
}

func TestUserResource_Read_notFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
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

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := userResourceSchema()
	state := userStateValue(ctx, s, "testuser", false, false, false, false, -1, 16384)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.UserResource{DB: mockDB}
	r.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error reading user" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error reading user' diagnostic")
	}
}

func TestUserResource_Update_renameError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("user exists"))

	ctx := context.Background()
	s := userResourceSchema()
	plan := userPlanValue(ctx, s, "newname", false, false, false, false, -1)
	state := userStateValue(ctx, s, "oldname", false, false, false, false, -1, 16384)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.UserResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error renaming user" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error renaming user' diagnostic")
	}
}

func TestUserResource_Update_alterError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// Same name, so no rename. ALTER USER fails.
	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("insufficient privilege"))

	ctx := context.Background()
	s := userResourceSchema()
	plan := userPlanValue(ctx, s, "testuser", false, false, false, false, -1)
	state := userStateValue(ctx, s, "testuser", false, false, false, false, -1, 16384)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.UserResource{DB: mockDB}
	r.Update(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error updating user" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error updating user' diagnostic")
	}
}

func TestUserResource_Update_grantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// ALTER USER succeeds, then GRANT membership fails
	gomock.InOrder(
		mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil),
		mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("role admin does not exist")),
	)

	ctx := context.Background()
	s := userResourceSchema()
	plan := userPlanValueWithRoles(ctx, s, "testuser", false, false, false, false, -1, []string{"admin"})
	state := userStateValue(ctx, s, "testuser", false, false, false, false, -1, 16384)
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.UserResource{DB: mockDB}
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
}

func TestUserResource_Update_revokeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	// ALTER USER succeeds, then REVOKE membership fails
	gomock.InOrder(
		mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil),
		mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot revoke")),
	)

	ctx := context.Background()
	s := userResourceSchema()
	// Plan has no roles, state has a role (revoking it)
	plan := userPlanValue(ctx, s, "testuser", false, false, false, false, -1)
	state := userStateValueWithRoles(ctx, s, "testuser", false, false, false, false, -1, 16384, []string{"admin"})
	req, resp := newUpdateReqResp(ctx, s, plan, state)

	r := &resource.UserResource{DB: mockDB}
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
}

func TestUserResource_Delete_dropError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("user has dependent objects"))

	ctx := context.Background()
	s := userResourceSchema()
	state := userStateValue(ctx, s, "testuser", false, false, false, false, -1, 16384)
	req, resp := newDeleteReqResp(ctx, s, state)

	r := &resource.UserResource{DB: mockDB}
	r.Delete(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Error deleting user" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error deleting user' diagnostic")
	}
}
