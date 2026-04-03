package resource_test

import (
	"os"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"postgresql": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func TestMain(m *testing.M) {
	if os.Getenv("PGHOST") != "" {
		os.Exit(m.Run())
	}
	acctest.SetupTestContainer(m)
}
