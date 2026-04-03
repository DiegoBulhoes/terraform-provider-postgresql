package datasource

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Unit tests for Configure error paths on all data sources.

func TestRoleDataSource_Configure_nilProviderData(t *testing.T) {
	d := &roleDataSource{}
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestRoleDataSource_Configure_wrongType(t *testing.T) {
	d := &roleDataSource{}
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestDatabaseDataSource_Configure_nilProviderData(t *testing.T) {
	d := &databaseDataSource{}
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: nil,
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Error("expected no error for nil provider data")
	}
}

func TestDatabaseDataSource_Configure_wrongType(t *testing.T) {
	d := &databaseDataSource{}
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: "not a db",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestSchemasDataSource_Configure_wrongType(t *testing.T) {
	d := &schemasDataSource{}
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: 42,
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestQueryDataSource_Configure_wrongType(t *testing.T) {
	d := &queryDataSource{}
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: true,
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestVersionDataSource_Configure_wrongType(t *testing.T) {
	d := &versionDataSource{}
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: []string{},
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestExtensionsDataSource_Configure_wrongType(t *testing.T) {
	d := &extensionsDataSource{}
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: "wrong",
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestRolesDataSource_Configure_wrongType(t *testing.T) {
	d := &rolesDataSource{}
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: 3.14,
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}

func TestTablesDataSource_Configure_wrongType(t *testing.T) {
	d := &tablesDataSource{}
	resp := &datasource.ConfigureResponse{}
	d.Configure(context.Background(), datasource.ConfigureRequest{
		ProviderData: map[string]string{},
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong type")
	}
}
