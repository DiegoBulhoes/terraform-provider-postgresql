package datasource_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/datasource"
	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Unit tests for Configure error paths on all data sources.

func TestRoleDataSource_Configure_nilProviderData(t *testing.T) {
	d := &datasource.RoleDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestRoleDataSource_Configure_wrongType(t *testing.T) {
	d := &datasource.RoleDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestDatabaseDataSource_Configure_nilProviderData(t *testing.T) {
	d := &datasource.DatabaseDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestDatabaseDataSource_Configure_wrongType(t *testing.T) {
	d := &datasource.DatabaseDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestSchemasDataSource_Configure_wrongType(t *testing.T) {
	d := &datasource.SchemasDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: 42,
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestQueryDataSource_Configure_wrongType(t *testing.T) {
	d := &datasource.QueryDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: true,
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestVersionDataSource_Configure_wrongType(t *testing.T) {
	d := &datasource.VersionDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: []string{},
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestExtensionsDataSource_Configure_wrongType(t *testing.T) {
	d := &datasource.ExtensionsDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: "wrong",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestRolesDataSource_Configure_wrongType(t *testing.T) {
	d := &datasource.RolesDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: 3.14,
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestTablesDataSource_Configure_wrongType(t *testing.T) {
	d := &datasource.TablesDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: map[string]string{},
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

// Configure success tests — pass a valid *common.DBWrapper

func TestRoleDataSource_Configure_success(t *testing.T) {
	d := &datasource.RoleDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if d.DB == nil {
		t.Error("expected db to be set")
	}
}

func TestDatabaseDataSource_Configure_success(t *testing.T) {
	d := &datasource.DatabaseDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if d.DB == nil {
		t.Error("expected db to be set")
	}
}

func TestSchemasDataSource_Configure_success(t *testing.T) {
	d := &datasource.SchemasDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestQueryDataSource_Configure_success(t *testing.T) {
	d := &datasource.QueryDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestVersionDataSource_Configure_success(t *testing.T) {
	d := &datasource.VersionDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestExtensionsDataSource_Configure_success(t *testing.T) {
	d := &datasource.ExtensionsDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestRolesDataSource_Configure_success(t *testing.T) {
	d := &datasource.RolesDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestTablesDataSource_Configure_success(t *testing.T) {
	d := &datasource.TablesDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestUserDataSource_Configure_nilProviderData(t *testing.T) {
	d := &datasource.UserDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestUserDataSource_Configure_wrongType(t *testing.T) {
	d := &datasource.UserDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestUserDataSource_Configure_success(t *testing.T) {
	d := &datasource.UserDataSource{}
	resp := &fwdatasource.ConfigureResponse{}
	d.Configure(context.Background(), fwdatasource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

// New and Metadata tests

func TestNewRoleDataSource(t *testing.T) {
	if d := datasource.NewRoleDataSource(); d == nil {
		t.Error("expected non-nil")
	}
}

func TestNewUserDataSource(t *testing.T) {
	if d := datasource.NewUserDataSource(); d == nil {
		t.Error("expected non-nil")
	}
}

func TestNewDatabaseDataSource(t *testing.T) {
	if d := datasource.NewDatabaseDataSource(); d == nil {
		t.Error("expected non-nil")
	}
}

func TestNewRolesDataSource(t *testing.T) {
	if d := datasource.NewRolesDataSource(); d == nil {
		t.Error("expected non-nil")
	}
}

func TestNewSchemasDataSource(t *testing.T) {
	if d := datasource.NewSchemasDataSource(); d == nil {
		t.Error("expected non-nil")
	}
}

func TestNewTablesDataSource(t *testing.T) {
	if d := datasource.NewTablesDataSource(); d == nil {
		t.Error("expected non-nil")
	}
}

func TestNewExtensionsDataSource(t *testing.T) {
	if d := datasource.NewExtensionsDataSource(); d == nil {
		t.Error("expected non-nil")
	}
}

func TestNewVersionDataSource(t *testing.T) {
	if d := datasource.NewVersionDataSource(); d == nil {
		t.Error("expected non-nil")
	}
}

func TestNewQueryDataSource(t *testing.T) {
	if d := datasource.NewQueryDataSource(); d == nil {
		t.Error("expected non-nil")
	}
}

func TestRoleDataSource_Metadata(t *testing.T) {
	d := &datasource.RoleDataSource{}
	resp := &fwdatasource.MetadataResponse{}
	d.Metadata(context.Background(), fwdatasource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_role" {
		t.Errorf("expected postgresql_role, got %s", resp.TypeName)
	}
}

func TestUserDataSource_Metadata(t *testing.T) {
	d := &datasource.UserDataSource{}
	resp := &fwdatasource.MetadataResponse{}
	d.Metadata(context.Background(), fwdatasource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_user" {
		t.Errorf("expected postgresql_user, got %s", resp.TypeName)
	}
}

func TestDatabaseDataSource_Metadata(t *testing.T) {
	d := &datasource.DatabaseDataSource{}
	resp := &fwdatasource.MetadataResponse{}
	d.Metadata(context.Background(), fwdatasource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_database" {
		t.Errorf("expected postgresql_database, got %s", resp.TypeName)
	}
}

func TestRolesDataSource_Metadata(t *testing.T) {
	d := &datasource.RolesDataSource{}
	resp := &fwdatasource.MetadataResponse{}
	d.Metadata(context.Background(), fwdatasource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_roles" {
		t.Errorf("expected postgresql_roles, got %s", resp.TypeName)
	}
}

func TestSchemasDataSource_Metadata(t *testing.T) {
	d := &datasource.SchemasDataSource{}
	resp := &fwdatasource.MetadataResponse{}
	d.Metadata(context.Background(), fwdatasource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_schemas" {
		t.Errorf("expected postgresql_schemas, got %s", resp.TypeName)
	}
}

func TestTablesDataSource_Metadata(t *testing.T) {
	d := &datasource.TablesDataSource{}
	resp := &fwdatasource.MetadataResponse{}
	d.Metadata(context.Background(), fwdatasource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_tables" {
		t.Errorf("expected postgresql_tables, got %s", resp.TypeName)
	}
}

func TestExtensionsDataSource_Metadata(t *testing.T) {
	d := &datasource.ExtensionsDataSource{}
	resp := &fwdatasource.MetadataResponse{}
	d.Metadata(context.Background(), fwdatasource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_extensions" {
		t.Errorf("expected postgresql_extensions, got %s", resp.TypeName)
	}
}

func TestVersionDataSource_Metadata(t *testing.T) {
	d := &datasource.VersionDataSource{}
	resp := &fwdatasource.MetadataResponse{}
	d.Metadata(context.Background(), fwdatasource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_version" {
		t.Errorf("expected postgresql_version, got %s", resp.TypeName)
	}
}

func TestQueryDataSource_Metadata(t *testing.T) {
	d := &datasource.QueryDataSource{}
	resp := &fwdatasource.MetadataResponse{}
	d.Metadata(context.Background(), fwdatasource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_query" {
		t.Errorf("expected postgresql_query, got %s", resp.TypeName)
	}
}
