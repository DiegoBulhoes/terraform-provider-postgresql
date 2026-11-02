//go:build integration

package provider_test

import (
	"fmt"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const dbSelectionConfig = `
resource "postgresql_database" "other" {
  name = "acctest_dbsel"
}

resource "postgresql_schema" "in_other" {
  name     = "acctest_dbsel_schema"
  database = postgresql_database.other.name
}
`

func TestAccProvider_databaseAttributeSelectsTheDatabase(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(_ *terraform.State) error {
			db, err := acctest.GetDB()
			if err != nil {
				return err
			}
			var exists bool
			if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'acctest_dbsel')`).Scan(&exists); err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("database acctest_dbsel still exists")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: dbSelectionConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					schemaIn("acctest_dbsel", "acctest_dbsel_schema", true),
					schemaIn("postgres", "acctest_dbsel_schema", false),
				),
			},
			{
				Config: dbSelectionConfig + `
data "postgresql_query" "where" {
  database = "acctest_dbsel"
  query    = "SELECT current_database() AS db"
}

data "postgresql_schemas" "other" {
  database    = "acctest_dbsel"
  like_pattern = "acctest_dbsel_%"
}

data "postgresql_extensions" "other" {
  database = "acctest_dbsel"
}

data "postgresql_tables" "other" {
  database = "acctest_dbsel"
  schema   = "acctest_dbsel_schema"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_query.where", "rows.0.db", "acctest_dbsel"),
					resource.TestCheckResourceAttr("data.postgresql_schemas.other", "schemas.#", "1"),
					resource.TestCheckResourceAttr("data.postgresql_schemas.other", "schemas.0.name", "acctest_dbsel_schema"),
				),
			},
		},
	})
}

func schemaIn(database, schema string, want bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		got, err := acctest.SchemaExists(database, schema)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("schema %s in database %s: exists=%v, want %v", schema, database, got, want)
		}
		return nil
	}
}
