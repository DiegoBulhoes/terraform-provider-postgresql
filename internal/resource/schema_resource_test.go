//go:build integration

// Tests for postgresql_schema resource.
package resource_test

import (
	"database/sql"
	"fmt"
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccPostgresqlSchema_basic(t *testing.T) {
	rName := "acctest_schema_basic"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlSchemaDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlSchemaConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_schema.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_schema.test", "if_not_exists", "false"),
					resource.TestCheckResourceAttrSet("postgresql_schema.test", "owner"),
					resource.TestCheckResourceAttrSet("postgresql_schema.test", "database"),
				),
			},
			{
				ResourceName:                         "postgresql_schema.test",
				ImportState:                          true,
				ImportStateId:                        rName,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

func TestAccPostgresqlSchema_withOwner(t *testing.T) {
	rName := "acctest_schema_owner"
	ownerRole := "acctest_schema_owner_role"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlSchemaDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlSchemaConfig_withOwner(rName, ownerRole),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_schema.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_schema.test", "owner", ownerRole),
					resource.TestCheckResourceAttrSet("postgresql_schema.test", "database"),
				),
			},
		},
	})
}

func TestAccPostgresqlSchema_ifNotExists(t *testing.T) {
	rName := "acctest_schema_ine"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlSchemaDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlSchemaConfig_ifNotExists(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_schema.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_schema.test", "if_not_exists", "true"),
					resource.TestCheckResourceAttrSet("postgresql_schema.test", "owner"),
				),
			},
		},
	})
}

func TestAccPostgresqlSchema_importWithDatabase(t *testing.T) {
	rName := "acctest_schema_impdb"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlSchemaDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlSchemaConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_schema.test", "name", rName),
				),
			},
			{
				ResourceName:                         "postgresql_schema.test",
				ImportState:                          true,
				ImportStateId:                        "postgres/" + rName,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

func TestAccPostgresqlSchema_update(t *testing.T) {
	rName := "acctest_schema_upd"
	rNameRenamed := "acctest_schema_upd_new"
	ownerRole := "acctest_schema_upd_owner"
	newOwnerRole := "acctest_schema_upd_newowner"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlSchemaDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlSchemaConfig_forUpdate(rName, ownerRole, newOwnerRole, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_schema.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_schema.test", "owner", ownerRole),
				),
			},
			// Update: change owner
			{
				Config: testAccPostgresqlSchemaConfig_forUpdate(rName, ownerRole, newOwnerRole, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_schema.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_schema.test", "owner", newOwnerRole),
				),
			},
			// Update: rename
			{
				Config: testAccPostgresqlSchemaConfig_forUpdate(rNameRenamed, ownerRole, newOwnerRole, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_schema.test", "name", rNameRenamed),
					resource.TestCheckResourceAttr("postgresql_schema.test", "owner", newOwnerRole),
				),
			},
		},
	})
}

func TestAccPostgresqlSchema_disappears(t *testing.T) {
	rName := "acctest_schema_disappears"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "postgresql_schema" "test" { name = %q }`, rName),
				Check:  resource.TestCheckResourceAttr("postgresql_schema.test", "name", rName),
			},
			{
				// Simulate external deletion, then re-apply to recreate
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s"`, rName))
				},
				Config: fmt.Sprintf(`resource "postgresql_schema" "test" { name = %q }`, rName),
				Check:  resource.TestCheckResourceAttr("postgresql_schema.test", "name", rName),
			},
		},
	})
}

func TestAccPostgresqlSchema_emptyName(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `resource "postgresql_schema" "test" { name = "" }`,
				ExpectError: regexp.MustCompile(`must be between 1 and 63`),
			},
		},
	})
}

func TestAccPostgresqlSchema_ifNotExistsIdempotent(t *testing.T) {
	rName := "acctest_schema_idempotent"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				// Pre-create the schema externally
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, rName))
				},
				Config: fmt.Sprintf(`resource "postgresql_schema" "test" {
					name          = %q
					if_not_exists = true
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_schema.test", "name", rName),
			},
		},
	})
}

func TestAccPostgresqlSchema_ownerChangeAndRename(t *testing.T) {
	rName := "acctest_sch_own_ren"
	newName := "acctest_sch_own_ren2"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "owner" { name = "acctest_sch_oc_owner" }
				resource "postgresql_schema" "test" {
					name  = %q
					owner = postgresql_role.owner.name
				}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_schema.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_schema.test", "owner", "acctest_sch_oc_owner"),
				),
			},
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "owner" { name = "acctest_sch_oc_owner" }
				resource "postgresql_schema" "test" {
					name  = %q
					owner = "postgres"
				}`, newName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_schema.test", "name", newName),
					resource.TestCheckResourceAttr("postgresql_schema.test", "owner", "postgres"),
				),
			},
		},
	})
}

func testAccCheckPostgresqlSchemaDestroy(s *terraform.State) error {
	db, err := acctest.GetDB()
	if err != nil {
		return fmt.Errorf("error getting test database connection: %s", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "postgresql_schema" {
			continue
		}

		schemaName := rs.Primary.Attributes["name"]
		var exists int
		err := db.QueryRow("SELECT 1 FROM information_schema.schemata WHERE schema_name = $1", schemaName).Scan(&exists)
		if err == nil {
			return fmt.Errorf("postgresql schema %q still exists", schemaName)
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("error checking if schema %q exists: %s", schemaName, err)
		}
	}

	return nil
}

func testAccPostgresqlSchemaConfig_basic(name string) string {
	return fmt.Sprintf(`
resource "postgresql_schema" "test" {
  name = %q
}
`, name)
}

func testAccPostgresqlSchemaConfig_withOwner(name, owner string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "owner" {
  name = %q
}

resource "postgresql_schema" "test" {
  name  = %q
  owner = postgresql_role.owner.name
}
`, owner, name)
}

func testAccPostgresqlSchemaConfig_ifNotExists(name string) string {
	return fmt.Sprintf(`
resource "postgresql_schema" "test" {
  name          = %q
  if_not_exists = true
}
`, name)
}

func testAccPostgresqlSchemaConfig_forUpdate(name, owner1, owner2 string, useOwner2 bool) string {
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

resource "postgresql_schema" "test" {
  name  = %q
  owner = %s
}
`, owner1, owner2, name, ownerRef)
}
