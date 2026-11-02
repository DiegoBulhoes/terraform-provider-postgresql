//go:build integration

package datasource_test

import (
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresqlQueryDataSource_duplicateColumnNames(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_query" "test" {
  database = "postgres"
  query    = "SELECT 1 AS id, 2 AS id"
}`,
				ExpectError: regexp.MustCompile(`(?i)duplicate column name "id"`),
			},
		},
	})
}

// A view is listed with its owner, and so are tables the provider user has no privilege on.
func TestAccPostgresqlTablesDataSource_viewOwner(t *testing.T) {
	acctest.MustExec(t, `DROP SCHEMA IF EXISTS acctest_dsfx_views CASCADE`)
	acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_dsfx_view_owner`)
	acctest.MustExec(t, `CREATE ROLE acctest_dsfx_view_owner`)
	acctest.MustExec(t, `CREATE SCHEMA acctest_dsfx_views`)
	acctest.MustExec(t, `CREATE VIEW acctest_dsfx_views.v AS SELECT 1 AS id`)
	acctest.MustExec(t, `ALTER VIEW acctest_dsfx_views.v OWNER TO acctest_dsfx_view_owner`)
	t.Cleanup(func() {
		acctest.MustExec(t, `DROP SCHEMA IF EXISTS acctest_dsfx_views CASCADE`)
		acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_dsfx_view_owner`)
	})
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_tables" "test" {
  schema = "acctest_dsfx_views"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.0.name", "v"),
					resource.TestCheckResourceAttr("data.postgresql_tables.test", "tables.0.owner", "acctest_dsfx_view_owner"),
				),
			},
		},
	})
}

// Since PostgreSQL 16 a membership granted by two grantors has two rows; the role is listed once.
func TestAccPostgresqlRoleDataSource_membershipFromTwoGrantors(t *testing.T) {
	for _, s := range []string{
		`DROP ROLE IF EXISTS acctest_dsfx_member`, `DROP ROLE IF EXISTS acctest_dsfx_granter`, `DROP ROLE IF EXISTS acctest_dsfx_group`,
		`CREATE ROLE acctest_dsfx_group`, `CREATE ROLE acctest_dsfx_granter`, `CREATE ROLE acctest_dsfx_member LOGIN`,
		`GRANT acctest_dsfx_group TO acctest_dsfx_granter WITH ADMIN OPTION`,
		`GRANT acctest_dsfx_group TO acctest_dsfx_member`,
		`GRANT acctest_dsfx_group TO acctest_dsfx_member GRANTED BY acctest_dsfx_granter`,
	} {
		acctest.MustExec(t, s)
	}
	t.Cleanup(func() {
		acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_dsfx_member`)
		acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_dsfx_granter`)
		acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_dsfx_group`)
	})
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_role" "test" {
  name = "acctest_dsfx_member"
}

data "postgresql_user" "test" {
  name = "acctest_dsfx_member"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_role.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "roles.#", "1"),
				),
			},
		},
	})
}
