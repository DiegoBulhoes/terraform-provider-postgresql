//go:build integration

// Tests for provider configuration validation and full stack integration.
package provider_test

import (
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
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
				resource "postgresql_user" "owner" {
					name            = "acctest_fullstack_owner"
					password        = "ownerpass"
					create_database = true
				}

				resource "postgresql_role" "reader" {
					name = "acctest_fullstack_reader"
				}

				resource "postgresql_user" "app" {
					name     = "acctest_fullstack_app"
					password = "apppass"
					roles    = [postgresql_role.reader.name]
				}

				resource "postgresql_database" "app" {
					name  = "acctest_fullstack_db"
					owner = postgresql_user.owner.name
				}

				resource "postgresql_schema" "api" {
					name  = "api"
					owner = postgresql_user.owner.name
				}

				resource "postgresql_grant" "db_connect" {
					role        = postgresql_user.app.name
					object_type = "database"
					database    = postgresql_database.app.name
					privileges  = ["CONNECT"]
				}

				resource "postgresql_grant" "schema_usage" {
					role        = postgresql_user.app.name
					object_type = "schema"
					schema      = postgresql_schema.api.name
					privileges  = ["USAGE"]
				}

				data "postgresql_role" "reader" {
					name = postgresql_role.reader.name
				}

				data "postgresql_user" "owner" {
					name = postgresql_user.owner.name
				}

				data "postgresql_database" "app" {
					name = postgresql_database.app.name
				}

				data "postgresql_schemas" "all" {
					include_system_schemas = false
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_user.owner", "create_database", "true"),
					resource.TestCheckResourceAttr("postgresql_user.app", "roles.#", "1"),
					resource.TestCheckResourceAttr("postgresql_database.app", "name", "acctest_fullstack_db"),
					resource.TestCheckResourceAttr("postgresql_schema.api", "name", "api"),
					resource.TestCheckResourceAttr("postgresql_grant.db_connect", "privileges.#", "1"),
					resource.TestCheckResourceAttr("postgresql_grant.schema_usage", "privileges.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_user.owner", "create_database", "true"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.app", "oid"),
				),
			},
		},
	})
}
