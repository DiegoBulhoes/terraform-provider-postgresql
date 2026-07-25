package common

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lib/pq"
)

// DefaultTimeout is the default per-operation timeout used by resources.
const DefaultTimeout = 5 * time.Minute

// Scanner abstracts a single-row query result (like *sql.Row).
type Scanner interface {
	Scan(dest ...any) error
}

// Rows abstracts a multi-row query result (like *sql.Rows).
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Columns() ([]string, error)
	Close() error
	Err() error
}

// Tx abstracts a database transaction (like *sql.Tx).
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
	Commit() error
	Rollback() error
}

// DBTX is the interface for database operations used by resources and data sources.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) Scanner
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
}

// ExecContext is a minimal interface for executing SQL statements.
// Both DBTX and Tx satisfy this interface.
type ExecContext interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// DBWrapper wraps *sql.DB to satisfy the DBTX interface, returning abstract
// interfaces instead of concrete sql types.
type DBWrapper struct {
	DB        *sql.DB
	Superuser bool
}

func NewDBWrapper(db *sql.DB) *DBWrapper {
	return &DBWrapper{DB: db, Superuser: true}
}

// NewDBWrapperWithOptions builds a wrapper with explicit options.
func NewDBWrapperWithOptions(db *sql.DB, superuser bool) *DBWrapper {
	return &DBWrapper{DB: db, Superuser: superuser}
}

func (w *DBWrapper) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return w.DB.ExecContext(ctx, query, args...)
}

func (w *DBWrapper) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return w.DB.QueryContext(ctx, query, args...)
}

func (w *DBWrapper) QueryRowContext(ctx context.Context, query string, args ...any) Scanner {
	return w.DB.QueryRowContext(ctx, query, args...)
}

func (w *DBWrapper) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := w.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &TxWrapper{tx: tx}, nil
}

// TxWrapper wraps *sql.Tx to satisfy the Tx interface.
type TxWrapper struct {
	tx *sql.Tx
}

func (w *TxWrapper) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return w.tx.ExecContext(ctx, query, args...)
}

func (w *TxWrapper) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return w.tx.QueryContext(ctx, query, args...)
}

func (w *TxWrapper) Commit() error {
	return w.tx.Commit()
}

func (w *TxWrapper) Rollback() error {
	return w.tx.Rollback()
}

// Compile-time checks.
var _ DBTX = (*DBWrapper)(nil)
var _ ExecContext = (*DBWrapper)(nil)
var _ Tx = (*TxWrapper)(nil)
var _ Rows = (*sql.Rows)(nil)
var _ Scanner = (*sql.Row)(nil)

// ConfigureDB extracts the DBTX from provider data, returning an error if
// the type is unexpected. Used by all resources and data sources in Configure().
func ConfigureDB(providerData any) (DBTX, error) {
	wrapper, ok := providerData.(*DBWrapper)
	if !ok {
		return nil, fmt.Errorf("expected *DBWrapper, got: %T", providerData)
	}
	return wrapper, nil
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

// IsRetryableError checks if a database error is transient and worth retrying.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if pqErr, ok := err.(*pq.Error); ok {
		return retryableErrorCodes[pqErr.Code]
	}
	return false
}

// LogRollback rolls back a transaction and logs the error if rollback itself fails.
// The sql.ErrTxDone result is ignored because it only means the tx was already committed.
func LogRollback(ctx context.Context, tx Tx) {
	err := tx.Rollback()
	if err == nil || errors.Is(err, sql.ErrTxDone) {
		return
	}
	tflog.Warn(ctx, "Transaction rollback failed", map[string]interface{}{
		"error": err.Error(),
	})
}

// validPrivileges is the set of privilege keywords accepted by PostgreSQL for
// object-level GRANT/REVOKE. Values are uppercase.
var validPrivileges = map[string]struct{}{
	"ALL":            {},
	"ALL PRIVILEGES": {},
	"SELECT":         {},
	"INSERT":         {},
	"UPDATE":         {},
	"DELETE":         {},
	"TRUNCATE":       {},
	"REFERENCES":     {},
	"TRIGGER":        {},
	"USAGE":          {},
	"CREATE":         {},
	"CONNECT":        {},
	"TEMPORARY":      {},
	"TEMP":           {},
	"EXECUTE":        {},
	"MAINTAIN":       {},
	"SET":            {},
	"ALTER SYSTEM":   {},
}

// NormalizePrivileges uppercases, trims, and validates privilege keywords.
// Returns an error listing any keywords that are not valid PostgreSQL privileges.
func NormalizePrivileges(privs []string) ([]string, error) {
	out := make([]string, 0, len(privs))
	var bad []string
	seen := make(map[string]struct{}, len(privs))
	for _, p := range privs {
		up := strings.ToUpper(strings.TrimSpace(p))
		if up == "" {
			continue
		}
		if _, ok := validPrivileges[up]; !ok {
			bad = append(bad, p)
			continue
		}
		if _, dup := seen[up]; dup {
			continue
		}
		seen[up] = struct{}{}
		out = append(out, up)
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return nil, fmt.Errorf("invalid privilege(s): %s", strings.Join(bad, ", "))
	}
	return out, nil
}

// RoleOptions captures the set of option flags that can be applied to a role
// (or user) via CREATE/ALTER ROLE ... WITH. Fields left empty or nil are
// omitted from the output clause.
type RoleOptions struct {
	// Login controls the LOGIN/NOLOGIN keyword. When nil, neither is emitted.
	Login *bool
	// Superuser, CreateDatabase, CreateRole, Replication always emit the
	// positive or negative keyword.
	Superuser       bool
	CreateDatabase  bool
	CreateRole      bool
	Replication     bool
	ConnectionLimit int64
	// Password, if non-empty, emits PASSWORD <quoted>.
	Password string
	// ValidUntil, if non-empty, emits VALID UNTIL <quoted>.
	ValidUntil string
}

// BuildRoleOptions renders a " WITH ..." clause from the given role options.
// Returns an empty string when no options would be emitted.
func BuildRoleOptions(o RoleOptions) string {
	var opts []string

	if o.Login != nil {
		if *o.Login {
			opts = append(opts, "LOGIN")
		} else {
			opts = append(opts, "NOLOGIN")
		}
	}

	if o.Superuser {
		opts = append(opts, "SUPERUSER")
	} else {
		opts = append(opts, "NOSUPERUSER")
	}

	if o.CreateDatabase {
		opts = append(opts, "CREATEDB")
	} else {
		opts = append(opts, "NOCREATEDB")
	}

	if o.CreateRole {
		opts = append(opts, "CREATEROLE")
	} else {
		opts = append(opts, "NOCREATEROLE")
	}

	if o.Replication {
		opts = append(opts, "REPLICATION")
	} else {
		opts = append(opts, "NOREPLICATION")
	}

	opts = append(opts, fmt.Sprintf("CONNECTION LIMIT %d", o.ConnectionLimit))

	if o.Password != "" {
		opts = append(opts, fmt.Sprintf("PASSWORD %s", pq.QuoteLiteral(o.Password)))
	}

	if o.ValidUntil != "" {
		opts = append(opts, fmt.Sprintf("VALID UNTIL %s", pq.QuoteLiteral(o.ValidUntil)))
	}

	if len(opts) == 0 {
		return ""
	}
	return " WITH " + strings.Join(opts, " ")
}

// QuoteConnStringValue escapes a value for inclusion in a libpq-style
// "key=value" connection string. Values containing whitespace, quotes, or
// backslashes are wrapped in single quotes with embedded quotes/backslashes
// escaped. See https://www.postgresql.org/docs/current/libpq-connect.html.
func QuoteConnStringValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `'`, `\'`)
	return "'" + v + "'"
}

// RetryExec executes a SQL statement with retry logic for transient errors.
// It retries up to 3 times with exponential backoff (1s, 2s, 4s).
func RetryExec(ctx context.Context, db ExecContext, query string, args ...interface{}) (sql.Result, error) {
	var result sql.Result
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		result, err = db.ExecContext(ctx, query, args...)
		if err == nil || !IsRetryableError(err) {
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
