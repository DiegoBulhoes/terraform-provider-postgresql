// Tests for postgresql_schemas data source.
package datasource_test

import (
	"fmt"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresqlSchemasDataSource_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_schemas" "test" {
  include_system_schemas = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.postgresql_schemas.test", "schemas.#"),
				),
			},
		},
	})
}

func TestAccPostgresqlSchemasDataSource_withPattern(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_schemas" "test" {
  like_pattern           = "pub%"
  include_system_schemas = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.postgresql_schemas.test", "schemas.#"),
				),
			},
		},
	})
}

func TestAccPostgresqlSchemasDataSource_notLikePattern(t *testing.T) {
	rSchema := "acctest_schemas_ds_nlp"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_schema" "test" {
  name = %q
}

data "postgresql_schemas" "test" {
  not_like_pattern       = "pub%%"
  include_system_schemas = false
  depends_on             = [postgresql_schema.test]
}
`, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.postgresql_schemas.test", "schemas.#"),
				),
			},
		},
	})
}

func TestAccPostgresqlSchemasDataSource_combinedFilters(t *testing.T) {
	rSchema1 := "acctest_schemas_cf_app"
	rSchema2 := "acctest_schemas_cf_api"
	rSchema3 := "acctest_schemas_cf_other"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_schema" "app" {
  name = %q
}

resource "postgresql_schema" "api" {
  name = %q
}

resource "postgresql_schema" "other" {
  name = %q
}

data "postgresql_schemas" "test" {
  like_pattern           = "acctest_schemas_cf_%%"
  not_like_pattern        = "%%other"
  include_system_schemas = false
  depends_on = [postgresql_schema.app, postgresql_schema.api, postgresql_schema.other]
}
`, rSchema1, rSchema2, rSchema3),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_schemas.test", "schemas.#", "2"),
				),
			},
		},
	})
}

func TestAccPostgresqlSchemasDataSource_verifyAttributes(t *testing.T) {
	rSchema := "acctest_schemas_va"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_schema" "test" {
  name = %q
}

data "postgresql_schemas" "test" {
  like_pattern           = "acctest_schemas_va"
  include_system_schemas = false
  depends_on = [postgresql_schema.test]
}
`, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_schemas.test", "schemas.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_schemas.test", "schemas.0.name", rSchema),
					resource.TestCheckResourceAttrSet("data.postgresql_schemas.test", "schemas.0.owner"),
				),
			},
		},
	})
}

func TestAccPostgresqlSchemasDataSource_includeSystem(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_schemas" "test" {
  include_system_schemas = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// With system schemas, we should have pg_catalog, information_schema, etc.
					resource.TestCheckResourceAttrSet("data.postgresql_schemas.test", "schemas.#"),
				),
			},
		},
	})
}

func TestAccPostgresqlSchemasDataSource_emptyResult(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_schemas" "test" {
					like_pattern           = "zzz_nonexistent_%"
					include_system_schemas = false
				}`,
				Check: resource.TestCheckResourceAttr("data.postgresql_schemas.test", "schemas.#", "0"),
			},
		},
	})
}
