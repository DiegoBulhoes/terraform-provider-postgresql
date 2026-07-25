package datasource_test

import (
	"context"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/datasource"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/mocks"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"go.uber.org/mock/gomock"
)

// ---------------------------------------------------------------------------
// RolesDataSource — filter branch coverage
// ---------------------------------------------------------------------------

func rolesFilteredConfig(ctx context.Context, s dschema.Schema, loginOnly *bool, like, notLike *string) tftypes.Value {
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
	loginVal := tftypes.NewValue(tftypes.Bool, nil)
	if loginOnly != nil {
		loginVal = tftypes.NewValue(tftypes.Bool, *loginOnly)
	}
	likeVal := tftypes.NewValue(tftypes.String, nil)
	if like != nil {
		likeVal = tftypes.NewValue(tftypes.String, *like)
	}
	notLikeVal := tftypes.NewValue(tftypes.String, nil)
	if notLike != nil {
		notLikeVal = tftypes.NewValue(tftypes.String, *notLike)
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"like_pattern":     likeVal,
		"not_like_pattern": notLikeVal,
		"login_only":       loginVal,
		"roles":            tftypes.NewValue(tftypes.List{ElementType: rolesObjType}, nil),
	})
}

// rolesNoRowsQuery wires a QueryContext that returns zero rows with the given
// argument matcher count, then verifies the call count.
func rolesNoRowsQuery(t *testing.T, s dschema.Schema, cfg tftypes.Value) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil).AnyTimes()
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil).AnyTimes()
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil).AnyTimes()
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	req, resp := newReadReqResp(ctx, s, cfg)
	d := &datasource.RolesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestRolesDataSource_Read_loginOnlyFilter(t *testing.T) {
	s := rolesSchema()
	loginOnly := true
	rolesNoRowsQuery(t, s, rolesFilteredConfig(context.Background(), s, &loginOnly, nil, nil))
}

func TestRolesDataSource_Read_likePatternFilter(t *testing.T) {
	s := rolesSchema()
	like := "app_%"
	rolesNoRowsQuery(t, s, rolesFilteredConfig(context.Background(), s, nil, &like, nil))
}

func TestRolesDataSource_Read_notLikePatternFilter(t *testing.T) {
	s := rolesSchema()
	notLike := "pg_%"
	rolesNoRowsQuery(t, s, rolesFilteredConfig(context.Background(), s, nil, nil, &notLike))
}

func TestRolesDataSource_Read_allFiltersCombined(t *testing.T) {
	s := rolesSchema()
	loginOnly := true
	like := "app_%"
	notLike := "pg_%"
	rolesNoRowsQuery(t, s, rolesFilteredConfig(context.Background(), s, &loginOnly, &like, &notLike))
}

// ---------------------------------------------------------------------------
// SchemasDataSource — filter branch coverage
// ---------------------------------------------------------------------------

func schemasFilteredConfig(ctx context.Context, s dschema.Schema, includeSystem *bool, like, notLike *string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	schemasObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":  tftypes.String,
		"owner": tftypes.String,
	}}
	inclVal := tftypes.NewValue(tftypes.Bool, nil)
	if includeSystem != nil {
		inclVal = tftypes.NewValue(tftypes.Bool, *includeSystem)
	}
	likeVal := tftypes.NewValue(tftypes.String, nil)
	if like != nil {
		likeVal = tftypes.NewValue(tftypes.String, *like)
	}
	notLikeVal := tftypes.NewValue(tftypes.String, nil)
	if notLike != nil {
		notLikeVal = tftypes.NewValue(tftypes.String, *notLike)
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"database":               tftypes.NewValue(tftypes.String, nil),
		"like_pattern":           likeVal,
		"not_like_pattern":       notLikeVal,
		"include_system_schemas": inclVal,
		"schemas":                tftypes.NewValue(tftypes.List{ElementType: schemasObjType}, nil),
	})
}

func schemasNoRowsQuery(t *testing.T, s dschema.Schema, cfg tftypes.Value) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil).AnyTimes()
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil).AnyTimes()
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil).AnyTimes()
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	req, resp := newReadReqResp(ctx, s, cfg)
	d := &datasource.SchemasDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestSchemasDataSource_Read_includeSystemFilter(t *testing.T) {
	s := schemasSchemaFn()
	incl := true
	schemasNoRowsQuery(t, s, schemasFilteredConfig(context.Background(), s, &incl, nil, nil))
}

func TestSchemasDataSource_Read_likePatternFilter(t *testing.T) {
	s := schemasSchemaFn()
	like := "app_%"
	schemasNoRowsQuery(t, s, schemasFilteredConfig(context.Background(), s, nil, &like, nil))
}

func TestSchemasDataSource_Read_notLikePatternFilter(t *testing.T) {
	s := schemasSchemaFn()
	notLike := "pg_%"
	schemasNoRowsQuery(t, s, schemasFilteredConfig(context.Background(), s, nil, nil, &notLike))
}

func TestSchemasDataSource_Read_allFiltersCombined(t *testing.T) {
	s := schemasSchemaFn()
	incl := true
	like := "app_%"
	notLike := "pg_%"
	schemasNoRowsQuery(t, s, schemasFilteredConfig(context.Background(), s, &incl, &like, &notLike))
}

// ---------------------------------------------------------------------------
// TablesDataSource — filter branch coverage
// ---------------------------------------------------------------------------

func tablesFilteredConfig(ctx context.Context, s dschema.Schema, schema, like, notLike, tableType *string) tftypes.Value {
	tfType := s.Type().TerraformType(ctx)
	tablesObjType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":   tftypes.String,
		"schema": tftypes.String,
		"type":   tftypes.String,
		"owner":  tftypes.String,
	}}
	strOrNull := func(p *string) tftypes.Value {
		if p == nil {
			return tftypes.NewValue(tftypes.String, nil)
		}
		return tftypes.NewValue(tftypes.String, *p)
	}
	return tftypes.NewValue(tfType, map[string]tftypes.Value{
		"database":         tftypes.NewValue(tftypes.String, nil),
		"schema":           strOrNull(schema),
		"like_pattern":     strOrNull(like),
		"not_like_pattern": strOrNull(notLike),
		"table_type":       strOrNull(tableType),
		"tables":           tftypes.NewValue(tftypes.List{ElementType: tablesObjType}, nil),
	})
}

func tablesNoRowsQuery(t *testing.T, s dschema.Schema, cfg tftypes.Value) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockRows := mocks.NewMockRows(ctrl)

	// Tables query can have 0-4 args depending on filters.
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any()).Return(mockRows, nil).AnyTimes()
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil).AnyTimes()
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil).AnyTimes()
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil).AnyTimes()
	mockDB.EXPECT().QueryContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockRows, nil).AnyTimes()
	mockRows.EXPECT().Next().Return(false)
	mockRows.EXPECT().Err().Return(nil)
	mockRows.EXPECT().Close().Return(nil)

	ctx := context.Background()
	req, resp := newReadReqResp(ctx, s, cfg)
	d := &datasource.TablesDataSource{DB: mockDB}
	d.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestTablesDataSource_Read_schemaFilter(t *testing.T) {
	s := tablesSchema()
	schema := "public"
	tablesNoRowsQuery(t, s, tablesFilteredConfig(context.Background(), s, &schema, nil, nil, nil))
}

func TestTablesDataSource_Read_likePatternFilter(t *testing.T) {
	s := tablesSchema()
	like := "app_%"
	tablesNoRowsQuery(t, s, tablesFilteredConfig(context.Background(), s, nil, &like, nil, nil))
}

func TestTablesDataSource_Read_notLikePatternFilter(t *testing.T) {
	s := tablesSchema()
	notLike := "pg_%"
	tablesNoRowsQuery(t, s, tablesFilteredConfig(context.Background(), s, nil, nil, &notLike, nil))
}

func TestTablesDataSource_Read_tableTypeFilter(t *testing.T) {
	s := tablesSchema()
	tt := "BASE TABLE"
	tablesNoRowsQuery(t, s, tablesFilteredConfig(context.Background(), s, nil, nil, nil, &tt))
}

func TestTablesDataSource_Read_allFiltersCombined(t *testing.T) {
	s := tablesSchema()
	schema := "public"
	like := "app_%"
	notLike := "pg_%"
	tt := "BASE TABLE"
	tablesNoRowsQuery(t, s, tablesFilteredConfig(context.Background(), s, &schema, &like, &notLike, &tt))
}

// ---------------------------------------------------------------------------
// VersionDataSource — SHOW server_version_num parse path
// ---------------------------------------------------------------------------

// TestVersionDataSource_Read_versionRegexMiss exercises the else-branch where
// the "x.y" regex doesn't match (e.g., a devel build string).
func TestVersionDataSource_Read_versionRegexMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockScan1 := mocks.NewMockScanner(ctrl)
	mockScan2 := mocks.NewMockScanner(ctrl)

	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScan1)
	mockScan1.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "PostgreSQL devel-build"
		return nil
	})
	mockDB.EXPECT().QueryRowContext(gomock.Any(), gomock.Any()).Return(mockScan2)
	mockScan2.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		*dest[0].(*string) = "999999"
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
