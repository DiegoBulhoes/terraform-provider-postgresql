package provider

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccPostgresqlDatabase_basic(t *testing.T) {
	rName := "acctest_db_basic"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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

func testAccCheckPostgresqlDatabaseDestroy(s *terraform.State) error {
	db, err := testAccGetDB()
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
