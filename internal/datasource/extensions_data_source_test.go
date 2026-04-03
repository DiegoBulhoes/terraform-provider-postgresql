//go:build integration

// Tests for postgresql_extensions data source.
package datasource_test

import (
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresqlExtensionsDataSource_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "postgresql_extensions" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// plpgsql is always installed
					resource.TestCheckResourceAttrSet("data.postgresql_extensions.test", "extensions.#"),
				),
			},
		},
	})
}

func TestAccPostgresqlExtensionsDataSource_withDatabase(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "postgresql_extensions" "test" {
					database = "postgres"
				}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.postgresql_extensions.test", "extensions.#"),
					resource.TestCheckResourceAttr("data.postgresql_extensions.test", "database", "postgres"),
					// plpgsql is always present; verify first extension has expected attributes
					resource.TestCheckResourceAttrSet("data.postgresql_extensions.test", "extensions.0.name"),
					resource.TestCheckResourceAttrSet("data.postgresql_extensions.test", "extensions.0.version"),
					resource.TestCheckResourceAttrSet("data.postgresql_extensions.test", "extensions.0.schema"),
				),
			},
		},
	})
}

// Example-based test: validate documentation example from examples/data-sources/postgresql_extensions/data-source.tf

func TestAccPostgresqlExtensionsDataSource_exampleAll(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_extensions" "all" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// plpgsql is always installed
					resource.TestCheckResourceAttrSet("data.postgresql_extensions.all", "extensions.#"),
					resource.TestCheckResourceAttrSet("data.postgresql_extensions.all", "extensions.0.name"),
					resource.TestCheckResourceAttrSet("data.postgresql_extensions.all", "extensions.0.version"),
				),
			},
		},
	})
}
