//go:build integration

// Tests for postgresql_roles data source.
package datasource_test

import (
	"fmt"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresqlRolesDataSource_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "postgresql_roles" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// At least postgres role exists
					resource.TestCheckResourceAttrSet("data.postgresql_roles.test", "roles.#"),
				),
			},
		},
	})
}

func TestAccPostgresqlRolesDataSource_withPattern(t *testing.T) {
	rName := "acctest_roles_ds_pat"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
				resource "postgresql_role" "test" { name = %q }
				data "postgresql_roles" "test" {
					like_pattern = "acctest_roles_ds_%%"
					depends_on   = [postgresql_role.test]
				}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.postgresql_roles.test", "roles.#"),
				),
			},
		},
	})
}

func TestAccPostgresqlRolesDataSource_loginOnly(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				resource "postgresql_role" "login_role" {
					name  = "acctest_roles_ds_login"
					login = true
				}
				resource "postgresql_role" "nologin_role" {
					name  = "acctest_roles_ds_nologin"
					login = false
				}
				data "postgresql_roles" "test" {
					like_pattern = "acctest_roles_ds_%"
					login_only   = true
					depends_on   = [postgresql_role.login_role, postgresql_role.nologin_role]
				}`,
				Check: resource.TestCheckResourceAttrSet("data.postgresql_roles.test", "roles.#"),
			},
		},
	})
}

func TestAccPostgresqlRolesDataSource_notLikePattern(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				data "postgresql_roles" "test" {
					not_like_pattern = "pg_%"
				}`,
				Check: resource.TestCheckResourceAttrSet("data.postgresql_roles.test", "roles.#"),
			},
		},
	})
}
