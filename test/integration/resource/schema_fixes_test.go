//go:build integration

package resource_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func schemaExists(name string, want bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		got, err := acctest.SchemaExists("postgres", name)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("schema %s exists=%v, want %v", name, got, want)
		}
		return nil
	}
}

// A schema owned by a role the provider user cannot use is still found.
func TestAccPostgresqlSchema_ownedByUnrelatedRoleIsStillRead(t *testing.T) {
	acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_sch_limited`)
	acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_sch_stranger`)
	acctest.MustExec(t, `CREATE ROLE acctest_sch_limited LOGIN PASSWORD 'limited'`)
	acctest.MustExec(t, `CREATE ROLE acctest_sch_stranger`)
	acctest.MustExec(t, `GRANT CREATE ON DATABASE postgres TO acctest_sch_limited`)
	t.Cleanup(func() {
		acctest.MustExec(t, `DROP SCHEMA IF EXISTS acctest_sch_hidden`)
		acctest.MustExec(t, `REVOKE CREATE ON DATABASE postgres FROM acctest_sch_limited`)
		acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_sch_limited`)
		acctest.MustExec(t, `DROP ROLE IF EXISTS acctest_sch_stranger`)
	})
	config := `
provider "postgresql" {
  username = "acctest_sch_limited"
  password = "limited"
}

resource "postgresql_schema" "test" {
  name = "acctest_sch_hidden"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					acctest.MustExec(t, `ALTER SCHEMA acctest_sch_hidden OWNER TO acctest_sch_stranger`)
				},
				Config: config,
				// The owner is computed, so a change made outside Terraform is adopted, not recreated.
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("postgresql_schema.test", "owner", "acctest_sch_stranger"),
			},
			{
				PreConfig: func() {
					acctest.MustExec(t, `ALTER SCHEMA acctest_sch_hidden OWNER TO acctest_sch_limited`)
				},
				Config: config,
			},
		},
	})
}

// A failed owner change must not leave the rename applied.
func TestAccPostgresqlSchema_renameRollsBackWhenOwnerFails(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlSchemaDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_schema" "test" {
  name = "acctest_sch_rb_a"
}`,
			},
			{
				Config: `
resource "postgresql_schema" "test" {
  name  = "acctest_sch_rb_b"
  owner = "acctest_no_such_role"
}`,
				ExpectError: regexp.MustCompile(`acctest_no_such_role`),
			},
			{
				Config: `
resource "postgresql_schema" "test" {
  name = "acctest_sch_rb_a"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					schemaExists("acctest_sch_rb_a", true),
					schemaExists("acctest_sch_rb_b", false),
				),
			},
		},
	})
}

func TestAccPostgresqlSchema_dropCascade(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlSchemaDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_schema" "test" {
  name         = "acctest_sch_cascade"
  drop_cascade = true
}`,
				Check: func(_ *terraform.State) error {
					acctest.MustExec(t, `CREATE TABLE acctest_sch_cascade.t (id int)`)
					return nil
				},
			},
		},
	})
}

func TestAccPostgresqlSchema_withoutCascadeKeepsObjects(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlSchemaDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_schema" "test" {
  name = "acctest_sch_nocascade"
}`,
				Check: func(_ *terraform.State) error {
					acctest.MustExec(t, `CREATE TABLE acctest_sch_nocascade.t (id int)`)
					return nil
				},
			},
			{
				Config:      ` `,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`drop_cascade`),
			},
			{
				PreConfig: func() { acctest.MustExec(t, `DROP TABLE acctest_sch_nocascade.t`) },
				Config: `
resource "postgresql_schema" "test" {
  name = "acctest_sch_nocascade"
}`,
			},
		},
	})
}

// if_not_exists on a schema that already exists still applies the configured owner.
func TestAccPostgresqlSchema_ifNotExistsAppliesOwner(t *testing.T) {
	acctest.MustExec(t, `CREATE SCHEMA IF NOT EXISTS acctest_sch_ine`)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlSchemaDestroy,
		Steps: []resource.TestStep{
			{
				Config: `
resource "postgresql_role" "owner" {
  name = "acctest_sch_ine_owner"
}

resource "postgresql_schema" "test" {
  name          = "acctest_sch_ine"
  owner         = postgresql_role.owner.name
  if_not_exists = true
}`,
				Check: resource.TestCheckResourceAttr("postgresql_schema.test", "owner", "acctest_sch_ine_owner"),
			},
		},
	})
}
