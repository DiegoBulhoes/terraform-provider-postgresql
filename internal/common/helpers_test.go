package common

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lib/pq"
)

func TestIsRetryableError_nil(t *testing.T) {
	if isRetryableError(nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsRetryableError_nonPq(t *testing.T) {
	if isRetryableError(errors.New("generic error")) {
		t.Error("expected false for non-pq error")
	}
}

func TestIsRetryableError_retryable(t *testing.T) {
	// 53300 = too_many_connections
	err := &pq.Error{Code: "53300"}
	if !isRetryableError(err) {
		t.Error("expected true for too_many_connections")
	}
}

func TestIsRetryableError_deadlock(t *testing.T) {
	err := &pq.Error{Code: "40P01"}
	if !isRetryableError(err) {
		t.Error("expected true for deadlock_detected")
	}
}

func TestIsRetryableError_nonRetryable(t *testing.T) {
	// 42P01 = undefined_table
	err := &pq.Error{Code: "42P01"}
	if isRetryableError(err) {
		t.Error("expected false for undefined_table")
	}
}

func TestRetryExec_ImmediateSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(0, 0))

	result, err := RetryExec(context.Background(), db, "SELECT 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRetryExec_NonRetryableError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// 42P01 = undefined_table, which is NOT retryable
	mock.ExpectExec("INSERT").WillReturnError(&pq.Error{Code: "42P01"})

	_, err = RetryExec(context.Background(), db, "INSERT INTO foo VALUES (1)")
	if err == nil {
		t.Fatal("expected error for non-retryable pq.Error")
	}
	// Should have returned immediately after 1 attempt (no retry)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestRetryExec_RetryableErrorThenSuccess verifies that a retryable error on the first
// attempt is retried and succeeds on the second attempt.
// NOTE: This test takes ~1s due to the exponential backoff sleep.
func TestRetryExec_RetryableErrorThenSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// First attempt: retryable error (too_many_connections)
	mock.ExpectExec("SELECT").WillReturnError(&pq.Error{Code: "53300"})
	// Second attempt: success
	mock.ExpectExec("SELECT").WillReturnResult(sqlmock.NewResult(0, 1))

	result, err := RetryExec(context.Background(), db, "SELECT 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestRetryExec_AllRetriesFail verifies that after all 3 retry attempts are exhausted,
// the last error is returned.
// NOTE: This test takes ~7s due to exponential backoff (1s + 2s + 4s).
func TestRetryExec_AllRetriesFail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// All 3 attempts: retryable error (deadlock_detected)
	mock.ExpectExec("UPDATE").WillReturnError(&pq.Error{Code: "40P01"})
	mock.ExpectExec("UPDATE").WillReturnError(&pq.Error{Code: "40P01"})
	mock.ExpectExec("UPDATE").WillReturnError(&pq.Error{Code: "40P01"})

	_, err = RetryExec(context.Background(), db, "UPDATE foo SET bar = 1")
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
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRetryExec_GenericErrorNoRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// Generic errors (not *pq.Error) should not be retried
	mock.ExpectExec("SELECT").WillReturnError(fmt.Errorf("generic error"))

	_, err = RetryExec(context.Background(), db, "SELECT 1")
	if err == nil {
		t.Fatal("expected error for generic error")
	}
	if err.Error() != "generic error" {
		t.Errorf("expected 'generic error', got %q", err.Error())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestConfigureDB_wrongType(t *testing.T) {
	_, err := ConfigureDB("not a db")
	if err == nil {
		t.Error("expected error for wrong type")
	}
}

func TestConfigureDB_correct(t *testing.T) {
	db := &sql.DB{}
	result, err := ConfigureDB(db)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if result != db {
		t.Error("expected same DB pointer")
	}
}

func TestIsSet_null(t *testing.T) {
	type mockVal struct{}
	// Test with a simple interface that returns null
	if IsSet(nullVal{}) {
		t.Error("expected false for null value")
	}
}

func TestIsSet_set(t *testing.T) {
	if !IsSet(setVal{}) {
		t.Error("expected true for set value")
	}
}

func TestStringSetToSlice_null(t *testing.T) {
	ctx := context.Background()
	result := StringSetToSlice(ctx, types.SetNull(types.StringType))
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
	result := StringSetToSlice(ctx, set)
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
	result := StringListToSlice(ctx, types.ListNull(types.StringType))
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
	result := StringListToSlice(ctx, list)
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
	result := PrivilegesToSlice(ctx, set)
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
	result := PrivilegesToSlice(ctx, types.SetNull(types.StringType))
	if result != nil {
		t.Errorf("expected nil for null set, got %v", result)
	}
}

type nullVal struct{}

func (n nullVal) IsNull() bool    { return true }
func (n nullVal) IsUnknown() bool { return false }

type setVal struct{}

func (s setVal) IsNull() bool    { return false }
func (s setVal) IsUnknown() bool { return false }
