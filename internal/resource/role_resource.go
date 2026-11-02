package resource

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

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
	ReassignOwnedTo types.String     `tfsdk:"reassign_owned_to"`
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
			"reassign_owned_to": schema.StringAttribute{
				Description: "On destroy, reassign the objects the role owns in the provider's database to this role and drop its privileges there (REASSIGN OWNED, DROP OWNED). Objects in other databases still block the drop.",
				Optional:    true,
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
							Description: "The set of privileges to grant (e.g. SELECT, INSERT, UPDATE, DELETE, USAGE, CREATE, ALL). Case-insensitive.",
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
							Description: "The database of the grant. Schema, table, sequence and function privileges run in this database, which defaults to the provider's database.",
							Optional:    true,
						},
						"objects": schema.ListAttribute{
							Description: "Specific object names; a function is named alone (all overloads) or with its argument types, as in f(integer). If empty, grants on ALL objects of the given type in the schema.",
							Optional:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Read:   true,
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

	createTimeout, d := plan.Timeouts.Create(ctx, common.DefaultTimeout)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	roleName := plan.Name.ValueString()
	grants, err := r.PrivilegeStatements(ctx, nil, roleName, plan.Privileges, true)
	if err != nil {
		resp.Diagnostics.AddError("Error preparing privileges", err.Error())
		return
	}
	main, others := r.splitByConn(ctx, grants)
	main = append([]string{fmt.Sprintf("CREATE ROLE %s", pq.QuoteIdentifier(roleName)) + r.BuildRoleOptions(ctx, &plan, nil)}, main...)
	if err := common.ExecInTx(ctx, r.DB, main); err != nil {
		resp.Diagnostics.AddError("Error creating role", fmt.Sprintf("Could not create role %s: %s", roleName, err.Error()))
		return
	}

	_, diags := r.ReadRole(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Saved before the grants in other databases: if one fails, the role is tracked and replaced.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err := runGroups(ctx, others); err != nil {
		resp.Diagnostics.AddError("Error granting privileges", err.Error())
	}
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RoleResourceModel
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

	found, diags := r.ReadRole(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(r.ReadPrivilegeBlocks(ctx, state.Name.ValueString(), state.Privileges)...)
	if resp.Diagnostics.HasError() {
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

	updateTimeout, d := plan.Timeouts.Update(ctx, common.DefaultTimeout)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	oldName := state.Name.ValueString()
	newName := plan.Name.ValueString()

	// Old privileges are revoked and new ones granted; every statement is built before any runs.
	groups, err := r.PrivilegeStatements(ctx, nil, newName, state.Privileges, false)
	if err == nil {
		groups, err = r.PrivilegeStatements(ctx, groups, newName, plan.Privileges, true)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error preparing privileges", err.Error())
		return
	}
	main, others := r.splitByConn(ctx, groups)

	var head []string
	if oldName != newName {
		head = append(head, fmt.Sprintf("ALTER ROLE %s RENAME TO %s", pq.QuoteIdentifier(oldName), pq.QuoteIdentifier(newName)))
	}
	head = append(head, fmt.Sprintf("ALTER ROLE %s", pq.QuoteIdentifier(newName))+r.BuildRoleOptions(ctx, &plan, &state))
	if err := common.ExecInTx(ctx, r.DB, append(head, main...)); err != nil {
		resp.Diagnostics.AddError("Error updating role", fmt.Sprintf("Could not update role %s: %s", oldName, err.Error()))
		return
	}
	if err := runGroups(ctx, others); err != nil {
		resp.Diagnostics.AddError("Error updating privileges", err.Error())
		return
	}

	_, diags := r.ReadRole(ctx, &plan)
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

	deleteTimeout, d := state.Timeouts.Delete(ctx, common.DefaultTimeout)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	roleName := state.Name.ValueString()

	revokes, err := r.PrivilegeStatements(ctx, nil, roleName, state.Privileges, false)
	if err != nil {
		resp.Diagnostics.AddError("Error preparing privileges", err.Error())
		return
	}
	for _, g := range revokes {
		for _, stmt := range g.Statements {
			tflog.Debug(ctx, "Executing REVOKE for role privilege", map[string]interface{}{"sql": stmt})
			_, err := g.Conn.ExecContext(ctx, stmt)
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && goneCodes[pqErr.Code] {
				continue
			}
			if err != nil {
				resp.Diagnostics.AddError("Error revoking privilege", fmt.Sprintf("SQL: %s\nError: %s", stmt, err.Error()))
				return
			}
		}
	}

	statements := append(ReassignOwned(roleName, state.ReassignOwnedTo), fmt.Sprintf("DROP ROLE %s", pq.QuoteIdentifier(roleName)))
	if err := common.ExecInTx(ctx, r.DB, statements); err != nil {
		resp.Diagnostics.AddError("Error deleting role", DropRoleError(roleName, err))
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// BuildRoleOptions renders the WITH clause; state is nil on create, and flags are sent only when they change.
func (r *RoleResource) BuildRoleOptions(_ context.Context, plan, state *RoleResourceModel) string {
	var prev *roleAttributes
	if state != nil {
		attrs := state.attributes()
		prev = &attrs
	}
	opts := plan.attributes().options(prev)
	if state == nil {
		noLogin := false
		opts.Login = &noLogin
	}
	return common.BuildRoleOptions(opts)
}

// ReadRole reads the role into model; found is false when it does not exist.
func (r *RoleResource) ReadRole(ctx context.Context, model *RoleResourceModel) (found bool, diags diag.Diagnostics) {
	roleName := model.Name.ValueString()

	row, found, err := readRoleRow(ctx, r.DB, roleName)
	if err != nil {
		diags.AddError("Error reading role", fmt.Errorf("read role %s: %w", roleName, err).Error())
		return true, diags
	}
	if !found {
		return false, diags
	}
	model.attributes().set(row)
	return true, diags
}

// ReadPrivilegeBlocks replaces the privileges of each block with what the
// catalog holds, when that differs from the block.
func (r *RoleResource) ReadPrivilegeBlocks(ctx context.Context, roleName string, blocks []PrivilegeModel) diag.Diagnostics {
	var diags diag.Diagnostics
	for i := range blocks {
		b := &blocks[i]
		objectType := strings.ToLower(b.ObjectType.ValueString())
		conn, err := GrantConn(ctx, r.DB, objectType, b.Database.ValueString())
		if err != nil && !isGone(err) {
			diags.AddError("Error connecting to database", err.Error())
			return diags
		}
		var granted GrantedPrivileges
		if err == nil {
			granted, err = ReadGrantedPrivileges(ctx, conn, roleName, objectType, b.Database.ValueString(),
				b.Schema.ValueString(), common.StringListToSlice(ctx, b.Objects))
			if err != nil {
				diags.AddError("Error reading privileges", err.Error())
				return diags
			}
		}
		if granted.TargetExists && granted.Objects == 0 {
			continue
		}
		if granted.TargetExists {
			full, err := FullPrivileges(ctx, conn, objectType)
			if err != nil {
				diags.AddError("Error reading privileges", err.Error())
				return diags
			}
			if slices.Equal(ExpandPrivileges(common.StringSetToSlice(ctx, b.Privileges), full), granted.Privileges) {
				continue
			}
		}
		privileges := granted.Privileges
		if privileges == nil {
			privileges = []string{}
		}
		set, d := types.SetValueFrom(ctx, types.StringType, privileges)
		diags.Append(d...)
		b.Privileges = set
	}
	return diags
}

// ConnStatements are statements that run on one connection.
type ConnStatements struct {
	Conn       common.DBTX
	Statements []string
}

// PrivilegeStatements appends the GRANT (grant) or REVOKE statements of the
// blocks to groups, one group per connection. GRANTs need every function to
// exist; REVOKEs skip the ones that are gone.
func (r *RoleResource) PrivilegeStatements(ctx context.Context, groups []ConnStatements, roleName string, blocks []PrivilegeModel, grant bool) ([]ConnStatements, error) {
	for _, b := range blocks {
		objectType := strings.ToLower(b.ObjectType.ValueString())
		database := b.Database.ValueString()
		schemaName := b.Schema.ValueString()
		conn, err := GrantConn(ctx, r.DB, objectType, database)
		if !grant && isGone(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		objects := common.StringListToSlice(ctx, b.Objects)

		var statements []string
		if grant {
			privileges, err := common.NormalizePrivileges(common.StringSetToSlice(ctx, b.Privileges))
			if err != nil {
				return nil, err
			}
			if objects, err = GrantObjects(ctx, conn, objectType, schemaName, objects); err != nil {
				return nil, err
			}
			statements = BuildGrantStatements(strings.Join(privileges, ", "), objectType, database, schemaName, roleName, objects, "")
		} else {
			if objectType == "function" && len(objects) > 0 {
				if objects, _, err = ResolveFunctions(ctx, conn, schemaName, objects); err != nil {
					return nil, err
				}
				if len(objects) == 0 {
					continue
				}
			}
			statements = BuildRevokeStatements(objectType, database, schemaName, roleName, objects)
		}

		i := slices.IndexFunc(groups, func(g ConnStatements) bool { return g.Conn == conn })
		if i < 0 {
			groups = append(groups, ConnStatements{Conn: conn})
			i = len(groups) - 1
		}
		groups[i].Statements = append(groups[i].Statements, statements...)
	}
	return groups, nil
}

// splitByConn returns the statements of the provider's connection and the groups of the others.
func (r *RoleResource) splitByConn(ctx context.Context, groups []ConnStatements) ([]string, []ConnStatements) {
	var main []string
	var others []ConnStatements
	for _, g := range groups {
		if g.Conn == r.DB {
			main = append(main, g.Statements...)
		} else {
			others = append(others, g)
		}
	}
	logStatements(ctx, main)
	return main, others
}

func runGroups(ctx context.Context, groups []ConnStatements) error {
	for _, g := range groups {
		logStatements(ctx, g.Statements)
		if err := common.ExecInTx(ctx, g.Conn, g.Statements); err != nil {
			return err
		}
	}
	return nil
}

// DiffRoles computes which roles to grant and which to revoke. The returned
// slices are deduplicated even if the inputs contain repeated role names.
func DiffRoles(oldRoles, newRoles []string) (toGrant, toRevoke []string) {
	oldSet := make(map[string]struct{}, len(oldRoles))
	for _, r := range oldRoles {
		oldSet[r] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newRoles))
	for _, r := range newRoles {
		newSet[r] = struct{}{}
	}

	emitted := make(map[string]struct{}, len(newRoles))
	for _, r := range newRoles {
		if _, exists := oldSet[r]; exists {
			continue
		}
		if _, dup := emitted[r]; dup {
			continue
		}
		emitted[r] = struct{}{}
		toGrant = append(toGrant, r)
	}
	emitted = make(map[string]struct{}, len(oldRoles))
	for _, r := range oldRoles {
		if _, exists := newSet[r]; exists {
			continue
		}
		if _, dup := emitted[r]; dup {
			continue
		}
		emitted[r] = struct{}{}
		toRevoke = append(toRevoke, r)
	}
	return
}
