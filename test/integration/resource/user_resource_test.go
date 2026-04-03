//go:build integration

// Tests for postgresql_user resource.
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

func TestAccPostgresqlUser_basic(t *testing.T) {
	rName := "acctest_user_basic"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_user" "test" {
  name     = %q
  password = "testpass"
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_user.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_user.test", "superuser", "false"),
					resource.TestCheckResourceAttr("postgresql_user.test", "create_database", "false"),
					resource.TestCheckResourceAttr("postgresql_user.test", "create_role", "false"),
					resource.TestCheckResourceAttr("postgresql_user.test", "replication", "false"),
					resource.TestCheckResourceAttr("postgresql_user.test", "connection_limit", "-1"),
					resource.TestCheckResourceAttrSet("postgresql_user.test", "oid"),
				),
			},
			{
				ResourceName:                         "postgresql_user.test",
				ImportState:                          true,
				ImportStateId:                        rName,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"password"},
			},
		},
	})
}

func TestAccPostgresqlUser_withRoles(t *testing.T) {
	rName := "acctest_user_roles"
	roleName := "acctest_user_roles_role"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "group" {
  name = %q
}

resource "postgresql_user" "test" {
  name     = %q
  password = "testpass"
  roles    = [postgresql_role.group.name]
}
`, roleName, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_user.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_user.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("postgresql_user.test", "roles.0", roleName),
				),
			},
		},
	})
}

func TestAccPostgresqlUser_update(t *testing.T) {
	rName := "acctest_user_update"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_user" "test" {
  name     = %q
  password = "initial_pass"
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_user.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_user.test", "create_database", "false"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "postgresql_user" "test" {
  name             = %q
  password         = "updated_pass"
  create_database  = true
  connection_limit = 5
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_user.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_user.test", "create_database", "true"),
					resource.TestCheckResourceAttr("postgresql_user.test", "connection_limit", "5"),
				),
			},
		},
	})
}

func TestAccPostgresqlUser_rename(t *testing.T) {
	rName := "acctest_user_ren_old"
	rNameNew := "acctest_user_ren_new"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_user" "test" {
  name     = %q
  password = "testpass"
}
`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_user.test", "name", rName),
			},
			{
				Config: fmt.Sprintf(`
resource "postgresql_user" "test" {
  name     = %q
  password = "testpass"
}
`, rNameNew),
				Check: resource.TestCheckResourceAttr("postgresql_user.test", "name", rNameNew),
			},
		},
	})
}

func TestAccPostgresqlUser_allFlags(t *testing.T) {
	rName := "acctest_user_allflags"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_user" "test" {
  name             = %q
  password         = "testpass"
  superuser        = true
  create_database  = true
  create_role      = true
  replication      = true
  connection_limit = 3
  valid_until      = "2099-12-31T23:59:59Z"
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_user.test", "superuser", "true"),
					resource.TestCheckResourceAttr("postgresql_user.test", "create_database", "true"),
					resource.TestCheckResourceAttr("postgresql_user.test", "create_role", "true"),
					resource.TestCheckResourceAttr("postgresql_user.test", "replication", "true"),
					resource.TestCheckResourceAttr("postgresql_user.test", "connection_limit", "3"),
					resource.TestCheckResourceAttrSet("postgresql_user.test", "valid_until"),
				),
			},
		},
	})
}

func TestAccPostgresqlUser_invalidConnectionLimit(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "postgresql_user" "test" {
					name             = "acctest_user_invalid_cl"
					password         = "testpass"
					connection_limit = -5
				}`,
				ExpectError: regexp.MustCompile(`must be at least -1`),
			},
		},
	})
}

func TestAccPostgresqlUser_invalidValidUntil(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "postgresql_user" "test" {
					name        = "acctest_user_invalid_vu"
					password    = "testpass"
					valid_until = "not-a-date"
				}`,
				ExpectError: regexp.MustCompile(`must be a valid timestamp`),
			},
		},
	})
}

func TestAccPostgresqlUser_membershipChange(t *testing.T) {
	rName := "acctest_user_mc"
	parentA := "acctest_user_mc_pa"
	parentB := "acctest_user_mc_pb"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "parent_a" { name = %q }
resource "postgresql_role" "parent_b" { name = %q }
resource "postgresql_user" "test" {
  name     = %q
  password = "testpass"
  roles    = [postgresql_role.parent_a.name]
}
`, parentA, parentB, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_user.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("postgresql_user.test", "roles.0", parentA),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "parent_a" { name = %q }
resource "postgresql_role" "parent_b" { name = %q }
resource "postgresql_user" "test" {
  name     = %q
  password = "testpass"
  roles    = [postgresql_role.parent_b.name]
}
`, parentA, parentB, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_user.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("postgresql_user.test", "roles.0", parentB),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "parent_a" { name = %q }
resource "postgresql_role" "parent_b" { name = %q }
resource "postgresql_user" "test" {
  name     = %q
  password = "testpass"
  roles    = []
}
`, parentA, parentB, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_user.test", "roles.#", "0"),
				),
			},
		},
	})
}

func testAccCheckPostgresqlUserDestroy(s *terraform.State) error {
	db, err := acctest.GetDB()
	if err != nil {
		return fmt.Errorf("error getting test database connection: %s", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "postgresql_user" {
			continue
		}

		userName := rs.Primary.Attributes["name"]
		var exists int
		err := db.QueryRow("SELECT 1 FROM pg_roles WHERE rolname = $1", userName).Scan(&exists)
		if err == nil {
			return fmt.Errorf("postgresql user %q still exists", userName)
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("error checking if user %q exists: %s", userName, err)
		}
	}

	return nil
}
