// Tests for provider configuration validation and full stack integration.
package provider

import (
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProvider_invalidSslmode(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				provider "postgresql" {
					sslmode = "invalid"
				}
				resource "postgresql_role" "test" {
					name = "acctest_sslmode_test"
				}`,
				ExpectError: regexp.MustCompile(`must be one of`),
			},
		},
	})
}

func TestAccProvider_invalidPort(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				provider "postgresql" {
					port = 0
				}
				resource "postgresql_role" "test" {
					name = "acctest_port_test"
				}`,
				ExpectError: regexp.MustCompile(`must be between 1 and 65535`),
			},
		},
	})
}

func TestAccPostgresql_fullStack(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				resource "postgresql_role" "owner" {
					name            = "acctest_fullstack_owner"
					login           = true
					create_database = true
				}

				resource "postgresql_role" "reader" {
					name  = "acctest_fullstack_reader"
					login = false
				}

				resource "postgresql_role" "app" {
					name     = "acctest_fullstack_app"
					login    = true
					password = "apppass"
					roles    = [postgresql_role.reader.name]
				}

				resource "postgresql_database" "app" {
					name  = "acctest_fullstack_db"
					owner = postgresql_role.owner.name
				}

				resource "postgresql_schema" "api" {
					name  = "api"
					owner = postgresql_role.owner.name
				}

				resource "postgresql_grant" "db_connect" {
					role        = postgresql_role.app.name
					object_type = "database"
					database    = postgresql_database.app.name
					privileges  = ["CONNECT"]
				}

				resource "postgresql_grant" "schema_usage" {
					role        = postgresql_role.app.name
					object_type = "schema"
					schema      = postgresql_schema.api.name
					privileges  = ["USAGE"]
				}

				resource "postgresql_default_privileges" "tables" {
					owner       = postgresql_role.owner.name
					role        = postgresql_role.reader.name
					database    = postgresql_database.app.name
					schema      = postgresql_schema.api.name
					object_type = "table"
					privileges  = ["SELECT"]
				}

				data "postgresql_role" "owner" {
					name = postgresql_role.owner.name
				}

				data "postgresql_database" "app" {
					name = postgresql_database.app.name
				}

				data "postgresql_schemas" "all" {
					include_system_schemas = false
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role.owner", "create_database", "true"),
					resource.TestCheckResourceAttr("postgresql_role.app", "roles.#", "1"),
					resource.TestCheckResourceAttr("postgresql_database.app", "name", "acctest_fullstack_db"),
					resource.TestCheckResourceAttr("postgresql_schema.api", "name", "api"),
					resource.TestCheckResourceAttr("postgresql_grant.db_connect", "privileges.#", "1"),
					resource.TestCheckResourceAttr("postgresql_grant.schema_usage", "privileges.#", "1"),
					resource.TestCheckResourceAttr("postgresql_default_privileges.tables", "object_type", "table"),
					resource.TestCheckResourceAttr("data.postgresql_role.owner", "create_database", "true"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.app", "oid"),
				),
			},
		},
	})
}
