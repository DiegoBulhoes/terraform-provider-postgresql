package resource_test

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/resource"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/mocks"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/lib/pq"
	"go.uber.org/mock/gomock"
)

const (
	dpGrant  = `ALTER DEFAULT PRIVILEGES FOR ROLE "owner" IN SCHEMA "app" GRANT SELECT, INSERT ON TABLES TO "reader"`
	dpRevoke = `ALTER DEFAULT PRIVILEGES FOR ROLE "owner" IN SCHEMA "app" REVOKE ALL ON TABLES FROM "reader"`
)

func dpSchema() rschema.Schema {
	resp := &fwresource.SchemaResponse{}
	(&resource.DefaultPrivilegesResource{}).Schema(context.Background(), fwresource.SchemaRequest{}, resp)
	return resp.Schema
}

// dpValue builds a plan or state; a nil privileges is a null set, as after an import.
func dpValue(ctx context.Context, s rschema.Schema, id, database, schemaName any, objectType string, privileges []string, withGrantOption bool) tftypes.Value {
	privs := tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil)
	if privileges != nil {
		vals := []tftypes.Value{}
		for _, p := range privileges {
			vals = append(vals, tftypes.NewValue(tftypes.String, p))
		}
		privs = tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, vals)
	}
	return tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
		"id":                tftypes.NewValue(tftypes.String, id),
		"role":              tftypes.NewValue(tftypes.String, "reader"),
		"owner":             tftypes.NewValue(tftypes.String, "owner"),
		"database":          tftypes.NewValue(tftypes.String, database),
		"schema":            tftypes.NewValue(tftypes.String, schemaName),
		"object_type":       tftypes.NewValue(tftypes.String, objectType),
		"privileges":        privs,
		"with_grant_option": tftypes.NewValue(tftypes.Bool, withGrantOption),
		"timeouts":          tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func dpPlan(ctx context.Context, s rschema.Schema, privileges ...string) tftypes.Value {
	return dpValue(ctx, s, nil, nil, "app", "table", privileges, false)
}

func dpState(ctx context.Context, s rschema.Schema, privileges ...string) tftypes.Value {
	return dpValue(ctx, s, "owner/reader/table//app", nil, "app", "table", privileges, false)
}

// dpWrapper fails to open any database but the provider's with err.
func dpWrapper(err error) *common.DBWrapper {
	return &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, err
	}}
}

func expectDPTarget(ctrl *gomock.Controller, db *mocks.MockDBTX, exists bool, err error) {
	scanner := mocks.NewMockScanner(ctrl)
	db.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), "owner", "reader", "app").Return(scanner)
	scanner.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*bool) = exists
		return err
	})
}

type dpRow struct {
	privilege string
	grantable bool
}

func dpRowsMock(ctrl *gomock.Controller, rows ...dpRow) *mocks.MockRows {
	m := mocks.NewMockRows(ctrl)
	var calls []any
	for _, r := range rows {
		calls = append(calls,
			m.EXPECT().Next().Return(true),
			m.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
				*dest[0].(*string) = r.privilege
				*dest[1].(*bool) = r.grantable
				return nil
			}))
	}
	calls = append(calls, m.EXPECT().Next().Return(false), m.EXPECT().Err().Return(nil))
	gomock.InOrder(calls...)
	m.EXPECT().Close().Return(nil)
	return m
}

func expectDPACL(ctrl *gomock.Controller, db *mocks.MockDBTX, rows ...dpRow) {
	db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "owner", "app", "r", "reader").Return(dpRowsMock(ctrl, rows...), nil)
}

func expectDPFull(ctrl *gomock.Controller, db *mocks.MockDBTX, privileges ...string) {
	db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "r").Return(stringRowsMock(ctrl, privileges...), nil)
}

func dpStatePrivileges(t *testing.T, state tfsdk.State) []string {
	t.Helper()
	var m resource.DefaultPrivilegesResourceModel
	if diags := state.Get(context.Background(), &m); diags.HasError() {
		t.Fatalf("state: %v", diags)
	}
	got := common.StringSetToSlice(context.Background(), m.Privileges)
	slices.Sort(got)
	return got
}

func TestBuildDefaultPrivileges_Statements(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"grant in schema", resource.BuildDefaultPrivilegesGrant("owner", "app", "table", "reader", []string{"SELECT", "INSERT"}, false), dpGrant},
		{"grant global with option", resource.BuildDefaultPrivilegesGrant("o\"x", "", "function", "r", []string{"EXECUTE"}, true),
			`ALTER DEFAULT PRIVILEGES FOR ROLE "o""x" GRANT EXECUTE ON FUNCTIONS TO "r" WITH GRANT OPTION`},
		{"grant schemas", resource.BuildDefaultPrivilegesGrant("o", "", "schema", "r", []string{"USAGE"}, false),
			`ALTER DEFAULT PRIVILEGES FOR ROLE "o" GRANT USAGE ON SCHEMAS TO "r"`},
		{"revoke in schema", resource.BuildDefaultPrivilegesRevoke("owner", "app", "table", "reader"), dpRevoke},
		{"revoke sequences", resource.BuildDefaultPrivilegesRevoke("o", "", "sequence", "r"),
			`ALTER DEFAULT PRIVILEGES FOR ROLE "o" REVOKE ALL ON SEQUENCES FROM "r"`},
		{"revoke types", resource.BuildDefaultPrivilegesRevoke("o", "s", "type", "r"),
			`ALTER DEFAULT PRIVILEGES FOR ROLE "o" IN SCHEMA "s" REVOKE ALL ON TYPES FROM "r"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got  %s\nwant %s", tc.got, tc.want)
			}
		})
	}
}

func TestDefaultPrivilegesResource_MetadataAndConfigure(t *testing.T) {
	r := &resource.DefaultPrivilegesResource{}
	mresp := &fwresource.MetadataResponse{}
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "postgresql"}, mresp)
	if mresp.TypeName != "postgresql_default_privileges" {
		t.Errorf("type name = %s", mresp.TypeName)
	}
	if resource.NewDefaultPrivilegesResource() == nil {
		t.Error("constructor returned nil")
	}

	cresp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{}, cresp)
	if cresp.Diagnostics.HasError() || r.DB != nil {
		t.Fatalf("nil provider data must be ignored: %v", cresp.Diagnostics)
	}
	r.Configure(context.Background(), fwresource.ConfigureRequest{ProviderData: "wrong"}, cresp)
	if !hasDiag(cresp.Diagnostics, "Unexpected Resource Configure Type") {
		t.Errorf("expected a configure error, got %v", cresp.Diagnostics)
	}
	cresp = &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{ProviderData: &common.DBWrapper{}}, cresp)
	if cresp.Diagnostics.HasError() || r.DB == nil {
		t.Errorf("expected the wrapper to be kept: %v", cresp.Diagnostics)
	}
}

func TestDefaultPrivilegesResource_ValidateConfig(t *testing.T) {
	ctx := context.Background()
	s := dpSchema()
	tests := []struct {
		name       string
		schemaName any
		objectType string
		privileges []string
		wantError  string
	}{
		{"tables in schema", "app", "table", []string{"select"}, ""},
		{"schemas globally", nil, "schema", []string{"USAGE"}, ""},
		{"schemas in a schema", "app", "schema", []string{"USAGE"}, "Unexpected schema"},
		{"invalid privilege", "app", "table", []string{"SELEC"}, "Invalid privileges"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := dpValue(ctx, s, nil, nil, tc.schemaName, tc.objectType, tc.privileges, false)
			resp := &fwresource.ValidateConfigResponse{}
			(&resource.DefaultPrivilegesResource{}).ValidateConfig(ctx,
				fwresource.ValidateConfigRequest{Config: tfsdk.Config{Raw: cfg, Schema: s}}, resp)
			if tc.wantError == "" {
				if resp.Diagnostics.HasError() {
					t.Errorf("unexpected errors: %v", resp.Diagnostics)
				}
				return
			}
			if !hasDiag(resp.Diagnostics, tc.wantError) {
				t.Errorf("expected %q, got %v", tc.wantError, resp.Diagnostics)
			}
		})
	}
}

func TestDefaultPrivilegesResource_Create(t *testing.T) {
	ctx := context.Background()
	s := dpSchema()

	t.Run("invalid privilege runs nothing", func(t *testing.T) {
		db := mocks.NewMockDBTX(gomock.NewController(t))
		req, resp := newCreateReqResp(ctx, s, dpPlan(ctx, s, "SELECT", "DROP"))
		(&resource.DefaultPrivilegesResource{DB: db}).Create(ctx, req, resp)
		if !hasDiag(resp.Diagnostics, "Error preparing ALTER DEFAULT PRIVILEGES") {
			t.Fatalf("got %v", resp.Diagnostics)
		}
	})

	t.Run("connect error", func(t *testing.T) {
		plan := dpValue(ctx, s, nil, "other", "app", "table", []string{"SELECT"}, false)
		req, resp := newCreateReqResp(ctx, s, plan)
		(&resource.DefaultPrivilegesResource{DB: dpWrapper(fmt.Errorf("refused"))}).Create(ctx, req, resp)
		if !hasDiag(resp.Diagnostics, "Error preparing ALTER DEFAULT PRIVILEGES") {
			t.Fatalf("got %v", resp.Diagnostics)
		}
	})

	t.Run("exec error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectTx(ctrl, db, []string{dpGrant}, 0)
		req, resp := newCreateReqResp(ctx, s, dpPlan(ctx, s, "select", "INSERT"))
		(&resource.DefaultPrivilegesResource{DB: db}).Create(ctx, req, resp)
		if !hasDiag(resp.Diagnostics, "Error executing ALTER DEFAULT PRIVILEGES") {
			t.Fatalf("got %v", resp.Diagnostics)
		}
	})

	t.Run("success keeps the configured spelling", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectTx(ctrl, db, []string{dpGrant}, -1)
		req, resp := newCreateReqResp(ctx, s, dpPlan(ctx, s, "select", "INSERT"))
		(&resource.DefaultPrivilegesResource{DB: db}).Create(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected errors: %v", resp.Diagnostics)
		}
		var m resource.DefaultPrivilegesResourceModel
		resp.State.Get(ctx, &m)
		if m.ID.ValueString() != "owner/reader/table//app" {
			t.Errorf("id = %s", m.ID.ValueString())
		}
		if got := dpStatePrivileges(t, resp.State); !slices.Equal(got, []string{"INSERT", "select"}) {
			t.Errorf("privileges = %v", got)
		}
	})
}

func TestDefaultPrivilegesResource_Update(t *testing.T) {
	ctx := context.Background()
	s := dpSchema()

	t.Run("invalid privilege runs nothing, not even the revoke", func(t *testing.T) {
		db := mocks.NewMockDBTX(gomock.NewController(t))
		req, resp := newUpdateReqResp(ctx, s, dpPlan(ctx, s, "FOO"), dpState(ctx, s, "SELECT"))
		(&resource.DefaultPrivilegesResource{DB: db}).Update(ctx, req, resp)
		if !hasDiag(resp.Diagnostics, "Error preparing ALTER DEFAULT PRIVILEGES") {
			t.Fatalf("got %v", resp.Diagnostics)
		}
	})

	for name, failAt := range map[string]int{"revoke fails": 0, "grant fails": 1, "commit fails": 2} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := mocks.NewMockDBTX(ctrl)
			expectTx(ctrl, db, []string{dpRevoke, dpGrant}, failAt)
			req, resp := newUpdateReqResp(ctx, s, dpPlan(ctx, s, "SELECT", "INSERT"), dpState(ctx, s, "SELECT"))
			(&resource.DefaultPrivilegesResource{DB: db}).Update(ctx, req, resp)
			if !hasDiag(resp.Diagnostics, "Error updating default privileges") {
				t.Fatalf("got %v", resp.Diagnostics)
			}
		})
	}

	t.Run("revoke and grant in one transaction", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectTx(ctrl, db, []string{dpRevoke, dpGrant}, -1)
		req, resp := newUpdateReqResp(ctx, s, dpPlan(ctx, s, "SELECT", "INSERT"), dpState(ctx, s, "SELECT"))
		(&resource.DefaultPrivilegesResource{DB: db}).Update(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected errors: %v", resp.Diagnostics)
		}
	})
}

func TestDefaultPrivilegesResource_Read(t *testing.T) {
	ctx := context.Background()
	s := dpSchema()

	t.Run("database gone", func(t *testing.T) {
		state := dpValue(ctx, s, "x", "gone", "app", "table", []string{"SELECT"}, false)
		req, resp := newReadReqResp(ctx, s, state)
		(&resource.DefaultPrivilegesResource{DB: dpWrapper(&pq.Error{Code: "3D000"})}).Read(ctx, req, resp)
		if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
			t.Fatalf("expected removal, got %v", resp.Diagnostics)
		}
	})

	t.Run("connect error", func(t *testing.T) {
		state := dpValue(ctx, s, "x", "other", "app", "table", []string{"SELECT"}, false)
		req, resp := newReadReqResp(ctx, s, state)
		(&resource.DefaultPrivilegesResource{DB: dpWrapper(fmt.Errorf("refused"))}).Read(ctx, req, resp)
		if !hasDiag(resp.Diagnostics, "Error connecting to database") {
			t.Fatalf("got %v", resp.Diagnostics)
		}
	})

	t.Run("target query error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectDPTarget(ctrl, db, false, fmt.Errorf("boom"))
		req, resp := newReadReqResp(ctx, s, dpState(ctx, s, "SELECT"))
		(&resource.DefaultPrivilegesResource{DB: db}).Read(ctx, req, resp)
		if !hasDiag(resp.Diagnostics, "Error reading default privileges") {
			t.Fatalf("got %v", resp.Diagnostics)
		}
	})

	t.Run("owner, role or schema gone", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectDPTarget(ctrl, db, false, nil)
		req, resp := newReadReqResp(ctx, s, dpState(ctx, s, "SELECT"))
		(&resource.DefaultPrivilegesResource{DB: db}).Read(ctx, req, resp)
		if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
			t.Fatalf("expected removal, got %v", resp.Diagnostics)
		}
	})

	t.Run("import of nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectDPTarget(ctrl, db, true, nil)
		expectDPACL(ctrl, db)
		req, resp := newReadReqResp(ctx, s, dpState(ctx, s))
		(&resource.DefaultPrivilegesResource{DB: db}).Read(ctx, req, resp)
		if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
			t.Fatalf("expected removal, got %v", resp.Diagnostics)
		}
	})

	t.Run("full privileges error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectDPTarget(ctrl, db, true, nil)
		expectDPACL(ctrl, db, dpRow{"SELECT", false})
		db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "r").Return(nil, fmt.Errorf("boom"))
		req, resp := newReadReqResp(ctx, s, dpState(ctx, s, "SELECT"))
		(&resource.DefaultPrivilegesResource{DB: db}).Read(ctx, req, resp)
		if !hasDiag(resp.Diagnostics, "Error reading default privileges") {
			t.Fatalf("got %v", resp.Diagnostics)
		}
	})

	t.Run("ALL in lower case is kept when the catalog matches", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectDPTarget(ctrl, db, true, nil)
		expectDPACL(ctrl, db, dpRow{"SELECT", true}, dpRow{"INSERT", true})
		expectDPFull(ctrl, db, "INSERT", "SELECT")
		req, resp := newReadReqResp(ctx, s, dpState(ctx, s, "all"))
		(&resource.DefaultPrivilegesResource{DB: db}).Read(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected errors: %v", resp.Diagnostics)
		}
		if got := dpStatePrivileges(t, resp.State); !slices.Equal(got, []string{"all"}) {
			t.Errorf("privileges = %v", got)
		}
		var m resource.DefaultPrivilegesResourceModel
		resp.State.Get(ctx, &m)
		if !m.WithGrantOption.ValueBool() {
			t.Error("expected with_grant_option true")
		}
	})

	t.Run("drift shows what the catalog holds", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectDPTarget(ctrl, db, true, nil)
		expectDPACL(ctrl, db, dpRow{"SELECT", false})
		expectDPFull(ctrl, db, "INSERT", "SELECT")
		req, resp := newReadReqResp(ctx, s, dpState(ctx, s, "SELECT", "INSERT"))
		(&resource.DefaultPrivilegesResource{DB: db}).Read(ctx, req, resp)
		if got := dpStatePrivileges(t, resp.State); !slices.Equal(got, []string{"SELECT"}) {
			t.Errorf("privileges = %v", got)
		}
	})

	t.Run("everything revoked leaves an empty set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectDPTarget(ctrl, db, true, nil)
		expectDPACL(ctrl, db)
		expectDPFull(ctrl, db, "INSERT", "SELECT")
		req, resp := newReadReqResp(ctx, s, dpState(ctx, s, "SELECT"))
		(&resource.DefaultPrivilegesResource{DB: db}).Read(ctx, req, resp)
		if resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
			t.Fatalf("expected the resource kept, got %v", resp.Diagnostics)
		}
		if got := dpStatePrivileges(t, resp.State); len(got) != 0 {
			t.Errorf("privileges = %v", got)
		}
	})
}

func TestReadDefaultPrivileges_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("unsupported type", func(t *testing.T) {
		db := mocks.NewMockDBTX(gomock.NewController(t))
		if _, err := resource.ReadDefaultPrivileges(ctx, db, "owner", "reader", "database", "app"); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("acl query error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectDPTarget(ctrl, db, true, nil)
		db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("boom"))
		if _, err := resource.ReadDefaultPrivileges(ctx, db, "owner", "reader", "table", "app"); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectDPTarget(ctrl, db, true, nil)
		rows := mocks.NewMockRows(ctrl)
		rows.EXPECT().Next().Return(true)
		rows.EXPECT().Scan(gomock.Any(), gomock.Any()).Return(fmt.Errorf("bad row"))
		rows.EXPECT().Close().Return(nil)
		db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(rows, nil)
		if _, err := resource.ReadDefaultPrivileges(ctx, db, "owner", "reader", "table", "app"); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("rows error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectDPTarget(ctrl, db, true, nil)
		rows := mocks.NewMockRows(ctrl)
		rows.EXPECT().Next().Return(false)
		rows.EXPECT().Err().Return(fmt.Errorf("cursor broke"))
		rows.EXPECT().Close().Return(nil)
		db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(rows, nil)
		if _, err := resource.ReadDefaultPrivileges(ctx, db, "owner", "reader", "table", "app"); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("partly grantable", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db := mocks.NewMockDBTX(ctrl)
		expectDPTarget(ctrl, db, true, nil)
		expectDPACL(ctrl, db, dpRow{"SELECT", true}, dpRow{"INSERT", false})
		got, err := resource.ReadDefaultPrivileges(ctx, db, "owner", "reader", "table", "app")
		if err != nil || got.Grantable || !slices.Equal(got.Privileges, []string{"INSERT", "SELECT"}) {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
}

func TestDefaultFullPrivileges_Type(t *testing.T) {
	got, err := resource.DefaultFullPrivileges(context.Background(), nil, "type")
	if err != nil || !slices.Equal(got, []string{"USAGE"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDefaultPrivilegesResource_Delete(t *testing.T) {
	ctx := context.Background()
	s := dpSchema()

	t.Run("database gone", func(t *testing.T) {
		state := dpValue(ctx, s, "x", "gone", "app", "table", []string{"SELECT"}, false)
		req, resp := newDeleteReqResp(ctx, s, state)
		(&resource.DefaultPrivilegesResource{DB: dpWrapper(&pq.Error{Code: "3D000"})}).Delete(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected errors: %v", resp.Diagnostics)
		}
	})

	t.Run("connect error", func(t *testing.T) {
		state := dpValue(ctx, s, "x", "other", "app", "table", []string{"SELECT"}, false)
		req, resp := newDeleteReqResp(ctx, s, state)
		(&resource.DefaultPrivilegesResource{DB: dpWrapper(fmt.Errorf("refused"))}).Delete(ctx, req, resp)
		if !hasDiag(resp.Diagnostics, "Error connecting to database") {
			t.Fatalf("got %v", resp.Diagnostics)
		}
	})

	for name, tc := range map[string]struct {
		err       error
		wantError bool
	}{
		"success":     {nil, false},
		"schema gone": {&pq.Error{Code: "3F000"}, false},
		"role gone":   {&pq.Error{Code: "42704"}, false},
		"other error": {fmt.Errorf("permission denied"), true},
	} {
		t.Run(name, func(t *testing.T) {
			db := mocks.NewMockDBTX(gomock.NewController(t))
			db.EXPECT().ExecContext(gomock.Any(), dpRevoke).Return(mockResult{}, tc.err)
			req, resp := newDeleteReqResp(ctx, s, dpState(ctx, s, "SELECT"))
			(&resource.DefaultPrivilegesResource{DB: db}).Delete(ctx, req, resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("errors = %v, want error %v", resp.Diagnostics, tc.wantError)
			}
		})
	}
}

func TestDefaultPrivilegesResource_ImportState(t *testing.T) {
	ctx := context.Background()
	s := dpSchema()
	tests := []struct {
		id, wantID, wantError string
	}{
		{"owner/reader/TABLE/mydb/app", "owner/reader/table/mydb/app", ""},
		{"owner/reader/schema/", "owner/reader/schema//", ""},
		{"owner/reader/table", "", "Invalid Import ID"},
		{"/reader/table/db", "", "Invalid Import ID"},
		{"owner/reader/view/db", "", "Invalid Import ID"},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			req, resp := newImportReqResp(ctx, s, tc.id)
			(&resource.DefaultPrivilegesResource{}).ImportState(ctx, req, resp)
			if tc.wantError != "" {
				if !hasDiag(resp.Diagnostics, tc.wantError) {
					t.Fatalf("expected %q, got %v", tc.wantError, resp.Diagnostics)
				}
				return
			}
			var m resource.DefaultPrivilegesResourceModel
			resp.State.Get(ctx, &m)
			if m.ID.ValueString() != tc.wantID {
				t.Errorf("id = %s, want %s", m.ID.ValueString(), tc.wantID)
			}
			if strings.HasSuffix(tc.id, "/") != m.Database.IsNull() {
				t.Errorf("database = %v", m.Database)
			}
		})
	}
}
