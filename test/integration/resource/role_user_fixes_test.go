//go:build integration

package resource_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func roleExists(name string, want bool) resource.TestCheckFunc {
	return hasPrivilege(fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s')`, name), want)
}

// A timestamp with an offset is the same instant the server returns in UTC.
func TestAccPostgresqlUser_validUntilWithOffset(t *testing.T) {
	config := `
resource "postgresql_user" "test" {
  name        = "acctest_ufx_valid"
  valid_until = "2030-01-01T12:00:00+02:00"
}`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{Config: config},
			{Config: config, ConfigPlanChecks: emptyPlan()},
		},
	})
}

// Removing valid_until from the config clears the expiry.
func TestAccPostgresqlUser_removeValidUntil(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_user" "test" {
  name        = "acctest_ufx_novalid"
  valid_until = "2030-01-01T00:00:00Z"
}`,
			},
			{
				Config: `
resource "postgresql_user" "test" {
  name = "acctest_ufx_novalid"
}`,
				Check: hasPrivilege(`SELECT coalesce(rolvaliduntil = 'infinity', true) FROM pg_roles WHERE rolname = 'acctest_ufx_novalid'`, true),
			},
		},
	})
}

// Membership order follows the config, and a second grantor (PostgreSQL 16+) does not repeat a role.
func TestAccPostgresqlUser_rolesOrderAndGrantors(t *testing.T) {
	config := `
resource "postgresql_role" "a" {
  name = "acctest_ufx_ra"
}

resource "postgresql_role" "b" {
  name = "acctest_ufx_rb"
}

resource "postgresql_user" "test" {
  name  = "acctest_ufx_roles"
  roles = [postgresql_role.b.name, postgresql_role.a.name]
}`
	t.Cleanup(func() { acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_ufx_granter`) })
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_ufx_granter`)
					acctest.MustExec(t, `CREATE ROLE acctest_ufx_granter`)
					acctest.MustExec(t, `GRANT acctest_ufx_ra TO acctest_ufx_granter WITH ADMIN OPTION`)
					acctest.MustExec(t, `GRANT acctest_ufx_ra TO acctest_ufx_roles GRANTED BY acctest_ufx_granter`)
				},
				Config:           config,
				ConfigPlanChecks: emptyPlan(),
			},
			{
				PreConfig: func() {
					acctest.MustExec(t, `REVOKE acctest_ufx_ra FROM acctest_ufx_roles GRANTED BY acctest_ufx_granter`)
					acctest.MustExec(t, `DROP ROLE acctest_ufx_granter`)
				},
				Config: config,
			},
		},
	})
}

// An unset flag defaults to false in the plan, so a change made outside Terraform shows up instead of being applied silently.
func TestAccPostgresqlUser_unsetFlagsArePlanned(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_user" "test" {
  name = "acctest_ufx_flags"
}`,
			},
			{
				PreConfig: func() { acctest.MustExec(t, `ALTER ROLE acctest_ufx_flags CREATEDB`) },
				Config: `
resource "postgresql_user" "test" {
  name             = "acctest_ufx_flags"
  connection_limit = 5
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue("postgresql_user.test", tfjsonpath.New("create_database"), knownvalue.Bool(false)),
					},
				},
			},
		},
	})
}

// A failed membership change must not leave the rename applied.
func TestAccPostgresqlUser_renameRollsBack(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_user" "test" {
  name = "acctest_ufx_rn_a"
}`,
			},
			{
				Config: `
resource "postgresql_user" "test" {
  name  = "acctest_ufx_rn_b"
  roles = ["acctest_no_such_role"]
}`,
				ExpectError: regexp.MustCompile(`acctest_no_such_role`),
			},
			{
				Config: `
resource "postgresql_user" "test" {
  name = "acctest_ufx_rn_a"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					roleExists("acctest_ufx_rn_a", true),
					roleExists("acctest_ufx_rn_b", false),
				),
			},
		},
	})
}

func TestAccPostgresqlUser_reassignOwnedOnDestroy(t *testing.T) {
	t.Cleanup(func() { acctest.MustExec(t, `DROP TABLE IF EXISTS public.acctest_ufx_owned`) })
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			if err := testAccCheckPostgresqlRoleDestroy(s); err != nil {
				return err
			}
			return hasPrivilege(`SELECT tableowner = 'postgres' FROM pg_tables WHERE tablename = 'acctest_ufx_owned'`, true)(s)
		},
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_user" "test" {
  name              = "acctest_ufx_owner"
  reassign_owned_to = "postgres"
}`,
				Check: func(_ *terraform.State) error {
					acctest.MustExec(t, `CREATE TABLE public.acctest_ufx_owned (id int)`)
					acctest.MustExec(t, `ALTER TABLE public.acctest_ufx_owned OWNER TO acctest_ufx_owner`)
					return nil
				},
			},
		},
	})
}

func TestAccPostgresqlRole_reassignOwnedOnDestroy(t *testing.T) {
	t.Cleanup(func() { acctest.MustExec(t, `DROP TABLE IF EXISTS public.acctest_rfx_owned`) })
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_role" "test" {
  name              = "acctest_rfx_owner"
  reassign_owned_to = "postgres"
}`,
				Check: func(_ *terraform.State) error {
					acctest.MustExec(t, `CREATE TABLE public.acctest_rfx_owned (id int)`)
					acctest.MustExec(t, `ALTER TABLE public.acctest_rfx_owned OWNER TO acctest_rfx_owner`)
					return nil
				},
			},
		},
	})
}

// An administrator with CREATEROLE but not SUPERUSER (managed services) can create and alter users and roles.
func TestAccPostgresqlRoleAndUser_nonSuperuserAdmin(t *testing.T) {
	acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_rfx_admin`)
	acctest.MustExec(t, `CREATE ROLE acctest_rfx_admin LOGIN PASSWORD 'admin' CREATEROLE`)
	t.Cleanup(func() { acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_rfx_admin`) })
	config := func(limit int) string {
		return fmt.Sprintf(`
provider "postgresql" {
  username = "acctest_rfx_admin"
  password = "admin"
}

resource "postgresql_role" "test" {
  name             = "acctest_rfx_managed_role"
  connection_limit = %[1]d
}

resource "postgresql_user" "test" {
  name             = "acctest_rfx_managed_user"
  password         = "secret"
  connection_limit = %[1]d
}`, limit)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{Config: config(3)},
			{Config: config(4)},
		},
	})
}

// Inline privileges are read back, so a revoke made outside Terraform is drift.
func TestAccPostgresqlRole_privilegeDrift(t *testing.T) {
	grantFixture(t, "acctest_rfx_priv", `CREATE TABLE acctest_rfx_priv.t (id int)`)
	config := `
resource "postgresql_role" "test" {
  name = "acctest_rfx_priv_role"

  privilege {
    object_type = "table"
    schema      = "acctest_rfx_priv"
    objects     = ["t"]
    privileges  = ["select", "INSERT"]
  }
}`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlRoleDestroy,
		Steps: []resource.TestStep{
			{Config: config},
			{Config: config, ConfigPlanChecks: emptyPlan()},
			{
				PreConfig:        func() { acctest.MustExec(t, `REVOKE INSERT ON acctest_rfx_priv.t FROM acctest_rfx_priv_role`) },
				Config:           config,
				ConfigPlanChecks: expectUpdate("postgresql_role.test"),
				Check:            hasPrivilege(`SELECT has_table_privilege('acctest_rfx_priv_role', 'acctest_rfx_priv.t', 'INSERT')`, true),
			},
		},
	})
}

// Inline privileges on a schema run in the database the block names.
func TestAccPostgresqlRole_privilegeInAnotherDatabase(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_database" "other" {
  name = "acctest_rfx_otherdb"
}

resource "postgresql_schema" "app" {
  name     = "app"
  database = postgresql_database.other.name
}

resource "postgresql_role" "test" {
  name = "acctest_rfx_otherdb_role"

  privilege {
    object_type = "schema"
    database    = postgresql_database.other.name
    schema      = postgresql_schema.app.name
    privileges  = ["USAGE"]
  }
}`,
				Check: func(_ *terraform.State) error {
					db, err := acctest.OpenDatabase("acctest_rfx_otherdb")
					if err != nil {
						return err
					}
					defer db.Close() //nolint:errcheck
					var ok bool
					if err := db.QueryRow(`SELECT has_schema_privilege('acctest_rfx_otherdb_role', 'app', 'USAGE')`).Scan(&ok); err != nil {
						return err
					}
					if !ok {
						return fmt.Errorf("USAGE on app was not granted in acctest_rfx_otherdb")
					}
					return nil
				},
			},
		},
	})
}
