package common

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lib/pq"
)

// DBTX is the interface for database operations used by resources and data sources.
// *sql.DB satisfies this interface.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// Compile-time check that *sql.DB satisfies DBTX.
var _ DBTX = (*sql.DB)(nil)

// ConfigureDB extracts the DBTX from provider data, returning an error if
// the type is unexpected. Used by all resources and data sources in Configure().
func ConfigureDB(providerData any) (DBTX, error) {
	db, ok := providerData.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("expected *sql.DB, got: %T", providerData)
	}
	return db, nil
}

// IsSet returns true if a Terraform attribute value is neither null nor unknown.
func IsSet(val interface {
	IsNull() bool
	IsUnknown() bool
}) bool {
	return !val.IsNull() && !val.IsUnknown()
}

// StringSetToSlice converts a types.Set of strings into a []string.
func StringSetToSlice(ctx context.Context, set types.Set) []string {
	if !IsSet(set) {
		return nil
	}
	var elems []types.String
	set.ElementsAs(ctx, &elems, false)
	result := make([]string, len(elems))
	for i, e := range elems {
		result[i] = e.ValueString()
	}
	return result
}

// StringListToSlice converts a types.List of strings into a []string.
func StringListToSlice(ctx context.Context, list types.List) []string {
	if !IsSet(list) {
		return nil
	}
	var elems []types.String
	list.ElementsAs(ctx, &elems, false)
	result := make([]string, len(elems))
	for i, e := range elems {
		result[i] = e.ValueString()
	}
	return result
}

// PrivilegesToSlice extracts privilege strings from a types.Set and uppercases them.
func PrivilegesToSlice(ctx context.Context, set types.Set) []string {
	raw := StringSetToSlice(ctx, set)
	for i, p := range raw {
		raw[i] = strings.ToUpper(p)
	}
	return raw
}

// retryableErrorCodes contains PostgreSQL error codes that are transient and worth retrying.
// See https://www.postgresql.org/docs/current/errcodes-appendix.html
var retryableErrorCodes = map[pq.ErrorCode]bool{
	"53300": true, // too_many_connections
	"53400": true, // configuration_limit_exceeded
	"40P01": true, // deadlock_detected
	"40001": true, // serialization_failure
	"08006": true, // connection_failure
	"08001": true, // sqlclient_unable_to_establish_sqlconnection
	"08004": true, // sqlserver_rejected_establishment_of_sqlconnection
	"57P03": true, // cannot_connect_now (server starting up)
}

// isRetryableError checks if a database error is transient and worth retrying.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if pqErr, ok := err.(*pq.Error); ok {
		return retryableErrorCodes[pqErr.Code]
	}
	return false
}

// RetryExec executes a SQL statement with retry logic for transient errors.
// It retries up to 3 times with exponential backoff (1s, 2s, 4s).
func RetryExec(ctx context.Context, db *sql.DB, query string, args ...interface{}) (sql.Result, error) {
	var result sql.Result
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		result, err = db.ExecContext(ctx, query, args...)
		if err == nil || !isRetryableError(err) {
			return result, err
		}

		wait := time.Duration(1<<uint(attempt)) * time.Second
		tflog.Warn(ctx, "Transient database error, retrying", map[string]interface{}{
			"attempt": attempt + 1,
			"error":   err.Error(),
			"wait":    wait.String(),
		})

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}

	return result, err
}
