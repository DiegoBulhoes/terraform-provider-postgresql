package resource_test

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/resource"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/mocks"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/lib/pq"
	"go.uber.org/mock/gomock"
)

func membershipResourceSchema() rschema.Schema {
	r := &resource.RoleMembershipResource{}
	sresp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func membershipValue(ctx context.Context, s rschema.Schema, id any, admin bool) tftypes.Value {
	return tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
		"id":           tftypes.NewValue(tftypes.String, id),
		"role":         tftypes.NewValue(tftypes.String, "grp"),
		"member":       tftypes.NewValue(tftypes.String, "usr"),
		"admin_option": tftypes.NewValue(tftypes.Bool, admin),
		"timeouts":     tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func membershipGrantsRows(ctrl *gomock.Controller, grants ...resource.MembershipGrant) *mocks.MockRows {
	m := mocks.NewMockRows(ctrl)
	var calls []any
	for _, g := range grants {
		calls = append(calls,
			m.EXPECT().Next().Return(true),
			m.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
				*dest[0].(*string) = g.Grantor
				*dest[1].(*bool) = g.AdminOption
				*dest[2].(*bool) = g.Revocable
				return nil
			}))
	}
	calls = append(calls, m.EXPECT().Next().Return(false), m.EXPECT().Err().Return(nil))
	gomock.InOrder(calls...)
	m.EXPECT().Close().Return(nil)
	return m
}

func expectMembershipTx(ctrl *gomock.Controller, db *mocks.MockDBTX, statements []string, execErr error) {
	tx := mocks.NewMockTx(ctrl)
	db.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(tx, nil)
	var calls []any
	for i, stmt := range statements {
		var err error
		if i == len(statements)-1 {
			err = execErr
		}
		calls = append(calls, tx.EXPECT().ExecContext(gomock.Any(), stmt).Return(mockResult{}, err))
	}
	if execErr == nil {
		calls = append(calls, tx.EXPECT().Commit().Return(nil))
	}
	gomock.InOrder(calls...)
	tx.EXPECT().Rollback().Return(sql.ErrTxDone)
}

func TestRoleMembershipResource_MetadataAndConfigure(t *testing.T) {
	r := resource.NewRoleMembershipResource().(*resource.RoleMembershipResource)
	mresp := &fwresource.MetadataResponse{}
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "postgresql"}, mresp)
	if mresp.TypeName != "postgresql_role_membership" {
		t.Errorf("TypeName = %q", mresp.TypeName)
	}

	r.Configure(context.Background(), fwresource.ConfigureRequest{}, &fwresource.ConfigureResponse{})
	if r.DB != nil {
		t.Error("nil provider data must leave DB unset")
	}
	bad := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{ProviderData: "x"}, bad)
	if !hasDiag(bad.Diagnostics, "Unexpected Resource Configure Type") {
		t.Errorf("expected configure error, got %v", bad.Diagnostics)
	}
	ok := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{ProviderData: &common.DBWrapper{}}, ok)
	if ok.Diagnostics.HasError() || r.DB == nil {
		t.Errorf("configure failed: %v", ok.Diagnostics)
	}
}

func TestRoleMembershipResource_Create(t *testing.T) {
	tests := []struct {
		name    string
		admin   bool
		stmt    string
		execErr error
	}{
		{"plain", false, `GRANT "grp" TO "usr"`, nil},
		{"admin", true, `GRANT "grp" TO "usr" WITH ADMIN OPTION`, nil},
		{"error", false, `GRANT "grp" TO "usr"`, fmt.Errorf("permission denied")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := mocks.NewMockDBTX(ctrl)
			db.EXPECT().ExecContext(gomock.Any(), tt.stmt).Return(mockResult{}, tt.execErr)

			ctx := context.Background()
			s := membershipResourceSchema()
			req, resp := newCreateReqResp(ctx, s, membershipValue(ctx, s, nil, tt.admin))
			(&resource.RoleMembershipResource{DB: db}).Create(ctx, req, resp)

			if tt.execErr != nil {
				if !hasDiag(resp.Diagnostics, "Error granting role membership") {
					t.Fatalf("expected grant error, got %v", resp.Diagnostics)
				}
				return
			}
			var state resource.RoleMembershipResourceModel
			resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
			if resp.Diagnostics.HasError() || state.ID.ValueString() != "grp/usr" {
				t.Fatalf("id %q, diags %v", state.ID.ValueString(), resp.Diagnostics)
			}
		})
	}
}

func TestRoleMembershipResource_Read(t *testing.T) {
	tests := []struct {
		name    string
		admin   sql.NullBool
		scanErr error
		removed bool
		wantErr bool
	}{
		{name: "admin", admin: sql.NullBool{Bool: true, Valid: true}},
		{name: "gone", removed: true},
		{name: "error", scanErr: fmt.Errorf("connection reset"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := mocks.NewMockDBTX(ctrl)
			scanner := mocks.NewMockScanner(ctrl)
			db.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), "grp", "usr").Return(scanner)
			scanner.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
				*dest[0].(*sql.NullBool) = tt.admin
				return tt.scanErr
			})

			ctx := context.Background()
			s := membershipResourceSchema()
			req, resp := newReadReqResp(ctx, s, membershipValue(ctx, s, "grp/usr", false))
			(&resource.RoleMembershipResource{DB: db}).Read(ctx, req, resp)

			if tt.wantErr != hasDiag(resp.Diagnostics, "Error reading role membership") {
				t.Fatalf("diags %v", resp.Diagnostics)
			}
			if tt.removed != resp.State.Raw.IsNull() {
				t.Fatalf("removed = %v, want %v", resp.State.Raw.IsNull(), tt.removed)
			}
			if tt.admin.Valid {
				var state resource.RoleMembershipResourceModel
				resp.State.Get(ctx, &state)
				if !state.AdminOption.ValueBool() {
					t.Error("admin_option not read")
				}
			}
		})
	}
}

func TestRevokeMembershipStatements(t *testing.T) {
	grants := []resource.MembershipGrant{
		{Grantor: "", AdminOption: true, Revocable: true},
		{Grantor: "a", AdminOption: false, Revocable: true},
		{Grantor: "b", AdminOption: true, Revocable: true},
		{Grantor: "c", AdminOption: true, Revocable: false},
	}
	stmts, skipped := resource.RevokeMembershipStatements("grp", "usr", grants, false)
	want := []string{`REVOKE "grp" FROM "usr"`, `REVOKE "grp" FROM "usr" GRANTED BY "a"`, `REVOKE "grp" FROM "usr" GRANTED BY "b"`}
	if !slices.Equal(stmts, want) || !slices.Equal(skipped, []string{"c"}) {
		t.Errorf("revoke: %v %v", stmts, skipped)
	}
	stmts, skipped = resource.RevokeMembershipStatements("grp", "usr", grants, true)
	want = []string{`REVOKE ADMIN OPTION FOR "grp" FROM "usr"`, `REVOKE ADMIN OPTION FOR "grp" FROM "usr" GRANTED BY "b"`}
	if !slices.Equal(stmts, want) || !slices.Equal(skipped, []string{"c"}) {
		t.Errorf("admin only: %v %v", stmts, skipped)
	}
}

func TestReadMembershipGrants_errors(t *testing.T) {
	ctrl := gomock.NewController(t)
	db := mocks.NewMockDBTX(ctrl)
	db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "grp", "usr").Return(nil, fmt.Errorf("boom"))
	if _, err := resource.ReadMembershipGrants(context.Background(), db, "grp", "usr"); err == nil {
		t.Error("expected query error")
	}

	rows := mocks.NewMockRows(ctrl)
	db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "grp", "usr").Return(rows, nil)
	rows.EXPECT().Next().Return(true)
	rows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("bad row"))
	rows.EXPECT().Close().Return(nil)
	if _, err := resource.ReadMembershipGrants(context.Background(), db, "grp", "usr"); err == nil {
		t.Error("expected scan error")
	}
}

func TestRoleMembershipResource_Update(t *testing.T) {
	tests := []struct {
		name     string
		admin    bool
		grants   []resource.MembershipGrant
		queryErr error
		stmts    []string
		execErr  error
		wantErr  string
		wantWarn bool
	}{
		{name: "grant admin", admin: true, stmts: []string{`GRANT "grp" TO "usr" WITH ADMIN OPTION`}},
		{
			name:     "revoke admin per grantor",
			grants:   []resource.MembershipGrant{{Grantor: "postgres", AdminOption: true, Revocable: true}, {Grantor: "other", AdminOption: true}},
			stmts:    []string{`REVOKE ADMIN OPTION FOR "grp" FROM "usr" GRANTED BY "postgres"`},
			wantWarn: true,
		},
		{name: "read error", queryErr: fmt.Errorf("boom"), wantErr: "Error reading role membership"},
		{name: "exec error", admin: true, stmts: []string{`GRANT "grp" TO "usr" WITH ADMIN OPTION`}, execErr: fmt.Errorf("denied"), wantErr: "Error updating role membership"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := mocks.NewMockDBTX(ctrl)
			if !tt.admin {
				if tt.queryErr != nil {
					db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "grp", "usr").Return(nil, tt.queryErr)
				} else {
					db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "grp", "usr").Return(membershipGrantsRows(ctrl, tt.grants...), nil)
				}
			}
			if tt.stmts != nil {
				expectMembershipTx(ctrl, db, tt.stmts, tt.execErr)
			}

			ctx := context.Background()
			s := membershipResourceSchema()
			req, resp := newUpdateReqResp(ctx, s, membershipValue(ctx, s, "grp/usr", tt.admin), membershipValue(ctx, s, "grp/usr", !tt.admin))
			(&resource.RoleMembershipResource{DB: db}).Update(ctx, req, resp)

			if tt.wantErr != "" {
				if !hasDiag(resp.Diagnostics, tt.wantErr) {
					t.Fatalf("expected %q, got %v", tt.wantErr, resp.Diagnostics)
				}
				return
			}
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", resp.Diagnostics)
			}
			if got := resp.Diagnostics.WarningsCount() > 0; got != tt.wantWarn {
				t.Errorf("warning = %v, want %v", got, tt.wantWarn)
			}
		})
	}
}

func TestRoleMembershipResource_Delete(t *testing.T) {
	two := []resource.MembershipGrant{{Grantor: "other", Revocable: true}, {Grantor: "postgres", Revocable: true}}
	twoStmts := []string{`REVOKE "grp" FROM "usr" GRANTED BY "other"`, `REVOKE "grp" FROM "usr" GRANTED BY "postgres"`}
	tests := []struct {
		name     string
		grants   []resource.MembershipGrant
		queryErr error
		stmts    []string
		execErr  error
		wantErr  string
	}{
		{name: "every grantor", grants: two, stmts: twoStmts},
		{name: "already gone"},
		{name: "role dropped meanwhile", grants: two, stmts: twoStmts, execErr: &pq.Error{Code: "42704"}},
		{name: "read error", queryErr: fmt.Errorf("boom"), wantErr: "Error reading role membership"},
		{name: "exec error", grants: two, stmts: twoStmts, execErr: fmt.Errorf("dependent privileges exist"), wantErr: "Error revoking role membership"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := mocks.NewMockDBTX(ctrl)
			if tt.queryErr != nil {
				db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "grp", "usr").Return(nil, tt.queryErr)
			} else {
				db.EXPECT().QueryContext(gomock.Any(), gomock.Any(), "grp", "usr").Return(membershipGrantsRows(ctrl, tt.grants...), nil)
			}
			if tt.stmts != nil {
				expectMembershipTx(ctrl, db, tt.stmts, tt.execErr)
			}

			ctx := context.Background()
			s := membershipResourceSchema()
			req, resp := newDeleteReqResp(ctx, s, membershipValue(ctx, s, "grp/usr", false))
			(&resource.RoleMembershipResource{DB: db}).Delete(ctx, req, resp)

			if tt.wantErr == "" && resp.Diagnostics.HasError() {
				t.Fatalf("unexpected error: %v", resp.Diagnostics)
			}
			if tt.wantErr != "" && !hasDiag(resp.Diagnostics, tt.wantErr) {
				t.Fatalf("expected %q, got %v", tt.wantErr, resp.Diagnostics)
			}
		})
	}
}

func TestRoleMembershipResource_ImportState(t *testing.T) {
	s := membershipResourceSchema()
	r := &resource.RoleMembershipResource{}
	importAndAssertName(t, s, r, "grp/usr", "grp/usr")

	for _, id := range []string{"grp", "grp/", "a/b/c"} {
		ctx := context.Background()
		req, resp := newImportReqResp(ctx, s, id)
		r.ImportState(ctx, req, resp)
		if !hasDiag(resp.Diagnostics, "Invalid Import ID") {
			t.Errorf("%q: expected invalid id, got %v", id, resp.Diagnostics)
		}
	}
}
