package resource_test

import (
	"context"
	"strings"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/resource"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestBuildGrantStatements_UnknownObjectType verifies that an unrecognized
// object type yields no statements. Defensive: the validator should already
// reject these, but the builder must not panic or emit invalid SQL.
func TestBuildGrantStatements_UnknownObjectType(t *testing.T) {
	stmts := resource.BuildGrantStatements("SELECT", "table_but_misspelled", "", "public", "r", []string{"t1"}, "")
	if len(stmts) != 0 {
		t.Errorf("expected 0 statements for unknown type, got %d: %v", len(stmts), stmts)
	}
}

func TestBuildRevokeStatements_UnknownObjectType(t *testing.T) {
	stmts := resource.BuildRevokeStatements("unknown", "", "public", "r", nil)
	if len(stmts) != 0 {
		t.Errorf("expected 0 statements for unknown type, got %d: %v", len(stmts), stmts)
	}
}

// TestBuildGrantStatements_QuoteInIdentifier ensures pq.QuoteIdentifier
// properly escapes identifiers containing double quotes.
func TestBuildGrantStatements_QuoteInIdentifier(t *testing.T) {
	stmts := resource.BuildGrantStatements("SELECT", "table", "", "pub\"lic", "role", []string{"t\"1"}, "")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	// pq.QuoteIdentifier doubles internal quotes: "pub""lic"."t""1"
	if !strings.Contains(stmts[0], `"pub""lic"."t""1"`) {
		t.Errorf("expected doubled-quote escaping, got %s", stmts[0])
	}
}

// TestBuildGrantStatements_AllObjectTypesTableDriven exercises every object
// type with and without WITH GRANT OPTION in a single concise table.
func TestBuildGrantStatements_AllObjectTypesTableDriven(t *testing.T) {
	tests := []struct {
		name       string
		objectType string
		database   string
		schema     string
		objects    []string
		grantOpt   string
		wantCount  int
		wantSubstr string
	}{
		{"database_no_opt", "database", "mydb", "", nil, "", 1, "ON DATABASE"},
		{"database_with_opt", "database", "mydb", "", nil, " WITH GRANT OPTION", 1, "WITH GRANT OPTION"},
		{"schema_no_opt", "schema", "", "public", nil, "", 1, "ON SCHEMA"},
		{"table_all", "table", "", "public", nil, "", 1, "ALL TABLES IN SCHEMA"},
		{"table_one", "table", "", "public", []string{"t1"}, "", 1, `ON TABLE "public"."t1"`},
		{"table_three", "table", "", "public", []string{"t1", "t2", "t3"}, "", 3, ""},
		{"table_three_with_opt", "table", "", "public", []string{"a", "b", "c"}, " WITH GRANT OPTION", 3, "WITH GRANT OPTION"},
		{"sequence_all", "sequence", "", "public", nil, "", 1, "ALL SEQUENCES IN SCHEMA"},
		{"sequence_one", "sequence", "", "public", []string{"s1"}, "", 1, `ON SEQUENCE "public"."s1"`},
		{"function_all", "function", "", "public", nil, "", 1, "ALL FUNCTIONS IN SCHEMA"},
		{"function_one", "function", "", "public", []string{"f1"}, "", 1, `ON FUNCTION "public"."f1"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stmts := resource.BuildGrantStatements("SELECT", tc.objectType, tc.database, tc.schema, "myrole", tc.objects, tc.grantOpt)
			if len(stmts) != tc.wantCount {
				t.Fatalf("expected %d statements, got %d: %v", tc.wantCount, len(stmts), stmts)
			}
			if tc.wantSubstr == "" {
				return
			}
			// For multi-statement cases, assert at least one contains the substring.
			found := false
			for _, s := range stmts {
				if strings.Contains(s, tc.wantSubstr) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected one of %v to contain %q", stmts, tc.wantSubstr)
			}
		})
	}
}

// TestBuildRevokeStatements_AllObjectTypesTableDriven mirrors the grant table.
func TestBuildRevokeStatements_AllObjectTypesTableDriven(t *testing.T) {
	tests := []struct {
		name       string
		objectType string
		database   string
		schema     string
		objects    []string
		wantCount  int
		wantSubstr string
	}{
		{"database", "database", "mydb", "", nil, 1, "ON DATABASE"},
		{"schema", "schema", "", "public", nil, 1, "ON SCHEMA"},
		{"table_all", "table", "", "public", nil, 1, "ALL TABLES IN SCHEMA"},
		{"table_multiple", "table", "", "public", []string{"t1", "t2"}, 2, ""},
		{"sequence_all", "sequence", "", "public", nil, 1, "ALL SEQUENCES IN SCHEMA"},
		{"sequence_multiple", "sequence", "", "public", []string{"s1", "s2", "s3"}, 3, ""},
		{"function_all", "function", "", "public", nil, 1, "ALL FUNCTIONS IN SCHEMA"},
		{"function_one", "function", "", "public", []string{"f1"}, 1, `ON FUNCTION "public"."f1"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stmts := resource.BuildRevokeStatements(tc.objectType, tc.database, tc.schema, "myrole", tc.objects)
			if len(stmts) != tc.wantCount {
				t.Fatalf("expected %d statements, got %d: %v", tc.wantCount, len(stmts), stmts)
			}
			// Every revoke must contain REVOKE ALL PRIVILEGES and FROM "myrole".
			for _, s := range stmts {
				if !strings.Contains(s, "REVOKE ALL PRIVILEGES") {
					t.Errorf("statement missing REVOKE ALL PRIVILEGES: %s", s)
				}
				if !strings.Contains(s, `FROM "myrole"`) {
					t.Errorf(`statement missing FROM "myrole": %s`, s)
				}
			}
			if tc.wantSubstr == "" {
				return
			}
			found := false
			for _, s := range stmts {
				if strings.Contains(s, tc.wantSubstr) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected one of %v to contain %q", stmts, tc.wantSubstr)
			}
		})
	}
}

// TestDiffRoles_TableDriven consolidates and extends DiffRoles coverage.
func TestDiffRoles_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		oldRoles   []string
		newRoles   []string
		wantGrant  []string
		wantRevoke []string
	}{
		{"both_empty", nil, nil, nil, nil},
		{"grant_all_from_empty", nil, []string{"a", "b"}, []string{"a", "b"}, nil},
		{"revoke_all", []string{"a", "b"}, nil, nil, []string{"a", "b"}},
		{"no_change", []string{"a", "b"}, []string{"a", "b"}, nil, nil},
		{"partial_overlap", []string{"a", "b", "c"}, []string{"b", "c", "d"}, []string{"d"}, []string{"a"}},
		{"reorder_no_change", []string{"a", "b", "c"}, []string{"c", "b", "a"}, nil, nil},
		{"duplicates_in_old", []string{"a", "a", "b"}, []string{"b"}, nil, []string{"a"}},
		{"duplicates_in_new", []string{"a"}, []string{"b", "b", "c"}, []string{"b", "c"}, []string{"a"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			toGrant, toRevoke := resource.DiffRoles(tc.oldRoles, tc.newRoles)
			if !sliceSetEqual(toGrant, tc.wantGrant) {
				t.Errorf("toGrant: expected %v, got %v", tc.wantGrant, toGrant)
			}
			if !sliceSetEqual(toRevoke, tc.wantRevoke) {
				t.Errorf("toRevoke: expected %v, got %v", tc.wantRevoke, toRevoke)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ImportState passthrough tests (database, role, user)
// ---------------------------------------------------------------------------

// importAndAssertName runs ImportState on a resource that uses the passthrough
// pattern (sets name = ID) and asserts that name=want appears in the resulting
// state.
func importAndAssertName(t *testing.T, s rschema.Schema, r fwresource.ResourceWithImportState, id, want string) {
	t.Helper()
	ctx := context.Background()
	tfType := s.Type().TerraformType(ctx)
	req := fwresource.ImportStateRequest{ID: id}
	resp := &fwresource.ImportStateResponse{
		State: tfsdk.State{Raw: tftypes.NewValue(tfType, nil), Schema: s},
	}
	r.ImportState(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ImportState failed: %v", resp.Diagnostics.Errors())
	}
	raw := resp.State.Raw.String()
	if !strings.Contains(raw, `"`+want+`"`) {
		t.Errorf("expected %q in imported state raw, got %s", want, raw)
	}
}

func TestDatabaseResource_ImportState(t *testing.T) {
	s := databaseResourceSchemaForTest()
	r := &resource.DatabaseResource{}
	importAndAssertName(t, s, r, "mydb", "mydb")
}

func TestRoleResource_ImportState(t *testing.T) {
	s := roleResourceSchemaForTest()
	r := &resource.RoleResource{}
	importAndAssertName(t, s, r, "myrole", "myrole")
}

func TestUserResource_ImportState(t *testing.T) {
	s := userResourceSchemaForTest()
	r := &resource.UserResource{}
	importAndAssertName(t, s, r, "myuser", "myuser")
}

// The schema helpers below wrap the resource's own Schema method so we don't
// duplicate attribute definitions here.
func databaseResourceSchemaForTest() rschema.Schema {
	r := &resource.DatabaseResource{}
	resp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, resp)
	return resp.Schema
}

func roleResourceSchemaForTest() rschema.Schema {
	r := &resource.RoleResource{}
	resp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, resp)
	return resp.Schema
}

func userResourceSchemaForTest() rschema.Schema {
	r := &resource.UserResource{}
	resp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, resp)
	return resp.Schema
}

// sliceSetEqual compares two slices ignoring order and duplicates.
func sliceSetEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	m := map[string]int{}
	for _, v := range a {
		m[v]++
	}
	for _, v := range b {
		m[v]--
	}
	for _, c := range m {
		if c != 0 {
			return false
		}
	}
	return true
}
