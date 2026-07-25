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
	"github.com/lib/pq"
)

func TestIsRetryableError_nil(t *testing.T) {
	if common.IsRetryableError(nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsRetryableError_nonPq(t *testing.T) {
	if common.IsRetryableError(errors.New("generic error")) {
		t.Error("expected false for non-pq error")
	}
}

func TestIsRetryableError_retryable(t *testing.T) {
	err := &pq.Error{Code: "53300"}
	if !common.IsRetryableError(err) {
		t.Error("expected true for too_many_connections")
	}
}

func TestIsRetryableError_deadlock(t *testing.T) {
	err := &pq.Error{Code: "40P01"}
	if !common.IsRetryableError(err) {
		t.Error("expected true for deadlock_detected")
	}
}

func TestIsRetryableError_nonRetryable(t *testing.T) {
	err := &pq.Error{Code: "42P01"}
	if common.IsRetryableError(err) {
		t.Error("expected false for undefined_table")
	}
}

// mockExec is a simple inline mock for the ExecContext interface, used to avoid
// import cycles with the generated mocks package.
type mockExec struct {
	calls   int
	results []sql.Result
	errors  []error
}

func (m *mockExec) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	i := m.calls
	m.calls++
	if i < len(m.errors) {
		return m.results[i], m.errors[i]
	}
	return nil, fmt.Errorf("unexpected call %d", i)
}

type noopResult struct{}

func (r noopResult) LastInsertId() (int64, error) { return 0, nil }
func (r noopResult) RowsAffected() (int64, error) { return 0, nil }

func TestRetryExec_ImmediateSuccess(t *testing.T) {
	m := &mockExec{
		results: []sql.Result{noopResult{}},
		errors:  []error{nil},
	}
	result, err := common.RetryExec(context.Background(), m, "SELECT 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if m.calls != 1 {
		t.Errorf("expected 1 call, got %d", m.calls)
	}
}

func TestRetryExec_NonRetryableError(t *testing.T) {
	m := &mockExec{
		results: []sql.Result{nil},
		errors:  []error{&pq.Error{Code: "42P01"}},
	}
	_, err := common.RetryExec(context.Background(), m, "INSERT INTO foo VALUES (1)")
	if err == nil {
		t.Fatal("expected error for non-retryable pq.Error")
	}
	if m.calls != 1 {
		t.Errorf("expected 1 call (no retry), got %d", m.calls)
	}
}

func TestRetryExec_RetryableErrorThenSuccess(t *testing.T) {
	m := &mockExec{
		results: []sql.Result{nil, noopResult{}},
		errors:  []error{&pq.Error{Code: "53300"}, nil},
	}
	result, err := common.RetryExec(context.Background(), m, "SELECT 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if m.calls != 2 {
		t.Errorf("expected 2 calls, got %d", m.calls)
	}
}

func TestRetryExec_AllRetriesFail(t *testing.T) {
	m := &mockExec{
		results: []sql.Result{nil, nil, nil},
		errors: []error{
			&pq.Error{Code: "40P01"},
			&pq.Error{Code: "40P01"},
			&pq.Error{Code: "40P01"},
		},
	}
	_, err := common.RetryExec(context.Background(), m, "UPDATE foo SET bar = 1")
	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}
	pqErr, ok := err.(*pq.Error)
	if !ok {
		t.Fatalf("expected *pq.Error, got %T", err)
	}
	if pqErr.Code != "40P01" {
		t.Errorf("expected error code 40P01, got %s", pqErr.Code)
	}
	if m.calls != 3 {
		t.Errorf("expected 3 calls, got %d", m.calls)
	}
}

func TestRetryExec_GenericErrorNoRetry(t *testing.T) {
	m := &mockExec{
		results: []sql.Result{nil},
		errors:  []error{fmt.Errorf("generic error")},
	}
	_, err := common.RetryExec(context.Background(), m, "SELECT 1")
	if err == nil {
		t.Fatal("expected error for generic error")
	}
	if err.Error() != "generic error" {
		t.Errorf("expected 'generic error', got %q", err.Error())
	}
	if m.calls != 1 {
		t.Errorf("expected 1 call (no retry), got %d", m.calls)
	}
}

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
	wants := []string{"NOLOGIN", "NOSUPERUSER", "NOCREATEDB", "NOCREATEROLE", "NOREPLICATION", "CONNECTION LIMIT -1"}
	for _, w := range wants {
		if !contains(got, w) {
			t.Errorf("expected %q in %q", w, got)
		}
	}
	if !startsWith(got, " WITH ") {
		t.Errorf("expected leading ' WITH ' in %q", got)
	}
}

func TestBuildRoleOptions_loginUserWithPassword(t *testing.T) {
	got := common.BuildRoleOptions(common.RoleOptions{
		Superuser:       true,
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
// NewDBWrapperWithOptions tests
// ---------------------------------------------------------------------------

func TestNewDBWrapperWithOptions_storesSuperuser(t *testing.T) {
	w := common.NewDBWrapperWithOptions(&sql.DB{}, false)
	if w.Superuser {
		t.Error("expected Superuser=false")
	}
	w2 := common.NewDBWrapperWithOptions(&sql.DB{}, true)
	if !w2.Superuser {
		t.Error("expected Superuser=true")
	}
}

func TestNewDBWrapper_defaultsSuperuserTrue(t *testing.T) {
	w := common.NewDBWrapper(&sql.DB{})
	if !w.Superuser {
		t.Error("expected default Superuser=true for backward compat")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	return len(sub) == 0 || indexOf(s, sub) >= 0
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
