package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresqlQueryDataSource_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_query" "test" {
  database = "postgres"
  query    = "SELECT 1::text AS num, 'hello' AS greeting"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.0.num", "1"),
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.0.greeting", "hello"),
				),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_multipleRows(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_query" "test" {
  database = "postgres"
  query    = "SELECT n::text AS id, ('item_' || n::text) AS name FROM generate_series(1,3) AS n"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.#", "3"),
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.0.id", "1"),
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.0.name", "item_1"),
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.1.id", "2"),
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.2.id", "3"),
				),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_nullValues(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_query" "test" {
  database = "postgres"
  query    = "SELECT 'value'::text AS col_a, NULL::text AS col_b"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.0.col_a", "value"),
					resource.TestCheckNoResourceAttr("data.postgresql_query.test", "rows.0.col_b"),
				),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_dataTypes(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_query" "test" {
  database = "postgres"
  query    = "SELECT 42::text AS int_val, 3.14::text AS float_val, true::text AS bool_val, '2024-01-01'::date::text AS date_val"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.0.int_val", "42"),
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.0.float_val", "3.14"),
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.0.bool_val", "true"),
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.0.date_val", "2024-01-01"),
				),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_emptyResult(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_query" "test" {
  database = "postgres"
  query    = "SELECT 1::text AS id WHERE false"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.#", "0"),
				),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_nonSelectError(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_query" "test" {
  database = "postgres"
  query    = "INSERT INTO nonexistent VALUES (1)"
}
`,
				ExpectError: regexp.MustCompile("Only SELECT queries are allowed"),
			},
		},
	})
}
