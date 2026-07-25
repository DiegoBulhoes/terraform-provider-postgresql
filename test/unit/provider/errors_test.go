package provider_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/provider"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// ---------------------------------------------------------------------------
// Sad paths for Configure: each one must return a diagnostic, not a broken
// client.
// ---------------------------------------------------------------------------

// Same as buildProviderConfig, but connect_timeout is a parameter.
func buildProviderConfigTimeout(host string, port int, connectTimeout int) tftypes.Value {
	return tftypes.NewValue(tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"host":                 tftypes.String,
			"port":                 tftypes.Number,
			"username":             tftypes.String,
			"password":             tftypes.String,
			"database":             tftypes.String,
			"sslmode":              tftypes.String,
			"sslcert":              tftypes.String,
			"sslkey":               tftypes.String,
			"sslrootcert":          tftypes.String,
			"connect_timeout":      tftypes.Number,
			"max_connections":      tftypes.Number,
			"max_idle_connections": tftypes.Number,
			"conn_max_lifetime":    tftypes.Number,
			"conn_max_idle_time":   tftypes.Number,
			"superuser":            tftypes.Bool,
			"expected_version":     tftypes.String,
		},
	}, map[string]tftypes.Value{
		"host":                 tftypes.NewValue(tftypes.String, host),
		"port":                 tftypes.NewValue(tftypes.Number, port),
		"username":             tftypes.NewValue(tftypes.String, "user"),
		"password":             tftypes.NewValue(tftypes.String, "pass"),
		"database":             tftypes.NewValue(tftypes.String, "db"),
		"sslmode":              tftypes.NewValue(tftypes.String, "disable"),
		"sslcert":              tftypes.NewValue(tftypes.String, nil),
		"sslkey":               tftypes.NewValue(tftypes.String, nil),
		"sslrootcert":          tftypes.NewValue(tftypes.String, nil),
		"connect_timeout":      tftypes.NewValue(tftypes.Number, connectTimeout),
		"max_connections":      tftypes.NewValue(tftypes.Number, nil),
		"max_idle_connections": tftypes.NewValue(tftypes.Number, nil),
		"conn_max_lifetime":    tftypes.NewValue(tftypes.Number, nil),
		"conn_max_idle_time":   tftypes.NewValue(tftypes.Number, nil),
		"superuser":            tftypes.NewValue(tftypes.Bool, nil),
		"expected_version":     tftypes.NewValue(tftypes.String, nil),
	})
}

// Runs Configure with the given ctx and config.
func configure(ctx context.Context, cfg tftypes.Value) *fwprovider.ConfigureResponse {
	p := &provider.PostgreSQLProvider{}
	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(ctx, fwprovider.SchemaRequest{}, schemaResp)

	resp := &fwprovider.ConfigureResponse{}
	p.Configure(ctx, fwprovider.ConfigureRequest{
		Config: tfsdk.Config{Raw: cfg, Schema: schemaResp.Schema},
	}, resp)
	return resp
}

// connect_timeout = 0 means "no limit" to libpq, but WithTimeout(ctx, 0) is
// already expired, so the ping always fails.
func TestProvider_Configure_connectTimeoutZeroAlwaysFails(t *testing.T) {
	resp := configure(context.Background(), buildProviderConfigTimeout("127.0.0.1", 5432, 0))

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error when connect_timeout is 0")
	}
	var detail string
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Unable to connect to PostgreSQL" {
			detail = d.Detail()
		}
	}
	if !strings.Contains(detail, "context deadline exceeded") {
		t.Errorf("expected an expired-context failure, got %q", detail)
	}
	if resp.ResourceData != nil || resp.DataSourceData != nil {
		t.Error("provider data must stay nil when the ping fails")
	}
}

// A closed port must be reported. sql.Open alone would not catch it.
func TestProvider_Configure_refusedConnection(t *testing.T) {
	resp := configure(context.Background(), buildProviderConfigTimeout("127.0.0.1", 1, 5))

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for a closed port")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Unable to connect to PostgreSQL" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a connection diagnostic, got %v", resp.Diagnostics.Errors())
	}
	if resp.ResourceData != nil {
		t.Error("provider data must stay nil when the ping fails")
	}
}

// A canceled context must stop Configure instead of blocking on the ping.
func TestProvider_Configure_canceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	resp := configure(ctx, buildProviderConfigTimeout("127.0.0.1", 5432, 30))

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for a canceled context")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Configure ignored cancellation for %s", elapsed)
	}
	if resp.ResourceData != nil {
		t.Error("provider data must stay nil when the ping fails")
	}
}
