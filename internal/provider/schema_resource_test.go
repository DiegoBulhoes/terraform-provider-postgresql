package provider

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccPostgresqlSchema_basic(t *testing.T) {
	rName := "acctest_schema_basic"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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

func testAccCheckPostgresqlSchemaDestroy(s *terraform.State) error {
	db, err := testAccGetDB()
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
