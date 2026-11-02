//go:build integration

package resource_test

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/lib/pq"
)

// installedExtension returns the schema and version of an extension, or sql.ErrNoRows.
func installedExtension(database, name string) (string, string, error) {
	db, err := acctest.OpenDatabase(database)
	if err != nil {
		return "", "", err
	}
	defer db.Close() //nolint:errcheck
	var schema, version string
	err = db.QueryRow(`SELECT n.nspname, e.extversion FROM pg_extension e
		JOIN pg_namespace n ON n.oid = e.extnamespace WHERE e.extname = $1`, name).Scan(&schema, &version)
	return schema, version, err
}

func extensionOnServer(database, name, schema, version string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		gotSchema, gotVersion, err := installedExtension(database, name)
		if err != nil {
			return fmt.Errorf("extension %s in %s: %w", name, database, err)
		}
		if gotSchema != schema || (version != "" && gotVersion != version) {
			return fmt.Errorf("extension %s is %s@%s, want %s@%s", name, gotSchema, gotVersion, schema, version)
		}
		return nil
	}
}

func testAccCheckPostgresqlExtensionDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "postgresql_extension" {
			continue
		}
		database, name := rs.Primary.Attributes["database"], rs.Primary.Attributes["name"]
		_, _, err := installedExtension(database, name)
		var pqErr *pq.Error
		switch {
		case err == nil:
			return fmt.Errorf("extension %s still exists in %s", name, database)
		case errors.Is(err, sql.ErrNoRows), errors.As(err, &pqErr) && pqErr.Code == "3D000":
		default:
			return err
		}
	}
	return nil
}

func TestAccPostgresqlExtension_basic(t *testing.T) {
	config := `
resource "postgresql_extension" "test" {
  name = "pgcrypto"
}`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlExtensionDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_extension.test", "name", "pgcrypto"),
					resource.TestCheckResourceAttr("postgresql_extension.test", "database", "postgres"),
					resource.TestCheckResourceAttr("postgresql_extension.test", "schema", "public"),
					resource.TestCheckResourceAttrSet("postgresql_extension.test", "version"),
					resource.TestCheckResourceAttr("postgresql_extension.test", "create_cascade", "false"),
					resource.TestCheckResourceAttr("postgresql_extension.test", "drop_cascade", "false"),
					extensionOnServer("postgres", "pgcrypto", "public", ""),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:                         "postgresql_extension.test",
				ImportState:                          true,
				ImportStateId:                        "pgcrypto",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
			{
				ResourceName:                         "postgresql_extension.test",
				ImportState:                          true,
				ImportStateId:                        "postgres/pgcrypto",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

func TestAccPostgresqlExtension_versionUpdate(t *testing.T) {
	config := func(version string) string {
		return fmt.Sprintf(`
resource "postgresql_extension" "test" {
  name    = "hstore"
  version = %q
}`, version)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlExtensionDestroy,
		Steps: []resource.TestStep{
			{
				Config: config("1.7"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_extension.test", "version", "1.7"),
					extensionOnServer("postgres", "hstore", "public", "1.7"),
				),
			},
			{
				Config: config("1.8"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("postgresql_extension.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_extension.test", "version", "1.8"),
					extensionOnServer("postgres", "hstore", "public", "1.8"),
				),
			},
			{
				Config: config("1.8"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config:      config("0.1"),
				ExpectError: regexp.MustCompile(`(?s)Error updating extension.*0\.1`),
			},
		},
	})
}

func TestAccPostgresqlExtension_schemaChange(t *testing.T) {
	config := func(schema string) string {
		return fmt.Sprintf(`
resource "postgresql_schema" "ext" {
  name = "acctest_ext_schema"
}

resource "postgresql_extension" "test" {
  name   = "citext"
  schema = %s
}`, schema)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlExtensionDestroy,
		Steps: []resource.TestStep{
			{
				Config: config(`"public"`),
				Check:  extensionOnServer("postgres", "citext", "public", ""),
			},
			{
				Config: config("postgresql_schema.ext.name"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("postgresql_extension.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_extension.test", "schema", "acctest_ext_schema"),
					extensionOnServer("postgres", "citext", "acctest_ext_schema", ""),
				),
			},
			{
				Config: config("postgresql_schema.ext.name"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccPostgresqlExtension_createCascade(t *testing.T) {
	t.Cleanup(func() { acctest.MustExec(t, `DROP EXTENSION IF EXISTS cube`) })
	config := func(cascade bool) string {
		return fmt.Sprintf(`
resource "postgresql_extension" "test" {
  name           = "earthdistance"
  create_cascade = %t
}`, cascade)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlExtensionDestroy,
		Steps: []resource.TestStep{
			{
				Config:      config(false),
				ExpectError: regexp.MustCompile(`(?s)Error creating extension.*cube`),
			},
			{
				Config: config(true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_extension.test", "create_cascade", "true"),
					extensionOnServer("postgres", "earthdistance", "public", ""),
					extensionOnServer("postgres", "cube", "public", ""),
				),
			},
			{
				Config: config(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccPostgresqlExtension_dropCascade(t *testing.T) {
	t.Cleanup(func() { acctest.MustExec(t, `DROP TABLE IF EXISTS acctest_ext_dependent`) })
	config := func(cascade bool) string {
		return fmt.Sprintf(`
resource "postgresql_extension" "test" {
  name         = "citext"
  drop_cascade = %t
}`, cascade)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlExtensionDestroy,
		Steps: []resource.TestStep{
			{
				Config: config(false),
				Check: func(_ *terraform.State) error {
					acctest.MustExec(t, `CREATE TABLE acctest_ext_dependent (email citext)`)
					return nil
				},
			},
			{
				Config:      config(false),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`drop_cascade = true`),
			},
			{
				Config: config(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("postgresql_extension.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

func TestAccPostgresqlExtension_otherDatabase(t *testing.T) {
	config := `
resource "postgresql_database" "other" {
  name = "acctest_ext_otherdb"
}

resource "postgresql_extension" "test" {
  name     = "pgcrypto"
  database = postgresql_database.other.name
}`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlExtensionDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_extension.test", "database", "acctest_ext_otherdb"),
					extensionOnServer("acctest_ext_otherdb", "pgcrypto", "public", ""),
					func(_ *terraform.State) error {
						if _, _, err := installedExtension("postgres", "pgcrypto"); !errors.Is(err, sql.ErrNoRows) {
							return fmt.Errorf("pgcrypto in postgres: %v, want none", err)
						}
						return nil
					},
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:                         "postgresql_extension.test",
				ImportState:                          true,
				ImportStateId:                        "acctest_ext_otherdb/pgcrypto",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

// An extension dropped outside Terraform is planned for creation again.
func TestAccPostgresqlExtension_droppedOutside(t *testing.T) {
	config := `
resource "postgresql_extension" "test" {
  name = "pgcrypto"
}`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckPostgresqlExtensionDestroy,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() { acctest.MustExec(t, `DROP EXTENSION pgcrypto`) },
				Config:    config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("postgresql_extension.test", plancheck.ResourceActionCreate),
					},
				},
				Check: extensionOnServer("postgres", "pgcrypto", "public", ""),
			},
		},
	})
}
