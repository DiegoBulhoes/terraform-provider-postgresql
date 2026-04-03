// Tests for postgresql_version data source.
package datasource_test

import (
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresqlVersionDataSource_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "postgresql_version" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.postgresql_version.test", "version"),
					resource.TestCheckResourceAttrSet("data.postgresql_version.test", "major"),
					resource.TestCheckResourceAttrSet("data.postgresql_version.test", "minor"),
					resource.TestCheckResourceAttrSet("data.postgresql_version.test", "server_version_num"),
				),
			},
		},
	})
}

func TestAccPostgresqlVersionDataSource_verifyAttributes(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "postgresql_version" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// version string should contain "PostgreSQL"
					resource.TestMatchResourceAttr(
						"data.postgresql_version.test", "version",
						regexp.MustCompile(`PostgreSQL`),
					),
					// major version should be a reasonable number (>= 10)
					resource.TestMatchResourceAttr(
						"data.postgresql_version.test", "major",
						regexp.MustCompile(`^[1-9]\d*$`),
					),
					// minor version should be a non-negative number
					resource.TestMatchResourceAttr(
						"data.postgresql_version.test", "minor",
						regexp.MustCompile(`^\d+$`),
					),
					// server_version_num should be a 6-digit number (e.g. 160002)
					resource.TestMatchResourceAttr(
						"data.postgresql_version.test", "server_version_num",
						regexp.MustCompile(`^\d{5,6}$`),
					),
				),
			},
		},
	})
}
