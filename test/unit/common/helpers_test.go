package common_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	_ "github.com/lib/pq"
)

type noopResult struct{}

func (r noopResult) LastInsertId() (int64, error) { return 0, nil }
func (r noopResult) RowsAffected() (int64, error) { return 0, nil }

func TestConfigureDB_wrongType(t *testing.T) {
	_, err := common.ConfigureDB("not a db")
	if err == nil {
		t.Error("expected error for wrong type")
	}
}

func TestConfigureDB_correct(t *testing.T) {
	wrapper := common.NewDBWrapper(&sql.DB{})
	result, err := common.ConfigureDB(wrapper)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if result != wrapper {
		t.Error("expected same DBWrapper pointer")
	}
}

func TestIsSet_null(t *testing.T) {
	if common.IsSet(nullVal{}) {
		t.Error("expected false for null value")
	}
}

func TestIsSet_set(t *testing.T) {
	if !common.IsSet(setVal{}) {
		t.Error("expected true for set value")
	}
}

func TestStringSetToSlice_null(t *testing.T) {
	ctx := context.Background()
	result := common.StringSetToSlice(ctx, types.SetNull(types.StringType))
	if result != nil {
		t.Errorf("expected nil for null set, got %v", result)
	}
}

func TestStringSetToSlice_valid(t *testing.T) {
	ctx := context.Background()
	set := types.SetValueMust(types.StringType, []attr.Value{
		types.StringValue("alpha"),
		types.StringValue("beta"),
		types.StringValue("gamma"),
	})
	result := common.StringSetToSlice(ctx, set)
	if len(result) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(result))
	}
	found := map[string]bool{}
	for _, v := range result {
		found[v] = true
	}
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !found[want] {
			t.Errorf("expected %q in result, got %v", want, result)
		}
	}
}

func TestStringListToSlice_null(t *testing.T) {
	ctx := context.Background()
	result := common.StringListToSlice(ctx, types.ListNull(types.StringType))
	if result != nil {
		t.Errorf("expected nil for null list, got %v", result)
	}
}

func TestStringListToSlice_valid(t *testing.T) {
	ctx := context.Background()
	list := types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("one"),
		types.StringValue("two"),
		types.StringValue("three"),
	})
	result := common.StringListToSlice(ctx, list)
	if len(result) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(result))
	}
	expected := []string{"one", "two", "three"}
	for i, want := range expected {
		if result[i] != want {
			t.Errorf("index %d: expected %q, got %q", i, want, result[i])
		}
	}
}

func TestPrivilegesToSlice_valid(t *testing.T) {
	ctx := context.Background()
	set := types.SetValueMust(types.StringType, []attr.Value{
		types.StringValue("select"),
		types.StringValue("insert"),
		types.StringValue("update"),
	})
	result := common.PrivilegesToSlice(ctx, set)
	if len(result) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(result))
	}
	found := map[string]bool{}
	for _, v := range result {
		found[v] = true
	}
	for _, want := range []string{"SELECT", "INSERT", "UPDATE"} {
		if !found[want] {
			t.Errorf("expected %q in result, got %v", want, result)
		}
	}
}

func TestPrivilegesToSlice_null(t *testing.T) {
	ctx := context.Background()
	result := common.PrivilegesToSlice(ctx, types.SetNull(types.StringType))
	if result != nil {
		t.Errorf("expected nil for null set, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// DBWrapper / TxWrapper tests
// ---------------------------------------------------------------------------

// fakeDB creates a real sql.DB from a fake driver that always errors.
// This lets us exercise the wrapper methods without needing a real database.
func fakeDB(t *testing.T) *sql.DB {
	t.Helper()
	// sql.Open with an invalid driver still returns a *sql.DB; calls will error.
	db, err := sql.Open("postgres", "host=__fake__ port=0 sslmode=disable connect_timeout=1")
	if err != nil {
		t.Fatalf("failed to create fake db: %v", err)
	}
	return db
}

func TestDBWrapper_ExecContext(t *testing.T) {
	w := common.NewDBWrapper(fakeDB(t))
	_, err := w.ExecContext(context.Background(), "SELECT 1")
	// Error expected (fake db), but no panic
	if err == nil {
		t.Log("no error from fake db (unexpected but not a failure)")
	}
}

func TestDBWrapper_QueryContext(t *testing.T) {
	w := common.NewDBWrapper(fakeDB(t))
	_, err := w.QueryContext(context.Background(), "SELECT 1")
	if err == nil {
		t.Log("no error from fake db")
	}
}

func TestDBWrapper_QueryRowContext(t *testing.T) {
	w := common.NewDBWrapper(fakeDB(t))
	scanner := w.QueryRowContext(context.Background(), "SELECT 1")
	if scanner == nil {
		t.Fatal("expected non-nil scanner")
	}
	// Scan will fail because there's no real connection
	var v int
	err := scanner.Scan(&v)
	if err == nil {
		t.Log("no error from fake db scan")
	}
}

func TestDBWrapper_BeginTx(t *testing.T) {
	w := common.NewDBWrapper(fakeDB(t))
	_, err := w.BeginTx(context.Background(), nil)
	// Error expected (can't connect)
	if err == nil {
		t.Log("no error from fake db begin")
	}
}

func TestTxWrapper_Methods(t *testing.T) {
	// To test TxWrapper we need a real tx, which requires a real db.
	// Instead, verify compile-time interface satisfaction (already done in helpers.go)
	// and test the wrapper constructor indirectly through DBWrapper.BeginTx.
	// The TxWrapper methods are tested via integration tests.
	// Here we just verify the types are correct.
	var _ common.Tx = (*common.TxWrapper)(nil)
}

type nullVal struct{}

func (n nullVal) IsNull() bool    { return true }
func (n nullVal) IsUnknown() bool { return false }

type setVal struct{}

func (s setVal) IsNull() bool    { return false }
func (s setVal) IsUnknown() bool { return false }

// ---------------------------------------------------------------------------
// QuoteConnStringValue tests
// ---------------------------------------------------------------------------

func TestQuoteConnStringValue_simple(t *testing.T) {
	got := common.QuoteConnStringValue("localhost")
	if got != "'localhost'" {
		t.Errorf("expected 'localhost', got %q", got)
	}
}

func TestQuoteConnStringValue_empty(t *testing.T) {
	got := common.QuoteConnStringValue("")
	if got != "''" {
		t.Errorf("expected '', got %q", got)
	}
}

func TestQuoteConnStringValue_withSpace(t *testing.T) {
	got := common.QuoteConnStringValue("a value")
	if got != "'a value'" {
		t.Errorf("expected 'a value', got %q", got)
	}
}

func TestQuoteConnStringValue_withSingleQuote(t *testing.T) {
	got := common.QuoteConnStringValue("pa'ss")
	if got != `'pa\'ss'` {
		t.Errorf(`expected 'pa\'ss', got %q`, got)
	}
}

func TestQuoteConnStringValue_withBackslash(t *testing.T) {
	got := common.QuoteConnStringValue(`foo\bar`)
	if got != `'foo\\bar'` {
		t.Errorf(`expected 'foo\\bar', got %q`, got)
	}
}

func TestQuoteConnStringValue_withBackslashAndQuote(t *testing.T) {
	// Backslash must be escaped before quote so we don't double-escape.
	got := common.QuoteConnStringValue(`a\'b`)
	if got != `'a\\\'b'` {
		t.Errorf(`expected 'a\\\'b', got %q`, got)
	}
}

// ---------------------------------------------------------------------------
// NormalizePrivileges tests
// ---------------------------------------------------------------------------

func TestNormalizePrivileges_uppercase(t *testing.T) {
	got, err := common.NormalizePrivileges([]string{"select", "Insert", "UPDATE"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"SELECT", "INSERT", "UPDATE"}
	if len(got) != len(want) {
		t.Fatalf("expected %d privileges, got %d", len(want), len(got))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("index %d: expected %q, got %q", i, w, got[i])
		}
	}
}

func TestNormalizePrivileges_dedup(t *testing.T) {
	got, err := common.NormalizePrivileges([]string{"select", "SELECT", "Select"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "SELECT" {
		t.Errorf("expected single SELECT, got %v", got)
	}
}

func TestNormalizePrivileges_trim(t *testing.T) {
	got, err := common.NormalizePrivileges([]string{"  select ", "\tINSERT\n"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 privileges, got %d", len(got))
	}
}

func TestNormalizePrivileges_skipEmpty(t *testing.T) {
	got, err := common.NormalizePrivileges([]string{"", "SELECT", "   "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "SELECT" {
		t.Errorf("expected single SELECT, got %v", got)
	}
}

func TestNormalizePrivileges_invalidRejected(t *testing.T) {
	_, err := common.NormalizePrivileges([]string{"SELECT", "DROP DATABASE"})
	if err == nil {
		t.Fatal("expected error for invalid privilege")
	}
	if !contains(err.Error(), "DROP DATABASE") {
		t.Errorf("expected error to mention invalid privilege, got %q", err.Error())
	}
}

func TestNormalizePrivileges_allValid(t *testing.T) {
	valid := []string{"ALL", "SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE",
		"REFERENCES", "TRIGGER", "USAGE", "CREATE", "CONNECT", "TEMPORARY", "TEMP", "EXECUTE"}
	got, err := common.NormalizePrivileges(valid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(valid) {
		t.Errorf("expected %d privileges, got %d", len(valid), len(got))
	}
}

func TestNormalizePrivileges_empty(t *testing.T) {
	got, err := common.NormalizePrivileges(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestNormalizePrivileges_multipleInvalid(t *testing.T) {
	_, err := common.NormalizePrivileges([]string{"FOO", "BAR", "SELECT"})
	if err == nil {
		t.Fatal("expected error for invalid privileges")
	}
	// Bad names are alphabetized in the error message.
	if !contains(err.Error(), "BAR") || !contains(err.Error(), "FOO") {
		t.Errorf("expected error to list both invalid privileges, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// BuildRoleOptions tests
// ---------------------------------------------------------------------------

func TestBuildRoleOptions_minimalRole(t *testing.T) {
	noLogin := false
	got := common.BuildRoleOptions(common.RoleOptions{
		Login:           &noLogin,
		ConnectionLimit: -1,
	})
	if got != " WITH NOLOGIN CONNECTION LIMIT -1" {
		t.Errorf("expected nil flags omitted, got %q", got)
	}
}

func TestBuildRoleOptions_emptyStillHasConnectionLimit(t *testing.T) {
	if got := common.BuildRoleOptions(common.RoleOptions{}); got != " WITH CONNECTION LIMIT 0" {
		t.Errorf("expected only CONNECTION LIMIT, got %q", got)
	}
}

func TestBuildRoleOptions_loginUserWithPassword(t *testing.T) {
	superuser := true
	got := common.BuildRoleOptions(common.RoleOptions{
		Superuser:       &superuser,
		ConnectionLimit: 20,
		Password:        "hunter2",
	})
	if contains(got, "LOGIN") || contains(got, "NOLOGIN") {
		t.Errorf("expected neither LOGIN nor NOLOGIN when Login=nil, got %q", got)
	}
	if !contains(got, "SUPERUSER") {
		t.Errorf("expected SUPERUSER in %q", got)
	}
	if !contains(got, "PASSWORD 'hunter2'") {
		t.Errorf("expected quoted password in %q", got)
	}
	if !contains(got, "CONNECTION LIMIT 20") {
		t.Errorf("expected CONNECTION LIMIT 20 in %q", got)
	}
}

func TestBuildRoleOptions_nullPassword(t *testing.T) {
	got := common.BuildRoleOptions(common.RoleOptions{NullPassword: true})
	if !contains(got, "PASSWORD NULL") {
		t.Errorf("expected PASSWORD NULL in %q", got)
	}
	// A password set wins over clearing it.
	got = common.BuildRoleOptions(common.RoleOptions{Password: "pw", NullPassword: true})
	if !contains(got, "PASSWORD 'pw'") || contains(got, "PASSWORD NULL") {
		t.Errorf("expected only the quoted password in %q", got)
	}
}

func TestFlagChange_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		plan  types.Bool
		state types.Bool
		want  *bool
	}{
		{"create true", types.BoolValue(true), types.BoolNull(), boolPtr(true)},
		{"create false omitted", types.BoolValue(false), types.BoolNull(), nil},
		{"create unknown state", types.BoolValue(true), types.BoolUnknown(), boolPtr(true)},
		{"alter unchanged", types.BoolValue(true), types.BoolValue(true), nil},
		{"alter to false", types.BoolValue(false), types.BoolValue(true), boolPtr(false)},
		{"alter to true", types.BoolValue(true), types.BoolValue(false), boolPtr(true)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := common.FlagChange(tc.plan, tc.state)
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Errorf("FlagChange(%v, %v) = %v, want %v", tc.plan, tc.state, fmtBoolPtr(got), fmtBoolPtr(tc.want))
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

func fmtBoolPtr(b *bool) string {
	if b == nil {
		return "nil"
	}
	return fmt.Sprint(*b)
}

func TestBuildRoleOptions_passwordQuoted(t *testing.T) {
	// Single-quote must be escaped in the password.
	got := common.BuildRoleOptions(common.RoleOptions{
		Password: "it's",
	})
	if !contains(got, "PASSWORD 'it''s'") {
		t.Errorf("expected escaped single-quote in password, got %q", got)
	}
}

func TestBuildRoleOptions_validUntil(t *testing.T) {
	got := common.BuildRoleOptions(common.RoleOptions{
		ValidUntil: "2030-01-01",
	})
	if !contains(got, "VALID UNTIL '2030-01-01'") {
		t.Errorf("expected VALID UNTIL clause, got %q", got)
	}
}

func TestBuildRoleOptions_explicitLogin(t *testing.T) {
	login := true
	got := common.BuildRoleOptions(common.RoleOptions{
		Login: &login,
	})
	if !contains(got, "LOGIN") || contains(got, "NOLOGIN") {
		t.Errorf("expected LOGIN (not NOLOGIN) in %q", got)
	}
}

// ---------------------------------------------------------------------------
// LogRollback tests
// ---------------------------------------------------------------------------

type fakeTx struct {
	rollbackErr error
	calls       int
}

func (f *fakeTx) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, nil
}
func (f *fakeTx) QueryContext(_ context.Context, _ string, _ ...any) (common.Rows, error) {
	return nil, nil
}
func (f *fakeTx) PrepareContext(_ context.Context, _ string) (common.Stmt, error) {
	return nil, nil
}
func (f *fakeTx) Commit() error { return nil }
func (f *fakeTx) Rollback() error {
	f.calls++
	return f.rollbackErr
}

func TestLogRollback_success(t *testing.T) {
	tx := &fakeTx{rollbackErr: nil}
	common.LogRollback(context.Background(), tx)
	if tx.calls != 1 {
		t.Errorf("expected 1 rollback call, got %d", tx.calls)
	}
}

func TestLogRollback_alreadyCommitted(t *testing.T) {
	// sql.ErrTxDone must not be logged or escalated — benign.
	tx := &fakeTx{rollbackErr: sql.ErrTxDone}
	common.LogRollback(context.Background(), tx)
	if tx.calls != 1 {
		t.Errorf("expected 1 rollback call, got %d", tx.calls)
	}
}

func TestLogRollback_genericError(t *testing.T) {
	// Non-ErrTxDone errors are logged via tflog (no panic, no propagation).
	tx := &fakeTx{rollbackErr: errors.New("connection dropped")}
	common.LogRollback(context.Background(), tx)
	if tx.calls != 1 {
		t.Errorf("expected 1 rollback call, got %d", tx.calls)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	return len(sub) == 0 || indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
