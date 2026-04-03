// Tests for postgresql_role resource.
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
					resource.TestCheckResourceAttr("postgresql_role.test", "login", "false"),
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
				// Password is not readable from PostgreSQL, so skip it during import verification.
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}

func TestAccPostgresqlRole_full(t *testing.T) {
	rName := "acctest_role_full"
	parentRole := "acctest_role_full_parent"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlRoleConfig_full(rName, parentRole),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_role.test", "login", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "superuser", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_database", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_role", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "replication", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "connection_limit", "10"),
					resource.TestCheckResourceAttr("postgresql_role.test", "valid_until", "2099-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr("postgresql_role.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("postgresql_role.test", "roles.0", parentRole),
					resource.TestCheckResourceAttrSet("postgresql_role.test", "oid"),
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
					resource.TestCheckResourceAttr("postgresql_role.test", "login", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_database", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "connection_limit", "-1"),
				),
			},
			{
				Config: testAccPostgresqlRoleConfig_updated(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_role.test", "login", "true"),
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

func TestAccPostgresqlRole_membershipChange(t *testing.T) {
	rName := "acctest_role_mc"
	parentA := "acctest_role_mc_pa"
	parentB := "acctest_role_mc_pb"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlRoleConfig_withMembership(rName, parentA, parentB, true, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_role.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("postgresql_role.test", "roles.0", parentA),
				),
			},
			// Switch membership: revoke parentA, grant parentB
			{
				Config: testAccPostgresqlRoleConfig_withMembership(rName, parentA, parentB, false, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("postgresql_role.test", "roles.0", parentB),
				),
			},
			// Remove all memberships
			{
				Config: testAccPostgresqlRoleConfig_withMembership(rName, parentA, parentB, false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "roles.#", "0"),
				),
			},
		},
	})
}

func TestAccPostgresqlRole_password(t *testing.T) {
	rName := "acctest_role_pwd"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "test" {
  name     = %q
  login    = true
  password = "initial_password"
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_role.test", "login", "true"),
				),
			},
			// Update password
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "test" {
  name     = %q
  login    = true
  password = "new_password"
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
				),
			},
		},
	})
}

func TestAccPostgresqlRole_allFlags(t *testing.T) {
	rName := "acctest_role_allflags"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "test" {
  name             = %q
  login            = true
  create_database  = true
  create_role      = true
  replication      = true
  connection_limit = 3
  valid_until      = "2099-12-31T23:59:59Z"
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("postgresql_role.test", "login", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_database", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_role", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "replication", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "connection_limit", "3"),
					resource.TestCheckResourceAttrSet("postgresql_role.test", "valid_until"),
				),
			},
			// Disable most flags
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "test" {
  name             = %q
  login            = false
  create_database  = false
  create_role      = false
  replication      = false
  connection_limit = -1
  valid_until      = "2099-12-31T23:59:59Z"
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "login", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_database", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_role", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "replication", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "connection_limit", "-1"),
				),
			},
		},
	})
}

func TestAccPostgresqlRole_bothMemberships(t *testing.T) {
	rName := "acctest_role_both"
	parentA := "acctest_role_both_pa"
	parentB := "acctest_role_both_pb"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlRoleConfig_withMembership(rName, parentA, parentB, true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "roles.#", "2"),
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
				// Simulate external deletion, then re-apply to recreate
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

func TestAccPostgresqlRole_invalidValidUntil(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "postgresql_role" "test" {
					name        = "acctest_role_invalid_vu"
					valid_until = "not-a-date"
				}`,
				ExpectError: regexp.MustCompile(`must be a valid timestamp`),
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

func TestAccPostgresqlRole_superuserWithReplication(t *testing.T) {
	rName := "acctest_role_super_repl"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "test" {
					name        = %q
					superuser   = true
					replication = true
					login       = true
					password    = "test123"
				}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "superuser", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "replication", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "login", "true"),
				),
			},
		},
	})
}

func TestAccPostgresqlRole_updateAllFlags(t *testing.T) {
	rName := "acctest_role_upd_flags"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "test" {
					name             = %q
					login            = false
					superuser        = false
					create_database  = false
					create_role      = false
					replication      = false
					connection_limit = -1
				}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "login", "false"),
					resource.TestCheckResourceAttr("postgresql_role.test", "superuser", "false"),
				),
			},
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "test" {
					name             = %q
					login            = true
					superuser        = true
					create_database  = true
					create_role      = true
					replication      = true
					connection_limit = 50
				}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "login", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "superuser", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_database", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "create_role", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "replication", "true"),
					resource.TestCheckResourceAttr("postgresql_role.test", "connection_limit", "50"),
				),
			},
		},
	})
}

func TestAccPostgresqlRole_createWithMultipleMemberships(t *testing.T) {
	rName := "acctest_role_multi_memb"
	parent1 := "acctest_role_multi_p1"
	parent2 := "acctest_role_multi_p2"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "parent1" { name = %q }
				resource "postgresql_role" "parent2" { name = %q }
				resource "postgresql_role" "test" {
					name     = %q
					login    = true
					password = "testpass"
					roles    = [postgresql_role.parent1.name, postgresql_role.parent2.name]
				}`, parent1, parent2, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.test", "roles.#", "2"),
					resource.TestCheckResourceAttr("postgresql_role.test", "login", "true"),
				),
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

func TestAccPostgresqlRole_passwordUpdate(t *testing.T) {
	rName := "acctest_role_pwd_upd"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "postgresql_role" "test" {
					name     = %q
					login    = true
					password = "first_password"
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_role.test", "login", "true"),
			},
			{
				Config: fmt.Sprintf(`resource "postgresql_role" "test" {
					name     = %q
					login    = true
					password = "second_password"
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_role.test", "login", "true"),
			},
			{
				Config: fmt.Sprintf(`resource "postgresql_role" "test" {
					name  = %q
					login = true
				}`, rName),
				Check: resource.TestCheckResourceAttr("postgresql_role.test", "login", "true"),
			},
		},
	})
}

func TestAccPostgresqlRole_validUntil(t *testing.T) {
	rName := "acctest_role_valid_until"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`resource "postgresql_role" "test" {
					name        = %q
					login       = true
					password    = "testpass"
					valid_until = "2030-12-31T23:59:59Z"
				}`, rName),
				Check: resource.TestCheckResourceAttrSet("postgresql_role.test", "valid_until"),
			},
			{
				Config: fmt.Sprintf(`resource "postgresql_role" "test" {
					name        = %q
					login       = true
					password    = "testpass"
					valid_until = "2035-06-15T12:00:00Z"
				}`, rName),
				Check: resource.TestCheckResourceAttrSet("postgresql_role.test", "valid_until"),
			},
		},
	})
}

func TestAccPostgresqlRole_removeMemberships(t *testing.T) {
	rName := "acctest_role_rm_memb"
	parent := "acctest_role_rm_memb_p"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "parent" { name = %q }
				resource "postgresql_role" "test" {
					name  = %q
					roles = [postgresql_role.parent.name]
				}`, parent, rName),
				Check: resource.TestCheckResourceAttr("postgresql_role.test", "roles.#", "1"),
			},
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "parent" { name = %q }
				resource "postgresql_role" "test" {
					name  = %q
					roles = []
				}`, parent, rName),
				Check: resource.TestCheckResourceAttr("postgresql_role.test", "roles.#", "0"),
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

func testAccPostgresqlRoleConfig_full(name, parentRole string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "parent" {
  name = %q
}

resource "postgresql_role" "test" {
  name             = %q
  login            = true
  superuser        = false
  create_database  = true
  password         = "testpassword123"
  connection_limit = 10
  valid_until      = "2099-01-01T00:00:00Z"
  roles            = [postgresql_role.parent.name]
}
`, parentRole, name)
}

func testAccPostgresqlRoleConfig_withMembership(name, parentA, parentB string, memberA, memberB bool) string {
	var rolesLine string
	switch {
	case memberA && memberB:
		rolesLine = `  roles = [postgresql_role.parent_a.name, postgresql_role.parent_b.name]`
	case memberA:
		rolesLine = `  roles = [postgresql_role.parent_a.name]`
	case memberB:
		rolesLine = `  roles = [postgresql_role.parent_b.name]`
	default:
		rolesLine = `  roles = []`
	}

	return fmt.Sprintf(`
resource "postgresql_role" "parent_a" {
  name = %q
}

resource "postgresql_role" "parent_b" {
  name = %q
}

resource "postgresql_role" "test" {
  name = %q
%s
}
`, parentA, parentB, name, rolesLine)
}

func testAccPostgresqlRoleConfig_updated(name string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name             = %q
  login            = true
  create_database  = true
  connection_limit = 5
}
`, name)
}
