package provider

import (
	"os"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"postgresql": providerserver.NewProtocol6WithError(New("test")()),
}

func TestMain(m *testing.M) {
	if os.Getenv("PGHOST") != "" {
		os.Exit(m.Run())
	}
	acctest.SetupTestContainer(m)
}

func TestEnvOrDefault_SetValue(t *testing.T) {
	got := envOrDefault(types.StringValue("explicit"), "TEST_PROVIDER_ENV_STR_SET", "fallback")
	if got != "explicit" {
		t.Errorf("expected %q, got %q", "explicit", got)
	}
}

func TestEnvOrDefault_NullWithEnvVar(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_STR_NULL", "from_env")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_STR_NULL") })

	got := envOrDefault(types.StringNull(), "TEST_PROVIDER_ENV_STR_NULL", "fallback")
	if got != "from_env" {
		t.Errorf("expected %q, got %q", "from_env", got)
	}
}

func TestEnvOrDefault_NullWithoutEnvVar(t *testing.T) {
	os.Unsetenv("TEST_PROVIDER_ENV_STR_UNSET")

	got := envOrDefault(types.StringNull(), "TEST_PROVIDER_ENV_STR_UNSET", "fallback")
	if got != "fallback" {
		t.Errorf("expected %q, got %q", "fallback", got)
	}
}

func TestEnvOrDefault_NullWithEmptyEnvVarName(t *testing.T) {
	got := envOrDefault(types.StringNull(), "", "fallback")
	if got != "fallback" {
		t.Errorf("expected %q, got %q", "fallback", got)
	}
}

func TestEnvOrDefaultInt_SetValue(t *testing.T) {
	got := envOrDefaultInt(types.Int64Value(42), "TEST_PROVIDER_ENV_INT_SET", 0)
	if got != 42 {
		t.Errorf("expected %d, got %d", 42, got)
	}
}

func TestEnvOrDefaultInt_NullWithValidEnvVar(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_INT_VALID", "99")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_INT_VALID") })

	got := envOrDefaultInt(types.Int64Null(), "TEST_PROVIDER_ENV_INT_VALID", 0)
	if got != 99 {
		t.Errorf("expected %d, got %d", 99, got)
	}
}

func TestEnvOrDefaultInt_NullWithInvalidEnvVar(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_INT_INVALID", "not_a_number")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_INT_INVALID") })

	got := envOrDefaultInt(types.Int64Null(), "TEST_PROVIDER_ENV_INT_INVALID", 7)
	if got != 7 {
		t.Errorf("expected %d, got %d", 7, got)
	}
}

func TestEnvOrDefaultInt_NullWithoutEnvVar(t *testing.T) {
	os.Unsetenv("TEST_PROVIDER_ENV_INT_UNSET")

	got := envOrDefaultInt(types.Int64Null(), "TEST_PROVIDER_ENV_INT_UNSET", 7)
	if got != 7 {
		t.Errorf("expected %d, got %d", 7, got)
	}
}

func TestEnvOrDefaultBool_SetValue(t *testing.T) {
	got := envOrDefaultBool(types.BoolValue(true), "TEST_PROVIDER_ENV_BOOL_SET", false)
	if got != true {
		t.Errorf("expected %v, got %v", true, got)
	}
}

func TestEnvOrDefaultBool_NullWithEnvTrue(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_BOOL_TRUE", "true")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_BOOL_TRUE") })

	got := envOrDefaultBool(types.BoolNull(), "TEST_PROVIDER_ENV_BOOL_TRUE", false)
	if got != true {
		t.Errorf("expected %v, got %v", true, got)
	}
}

func TestEnvOrDefaultBool_NullWithEnvFalse(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_BOOL_FALSE", "false")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_BOOL_FALSE") })

	got := envOrDefaultBool(types.BoolNull(), "TEST_PROVIDER_ENV_BOOL_FALSE", true)
	if got != false {
		t.Errorf("expected %v, got %v", false, got)
	}
}

func TestEnvOrDefaultBool_NullWithInvalidEnvVar(t *testing.T) {
	os.Setenv("TEST_PROVIDER_ENV_BOOL_INVALID", "not_a_bool")
	t.Cleanup(func() { os.Unsetenv("TEST_PROVIDER_ENV_BOOL_INVALID") })

	got := envOrDefaultBool(types.BoolNull(), "TEST_PROVIDER_ENV_BOOL_INVALID", true)
	if got != true {
		t.Errorf("expected %v, got %v", true, got)
	}
}

func TestEnvOrDefaultBool_NullWithoutEnvVar(t *testing.T) {
	os.Unsetenv("TEST_PROVIDER_ENV_BOOL_UNSET")

	got := envOrDefaultBool(types.BoolNull(), "TEST_PROVIDER_ENV_BOOL_UNSET", true)
	if got != true {
		t.Errorf("expected %v, got %v", true, got)
	}
}
