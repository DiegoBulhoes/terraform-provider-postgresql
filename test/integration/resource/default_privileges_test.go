//go:build integration

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

// dpFixture creates owner and grantee roles and a schema the owner can create in.
// The roles live outside Terraform, so dropping them also drops their default privileges.
func dpFixture(t *testing.T, prefix string, statements ...string) {
	t.Helper()
	owner, grantee, schema := prefix+"_owner", prefix+"_role", prefix
	drop := func() {
		acctest.MustExec(t, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schema))
		for _, r := range []string{owner, grantee} {
			acctest.MustExec(t, fmt.Sprintf(`DO $$ BEGIN IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%[1]s') THEN
				DROP OWNED BY %[1]s; DROP ROLE %[1]s; END IF; END $$`, r))
		}
	}
	drop()
	acctest.MustExec(t, fmt.Sprintf(`CREATE ROLE %s`, owner))
	acctest.MustExec(t, fmt.Sprintf(`CREATE ROLE %s`, grantee))
	acctest.MustExec(t, fmt.Sprintf(`CREATE SCHEMA %s AUTHORIZATION %s`, schema, owner))
	for _, s := range statements {
		acctest.MustExec(t, s)
	}
	t.Cleanup(drop)
}

func dpConfig(body string) string {
	return fmt.Sprintf(`
resource "postgresql_default_privileges" "test" {
%s
}
`, body)
}

// futureObject creates an object as owner in a transaction it rolls back, and
// checks a privilege on it: the object must get the privilege from the defaults.
func futureObject(database, owner, create, check string, want bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		db, err := acctest.OpenDatabase(database)
		if err != nil {
			return err
		}
		defer db.Close() //nolint:errcheck
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback() //nolint:errcheck
		if _, err := tx.Exec(fmt.Sprintf(`SET LOCAL ROLE %s`, owner)); err != nil {
			return err
		}
		if _, err := tx.Exec(create); err != nil {
			return fmt.Errorf("%s: %w", create, err)
		}
		var got bool
		if err := tx.QueryRow(check).Scan(&got); err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("%s = %v, want %v", check, got, want)
		}
		return nil
	}
}

func testAccCheckDefaultPrivilegesDestroy(owner string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		db, err := acctest.GetDB()
		if err != nil {
			return err
		}
		var n int
		err = db.QueryRow(`SELECT count(*) FROM pg_default_acl d JOIN pg_roles r ON r.oid = d.defaclrole
			WHERE r.rolname = $1`, owner).Scan(&n)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if n != 0 {
			return fmt.Errorf("%d default privilege entries of %s remain", n, owner)
		}
		return nil
	}
}

func TestAccPostgresqlDefaultPrivileges_tablesInSchema(t *testing.T) {
	dpFixture(t, "acctest_dp_tbl")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckDefaultPrivilegesDestroy("acctest_dp_tbl_owner"),
		Steps: []resource.TestStep{
			{
				Config: dpConfig(`
  owner       = "acctest_dp_tbl_owner"
  role        = "acctest_dp_tbl_role"
  schema      = "acctest_dp_tbl"
  object_type = "table"
  privileges  = ["SELECT", "INSERT"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "id",
						"acctest_dp_tbl_owner/acctest_dp_tbl_role/table//acctest_dp_tbl"),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "2"),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "with_grant_option", "false"),
					futureObject("postgres", "acctest_dp_tbl_owner", `CREATE TABLE acctest_dp_tbl.t (id int)`,
						`SELECT has_table_privilege('acctest_dp_tbl_role', 'acctest_dp_tbl.t', 'INSERT')`, true),
					futureObject("postgres", "acctest_dp_tbl_owner", `CREATE TABLE acctest_dp_tbl.t (id int)`,
						`SELECT has_table_privilege('acctest_dp_tbl_role', 'acctest_dp_tbl.t', 'DELETE')`, false),
				),
			},
			{
				// Update: the new set replaces the old one.
				Config: dpConfig(`
  owner             = "acctest_dp_tbl_owner"
  role              = "acctest_dp_tbl_role"
  schema            = "acctest_dp_tbl"
  object_type       = "table"
  privileges        = ["DELETE"]
  with_grant_option = true`),
				ConfigPlanChecks: expectUpdate("postgresql_default_privileges.test"),
				Check: resource.ComposeAggregateTestCheckFunc(
					futureObject("postgres", "acctest_dp_tbl_owner", `CREATE TABLE acctest_dp_tbl.t (id int)`,
						`SELECT has_table_privilege('acctest_dp_tbl_role', 'acctest_dp_tbl.t', 'DELETE WITH GRANT OPTION')`, true),
					futureObject("postgres", "acctest_dp_tbl_owner", `CREATE TABLE acctest_dp_tbl.t (id int)`,
						`SELECT has_table_privilege('acctest_dp_tbl_role', 'acctest_dp_tbl.t', 'SELECT')`, false),
				),
			},
		},
	})
}

// Without schema the owner's global defaults apply in every schema.
func TestAccPostgresqlDefaultPrivileges_globalTables(t *testing.T) {
	dpFixture(t, "acctest_dp_glb")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckDefaultPrivilegesDestroy("acctest_dp_glb_owner"),
		Steps: []resource.TestStep{
			{
				Config: dpConfig(`
  owner       = "acctest_dp_glb_owner"
  role        = "acctest_dp_glb_role"
  object_type = "table"
  privileges  = ["SELECT"]`),
				Check: futureObject("postgres", "acctest_dp_glb_owner", `CREATE TABLE acctest_dp_glb.t (id int)`,
					`SELECT has_table_privilege('acctest_dp_glb_role', 'acctest_dp_glb.t', 'SELECT')`, true),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_sequencesAndFunctions(t *testing.T) {
	// PUBLIC executes new functions by default; without that, only the grantee does.
	dpFixture(t, "acctest_dp_seqfn",
		`ALTER DEFAULT PRIVILEGES FOR ROLE acctest_dp_seqfn_owner REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_default_privileges" "seq" {
  owner       = "acctest_dp_seqfn_owner"
  role        = "acctest_dp_seqfn_role"
  schema      = "acctest_dp_seqfn"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}

resource "postgresql_default_privileges" "fn" {
  owner       = "acctest_dp_seqfn_owner"
  role        = "acctest_dp_seqfn_role"
  schema      = "acctest_dp_seqfn"
  object_type = "function"
  privileges  = ["EXECUTE"]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					futureObject("postgres", "acctest_dp_seqfn_owner", `CREATE SEQUENCE acctest_dp_seqfn.s`,
						`SELECT has_sequence_privilege('acctest_dp_seqfn_role', 'acctest_dp_seqfn.s', 'USAGE')`, true),
					futureObject("postgres", "acctest_dp_seqfn_owner",
						`CREATE FUNCTION acctest_dp_seqfn.f() RETURNS int LANGUAGE sql AS 'SELECT 1'`,
						`SELECT has_function_privilege('acctest_dp_seqfn_role', 'acctest_dp_seqfn.f()', 'EXECUTE')`, true),
					futureObject("postgres", "acctest_dp_seqfn_owner",
						`CREATE FUNCTION acctest_dp_seqfn.f() RETURNS int LANGUAGE sql AS 'SELECT 1'`,
						`SELECT has_function_privilege('pg_monitor', 'acctest_dp_seqfn.f()', 'EXECUTE')`, false),
				),
			},
		},
	})
}

// Lower case and ALL converge: the second plan is empty.
func TestAccPostgresqlDefaultPrivileges_spellingConverges(t *testing.T) {
	for name, c := range map[string]struct{ objectType, privileges string }{
		"lowercase": {"table", `["select", "update"]`},
		"all":       {"table", `["ALL"]`},
		"type":      {"type", `["all privileges"]`},
	} {
		t.Run(name, func(t *testing.T) {
			prefix := "acctest_dp_sp_" + name
			dpFixture(t, prefix)
			config := dpConfig(fmt.Sprintf(`
  owner       = "%[1]s_owner"
  role        = "%[1]s_role"
  schema      = "%[1]s"
  object_type = "%[2]s"
  privileges  = %[3]s`, prefix, c.objectType, c.privileges))
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testProviderFactories,
				Steps: []resource.TestStep{
					{Config: config},
					{Config: config, ConfigPlanChecks: emptyPlan()},
				},
			})
		})
	}
}

// A default revoked outside Terraform shows up as an update and is granted again.
func TestAccPostgresqlDefaultPrivileges_drift(t *testing.T) {
	dpFixture(t, "acctest_dp_drift")
	config := dpConfig(`
  owner       = "acctest_dp_drift_owner"
  role        = "acctest_dp_drift_role"
  schema      = "acctest_dp_drift"
  object_type = "table"
  privileges  = ["SELECT", "INSERT"]`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					acctest.MustExec(t, `ALTER DEFAULT PRIVILEGES FOR ROLE acctest_dp_drift_owner IN SCHEMA acctest_dp_drift
						REVOKE INSERT ON TABLES FROM acctest_dp_drift_role`)
				},
				Config:           config,
				ConfigPlanChecks: expectUpdate("postgresql_default_privileges.test"),
				Check: futureObject("postgres", "acctest_dp_drift_owner", `CREATE TABLE acctest_dp_drift.t (id int)`,
					`SELECT has_table_privilege('acctest_dp_drift_role', 'acctest_dp_drift.t', 'INSERT')`, true),
			},
			{
				PreConfig: func() {
					acctest.MustExec(t, `ALTER DEFAULT PRIVILEGES FOR ROLE acctest_dp_drift_owner IN SCHEMA acctest_dp_drift
						REVOKE ALL ON TABLES FROM acctest_dp_drift_role`)
				},
				Config:           config,
				ConfigPlanChecks: expectUpdate("postgresql_default_privileges.test"),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_import(t *testing.T) {
	dpFixture(t, "acctest_dp_imp")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: dpConfig(`
  owner             = "acctest_dp_imp_owner"
  role              = "acctest_dp_imp_role"
  database          = "postgres"
  schema            = "acctest_dp_imp"
  object_type       = "table"
  privileges        = ["SELECT", "UPDATE"]
  with_grant_option = true`),
			},
			{
				ResourceName:      "postgresql_default_privileges.test",
				ImportState:       true,
				ImportStateId:     "acctest_dp_imp_owner/acctest_dp_imp_role/TABLE/postgres/acctest_dp_imp",
				ImportStateVerify: true,
			},
			{
				ResourceName:  "postgresql_default_privileges.test",
				ImportState:   true,
				ImportStateId: "acctest_dp_imp_owner/acctest_dp_imp_role/sequence/postgres/acctest_dp_imp",
				ExpectError:   regexp.MustCompile(`(?i)non-existent|cannot import`),
			},
		},
	})
}

// The defaults are set in the database the resource names.
func TestAccPostgresqlDefaultPrivileges_inAnotherDatabase(t *testing.T) {
	dpFixture(t, "acctest_dp_odb")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_database" "other" {
  name = "acctest_dp_otherdb"
}

resource "postgresql_schema" "app" {
  name     = "app"
  database = postgresql_database.other.name
  owner    = "acctest_dp_odb_owner"
}

resource "postgresql_default_privileges" "test" {
  owner       = "acctest_dp_odb_owner"
  role        = "acctest_dp_odb_role"
  database    = postgresql_database.other.name
  schema      = postgresql_schema.app.name
  object_type = "table"
  privileges  = ["SELECT"]
}`,
				Check: futureObject("acctest_dp_otherdb", "acctest_dp_odb_owner", `CREATE TABLE app.t (id int)`,
					`SELECT has_table_privilege('acctest_dp_odb_role', 'app.t', 'SELECT')`, true),
			},
		},
	})
}

// An invalid privilege fails before any SQL runs, and the old defaults stay.
func TestAccPostgresqlDefaultPrivileges_invalidPrivilege(t *testing.T) {
	dpFixture(t, "acctest_dp_inv")
	ok := dpConfig(`
  owner       = "acctest_dp_inv_owner"
  role        = "acctest_dp_inv_role"
  schema      = "acctest_dp_inv"
  object_type = "table"
  privileges  = ["SELECT"]`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{Config: ok},
			{
				Config: dpConfig(`
  owner       = "acctest_dp_inv_owner"
  role        = "acctest_dp_inv_role"
  schema      = "acctest_dp_inv"
  object_type = "table"
  privileges  = ["SELECT", "SELEC"]`),
				ExpectError: regexp.MustCompile(`(?i)invalid privilege`),
			},
			{
				// EXECUTE is a valid keyword, but not for tables: the server rejects it in the transaction.
				Config: dpConfig(`
  owner       = "acctest_dp_inv_owner"
  role        = "acctest_dp_inv_role"
  schema      = "acctest_dp_inv"
  object_type = "table"
  privileges  = ["SELECT", "EXECUTE"]`),
				ExpectError: regexp.MustCompile(`(?i)invalid privilege type EXECUTE`),
			},
			{
				Config:           ok,
				ConfigPlanChecks: emptyPlan(),
				Check: futureObject("postgres", "acctest_dp_inv_owner", `CREATE TABLE acctest_dp_inv.t (id int)`,
					`SELECT has_table_privilege('acctest_dp_inv_role', 'acctest_dp_inv.t', 'SELECT')`, true),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_schemaObjectTypeWithSchemaFailsAtPlan(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: dpConfig(`
  owner       = "postgres"
  role        = "postgres"
  schema      = "public"
  object_type = "schema"
  privileges  = ["USAGE"]`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)object_type schema.*without schema|schema.*object_type`),
			},
		},
	})
}

// Schema defaults apply to schemas the owner creates later.
func TestAccPostgresqlDefaultPrivileges_schemas(t *testing.T) {
	dpFixture(t, "acctest_dp_nsp", `GRANT CREATE ON DATABASE postgres TO acctest_dp_nsp_owner`)
	t.Cleanup(func() { acctest.MustExec(t, `DROP SCHEMA IF EXISTS acctest_dp_nsp_new`) })
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: dpConfig(`
  owner       = "acctest_dp_nsp_owner"
  role        = "acctest_dp_nsp_role"
  object_type = "schema"
  privileges  = ["usage"]`),
				Check: futureObject("postgres", "acctest_dp_nsp_owner", `CREATE SCHEMA acctest_dp_nsp_new`,
					`SELECT has_schema_privilege('acctest_dp_nsp_role', 'acctest_dp_nsp_new', 'USAGE')`, true),
			},
		},
	})
}
