package datasource_test

import (
	"context"
	"strings"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/datasource"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/mocks"
	"go.uber.org/mock/gomock"
)

// ---------------------------------------------------------------------------
// Absurd-input tests: hostile filter values must arrive as bind parameters,
// never spliced into the SQL text.
// ---------------------------------------------------------------------------

// captureQuery records the statement and args QueryContext was called with.
// argCount must match the number of bind parameters expected.
func captureQuery(t *testing.T, ctrl *gomock.Controller, argCount int) (*mocks.MockDBTX, *string, *[]any) {
	t.Helper()

	gotQuery := new(string)
	gotArgs := new([]any)

	mockRows := mocks.NewMockRows(ctrl)
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	// ctx and query go separately: EXPECT's signature is (any, any, ...any).
	argMatchers := make([]any, 0, argCount)
	for i := 0; i < argCount; i++ {
		argMatchers = append(argMatchers, gomock.Any())
	}

	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), argMatchers...).DoAndReturn(
		func(_ context.Context, query string, args ...any) (common.Rows, error) {
			*gotQuery = query
			*gotArgs = args
			return mockRows, nil
		})

	return mockDB, gotQuery, gotArgs
}

// assertParameterized checks payload stayed out of the SQL text and survived
// unchanged as the bind parameter at argIdx.
func assertParameterized(t *testing.T, query string, args []any, argIdx int, payload, wantPlaceholder string) {
	t.Helper()

	if strings.Contains(query, payload) {
		t.Errorf("payload was interpolated into the statement: %s", query)
	}
	if !strings.Contains(query, wantPlaceholder) {
		t.Errorf("expected placeholder %q in statement, got: %s", wantPlaceholder, query)
	}
	if len(args) <= argIdx {
		t.Fatalf("expected at least %d args, got %d: %#v", argIdx+1, len(args), args)
	}
	got, ok := args[argIdx].(string)
	if !ok {
		t.Fatalf("arg %d is %T, want string", argIdx, args[argIdx])
	}
	if got != payload {
		t.Errorf("arg %d was mutated:\n got: %q\nwant: %q", argIdx, got, payload)
	}
}

// Feeds a statement terminator plus a DROP into like_pattern.
func TestRolesDataSource_Read_absurdSQLInjectionInLikePattern(t *testing.T) {
	const payload = `%'; DROP TABLE pg_catalog.pg_roles; --`

	ctrl := gomock.NewController(t)
	mockDB, gotQuery, gotArgs := captureQuery(t, ctrl, 1)

	ctx := context.Background()
	s := rolesSchema()
	like := payload
	cfg := rolesFilteredConfig(ctx, s, nil, &like, nil)

	req, resp := newReadReqResp(ctx, s, cfg)
	d := &datasource.RolesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	assertParameterized(t, *gotQuery, *gotArgs, 0, payload, "rolname LIKE $1")
	if strings.Contains(strings.ToUpper(*gotQuery), "DROP") {
		t.Errorf("DROP leaked into the statement: %s", *gotQuery)
	}
}

// Embedded quotes through two filters at once, pinning the $1/$2 ordering.
func TestTablesDataSource_Read_absurdQuotesInSchemaAndType(t *testing.T) {
	const schemaPayload = `pub"lic'; --`
	const typePayload = `BASE TABLE'); DELETE FROM t; --`

	ctrl := gomock.NewController(t)
	mockDB, gotQuery, gotArgs := captureQuery(t, ctrl, 2)

	ctx := context.Background()
	s := tablesSchema()
	sch := schemaPayload
	tt := typePayload
	cfg := tablesFilteredConfig(ctx, s, &sch, nil, nil, &tt)

	req, resp := newReadReqResp(ctx, s, cfg)
	d := &datasource.TablesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	assertParameterized(t, *gotQuery, *gotArgs, 0, schemaPayload, "t.table_schema = $1")
	assertParameterized(t, *gotQuery, *gotArgs, 1, typePayload, "t.table_type = $2")
}

// Multi-byte runes, control chars, a NUL byte and 8 KiB of padding.
func TestSchemasDataSource_Read_absurdUnicodeControlAndLongPattern(t *testing.T) {
	payload := "schéma_🙂\n\t\x00" + strings.Repeat("ó", 8192) + "%"

	ctrl := gomock.NewController(t)
	mockDB, gotQuery, gotArgs := captureQuery(t, ctrl, 1)

	ctx := context.Background()
	s := schemasSchemaFn()
	like := payload
	cfg := schemasFilteredConfig(ctx, s, nil, &like, nil)

	req, resp := newReadReqResp(ctx, s, cfg)
	d := &datasource.SchemasDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	assertParameterized(t, *gotQuery, *gotArgs, 0, payload, "$1")

	// Byte-for-byte, not just equal-length: catches silent NUL truncation.
	got := (*gotArgs)[0].(string)
	if len(got) != len(payload) {
		t.Errorf("arg length changed: got %d bytes, want %d", len(got), len(payload))
	}
}
