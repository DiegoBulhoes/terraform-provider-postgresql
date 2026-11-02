//go:build integration

package resource_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// grantFixture creates a schema owned outside Terraform and drops it after the test.
func grantFixture(t *testing.T, schema string, statements ...string) {
	t.Helper()
	acctest.MustExec(t, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schema))
	acctest.MustExec(t, fmt.Sprintf(`CREATE SCHEMA %s`, schema))
	for _, s := range statements {
		acctest.MustExec(t, s)
	}
	t.Cleanup(func() { acctest.MustExec(t, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schema)) })
}

func grantConfig(role, body string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

resource "postgresql_grant" "test" {
  role = postgresql_role.test.name
%s
}
`, role, body)
}

func emptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

func expectUpdate(name string) resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
	}}
}

func hasPrivilege(query string, want bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		db, err := acctest.GetDB()
		if err != nil {
			return err
		}
		var got bool
		if err := db.QueryRow(query).Scan(&got); err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("%s = %v, want %v", query, got, want)
		}
		return nil
	}
}

// A failed update must leave the existing privileges in place.
func TestAccPostgresqlGrant_failedUpdateKeepsPrivileges(t *testing.T) {
	grantFixture(t, "acctest_gfx_upd", `CREATE TABLE acctest_gfx_upd.t (id int)`)
	ok := grantConfig("acctest_gfx_upd_role", `
  object_type = "table"
  schema      = "acctest_gfx_upd"
  objects     = ["t"]
  privileges  = ["SELECT"]`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{Config: ok},
			{
				Config: grantConfig("acctest_gfx_upd_role", `
  object_type = "table"
  schema      = "acctest_gfx_upd"
  objects     = ["t"]
  privileges  = ["SELECT", "SELEC"]`),
				ExpectError: regexp.MustCompile(`(?i)invalid privilege`),
			},
			{
				// EXECUTE is a valid keyword, but not for a table: the server rejects the GRANT.
				Config: grantConfig("acctest_gfx_upd_role", `
  object_type = "table"
  schema      = "acctest_gfx_upd"
  objects     = ["t"]
  privileges  = ["SELECT", "EXECUTE"]`),
				ExpectError: regexp.MustCompile(`(?i)invalid privilege type EXECUTE`),
			},
			{
				Config:           ok,
				ConfigPlanChecks: emptyPlan(),
				Check:            hasPrivilege(`SELECT has_table_privilege('acctest_gfx_upd_role', 'acctest_gfx_upd.t', 'SELECT')`, true),
			},
		},
	})
}

// Lower case, ALL and TEMP converge: the second plan is empty.
func TestAccPostgresqlGrant_privilegeSpellingConverges(t *testing.T) {
	grantFixture(t, "acctest_gfx_spell", `CREATE TABLE acctest_gfx_spell.t (id int)`)
	for name, body := range map[string]string{
		"lowercase": `
  object_type = "table"
  schema      = "acctest_gfx_spell"
  objects     = ["t"]
  privileges  = ["select", "insert"]`,
		"all": `
  object_type = "table"
  schema      = "acctest_gfx_spell"
  objects     = ["t"]
  privileges  = ["ALL"]`,
		"temp": `
  object_type = "database"
  database    = "postgres"
  privileges  = ["TEMP", "connect"]`,
	} {
		t.Run(name, func(t *testing.T) {
			config := grantConfig("acctest_gfx_spell_"+name, body)
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testProviderFactories,
				CheckDestroy:             testAccCheckPostgresqlGrantDestroy,
				Steps: []resource.TestStep{
					{Config: config},
					{Config: config, ConfigPlanChecks: emptyPlan()},
				},
			})
		})
	}
}

// A revoke on any listed object, not only the first, shows up as drift.
func TestAccPostgresqlGrant_driftOnEveryObject(t *testing.T) {
	grantFixture(t, "acctest_gfx_multi",
		`CREATE TABLE acctest_gfx_multi.t1 (id int)`,
		`CREATE TABLE acctest_gfx_multi.t2 (id int)`)
	config := grantConfig("acctest_gfx_multi_role", `
  object_type = "table"
  schema      = "acctest_gfx_multi"
  objects     = ["t1", "t2"]
  privileges  = ["SELECT"]`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig:        func() { acctest.MustExec(t, `REVOKE SELECT ON acctest_gfx_multi.t2 FROM acctest_gfx_multi_role`) },
				Config:           config,
				ConfigPlanChecks: expectUpdate("postgresql_grant.test"),
				Check:            hasPrivilege(`SELECT has_table_privilege('acctest_gfx_multi_role', 'acctest_gfx_multi.t2', 'SELECT')`, true),
			},
		},
	})
}

// A grant on all tables notices a table created later without the privilege.
func TestAccPostgresqlGrant_driftOnAllObjects(t *testing.T) {
	grantFixture(t, "acctest_gfx_all", `CREATE TABLE acctest_gfx_all.t1 (id int)`)
	config := grantConfig("acctest_gfx_all_role", `
  object_type = "table"
  schema      = "acctest_gfx_all"
  privileges  = ["SELECT"]`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig:        func() { acctest.MustExec(t, `CREATE TABLE acctest_gfx_all.t2 (id int)`) },
				Config:           config,
				ConfigPlanChecks: expectUpdate("postgresql_grant.test"),
				Check:            hasPrivilege(`SELECT has_table_privilege('acctest_gfx_all_role', 'acctest_gfx_all.t2', 'SELECT')`, true),
			},
		},
	})
}

// A function is named alone (every overload) or with its argument types (one overload).
func TestAccPostgresqlGrant_functionSignatures(t *testing.T) {
	grantFixture(t, "acctest_gfx_fn",
		`CREATE FUNCTION acctest_gfx_fn.f(integer) RETURNS integer LANGUAGE sql AS 'SELECT 1'`,
		`CREATE FUNCTION acctest_gfx_fn.f(text) RETURNS integer LANGUAGE sql AS 'SELECT 2'`,
		`CREATE FUNCTION acctest_gfx_fn.g(integer) RETURNS integer LANGUAGE sql AS 'SELECT 3'`,
		`CREATE FUNCTION acctest_gfx_fn.g(text) RETURNS integer LANGUAGE sql AS 'SELECT 4'`,
		`REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA acctest_gfx_fn FROM PUBLIC`)
	config := grantConfig("acctest_gfx_fn_role", `
  object_type = "function"
  schema      = "acctest_gfx_fn"
  objects     = ["f(integer)", "g"]
  privileges  = ["EXECUTE"]`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					hasPrivilege(`SELECT has_function_privilege('acctest_gfx_fn_role', 'acctest_gfx_fn.f(integer)', 'EXECUTE')`, true),
					hasPrivilege(`SELECT has_function_privilege('acctest_gfx_fn_role', 'acctest_gfx_fn.f(text)', 'EXECUTE')`, false),
					hasPrivilege(`SELECT has_function_privilege('acctest_gfx_fn_role', 'acctest_gfx_fn.g(integer)', 'EXECUTE')`, true),
					hasPrivilege(`SELECT has_function_privilege('acctest_gfx_fn_role', 'acctest_gfx_fn.g(text)', 'EXECUTE')`, true),
				),
			},
			{Config: config, ConfigPlanChecks: emptyPlan()},
		},
	})
}

// Import accepts any case for the object type and restores the object list.
func TestAccPostgresqlGrant_importSpecificObjects(t *testing.T) {
	grantFixture(t, "acctest_gfx_imp",
		`CREATE TABLE acctest_gfx_imp.t1 (id int)`,
		`CREATE TABLE acctest_gfx_imp.t2 (id int)`)
	config := grantConfig("acctest_gfx_imp_role", `
  object_type = "table"
  database    = "postgres"
  schema      = "acctest_gfx_imp"
  objects     = ["t1", "t2"]
  privileges  = ["SELECT", "UPDATE"]`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				// The ID is the import ID, with "/" where the old one used "_", which names also contain.
				Check: resource.TestCheckResourceAttr("postgresql_grant.test", "id", "acctest_gfx_imp_role/table/postgres/acctest_gfx_imp/t1,t2"),
			},
			{
				ResourceName:                         "postgresql_grant.test",
				ImportState:                          true,
				ImportStateId:                        "acctest_gfx_imp_role/TABLE/postgres/acctest_gfx_imp/t1,t2",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "role",
			},
		},
	})
}

func TestAccPostgresqlGrant_missingSchemaFailsAtPlan(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: grantConfig("acctest_gfx_val_role", `
  object_type = "table"
  privileges  = ["SELECT"]`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)schema.*required`),
			},
		},
	})
}

// Schema and table grants run in the database the grant names.
func TestAccPostgresqlGrant_inAnotherDatabase(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_database" "other" {
  name = "acctest_gfx_otherdb"
}

resource "postgresql_schema" "app" {
  name     = "app"
  database = postgresql_database.other.name
}

resource "postgresql_role" "test" {
  name = "acctest_gfx_otherdb_role"
}

resource "postgresql_grant" "test" {
  role        = postgresql_role.test.name
  object_type = "schema"
  database    = postgresql_database.other.name
  schema      = postgresql_schema.app.name
  privileges  = ["USAGE"]
}`,
				Check: func(_ *terraform.State) error {
					db, err := acctest.OpenDatabase("acctest_gfx_otherdb")
					if err != nil {
						return err
					}
					defer db.Close() //nolint:errcheck
					var ok bool
					if err := db.QueryRow(`SELECT has_schema_privilege('acctest_gfx_otherdb_role', 'app', 'USAGE')`).Scan(&ok); err != nil {
						return err
					}
					if !ok {
						return fmt.Errorf("USAGE on app was not granted in acctest_gfx_otherdb")
					}
					return nil
				},
			},
		},
	})
}
