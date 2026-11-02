//go:build integration

package resource_test

import (
	"database/sql"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Any spelling the server accepts for the encoding converges.
func TestAccPostgresqlDatabase_encodingSpelling(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_database" "test" {
  name     = "acctest_db_enc"
  encoding = "utf-8"
}`,
				Check: resource.TestCheckResourceAttr("postgresql_database.test", "encoding", "utf-8"),
			},
			{
				Config: `
resource "postgresql_database" "test" {
  name     = "acctest_db_enc"
  encoding = "utf-8"
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// The template cannot be read back, so an imported database must not be replaced over it.
func TestAccPostgresqlDatabase_importDoesNotReplaceOverTemplate(t *testing.T) {
	config := `
resource "postgresql_database" "test" {
  name     = "acctest_db_tpl"
  template = "template1"
}`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				PreConfig:          func() { acctest.MustExec(t, `CREATE DATABASE acctest_db_tpl TEMPLATE template1`) },
				Config:             config,
				ResourceName:       "postgresql_database.test",
				ImportState:        true,
				ImportStateId:      "acctest_db_tpl",
				ImportStatePersist: true,
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("postgresql_database.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

func TestAccPostgresqlDatabase_forceDropWithOpenConnection(t *testing.T) {
	var conn *sql.DB
	t.Cleanup(func() {
		if conn != nil {
			_ = conn.Close()
		}
	})
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlDatabaseDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_database" "test" {
  name       = "acctest_db_force"
  force_drop = true
}`,
				Check: func(_ *terraform.State) error {
					var err error
					if conn, err = acctest.OpenDatabase("acctest_db_force"); err != nil {
						return err
					}
					return conn.Ping()
				},
			},
		},
	})
}
