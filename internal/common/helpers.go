package common

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
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
	PrepareContext(ctx context.Context, query string) (Stmt, error)
	Commit() error
	Rollback() error
}

// Stmt abstracts a prepared statement (like *sql.Stmt).
type Stmt interface {
	QueryContext(ctx context.Context, args ...any) (Rows, error)
	Close() error
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
	DB *sql.DB
	// Database is the database DB is connected to.
	Database string
	// Open connects to another database on the same server; nil keeps every
	// operation on DB.
	Open func(ctx context.Context, database string) (*sql.DB, error)

	mu    sync.Mutex
	pools map[string]*DBWrapper
}

func NewDBWrapper(db *sql.DB) *DBWrapper {
	return &DBWrapper{DB: db}
}

// DatabaseSwitcher is implemented by connections that can reach other databases.
type DatabaseSwitcher interface {
	ForDatabase(ctx context.Context, database string) (DBTX, error)
	ReleaseDatabase(database string) error
}

// switchers holds every wrapper that opened another database. Terraform runs one
// provider per process, but tests configure many in one, and any of them blocks a drop.
var (
	switchersMu sync.Mutex
	switchers   = map[*DBWrapper]struct{}{}
)

// ForDatabase returns a connection to the named database, opening and caching
// a pool on first use. An empty name or the default database returns w.
func (w *DBWrapper) ForDatabase(ctx context.Context, database string) (DBTX, error) {
	if database == "" || database == w.Database || w.Open == nil {
		return w, nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if p, ok := w.pools[database]; ok {
		return p, nil
	}
	db, err := w.Open(ctx, database)
	if err != nil {
		return nil, fmt.Errorf("connect to database %q: %w", database, err)
	}
	if w.pools == nil {
		w.pools = map[string]*DBWrapper{}
		switchersMu.Lock()
		switchers[w] = struct{}{}
		switchersMu.Unlock()
	}
	p := &DBWrapper{DB: db, Database: database}
	w.pools[database] = p
	return p, nil
}

// ReleaseDatabase closes the cached pools of a database, so that it can be dropped or renamed.
func (w *DBWrapper) ReleaseDatabase(database string) error {
	switchersMu.Lock()
	all := make([]*DBWrapper, 0, len(switchers))
	for s := range switchers {
		all = append(all, s)
	}
	switchersMu.Unlock()

	var errs []error
	for _, s := range all {
		s.mu.Lock()
		if p, ok := s.pools[database]; ok {
			delete(s.pools, database)
			errs = append(errs, p.DB.Close())
		}
		s.mu.Unlock()
	}
	return errors.Join(errs...)
}

// Close closes every cached pool and the default one.
func (w *DBWrapper) Close() error {
	w.mu.Lock()
	pools := w.pools
	w.pools = nil
	w.mu.Unlock()
	switchersMu.Lock()
	delete(switchers, w)
	switchersMu.Unlock()
	errs := make([]error, 0, len(pools)+1)
	for _, p := range pools {
		errs = append(errs, p.DB.Close())
	}
	errs = append(errs, w.DB.Close())
	return errors.Join(errs...)
}

// OnDatabase returns a connection to database when db can switch databases, and db otherwise.
func OnDatabase(ctx context.Context, db DBTX, database string) (DBTX, error) {
	if s, ok := db.(DatabaseSwitcher); ok {
		return s.ForDatabase(ctx, database)
	}
	return db, nil
}

// ReleaseDatabase closes any pool db keeps to database.
func ReleaseDatabase(db DBTX, database string) error {
	if s, ok := db.(DatabaseSwitcher); ok {
		return s.ReleaseDatabase(database)
	}
	return nil
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

func (w *TxWrapper) PrepareContext(ctx context.Context, query string) (Stmt, error) {
	stmt, err := w.tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &StmtWrapper{stmt: stmt}, nil
}

// StmtWrapper wraps *sql.Stmt to satisfy the Stmt interface.
type StmtWrapper struct {
	stmt *sql.Stmt
}

func (w *StmtWrapper) QueryContext(ctx context.Context, args ...any) (Rows, error) {
	return w.stmt.QueryContext(ctx, args...)
}

func (w *StmtWrapper) Close() error {
	return w.stmt.Close()
}

func (w *TxWrapper) Commit() error {
	return w.tx.Commit()
}

func (w *TxWrapper) Rollback() error {
	return w.tx.Rollback()
}

// Compile-time checks.
var _ DBTX = (*DBWrapper)(nil)
var _ DatabaseSwitcher = (*DBWrapper)(nil)
var _ ExecContext = (*DBWrapper)(nil)
var _ Tx = (*TxWrapper)(nil)
var _ Stmt = (*StmtWrapper)(nil)
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
// (or user) via CREATE/ALTER ROLE ... WITH. Nil or empty fields are omitted:
// since PostgreSQL 16 an administrator without SUPERUSER may not even name
// SUPERUSER or REPLICATION in ALTER ROLE.
type RoleOptions struct {
	Login           *bool
	Superuser       *bool
	CreateDatabase  *bool
	CreateRole      *bool
	Replication     *bool
	ConnectionLimit int64
	// Password, if non-empty, emits PASSWORD <quoted>; NullPassword emits PASSWORD NULL.
	Password     string
	NullPassword bool
	// ValidUntil, if non-empty, emits VALID UNTIL <quoted>.
	ValidUntil string
}

// BuildRoleOptions renders a " WITH ..." clause from the given role options.
func BuildRoleOptions(o RoleOptions) string {
	var opts []string
	flag := func(v *bool, yes, no string) {
		switch {
		case v == nil:
		case *v:
			opts = append(opts, yes)
		default:
			opts = append(opts, no)
		}
	}
	flag(o.Login, "LOGIN", "NOLOGIN")
	flag(o.Superuser, "SUPERUSER", "NOSUPERUSER")
	flag(o.CreateDatabase, "CREATEDB", "NOCREATEDB")
	flag(o.CreateRole, "CREATEROLE", "NOCREATEROLE")
	flag(o.Replication, "REPLICATION", "NOREPLICATION")

	opts = append(opts, fmt.Sprintf("CONNECTION LIMIT %d", o.ConnectionLimit))

	switch {
	case o.Password != "":
		opts = append(opts, fmt.Sprintf("PASSWORD %s", pq.QuoteLiteral(o.Password)))
	case o.NullPassword:
		opts = append(opts, "PASSWORD NULL")
	}

	if o.ValidUntil != "" {
		opts = append(opts, fmt.Sprintf("VALID UNTIL %s", pq.QuoteLiteral(o.ValidUntil)))
	}

	return " WITH " + strings.Join(opts, " ")
}

// FlagChange returns the flag value to send: on create (state null) only a
// true value, on alter only a changed one.
func FlagChange(plan, state types.Bool) *bool {
	v := plan.ValueBool()
	if state.IsNull() || state.IsUnknown() {
		if v {
			return &v
		}
		return nil
	}
	if v == state.ValueBool() {
		return nil
	}
	return &v
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

// ExecInTx runs the statements in one transaction; an empty list runs nothing.
// It neither logs nor returns the SQL, which may hold a password.
func ExecInTx(ctx context.Context, db DBTX, statements []string) error {
	if len(statements) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer LogRollback(ctx, tx)
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return tx.Commit()
}
