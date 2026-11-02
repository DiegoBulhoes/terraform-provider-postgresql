package resource

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lib/pq"
)

var (
	_ resource.Resource                = (*RoleMembershipResource)(nil)
	_ resource.ResourceWithImportState = (*RoleMembershipResource)(nil)
)

type RoleMembershipResource struct {
	DB common.DBTX
}

type RoleMembershipResourceModel struct {
	ID          types.String   `tfsdk:"id"`
	Role        types.String   `tfsdk:"role"`
	Member      types.String   `tfsdk:"member"`
	AdminOption types.Bool     `tfsdk:"admin_option"`
	Timeouts    timeouts.Value `tfsdk:"timeouts"`
}

func NewRoleMembershipResource() resource.Resource {
	return &RoleMembershipResource{}
}

func (r *RoleMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_membership"
}

func (r *RoleMembershipResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     0,
		Description: "Manages the membership of one role in another (GRANT role TO member).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier: {role}/{member}.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"role": schema.StringAttribute{
				Description: "The role being granted.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"member": schema.StringAttribute{
				Description: "The role that receives the membership.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"admin_option": schema.BoolAttribute{
				Description: "Whether the member can grant the role to others. Default: false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Read:   true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *RoleMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	db, err := common.ConfigureDB(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", err.Error())
		return
	}
	r.DB = db
}

func membershipID(m *RoleMembershipResourceModel) string {
	return m.Role.ValueString() + "/" + m.Member.ValueString()
}

func grantMembershipStatement(role, member string, admin bool) string {
	stmt := fmt.Sprintf("GRANT %s TO %s", pq.QuoteIdentifier(role), pq.QuoteIdentifier(member))
	if admin {
		stmt += " WITH ADMIN OPTION"
	}
	return stmt
}

func (r *RoleMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RoleMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, d := plan.Timeouts.Create(ctx, common.DefaultTimeout)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	stmt := grantMembershipStatement(plan.Role.ValueString(), plan.Member.ValueString(), plan.AdminOption.ValueBool())
	logStatements(ctx, []string{stmt})
	if _, err := r.DB.ExecContext(ctx, stmt); err != nil {
		resp.Diagnostics.AddError("Error granting role membership", err.Error())
		return
	}

	plan.ID = types.StringValue(membershipID(&plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Since PostgreSQL 16 a membership has one row per grantor; it holds while any row does.
const membershipQuery = `SELECT bool_or(m.admin_option)
	FROM pg_catalog.pg_auth_members m
	JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
	JOIN pg_catalog.pg_roles u ON u.oid = m.member
	WHERE r.rolname = $1 AND u.rolname = $2`

func (r *RoleMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RoleMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readTimeout, d := state.Timeouts.Read(ctx, common.DefaultTimeout)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	var admin sql.NullBool
	if err := r.DB.QueryRowContext(ctx, membershipQuery, state.Role.ValueString(), state.Member.ValueString()).Scan(&admin); err != nil {
		resp.Diagnostics.AddError("Error reading role membership", err.Error())
		return
	}
	if !admin.Valid {
		resp.State.RemoveResource(ctx)
		return
	}

	state.AdminOption = types.BoolValue(admin.Bool)
	state.ID = types.StringValue(membershipID(&state))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// MembershipGrant is one grantor's row of a membership.
type MembershipGrant struct {
	// Grantor is empty when the grantor role no longer exists (PostgreSQL 15 and older).
	Grantor     string
	AdminOption bool
	// Revocable is false when the provider's user lacks the privileges of Grantor,
	// which PostgreSQL 16+ requires for REVOKE ... GRANTED BY.
	Revocable bool
}

// Before PostgreSQL 16 there is one row and REVOKE ignores GRANTED BY.
const membershipGrantsQuery = `SELECT coalesce(g.rolname, ''), m.admin_option,
		current_setting('server_version_num')::int < 160000 OR g.oid IS NULL OR pg_has_role(g.oid, 'USAGE')
	FROM pg_catalog.pg_auth_members m
	JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
	JOIN pg_catalog.pg_roles u ON u.oid = m.member
	LEFT JOIN pg_catalog.pg_roles g ON g.oid = m.grantor
	WHERE r.rolname = $1 AND u.rolname = $2
	ORDER BY 1`

// ReadMembershipGrants returns every grantor's row of the membership of member in role.
func ReadMembershipGrants(ctx context.Context, db common.DBTX, role, member string) ([]MembershipGrant, error) {
	rows, err := db.QueryContext(ctx, membershipGrantsQuery, role, member)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var grants []MembershipGrant
	for rows.Next() {
		var g MembershipGrant
		if err := rows.Scan(&g.Grantor, &g.AdminOption, &g.Revocable); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

// RevokeMembershipStatements revokes the membership, or only its admin option,
// once per grantor the provider may revoke for, and lists the grantors it may not.
func RevokeMembershipStatements(role, member string, grants []MembershipGrant, adminOnly bool) ([]string, []string) {
	prefix := "REVOKE "
	if adminOnly {
		prefix = "REVOKE ADMIN OPTION FOR "
	}
	base := prefix + pq.QuoteIdentifier(role) + " FROM " + pq.QuoteIdentifier(member)
	var statements, skipped []string
	for _, g := range grants {
		switch {
		case adminOnly && !g.AdminOption:
		case !g.Revocable:
			skipped = append(skipped, g.Grantor)
		case g.Grantor == "":
			statements = append(statements, base)
		default:
			statements = append(statements, base+" GRANTED BY "+pq.QuoteIdentifier(g.Grantor))
		}
	}
	return statements, skipped
}

func skippedGrantorsWarning(diags *diag.Diagnostics, role, member string, skipped []string) {
	if len(skipped) == 0 {
		return
	}
	diags.AddWarning("Role membership kept by other grantors",
		fmt.Sprintf("The provider's user lacks the privileges of %s, so their grants of %s to %s were not revoked.",
			strings.Join(skipped, ", "), role, member))
}

func (r *RoleMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RoleMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, d := plan.Timeouts.Update(ctx, common.DefaultTimeout)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	role, member := plan.Role.ValueString(), plan.Member.ValueString()
	var statements []string
	if plan.AdminOption.ValueBool() {
		statements = []string{grantMembershipStatement(role, member, true)}
	} else {
		grants, err := ReadMembershipGrants(ctx, r.DB, role, member)
		if err != nil {
			resp.Diagnostics.AddError("Error reading role membership", err.Error())
			return
		}
		var skipped []string
		statements, skipped = RevokeMembershipStatements(role, member, grants, true)
		skippedGrantorsWarning(&resp.Diagnostics, role, member, skipped)
	}
	logStatements(ctx, statements)
	if err := common.ExecInTx(ctx, r.DB, statements); err != nil {
		resp.Diagnostics.AddError("Error updating role membership", err.Error())
		return
	}

	plan.ID = types.StringValue(membershipID(&plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RoleMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, d := state.Timeouts.Delete(ctx, common.DefaultTimeout)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	role, member := state.Role.ValueString(), state.Member.ValueString()
	grants, err := ReadMembershipGrants(ctx, r.DB, role, member)
	if err != nil {
		resp.Diagnostics.AddError("Error reading role membership", err.Error())
		return
	}
	statements, skipped := RevokeMembershipStatements(role, member, grants, false)
	skippedGrantorsWarning(&resp.Diagnostics, role, member, skipped)
	logStatements(ctx, statements)
	err = common.ExecInTx(ctx, r.DB, statements)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "42704" {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error revoking role membership", err.Error())
	}
}

func (r *RoleMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid Import ID",
			fmt.Sprintf("Expected format: role/member. Got: %s", req.ID))
		return
	}
	model := RoleMembershipResourceModel{
		Role:        types.StringValue(parts[0]),
		Member:      types.StringValue(parts[1]),
		AdminOption: types.BoolValue(false),
		Timeouts:    timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{"create": types.StringType, "read": types.StringType, "update": types.StringType, "delete": types.StringType})},
	}
	model.ID = types.StringValue(membershipID(&model))
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
