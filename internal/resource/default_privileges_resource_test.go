//go:build integration

// Tests for postgresql_default_privileges resource.
package resource_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccPostgresqlDefaultPrivileges_basic(t *testing.T) {
	rOwner := "acctest_defpriv_owner"
	rGrantee := "acctest_defpriv_grantee"
	rDB := "acctest_defpriv_db"
	rSchema := "acctest_defpriv_schema"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDefaultPrivilegesDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDefaultPrivilegesConfig_basic(rOwner, rGrantee, rDB, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "owner", rOwner),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "role", rGrantee),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "database", rDB),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "schema", rSchema),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "object_type", "table"),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "2"),
					resource.TestCheckResourceAttrSet("postgresql_default_privileges.test", "id"),
				),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_sequence(t *testing.T) {
	rOwner := "acctest_defpriv_seq_owner"
	rGrantee := "acctest_defpriv_seq_grantee"
	rDB := "acctest_defpriv_seq_db"
	rSchema := "acctest_defpriv_seq_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDefaultPrivilegesDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDefaultPrivilegesConfig_sequence(rOwner, rGrantee, rDB, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "object_type", "sequence"),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "2"),
				),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_function(t *testing.T) {
	rOwner := "acctest_defpriv_fn_owner"
	rGrantee := "acctest_defpriv_fn_grantee"
	rDB := "acctest_defpriv_fn_db"
	rSchema := "acctest_defpriv_fn_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDefaultPrivilegesDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDefaultPrivilegesConfig_function(rOwner, rGrantee, rDB, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "object_type", "function"),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "1"),
				),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_type(t *testing.T) {
	rOwner := "acctest_defpriv_tp_owner"
	rGrantee := "acctest_defpriv_tp_grantee"
	rDB := "acctest_defpriv_tp_db"
	rSchema := "acctest_defpriv_tp_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDefaultPrivilegesDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDefaultPrivilegesConfig_type(rOwner, rGrantee, rDB, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "object_type", "type"),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "1"),
				),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_databaseWide(t *testing.T) {
	rOwner := "acctest_defpriv_dbw_owner"
	rGrantee := "acctest_defpriv_dbw_grantee"
	rDB := "acctest_defpriv_dbw_db"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDefaultPrivilegesDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDefaultPrivilegesConfig_databaseWide(rOwner, rGrantee, rDB),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "owner", rOwner),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "role", rGrantee),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "object_type", "table"),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "1"),
				),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_tableAllPrivileges(t *testing.T) {
	rOwner := "acctest_defpriv_ta_owner"
	rGrantee := "acctest_defpriv_ta_grantee"
	rDB := "acctest_defpriv_ta_db"
	rSchema := "acctest_defpriv_ta_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDefaultPrivilegesDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "owner" {
  name = %q
}

resource "postgresql_role" "grantee" {
  name = %q
}

resource "postgresql_database" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_default_privileges" "test" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.grantee.name
  database    = postgresql_database.test.name
  schema      = postgresql_schema.test.name
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"]
}
`, rOwner, rGrantee, rDB, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "7"),
				),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_update(t *testing.T) {
	rOwner := "acctest_defpriv_upd_owner"
	rGrantee := "acctest_defpriv_upd_grantee"
	rDB := "acctest_defpriv_upd_db"
	rSchema := "acctest_defpriv_upd_sch"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDefaultPrivilegesDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDefaultPrivilegesConfig_basic(rOwner, rGrantee, rDB, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "2"),
				),
			},
			{
				Config: testAccPostgresqlDefaultPrivilegesConfig_updated(rOwner, rGrantee, rDB, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "3"),
				),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_invalidObjectType(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				resource "postgresql_default_privileges" "test" {
					owner       = "postgres"
					role        = "postgres"
					database    = "postgres"
					object_type = "invalid"
					privileges  = ["SELECT"]
				}`,
				ExpectError: regexp.MustCompile(`must be one of`),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_emptyPrivileges(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				resource "postgresql_default_privileges" "test" {
					owner       = "postgres"
					role        = "postgres"
					database    = "postgres"
					object_type = "table"
					privileges  = []
				}`,
				ExpectError: regexp.MustCompile(`must contain at least 1`),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_import(t *testing.T) {
	rOwner := "acctest_defpriv_imp_owner"
	rGrantee := "acctest_defpriv_imp_grant"
	rDB := "acctest_defpriv_imp_db"
	rSchema := "acctest_defpriv_imp_sch"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDefaultPrivilegesDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDefaultPrivilegesConfig_basic(rOwner, rGrantee, rDB, rSchema),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "owner", rOwner),
					resource.TestCheckResourceAttr("postgresql_default_privileges.test", "object_type", "table"),
				),
			},
			{
				ResourceName:      "postgresql_default_privileges.test",
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s/%s/%s/%s/table", rOwner, rGrantee, rDB, rSchema),
				ImportStateVerify: false,
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_disappears(t *testing.T) {
	rOwner := "acctest_dp_disapp_own"
	rGrantee := "acctest_dp_disapp_grt"
	rDB := "acctest_dp_disapp_db"
	rSchema := "acctest_dp_disapp_sch"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDefaultPrivilegesDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPostgresqlDefaultPrivilegesConfig_basic(rOwner, rGrantee, rDB, rSchema),
				Check:  resource.TestCheckResourceAttrSet("postgresql_default_privileges.test", "id"),
			},
			{
				PreConfig: func() {
					db, _ := acctest.GetDB()
					db.Exec(fmt.Sprintf(
						`ALTER DEFAULT PRIVILEGES FOR ROLE "%s" IN SCHEMA "%s" REVOKE ALL ON TABLES FROM "%s"`,
						rOwner, rSchema, rGrantee,
					))
				},
				Config: testAccPostgresqlDefaultPrivilegesConfig_basic(rOwner, rGrantee, rDB, rSchema),
				Check:  resource.TestCheckResourceAttrSet("postgresql_default_privileges.test", "id"),
			},
		},
	})
}

func TestAccPostgresqlDefaultPrivileges_updatePrivileges(t *testing.T) {
	rOwner := "acctest_dp_upd_own"
	rGrantee := "acctest_dp_upd_grt"
	rDB := "acctest_dp_upd_db"
	rSchema := "acctest_dp_upd_sch"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDefaultPrivilegesDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "owner" { name = %q }
				resource "postgresql_role" "grantee" { name = %q }
				resource "postgresql_database" "test" { name = %q }
				resource "postgresql_schema" "test" { name = %q }
				resource "postgresql_default_privileges" "test" {
					owner       = postgresql_role.owner.name
					role        = postgresql_role.grantee.name
					database    = postgresql_database.test.name
					schema      = postgresql_schema.test.name
					object_type = "table"
					privileges  = ["SELECT"]
				}`, rOwner, rGrantee, rDB, rSchema),
				Check: resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "1"),
			},
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "owner" { name = %q }
				resource "postgresql_role" "grantee" { name = %q }
				resource "postgresql_database" "test" { name = %q }
				resource "postgresql_schema" "test" { name = %q }
				resource "postgresql_default_privileges" "test" {
					owner       = postgresql_role.owner.name
					role        = postgresql_role.grantee.name
					database    = postgresql_database.test.name
					schema      = postgresql_schema.test.name
					object_type = "table"
					privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
				}`, rOwner, rGrantee, rDB, rSchema),
				Check: resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "4"),
			},
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "owner" { name = %q }
				resource "postgresql_role" "grantee" { name = %q }
				resource "postgresql_database" "test" { name = %q }
				resource "postgresql_schema" "test" { name = %q }
				resource "postgresql_default_privileges" "test" {
					owner       = postgresql_role.owner.name
					role        = postgresql_role.grantee.name
					database    = postgresql_database.test.name
					schema      = postgresql_schema.test.name
					object_type = "table"
					privileges  = ["SELECT"]
				}`, rOwner, rGrantee, rDB, rSchema),
				Check: resource.TestCheckResourceAttr("postgresql_default_privileges.test", "privileges.#", "1"),
			},
		},
	})
}

func testAccCheckPostgresqlDefaultPrivilegesDestroy(s *terraform.State) error {
	db, err := acctest.GetDB()
	if err != nil {
		return fmt.Errorf("error getting test database connection: %s", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "postgresql_default_privileges" {
			continue
		}

		owner := rs.Primary.Attributes["owner"]
		role := rs.Primary.Attributes["role"]

		// If the owner role no longer exists, default privileges are implicitly gone.
		var ownerExists int
		err := db.QueryRow("SELECT 1 FROM pg_roles WHERE rolname = $1", owner).Scan(&ownerExists)
		if err != nil {
			continue
		}

		// If the grantee role no longer exists, default privileges are implicitly gone.
		var roleExists int
		err = db.QueryRow("SELECT 1 FROM pg_roles WHERE rolname = $1", role).Scan(&roleExists)
		if err != nil {
			continue
		}

		objectType := rs.Primary.Attributes["object_type"]
		objTypeChars := map[string]string{
			"table": "r", "sequence": "S", "function": "f", "type": "T",
		}
		objTypeChar := objTypeChars[objectType]

		var count int
		err = db.QueryRow(`
			SELECT count(*)
			FROM (
				SELECT (aclexplode(defaclacl)).grantee
				FROM pg_default_acl da
				WHERE da.defaclrole = (SELECT oid FROM pg_roles WHERE rolname = $1)
				  AND da.defaclobjtype = $2
			) AS acl
			JOIN pg_roles ON acl.grantee = pg_roles.oid
			WHERE pg_roles.rolname = $3
		`, owner, objTypeChar, role).Scan(&count)
		if err != nil {
			continue
		}
		if count > 0 {
			return fmt.Errorf("default privileges still exist: owner %q still has %d default privileges for role %q on %s", owner, count, role, objectType)
		}
	}

	return nil
}

func testAccPostgresqlDefaultPrivilegesConfig_basic(owner, grantee, dbName, schemaName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "owner" {
  name = %q
}

resource "postgresql_role" "grantee" {
  name = %q
}

resource "postgresql_database" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_default_privileges" "test" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.grantee.name
  database    = postgresql_database.test.name
  schema      = postgresql_schema.test.name
  object_type = "table"
  privileges  = ["SELECT", "INSERT"]
}
`, owner, grantee, dbName, schemaName)
}

func testAccPostgresqlDefaultPrivilegesConfig_updated(owner, grantee, dbName, schemaName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "owner" {
  name = %q
}

resource "postgresql_role" "grantee" {
  name = %q
}

resource "postgresql_database" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_default_privileges" "test" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.grantee.name
  database    = postgresql_database.test.name
  schema      = postgresql_schema.test.name
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE"]
}
`, owner, grantee, dbName, schemaName)
}

func testAccPostgresqlDefaultPrivilegesConfig_sequence(owner, grantee, dbName, schemaName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "owner" {
  name = %q
}

resource "postgresql_role" "grantee" {
  name = %q
}

resource "postgresql_database" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_default_privileges" "test" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.grantee.name
  database    = postgresql_database.test.name
  schema      = postgresql_schema.test.name
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}
`, owner, grantee, dbName, schemaName)
}

func testAccPostgresqlDefaultPrivilegesConfig_function(owner, grantee, dbName, schemaName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "owner" {
  name = %q
}

resource "postgresql_role" "grantee" {
  name = %q
}

resource "postgresql_database" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_default_privileges" "test" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.grantee.name
  database    = postgresql_database.test.name
  schema      = postgresql_schema.test.name
  object_type = "function"
  privileges  = ["EXECUTE"]
}
`, owner, grantee, dbName, schemaName)
}

func testAccPostgresqlDefaultPrivilegesConfig_type(owner, grantee, dbName, schemaName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "owner" {
  name = %q
}

resource "postgresql_role" "grantee" {
  name = %q
}

resource "postgresql_database" "test" {
  name = %q
}

resource "postgresql_schema" "test" {
  name = %q
}

resource "postgresql_default_privileges" "test" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.grantee.name
  database    = postgresql_database.test.name
  schema      = postgresql_schema.test.name
  object_type = "type"
  privileges  = ["USAGE"]
}
`, owner, grantee, dbName, schemaName)
}

func testAccPostgresqlDefaultPrivilegesConfig_databaseWide(owner, grantee, dbName string) string {
	return fmt.Sprintf(`
resource "postgresql_role" "owner" {
  name = %q
}

resource "postgresql_role" "grantee" {
  name = %q
}

resource "postgresql_database" "test" {
  name = %q
}

resource "postgresql_default_privileges" "test" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.grantee.name
  database    = postgresql_database.test.name
  object_type = "table"
  privileges  = ["SELECT"]
}
`, owner, grantee, dbName)
}
