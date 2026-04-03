//go:build integration

// Tests for postgresql_role resource.
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

func TestAccPostgresqlRole_basic(t *testing.T) {
	rName := "acctest_role_basic"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_role.test", "superuser", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_database", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_role", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "replication", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "connection_limit", "-1"),
					resource.TestCheckResourceAttrSet("postgresql_role.test", "oid"),
				),
			},
			{
				ResourceName:                         "postgresql_role.test",
				ImportState:                          true,
				ImportStateId:                        rName,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"privilege"},
			},
		},
	})
}

func TestAccPostgresqlRole_withPrivileges(t *testing.T) {
	rName := "acctest_role_privs"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q

  privilege {
    privileges  = ["SELECT"]
    object_type = "table"
    schema      = "public"
  }
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_role.test", "privilege.#", "1"),
					resource.TestCheckResourceAttr("postgresql_role.test", "privilege.0.object_type", "table"),
				),
			},
		},
	})
}

func TestAccPostgresqlRole_multiplePrivileges(t *testing.T) {
	rName := "acctest_role_multi_privs"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q

  privilege {
    privileges  = ["SELECT", "INSERT"]
    object_type = "table"
    schema      = "public"
  }

  privilege {
    privileges  = ["USAGE", "SELECT"]
    object_type = "sequence"
    schema      = "public"
  }
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_role.test", "privilege.#", "2"),
				),
			},
		},
	})
}

func TestAccPostgresqlRole_update(t *testing.T) {
	rName := "acctest_role_update"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_database", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "connection_limit", "-1"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "test" {
  name             = %q
  create_database  = true
  connection_limit = 5
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_database", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "connection_limit", "5"),
				),
			},
		},
	})
}

func TestAccPostgresqlRole_rename(t *testing.T) {
	rName := "acctest_role_ren_old"
	rNameNew := "acctest_role_ren_new"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
				),
			},
			{
				Config: testAccPostgresqlRoleConfig_basic(rNameNew),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rNameNew),
				),
			},
		},
	})
}

func TestAccPostgresqlRole_disappears(t *testing.T) {
	rName := "acctest_role_disappears"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
				),
			},
			{
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(fmt.Sprintf(`DROP ROLE IF EXISTS "%s"`, rName))
				},
				Config: testAccPostgresqlRoleConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
				),
			},
		},
	})
}

func TestAccPostgresqlRole_invalidConnectionLimit(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "postgresql_role" "test" {
					name             = "acctest_role_invalid_cl"
					connection_limit = -5
				}`,
				ExpectError: regexp.MustCompile(`must be at least -1`),
			},
		},
	})
}

func TestAccPostgresqlRole_nameTooLong(t *testing.T) {
	longName := "a123456789012345678901234567890123456789012345678901234567890abcd" // 65 chars

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      fmt.Sprintf(`resource "postgresql_role" "test" { name = %q }`, longName),
				ExpectError: regexp.MustCompile(`must be between 1 and 63`),
			},
		},
	})
}

func TestAccPostgresqlRole_emptyName(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `resource "postgresql_role" "test" { name = "" }`,
				ExpectError: regexp.MustCompile(`must be between 1 and 63`),
			},
		},
	})
}

func TestAccPostgresqlRole_connectionLimitZero(t *testing.T) {
	rName := "acctest_role_cl_zero"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "postgresql_role" "test" {
					name             = %q
					connection_limit = 0
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_role.test", "connection_limit", "0"),
			},
		},
	})
}

func TestAccPostgresqlRole_exampleImport(t *testing.T) {
	rName := "acctest_role_ex_imp"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "app" {
  name = %q
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.app", "name", rName),
				),
			},
			{
				ResourceName:                         "postgresql_role.app",
				ImportState:                          true,
				ImportStateId:                        rName,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"privilege"},
			},
		},
	})
}

func testAccCheckPostgresqlRoleDestroy(s *terraform.State) error {
	db, err := acctest.GetDB()
	if err != nil {
		return fmt.Errorf("error getting test database connection: %s", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "postgresql_role" {
			continue
		}

		roleName := rs.Primary.Attributes["name"]
		var exists int
		err := db.QueryRow("SELECT 1 FROM pg_roles WHERE rolname = $1", roleName).Scan(&exists)
		if err == nil {
			return fmt.Errorf("postgresql role %q still exists", roleName)
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("error checking if role %q exists: %s", roleName, err)
		}
	}

	return nil
}

func testAccPostgresqlRoleConfig_basic(name string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}
`, name)
}
