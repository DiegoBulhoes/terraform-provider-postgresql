package resource

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lib/pq"
)

var (
	_ resource.Resource                = (*RoleResource)(nil)
	_ resource.ResourceWithImportState = (*RoleResource)(nil)
)

type RoleResource struct {
	DB common.DBTX
}

type PrivilegeModel struct {
	Privileges types.Set    `tfsdk:"privileges"`
	ObjectType types.String `tfsdk:"object_type"`
	Schema     types.String `tfsdk:"schema"`
	Database   types.String `tfsdk:"database"`
	Objects    types.List   `tfsdk:"objects"`
}

type RoleResourceModel struct {
	Name            types.String     `tfsdk:"name"`
	Superuser       types.Bool       `tfsdk:"superuser"`
	CreateDatabase  types.Bool       `tfsdk:"create_database"`
	CreateRole      types.Bool       `tfsdk:"create_role"`
	Replication     types.Bool       `tfsdk:"replication"`
	ConnectionLimit types.Int64      `tfsdk:"connection_limit"`
	Privileges      []PrivilegeModel `tfsdk:"privilege"`
	OID             types.Int64      `tfsdk:"oid"`
	Timeouts        timeouts.Value   `tfsdk:"timeouts"`
}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

func (r *RoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     1,
		Description: "Manages a PostgreSQL role (permission group). Use postgresql_user for login users.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the role.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 63),
				},
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
			"oid": schema.Int64Attribute{
				Description: "The OID of the role.",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"privilege": schema.ListNestedBlock{
				Description: "Privilege grants for this role. Each block defines a set of privileges on a specific object type.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"privileges": schema.SetAttribute{
							Description: "The set of privileges to grant (e.g. SELECT, INSERT, UPDATE, DELETE, USAGE, CREATE, ALL).",
							Required:    true,
							ElementType: types.StringType,
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
								setvalidator.ValueStringsAre(
									stringvalidator.LengthAtLeast(1),
								),
							},
						},
						"object_type": schema.StringAttribute{
							Description: "The object type to grant privileges on: database, schema, table, sequence, or function.",
							Required:    true,
							Validators: []validator.String{
								stringvalidator.OneOf("database", "schema", "table", "sequence", "function"),
							},
						},
						"schema": schema.StringAttribute{
							Description: "The schema containing the objects.",
							Optional:    true,
						},
						"database": schema.StringAttribute{
							Description: "The database for database-level grants.",
							Optional:    true,
						},
						"objects": schema.ListAttribute{
							Description: "Specific object names. If empty, grants on ALL objects of the given type in the schema.",
							Optional:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *RoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RoleResourceModel
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
	sqlStr += r.BuildRoleOptions(ctx, &plan)

	tx, err := r.DB.BeginTx(ctx, nil)
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

	// Grant inline privileges
	resp.Diagnostics.Append(r.GrantPrivileges(ctx, tx, roleName, plan.Privileges)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := tx.Commit(); err != nil {
		resp.Diagnostics.AddError("Error committing transaction", err.Error())
		return
	}

	diags := r.ReadRole(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags := r.ReadRole(ctx, &state)
	if diags.HasError() {
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

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RoleResourceModel
	var state RoleResourceModel
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
		renameSQL := fmt.Sprintf("ALTER ROLE %s RENAME TO %s",
			pq.QuoteIdentifier(oldName),
			pq.QuoteIdentifier(newName),
		)
		_, err := r.DB.ExecContext(ctx, renameSQL)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error renaming role",
				fmt.Sprintf("Could not rename role %s to %s: %s", oldName, newName, err.Error()),
			)
			return
		}
	}

	alterSQL := fmt.Sprintf("ALTER ROLE %s", pq.QuoteIdentifier(newName))
	alterSQL += r.BuildRoleOptions(ctx, &plan)

	_, err := r.DB.ExecContext(ctx, alterSQL)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating role",
			fmt.Sprintf("Could not update role %s: %s", newName, err.Error()),
		)
		return
	}

	// Revoke old privileges, then grant new ones
	resp.Diagnostics.Append(r.RevokePrivileges(ctx, r.DB, newName, state.Privileges)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.GrantPrivileges(ctx, r.DB, newName, plan.Privileges)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags := r.ReadRole(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RoleResourceModel
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

	// Revoke privileges before dropping the role
	resp.Diagnostics.Append(r.RevokePrivileges(ctx, r.DB, roleName, state.Privileges)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sqlStr := fmt.Sprintf("DROP ROLE %s", pq.QuoteIdentifier(roleName))
	_, err := r.DB.ExecContext(ctx, sqlStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting role",
			fmt.Sprintf("Could not drop role %s: %s", roleName, err.Error()),
		)
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *RoleResource) BuildRoleOptions(_ context.Context, model *RoleResourceModel) string {
	var opts []string

	opts = append(opts, "NOLOGIN")

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

	if len(opts) == 0 {
		return ""
	}
	return " WITH " + strings.Join(opts, " ")
}

func (r *RoleResource) ReadRole(ctx context.Context, model *RoleResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	roleName := model.Name.ValueString()

	var oid int64
	var rolSuper, rolCreateDB, rolCreateRole, rolReplication bool
	var rolConnLimit int64

	query := fmt.Sprintf(
		`SELECT oid, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolconnlimit
		 FROM pg_catalog.pg_roles WHERE rolname = %s`,
		pq.QuoteLiteral(roleName),
	)

	err := r.DB.QueryRowContext(ctx, query).Scan(
		&oid,
		&rolSuper,
		&rolCreateDB,
		&rolCreateRole,
		&rolReplication,
		&rolConnLimit,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			diags.AddError("Role not found", fmt.Sprintf("Role %s does not exist.", roleName))
			return diags
		}
		diags.AddError("Error reading role", fmt.Sprintf("Could not read role %s: %s", roleName, err.Error()))
		return diags
	}

	model.OID = types.Int64Value(oid)
	model.Superuser = types.BoolValue(rolSuper)
	model.CreateDatabase = types.BoolValue(rolCreateDB)
	model.CreateRole = types.BoolValue(rolCreateRole)
	model.Replication = types.BoolValue(rolReplication)
	model.ConnectionLimit = types.Int64Value(rolConnLimit)

	// Privileges are write-only from the Terraform perspective.
	// We preserve the plan/state value since reading all grants back
	// from PostgreSQL catalogs for every object type is complex and
	// the privileges are already tracked in state.

	return diags
}

// grantPrivileges executes GRANT statements for the given privilege blocks.
func (r *RoleResource) GrantPrivileges(ctx context.Context, exec common.ExecContext, roleName string, privileges []PrivilegeModel) diag.Diagnostics {
	var diags diag.Diagnostics

	for _, priv := range privileges {
		privSlice := common.StringSetToSlice(ctx, priv.Privileges)
		privList := strings.Join(privSlice, ", ")
		objectType := strings.ToLower(priv.ObjectType.ValueString())
		database := priv.Database.ValueString()
		schemaName := priv.Schema.ValueString()

		var objects []string
		if common.IsSet(priv.Objects) {
			objects = common.StringListToSlice(ctx, priv.Objects)
		}

		statements := BuildGrantStatements(privList, objectType, database, schemaName, roleName, objects, "")
		for _, stmt := range statements {
			tflog.Debug(ctx, "Executing GRANT for role privilege", map[string]interface{}{"sql": stmt})
			_, err := exec.ExecContext(ctx, stmt)
			if err != nil {
				diags.AddError("Error granting privilege",
					fmt.Sprintf("SQL: %s\nError: %s", stmt, err.Error()))
				return diags
			}
		}
	}

	return diags
}

// revokePrivileges executes REVOKE statements for the given privilege blocks.
func (r *RoleResource) RevokePrivileges(ctx context.Context, exec common.ExecContext, roleName string, privileges []PrivilegeModel) diag.Diagnostics {
	var diags diag.Diagnostics

	for _, priv := range privileges {
		objectType := strings.ToLower(priv.ObjectType.ValueString())
		database := priv.Database.ValueString()
		schemaName := priv.Schema.ValueString()

		var objects []string
		if common.IsSet(priv.Objects) {
			objects = common.StringListToSlice(ctx, priv.Objects)
		}

		statements := BuildRevokeStatements(objectType, database, schemaName, roleName, objects)
		for _, stmt := range statements {
			tflog.Debug(ctx, "Executing REVOKE for role privilege", map[string]interface{}{"sql": stmt})
			_, err := exec.ExecContext(ctx, stmt)
			if err != nil {
				diags.AddError("Error revoking privilege",
					fmt.Sprintf("SQL: %s\nError: %s", stmt, err.Error()))
				return diags
			}
		}
	}

	return diags
}

// DiffRoles computes which roles to grant and which to revoke.
func DiffRoles(oldRoles, newRoles []string) (toGrant, toRevoke []string) {
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
