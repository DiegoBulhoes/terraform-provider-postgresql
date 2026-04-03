//go:build integration

// Tests for postgresql_role data source.
package datasource_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresqlRoleDataSource_basic(t *testing.T) {
	rName := "acctest_role_ds"
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

data "postgresql_role" "test" {
  name = postgresql_role.test.name
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "login", "false"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "superuser", "false"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "create_database", "false"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "create_role", "false"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "replication", "false"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "connection_limit", "-1"),
					resource.TestCheckResourceAttrSet("data.postgresql_role.test", "oid"),
				),
			},
		},
	})
}

func TestAccPostgresqlRoleDataSource_withMembership(t *testing.T) {
	rName := "acctest_role_ds_member"
	parentRole := "acctest_role_ds_parent"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "parent" {
  name = %q
}

resource "postgresql_user" "test" {
  name     = %q
  password = "testpass"
  roles    = [postgresql_role.parent.name]
}

data "postgresql_role" "test" {
  name = postgresql_user.test.name
}
`, parentRole, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "login", "true"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "roles.0", parentRole),
				),
			},
		},
	})
}

func TestAccPostgresqlRoleDataSource_fullAttributes(t *testing.T) {
	rName := "acctest_role_ds_full"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_user" "test" {
  name             = %q
  password         = "testpass"
  create_database  = true
  create_role      = true
  connection_limit = 5
  valid_until      = "2099-06-15T00:00:00Z"
}

data "postgresql_role" "test" {
  name = postgresql_user.test.name
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "login", "true"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "create_database", "true"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "create_role", "true"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "connection_limit", "5"),
					resource.TestCheckResourceAttrSet("data.postgresql_role.test", "valid_until"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "roles.#", "0"),
				),
			},
		},
	})
}

func TestAccPostgresqlRoleDataSource_noLogin(t *testing.T) {
	rName := "acctest_role_ds_nologin"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "test" {
  name = %q
}

data "postgresql_role" "test" {
  name = postgresql_role.test.name
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_role.test", "name", rName),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "login", "false"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "superuser", "false"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "create_database", "false"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "create_role", "false"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "replication", "false"),
					resource.TestCheckResourceAttr("data.postgresql_role.test", "connection_limit", "-1"),
				),
			},
		},
	})
}

func TestAccPostgresqlRoleDataSource_nonExistent(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_role" "test" {
					name = "nonexistent_role_12345"
				}`,
				ExpectError: regexp.MustCompile(`not found|does not exist|Error reading role`),
			},
		},
	})
}

func TestAccPostgresqlRoleDataSource_exampleAdmin(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_role" "admin" {
  name = "postgres"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_role.admin", "name", "postgres"),
					resource.TestCheckResourceAttr("data.postgresql_role.admin", "superuser", "true"),
					resource.TestCheckResourceAttr("data.postgresql_role.admin", "login", "true"),
					resource.TestCheckResourceAttrSet("data.postgresql_role.admin", "oid"),
				),
			},
		},
	})
}
