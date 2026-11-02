//go:build integration

package resource_test

import (
	"fmt"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// membershipRoles creates the group and member roles outside Terraform, so the
// membership can be checked after destroy, and drops them when the test ends.
func membershipRoles(t *testing.T, roles ...string) {
	t.Helper()
	drop := func() {
		for i := len(roles) - 1; i >= 0; i-- {
			acctest.MustExec(t, fmt.Sprintf(`DROP ROLE IF EXISTS %s`, roles[i]))
		}
	}
	drop()
	for _, r := range roles {
		acctest.MustExec(t, fmt.Sprintf(`CREATE ROLE %s`, r))
	}
	t.Cleanup(drop)
}

func membershipConfig(role, member string, admin bool) string {
	return fmt.Sprintf(`
resource "postgresql_role_membership" "test" {
  role         = %q
  member       = %q
  admin_option = %t
}`, role, member, admin)
}

func isMember(role, member string, want bool) resource.TestCheckFunc {
	return hasPrivilege(fmt.Sprintf(`SELECT pg_has_role('%s', '%s', 'MEMBER')`, member, role), want)
}

func hasAdmin(role, member string, want bool) resource.TestCheckFunc {
	return hasPrivilege(fmt.Sprintf(`SELECT coalesce(bool_or(admin_option), false) FROM pg_auth_members
		WHERE roleid = '%s'::regrole AND member = '%s'::regrole`, role, member), want)
}

func serverVersionNum(t *testing.T) int {
	t.Helper()
	db, err := acctest.GetDB()
	if err != nil {
		t.Fatal(err)
	}
	var v int
	if err := db.QueryRow(`SELECT current_setting('server_version_num')::int`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestAccPostgresqlRoleMembership_basic(t *testing.T) {
	membershipRoles(t, "acctest_rm_group", "acctest_rm_member")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             isMember("acctest_rm_group", "acctest_rm_member", false),
		Steps: []resource.TestStep{
			{
				Config: membershipConfig("acctest_rm_group", "acctest_rm_member", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("postgresql_role_membership.test", "id", "acctest_rm_group/acctest_rm_member"),
					resource.TestCheckResourceAttr("postgresql_role_membership.test", "admin_option", "false"),
					isMember("acctest_rm_group", "acctest_rm_member", true),
					hasAdmin("acctest_rm_group", "acctest_rm_member", false),
				),
			},
			{
				Config:           membershipConfig("acctest_rm_group", "acctest_rm_member", false),
				ConfigPlanChecks: emptyPlan(),
			},
			{
				ResourceName:            "postgresql_role_membership.test",
				ImportState:             true,
				ImportStateId:           "acctest_rm_group/acctest_rm_member",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				Config:           membershipConfig("acctest_rm_group", "acctest_rm_member", true),
				ConfigPlanChecks: expectUpdate("postgresql_role_membership.test"),
				Check: resource.ComposeTestCheckFunc(
					isMember("acctest_rm_group", "acctest_rm_member", true),
					hasAdmin("acctest_rm_group", "acctest_rm_member", true),
				),
			},
			{
				Config:           membershipConfig("acctest_rm_group", "acctest_rm_member", true),
				ConfigPlanChecks: emptyPlan(),
			},
			{
				ResourceName:            "postgresql_role_membership.test",
				ImportState:             true,
				ImportStateId:           "acctest_rm_group/acctest_rm_member",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				Config:           membershipConfig("acctest_rm_group", "acctest_rm_member", false),
				ConfigPlanChecks: expectUpdate("postgresql_role_membership.test"),
				Check: resource.ComposeTestCheckFunc(
					isMember("acctest_rm_group", "acctest_rm_member", true),
					hasAdmin("acctest_rm_group", "acctest_rm_member", false),
				),
			},
		},
	})
}

// A membership revoked outside Terraform is granted again.
func TestAccPostgresqlRoleMembership_drift(t *testing.T) {
	membershipRoles(t, "acctest_rm_dgroup", "acctest_rm_dmember")
	config := membershipConfig("acctest_rm_dgroup", "acctest_rm_dmember", false)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             isMember("acctest_rm_dgroup", "acctest_rm_dmember", false),
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() { acctest.MustExec(t, `REVOKE acctest_rm_dgroup FROM acctest_rm_dmember`) },
				Config:    config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("postgresql_role_membership.test", plancheck.ResourceActionCreate),
				}},
				Check: isMember("acctest_rm_dgroup", "acctest_rm_dmember", true),
			},
			{Config: config, ConfigPlanChecks: emptyPlan()},
		},
	})
}

// A member that was dropped removes the membership from state.
func TestAccPostgresqlRoleMembership_memberDropped(t *testing.T) {
	membershipRoles(t, "acctest_rm_xgroup", "acctest_rm_xmember")
	config := membershipConfig("acctest_rm_xgroup", "acctest_rm_xmember", false)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig:          func() { acctest.MustExec(t, `DROP ROLE acctest_rm_xmember`) },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// Since PostgreSQL 16 a second grantor adds its own row: plan stays empty, its
// admin option is revoked with the provider's, and destroy revokes both.
func TestAccPostgresqlRoleMembership_secondGrantor(t *testing.T) {
	if serverVersionNum(t) < 160000 {
		t.Skip("one row per grantor needs PostgreSQL 16+")
	}
	membershipRoles(t, "acctest_rm_ggroup", "acctest_rm_gmember", "acctest_rm_granter")
	config := membershipConfig("acctest_rm_ggroup", "acctest_rm_gmember", false)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             isMember("acctest_rm_ggroup", "acctest_rm_gmember", false),
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					acctest.MustExec(t, `GRANT acctest_rm_ggroup TO acctest_rm_granter WITH ADMIN OPTION`)
					acctest.MustExec(t, `GRANT acctest_rm_ggroup TO acctest_rm_gmember GRANTED BY acctest_rm_granter`)
				},
				Config:           config,
				ConfigPlanChecks: emptyPlan(),
			},
			{
				PreConfig: func() {
					acctest.MustExec(t, `GRANT acctest_rm_ggroup TO acctest_rm_gmember WITH ADMIN OPTION GRANTED BY acctest_rm_granter`)
				},
				Config:           config,
				ConfigPlanChecks: expectUpdate("postgresql_role_membership.test"),
				Check:            hasAdmin("acctest_rm_ggroup", "acctest_rm_gmember", false),
			},
			{Config: config, ConfigPlanChecks: emptyPlan()},
		},
	})
}
