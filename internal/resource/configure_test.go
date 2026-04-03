package resource

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Unit tests for Configure error paths on all resources.
// These cover the 71.4% → 100% gap in Configure methods.

func TestRoleResource_Configure_nilProviderData(t *testing.T) {
	r := &roleResource{}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
	if r.db != nil {
		t.Error("expected db to remain nil")
	}
}

func TestRoleResource_Configure_wrongType(t *testing.T) {
	r := &roleResource{}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestDatabaseResource_Configure_nilProviderData(t *testing.T) {
	r := &databaseResource{}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestDatabaseResource_Configure_wrongType(t *testing.T) {
	r := &databaseResource{}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestSchemaResource_Configure_nilProviderData(t *testing.T) {
	r := &schemaResource{}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestSchemaResource_Configure_wrongType(t *testing.T) {
	r := &schemaResource{}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestGrantResource_Configure_nilProviderData(t *testing.T) {
	r := &grantResource{}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestGrantResource_Configure_wrongType(t *testing.T) {
	r := &grantResource{}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestDefaultPrivilegesResource_Configure_nilProviderData(t *testing.T) {
	r := &defaultPrivilegesResource{}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestDefaultPrivilegesResource_Configure_wrongType(t *testing.T) {
	r := &defaultPrivilegesResource{}
	resp := &resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}
