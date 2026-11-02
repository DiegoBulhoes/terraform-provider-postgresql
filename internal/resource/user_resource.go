package resource

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lib/pq"
)

var (
	_ resource.Resource                = (*UserResource)(nil)
	_ resource.ResourceWithImportState = (*UserResource)(nil)
)

type UserResource struct {
	DB common.DBTX
}

type UserResourceModel struct {
	Name            types.String   `tfsdk:"name"`
	Password        types.String   `tfsdk:"password"`
	Superuser       types.Bool     `tfsdk:"superuser"`
	CreateDatabase  types.Bool     `tfsdk:"create_database"`
	CreateRole      types.Bool     `tfsdk:"create_role"`
	Replication     types.Bool     `tfsdk:"replication"`
	ConnectionLimit types.Int64    `tfsdk:"connection_limit"`
	ValidUntil      types.String   `tfsdk:"valid_until"`
	Roles           types.List     `tfsdk:"roles"`
	ReassignOwnedTo types.String   `tfsdk:"reassign_owned_to"`
	OID             types.Int64    `tfsdk:"oid"`
	Timeouts        timeouts.Value `tfsdk:"timeouts"`
}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

func (r *UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     0,
		Description: "Manages a PostgreSQL user (a role with LOGIN privilege).",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the user.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 63),
				},
			},
			"password": schema.StringAttribute{
				Description: "The password for the user.",
				Optional:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"superuser": schema.BoolAttribute{
				Description: "Whether the user is a superuser. Default: false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"create_database": schema.BoolAttribute{
				Description: "Whether the user can create databases. Default: false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"create_role": schema.BoolAttribute{
				Description: "Whether the user can create other roles. Default: false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"replication": schema.BoolAttribute{
				Description: "Whether the user can initiate streaming replication. Default: false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"connection_limit": schema.Int64Attribute{
				Description: "Maximum number of concurrent connections the user can make. -1 means no limit. Default: -1.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(-1),
				Validators: []validator.Int64{
					int64validator.AtLeast(-1),
				},
			},
			"valid_until": schema.StringAttribute{
				Description: "Timestamp until which the user's password is valid. If omitted, the password never expires. Format: RFC 3339 (e.g. 2025-12-31T23:59:59Z).",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`),
						"must be a valid timestamp (e.g. 2025-12-31T23:59:59Z)",
					),
				},
			},
			"roles": schema.ListAttribute{
				Description: "List of roles that this user is a member of. The order is not significant.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"reassign_owned_to": schema.StringAttribute{
				Description: "On destroy, reassign the objects the user owns in the provider's database to this role and drop its privileges there (REASSIGN OWNED, DROP OWNED). Objects in other databases still block the drop.",
				Optional:    true,
			},
			"oid": schema.Int64Attribute{
				Description: "The OID of the user.",
				Computed:    true,
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

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan UserResourceModel
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

	userName := plan.Name.ValueString()
	statements := []string{
		fmt.Sprintf("CREATE USER %s", pq.QuoteIdentifier(userName)) + r.BuildUserOptions(ctx, &plan, nil),
	}
	for _, memberOf := range common.StringListToSlice(ctx, plan.Roles) {
		statements = append(statements, fmt.Sprintf("GRANT %s TO %s", pq.QuoteIdentifier(memberOf), pq.QuoteIdentifier(userName)))
	}
	if err := common.ExecInTx(ctx, r.DB, statements); err != nil {
		resp.Diagnostics.AddError("Error creating user", fmt.Sprintf("Could not create user %s: %s", userName, err.Error()))
		return
	}

	_, diags := r.ReadUser(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserResourceModel
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

	found, diags := r.ReadUser(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan UserResourceModel
	var state UserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
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

	oldName := state.Name.ValueString()
	newName := plan.Name.ValueString()

	var statements []string
	if oldName != newName {
		statements = append(statements, fmt.Sprintf("ALTER USER %s RENAME TO %s",
			pq.QuoteIdentifier(oldName), pq.QuoteIdentifier(newName)))
	}
	statements = append(statements, fmt.Sprintf("ALTER USER %s", pq.QuoteIdentifier(newName))+r.BuildUserOptions(ctx, &plan, &state))

	toGrant, toRevoke := DiffRoles(common.StringListToSlice(ctx, state.Roles), common.StringListToSlice(ctx, plan.Roles))
	for _, memberOf := range toGrant {
		statements = append(statements, fmt.Sprintf("GRANT %s TO %s", pq.QuoteIdentifier(memberOf), pq.QuoteIdentifier(newName)))
	}
	for _, memberOf := range toRevoke {
		statements = append(statements, fmt.Sprintf("REVOKE %s FROM %s", pq.QuoteIdentifier(memberOf), pq.QuoteIdentifier(newName)))
	}

	if err := common.ExecInTx(ctx, r.DB, statements); err != nil {
		resp.Diagnostics.AddError("Error updating user", fmt.Sprintf("Could not update user %s: %s", oldName, err.Error()))
		return
	}

	_, diags := r.ReadUser(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state UserResourceModel
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

	userName := state.Name.ValueString()
	statements := append(ReassignOwned(userName, state.ReassignOwnedTo), fmt.Sprintf("DROP USER %s", pq.QuoteIdentifier(userName)))
	if err := common.ExecInTx(ctx, r.DB, statements); err != nil {
		resp.Diagnostics.AddError("Error deleting user", DropRoleError(userName, err))
		return
	}
}

// ReassignOwned returns the statements that free a role of what it owns, when reassignTo is set.
func ReassignOwned(roleName string, reassignTo types.String) []string {
	if !common.IsSet(reassignTo) || reassignTo.ValueString() == "" {
		return nil
	}
	return []string{
		fmt.Sprintf("REASSIGN OWNED BY %s TO %s", pq.QuoteIdentifier(roleName), pq.QuoteIdentifier(reassignTo.ValueString())),
		fmt.Sprintf("DROP OWNED BY %s", pq.QuoteIdentifier(roleName)),
	}
}

// DropRoleError explains a failed DROP ROLE, pointing at reassign_owned_to when objects depend on it.
func DropRoleError(roleName string, err error) string {
	msg := fmt.Sprintf("Could not drop %s: %s", roleName, err.Error())
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "2BP01" {
		msg += "\nSet reassign_owned_to to reassign its objects and drop its privileges first."
	}
	return msg
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// BuildUserOptions renders the WITH clause; state is nil on create. Flags are
// sent only when they change, and a removed password or expiry is cleared.
func (r *UserResource) BuildUserOptions(_ context.Context, plan, state *UserResourceModel) string {
	var prev *roleAttributes
	prevPassword, prevValidUntil := types.StringNull(), types.StringNull()
	if state != nil {
		attrs := state.attributes()
		prev = &attrs
		prevPassword, prevValidUntil = state.Password, state.ValidUntil
	}
	opts := plan.attributes().options(prev)

	switch {
	case common.IsSet(plan.Password):
		opts.Password = plan.Password.ValueString()
	case common.IsSet(prevPassword):
		opts.NullPassword = true
	}
	switch {
	case common.IsSet(plan.ValidUntil):
		opts.ValidUntil = plan.ValidUntil.ValueString()
	case common.IsSet(prevValidUntil):
		opts.ValidUntil = "infinity"
	}
	return common.BuildRoleOptions(opts)
}

// ReadUser reads the user into model; found is false when it does not exist.
func (r *UserResource) ReadUser(ctx context.Context, model *UserResourceModel) (found bool, diags diag.Diagnostics) {
	userName := model.Name.ValueString()

	row, found, err := readRoleRow(ctx, r.DB, userName)
	if err != nil {
		diags.AddError("Error reading user", fmt.Errorf("read user %s: %w", userName, err).Error())
		return true, diags
	}
	if !found {
		return false, diags
	}

	if !row.CanLogin {
		diags.AddWarning(
			"Role is not a user",
			fmt.Sprintf("Role %s does not have LOGIN privilege. Consider using postgresql_role instead.", userName),
		)
	}

	model.attributes().set(row)
	oid := row.OID

	switch {
	case !row.ValidUntil.Valid:
		model.ValidUntil = types.StringNull()
	case !SameInstant(model.ValidUntil.ValueString(), row.ValidUntil.String):
		model.ValidUntil = types.StringValue(row.ValidUntil.String)
	}

	// Read role memberships
	// Since PostgreSQL 16 a membership has one row per grantor.
	const memberQuery = `SELECT DISTINCT r.rolname
		FROM pg_catalog.pg_auth_members m
		JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
		WHERE m.member = $1
		ORDER BY r.rolname`

	rows, err := r.DB.QueryContext(ctx, memberQuery, oid)
	if err != nil {
		diags.AddError(
			"Error reading role memberships",
			fmt.Sprintf("Could not read memberships for user %s: %s", userName, err.Error()),
		)
		return true, diags
	}
	defer rows.Close() //nolint:errcheck

	var memberOfRoles []attr.Value
	for rows.Next() {
		var memberOfName string
		if err := rows.Scan(&memberOfName); err != nil {
			diags.AddError("Error scanning role membership", fmt.Sprintf("Could not scan membership row for user %s: %s", userName, err.Error()))
			return true, diags
		}
		memberOfRoles = append(memberOfRoles, types.StringValue(memberOfName))
	}
	if err := rows.Err(); err != nil {
		diags.AddError("Error iterating role memberships", fmt.Sprintf("Error iterating memberships for user %s: %s", userName, err.Error()))
		return true, diags
	}

	switch {
	case len(memberOfRoles) > 0 && SameMembers(common.StringListToSlice(ctx, model.Roles), memberOfRoles):
		// The configured order is kept.
	case len(memberOfRoles) > 0:
		rolesList, listDiags := types.ListValue(types.StringType, memberOfRoles)
		diags.Append(listDiags...)
		model.Roles = rolesList
	case !model.Roles.IsNull():
		model.Roles, _ = types.ListValue(types.StringType, []attr.Value{})
	default:
		model.Roles = types.ListNull(types.StringType)
	}

	return true, diags
}

// SameMembers reports whether names and values hold the same role names, in any order.
func SameMembers(names []string, values []attr.Value) bool {
	if len(names) != len(values) {
		return false
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	for _, v := range values {
		s, ok := v.(types.String)
		if !ok || !set[s.ValueString()] {
			return false
		}
	}
	return true
}

var timestampLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999Z07",
	"2006-01-02T15:04:05.999999999Z07",
}

// SameInstant reports whether two timestamps with time zones name the same instant.
func SameInstant(a, b string) bool {
	if a == b {
		return true
	}
	ta, errA := parseTimestamp(a)
	tb, errB := parseTimestamp(b)
	return errA == nil && errB == nil && ta.Equal(tb)
}

func parseTimestamp(v string) (time.Time, error) {
	var err error
	for _, layout := range timestampLayouts {
		var t time.Time
		if t, err = time.Parse(layout, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, err
}
