package resource_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/resource"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestRoleResource_Configure_nilProviderData(t *testing.T) {
	r := &resource.RoleResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
	if r.DB != nil {
		t.Error("expected db to remain nil")
	}
}

func TestRoleResource_Configure_wrongType(t *testing.T) {
	r := &resource.RoleResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestDatabaseResource_Configure_nilProviderData(t *testing.T) {
	r := &resource.DatabaseResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestDatabaseResource_Configure_wrongType(t *testing.T) {
	r := &resource.DatabaseResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestSchemaResource_Configure_nilProviderData(t *testing.T) {
	r := &resource.SchemaResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestSchemaResource_Configure_wrongType(t *testing.T) {
	r := &resource.SchemaResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestGrantResource_Configure_nilProviderData(t *testing.T) {
	r := &resource.GrantResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestGrantResource_Configure_wrongType(t *testing.T) {
	r := &resource.GrantResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestUserResource_Configure_nilProviderData(t *testing.T) {
	r := &resource.UserResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
	if r.DB != nil {
		t.Error("expected db to remain nil")
	}
}

func TestUserResource_Configure_wrongType(t *testing.T) {
	r := &resource.UserResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

// Configure success tests

func TestRoleResource_Configure_success(t *testing.T) {
	r := &resource.RoleResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	if r.DB == nil {
		t.Error("expected db to be set")
	}
}

func TestDatabaseResource_Configure_success(t *testing.T) {
	r := &resource.DatabaseResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestSchemaResource_Configure_success(t *testing.T) {
	r := &resource.SchemaResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestGrantResource_Configure_success(t *testing.T) {
	r := &resource.GrantResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

func TestUserResource_Configure_success(t *testing.T) {
	r := &resource.UserResource{}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{
		ProviderData: common.NewDBWrapper(&sql.DB{}),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected error: %v", resp.Diagnostics.Errors())
	}
}

// New and Metadata tests

func TestNewRoleResource(t *testing.T) {
	if r := resource.NewRoleResource(); r == nil {
		t.Error("expected non-nil")
	}
}

func TestNewUserResource(t *testing.T) {
	if r := resource.NewUserResource(); r == nil {
		t.Error("expected non-nil")
	}
}

func TestNewDatabaseResource(t *testing.T) {
	if r := resource.NewDatabaseResource(); r == nil {
		t.Error("expected non-nil")
	}
}

func TestNewSchemaResource(t *testing.T) {
	if r := resource.NewSchemaResource(); r == nil {
		t.Error("expected non-nil")
	}
}

func TestNewGrantResource(t *testing.T) {
	if r := resource.NewGrantResource(); r == nil {
		t.Error("expected non-nil")
	}
}

func TestRoleResource_Metadata(t *testing.T) {
	r := &resource.RoleResource{}
	resp := &fwresource.MetadataResponse{}
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_role" {
		t.Errorf("expected postgresql_role, got %s", resp.TypeName)
	}
}

func TestUserResource_Metadata(t *testing.T) {
	r := &resource.UserResource{}
	resp := &fwresource.MetadataResponse{}
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_user" {
		t.Errorf("expected postgresql_user, got %s", resp.TypeName)
	}
}

func TestDatabaseResource_Metadata(t *testing.T) {
	r := &resource.DatabaseResource{}
	resp := &fwresource.MetadataResponse{}
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_database" {
		t.Errorf("expected postgresql_database, got %s", resp.TypeName)
	}
}

func TestSchemaResource_Metadata(t *testing.T) {
	r := &resource.SchemaResource{}
	resp := &fwresource.MetadataResponse{}
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_schema" {
		t.Errorf("expected postgresql_schema, got %s", resp.TypeName)
	}
}

func TestGrantResource_Metadata(t *testing.T) {
	r := &resource.GrantResource{}
	resp := &fwresource.MetadataResponse{}
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "postgresql"}, resp)
	if resp.TypeName != "postgresql_grant" {
		t.Errorf("expected postgresql_grant, got %s", resp.TypeName)
	}
}
