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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lib/pq"
	"go.uber.org/mock/gomock"
)

// ---------------------------------------------------------------------------
// readRole tests
// ---------------------------------------------------------------------------

func TestReadRole_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sql.ErrNoRows)

	r := &resource.RoleResource{DB: mockDB}
	model := &resource.RoleResourceModel{Name: types.StringValue("ghost")}

	found, diags := r.ReadRole(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("expected no error for a missing role, got %v", diags)
	}
	if found {
		t.Error("expected found to be false")
	}
	if !model.OID.IsNull() {
		t.Errorf("expected the model untouched, got OID %v", model.OID)
	}
}

func TestReadRole_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection refused"))

	r := &resource.RoleResource{DB: mockDB}
	model := &resource.RoleResourceModel{Name: types.StringValue("testrole")}

	_, diags := r.ReadRole(context.Background(), model)
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

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 99
		*dest[1].(*bool) = false
		*dest[2].(*bool) = true
		*dest[3].(*bool) = false
		*dest[4].(*bool) = true
		*dest[5].(*bool) = false
		*dest[6].(*int64) = 10
		*dest[7].(*sql.NullString) = sql.NullString{Valid: false}
		return nil
	})

	r := &resource.RoleResource{DB: mockDB}
	model := &resource.RoleResourceModel{Name: types.StringValue("testrole")}

	found, diags := r.ReadRole(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !found {
		t.Fatal("expected found to be true")
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

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sql.ErrNoRows)

	r := &resource.UserResource{DB: mockDB}
	model := &resource.UserResourceModel{Name: types.StringValue("ghost")}

	found, diags := r.ReadUser(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("expected no error for a missing user, got %v", diags)
	}
	if found {
		t.Error("expected found to be false")
	}
	if !model.OID.IsNull() {
		t.Errorf("expected the model untouched, got OID %v", model.OID)
	}
}

func TestReadUser_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("connection refused"))

	r := &resource.UserResource{DB: mockDB}
	model := &resource.UserResourceModel{Name: types.StringValue("testuser")}

	_, diags := r.ReadUser(context.Background(), model)
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

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
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

	_, diags := r.ReadUser(context.Background(), model)
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

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
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

	found, diags := r.ReadUser(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !found {
		t.Fatal("expected found to be true")
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

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
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

	found, diags := r.ReadUser(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !found {
		t.Fatal("expected found to be true")
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

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
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

	found, diags := r.ReadUser(context.Background(), model)
	// Should not have errors, but should have a warning
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !found {
		t.Fatal("expected found to be true")
	}

	warned := false
	for _, d := range diags {
		if d.Summary() == "Role is not a user" {
			warned = true
			break
		}
	}
	if !warned {
		t.Error("expected 'Role is not a user' warning diagnostic")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}
}

// ---------------------------------------------------------------------------
// buildUserOptions tests
// ---------------------------------------------------------------------------

func userModel(superuser, createDB, createRole, replication bool, password, validUntil types.String) *resource.UserResourceModel {
	return &resource.UserResourceModel{
		Superuser:       types.BoolValue(superuser),
		CreateDatabase:  types.BoolValue(createDB),
		CreateRole:      types.BoolValue(createRole),
		Replication:     types.BoolValue(replication),
		ConnectionLimit: types.Int64Value(-1),
		Password:        password,
		ValidUntil:      validUntil,
	}
}

func TestBuildUserOptions_CreateDefaults(t *testing.T) {
	r := &resource.UserResource{}
	plan := userModel(false, false, false, false, types.StringNull(), types.StringNull())

	result := r.BuildUserOptions(context.Background(), plan, nil)

	// False flags are omitted on create: since PostgreSQL 16 naming NOSUPERUSER needs SUPERUSER.
	if result != " WITH CONNECTION LIMIT -1" {
		t.Errorf("expected only CONNECTION LIMIT, got %q", result)
	}
}

func TestBuildUserOptions_CreateAllEnabled(t *testing.T) {
	r := &resource.UserResource{}
	plan := userModel(true, true, true, true, types.StringValue("s3cret"), types.StringValue("2025-12-31T23:59:59Z"))
	plan.ConnectionLimit = types.Int64Value(50)

	result := r.BuildUserOptions(context.Background(), plan, nil)

	want := " WITH SUPERUSER CREATEDB CREATEROLE REPLICATION CONNECTION LIMIT 50 PASSWORD 's3cret' VALID UNTIL '2025-12-31T23:59:59Z'"
	if result != want {
		t.Errorf("expected %q, got %q", want, result)
	}
}

func TestBuildUserOptions_PasswordIncluded(t *testing.T) {
	r := &resource.UserResource{}
	plan := userModel(false, false, false, false, types.StringValue("mypass"), types.StringNull())

	result := r.BuildUserOptions(context.Background(), plan, nil)

	if !strings.Contains(result, "PASSWORD 'mypass'") {
		t.Errorf("expected PASSWORD 'mypass' in result %q", result)
	}
}

func TestBuildUserOptions_Update_TableDriven(t *testing.T) {
	null := types.StringNull()
	tests := []struct {
		name    string
		plan    *resource.UserResourceModel
		state   *resource.UserResourceModel
		want    []string
		notWant []string
	}{
		{
			name:    "unchanged sends no flags",
			plan:    userModel(true, false, false, false, null, null),
			state:   userModel(true, false, false, false, null, null),
			want:    []string{"CONNECTION LIMIT -1"},
			notWant: []string{"SUPERUSER", "CREATEDB", "PASSWORD", "VALID UNTIL"},
		},
		{
			name:    "flag turned off",
			plan:    userModel(false, true, false, false, null, null),
			state:   userModel(true, true, false, false, null, null),
			want:    []string{"NOSUPERUSER"},
			notWant: []string{"CREATEDB", "REPLICATION"},
		},
		{
			name:  "flags turned on",
			plan:  userModel(false, false, true, true, null, null),
			state: userModel(false, false, false, false, null, null),
			want:  []string{"CREATEROLE", "REPLICATION"},
		},
		{
			name:    "removed password is cleared",
			plan:    userModel(false, false, false, false, null, null),
			state:   userModel(false, false, false, false, types.StringValue("old"), null),
			want:    []string{"PASSWORD NULL"},
			notWant: []string{"'old'"},
		},
		{
			name:    "changed password is set",
			plan:    userModel(false, false, false, false, types.StringValue("new"), null),
			state:   userModel(false, false, false, false, types.StringValue("old"), null),
			want:    []string{"PASSWORD 'new'"},
			notWant: []string{"PASSWORD NULL"},
		},
		{
			name:  "removed expiry is cleared",
			plan:  userModel(false, false, false, false, null, null),
			state: userModel(false, false, false, false, null, types.StringValue("2030-01-01T00:00:00Z")),
			want:  []string{"VALID UNTIL 'infinity'"},
		},
		{
			name:    "changed expiry is set",
			plan:    userModel(false, false, false, false, null, types.StringValue("2031-01-01T00:00:00Z")),
			state:   userModel(false, false, false, false, null, types.StringValue("2030-01-01T00:00:00Z")),
			want:    []string{"VALID UNTIL '2031-01-01T00:00:00Z'"},
			notWant: []string{"infinity"},
		},
	}
	r := &resource.UserResource{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := r.BuildUserOptions(context.Background(), tc.plan, tc.state)
			if !strings.HasPrefix(result, " WITH ") {
				t.Errorf("expected ' WITH ' prefix, got %q", result)
			}
			for _, w := range tc.want {
				if !strings.Contains(result, w) {
					t.Errorf("expected %q in %q", w, result)
				}
			}
			for _, w := range tc.notWant {
				if strings.Contains(result, w) {
					t.Errorf("did not expect %q in %q", w, result)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// user helper tests
// ---------------------------------------------------------------------------

func TestReassignOwned(t *testing.T) {
	if got := resource.ReassignOwned("app", types.StringNull()); got != nil {
		t.Errorf("expected nothing when unset, got %v", got)
	}
	if got := resource.ReassignOwned("app", types.StringValue("")); got != nil {
		t.Errorf("expected nothing when empty, got %v", got)
	}
	want := []string{`REASSIGN OWNED BY "app" TO "postgres"`, `DROP OWNED BY "app"`}
	if got := resource.ReassignOwned("app", types.StringValue("postgres")); !slices.Equal(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestDropRoleError(t *testing.T) {
	plain := resource.DropRoleError("app", fmt.Errorf("permission denied"))
	if plain != "Could not drop app: permission denied" {
		t.Errorf("unexpected message %q", plain)
	}
	dependent := resource.DropRoleError("app", &pq.Error{Code: "2BP01", Message: "role cannot be dropped"})
	if !strings.Contains(dependent, "reassign_owned_to") {
		t.Errorf("expected the reassign_owned_to hint, got %q", dependent)
	}
	other := resource.DropRoleError("app", &pq.Error{Code: "42704", Message: "role does not exist"})
	if strings.Contains(other, "reassign_owned_to") {
		t.Errorf("expected no hint for another code, got %q", other)
	}
}

func TestSameMembers_TableDriven(t *testing.T) {
	vals := func(names ...string) []attr.Value {
		out := make([]attr.Value, len(names))
		for i, n := range names {
			out[i] = types.StringValue(n)
		}
		return out
	}
	tests := []struct {
		name   string
		names  []string
		values []attr.Value
		want   bool
	}{
		{"same order", []string{"a", "b"}, vals("a", "b"), true},
		{"other order", []string{"b", "a"}, vals("a", "b"), true},
		{"different length", []string{"a"}, vals("a", "b"), false},
		{"different member", []string{"a", "c"}, vals("a", "b"), false},
		{"not a string", []string{"a"}, []attr.Value{types.Int64Value(1)}, false},
		{"both empty", nil, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resource.SameMembers(tc.names, tc.values); got != tc.want {
				t.Errorf("SameMembers(%v, %v) = %v, want %v", tc.names, tc.values, got, tc.want)
			}
		})
	}
}

func TestSameInstant_TableDriven(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"2030-01-01T00:00:00Z", "2030-01-01T00:00:00Z", true},
		{"2030-01-01T00:00:00Z", "2030-01-01 00:00:00+00", true},
		{"2030-01-01T03:00:00+03:00", "2030-01-01 00:00:00+00", true},
		{"2029-12-31T21:00:00-03:00", "2030-01-01 00:00:00+00:00", true},
		{"2030-01-01T00:00:00.5Z", "2030-01-01 00:00:00.5+00", true},
		{"2030-01-01T00:00:01Z", "2030-01-01 00:00:00+00", false},
		{"2030-01-01T00:00:00+01", "2030-01-01 00:00:00+00", false},
		{"tomorrow", "2030-01-01 00:00:00+00", false},
		{"2030-01-01T00:00:00Z", "not a date", false},
		{"", "2030-01-01 00:00:00+00", false},
	}
	for _, tc := range tests {
		t.Run(tc.a+"|"+tc.b, func(t *testing.T) {
			if got := resource.SameInstant(tc.a, tc.b); got != tc.want {
				t.Errorf("SameInstant(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// readDatabase tests
// ---------------------------------------------------------------------------

func TestReadDatabase_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sql.ErrNoRows)

	r := &resource.DatabaseResource{DB: mockDB}
	model := &resource.DatabaseResourceModel{Name: types.StringValue("testdb")}

	found, diags := r.ReadDatabase(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("expected no error for a missing database, got %v", diags)
	}
	if found {
		t.Error("expected found to be false")
	}
	if !model.OID.IsNull() {
		t.Errorf("expected the model untouched, got OID %v", model.OID)
	}
}

func TestReadDatabase_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("disk I/O error"))

	r := &resource.DatabaseResource{DB: mockDB}
	model := &resource.DatabaseResourceModel{Name: types.StringValue("testdb")}

	_, diags := r.ReadDatabase(context.Background(), model)
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

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 16384
		*dest[1].(*string) = "postgres"
		*dest[2].(*string) = "UTF8"
		*dest[3].(*bool) = false
		*dest[4].(*string) = "en_US.UTF-8"
		*dest[5].(*string) = "en_US.UTF-8"
		*dest[6].(*bool) = true
		*dest[7].(*int64) = -1
		*dest[8].(*bool) = false
		*dest[9].(*string) = "pg_default"
		return nil
	})

	r := &resource.DatabaseResource{DB: mockDB}
	model := &resource.DatabaseResourceModel{
		Name:     types.StringValue("mydb"),
		Template: types.StringNull(),
	}

	found, diags := r.ReadDatabase(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !found {
		t.Fatal("expected found to be true")
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
	// The server does not record the template, so an imported one stays null.
	if !model.Template.IsNull() {
		t.Errorf("expected template to stay null, got %q", model.Template.ValueString())
	}
	if model.ForceDrop.IsNull() || model.ForceDrop.ValueBool() {
		t.Errorf("expected force_drop false, got %v", model.ForceDrop)
	}
}

func TestReadDatabase_KeepsMatchingEncodingSpelling(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), "mydb", "utf8").Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 16384
		*dest[1].(*string) = "postgres"
		*dest[2].(*string) = "UTF8"
		*dest[3].(*bool) = true
		*dest[4].(*string) = "C"
		*dest[5].(*string) = "C"
		*dest[6].(*bool) = true
		*dest[7].(*int64) = -1
		*dest[8].(*bool) = false
		*dest[9].(*string) = "pg_default"
		return nil
	})

	r := &resource.DatabaseResource{DB: mockDB}
	model := &resource.DatabaseResourceModel{
		Name:      types.StringValue("mydb"),
		Encoding:  types.StringValue("utf8"),
		Template:  types.StringUnknown(),
		ForceDrop: types.BoolValue(true),
	}

	found, diags := r.ReadDatabase(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !found {
		t.Fatal("expected found to be true")
	}
	if model.Encoding.ValueString() != "utf8" {
		t.Errorf("expected configured encoding 'utf8' kept, got %q", model.Encoding.ValueString())
	}
	if !model.Template.IsNull() {
		t.Errorf("expected unknown template to become null, got %v", model.Template)
	}
	if !model.ForceDrop.ValueBool() {
		t.Error("expected force_drop true to be kept")
	}
}

func TestReadDatabase_PreservesExistingTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 16384
		*dest[1].(*string) = "postgres"
		*dest[2].(*string) = "UTF8"
		*dest[3].(*bool) = false
		*dest[4].(*string) = "en_US.UTF-8"
		*dest[5].(*string) = "en_US.UTF-8"
		*dest[6].(*bool) = true
		*dest[7].(*int64) = -1
		*dest[8].(*bool) = false
		*dest[9].(*string) = "pg_default"
		return nil
	})

	r := &resource.DatabaseResource{DB: mockDB}
	model := &resource.DatabaseResourceModel{
		Name:     types.StringValue("mydb"),
		Template: types.StringValue("template1"),
	}

	found, diags := r.ReadDatabase(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !found {
		t.Fatal("expected found to be true")
	}

	// Template should be preserved when already set
	if model.Template.ValueString() != "template1" {
		t.Errorf("expected template 'template1' to be preserved, got %q", model.Template.ValueString())
	}
}

// ---------------------------------------------------------------------------
// buildRoleOptions tests
// ---------------------------------------------------------------------------

func TestBuildRoleOptions_CreateDefaults(t *testing.T) {
	r := &resource.RoleResource{}
	model := &resource.RoleResourceModel{
		Superuser:       types.BoolValue(false),
		CreateDatabase:  types.BoolValue(false),
		CreateRole:      types.BoolValue(false),
		Replication:     types.BoolValue(false),
		ConnectionLimit: types.Int64Value(-1),
	}

	result := r.BuildRoleOptions(context.Background(), model, nil)

	if result != " WITH NOLOGIN CONNECTION LIMIT -1" {
		t.Errorf("expected NOLOGIN and CONNECTION LIMIT only, got %q", result)
	}
}

func TestBuildRoleOptions_CreateAllEnabled(t *testing.T) {
	r := &resource.RoleResource{}
	model := &resource.RoleResourceModel{
		Superuser:       types.BoolValue(true),
		CreateDatabase:  types.BoolValue(true),
		CreateRole:      types.BoolValue(true),
		Replication:     types.BoolValue(true),
		ConnectionLimit: types.Int64Value(50),
	}

	result := r.BuildRoleOptions(context.Background(), model, nil)

	want := " WITH NOLOGIN SUPERUSER CREATEDB CREATEROLE REPLICATION CONNECTION LIMIT 50"
	if result != want {
		t.Errorf("expected %q, got %q", want, result)
	}
}

func TestBuildRoleOptions_UpdateSendsChangesOnly(t *testing.T) {
	r := &resource.RoleResource{}
	state := &resource.RoleResourceModel{
		Superuser:       types.BoolValue(false),
		CreateDatabase:  types.BoolValue(true),
		CreateRole:      types.BoolValue(false),
		Replication:     types.BoolValue(false),
		ConnectionLimit: types.Int64Value(-1),
	}
	plan := &resource.RoleResourceModel{
		Superuser:       types.BoolValue(false),
		CreateDatabase:  types.BoolValue(false),
		CreateRole:      types.BoolValue(true),
		Replication:     types.BoolValue(false),
		ConnectionLimit: types.Int64Value(5),
	}

	result := r.BuildRoleOptions(context.Background(), plan, state)

	// No LOGIN keyword on alter, and unchanged flags are not named.
	want := " WITH NOCREATEDB CREATEROLE CONNECTION LIMIT 5"
	if result != want {
		t.Errorf("expected %q, got %q", want, result)
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
// ReadGrantedPrivileges tests
// ---------------------------------------------------------------------------

// aclRow is one row of the ACL query; a nil privilege is an object the role holds nothing on.
type aclRow struct {
	object    string
	privilege any
	grantable bool
}

func aclRowsMock(ctrl *gomock.Controller, rows ...aclRow) *mocks.MockRows {
	m := mocks.NewMockRows(ctrl)
	var calls []any
	for _, r := range rows {
		calls = append(calls,
			m.EXPECT().Next().Return(true),
			m.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
				*dest[0].(*string) = r.object
				if p, ok := r.privilege.(string); ok {
					*dest[1].(*sql.NullString) = sql.NullString{String: p, Valid: true}
					*dest[2].(*sql.NullBool) = sql.NullBool{Bool: r.grantable, Valid: true}
				}
				return nil
			}))
	}
	calls = append(calls, m.EXPECT().Next().Return(false), m.EXPECT().Err().Return(nil))
	gomock.InOrder(calls...)
	m.EXPECT().Close().Return(nil)
	return m
}

func stringRowsMock(ctrl *gomock.Controller, values ...string) *mocks.MockRows {
	m := mocks.NewMockRows(ctrl)
	var calls []any
	for _, v := range values {
		calls = append(calls,
			m.EXPECT().Next().Return(true),
			m.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
				*dest[0].(*string) = v
				return nil
			}))
	}
	calls = append(calls, m.EXPECT().Next().Return(false), m.EXPECT().Err().Return(nil))
	gomock.InOrder(calls...)
	m.EXPECT().Close().Return(nil)
	return m
}

func expectSchemaExists(ctrl *gomock.Controller, db *mocks.MockDBTX, schemaName string, exists bool) {
	scanner := mocks.NewMockScanner(ctrl)
	db.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), schemaName).Return(scanner)
	scanner.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*bool) = exists
		return nil
	})
}

func TestReadGrantedPrivileges_Database_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	rows := aclRowsMock(ctrl,
		aclRow{"mydb", "TEMPORARY", true},
		aclRow{"mydb", "CONNECT", true},
		aclRow{"mydb", "CREATE", true},
	)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "mydb", "myrole").Return(rows, nil)

	got, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "database", "mydb", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.TargetExists || got.Objects != 1 {
		t.Errorf("expected an existing target with 1 object, got %+v", got)
	}
	if !slices.Equal(got.Privileges, []string{"CONNECT", "CREATE", "TEMPORARY"}) {
		t.Errorf("expected sorted [CONNECT CREATE TEMPORARY], got %v", got.Privileges)
	}
	if !got.Grantable {
		t.Error("expected grantable when every privilege is held with grant option")
	}
}

func TestReadGrantedPrivileges_Schema_PartlyGrantable(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	rows := aclRowsMock(ctrl,
		aclRow{"public", "USAGE", true},
		aclRow{"public", "CREATE", false},
	)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "myrole").Return(rows, nil)

	got, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "schema", "", "public", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(got.Privileges, []string{"CREATE", "USAGE"}) {
		t.Errorf("expected [CREATE USAGE], got %v", got.Privileges)
	}
	if got.Grantable {
		t.Error("expected not grantable when one privilege lacks grant option")
	}
}

func TestReadGrantedPrivileges_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("connection refused"))

	got, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "database", "mydb", "", nil)
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("expected 'connection refused', got %v", err)
	}
	if got.TargetExists || got.Privileges != nil {
		t.Errorf("expected zero result on error, got %+v", got)
	}
}

func TestReadGrantedPrivileges_ScanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("cannot convert")),
	)
	mockRows.EXPECT().Close().Return(nil)

	_, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "database", "mydb", "", nil)
	if err == nil || !strings.Contains(err.Error(), "cannot convert") {
		t.Fatalf("expected scan error, got %v", err)
	}
}

func TestReadGrantedPrivileges_RowsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(fmt.Errorf("connection reset"))
	mockRows.EXPECT().Close().Return(nil)

	_, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "database", "mydb", "", nil)
	if err == nil || !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("expected rows error, got %v", err)
	}
}

func TestReadGrantedPrivileges_UnsupportedType(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	_, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "invalid", "", "", nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported object type") {
		t.Fatalf("expected 'unsupported object type', got %v", err)
	}
}

func TestReadGrantedPrivileges_TargetGone(t *testing.T) {
	t.Run("database", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "mydb", "myrole").Return(aclRowsMock(ctrl), nil)

		got, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "database", "mydb", "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.TargetExists {
			t.Errorf("expected target gone, got %+v", got)
		}
	})
	t.Run("schema of a table grant", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		expectSchemaExists(ctrl, mockDB, "gone", false)

		got, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "table", "", "gone", []string{"t1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.TargetExists {
			t.Errorf("expected target gone, got %+v", got)
		}
	})
}

func TestReadGrantedPrivileges_SchemaExistsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	scanner := mocks.NewMockScanner(ctrl)
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), "public").Return(scanner)
	scanner.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("broken pipe"))

	_, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "sequence", "", "public", nil)
	if err == nil || !strings.Contains(err.Error(), "broken pipe") {
		t.Fatalf("expected schema check error, got %v", err)
	}
}

func TestReadGrantedPrivileges_RoleHoldsNothing(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "mydb", "myrole").Return(aclRowsMock(ctrl, aclRow{object: "mydb"}), nil)

	got, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "database", "mydb", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.TargetExists || got.Objects != 1 || len(got.Privileges) != 0 || got.Grantable {
		t.Errorf("expected an existing target with no privileges, got %+v", got)
	}
}

func TestReadGrantedPrivileges_EmptySchema(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	expectSchemaExists(ctrl, mockDB, "public", true)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "myrole", gomock.Any()).Return(aclRowsMock(ctrl), nil)

	got, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "table", "", "public", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.TargetExists || got.Objects != 0 || got.Privileges != nil {
		t.Errorf("expected an existing schema with no objects, got %+v", got)
	}
}

func TestReadGrantedPrivileges_IntersectionAcrossObjects(t *testing.T) {
	tests := []struct {
		name          string
		rows          []aclRow
		wantPrivs     []string
		wantGrantable bool
	}{
		{
			name: "shared privileges only",
			rows: []aclRow{
				{"t1", "SELECT", true}, {"t1", "INSERT", true},
				{"t2", "SELECT", true}, {"t2", "UPDATE", true},
			},
			wantPrivs:     []string{"SELECT"},
			wantGrantable: true,
		},
		{
			name: "grantable on one object only",
			rows: []aclRow{
				{"t1", "SELECT", true},
				{"t2", "SELECT", false},
			},
			wantPrivs:     []string{"SELECT"},
			wantGrantable: false,
		},
		{
			name: "several grantors, one with grant option",
			rows: []aclRow{
				{"t1", "SELECT", false}, {"t1", "SELECT", true},
				{"t2", "SELECT", true},
			},
			wantPrivs:     []string{"SELECT"},
			wantGrantable: true,
		},
		{
			name: "nothing shared",
			rows: []aclRow{
				{"t1", "SELECT", true},
				{"t2", "INSERT", true},
			},
			wantPrivs:     nil,
			wantGrantable: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mocks.NewMockDBTX(ctrl)
			expectSchemaExists(ctrl, mockDB, "public", true)
			mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "myrole", gomock.Any()).Return(aclRowsMock(ctrl, tc.rows...), nil)

			got, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "table", "", "public", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Objects != 2 || !got.TargetExists {
				t.Errorf("expected 2 objects on an existing target, got %+v", got)
			}
			if !slices.Equal(got.Privileges, tc.wantPrivs) {
				t.Errorf("expected %v, got %v", tc.wantPrivs, got.Privileges)
			}
			if got.Grantable != tc.wantGrantable {
				t.Errorf("expected grantable %v, got %v", tc.wantGrantable, got.Grantable)
			}
		})
	}
}

func TestReadGrantedPrivileges_MissingNamedObject(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	expectSchemaExists(ctrl, mockDB, "public", true)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "myrole", gomock.Any()).Return(aclRowsMock(ctrl, aclRow{"t1", "SELECT", true}), nil)

	got, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "table", "", "public", []string{"t1", "t2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.TargetExists || got.Objects != 2 {
		t.Errorf("expected 2 objects counted, the missing one included, got %+v", got)
	}
	// A missing object holds nothing, so nothing is held on every object.
	if got.Privileges != nil || got.Grantable {
		t.Errorf("expected no privileges, got %+v", got)
	}
}

func TestReadGrantedPrivileges_FunctionsResolved(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	gomock.InOrder(
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "").Return(stringRowsMock(ctrl, "public.f(integer)", "public.f(text)"), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "g", "(integer)").Return(stringRowsMock(ctrl, "public.g(integer)"), nil),
	)
	expectSchemaExists(ctrl, mockDB, "public", true)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "myrole", gomock.Any()).Return(aclRowsMock(ctrl,
		aclRow{"public.f(integer)", "EXECUTE", false},
		aclRow{"public.f(text)", "EXECUTE", false},
		aclRow{"public.g(integer)", "EXECUTE", false},
	), nil)

	got, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "function", "", "public", []string{"f", "g(integer)"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Objects != 3 || !slices.Equal(got.Privileges, []string{"EXECUTE"}) || got.Grantable {
		t.Errorf("expected EXECUTE on 3 overloads, not grantable, got %+v", got)
	}
}

func TestReadGrantedPrivileges_MissingFunction(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	gomock.InOrder(
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "").Return(stringRowsMock(ctrl, "public.f()"), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "gone", "").Return(stringRowsMock(ctrl), nil),
	)
	expectSchemaExists(ctrl, mockDB, "public", true)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "myrole", gomock.Any()).Return(aclRowsMock(ctrl,
		aclRow{"public.f()", "EXECUTE", true},
	), nil)

	got, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "function", "", "public", []string{"f", "gone"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Objects != 2 || got.Privileges != nil {
		t.Errorf("expected the missing function to count and hold nothing, got %+v", got)
	}
}

func TestReadGrantedPrivileges_ResolveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "public", "f", "").Return(nil, fmt.Errorf("timeout"))

	_, err := resource.ReadGrantedPrivileges(context.Background(), mockDB, "myrole", "function", "", "public", []string{"f"})
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected resolve error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FullPrivileges / ExpandPrivileges tests
// ---------------------------------------------------------------------------

func TestFullPrivileges_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "r").Return(stringRowsMock(ctrl, "INSERT", "SELECT"), nil)

	got, err := resource.FullPrivileges(context.Background(), mockDB, "table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(got, []string{"INSERT", "SELECT"}) {
		t.Errorf("expected [INSERT SELECT], got %v", got)
	}
}

func TestFullPrivileges_Errors(t *testing.T) {
	t.Run("query", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "d").Return(nil, fmt.Errorf("boom"))

		if _, err := resource.FullPrivileges(context.Background(), mockDB, "database"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		mockRows := mocks.NewMockRows(ctrl)
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "f").Return(mockRows, nil)
		mockRows.EXPECT().Next().Return(true)
		mockRows.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("bad"))
		mockRows.EXPECT().Close().Return(nil)

		if _, err := resource.FullPrivileges(context.Background(), mockDB, "function"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestExpandPrivileges_TableDriven(t *testing.T) {
	full := []string{"CREATE", "CONNECT", "TEMPORARY"}
	tests := []struct {
		name  string
		privs []string
		want  []string
	}{
		{"ALL expands to full", []string{"ALL"}, []string{"CONNECT", "CREATE", "TEMPORARY"}},
		{"ALL PRIVILEGES expands to full", []string{"all privileges"}, []string{"CONNECT", "CREATE", "TEMPORARY"}},
		{"TEMP is TEMPORARY", []string{"temp"}, []string{"TEMPORARY"}},
		{"case and spaces", []string{" connect ", "Create"}, []string{"CONNECT", "CREATE"}},
		{"dedupe", []string{"CONNECT", "connect", "TEMP", "TEMPORARY", "ALL"}, []string{"CONNECT", "CREATE", "TEMPORARY"}},
		{"empty", nil, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resource.ExpandPrivileges(tc.privs, full)
			if !slices.Equal(got, tc.want) {
				t.Errorf("ExpandPrivileges(%v) = %v, want %v", tc.privs, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveFunctions / GrantObjects tests
// ---------------------------------------------------------------------------

func TestResolveFunctions_BareNameAndSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	gomock.InOrder(
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "f", "").Return(stringRowsMock(ctrl, "s.f(integer)", "s.f(text)"), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "g", "(integer, text)").Return(stringRowsMock(ctrl, "s.g(integer, text)"), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "gone", "").Return(stringRowsMock(ctrl), nil),
	)

	sigs, missing, err := resource.ResolveFunctions(context.Background(), mockDB, "s", []string{"f", " g(integer, text) ", "gone"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(sigs, []string{"s.f(integer)", "s.f(text)", "s.g(integer, text)"}) {
		t.Errorf("unexpected signatures %v", sigs)
	}
	if missing != 1 {
		t.Errorf("expected 1 missing, got %d", missing)
	}
}

func TestResolveFunctions_Errors(t *testing.T) {
	t.Run("query", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "f", "").Return(nil, fmt.Errorf("boom"))

		if _, _, err := resource.ResolveFunctions(context.Background(), mockDB, "s", []string{"f"}); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		mockRows := mocks.NewMockRows(ctrl)
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "f", "").Return(mockRows, nil)
		mockRows.EXPECT().Next().Return(true)
		mockRows.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("bad"))
		mockRows.EXPECT().Close().Return(nil)

		if _, _, err := resource.ResolveFunctions(context.Background(), mockDB, "s", []string{"f"}); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("rows", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		mockRows := mocks.NewMockRows(ctrl)
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "f", "").Return(mockRows, nil)
		mockRows.EXPECT().Next().Return(false)
		mockRows.EXPECT().Err().Return(fmt.Errorf("reset"))
		mockRows.EXPECT().Close().Return(nil)

		if _, _, err := resource.ResolveFunctions(context.Background(), mockDB, "s", []string{"f"}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestGrantObjects_Unchanged(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	got, err := resource.GrantObjects(context.Background(), mockDB, "table", "s", []string{"t1", "t2"})
	if err != nil || !slices.Equal(got, []string{"t1", "t2"}) {
		t.Errorf("expected tables unchanged, got %v, %v", got, err)
	}
	got, err = resource.GrantObjects(context.Background(), mockDB, "function", "s", nil)
	if err != nil || got != nil {
		t.Errorf("expected no functions unchanged, got %v, %v", got, err)
	}
}

func TestGrantObjects_FunctionsResolved(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	gomock.InOrder(
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "f", "").Return(stringRowsMock(ctrl, "s.f(integer)"), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "g", "()").Return(stringRowsMock(ctrl, "s.g()"), nil),
	)

	got, err := resource.GrantObjects(context.Background(), mockDB, "function", "s", []string{"f", "g()"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(got, []string{"s.f(integer)", "s.g()"}) {
		t.Errorf("unexpected signatures %v", got)
	}
}

func TestGrantObjects_MissingFunction(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	// Resolved once together, then once per name to list the missing ones.
	gomock.InOrder(
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "f", "").Return(stringRowsMock(ctrl, "s.f()"), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "nope", "(integer)").Return(stringRowsMock(ctrl), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "f", "").Return(stringRowsMock(ctrl, "s.f()"), nil),
		mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "nope", "(integer)").Return(stringRowsMock(ctrl), nil),
	)

	_, err := resource.GrantObjects(context.Background(), mockDB, "function", "s", []string{"f", "nope(integer)"})
	if err == nil {
		t.Fatal("expected error for a missing function")
	}
	if !strings.Contains(err.Error(), "nope(integer)") || strings.Contains(err.Error(), "f,") {
		t.Errorf("expected only the missing function named, got %q", err.Error())
	}
}

func TestGrantObjects_ResolveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "s", "f", "").Return(nil, fmt.Errorf("boom"))

	if _, err := resource.GrantObjects(context.Background(), mockDB, "function", "s", []string{"f"}); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// GrantConn tests
// ---------------------------------------------------------------------------

func TestGrantConn(t *testing.T) {
	w := &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, fmt.Errorf("refused")
	}}
	got, err := resource.GrantConn(context.Background(), w, "database", "other")
	if err != nil || got != w {
		t.Errorf("expected database grants on the provider's connection, got %v, %v", got, err)
	}
	if _, err := resource.GrantConn(context.Background(), w, "table", "other"); err == nil {
		t.Error("expected table grants to connect to their database")
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
	stmts := resource.BuildGrantStatements("EXECUTE", "function", "", "myschema", "myrole", []string{"myschema.fn1(integer)", "myschema.fn2()"}, "")
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ON ROUTINE myschema.fn1(integer) ") {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], "ON ROUTINE myschema.fn2() ") {
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
	stmts := resource.BuildRevokeStatements("function", "", "myschema", "myrole", []string{"myschema.fn1(integer)", "myschema.fn2()"})
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ON ROUTINE myschema.fn1(integer) ") {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], "ON ROUTINE myschema.fn2() ") {
		t.Errorf("unexpected statement: %s", stmts[1])
	}
}
