package resource

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
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
	_ resource.Resource                = (*roleResource)(nil)
	_ resource.ResourceWithImportState = (*roleResource)(nil)
)

type roleResource struct {
	db common.DBTX
}

type roleResourceModel struct {
	Name            types.String   `tfsdk:"name"`
	Password        types.String   `tfsdk:"password"`
	Login           types.Bool     `tfsdk:"login"`
	Superuser       types.Bool     `tfsdk:"superuser"`
	CreateDatabase  types.Bool     `tfsdk:"create_database"`
	CreateRole      types.Bool     `tfsdk:"create_role"`
	Replication     types.Bool     `tfsdk:"replication"`
	ConnectionLimit types.Int64    `tfsdk:"connection_limit"`
	ValidUntil      types.String   `tfsdk:"valid_until"`
	Roles           types.List     `tfsdk:"roles"`
	OID             types.Int64    `tfsdk:"oid"`
	Timeouts        timeouts.Value `tfsdk:"timeouts"`
}

func NewRoleResource() resource.Resource {
	return &roleResource{}
}

func (r *roleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *roleResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     0,
		Description: "Manages a PostgreSQL role.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the role.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 63),
				},
			},
			"password": schema.StringAttribute{
				Description: "The password for the role.",
				Optional:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"login": schema.BoolAttribute{
				Description: "Whether the role can log in. Default: false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"superuser": schema.BoolAttribute{
				Description: "Whether the role is a superuser. Default: false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"create_database": schema.BoolAttribute{
				Description: "Whether the role can create databases. Default: false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"create_role": schema.BoolAttribute{
				Description: "Whether the role can create other roles. Default: false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"replication": schema.BoolAttribute{
				Description: "Whether the role can initiate streaming replication. Default: false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"connection_limit": schema.Int64Attribute{
				Description: "Maximum number of concurrent connections the role can make. -1 means no limit. Default: -1.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(-1),
				Validators: []validator.Int64{
					int64validator.AtLeast(-1),
				},
			},
			"valid_until": schema.StringAttribute{
				Description: "Timestamp until which the role's password is valid. If omitted, the password never expires. Format: RFC 3339 (e.g. 2025-12-31T23:59:59Z).",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`),
						"must be a valid timestamp (e.g. 2025-12-31T23:59:59Z)",
					),
				},
			},
			"roles": schema.ListAttribute{
				Description: "List of roles that this role is a member of.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"oid": schema.Int64Attribute{
				Description: "The OID of the role.",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *roleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	db, err := common.ConfigureDB(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", err.Error())
		return
	}
	r.db = db
}

func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan roleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, d := plan.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	roleName := plan.Name.ValueString()
	sqlStr := fmt.Sprintf("CREATE ROLE %s", pq.QuoteIdentifier(roleName))
	sqlStr += r.buildRoleOptions(ctx, &plan)

	// Use a transaction to ensure CREATE ROLE + GRANT memberships are atomic.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error starting transaction", err.Error())
		return
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx, sqlStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating role",
			fmt.Sprintf("Could not create role %s: %s", roleName, err.Error()),
		)
		return
	}

	// Grant role memberships
	if common.IsSet(plan.Roles) {
		var roles []string
		resp.Diagnostics.Append(plan.Roles.ElementsAs(ctx, &roles, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, memberOf := range roles {
			grantSQL := fmt.Sprintf("GRANT %s TO %s",
				pq.QuoteIdentifier(memberOf),
				pq.QuoteIdentifier(roleName),
			)
			_, err := tx.ExecContext(ctx, grantSQL)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error granting role membership",
					fmt.Sprintf("Could not grant %s to %s: %s", memberOf, roleName, err.Error()),
				)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		resp.Diagnostics.AddError("Error committing transaction", err.Error())
		return
	}

	// Read back the role to populate computed attributes
	diags := r.readRole(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags := r.readRole(ctx, &state)
	if diags.HasError() {
		// If the role no longer exists, remove it from state
		for _, d := range diags {
			if d.Summary() == "Role not found" {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan roleResourceModel
	var state roleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, d := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	oldName := state.Name.ValueString()
	newName := plan.Name.ValueString()

	// Rename the role if the name changed
	if oldName != newName {
		renameSQL := fmt.Sprintf("ALTER ROLE %s RENAME TO %s",
			pq.QuoteIdentifier(oldName),
			pq.QuoteIdentifier(newName),
		)
		_, err := r.db.ExecContext(ctx, renameSQL)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error renaming role",
				fmt.Sprintf("Could not rename role %s to %s: %s", oldName, newName, err.Error()),
			)
			return
		}
	}

	// Alter role options
	alterSQL := fmt.Sprintf("ALTER ROLE %s", pq.QuoteIdentifier(newName))
	alterSQL += r.buildRoleOptions(ctx, &plan)

	_, err := r.db.ExecContext(ctx, alterSQL)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating role",
			fmt.Sprintf("Could not update role %s: %s", newName, err.Error()),
		)
		return
	}

	// Manage role memberships
	var oldRoles []string
	var newRoles []string
	if common.IsSet(state.Roles) {
		resp.Diagnostics.Append(state.Roles.ElementsAs(ctx, &oldRoles, false)...)
	}
	if common.IsSet(plan.Roles) {
		resp.Diagnostics.Append(plan.Roles.ElementsAs(ctx, &newRoles, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	toGrant, toRevoke := diffRoles(oldRoles, newRoles)

	for _, memberOf := range toGrant {
		grantSQL := fmt.Sprintf("GRANT %s TO %s",
			pq.QuoteIdentifier(memberOf),
			pq.QuoteIdentifier(newName),
		)
		_, err := r.db.ExecContext(ctx, grantSQL)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error granting role membership",
				fmt.Sprintf("Could not grant %s to %s: %s", memberOf, newName, err.Error()),
			)
			return
		}
	}

	for _, memberOf := range toRevoke {
		revokeSQL := fmt.Sprintf("REVOKE %s FROM %s",
			pq.QuoteIdentifier(memberOf),
			pq.QuoteIdentifier(newName),
		)
		_, err := r.db.ExecContext(ctx, revokeSQL)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error revoking role membership",
				fmt.Sprintf("Could not revoke %s from %s: %s", memberOf, newName, err.Error()),
			)
			return
		}
	}

	// Read back updated role
	diags := r.readRole(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, d := state.Timeouts.Delete(ctx, 5*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	roleName := state.Name.ValueString()
	sqlStr := fmt.Sprintf("DROP ROLE %s", pq.QuoteIdentifier(roleName))
	_, err := r.db.ExecContext(ctx, sqlStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting role",
			fmt.Sprintf("Could not drop role %s: %s", roleName, err.Error()),
		)
		return
	}
}

func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// buildRoleOptions constructs the WITH clause for CREATE ROLE or ALTER ROLE.
func (r *roleResource) buildRoleOptions(_ context.Context, model *roleResourceModel) string {
	var opts []string

	if model.Login.ValueBool() {
		opts = append(opts, "LOGIN")
	} else {
		opts = append(opts, "NOLOGIN")
	}

	if model.Superuser.ValueBool() {
		opts = append(opts, "SUPERUSER")
	} else {
		opts = append(opts, "NOSUPERUSER")
	}

	if model.CreateDatabase.ValueBool() {
		opts = append(opts, "CREATEDB")
	} else {
		opts = append(opts, "NOCREATEDB")
	}

	if model.CreateRole.ValueBool() {
		opts = append(opts, "CREATEROLE")
	} else {
		opts = append(opts, "NOCREATEROLE")
	}

	if model.Replication.ValueBool() {
		opts = append(opts, "REPLICATION")
	} else {
		opts = append(opts, "NOREPLICATION")
	}

	opts = append(opts, fmt.Sprintf("CONNECTION LIMIT %d", model.ConnectionLimit.ValueInt64()))

	if common.IsSet(model.Password) {
		opts = append(opts, fmt.Sprintf("PASSWORD %s", pq.QuoteLiteral(model.Password.ValueString())))
	}

	if common.IsSet(model.ValidUntil) {
		opts = append(opts, fmt.Sprintf("VALID UNTIL %s", pq.QuoteLiteral(model.ValidUntil.ValueString())))
	}

	if len(opts) == 0 {
		return ""
	}
	return " WITH " + strings.Join(opts, " ")
}

// readRole queries PostgreSQL for role attributes and populates the model.
func (r *roleResource) readRole(ctx context.Context, model *roleResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	roleName := model.Name.ValueString()

	var oid int64
	var rolCanLogin, rolSuper, rolCreateDB, rolCreateRole, rolReplication bool
	var rolConnLimit int64
	var rolValidUntil sql.NullString

	query := fmt.Sprintf(
		`SELECT oid, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolconnlimit, rolvaliduntil
		 FROM pg_catalog.pg_roles WHERE rolname = %s`,
		pq.QuoteLiteral(roleName),
	)

	err := r.db.QueryRowContext(ctx, query).Scan(
		&oid,
		&rolCanLogin,
		&rolSuper,
		&rolCreateDB,
		&rolCreateRole,
		&rolReplication,
		&rolConnLimit,
		&rolValidUntil,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			diags.AddError(
				"Role not found",
				fmt.Sprintf("Role %s does not exist.", roleName),
			)
			return diags
		}
		diags.AddError(
			"Error reading role",
			fmt.Sprintf("Could not read role %s: %s", roleName, err.Error()),
		)
		return diags
	}

	model.OID = types.Int64Value(oid)
	model.Login = types.BoolValue(rolCanLogin)
	model.Superuser = types.BoolValue(rolSuper)
	model.CreateDatabase = types.BoolValue(rolCreateDB)
	model.CreateRole = types.BoolValue(rolCreateRole)
	model.Replication = types.BoolValue(rolReplication)
	model.ConnectionLimit = types.Int64Value(rolConnLimit)

	if rolValidUntil.Valid {
		model.ValidUntil = types.StringValue(rolValidUntil.String)
	} else {
		model.ValidUntil = types.StringNull()
	}

	// Read role memberships
	memberQuery := fmt.Sprintf(
		`SELECT r.rolname
		 FROM pg_catalog.pg_auth_members m
		 JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
		 WHERE m.member = %d
		 ORDER BY r.rolname`,
		oid,
	)

	rows, err := r.db.QueryContext(ctx, memberQuery)
	if err != nil {
		diags.AddError(
			"Error reading role memberships",
			fmt.Sprintf("Could not read memberships for role %s: %s", roleName, err.Error()),
		)
		return diags
	}
	defer rows.Close() //nolint:errcheck

	var memberOfRoles []attr.Value
	for rows.Next() {
		var memberOfName string
		if err := rows.Scan(&memberOfName); err != nil {
			diags.AddError(
				"Error scanning role membership",
				fmt.Sprintf("Could not scan membership row for role %s: %s", roleName, err.Error()),
			)
			return diags
		}
		memberOfRoles = append(memberOfRoles, types.StringValue(memberOfName))
	}
	if err := rows.Err(); err != nil {
		diags.AddError(
			"Error iterating role memberships",
			fmt.Sprintf("Error iterating memberships for role %s: %s", roleName, err.Error()),
		)
		return diags
	}

	if len(memberOfRoles) > 0 {
		rolesList, listDiags := types.ListValue(types.StringType, memberOfRoles)
		diags.Append(listDiags...)
		model.Roles = rolesList
	} else if !model.Roles.IsNull() {
		// Preserve empty list if roles was explicitly set (even as [])
		model.Roles, _ = types.ListValue(types.StringType, []attr.Value{})
	} else {
		model.Roles = types.ListNull(types.StringType)
	}

	// Password is not readable from pg_roles, preserve existing state value
	// (model.Password is already set from plan or state)

	return diags
}

// diffRoles computes which roles to grant and which to revoke.
func diffRoles(oldRoles, newRoles []string) (toGrant, toRevoke []string) {
	oldSet := make(map[string]struct{}, len(oldRoles))
	for _, r := range oldRoles {
		oldSet[r] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newRoles))
	for _, r := range newRoles {
		newSet[r] = struct{}{}
	}

	for _, r := range newRoles {
		if _, exists := oldSet[r]; !exists {
			toGrant = append(toGrant, r)
		}
	}
	for _, r := range oldRoles {
		if _, exists := newSet[r]; !exists {
			toRevoke = append(toRevoke, r)
		}
	}
	return
}
