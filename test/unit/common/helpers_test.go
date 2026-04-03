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
