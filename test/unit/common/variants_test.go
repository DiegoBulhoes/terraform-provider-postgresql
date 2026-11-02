package common_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
)

// ---------------------------------------------------------------------------
// NormalizePrivileges — table-driven
// ---------------------------------------------------------------------------

func TestNormalizePrivileges_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{"empty_slice", []string{}, []string{}, false},
		{"single_valid", []string{"SELECT"}, []string{"SELECT"}, false},
		{"lowercase_normalized", []string{"select"}, []string{"SELECT"}, false},
		{"mixed_case_normalized", []string{"SeLeCt"}, []string{"SELECT"}, false},
		{"dedupe_same_case", []string{"SELECT", "SELECT"}, []string{"SELECT"}, false},
		{"dedupe_different_case", []string{"SELECT", "select", "Select"}, []string{"SELECT"}, false},
		{"trim_whitespace", []string{" SELECT ", "\tINSERT\n"}, []string{"SELECT", "INSERT"}, false},
		{"skip_empty_strings", []string{"", "SELECT", "   "}, []string{"SELECT"}, false},
		{"preserves_order", []string{"UPDATE", "SELECT", "INSERT"}, []string{"UPDATE", "SELECT", "INSERT"}, false},
		{"all_privileges_variants", []string{"ALL", "ALL PRIVILEGES"}, []string{"ALL", "ALL PRIVILEGES"}, false},
		{"maintain_valid", []string{"MAINTAIN"}, []string{"MAINTAIN"}, false},
		{"temp_and_temporary", []string{"TEMP", "TEMPORARY"}, []string{"TEMP", "TEMPORARY"}, false},
		{"set_and_alter_system", []string{"SET", "ALTER SYSTEM"}, []string{"SET", "ALTER SYSTEM"}, false},
		{"reject_single_invalid", []string{"DROP"}, nil, true},
		{"reject_among_valid", []string{"SELECT", "DROP"}, nil, true},
		{"reject_sql_injection_attempt", []string{"SELECT; DROP TABLE users"}, nil, true},
		{"reject_multiple_invalid_alphabetized", []string{"ZZZ", "FOO", "BAR"}, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := common.NormalizePrivileges(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; result=%v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len: expected %d (%v), got %d (%v)", len(tc.want), tc.want, len(got), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: expected %q, got %q", i, tc.want[i], got[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BuildRoleOptions — exact-token order tests
// ---------------------------------------------------------------------------

func TestBuildRoleOptions_OrderTableDriven(t *testing.T) {
	noLogin := false
	yesLogin := true

	tests := []struct {
		name   string
		opts   common.RoleOptions
		tokens []string // expected, in the order they must appear
	}{
		{
			"nil_flags_omitted",
			common.RoleOptions{Login: &noLogin, ConnectionLimit: -1},
			[]string{"NOLOGIN", "CONNECTION LIMIT -1"},
		},
		{
			"false_flags_negated_in_order",
			common.RoleOptions{Superuser: &noLogin, CreateDatabase: &noLogin, CreateRole: &noLogin, Replication: &noLogin, ConnectionLimit: -1},
			[]string{"NOSUPERUSER", "NOCREATEDB", "NOCREATEROLE", "NOREPLICATION", "CONNECTION LIMIT -1"},
		},
		{
			"all_flags_on_with_login",
			common.RoleOptions{
				Login: &yesLogin, Superuser: &yesLogin, CreateDatabase: &yesLogin,
				CreateRole: &yesLogin, Replication: &yesLogin, ConnectionLimit: 100,
			},
			[]string{"LOGIN", "SUPERUSER", "CREATEDB", "CREATEROLE", "REPLICATION", "CONNECTION LIMIT 100"},
		},
		{
			"null_password_after_connection_limit",
			common.RoleOptions{ConnectionLimit: 5, NullPassword: true, ValidUntil: "infinity"},
			[]string{"CONNECTION LIMIT 5", "PASSWORD NULL", "VALID UNTIL 'infinity'"},
		},
		{
			"password_appears_after_connection_limit",
			common.RoleOptions{ConnectionLimit: 5, Password: "pw"},
			[]string{"CONNECTION LIMIT 5", "PASSWORD 'pw'"},
		},
		{
			"valid_until_last",
			common.RoleOptions{ConnectionLimit: 5, ValidUntil: "2030-01-01"},
			[]string{"CONNECTION LIMIT 5", "VALID UNTIL '2030-01-01'"},
		},
		{
			"password_then_valid_until",
			common.RoleOptions{ConnectionLimit: 5, Password: "pw", ValidUntil: "2030-01-01"},
			[]string{"PASSWORD 'pw'", "VALID UNTIL '2030-01-01'"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := common.BuildRoleOptions(tc.opts)
			// Verify " WITH " prefix.
			if len(got) < 6 || got[:6] != " WITH " {
				t.Fatalf("expected ' WITH ' prefix, got %q", got)
			}
			// Verify each token appears, and appears in the expected order.
			prev := 0
			for _, tok := range tc.tokens {
				idx := indexAfter(got, tok, prev)
				if idx < 0 {
					t.Errorf("token %q not found (or out of order) in %q", tok, got)
					return
				}
				prev = idx + len(tok)
			}
		})
	}
}

func TestBuildRoleOptions_PasswordEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     string
	}{
		{"simple", "abc", "PASSWORD 'abc'"},
		{"with_single_quote", "ab'c", "PASSWORD 'ab''c'"},
		{"empty_skipped", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := common.BuildRoleOptions(common.RoleOptions{Password: tc.password})
			if tc.want == "" {
				// Empty password → no PASSWORD clause.
				if indexAfter(got, "PASSWORD", 0) >= 0 {
					t.Errorf("expected no PASSWORD clause for empty password, got %q", got)
				}
				return
			}
			if indexAfter(got, tc.want, 0) < 0 {
				t.Errorf("expected %q in %q", tc.want, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// QuoteConnStringValue — table-driven
// ---------------------------------------------------------------------------

func TestQuoteConnStringValue_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "abc", "'abc'"},
		{"empty", "", "''"},
		{"space", "a b", "'a b'"},
		{"single_quote", "a'b", `'a\'b'`},
		{"backslash", `a\b`, `'a\\b'`},
		{"backslash_then_quote", `\'`, `'\\\''`},
		{"unicode", "résumé", "'résumé'"},
		{"newline_preserved", "a\nb", "'a\nb'"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := common.QuoteConnStringValue(tc.in)
			if got != tc.want {
				t.Errorf("in=%q: expected %q, got %q", tc.in, tc.want, got)
			}
		})
	}
}

// TestDBWrapper_BeginTxErrorWraps exercises BeginTx's error-wrapping path via
// a never-connecting host. The TxWrapper's Exec/Query/Commit/Rollback methods
// themselves are thin pass-throughs and exercised by integration tests.
func TestDBWrapper_BeginTxErrorWraps(t *testing.T) {
	db, err := sql.Open("postgres", "host=127.0.0.1 port=1 sslmode=disable connect_timeout=1")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	w := common.NewDBWrapper(db)
	if _, err := w.BeginTx(context.Background(), nil); err == nil {
		t.Error("expected BeginTx to fail against unreachable host")
	}
}

// ---------------------------------------------------------------------------
// LogRollback additional paths
// ---------------------------------------------------------------------------

func TestLogRollback_NilError(t *testing.T) {
	// Defensive: make sure nil error is a no-op.
	tx := &fakeTx{rollbackErr: nil}
	common.LogRollback(context.Background(), tx)
	if tx.calls != 1 {
		t.Errorf("expected 1 call, got %d", tx.calls)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// indexAfter returns the first index of sub in s at or after offset, or -1.
func indexAfter(s, sub string, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset > len(s) {
		return -1
	}
	for i := offset; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
