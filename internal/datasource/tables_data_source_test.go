//go:build integration

// Tests for postgresql_tables data source.
package datasource_test

import (
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresqlTablesDataSource_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "postgresql_tables" "test" {
					schema = "information_schema"
				}`,
				Check: resource.TestCheckResourceAttrSet("data.postgresql_tables.test", "tables.#"),
			},
		},
	})
}

func TestAccPostgresqlTablesDataSource_withPattern(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(`CREATE TABLE IF NOT EXISTS acctest_tables_ds_t1 (id int)`)
					db.Exec(`CREATE TABLE IF NOT EXISTS acctest_tables_ds_t2 (id int)`)
				},
				Config: `data "postgresql_tables" "test" {
					schema       = "public"
					like_pattern = "acctest_tables_ds_%"
				}`,
				Check: resource.TestCheckResourceAttrSet("data.postgresql_tables.test", "tables.#"),
			},
		},
	})
}

func TestAccPostgresqlTablesDataSource_emptyResult(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "postgresql_tables" "test" {
					schema       = "public"
					like_pattern = "zzz_nonexistent_%"
				}`,
				Check: resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.#", "0"),
			},
		},
	})
}

func TestAccPostgresqlTablesDataSource_notLikePattern(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(`CREATE TABLE IF NOT EXISTS acctest_nlp_keep (id int)`)
					db.Exec(`CREATE TABLE IF NOT EXISTS acctest_nlp_exclude_me (id int)`)
				},
				Config: `data "postgresql_tables" "test" {
					schema           = "public"
					like_pattern     = "acctest_nlp_%"
					not_like_pattern = "%_exclude_%"
				}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.0.name", "acctest_nlp_keep"),
					resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.0.schema", "public"),
					resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.0.type", "BASE TABLE"),
				),
			},
		},
	})
}

func TestAccPostgresqlTablesDataSource_tableType(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(`CREATE TABLE IF NOT EXISTS acctest_tt_base (id int)`)
					db.Exec(`CREATE OR REPLACE VIEW acctest_tt_view AS SELECT id FROM acctest_tt_base`)
				},
				Config: `data "postgresql_tables" "base_only" {
					schema       = "public"
					like_pattern = "acctest_tt_%"
					table_type   = "BASE TABLE"
				}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_tables.base_only", "tables.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_tables.base_only", "tables.0.name", "acctest_tt_base"),
					resource.TestCheckResourceAttr("data.postgresql_tables.base_only", "tables.0.type", "BASE TABLE"),
				),
			},
			{
				Config: `data "postgresql_tables" "views_only" {
					schema       = "public"
					like_pattern = "acctest_tt_%"
					table_type   = "VIEW"
				}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_tables.views_only", "tables.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_tables.views_only", "tables.0.name", "acctest_tt_view"),
					resource.TestCheckResourceAttr("data.postgresql_tables.views_only", "tables.0.type", "VIEW"),
				),
			},
		},
	})
}

func TestAccPostgresqlTablesDataSource_combinedFilters(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(`CREATE SCHEMA IF NOT EXISTS acctest_cf_schema`)
					db.Exec(`CREATE TABLE IF NOT EXISTS acctest_cf_schema.alpha_one (id int)`)
					db.Exec(`CREATE TABLE IF NOT EXISTS acctest_cf_schema.alpha_two (id int)`)
					db.Exec(`CREATE TABLE IF NOT EXISTS acctest_cf_schema.alpha_two_skip (id int)`)
					db.Exec(`CREATE TABLE IF NOT EXISTS acctest_cf_schema.beta_one (id int)`)
				},
				Config: `data "postgresql_tables" "test" {
					schema           = "acctest_cf_schema"
					like_pattern     = "alpha_%"
					not_like_pattern = "%_skip"
				}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.#", "2"),
					resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.0.name", "alpha_one"),
					resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.0.schema", "acctest_cf_schema"),
					resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.1.name", "alpha_two"),
				),
			},
		},
	})
}

// Example-based test: validate documentation example from examples/data-sources/postgresql_tables/data-source.tf

func TestAccPostgresqlTablesDataSource_examplePublicSchema(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_tables" "public" {
  schema = "information_schema"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// information_schema always contains system tables
					resource.TestCheckResourceAttrSet("data.postgresql_tables.public", "tables.#"),
					resource.TestCheckResourceAttrSet("data.postgresql_tables.public", "tables.0.name"),
					resource.TestCheckResourceAttr("data.postgresql_tables.public", "tables.0.schema", "information_schema"),
				),
			},
		},
	})
}
