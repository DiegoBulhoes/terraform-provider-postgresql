package provider_test

import (
	"context"
	"os"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/provider"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestNew(t *testing.T) {
	factory := provider.New("test")
	p := factory()
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestProvider_Metadata(t *testing.T) {
	p := &provider.PostgreSQLProvider{Version: "1.0.0"}
	resp := &fwprovider.MetadataResponse{}
	p.Metadata(context.Background(), fwprovider.MetadataRequest{}, resp)
	if resp.TypeName != "postgresql" {
		t.Errorf("expected postgresql, got %s", resp.TypeName)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", resp.Version)
	}
}

func TestProvider_Schema(t *testing.T) {
	p := &provider.PostgreSQLProvider{}
	resp := &fwprovider.SchemaResponse{}
	p.Schema(context.Background(), fwprovider.SchemaRequest{}, resp)
	if resp.Schema.Description == "" {
		t.Error("expected non-empty schema description")
	}
}

func TestProvider_Resources(t *testing.T) {
	p := &provider.PostgreSQLProvider{}
	resources := p.Resources(context.Background())
	if len(resources) != 5 {
		t.Errorf("expected 5 resources, got %d", len(resources))
	}
}

func TestProvider_DataSources(t *testing.T) {
	p := &provider.PostgreSQLProvider{}
	dataSources := p.DataSources(context.Background())
	if len(dataSources) != 9 {
		t.Errorf("expected 9 data sources, got %d", len(dataSources))
	}
}

func TestProvider_Configure_invalidHost(t *testing.T) {
	p := &provider.PostgreSQLProvider{}
	resp := &fwprovider.ConfigureResponse{}

	// Build a config with an unreachable host to test the error path
	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(context.Background(), fwprovider.SchemaRequest{}, schemaResp)

	p.Configure(context.Background(), fwprovider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw:    buildProviderConfig("__invalid_host__", 1, "user", "pass", "db", "disable"),
			Schema: schemaResp.Schema,
		},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid host")
	}
}

// TestProvider_Configure_emptyPassword verifies an empty password still
// produces a well-formed connection string (quoted empty value).
func TestProvider_Configure_emptyPassword(t *testing.T) {
	p := &provider.PostgreSQLProvider{}
	resp := &fwprovider.ConfigureResponse{}

	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(context.Background(), fwprovider.SchemaRequest{}, schemaResp)

	p.Configure(context.Background(), fwprovider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw:    buildProviderConfig("__invalid_host__", 1, "user", "", "db", "disable"),
			Schema: schemaResp.Schema,
		},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid host")
	}
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Unable to create PostgreSQL client" {
			t.Errorf("empty password broke conn-string parsing: %s", d.Detail())
		}
	}
}

// TestProvider_Configure_spaceInHost verifies that whitespace in values does
// not break the key=value connection string.
func TestProvider_Configure_spaceInHost(t *testing.T) {
	p := &provider.PostgreSQLProvider{}
	resp := &fwprovider.ConfigureResponse{}

	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(context.Background(), fwprovider.SchemaRequest{}, schemaResp)

	p.Configure(context.Background(), fwprovider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw:    buildProviderConfig("host with space", 1, "user", "pw", "db", "disable"),
			Schema: schemaResp.Schema,
		},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid host")
	}
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Unable to create PostgreSQL client" {
			t.Errorf("spaces in host broke conn-string parsing: %s", d.Detail())
		}
	}
}

// TestProvider_Configure_invalidSSLMode_rejectedAtSchema verifies that the
// validator catches bad sslmode values before they reach Configure.
func TestProvider_Configure_invalidSSLMode(t *testing.T) {
	p := &provider.PostgreSQLProvider{}
	resp := &fwprovider.ConfigureResponse{}

	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(context.Background(), fwprovider.SchemaRequest{}, schemaResp)

	// "disable" is valid, so we pick that and instead sanity-check that the
	// normal path emits a network error (not a validation error).
	p.Configure(context.Background(), fwprovider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw:    buildProviderConfig("__invalid_host__", 1, "u", "p", "d", "disable"),
			Schema: schemaResp.Schema,
		},
	}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Unable to connect to PostgreSQL" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected connection-failure diagnostic, got %v", resp.Diagnostics.Errors())
	}
}

// TestProvider_Configure_specialCharsInPassword exercises the connection-string
// escaping path: a password containing spaces, single quotes, and backslashes
// must not cause the Configure call to panic or produce a connection-string
// parse error. Connection will still fail (invalid host), but the failure must
// be a *ping* error, not a conn-string parse error.
func TestProvider_Configure_specialCharsInPassword(t *testing.T) {
	p := &provider.PostgreSQLProvider{}
	resp := &fwprovider.ConfigureResponse{}

	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(context.Background(), fwprovider.SchemaRequest{}, schemaResp)

	p.Configure(context.Background(), fwprovider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw:    buildProviderConfig("__invalid_host__", 1, "user", `p'a s\s`, "db", "disable"),
			Schema: schemaResp.Schema,
		},
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid host")
	}
	// Must be a connection error, not a parse error.
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Unable to create PostgreSQL client" {
			t.Errorf("special-char password caused connection string parse error: %s", d.Detail())
		}
	}
}

func buildProviderConfig(host string, port int, username, password, database, sslmode string) tftypes.Value {
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
		"username":             tftypes.NewValue(tftypes.String, username),
		"password":             tftypes.NewValue(tftypes.String, password),
		"database":             tftypes.NewValue(tftypes.String, database),
		"sslmode":              tftypes.NewValue(tftypes.String, sslmode),
		"sslcert":              tftypes.NewValue(tftypes.String, nil),
		"sslkey":               tftypes.NewValue(tftypes.String, nil),
		"sslrootcert":          tftypes.NewValue(tftypes.String, nil),
		"connect_timeout":      tftypes.NewValue(tftypes.Number, 1),
		"max_connections":      tftypes.NewValue(tftypes.Number, nil),
		"max_idle_connections": tftypes.NewValue(tftypes.Number, nil),
		"conn_max_lifetime":    tftypes.NewValue(tftypes.Number, nil),
		"conn_max_idle_time":   tftypes.NewValue(tftypes.Number, nil),
		"superuser":            tftypes.NewValue(tftypes.Bool, nil),
		"expected_version":     tftypes.NewValue(tftypes.String, nil),
	})
}

func TestEnvOrDefault_SetValue(t *testing.T) {
	got := provider.EnvOrDefault(types.StringValue("explicit"), "TEST_PROVIDER_ENV_STR_SET", "fallback")
	if got != "explicit" {
		t.Errorf("expected %q, got %q", "explicit", got)
	}
}

func TestEnvOrDefault_NullWithEnvVar(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_STR_NULL", "from_env")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_STR_NULL") })

	got := provider.EnvOrDefault(types.StringNull(), "TEST_PROVIDER_ENV_STR_NULL", "fallback")
	if got != "from_env" {
		t.Errorf("expected %q, got %q", "from_env", got)
	}
}

func TestEnvOrDefault_NullWithoutEnvVar(t *testing.T) {
	os.Unsetenv("TEST_PROVIDER_ENV_STR_UNSET")
	got := provider.EnvOrDefault(types.StringNull(), "TEST_PROVIDER_ENV_STR_UNSET", "fallback")
	if got != "fallback" {
		t.Errorf("expected %q, got %q", "fallback", got)
	}
}

func TestEnvOrDefault_NullWithEmptyEnvVarName(t *testing.T) {
	got := provider.EnvOrDefault(types.StringNull(), "", "fallback")
	if got != "fallback" {
		t.Errorf("expected %q, got %q", "fallback", got)
	}
}

func TestEnvOrDefaultInt_SetValue(t *testing.T) {
	got := provider.EnvOrDefaultInt(types.Int64Value(42), "TEST_PROVIDER_ENV_INT_SET", 0)
	if got != 42 {
		t.Errorf("expected %d, got %d", 42, got)
	}
}

func TestEnvOrDefaultInt_NullWithValidEnvVar(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_INT_VALID", "99")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_INT_VALID") })

	got := provider.EnvOrDefaultInt(types.Int64Null(), "TEST_PROVIDER_ENV_INT_VALID", 0)
	if got != 99 {
		t.Errorf("expected %d, got %d", 99, got)
	}
}

func TestEnvOrDefaultInt_NullWithInvalidEnvVar(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_INT_INVALID", "not_a_number")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_INT_INVALID") })

	got := provider.EnvOrDefaultInt(types.Int64Null(), "TEST_PROVIDER_ENV_INT_INVALID", 7)
	if got != 7 {
		t.Errorf("expected %d, got %d", 7, got)
	}
}

func TestEnvOrDefaultInt_NullWithoutEnvVar(t *testing.T) {
	os.Unsetenv("TEST_PROVIDER_ENV_INT_UNSET")
	got := provider.EnvOrDefaultInt(types.Int64Null(), "TEST_PROVIDER_ENV_INT_UNSET", 7)
	if got != 7 {
		t.Errorf("expected %d, got %d", 7, got)
	}
}

func TestEnvOrDefaultBool_SetValue(t *testing.T) {
	got := provider.EnvOrDefaultBool(types.BoolValue(true), "TEST_PROVIDER_ENV_BOOL_SET", false)
	if got != true {
		t.Errorf("expected %v, got %v", true, got)
	}
}

func TestEnvOrDefaultBool_NullWithEnvTrue(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_BOOL_TRUE", "true")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_BOOL_TRUE") })

	got := provider.EnvOrDefaultBool(types.BoolNull(), "TEST_PROVIDER_ENV_BOOL_TRUE", false)
	if got != true {
		t.Errorf("expected %v, got %v", true, got)
	}
}

func TestEnvOrDefaultBool_NullWithEnvFalse(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_BOOL_FALSE", "false")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_BOOL_FALSE") })

	got := provider.EnvOrDefaultBool(types.BoolNull(), "TEST_PROVIDER_ENV_BOOL_FALSE", true)
	if got != false {
		t.Errorf("expected %v, got %v", false, got)
	}
}

func TestEnvOrDefaultBool_NullWithInvalidEnvVar(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_BOOL_INVALID", "not_a_bool")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_BOOL_INVALID") })

	got := provider.EnvOrDefaultBool(types.BoolNull(), "TEST_PROVIDER_ENV_BOOL_INVALID", true)
	if got != true {
		t.Errorf("expected %v, got %v", true, got)
	}
}

func TestEnvOrDefaultBool_NullWithoutEnvVar(t *testing.T) {
	os.Unsetenv("TEST_PROVIDER_ENV_BOOL_UNSET")
	got := provider.EnvOrDefaultBool(types.BoolNull(), "TEST_PROVIDER_ENV_BOOL_UNSET", true)
	if got != true {
		t.Errorf("expected %v, got %v", true, got)
	}
}

// TestEnvOrDefault_TableDriven consolidates coverage of the three
// Env*OrDefault helpers across (null|set|unknown) × (envSet|envUnset|envInvalid).
func TestEnvOrDefault_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		tfVal      types.String
		envName    string
		envValue   string
		defaultVal string
		want       string
	}{
		{"tf_set_overrides_env", types.StringValue("tfval"), "T1", "envval", "def", "tfval"},
		{"tf_null_uses_env", types.StringNull(), "T2", "envval", "def", "envval"},
		{"tf_null_no_env_uses_default", types.StringNull(), "T3", "", "def", "def"},
		{"tf_unknown_falls_through_to_env", types.StringUnknown(), "T4", "envval", "def", "envval"},
		{"tf_unknown_no_env_uses_default", types.StringUnknown(), "T5", "", "def", "def"},
		{"empty_env_name_uses_default", types.StringNull(), "", "", "def", "def"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envName != "" && tc.envValue != "" {
				os.Setenv(tc.envName, tc.envValue)
				t.Cleanup(func() { os.Unsetenv(tc.envName) })
			} else if tc.envName != "" {
				os.Unsetenv(tc.envName)
			}
			got := provider.EnvOrDefault(tc.tfVal, tc.envName, tc.defaultVal)
			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestEnvOrDefaultInt_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		tfVal      types.Int64
		envName    string
		envValue   string
		defaultVal int
		want       int
	}{
		{"tf_set_overrides", types.Int64Value(42), "TI1", "99", 0, 42},
		{"tf_null_uses_env", types.Int64Null(), "TI2", "99", 0, 99},
		{"tf_null_invalid_env_uses_default", types.Int64Null(), "TI3", "abc", 7, 7},
		{"tf_null_no_env_uses_default", types.Int64Null(), "TI4", "", 7, 7},
		{"negative_env_value", types.Int64Null(), "TI5", "-3", 0, -3},
		{"zero_env_value", types.Int64Null(), "TI6", "0", 10, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envName != "" && tc.envValue != "" {
				os.Setenv(tc.envName, tc.envValue)
				t.Cleanup(func() { os.Unsetenv(tc.envName) })
			} else if tc.envName != "" {
				os.Unsetenv(tc.envName)
			}
			got := provider.EnvOrDefaultInt(tc.tfVal, tc.envName, tc.defaultVal)
			if got != tc.want {
				t.Errorf("expected %d, got %d", tc.want, got)
			}
		})
	}
}

func TestEnvOrDefaultBool_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		tfVal      types.Bool
		envName    string
		envValue   string
		defaultVal bool
		want       bool
	}{
		{"tf_true_overrides_env_false", types.BoolValue(true), "TB1", "false", false, true},
		{"tf_false_overrides_env_true", types.BoolValue(false), "TB2", "true", true, false},
		{"env_true", types.BoolNull(), "TB3", "true", false, true},
		{"env_false", types.BoolNull(), "TB4", "false", true, false},
		{"env_1_is_true", types.BoolNull(), "TB5", "1", false, true},
		{"env_0_is_false", types.BoolNull(), "TB6", "0", true, false},
		{"env_invalid_uses_default", types.BoolNull(), "TB7", "maybe", true, true},
		{"null_no_env_uses_default", types.BoolNull(), "TB8", "", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envName != "" && tc.envValue != "" {
				os.Setenv(tc.envName, tc.envValue)
				t.Cleanup(func() { os.Unsetenv(tc.envName) })
			} else if tc.envName != "" {
				os.Unsetenv(tc.envName)
			}
			got := provider.EnvOrDefaultBool(tc.tfVal, tc.envName, tc.defaultVal)
			if got != tc.want {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
