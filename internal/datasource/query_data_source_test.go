//go:build integration

// Tests for postgresql_query data source.
package datasource_test

import (
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresqlQueryDataSource_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
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
		ProtoV6ProviderFactories: testProviderFactories,
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
		ProtoV6ProviderFactories: testProviderFactories,
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
		ProtoV6ProviderFactories: testProviderFactories,
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
		ProtoV6ProviderFactories: testProviderFactories,
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

func TestAccPostgresqlQueryDataSource_cte(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_query" "test" {
					database = "postgres"
					query    = "WITH cte AS (SELECT 1 AS num) SELECT num FROM cte"
				}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.#", "1"),
				),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_multipleColumnsAndTypes(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_query" "test" {
					database = "postgres"
					query    = "SELECT 42 AS int_val, 3.14 AS float_val, true AS bool_val, 'hello' AS text_val, ARRAY[1,2,3]::text AS array_val"
				}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.#", "1"),
				),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_jsonData(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_query" "test" {
					database = "postgres"
					query    = "SELECT '{\"key\": \"value\"}'::json::text AS json_data"
				}`,
				Check: resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.#", "1"),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_manyRows(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_query" "test" {
					database = "postgres"
					query    = "SELECT generate_series AS num FROM generate_series(1, 100)"
				}`,
				Check: resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.#", "100"),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_nonSelectError(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
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

func TestAccPostgresqlQueryDataSource_syntaxError(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_query" "test" {
					database = "postgres"
					query    = "SELEC invalid syntax"
				}`,
				ExpectError: regexp.MustCompile(`Only SELECT queries are allowed|Invalid Query`),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_nonExistentTable(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_query" "test" {
					database = "postgres"
					query    = "SELECT * FROM nonexistent_table_12345"
				}`,
				ExpectError: regexp.MustCompile(`does not exist|Error executing query`),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_readOnlyProtection(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_query" "test" {
					database = "postgres"
					query    = "SELECT 1; CREATE TABLE test_injection(id int)"
				}`,
				ExpectError: regexp.MustCompile(`(?s)cannot execute .* in a read-only`),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_allowDestructive(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				resource "postgresql_role" "test" { name = "acctest_query_destr" }

				data "postgresql_query" "test" {
					database          = "postgres"
					query             = "DELETE FROM pg_catalog.pg_description WHERE objoid = 0 AND classoid = 0 AND objsubid = -99999 RETURNING objoid"
					allow_destructive = true
				}`,
				Check: resource.TestCheckResourceAttr("data.postgresql_query.test", "rows.#", "0"),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_allowDestructiveBlocked(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_query" "test" {
					database = "postgres"
					query    = "DELETE FROM pg_description WHERE objoid = 0"
				}`,
				ExpectError: regexp.MustCompile(`Only SELECT queries are allowed`),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_allowDestructiveFalse(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_query" "test" {
					database          = "postgres"
					query             = "DELETE FROM pg_description WHERE objoid = 0"
					allow_destructive = false
				}`,
				ExpectError: regexp.MustCompile(`Only SELECT queries are allowed`),
			},
		},
	})
}

// Example-based tests: validate documentation examples from examples/data-sources/postgresql_query/data-source.tf

func TestAccPostgresqlQueryDataSource_exampleVersion(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_query" "version" {
  database = "postgres"
  query    = "SELECT version() AS pg_version"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_query.version", "rows.#", "1"),
					resource.TestMatchResourceAttr(
						"data.postgresql_query.version", "rows.0.pg_version",
						regexp.MustCompile(`PostgreSQL`),
					),
				),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_exampleConnections(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_query" "connections" {
  database = "postgres"
  query    = "SELECT datname, count(*)::text AS count FROM pg_stat_activity WHERE datname IS NOT NULL GROUP BY datname ORDER BY count DESC"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.postgresql_query.connections", "rows.#"),
					resource.TestCheckResourceAttrSet("data.postgresql_query.connections", "rows.0.count"),
				),
			},
		},
	})
}

func TestAccPostgresqlQueryDataSource_exampleExtensions(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_query" "extensions" {
  database = "postgres"
  query    = "SELECT extname, extversion FROM pg_extension ORDER BY extname"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.postgresql_query.extensions", "rows.#"),
					// plpgsql is always installed, so at least one row
					resource.TestCheckResourceAttrSet("data.postgresql_query.extensions", "rows.0.extname"),
					resource.TestCheckResourceAttrSet("data.postgresql_query.extensions", "rows.0.extversion"),
				),
			},
		},
	})
}
