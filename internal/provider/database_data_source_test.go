package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresqlDatabaseDataSource_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_database" "test" {
  name = "postgres"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_database.test", "name", "postgres"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "owner"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "encoding"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "lc_collate"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "lc_ctype"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "tablespace_name"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "oid"),
				),
			},
		},
	})
}

func TestAccPostgresqlDatabaseDataSource_allAttributes(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "postgresql_database" "test" {
  name = "postgres"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_database.test", "name", "postgres"),
					resource.TestCheckResourceAttr("data.postgresql_database.test", "allow_connections", "true"),
					resource.TestCheckResourceAttr("data.postgresql_database.test", "is_template", "false"),
					resource.TestCheckResourceAttr("data.postgresql_database.test", "connection_limit", "-1"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "owner"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "encoding"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "lc_collate"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "lc_ctype"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "tablespace_name"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "oid"),
				),
			},
		},
	})
}

func TestAccPostgresqlDatabaseDataSource_custom(t *testing.T) {
	rName := "acctest_db_ds_custom"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "postgresql_database" "test" {
  name             = %q
  connection_limit = 25
}

data "postgresql_database" "test" {
  name = postgresql_database.test.name
}
`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.postgresql_database.test", "name", rName),
					resource.TestCheckResourceAttr("data.postgresql_database.test", "encoding", "UTF8"),
					resource.TestCheckResourceAttr("data.postgresql_database.test", "connection_limit", "25"),
					resource.TestCheckResourceAttr("data.postgresql_database.test", "allow_connections", "true"),
					resource.TestCheckResourceAttr("data.postgresql_database.test", "is_template", "false"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "owner"),
					resource.TestCheckResourceAttrSet("data.postgresql_database.test", "oid"),
				),
			},
		},
	})
}
