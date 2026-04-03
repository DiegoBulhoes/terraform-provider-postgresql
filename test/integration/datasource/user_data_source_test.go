//go:build integration

// Tests for postgresql_user data source.
package datasource_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresqlUserDataSource_basic(t *testing.T) {
	rName := "acctest_user_ds"
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_user" "test" {
  name  = %q
}

data "postgresql_user" "test" {
  name = postgresql_user.test.name
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_user.test", "name", rName),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "superuser", "false"),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "create_database", "false"),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "create_role", "false"),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "replication", "false"),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "connection_limit", "-1"),
					resource.TestCheckResourceAttrSet("data.postgresql_user.test", "oid"),
				),
			},
		},
	})
}

func TestAccPostgresqlUserDataSource_withMembership(t *testing.T) {
	rName := "acctest_user_ds_member"
	parentRole := "acctest_user_ds_parent"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_role" "parent" {
  name = %q
}

resource "postgresql_user" "test" {
  name  = %q
  roles = [postgresql_role.parent.name]
}

data "postgresql_user" "test" {
  name = postgresql_user.test.name
}
`, parentRole, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_user.test", "name", rName),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "roles.0", parentRole),
				),
			},
		},
	})
}

func TestAccPostgresqlUserDataSource_fullAttributes(t *testing.T) {
	rName := "acctest_user_ds_full"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_user" "test" {
  name             = %q
  create_database  = true
  create_role      = true
  connection_limit = 5
  valid_until      = "2099-06-15T00:00:00Z"
}

data "postgresql_user" "test" {
  name = postgresql_user.test.name
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_user.test", "name", rName),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "create_database", "true"),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "create_role", "true"),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "connection_limit", "5"),
					resource.TestCheckResourceAttrSet("data.postgresql_user.test", "valid_until"),
					resource.TestCheckResourceAttr("data.postgresql_user.test", "roles.#", "0"),
				),
			},
		},
	})
}

func TestAccPostgresqlUserDataSource_nonExistent(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_user" "test" {
					name = "nonexistent_user_12345"
				}`,
				ExpectError: regexp.MustCompile(`not found|does not exist|Error reading user`),
			},
		},
	})
}

// Example-based test: look up the built-in "postgres" superuser via the user data source.

func TestAccPostgresqlUserDataSource_exampleAdmin(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_user" "admin" {
  name = "postgres"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_user.admin", "name", "postgres"),
					resource.TestCheckResourceAttr("data.postgresql_user.admin", "superuser", "true"),
					resource.TestCheckResourceAttrSet("data.postgresql_user.admin", "oid"),
				),
			},
		},
	})
}
