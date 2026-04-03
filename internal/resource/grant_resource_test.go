//go:build integration

// Tests for postgresql_grant resource.
package resource_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccPostgresqlGrant_database(t *testing.T) {
	rRole := "acctest_grant_db_role"
	rDB := "acctest_grant_db"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlGrantConfig_database(rRole, rDB),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "role", rRole),
					resource.TestCheckResourceAttr("postgresql_grant.test", "object_type", "database"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "database", rDB),
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "with_grant_option", "false"),
					resource.TestCheckResourceAttrSet("postgresql_grant.test", "id"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_schema(t *testing.T) {
	rRole := "acctest_grant_sch_role"
	rSchema := "acctest_grant_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlGrantConfig_schema(rRole, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "role", rRole),
					resource.TestCheckResourceAttr("postgresql_grant.test", "object_type", "schema"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "schema", rSchema),
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "with_grant_option", "false"),
					resource.TestCheckResourceAttrSet("postgresql_grant.test", "id"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_table(t *testing.T) {
	rRole := "acctest_grant_tbl_role"
	rSchema := "acctest_grant_tbl_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlGrantConfig_table(rRole, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "role", rRole),
					resource.TestCheckResourceAttr("postgresql_grant.test", "object_type", "table"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "schema", rSchema),
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
					resource.TestCheckResourceAttrSet("postgresql_grant.test", "id"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_withGrantOption(t *testing.T) {
	rRole := "acctest_grant_go_role"
	rDB := "acctest_grant_go_db"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlGrantConfig_withGrantOption(rRole, rDB),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "role", rRole),
					resource.TestCheckResourceAttr("postgresql_grant.test", "object_type", "database"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "database", rDB),
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "with_grant_option", "true"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_update(t *testing.T) {
	rRole := "acctest_grant_upd_role"
	rDB := "acctest_grant_upd_db"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlGrantConfig_database(rRole, rDB),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "with_grant_option", "false"),
				),
			},
			{
				Config: testAccPostgresqlGrantConfig_databaseUpdated(rRole, rDB),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "1"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "with_grant_option", "true"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_databaseTemporary(t *testing.T) {
	rRole := "acctest_grant_tmp_role"
	rDB := "acctest_grant_tmp_db"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_database" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role        = postgresql_role.test.name
  object_type = "database"
  database    = postgresql_database.test.name
  privileges  = ["CONNECT", "TEMPORARY"]
}
`, rRole, rDB),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_schemaWithGrantOption(t *testing.T) {
	rRole := "acctest_grant_sgo_role"
	rSchema := "acctest_grant_sgo_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role              = postgresql_role.test.name
  object_type       = "schema"
  schema            = postgresql_schema.test.name
  privileges        = ["USAGE", "CREATE"]
  with_grant_option = true
}
`, rRole, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "with_grant_option", "true"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_sequence(t *testing.T) {
	rRole := "acctest_grant_seq_role"
	rSchema := "acctest_grant_seq_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlGrantConfig_sequence(rRole, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "role", rRole),
					resource.TestCheckResourceAttr("postgresql_grant.test", "object_type", "sequence"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "schema", rSchema),
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_function(t *testing.T) {
	rRole := "acctest_grant_fn_role"
	rSchema := "acctest_grant_fn_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlGrantConfig_function(rRole, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "role", rRole),
					resource.TestCheckResourceAttr("postgresql_grant.test", "object_type", "function"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "schema", rSchema),
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "1"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_specificTable(t *testing.T) {
	rRole := "acctest_grant_st_role"
	rSchema := "acctest_grant_st_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, rSchema))
					db.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS "%s".test_tbl (id serial PRIMARY KEY)`, rSchema))
				},
				Config: testAccPostgresqlGrantConfig_specificObject(rRole, rSchema, "table", "test_tbl", `["SELECT", "INSERT"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "object_type", "table"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "objects.#", "1"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
				),
			},
		},
	})
	// Cleanup PreConfig objects
	db, _ := acctest.GetDB()
	db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s".test_tbl`, rSchema))
	db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s"`, rSchema))
}

func TestAccPostgresqlGrant_specificSequence(t *testing.T) {
	rRole := "acctest_grant_ss_role"
	rSchema := "acctest_grant_ss_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, rSchema))
					db.Exec(fmt.Sprintf(`CREATE SEQUENCE IF NOT EXISTS "%s".test_seq`, rSchema))
				},
				Config: testAccPostgresqlGrantConfig_specificObject(rRole, rSchema, "sequence", "test_seq", `["USAGE", "SELECT"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "object_type", "sequence"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "objects.#", "1"),
				),
			},
		},
	})
	db, _ := acctest.GetDB()
	db.Exec(fmt.Sprintf(`DROP SEQUENCE IF EXISTS "%s".test_seq`, rSchema))
	db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s"`, rSchema))
}

func TestAccPostgresqlGrant_specificFunction(t *testing.T) {
	rRole := "acctest_grant_sf_role"
	rSchema := "acctest_grant_sf_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, rSchema))
					db.Exec(fmt.Sprintf(`CREATE OR REPLACE FUNCTION "%s".test_func() RETURNS void AS $$ BEGIN END; $$ LANGUAGE plpgsql`, rSchema))
				},
				Config: testAccPostgresqlGrantConfig_specificObject(rRole, rSchema, "function", "test_func", `["EXECUTE"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "object_type", "function"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "objects.#", "1"),
				),
			},
		},
	})
	db, _ := acctest.GetDB()
	db.Exec(fmt.Sprintf(`DROP FUNCTION IF EXISTS "%s".test_func()`, rSchema))
	db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s"`, rSchema))
}

func TestAccPostgresqlGrant_schemaUpdate(t *testing.T) {
	rRole := "acctest_grant_su_role"
	rSchema := "acctest_grant_su_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlGrantConfig_schema(rRole, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
				),
			},
			{
				Config: testAccPostgresqlGrantConfig_schemaUsageOnly(rRole, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "1"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_invalidObjectType(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				resource "postgresql_role" "test" { name = "acctest_grant_inv_ot_role" }
				resource "postgresql_grant" "test" {
					role        = postgresql_role.test.name
					object_type = "invalid"
					privileges  = ["SELECT"]
				}`,
				ExpectError: regexp.MustCompile(`must be one of`),
			},
		},
	})
}

func TestAccPostgresqlGrant_emptyPrivileges(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				resource "postgresql_role" "test" { name = "acctest_grant_empty_priv_role" }
				resource "postgresql_grant" "test" {
					role        = postgresql_role.test.name
					object_type = "database"
					database    = "postgres"
					privileges  = []
				}`,
				ExpectError: regexp.MustCompile(`must contain at least 1`),
			},
		},
	})
}

func TestAccPostgresqlGrant_import(t *testing.T) {
	rRole := "acctest_grant_import_role"
	rDB := "acctest_grant_import_db"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlGrantConfig_database(rRole, rDB),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "role", rRole),
					resource.TestCheckResourceAttr("postgresql_grant.test", "object_type", "database"),
				),
			},
			{
				ResourceName:      "postgresql_grant.test",
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s/database/%s/", rRole, rDB),
				ImportStateVerify: false, // privileges are read back from DB, may differ in ordering
			},
		},
	})
}

func TestAccPostgresqlGrant_schemaAndDatabaseCombined(t *testing.T) {
	rRole := "acctest_grant_combined_role"
	rDB := "acctest_grant_combined_db"
	rSchema := "acctest_grant_combined_sch"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "test" {
					name = %q
				}

				resource "postgresql_database" "test" {
					name = %q
				}

				resource "postgresql_schema" "test" {
					name = %q
				}

				resource "postgresql_grant" "db" {
					role        = postgresql_role.test.name
					object_type = "database"
					database    = postgresql_database.test.name
					privileges  = ["CONNECT"]
				}

				resource "postgresql_grant" "schema" {
					role        = postgresql_role.test.name
					object_type = "schema"
					schema      = postgresql_schema.test.name
					privileges  = ["USAGE"]
				}

				resource "postgresql_grant" "tables" {
					role        = postgresql_role.test.name
					object_type = "table"
					schema      = postgresql_schema.test.name
					privileges  = ["SELECT"]
				}
				`, rRole, rDB, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.db", "object_type", "database"),
					resource.TestCheckResourceAttr("postgresql_grant.schema", "object_type", "schema"),
					resource.TestCheckResourceAttr("postgresql_grant.tables", "object_type", "table"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_revokeAndRegrant(t *testing.T) {
	rRole := "acctest_grant_revoke_role"
	rSchema := "acctest_grant_revoke_sch"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "test" { name = %q }
				resource "postgresql_schema" "test" { name = %q }
				resource "postgresql_grant" "test" {
					role        = postgresql_role.test.name
					object_type = "schema"
					schema      = postgresql_schema.test.name
					privileges  = ["USAGE", "CREATE"]
				}`, rRole, rSchema),
				Check: resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
			},
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "test" { name = %q }
				resource "postgresql_schema" "test" { name = %q }
				resource "postgresql_grant" "test" {
					role        = postgresql_role.test.name
					object_type = "schema"
					schema      = postgresql_schema.test.name
					privileges  = ["USAGE"]
				}`, rRole, rSchema),
				Check: resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "1"),
			},
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "test" { name = %q }
				resource "postgresql_schema" "test" { name = %q }
				resource "postgresql_grant" "test" {
					role              = postgresql_role.test.name
					object_type       = "schema"
					schema            = postgresql_schema.test.name
					privileges        = ["USAGE", "CREATE"]
					with_grant_option = true
				}`, rRole, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "2"),
					resource.TestCheckResourceAttr("postgresql_grant.test", "with_grant_option", "true"),
				),
			},
		},
	})
}

func TestAccPostgresqlGrant_disappears(t *testing.T) {
	rRole := "acctest_grant_disapp_role"
	rDB := "acctest_grant_disapp_db"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlGrantConfig_database(rRole, rDB),
				Check:  resource.TestCheckResourceAttrSet("postgresql_grant.test", "id"),
			},
			{
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(fmt.Sprintf(`REVOKE ALL PRIVILEGES ON DATABASE "%s" FROM "%s"`, rDB, rRole))
				},
				Config: testAccPostgresqlGrantConfig_database(rRole, rDB),
				Check:  resource.TestCheckResourceAttrSet("postgresql_grant.test", "id"),
			},
		},
	})
}

func TestAccPostgresqlGrant_databaseAllPrivileges(t *testing.T) {
	rRole := "acctest_grant_allpriv_role"
	rDB := "acctest_grant_allpriv_db"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "test" { name = %q }
				resource "postgresql_database" "test" { name = %q }
				resource "postgresql_grant" "test" {
					role        = postgresql_role.test.name
					object_type = "database"
					database    = postgresql_database.test.name
					privileges  = ["CONNECT", "CREATE", "TEMPORARY"]
				}`, rRole, rDB),
				Check: resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "3"),
			},
		},
	})
}

func TestAccPostgresqlGrant_sequenceUpdate(t *testing.T) {
	rRole := "acctest_grant_seq_upd_role"
	rSchema := "acctest_grant_seq_upd_sch"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "test" { name = %q }
				resource "postgresql_schema" "test" { name = %q }
				resource "postgresql_grant" "test" {
					role        = postgresql_role.test.name
					object_type = "sequence"
					schema      = postgresql_schema.test.name
					privileges  = ["USAGE"]
				}`, rRole, rSchema),
				Check: resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "1"),
			},
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "test" { name = %q }
				resource "postgresql_schema" "test" { name = %q }
				resource "postgresql_grant" "test" {
					role        = postgresql_role.test.name
					object_type = "sequence"
					schema      = postgresql_schema.test.name
					privileges  = ["USAGE", "SELECT", "UPDATE"]
				}`, rRole, rSchema),
				Check: resource.TestCheckResourceAttr("postgresql_grant.test", "privileges.#", "3"),
			},
		},
	})
}

func testAccCheckPostgresqlGrantDestroy(s *terraform.State) error {
	db, err := acctest.GetDB()
	if err != nil {
		return fmt.Errorf("error getting test database connection: %s", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "postgresql_grant" {
			continue
		}

		role := rs.Primary.Attributes["role"]
		objectType := rs.Primary.Attributes["object_type"]

		// Check if the role still exists; if not, the grant is implicitly gone.
		var roleExists int
		err := db.QueryRow("SELECT 1 FROM pg_roles WHERE rolname = $1", role).Scan(&roleExists)
		if err != nil {
			continue // role doesn't exist, grant is gone
		}

		switch objectType {
		case "database":
			database := rs.Primary.Attributes["database"]
			var count int
			err := db.QueryRow(`
				SELECT count(*)
				FROM (
					SELECT (aclexplode(datacl)).grantee
					FROM pg_database WHERE datname = $1 AND datacl IS NOT NULL
				) AS acl
				JOIN pg_roles ON acl.grantee = pg_roles.oid
				WHERE pg_roles.rolname = $2
			`, database, role).Scan(&count)
			if err != nil {
				continue
			}
			if count > 0 {
				return fmt.Errorf("grant still exists: role %q has %d privileges on database %q", role, count, database)
			}

		case "schema":
			schemaName := rs.Primary.Attributes["schema"]
			var count int
			err := db.QueryRow(`
				SELECT count(*)
				FROM (
					SELECT (aclexplode(nspacl)).grantee
					FROM pg_namespace WHERE nspname = $1 AND nspacl IS NOT NULL
				) AS acl
				JOIN pg_roles ON acl.grantee = pg_roles.oid
				WHERE pg_roles.rolname = $2
			`, schemaName, role).Scan(&count)
			if err != nil {
				continue
			}
			if count > 0 {
				return fmt.Errorf("grant still exists: role %q has %d privileges on schema %q", role, count, schemaName)
			}
		}
	}

	return nil
}

func testAccPostgresqlGrantConfig_database(role, dbName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_database" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role        = postgresql_role.test.name
  object_type = "database"
  database    = postgresql_database.test.name
  privileges  = ["CONNECT", "CREATE"]
}
`, role, dbName)
}

func testAccPostgresqlGrantConfig_databaseUpdated(role, dbName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_database" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role              = postgresql_role.test.name
  object_type       = "database"
  database          = postgresql_database.test.name
  privileges        = ["CONNECT"]
  with_grant_option = true
}
`, role, dbName)
}

func testAccPostgresqlGrantConfig_schema(role, schemaName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role        = postgresql_role.test.name
  object_type = "schema"
  schema      = postgresql_schema.test.name
  privileges  = ["USAGE", "CREATE"]
}
`, role, schemaName)
}

func testAccPostgresqlGrantConfig_table(role, schemaName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role        = postgresql_role.test.name
  object_type = "table"
  schema      = postgresql_schema.test.name
  privileges  = ["SELECT", "INSERT"]
}
`, role, schemaName)
}

func testAccPostgresqlGrantConfig_sequence(role, schemaName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role        = postgresql_role.test.name
  object_type = "sequence"
  schema      = postgresql_schema.test.name
  privileges  = ["USAGE", "SELECT"]
}
`, role, schemaName)
}

func testAccPostgresqlGrantConfig_function(role, schemaName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role        = postgresql_role.test.name
  object_type = "function"
  schema      = postgresql_schema.test.name
  privileges  = ["EXECUTE"]
}
`, role, schemaName)
}

func testAccPostgresqlGrantConfig_schemaUsageOnly(role, schemaName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role        = postgresql_role.test.name
  object_type = "schema"
  schema      = postgresql_schema.test.name
  privileges  = ["USAGE"]
}
`, role, schemaName)
}

// testAccPostgresqlGrantConfig_specificObject creates a grant on a specific named object.
// Schema is NOT managed by Terraform (created in PreConfig) to avoid DROP CASCADE issues.
func testAccPostgresqlGrantConfig_specificObject(role, schemaName, objectType, objectName, privileges string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role        = postgresql_role.test.name
  object_type = %q
  schema      = %q
  objects     = [%q]
  privileges  = %s
}
`, role, objectType, schemaName, objectName, privileges)
}

func testAccPostgresqlGrantConfig_withGrantOption(role, dbName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_database" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role              = postgresql_role.test.name
  object_type       = "database"
  database          = postgresql_database.test.name
  privileges        = ["CONNECT", "CREATE"]
  with_grant_option = true
}
`, role, dbName)
}
