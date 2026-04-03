package datasource_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/datasource"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/mocks"
	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"go.uber.org/mock/gomock"
)

// newReadReqResp builds a fwdatasource.ReadRequest and ReadResponse using the
// given schema and tftypes config value. The config is used both for Config
// and the initial (empty) State, which is required so that resp.State.Set
// can work after a successful Read.
func newReadReqResp(ctx context.Context, s dschema.Schema, configVal tftypes.Value) (fwdatasource.ReadRequest, *fwdatasource.ReadResponse) {
	tfType := s.Type().TerraformType(ctx)
	nullState := tftypes.NewValue(tfType, nil)

	req := fwdatasource.ReadRequest{
		Config: tfsdk.Config{
			Raw:    configVal,
			Schema: s,
		},
	}
	resp := &fwdatasource.ReadResponse{
		State: tfsdk.State{
			Raw:    nullState,
			Schema: s,
		},
	}
	return req, resp
}

// ---------------------------------------------------------------------------
// versionDataSource
// ---------------------------------------------------------------------------

func versionSchema() dschema.Schema {
	d := &datasource.VersionDataSource{}
	sreq := fwdatasource.SchemaRequest{}
	sresp := &fwdatasource.SchemaResponse{}
	d.Schema(context.Background(), sreq, sresp)
	return sresp.Schema
}

func versionConfigValue(ctx context.Context, s dschema.Schema) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"version":            tftypes.NewValue(tftypes.String, nil),
		"major":              tftypes.NewValue(tftypes.Number, nil),
		"minor":              tftypes.NewValue(tftypes.Number, nil),
		"server_version_num": tftypes.NewValue(tftypes.Number, nil),
	})
}

func TestVersionDataSource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("connection refused"))

	ctx := context.Background()
	s := versionSchema()
	req, resp := newReadReqResp(ctx, s, versionConfigValue(ctx, s))

	d := &datasource.VersionDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying version" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error querying version' diagnostic")
	}
}

func TestVersionDataSource_Read_serverVersionNumQueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner1 := mocks.NewMockScanner(ctrl)
	mockScanner2 := mocks.NewMockScanner(ctrl)

	// First query succeeds
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner1)
	mockScanner1.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "PostgreSQL 16.2 on x86_64"
		return nil
	})
	// Second query fails
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner2)
	mockScanner2.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("connection lost"))

	ctx := context.Background()
	s := versionSchema()
	req, resp := newReadReqResp(ctx, s, versionConfigValue(ctx, s))

	d := &datasource.VersionDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for server_version_num query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying server_version_num" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error querying server_version_num' diagnostic")
	}
}

func TestVersionDataSource_Read_serverVersionNumParseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner1 := mocks.NewMockScanner(ctrl)
	mockScanner2 := mocks.NewMockScanner(ctrl)

	// First query succeeds
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner1)
	mockScanner1.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "PostgreSQL 16.2 on x86_64"
		return nil
	})
	// Second query returns a non-numeric value
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScanner2)
	mockScanner2.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "not_a_number"
		return nil
	})

	ctx := context.Background()
	s := versionSchema()
	req, resp := newReadReqResp(ctx, s, versionConfigValue(ctx, s))

	d := &datasource.VersionDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for parse failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error parsing server_version_num" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error parsing server_version_num' diagnostic")
	}
}

// ---------------------------------------------------------------------------
// extensionsDataSource
// ---------------------------------------------------------------------------

func extensionsSchema() dschema.Schema {
	d := &datasource.ExtensionsDataSource{}
	sresp := &fwdatasource.SchemaResponse{}
	d.Schema(context.Background(), fwdatasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func extensionsConfigValue(ctx context.Context, s dschema.Schema) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	// extensions is a list of objects - use nil for computed
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"database":   tftypes.NewValue(tftypes.String, nil),
		"extensions": tftypes.NewValue(tftypes.List{ElementType: extensionsListTfType()}, nil),
	})
}

func extensionsListTfType() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":        tftypes.String,
		"version":     tftypes.String,
		"schema":      tftypes.String,
		"description": tftypes.String,
	}}
}

func TestExtensionsDataSource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := extensionsSchema()
	req, resp := newReadReqResp(ctx, s, extensionsConfigValue(ctx, s))

	d := &datasource.ExtensionsDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying extensions" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error querying extensions' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestExtensionsDataSource_Read_scanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("scan error"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := extensionsSchema()
	req, resp := newReadReqResp(ctx, s, extensionsConfigValue(ctx, s))

	d := &datasource.ExtensionsDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for scan failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error scanning extension row" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error scanning extension row' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestExtensionsDataSource_Read_rowsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "plpgsql"
		*dest[1].(*string) = "1.0"
		*dest[2].(*string) = "pg_catalog"
		*dest[3].(*string) = "PL/pgSQL"
		return nil
	})
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(fmt.Errorf("row iteration error"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := extensionsSchema()
	req, resp := newReadReqResp(ctx, s, extensionsConfigValue(ctx, s))

	d := &datasource.ExtensionsDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for rows.Err()")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error iterating extension rows" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error iterating extension rows' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// tablesDataSource
// ---------------------------------------------------------------------------

func tablesSchema() dschema.Schema {
	d := &datasource.TablesDataSource{}
	sresp := &fwdatasource.SchemaResponse{}
	d.Schema(context.Background(), fwdatasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func tablesConfigValue(ctx context.Context, s dschema.Schema) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	tablesObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":   tftypes.String,
		"schema": tftypes.String,
		"type":   tftypes.String,
		"owner":  tftypes.String,
	}}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"database":         tftypes.NewValue(tftypes.String, nil),
		"schema":           tftypes.NewValue(tftypes.String, nil),
		"like_pattern":     tftypes.NewValue(tftypes.String, nil),
		"not_like_pattern": tftypes.NewValue(tftypes.String, nil),
		"table_type":       tftypes.NewValue(tftypes.String, nil),
		"tables":           tftypes.NewValue(tftypes.List{ElementType: tablesObjType}, nil),
	})
}

func TestTablesDataSource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("relation does not exist"))

	ctx := context.Background()
	s := tablesSchema()
	req, resp := newReadReqResp(ctx, s, tablesConfigValue(ctx, s))

	d := &datasource.TablesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying tables" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error querying tables' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestTablesDataSource_Read_scanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("scan error"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := tablesSchema()
	req, resp := newReadReqResp(ctx, s, tablesConfigValue(ctx, s))

	d := &datasource.TablesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for scan failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error scanning table row" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error scanning table row' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestTablesDataSource_Read_rowsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "t1"
		*dest[1].(*string) = "public"
		*dest[2].(*string) = "BASE TABLE"
		*dest[3].(*string) = "postgres"
		return nil
	})
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(fmt.Errorf("row iteration error"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := tablesSchema()
	req, resp := newReadReqResp(ctx, s, tablesConfigValue(ctx, s))

	d := &datasource.TablesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for rows.Err()")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error iterating table rows" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error iterating table rows' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// roleDataSource
// ---------------------------------------------------------------------------

func roleSchema() dschema.Schema {
	d := &datasource.RoleDataSource{}
	sresp := &fwdatasource.SchemaResponse{}
	d.Schema(context.Background(), fwdatasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func roleConfigValue(ctx context.Context, s dschema.Schema, name string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"oid":              tftypes.NewValue(tftypes.Number, nil),
		"login":            tftypes.NewValue(tftypes.Bool, nil),
		"superuser":        tftypes.NewValue(tftypes.Bool, nil),
		"create_database":  tftypes.NewValue(tftypes.Bool, nil),
		"create_role":      tftypes.NewValue(tftypes.Bool, nil),
		"replication":      tftypes.NewValue(tftypes.Bool, nil),
		"connection_limit": tftypes.NewValue(tftypes.Number, nil),
		"valid_until":      tftypes.NewValue(tftypes.String, nil),
		"roles":            tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
	})
}

func TestRoleDataSource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("role not found"))

	ctx := context.Background()
	s := roleSchema()
	req, resp := newReadReqResp(ctx, s, roleConfigValue(ctx, s, "nonexistent"))

	d := &datasource.RoleDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error reading role" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error reading role' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestRoleDataSource_Read_membershipQueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	// First query (QueryRowContext) succeeds
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 16384
		*dest[1].(*bool) = true
		*dest[2].(*bool) = false
		*dest[3].(*bool) = false
		*dest[4].(*bool) = false
		*dest[5].(*bool) = false
		*dest[6].(*int64) = -1
		*dest[7].(*sql.NullString) = sql.NullString{Valid: false}
		return nil
	})

	// Membership query fails
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := roleSchema()
	req, resp := newReadReqResp(ctx, s, roleConfigValue(ctx, s, "testrole"))

	d := &datasource.RoleDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for membership query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error reading role memberships" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error reading role memberships' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestRoleDataSource_Read_membershipScanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	// First query succeeds
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 16384
		*dest[1].(*bool) = true
		*dest[2].(*bool) = false
		*dest[3].(*bool) = false
		*dest[4].(*bool) = false
		*dest[5].(*bool) = false
		*dest[6].(*int64) = -1
		*dest[7].(*sql.NullString) = sql.NullString{Valid: false}
		return nil
	})

	// Membership query returns rows
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("scan error"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := roleSchema()
	req, resp := newReadReqResp(ctx, s, roleConfigValue(ctx, s, "testrole"))

	d := &datasource.RoleDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for membership scan failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error scanning role membership" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error scanning role membership' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestRoleDataSource_Read_membershipRowsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	// First query succeeds
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 16384
		*dest[1].(*bool) = true
		*dest[2].(*bool) = false
		*dest[3].(*bool) = false
		*dest[4].(*bool) = false
		*dest[5].(*bool) = false
		*dest[6].(*int64) = -1
		*dest[7].(*sql.NullString) = sql.NullString{Valid: false}
		return nil
	})

	// Membership query returns rows that iterate successfully then report error
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "admin"
		return nil
	})
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(fmt.Errorf("row iteration failure"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := roleSchema()
	req, resp := newReadReqResp(ctx, s, roleConfigValue(ctx, s, "testrole"))

	d := &datasource.RoleDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for rows.Err()")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error iterating role memberships" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error iterating role memberships' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// userDataSource
// ---------------------------------------------------------------------------

func userSchema() dschema.Schema {
	d := &datasource.UserDataSource{}
	sresp := &fwdatasource.SchemaResponse{}
	d.Schema(context.Background(), fwdatasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func userConfigValue(ctx context.Context, s dschema.Schema, name string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":             tftypes.NewValue(tftypes.String, name),
		"oid":              tftypes.NewValue(tftypes.Number, nil),
		"superuser":        tftypes.NewValue(tftypes.Bool, nil),
		"create_database":  tftypes.NewValue(tftypes.Bool, nil),
		"create_role":      tftypes.NewValue(tftypes.Bool, nil),
		"replication":      tftypes.NewValue(tftypes.Bool, nil),
		"connection_limit": tftypes.NewValue(tftypes.Number, nil),
		"valid_until":      tftypes.NewValue(tftypes.String, nil),
		"roles":            tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
	})
}

// setupRoleQueryRowSuccess sets up a successful QueryRowContext + Scan for
// role/user data sources that query pg_roles.
func setupRoleQueryRowSuccess(mockDB *mocks.MockDBTX, mockScanner *mocks.MockScanner, login bool, connLimit int64) {
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 16384
		*dest[1].(*bool) = login
		*dest[2].(*bool) = false
		*dest[3].(*bool) = false
		*dest[4].(*bool) = false
		*dest[5].(*bool) = false
		*dest[6].(*int64) = connLimit
		*dest[7].(*sql.NullString) = sql.NullString{Valid: false}
		return nil
	})
}

// setupMembershipRowsSuccess sets up a successful QueryContext that returns
// the given role names.
func setupMembershipRowsSuccess(mockDB *mocks.MockDBTX, mockRows *mocks.MockRows, roleNames []string) {
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	for _, name := range roleNames {
		n := name // capture
		mockRows.EXPECT().Next().Return(true)
		mockRows.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = n
			return nil
		})
	}
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)
}

func TestUserDataSource_Read_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	// First query succeeds with login=true, createDB=true
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 16384
		*dest[1].(*bool) = true
		*dest[2].(*bool) = false
		*dest[3].(*bool) = true
		*dest[4].(*bool) = false
		*dest[5].(*bool) = false
		*dest[6].(*int64) = 10
		*dest[7].(*sql.NullString) = sql.NullString{Valid: false}
		return nil
	})

	// Membership query returns two roles
	setupMembershipRowsSuccess(mockDB, mockRows, []string{"admin", "developers"})

	ctx := context.Background()
	s := userSchema()
	req, resp := newReadReqResp(ctx, s, userConfigValue(ctx, s, "testuser"))

	d := &datasource.UserDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestUserDataSource_Read_successWithWarningNoLogin(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	// Return a role without login privilege
	setupRoleQueryRowSuccess(mockDB, mockScanner, false, -1)

	// Membership query returns no roles
	setupMembershipRowsSuccess(mockDB, mockRows, nil)

	ctx := context.Background()
	s := userSchema()
	req, resp := newReadReqResp(ctx, s, userConfigValue(ctx, s, "norole"))

	d := &datasource.UserDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}

	// Should have a warning about missing LOGIN privilege
	foundWarning := false
	for _, diag := range resp.Diagnostics.Warnings() {
		if diag.Summary() == "Role is not a user" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected 'Role is not a user' warning diagnostic")
	}
}

func TestUserDataSource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("user not found"))

	ctx := context.Background()
	s := userSchema()
	req, resp := newReadReqResp(ctx, s, userConfigValue(ctx, s, "nonexistent"))

	d := &datasource.UserDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error reading user" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error reading user' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestUserDataSource_Read_membershipQueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	// First query succeeds
	setupRoleQueryRowSuccess(mockDB, mockScanner, true, -1)

	// Membership query fails
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := userSchema()
	req, resp := newReadReqResp(ctx, s, userConfigValue(ctx, s, "testuser"))

	d := &datasource.UserDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for membership query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error reading role memberships" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error reading role memberships' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestUserDataSource_Read_membershipScanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	// First query succeeds
	setupRoleQueryRowSuccess(mockDB, mockScanner, true, -1)

	// Membership query returns rows with scan error
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("scan error"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := userSchema()
	req, resp := newReadReqResp(ctx, s, userConfigValue(ctx, s, "testuser"))

	d := &datasource.UserDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for membership scan failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error scanning role membership" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error scanning role membership' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestUserDataSource_Read_membershipRowsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	// First query succeeds
	setupRoleQueryRowSuccess(mockDB, mockScanner, true, -1)

	// Membership query returns rows that iterate then report error
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "admin"
		return nil
	})
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(fmt.Errorf("row iteration failure"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := userSchema()
	req, resp := newReadReqResp(ctx, s, userConfigValue(ctx, s, "testuser"))

	d := &datasource.UserDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for rows.Err()")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error iterating role memberships" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error iterating role memberships' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// rolesDataSource
// ---------------------------------------------------------------------------

func rolesSchema() dschema.Schema {
	d := &datasource.RolesDataSource{}
	sresp := &fwdatasource.SchemaResponse{}
	d.Schema(context.Background(), fwdatasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func rolesConfigValue(ctx context.Context, s dschema.Schema) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	rolesObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":             tftypes.String,
		"oid":              tftypes.Number,
		"login":            tftypes.Bool,
		"superuser":        tftypes.Bool,
		"create_database":  tftypes.Bool,
		"create_role":      tftypes.Bool,
		"replication":      tftypes.Bool,
		"connection_limit": tftypes.Number,
	}}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"like_pattern":     tftypes.NewValue(tftypes.String, nil),
		"not_like_pattern": tftypes.NewValue(tftypes.String, nil),
		"login_only":       tftypes.NewValue(tftypes.Bool, nil),
		"roles":            tftypes.NewValue(tftypes.List{ElementType: rolesObjType}, nil),
	})
}

func TestRolesDataSource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("access denied"))

	ctx := context.Background()
	s := rolesSchema()
	req, resp := newReadReqResp(ctx, s, rolesConfigValue(ctx, s))

	d := &datasource.RolesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying roles" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error querying roles' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestRolesDataSource_Read_scanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("scan error"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := rolesSchema()
	req, resp := newReadReqResp(ctx, s, rolesConfigValue(ctx, s))

	d := &datasource.RolesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for scan failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error scanning role row" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error scanning role row' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestRolesDataSource_Read_rowsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "postgres"
		*dest[1].(*int64) = 10
		*dest[2].(*bool) = true
		*dest[3].(*bool) = true
		*dest[4].(*bool) = true
		*dest[5].(*bool) = true
		*dest[6].(*bool) = true
		*dest[7].(*int64) = -1
		return nil
	})
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(fmt.Errorf("row iteration error"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := rolesSchema()
	req, resp := newReadReqResp(ctx, s, rolesConfigValue(ctx, s))

	d := &datasource.RolesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for rows.Err()")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error iterating role rows" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error iterating role rows' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// schemasDataSource
// ---------------------------------------------------------------------------

func schemasSchemaFn() dschema.Schema {
	d := &datasource.SchemasDataSource{}
	sresp := &fwdatasource.SchemaResponse{}
	d.Schema(context.Background(), fwdatasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func schemasConfigValue(ctx context.Context, s dschema.Schema) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	schemasObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":  tftypes.String,
		"owner": tftypes.String,
	}}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"database":               tftypes.NewValue(tftypes.String, nil),
		"like_pattern":           tftypes.NewValue(tftypes.String, nil),
		"not_like_pattern":       tftypes.NewValue(tftypes.String, nil),
		"include_system_schemas": tftypes.NewValue(tftypes.Bool, nil),
		"schemas":                tftypes.NewValue(tftypes.List{ElementType: schemasObjType}, nil),
	})
}

func TestSchemasDataSource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := schemasSchemaFn()
	req, resp := newReadReqResp(ctx, s, schemasConfigValue(ctx, s))

	d := &datasource.SchemasDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error querying schemas" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error querying schemas' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestSchemasDataSource_Read_scanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).Return(fmt.Errorf("scan error"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := schemasSchemaFn()
	req, resp := newReadReqResp(ctx, s, schemasConfigValue(ctx, s))

	d := &datasource.SchemasDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for scan failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error scanning schema row" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error scanning schema row' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestSchemasDataSource_Read_rowsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "public"
		*dest[1].(*string) = "postgres"
		return nil
	})
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(fmt.Errorf("row iteration error"))
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	s := schemasSchemaFn()
	req, resp := newReadReqResp(ctx, s, schemasConfigValue(ctx, s))

	d := &datasource.SchemasDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for rows.Err()")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error iterating schema rows" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error iterating schema rows' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// databaseDataSource
// ---------------------------------------------------------------------------

func databaseSchema() dschema.Schema {
	d := &datasource.DatabaseDataSource{}
	sresp := &fwdatasource.SchemaResponse{}
	d.Schema(context.Background(), fwdatasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func databaseConfigValue(ctx context.Context, s dschema.Schema, name string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"name":              tftypes.NewValue(tftypes.String, name),
		"oid":               tftypes.NewValue(tftypes.Number, nil),
		"owner":             tftypes.NewValue(tftypes.String, nil),
		"encoding":          tftypes.NewValue(tftypes.String, nil),
		"lc_collate":        tftypes.NewValue(tftypes.String, nil),
		"lc_ctype":          tftypes.NewValue(tftypes.String, nil),
		"tablespace_name":   tftypes.NewValue(tftypes.String, nil),
		"connection_limit":  tftypes.NewValue(tftypes.Number, nil),
		"allow_connections": tftypes.NewValue(tftypes.Bool, nil),
		"is_template":       tftypes.NewValue(tftypes.Bool, nil),
	})
}

func TestDatabaseDataSource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("database not found"))

	ctx := context.Background()
	s := databaseSchema()
	req, resp := newReadReqResp(ctx, s, databaseConfigValue(ctx, s, "nonexistent"))

	d := &datasource.DatabaseDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error reading database" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error reading database' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// queryDataSource
// ---------------------------------------------------------------------------

func querySchema() dschema.Schema {
	d := &datasource.QueryDataSource{}
	sresp := &fwdatasource.SchemaResponse{}
	d.Schema(context.Background(), fwdatasource.SchemaRequest{}, sresp)
	return sresp.Schema
}

func queryConfigValue(ctx context.Context, s dschema.Schema, query, database string) tftypes.Value {
	return queryConfigValueWithDestructive(ctx, s, query, database, nil)
}

func queryConfigValueWithDestructive(ctx context.Context, s dschema.Schema, query, database string, allowDestructive *bool) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	var destructiveVal tftypes.Value
	if allowDestructive != nil {
		destructiveVal = tftypes.NewValue(tftypes.Bool, *allowDestructive)
	} else {
		destructiveVal = tftypes.NewValue(tftypes.Bool, nil)
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"query":             tftypes.NewValue(tftypes.String, query),
		"database":          tftypes.NewValue(tftypes.String, database),
		"allow_destructive": destructiveVal,
		"rows":              tftypes.NewValue(tftypes.List{ElementType: tftypes.Map{ElementType: tftypes.String}}, nil),
	})
}

func TestQueryDataSource_Read_nonSelectQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "INSERT INTO foo VALUES(1)", "postgres"))

	d := &datasource.QueryDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for non-SELECT query")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Invalid Query" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Invalid Query' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestQueryDataSource_Read_beginTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("cannot start transaction"))

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "SELECT 1", "postgres"))

	d := &datasource.QueryDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for BeginTx failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error starting transaction" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error starting transaction' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestQueryDataSource_Read_queryExecError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("syntax error"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "SELECT bad_syntax", "postgres"))

	d := &datasource.QueryDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for query execution failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error executing query" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Error executing query' diagnostic, got: %v", resp.Diagnostics.Errors())
	}
}

func TestQueryDataSource_Read_scanError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Columns().Return([]string{"col1"}, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any()).Return(fmt.Errorf("scan failure during iteration"))
	mockRows.EXPECT().Close().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "SELECT col1 FROM t", "postgres"))

	d := &datasource.QueryDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for scan/rows error")
	}
}

func TestQueryDataSource_Read_withCTE(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Columns().Return([]string{"num"}, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*sql.NullString) = sql.NullString{String: "1", Valid: true}
		return nil
	})
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "WITH cte AS (SELECT 1 AS num) SELECT num FROM cte", "postgres"))

	d := &datasource.QueryDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

// ---------------------------------------------------------------------------
// allow_destructive tests
// ---------------------------------------------------------------------------

func TestQueryDataSource_Read_destructiveBlocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	ctx := context.Background()
	s := querySchema()
	// DELETE without allow_destructive -> should be rejected
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "DELETE FROM sessions WHERE expired = true", "postgres"))

	d := &datasource.QueryDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for DELETE without allow_destructive")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Invalid Query" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Invalid Query' diagnostic")
	}
}

func TestQueryDataSource_Read_destructiveAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Columns().Return([]string{"id"}, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*sql.NullString) = sql.NullString{String: "1", Valid: true}
		return nil
	})
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := querySchema()
	allowDestructive := true
	req, resp := newReadReqResp(ctx, s, queryConfigValueWithDestructive(ctx, s, "DELETE FROM sessions WHERE expired = true RETURNING id", "postgres", &allowDestructive))

	d := &datasource.QueryDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestQueryDataSource_Read_destructiveFalseBlocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	ctx := context.Background()
	s := querySchema()
	// Explicit allow_destructive = false -> still blocked
	allowDestructive := false
	req, resp := newReadReqResp(ctx, s, queryConfigValueWithDestructive(ctx, s, "DROP TABLE users", "postgres", &allowDestructive))

	d := &datasource.QueryDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for DROP with allow_destructive=false")
	}
}

func TestQueryDataSource_Read_commitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Columns().Return([]string{"num"}, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*sql.NullString) = sql.NullString{String: "1", Valid: true}
		return nil
	})
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)
	mockTx.EXPECT().Commit().Return(fmt.Errorf("commit failed"))
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "SELECT 1 AS num", "postgres"))

	d := &datasource.QueryDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for commit failure")
	}
	found := false
	for _, diag := range resp.Diagnostics.Errors() {
		if diag.Summary() == "Error committing transaction" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Error committing transaction' diagnostic")
	}
}

// ---------------------------------------------------------------------------
// Success path tests
// ---------------------------------------------------------------------------

func TestVersionDataSource_Read_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	scanner1 := mocks.NewMockScanner(ctrl)
	scanner2 := mocks.NewMockScanner(ctrl)

	gomock.InOrder(
		mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(scanner1),
		mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(scanner2),
	)
	scanner1.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "PostgreSQL 16.2 on x86_64"
		return nil
	})
	scanner2.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "160002"
		return nil
	})

	ctx := context.Background()
	s := versionSchema()
	req, resp := newReadReqResp(ctx, s, versionConfigValue(ctx, s))

	d := &datasource.VersionDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestDatabaseDataSource_Read_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 16384
		*dest[1].(*string) = "postgres"
		*dest[2].(*string) = "UTF8"
		*dest[3].(*string) = "en_US.UTF-8"
		*dest[4].(*string) = "en_US.UTF-8"
		*dest[5].(*string) = "pg_default"
		*dest[6].(*int64) = -1
		*dest[7].(*bool) = true
		*dest[8].(*bool) = false
		return nil
	})

	ctx := context.Background()
	s := databaseSchema()
	req, resp := newReadReqResp(ctx, s, databaseConfigValue(ctx, s, "mydb"))

	d := &datasource.DatabaseDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestExtensionsDataSource_Read_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "plpgsql"
			*dest[1].(*string) = "1.0"
			*dest[2].(*string) = "pg_catalog"
			*dest[3].(*string) = "PL/pgSQL procedural language"
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
		mockRows.EXPECT().Err().Return(nil),
		mockRows.EXPECT().Close().Return(nil),
	)

	ctx := context.Background()
	s := extensionsSchema()
	req, resp := newReadReqResp(ctx, s, extensionsConfigValue(ctx, s))

	d := &datasource.ExtensionsDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestRoleDataSource_Read_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScanner := mocks.NewMockScanner(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockScanner)
	mockScanner.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*int64) = 99
		*dest[1].(*bool) = true
		*dest[2].(*bool) = false
		*dest[3].(*bool) = true
		*dest[4].(*bool) = false
		*dest[5].(*bool) = false
		*dest[6].(*int64) = -1
		*dest[7].(*sql.NullString) = sql.NullString{String: "2030-12-31T23:59:59Z", Valid: true}
		return nil
	})

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "admin"
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
		mockRows.EXPECT().Err().Return(nil),
		mockRows.EXPECT().Close().Return(nil),
	)

	ctx := context.Background()
	s := roleSchema()
	req, resp := newReadReqResp(ctx, s, roleConfigValue(ctx, s, "testrole"))

	d := &datasource.RoleDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestRolesDataSource_Read_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "postgres"
			*dest[1].(*int64) = 10
			*dest[2].(*bool) = true  // login
			*dest[3].(*bool) = true  // superuser
			*dest[4].(*bool) = true  // createdb
			*dest[5].(*bool) = true  // createrole
			*dest[6].(*bool) = false // replication
			*dest[7].(*int64) = -1
			return nil
		}),
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "app_user"
			*dest[1].(*int64) = 16384
			*dest[2].(*bool) = true  // login
			*dest[3].(*bool) = false // superuser
			*dest[4].(*bool) = false // createdb
			*dest[5].(*bool) = false // createrole
			*dest[6].(*bool) = false // replication
			*dest[7].(*int64) = 5
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
		mockRows.EXPECT().Err().Return(nil),
		mockRows.EXPECT().Close().Return(nil),
	)

	ctx := context.Background()
	s := rolesSchema()
	req, resp := newReadReqResp(ctx, s, rolesConfigValue(ctx, s))

	d := &datasource.RolesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestSchemasDataSource_Read_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "public"
			*dest[1].(*string) = "postgres"
			return nil
		}),
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "app"
			*dest[1].(*string) = "app_owner"
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
		mockRows.EXPECT().Err().Return(nil),
		mockRows.EXPECT().Close().Return(nil),
	)

	ctx := context.Background()
	s := schemasSchemaFn()
	req, resp := newReadReqResp(ctx, s, schemasConfigValue(ctx, s))

	d := &datasource.SchemasDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestQueryDataSource_Read_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	mockRows.EXPECT().Columns().Return([]string{"id", "name"}, nil)

	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*sql.NullString) = sql.NullString{String: "1", Valid: true}
			*dest[1].(*sql.NullString) = sql.NullString{String: "alice", Valid: true}
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
		mockRows.EXPECT().Err().Return(nil),
		mockRows.EXPECT().Close().Return(nil),
	)
	mockTx.EXPECT().Commit().Return(nil)
	mockTx.EXPECT().Rollback().Return(nil)

	ctx := context.Background()
	s := querySchema()
	req, resp := newReadReqResp(ctx, s, queryConfigValue(ctx, s, "SELECT id, name FROM users", "postgres"))

	d := &datasource.QueryDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestRolesDataSource_Read_successWithLoginFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	// login_only=true adds WHERE rolcanlogin = true, no positional args
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "app_user"
			*dest[1].(*int64) = 16384
			*dest[2].(*bool) = true
			*dest[3].(*bool) = false
			*dest[4].(*bool) = false
			*dest[5].(*bool) = false
			*dest[6].(*bool) = false
			*dest[7].(*int64) = -1
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
		mockRows.EXPECT().Err().Return(nil),
		mockRows.EXPECT().Close().Return(nil),
	)

	ctx := context.Background()
	s := rolesSchema()
	tfType := s.Type().TerraformType(ctx)
	rolesObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name": tftypes.String, "oid": tftypes.Number, "login": tftypes.Bool,
		"superuser": tftypes.Bool, "create_database": tftypes.Bool, "create_role": tftypes.Bool,
		"replication": tftypes.Bool, "connection_limit": tftypes.Number,
	}}
	configVal := tftypes.NewValue(tfType, map[string]tftypes.Value{
		"like_pattern":     tftypes.NewValue(tftypes.String, nil),
		"not_like_pattern": tftypes.NewValue(tftypes.String, nil),
		"login_only":       tftypes.NewValue(tftypes.Bool, true),
		"roles":            tftypes.NewValue(tftypes.List{ElementType: rolesObjType}, nil),
	})
	req, resp := newReadReqResp(ctx, s, configVal)

	d := &datasource.RolesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestRolesDataSource_Read_successWithLikePattern(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(false),
		mockRows.EXPECT().Err().Return(nil),
		mockRows.EXPECT().Close().Return(nil),
	)

	ctx := context.Background()
	s := rolesSchema()
	tfType := s.Type().TerraformType(ctx)
	rolesObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name": tftypes.String, "oid": tftypes.Number, "login": tftypes.Bool,
		"superuser": tftypes.Bool, "create_database": tftypes.Bool, "create_role": tftypes.Bool,
		"replication": tftypes.Bool, "connection_limit": tftypes.Number,
	}}
	configVal := tftypes.NewValue(tfType, map[string]tftypes.Value{
		"like_pattern":     tftypes.NewValue(tftypes.String, "app_%"),
		"not_like_pattern": tftypes.NewValue(tftypes.String, nil),
		"login_only":       tftypes.NewValue(tftypes.Bool, nil),
		"roles":            tftypes.NewValue(tftypes.List{ElementType: rolesObjType}, nil),
	})
	req, resp := newReadReqResp(ctx, s, configVal)

	d := &datasource.RolesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestSchemasDataSource_Read_successWithSystemSchemas(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "pg_catalog"
			*dest[1].(*string) = "postgres"
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
		mockRows.EXPECT().Err().Return(nil),
		mockRows.EXPECT().Close().Return(nil),
	)

	ctx := context.Background()
	s := schemasSchemaFn()
	tfType := s.Type().TerraformType(ctx)
	schemasObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name": tftypes.String, "owner": tftypes.String,
	}}
	configVal := tftypes.NewValue(tfType, map[string]tftypes.Value{
		"database":               tftypes.NewValue(tftypes.String, nil),
		"like_pattern":           tftypes.NewValue(tftypes.String, nil),
		"not_like_pattern":       tftypes.NewValue(tftypes.String, nil),
		"include_system_schemas": tftypes.NewValue(tftypes.Bool, true),
		"schemas":                tftypes.NewValue(tftypes.List{ElementType: schemasObjType}, nil),
	})
	req, resp := newReadReqResp(ctx, s, configVal)

	d := &datasource.SchemasDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestTablesDataSource_Read_successWithFilters(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	// With schema filter, expects 1 positional arg
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "users"
			*dest[1].(*string) = "public"
			*dest[2].(*string) = "BASE TABLE"
			*dest[3].(*string) = "postgres"
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
		mockRows.EXPECT().Err().Return(nil),
		mockRows.EXPECT().Close().Return(nil),
	)

	ctx := context.Background()
	s := tablesSchema()
	tfType := s.Type().TerraformType(ctx)
	tablesObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name": tftypes.String, "schema": tftypes.String, "type": tftypes.String, "owner": tftypes.String,
	}}
	configVal := tftypes.NewValue(tfType, map[string]tftypes.Value{
		"database":         tftypes.NewValue(tftypes.String, nil),
		"schema":           tftypes.NewValue(tftypes.String, "public"),
		"like_pattern":     tftypes.NewValue(tftypes.String, nil),
		"not_like_pattern": tftypes.NewValue(tftypes.String, nil),
		"table_type":       tftypes.NewValue(tftypes.String, nil),
		"tables":           tftypes.NewValue(tftypes.List{ElementType: tablesObjType}, nil),
	})
	req, resp := newReadReqResp(ctx, s, configVal)

	d := &datasource.TablesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestTablesDataSource_Read_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil)
	gomock.InOrder(
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "users"
			*dest[1].(*string) = "public"
			*dest[2].(*string) = "BASE TABLE"
			*dest[3].(*string) = "postgres"
			return nil
		}),
		mockRows.EXPECT().Next().Return(true),
		mockRows.EXPECT().Scan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
			*dest[0].(*string) = "orders"
			*dest[1].(*string) = "public"
			*dest[2].(*string) = "BASE TABLE"
			*dest[3].(*string) = "postgres"
			return nil
		}),
		mockRows.EXPECT().Next().Return(false),
		mockRows.EXPECT().Err().Return(nil),
		mockRows.EXPECT().Close().Return(nil),
	)

	ctx := context.Background()
	s := tablesSchema()
	req, resp := newReadReqResp(ctx, s, tablesConfigValue(ctx, s))

	d := &datasource.TablesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}
