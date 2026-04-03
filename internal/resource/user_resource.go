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
			},
			"create_database": schema.BoolAttribute{
				Description: "Whether the user can create databases. Default: false.",
				Optional:    true,
				Computed:    true,
			},
			"create_role": schema.BoolAttribute{
				Description: "Whether the user can create other roles. Default: false.",
				Optional:    true,
				Computed:    true,
			},
			"replication": schema.BoolAttribute{
				Description: "Whether the user can initiate streaming replication. Default: false.",
				Optional:    true,
				Computed:    true,
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
				Description: "List of roles that this user is a member of.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"oid": schema.Int64Attribute{
				Description: "The OID of the user.",
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

	createTimeout, d := plan.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	userName := plan.Name.ValueString()
	sqlStr := fmt.Sprintf("CREATE USER %s", pq.QuoteIdentifier(userName))
	sqlStr += r.BuildUserOptions(ctx, &plan)

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error starting transaction", err.Error())
		return
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx, sqlStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating user",
			fmt.Sprintf("Could not create user %s: %s", userName, err.Error()),
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
				pq.QuoteIdentifier(userName),
			)
			_, err := tx.ExecContext(ctx, grantSQL)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error granting role membership",
					fmt.Sprintf("Could not grant %s to %s: %s", memberOf, userName, err.Error()),
				)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		resp.Diagnostics.AddError("Error committing transaction", err.Error())
		return
	}

	diags := r.ReadUser(ctx, &plan)
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

	diags := r.ReadUser(ctx, &state)
	if diags.HasError() {
		for _, d := range diags {
			if d.Summary() == "User not found" {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.Append(diags...)
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

	updateTimeout, d := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	oldName := state.Name.ValueString()
	newName := plan.Name.ValueString()

	if oldName != newName {
		renameSQL := fmt.Sprintf("ALTER USER %s RENAME TO %s",
			pq.QuoteIdentifier(oldName),
			pq.QuoteIdentifier(newName),
		)
		_, err := r.DB.ExecContext(ctx, renameSQL)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error renaming user",
				fmt.Sprintf("Could not rename user %s to %s: %s", oldName, newName, err.Error()),
			)
			return
		}
	}

	alterSQL := fmt.Sprintf("ALTER USER %s", pq.QuoteIdentifier(newName))
	alterSQL += r.BuildUserOptions(ctx, &plan)

	_, err := r.DB.ExecContext(ctx, alterSQL)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating user",
			fmt.Sprintf("Could not update user %s: %s", newName, err.Error()),
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

	toGrant, toRevoke := DiffRoles(oldRoles, newRoles)

	for _, memberOf := range toGrant {
		grantSQL := fmt.Sprintf("GRANT %s TO %s",
			pq.QuoteIdentifier(memberOf),
			pq.QuoteIdentifier(newName),
		)
		_, err := r.DB.ExecContext(ctx, grantSQL)
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
		_, err := r.DB.ExecContext(ctx, revokeSQL)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error revoking role membership",
				fmt.Sprintf("Could not revoke %s from %s: %s", memberOf, newName, err.Error()),
			)
			return
		}
	}

	diags := r.ReadUser(ctx, &plan)
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

	deleteTimeout, d := state.Timeouts.Delete(ctx, 5*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	userName := state.Name.ValueString()
	sqlStr := fmt.Sprintf("DROP USER %s", pq.QuoteIdentifier(userName))
	_, err := r.DB.ExecContext(ctx, sqlStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting user",
			fmt.Sprintf("Could not drop user %s: %s", userName, err.Error()),
		)
		return
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *UserResource) BuildUserOptions(_ context.Context, model *UserResourceModel) string {
	var opts []string

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

func (r *UserResource) ReadUser(ctx context.Context, model *UserResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	userName := model.Name.ValueString()

	var oid int64
	var rolCanLogin, rolSuper, rolCreateDB, rolCreateRole, rolReplication bool
	var rolConnLimit int64
	var rolValidUntil sql.NullString

	query := fmt.Sprintf(
		`SELECT oid, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolconnlimit, rolvaliduntil
		 FROM pg_catalog.pg_roles WHERE rolname = %s`,
		pq.QuoteLiteral(userName),
	)

	err := r.DB.QueryRowContext(ctx, query).Scan(
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
			diags.AddError("User not found", fmt.Sprintf("User %s does not exist.", userName))
			return diags
		}
		diags.AddError("Error reading user", fmt.Sprintf("Could not read user %s: %s", userName, err.Error()))
		return diags
	}

	if !rolCanLogin {
		diags.AddWarning(
			"Role is not a user",
			fmt.Sprintf("Role %s does not have LOGIN privilege. Consider using postgresql_role instead.", userName),
		)
	}

	model.OID = types.Int64Value(oid)
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

	rows, err := r.DB.QueryContext(ctx, memberQuery)
	if err != nil {
		diags.AddError(
			"Error reading role memberships",
			fmt.Sprintf("Could not read memberships for user %s: %s", userName, err.Error()),
		)
		return diags
	}
	defer rows.Close() //nolint:errcheck

	var memberOfRoles []attr.Value
	for rows.Next() {
		var memberOfName string
		if err := rows.Scan(&memberOfName); err != nil {
			diags.AddError("Error scanning role membership", fmt.Sprintf("Could not scan membership row for user %s: %s", userName, err.Error()))
			return diags
		}
		memberOfRoles = append(memberOfRoles, types.StringValue(memberOfName))
	}
	if err := rows.Err(); err != nil {
		diags.AddError("Error iterating role memberships", fmt.Sprintf("Error iterating memberships for user %s: %s", userName, err.Error()))
		return diags
	}

	if len(memberOfRoles) > 0 {
		rolesList, listDiags := types.ListValue(types.StringType, memberOfRoles)
		diags.Append(listDiags...)
		model.Roles = rolesList
	} else if !model.Roles.IsNull() {
		model.Roles, _ = types.ListValue(types.StringType, []attr.Value{})
	} else {
		model.Roles = types.ListNull(types.StringType)
	}

	return diags
}
