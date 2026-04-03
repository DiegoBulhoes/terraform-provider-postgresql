package resource_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/resource"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/mocks"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"go.uber.org/mock/gomock"
)

// ---------------------------------------------------------------------------
// readRole tests
// ---------------------------------------------------------------------------

func TestReadRole_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sql.ErrNoRows)

	r := &resource.RoleResource{DB: mockDB}
	model := &resource.RoleResourceModel{Name: types.StringValue("ghost")}

	diags := r.ReadRole(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Role not found" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Role not found' diagnostic, got different errors")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}
}

func TestReadRole_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection refused"))

	r := &resource.RoleResource{DB: mockDB}
	model := &resource.RoleResourceModel{Name: types.StringValue("testrole")}

	diags := r.ReadRole(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Error reading role" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error reading role' diagnostic")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}
}

func TestReadRole_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 99
		*dest[1].(*bool) = true
		*dest[2].(*bool) = false
		*dest[3].(*bool) = true
		*dest[4].(*bool) = false
		*dest[5].(*int64) = 10
		return nil
	})

	r := &resource.RoleResource{DB: mockDB}
	model := &resource.RoleResourceModel{Name: types.StringValue("testrole")}

	diags := r.ReadRole(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if model.OID.ValueInt64() != 99 {
		t.Errorf("expected OID 99, got %d", model.OID.ValueInt64())
	}
	if !model.Superuser.ValueBool() {
		t.Error("expected Superuser to be true")
	}
	if model.CreateDatabase.ValueBool() {
		t.Error("expected CreateDatabase to be false")
	}
	if !model.CreateRole.ValueBool() {
		t.Error("expected CreateRole to be true")
	}
	if model.Replication.ValueBool() {
		t.Error("expected Replication to be false")
	}
	if model.ConnectionLimit.ValueInt64() != 10 {
		t.Errorf("expected ConnectionLimit 10, got %d", model.ConnectionLimit.ValueInt64())
	}
}

// ---------------------------------------------------------------------------
// readUser tests
// ---------------------------------------------------------------------------

func TestReadUser_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sql.ErrNoRows)

	r := &resource.UserResource{DB: mockDB}
	model := &resource.UserResourceModel{Name: types.StringValue("ghost")}

	diags := r.ReadUser(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "User not found" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'User not found' diagnostic, got different errors")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}
}

func TestReadUser_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection refused"))

	r := &resource.UserResource{DB: mockDB}
	model := &resource.UserResourceModel{Name: types.StringValue("testuser")}

	diags := r.ReadUser(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Error reading user" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error reading user' diagnostic")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}
}

func TestReadUser_MembershipQueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 42
		*dest[1].(*bool) = true
		*dest[2].(*bool) = false
		*dest[3].(*bool) = false
		*dest[4].(*bool) = false
		*dest[5].(*bool) = false
		*dest[6].(*int64) = -1
		*dest[7].(*sql.NullString) = sql.NullString{Valid: false}
		return nil
	})

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("membership query failed"))

	r := &resource.UserResource{DB: mockDB}
	model := &resource.UserResourceModel{Name: types.StringValue("testuser")}

	diags := r.ReadUser(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Error reading role memberships" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error reading role memberships' diagnostic")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}
}

func TestReadUser_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 99
		*dest[1].(*bool) = true
		*dest[2].(*bool) = true
		*dest[3].(*bool) = false
		*dest[4].(*bool) = true
		*dest[5].(*bool) = false
		*dest[6].(*int64) = 10
		*dest[7].(*sql.NullString) = sql.NullString{String: "2025-12-31T23:59:59Z", Valid: true}
		return nil
	})

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "admin"
			return nil
		}),
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "readers"
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
	)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	r := &resource.UserResource{DB: mockDB}
	model := &resource.UserResourceModel{Name: types.StringValue("testuser")}

	diags := r.ReadUser(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if model.OID.ValueInt64() != 99 {
		t.Errorf("expected OID 99, got %d", model.OID.ValueInt64())
	}
	if !model.Superuser.ValueBool() {
		t.Error("expected Superuser to be true")
	}
	if model.CreateDatabase.ValueBool() {
		t.Error("expected CreateDatabase to be false")
	}
	if !model.CreateRole.ValueBool() {
		t.Error("expected CreateRole to be true")
	}
	if model.Replication.ValueBool() {
		t.Error("expected Replication to be false")
	}
	if model.ConnectionLimit.ValueInt64() != 10 {
		t.Errorf("expected ConnectionLimit 10, got %d", model.ConnectionLimit.ValueInt64())
	}
	if model.ValidUntil.ValueString() != "2025-12-31T23:59:59Z" {
		t.Errorf("expected ValidUntil '2025-12-31T23:59:59Z', got %q", model.ValidUntil.ValueString())
	}

	var roles []string
	diags2 := model.Roles.ElementsAs(context.Background(), &roles, false)
	if diags2.HasError() {
		t.Fatalf("error extracting roles: %v", diags2)
	}
	if len(roles) != 2 || roles[0] != "admin" || roles[1] != "readers" {
		t.Errorf("expected roles [admin, readers], got %v", roles)
	}
}

func TestReadUser_NullValidUntil(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 42
		*dest[1].(*bool) = true
		*dest[2].(*bool) = false
		*dest[3].(*bool) = false
		*dest[4].(*bool) = false
		*dest[5].(*bool) = false
		*dest[6].(*int64) = -1
		*dest[7].(*sql.NullString) = sql.NullString{Valid: false}
		return nil
	})

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	r := &resource.UserResource{DB: mockDB}
	model := &resource.UserResourceModel{Name: types.StringValue("testuser")}

	diags := r.ReadUser(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if !model.ValidUntil.IsNull() {
		t.Errorf("expected ValidUntil to be null, got %q", model.ValidUntil.ValueString())
	}
}

func TestReadUser_NoLoginWarning(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 42
		*dest[1].(*bool) = false // rolcanlogin = false
		*dest[2].(*bool) = false
		*dest[3].(*bool) = false
		*dest[4].(*bool) = false
		*dest[5].(*bool) = false
		*dest[6].(*int64) = -1
		*dest[7].(*sql.NullString) = sql.NullString{Valid: false}
		return nil
	})

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	r := &resource.UserResource{DB: mockDB}
	model := &resource.UserResourceModel{Name: types.StringValue("testuser")}

	diags := r.ReadUser(context.Background(), model)
	// Should not have errors, but should have a warning
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Role is not a user" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Role is not a user' warning diagnostic")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}
}

// ---------------------------------------------------------------------------
// buildUserOptions tests
// ---------------------------------------------------------------------------

func TestBuildUserOptions_AllDefaults(t *testing.T) {
	r := &resource.UserResource{}
	model := &resource.UserResourceModel{
		Superuser:       types.BoolValue(false),
		CreateDatabase:  types.BoolValue(false),
		CreateRole:      types.BoolValue(false),
		Replication:     types.BoolValue(false),
		ConnectionLimit: types.Int64Value(-1),
		Password:        types.StringNull(),
		ValidUntil:      types.StringNull(),
	}

	result := r.BuildUserOptions(context.Background(), model)

	expected := []string{"NOSUPERUSER", "NOCREATEDB", "NOCREATEROLE", "NOREPLICATION", "CONNECTION LIMIT -1"}
	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("expected %q in result %q", exp, result)
		}
	}

	// Should NOT contain PASSWORD or VALID UNTIL
	if strings.Contains(result, "PASSWORD") {
		t.Errorf("did not expect PASSWORD in result %q", result)
	}
	if strings.Contains(result, "VALID UNTIL") {
		t.Errorf("did not expect VALID UNTIL in result %q", result)
	}
}

func TestBuildUserOptions_AllEnabled(t *testing.T) {
	r := &resource.UserResource{}
	model := &resource.UserResourceModel{
		Superuser:       types.BoolValue(true),
		CreateDatabase:  types.BoolValue(true),
		CreateRole:      types.BoolValue(true),
		Replication:     types.BoolValue(true),
		ConnectionLimit: types.Int64Value(50),
		Password:        types.StringValue("s3cret"),
		ValidUntil:      types.StringValue("2025-12-31T23:59:59Z"),
	}

	result := r.BuildUserOptions(context.Background(), model)

	expected := []string{"SUPERUSER", "CREATEDB", "CREATEROLE", "REPLICATION", "CONNECTION LIMIT 50", "PASSWORD", "VALID UNTIL"}
	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("expected %q in result %q", exp, result)
		}
	}

	if !strings.HasPrefix(result, " WITH ") {
		t.Errorf("expected result to start with ' WITH ', got %q", result)
	}
}

func TestBuildUserOptions_PasswordIncluded(t *testing.T) {
	r := &resource.UserResource{}
	model := &resource.UserResourceModel{
		Superuser:       types.BoolValue(false),
		CreateDatabase:  types.BoolValue(false),
		CreateRole:      types.BoolValue(false),
		Replication:     types.BoolValue(false),
		ConnectionLimit: types.Int64Value(-1),
		Password:        types.StringValue("mypass"),
		ValidUntil:      types.StringNull(),
	}

	result := r.BuildUserOptions(context.Background(), model)

	// Password should be quoted with pq.QuoteLiteral
	if !strings.Contains(result, "PASSWORD 'mypass'") {
		t.Errorf("expected PASSWORD 'mypass' in result %q", result)
	}
}

// ---------------------------------------------------------------------------
// readDatabase tests
// ---------------------------------------------------------------------------

func TestReadDatabase_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sql.ErrNoRows)

	r := &resource.DatabaseResource{DB: mockDB}
	model := &resource.DatabaseResourceModel{Name: types.StringValue("testdb")}

	diags := r.ReadDatabase(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Database not found" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Database not found' diagnostic")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}
}

func TestReadDatabase_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("disk I/O error"))

	r := &resource.DatabaseResource{DB: mockDB}
	model := &resource.DatabaseResourceModel{Name: types.StringValue("testdb")}

	diags := r.ReadDatabase(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Error reading database" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error reading database' diagnostic")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}
}

func TestReadDatabase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 16384
		*dest[1].(*string) = "postgres"
		*dest[2].(*string) = "UTF8"
		*dest[3].(*string) = "en_US.UTF-8"
		*dest[4].(*string) = "en_US.UTF-8"
		*dest[5].(*bool) = true
		*dest[6].(*int64) = -1
		*dest[7].(*bool) = false
		*dest[8].(*string) = "pg_default"
		return nil
	})

	r := &resource.DatabaseResource{DB: mockDB}
	model := &resource.DatabaseResourceModel{
		Name:     types.StringValue("mydb"),
		Template: types.StringNull(),
	}

	diags := r.ReadDatabase(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if model.OID.ValueInt64() != 16384 {
		t.Errorf("expected OID 16384, got %d", model.OID.ValueInt64())
	}
	if model.Owner.ValueString() != "postgres" {
		t.Errorf("expected owner 'postgres', got %q", model.Owner.ValueString())
	}
	if model.Encoding.ValueString() != "UTF8" {
		t.Errorf("expected encoding 'UTF8', got %q", model.Encoding.ValueString())
	}
	if model.LcCollate.ValueString() != "en_US.UTF-8" {
		t.Errorf("expected lc_collate 'en_US.UTF-8', got %q", model.LcCollate.ValueString())
	}
	if model.AllowConnections.ValueBool() != true {
		t.Error("expected AllowConnections to be true")
	}
	if model.ConnectionLimit.ValueInt64() != -1 {
		t.Errorf("expected ConnectionLimit -1, got %d", model.ConnectionLimit.ValueInt64())
	}
	if model.IsTemplate.ValueBool() != false {
		t.Error("expected IsTemplate to be false")
	}
	if model.TablespaceName.ValueString() != "pg_default" {
		t.Errorf("expected tablespace 'pg_default', got %q", model.TablespaceName.ValueString())
	}
	// Template should be set to default when null
	if model.Template.ValueString() != "template0" {
		t.Errorf("expected template 'template0', got %q", model.Template.ValueString())
	}
}

func TestReadDatabase_PreservesExistingTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 16384
		*dest[1].(*string) = "postgres"
		*dest[2].(*string) = "UTF8"
		*dest[3].(*string) = "en_US.UTF-8"
		*dest[4].(*string) = "en_US.UTF-8"
		*dest[5].(*bool) = true
		*dest[6].(*int64) = -1
		*dest[7].(*bool) = false
		*dest[8].(*string) = "pg_default"
		return nil
	})

	r := &resource.DatabaseResource{DB: mockDB}
	model := &resource.DatabaseResourceModel{
		Name:     types.StringValue("mydb"),
		Template: types.StringValue("template1"),
	}

	diags := r.ReadDatabase(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	// Template should be preserved when already set
	if model.Template.ValueString() != "template1" {
		t.Errorf("expected template 'template1' to be preserved, got %q", model.Template.ValueString())
	}
}

// ---------------------------------------------------------------------------
// buildRoleOptions tests
// ---------------------------------------------------------------------------

func TestBuildRoleOptions_AllDefaults(t *testing.T) {
	r := &resource.RoleResource{}
	model := &resource.RoleResourceModel{
		Superuser:       types.BoolValue(false),
		CreateDatabase:  types.BoolValue(false),
		CreateRole:      types.BoolValue(false),
		Replication:     types.BoolValue(false),
		ConnectionLimit: types.Int64Value(-1),
	}

	result := r.BuildRoleOptions(context.Background(), model)

	expected := []string{"NOLOGIN", "NOSUPERUSER", "NOCREATEDB", "NOCREATEROLE", "NOREPLICATION", "CONNECTION LIMIT -1"}
	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("expected %q in result %q", exp, result)
		}
	}
}

func TestBuildRoleOptions_AllEnabled(t *testing.T) {
	r := &resource.RoleResource{}
	model := &resource.RoleResourceModel{
		Superuser:       types.BoolValue(true),
		CreateDatabase:  types.BoolValue(true),
		CreateRole:      types.BoolValue(true),
		Replication:     types.BoolValue(true),
		ConnectionLimit: types.Int64Value(50),
	}

	result := r.BuildRoleOptions(context.Background(), model)

	expected := []string{"NOLOGIN", "SUPERUSER", "CREATEDB", "CREATEROLE", "REPLICATION", "CONNECTION LIMIT 50"}
	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("expected %q in result %q", exp, result)
		}
	}

	if !strings.HasPrefix(result, " WITH ") {
		t.Errorf("expected result to start with ' WITH ', got %q", result)
	}
}

// ---------------------------------------------------------------------------
// diffRoles tests
// ---------------------------------------------------------------------------

func TestDiffRoles_NoChange(t *testing.T) {
	toGrant, toRevoke := resource.DiffRoles([]string{"a", "b"}, []string{"a", "b"})
	if len(toGrant) != 0 {
		t.Errorf("expected no grants, got %v", toGrant)
	}
	if len(toRevoke) != 0 {
		t.Errorf("expected no revokes, got %v", toRevoke)
	}
}

func TestDiffRoles_AddNew(t *testing.T) {
	toGrant, toRevoke := resource.DiffRoles([]string{"a"}, []string{"a", "b", "c"})
	if len(toGrant) != 2 {
		t.Fatalf("expected 2 grants, got %v", toGrant)
	}
	if len(toRevoke) != 0 {
		t.Errorf("expected no revokes, got %v", toRevoke)
	}
}

func TestDiffRoles_RemoveOld(t *testing.T) {
	toGrant, toRevoke := resource.DiffRoles([]string{"a", "b", "c"}, []string{"a"})
	if len(toGrant) != 0 {
		t.Errorf("expected no grants, got %v", toGrant)
	}
	if len(toRevoke) != 2 {
		t.Fatalf("expected 2 revokes, got %v", toRevoke)
	}
}

func TestDiffRoles_Mixed(t *testing.T) {
	toGrant, toRevoke := resource.DiffRoles([]string{"a", "b"}, []string{"b", "c"})
	if len(toGrant) != 1 || toGrant[0] != "c" {
		t.Errorf("expected grant ['c'], got %v", toGrant)
	}
	if len(toRevoke) != 1 || toRevoke[0] != "a" {
		t.Errorf("expected revoke ['a'], got %v", toRevoke)
	}
}

func TestDiffRoles_BothEmpty(t *testing.T) {
	toGrant, toRevoke := resource.DiffRoles(nil, nil)
	if len(toGrant) != 0 {
		t.Errorf("expected no grants, got %v", toGrant)
	}
	if len(toRevoke) != 0 {
		t.Errorf("expected no revokes, got %v", toRevoke)
	}
}

// ---------------------------------------------------------------------------
// readPrivileges tests
// ---------------------------------------------------------------------------

func TestReadPrivileges_Database_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "CONNECT"
			*dest[1].(*bool) = true
			return nil
		}),
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "CREATE"
			*dest[1].(*bool) = true
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
	)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	r := &resource.GrantResource{DB: mockDB}
	privs, allGrantable, err := r.ReadPrivileges(context.Background(), "myrole", "database", "mydb", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allGrantable {
		t.Error("expected allGrantable to be true")
	}
	if len(privs) != 2 {
		t.Fatalf("expected 2 privileges, got %d", len(privs))
	}
	if privs[0] != "CONNECT" || privs[1] != "CREATE" {
		t.Errorf("expected [CONNECT, CREATE], got %v", privs)
	}
}

func TestReadPrivileges_Database_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("connection refused"))

	r := &resource.GrantResource{DB: mockDB}
	privs, allGrantable, err := r.ReadPrivileges(context.Background(), "myrole", "database", "mydb", "", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected 'connection refused' in error, got %q", err.Error())
	}
	if privs != nil {
		t.Errorf("expected nil privileges, got %v", privs)
	}
	if allGrantable {
		t.Error("expected allGrantable to be false")
	}
}

func TestReadPrivileges_Database_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	r := &resource.GrantResource{DB: mockDB}
	privs, allGrantable, err := r.ReadPrivileges(context.Background(), "myrole", "database", "mydb", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allGrantable {
		t.Error("expected allGrantable to be false when no rows returned")
	}
	if privs != nil {
		t.Errorf("expected nil privileges, got %v", privs)
	}
}

func TestReadPrivileges_Schema_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "USAGE"
			*dest[1].(*bool) = true
			return nil
		}),
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "CREATE"
			*dest[1].(*bool) = false
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
	)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	r := &resource.GrantResource{DB: mockDB}
	privs, allGrantable, err := r.ReadPrivileges(context.Background(), "myrole", "schema", "", "public", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allGrantable {
		t.Error("expected allGrantable to be false when one privilege is not grantable")
	}
	if len(privs) != 2 {
		t.Fatalf("expected 2 privileges, got %d", len(privs))
	}
	if privs[0] != "USAGE" || privs[1] != "CREATE" {
		t.Errorf("expected [USAGE, CREATE], got %v", privs)
	}
}

func TestReadPrivileges_Table_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "SELECT"
			*dest[1].(*bool) = true
			return nil
		}),
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "INSERT"
			*dest[1].(*bool) = true
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
	)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	r := &resource.GrantResource{DB: mockDB}
	privs, allGrantable, err := r.ReadPrivileges(context.Background(), "myrole", "table", "", "public", []string{"my_table"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allGrantable {
		t.Error("expected allGrantable to be true")
	}
	if len(privs) != 2 {
		t.Fatalf("expected 2 privileges, got %d", len(privs))
	}
}

func TestReadPrivileges_Sequence_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "USAGE"
			*dest[1].(*bool) = true
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
	)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	r := &resource.GrantResource{DB: mockDB}
	privs, allGrantable, err := r.ReadPrivileges(context.Background(), "myrole", "sequence", "", "public", []string{"my_seq"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allGrantable {
		t.Error("expected allGrantable to be true")
	}
	if len(privs) != 1 || privs[0] != "USAGE" {
		t.Errorf("expected [USAGE], got %v", privs)
	}
}

func TestReadPrivileges_Function_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "EXECUTE"
			*dest[1].(*bool) = false
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
	)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	r := &resource.GrantResource{DB: mockDB}
	privs, allGrantable, err := r.ReadPrivileges(context.Background(), "myrole", "function", "", "public", []string{"my_func"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allGrantable {
		t.Error("expected allGrantable to be false")
	}
	if len(privs) != 1 || privs[0] != "EXECUTE" {
		t.Errorf("expected [EXECUTE], got %v", privs)
	}
}

func TestReadPrivileges_UnsupportedType(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	r := &resource.GrantResource{DB: mockDB}
	privs, allGrantable, err := r.ReadPrivileges(context.Background(), "myrole", "invalid", "", "", nil)
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported object type") {
		t.Errorf("expected 'unsupported object type' in error, got %q", err.Error())
	}
	if privs != nil {
		t.Errorf("expected nil privileges, got %v", privs)
	}
	if allGrantable {
		t.Error("expected allGrantable to be false")
	}
}

func TestReadPrivileges_ScanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).Return(fmt.Errorf("scan error: cannot convert")),
	)
	mockRows.EXPECT().Close().Return(nil)

	r := &resource.GrantResource{DB: mockDB}
	_, _, err := r.ReadPrivileges(context.Background(), "myrole", "database", "mydb", "", nil)
	if err == nil {
		t.Fatal("expected scan error, got nil")
	}
}

// ---------------------------------------------------------------------------
// buildGrantStatements tests
// ---------------------------------------------------------------------------

func TestBuildGrantStatements_Database(t *testing.T) {
	stmts := resource.BuildGrantStatements("CONNECT, CREATE", "database", "mydb", "", "myrole", nil, "")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "GRANT CONNECT, CREATE ON DATABASE") {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `"mydb"`) {
		t.Errorf("expected quoted database name in statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `TO "myrole"`) {
		t.Errorf("expected TO quoted role in statement: %s", stmts[0])
	}
}

func TestBuildGrantStatements_Schema(t *testing.T) {
	stmts := resource.BuildGrantStatements("USAGE, CREATE", "schema", "", "myschema", "myrole", nil, "")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "GRANT USAGE, CREATE ON SCHEMA") {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `"myschema"`) {
		t.Errorf("expected quoted schema name in statement: %s", stmts[0])
	}
}

func TestBuildGrantStatements_TableAll(t *testing.T) {
	stmts := resource.BuildGrantStatements("SELECT", "table", "", "myschema", "myrole", nil, "")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL TABLES IN SCHEMA") {
		t.Errorf("expected ALL TABLES IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildGrantStatements_TableSpecific(t *testing.T) {
	stmts := resource.BuildGrantStatements("SELECT", "table", "", "myschema", "myrole", []string{"t1", "t2"}, "")
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON TABLE "myschema"."t1"`) {
		t.Errorf("expected first statement to reference t1: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], `ON TABLE "myschema"."t2"`) {
		t.Errorf("expected second statement to reference t2: %s", stmts[1])
	}
}

func TestBuildGrantStatements_WithGrantOption(t *testing.T) {
	stmts := resource.BuildGrantStatements("SELECT", "database", "mydb", "", "myrole", nil, " WITH GRANT OPTION")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.HasSuffix(stmts[0], " WITH GRANT OPTION") {
		t.Errorf("expected statement to end with WITH GRANT OPTION: %s", stmts[0])
	}
}

func TestBuildGrantStatements_SequenceAll(t *testing.T) {
	stmts := resource.BuildGrantStatements("USAGE", "sequence", "", "myschema", "myrole", nil, "")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL SEQUENCES IN SCHEMA") {
		t.Errorf("expected ALL SEQUENCES IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildGrantStatements_SequenceSpecific(t *testing.T) {
	stmts := resource.BuildGrantStatements("USAGE", "sequence", "", "myschema", "myrole", []string{"seq1", "seq2"}, "")
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON SEQUENCE "myschema"."seq1"`) {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], `ON SEQUENCE "myschema"."seq2"`) {
		t.Errorf("unexpected statement: %s", stmts[1])
	}
}

func TestBuildGrantStatements_FunctionAll(t *testing.T) {
	stmts := resource.BuildGrantStatements("EXECUTE", "function", "", "myschema", "myrole", nil, "")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL FUNCTIONS IN SCHEMA") {
		t.Errorf("expected ALL FUNCTIONS IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildGrantStatements_FunctionSpecific(t *testing.T) {
	stmts := resource.BuildGrantStatements("EXECUTE", "function", "", "myschema", "myrole", []string{"fn1", "fn2"}, "")
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON FUNCTION "myschema"."fn1"`) {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], `ON FUNCTION "myschema"."fn2"`) {
		t.Errorf("unexpected statement: %s", stmts[1])
	}
}

// ---------------------------------------------------------------------------
// buildRevokeStatements tests
// ---------------------------------------------------------------------------

func TestBuildRevokeStatements_Database(t *testing.T) {
	stmts := resource.BuildRevokeStatements("database", "mydb", "", "myrole", nil)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "REVOKE ALL PRIVILEGES ON DATABASE") {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `FROM "myrole"`) {
		t.Errorf("expected FROM quoted role in statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_Schema(t *testing.T) {
	stmts := resource.BuildRevokeStatements("schema", "", "myschema", "myrole", nil)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "REVOKE ALL PRIVILEGES ON SCHEMA") {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `"myschema"`) {
		t.Errorf("expected quoted schema in statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_TableAll(t *testing.T) {
	stmts := resource.BuildRevokeStatements("table", "", "myschema", "myrole", nil)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL TABLES IN SCHEMA") {
		t.Errorf("expected ALL TABLES IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_TableSpecific(t *testing.T) {
	stmts := resource.BuildRevokeStatements("table", "", "myschema", "myrole", []string{"t1", "t2"})
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON TABLE "myschema"."t1"`) {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], `ON TABLE "myschema"."t2"`) {
		t.Errorf("unexpected statement: %s", stmts[1])
	}
}

func TestBuildRevokeStatements_SequenceAll(t *testing.T) {
	stmts := resource.BuildRevokeStatements("sequence", "", "myschema", "myrole", nil)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL SEQUENCES IN SCHEMA") {
		t.Errorf("expected ALL SEQUENCES IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_SequenceSpecific(t *testing.T) {
	stmts := resource.BuildRevokeStatements("sequence", "", "myschema", "myrole", []string{"seq1"})
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON SEQUENCE "myschema"."seq1"`) {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_FunctionAll(t *testing.T) {
	stmts := resource.BuildRevokeStatements("function", "", "myschema", "myrole", nil)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL FUNCTIONS IN SCHEMA") {
		t.Errorf("expected ALL FUNCTIONS IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_FunctionSpecific(t *testing.T) {
	stmts := resource.BuildRevokeStatements("function", "", "myschema", "myrole", []string{"fn1", "fn2"})
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON FUNCTION "myschema"."fn1"`) {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], `ON FUNCTION "myschema"."fn2"`) {
		t.Errorf("unexpected statement: %s", stmts[1])
	}
}
