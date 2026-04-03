//go:build integration

// Tests for postgresql_database resource.
package resource_test

import (
	"database/sql"
	"fmt"
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccPostgresqlDatabase_basic(t *testing.T) {
	rName := "acctest_db_basic"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDatabaseConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_database.test", "encoding", "UTF8"),
					resource.TestCheckResourceAttr("postgresql_database.test", "template", "template0"),
					resource.TestCheckResourceAttr("postgresql_database.test", "connection_limit", "-1"),
					resource.TestCheckResourceAttr("postgresql_database.test", "allow_connections", "true"),
					resource.TestCheckResourceAttr("postgresql_database.test", "is_template", "false"),
					resource.TestCheckResourceAttr("postgresql_database.test", "tablespace_name", "pg_default"),
					resource.TestCheckResourceAttrSet("postgresql_database.test", "owner"),
					resource.TestCheckResourceAttrSet("postgresql_database.test", "lc_collate"),
					resource.TestCheckResourceAttrSet("postgresql_database.test", "lc_ctype"),
					resource.TestCheckResourceAttrSet("postgresql_database.test", "oid"),
				),
			},
			{
				ResourceName:                         "postgresql_database.test",
				ImportState:                          true,
				ImportStateId:                        rName,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

func TestAccPostgresqlDatabase_full(t *testing.T) {
	rName := "acctest_db_full"
	ownerRole := "acctest_db_full_owner"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDatabaseConfig_full(rName, ownerRole),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_database.test", "owner", ownerRole),
					resource.TestCheckResourceAttr("postgresql_database.test", "encoding", "UTF8"),
					resource.TestCheckResourceAttr("postgresql_database.test", "template", "template0"),
					resource.TestCheckResourceAttr("postgresql_database.test", "connection_limit", "50"),
					resource.TestCheckResourceAttr("postgresql_database.test", "allow_connections", "true"),
					resource.TestCheckResourceAttr("postgresql_database.test", "is_template", "false"),
					resource.TestCheckResourceAttr("postgresql_database.test", "tablespace_name", "pg_default"),
					resource.TestCheckResourceAttrSet("postgresql_database.test", "lc_collate"),
					resource.TestCheckResourceAttrSet("postgresql_database.test", "lc_ctype"),
					resource.TestCheckResourceAttrSet("postgresql_database.test", "oid"),
				),
			},
		},
	})
}

func TestAccPostgresqlDatabase_update(t *testing.T) {
	rName := "acctest_db_update"
	ownerRole := "acctest_db_upd_owner"
	newOwnerRole := "acctest_db_upd_newowner"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDatabaseConfig_forUpdate(rName, ownerRole, newOwnerRole, false, 10),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_database.test", "owner", ownerRole),
					resource.TestCheckResourceAttr("postgresql_database.test", "connection_limit", "10"),
				),
			},
			{
				Config: testAccPostgresqlDatabaseConfig_forUpdate(rName, ownerRole, newOwnerRole, true, 20),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_database.test", "owner", newOwnerRole),
					resource.TestCheckResourceAttr("postgresql_database.test", "connection_limit", "20"),
				),
			},
		},
	})
}

func TestAccPostgresqlDatabase_withLocale(t *testing.T) {
	rName := "acctest_db_locale"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_database" "test" {
  name             = %q
  encoding         = "UTF8"
  template         = "template0"
  lc_collate       = "C"
  lc_ctype         = "C"
  connection_limit = 10
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_database.test", "encoding", "UTF8"),
					resource.TestCheckResourceAttr("postgresql_database.test", "lc_collate", "C"),
					resource.TestCheckResourceAttr("postgresql_database.test", "lc_ctype", "C"),
					resource.TestCheckResourceAttr("postgresql_database.test", "connection_limit", "10"),
				),
			},
			{
				ResourceName:                         "postgresql_database.test",
				ImportState:                          true,
				ImportStateId:                        rName,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

func TestAccPostgresqlDatabase_isTemplate(t *testing.T) {
	rName := "acctest_db_tmpl"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDatabaseConfig_isTemplate(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_database.test", "is_template", "true"),
				),
			},
		},
	})
}

func TestAccPostgresqlDatabase_updateFlags(t *testing.T) {
	rName := "acctest_db_flags"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDatabaseConfig_withFlags(rName, true, false, -1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_database.test", "allow_connections", "true"),
					resource.TestCheckResourceAttr("postgresql_database.test", "is_template", "false"),
					resource.TestCheckResourceAttr("postgresql_database.test", "connection_limit", "-1"),
				),
			},
			{
				Config: testAccPostgresqlDatabaseConfig_withFlags(rName, true, true, 100),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.test", "is_template", "true"),
					resource.TestCheckResourceAttr("postgresql_database.test", "connection_limit", "100"),
				),
			},
			// Revert is_template to allow proper cleanup
			{
				Config: testAccPostgresqlDatabaseConfig_withFlags(rName, true, false, 100),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.test", "is_template", "false"),
				),
			},
		},
	})
}

func TestAccPostgresqlDatabase_emptyName(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `resource "postgresql_database" "test" { name = "" }`,
				ExpectError: regexp.MustCompile(`must be between 1 and 63`),
			},
		},
	})
}

func TestAccPostgresqlDatabase_invalidTemplate(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "postgresql_database" "test" {
					name     = "acctest_db_inv_tmpl"
					template = "nonexistent_template"
				}`,
				ExpectError: regexp.MustCompile(`template database .* does not exist|Error creating database`),
			},
		},
	})
}

func TestAccPostgresqlDatabase_connectionLimitUpdate(t *testing.T) {
	rName := "acctest_db_connlimit"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "postgresql_database" "test" {
					name             = %q
					connection_limit = 10
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_database.test", "connection_limit", "10"),
			},
			{
				Config: fmt.Sprintf(`resource "postgresql_database" "test" {
					name             = %q
					connection_limit = 50
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_database.test", "connection_limit", "50"),
			},
			{
				Config: fmt.Sprintf(`resource "postgresql_database" "test" {
					name             = %q
					connection_limit = -1
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_database.test", "connection_limit", "-1"),
			},
		},
	})
}

func TestAccPostgresqlDatabase_disappears(t *testing.T) {
	rName := "acctest_db_disappears"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "postgresql_database" "test" { name = %q }`, rName),
				Check:  resource.TestCheckResourceAttr("postgresql_database.test", "name", rName),
			},
			{
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, rName))
				},
				Config: fmt.Sprintf(`resource "postgresql_database" "test" { name = %q }`, rName),
				Check:  resource.TestCheckResourceAttr("postgresql_database.test", "name", rName),
			},
		},
	})
}

func TestAccPostgresqlDatabase_ownerChange(t *testing.T) {
	rName := "acctest_db_ownchg"
	owner1 := "acctest_db_ownchg_o1"
	owner2 := "acctest_db_ownchg_o2"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDatabaseConfig_forUpdate(rName, owner1, owner2, false, -1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_database.test", "owner", owner1),
				),
			},
			{
				Config: testAccPostgresqlDatabaseConfig_forUpdate(rName, owner1, owner2, true, 25),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.test", "owner", owner2),
					resource.TestCheckResourceAttr("postgresql_database.test", "connection_limit", "25"),
				),
			},
		},
	})
}

func TestAccPostgresqlDatabase_templateIsTemplateLifecycle(t *testing.T) {
	rName := "acctest_db_tmpl_lifecycle"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "postgresql_database" "test" {
					name        = %q
					is_template = true
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_database.test", "is_template", "true"),
			},
			{
				Config: fmt.Sprintf(`resource "postgresql_database" "test" {
					name        = %q
					is_template = false
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_database.test", "is_template", "false"),
			},
		},
	})
}

func TestAccPostgresqlDatabase_allowConnectionsToggle(t *testing.T) {
	rName := "acctest_db_allowconn"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "postgresql_database" "test" {
					name              = %q
					allow_connections = true
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_database.test", "allow_connections", "true"),
			},
			{
				Config: fmt.Sprintf(`resource "postgresql_database" "test" {
					name              = %q
					allow_connections = false
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_database.test", "allow_connections", "false"),
			},
		},
	})
}

func testAccCheckPostgresqlDatabaseDestroy(s *terraform.State) error {
	db, err := acctest.GetDB()
	if err != nil {
		return fmt.Errorf("error getting test database connection: %s", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "postgresql_database" {
			continue
		}

		dbName := rs.Primary.Attributes["name"]
		var exists int
		err := db.QueryRow("SELECT 1 FROM pg_database WHERE datname = $1", dbName).Scan(&exists)
		if err == nil {
			return fmt.Errorf("postgresql database %q still exists", dbName)
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("error checking if database %q exists: %s", dbName, err)
		}
	}

	return nil
}

func testAccPostgresqlDatabaseConfig_basic(name string) string {
	return fmt.Sprintf(`
resource "postgresql_database" "test" {
  name = %q
}
`, name)
}

func testAccPostgresqlDatabaseConfig_full(name, owner string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "owner" {
  name = %q
}

resource "postgresql_database" "test" {
  name             = %q
  owner            = postgresql_role.owner.name
  encoding         = "UTF8"
  template         = "template0"
  connection_limit = 50
  allow_connections = true
  is_template      = false
  tablespace_name  = "pg_default"
}
`, owner, name)
}

func testAccPostgresqlDatabaseConfig_isTemplate(name string) string {
	return fmt.Sprintf(`
resource "postgresql_database" "test" {
  name        = %q
  is_template = true
}
`, name)
}

func testAccPostgresqlDatabaseConfig_withFlags(name string, allowConn, isTemplate bool, connLimit int) string {
	return fmt.Sprintf(`
resource "postgresql_database" "test" {
  name              = %q
  allow_connections = %t
  is_template       = %t
  connection_limit  = %d
}
`, name, allowConn, isTemplate, connLimit)
}

// Example-based test: validate import workflow from examples/resources/postgresql_database/import.sh

func TestAccPostgresqlDatabase_exampleImport(t *testing.T) {
	rName := "acctest_db_ex_imp"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_database" "mydb" {
  name = %q
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_database.mydb", "name", rName),
					resource.TestCheckResourceAttrSet("postgresql_database.mydb", "owner"),
					resource.TestCheckResourceAttrSet("postgresql_database.mydb", "encoding"),
				),
			},
			{
				ResourceName:                         "postgresql_database.mydb",
				ImportState:                          true,
				ImportStateId:                        rName,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

func testAccPostgresqlDatabaseConfig_forUpdate(name, owner1, owner2 string, useOwner2 bool, connLimit int) string {
	ownerRef := "postgresql_role.owner1.name"
	if useOwner2 {
		ownerRef = "postgresql_role.owner2.name"
	}
	return fmt.Sprintf(`
resource "postgresql_role" "owner1" {
  name = %q
}

resource "postgresql_role" "owner2" {
  name = %q
}

resource "postgresql_database" "test" {
  name             = %q
  owner            = %s
  connection_limit = %d
}
`, owner1, owner2, name, ownerRef, connLimit)
}
