package resource

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lib/pq"
)

var (
	_ resource.Resource                   = (*DefaultPrivilegesResource)(nil)
	_ resource.ResourceWithImportState    = (*DefaultPrivilegesResource)(nil)
	_ resource.ResourceWithValidateConfig = (*DefaultPrivilegesResource)(nil)
)

type DefaultPrivilegesResource struct {
	DB common.DBTX
}

type DefaultPrivilegesResourceModel struct {
	ID              types.String   `tfsdk:"id"`
	Role            types.String   `tfsdk:"role"`
	Owner           types.String   `tfsdk:"owner"`
	Database        types.String   `tfsdk:"database"`
	Schema          types.String   `tfsdk:"schema"`
	ObjectType      types.String   `tfsdk:"object_type"`
	Privileges      types.Set      `tfsdk:"privileges"`
	WithGrantOption types.Bool     `tfsdk:"with_grant_option"`
	Timeouts        timeouts.Value `tfsdk:"timeouts"`
}

// defaultObjectTypes maps an object_type to its ALTER DEFAULT PRIVILEGES keyword and pg_default_acl.defaclobjtype.
var defaultObjectTypes = map[string]struct{ keyword, catalog string }{
	"table":    {"TABLES", "r"},
	"sequence": {"SEQUENCES", "S"},
	"function": {"FUNCTIONS", "f"},
	"type":     {"TYPES", "T"},
	"schema":   {"SCHEMAS", "n"},
}

func NewDefaultPrivilegesResource() resource.Resource {
	return &DefaultPrivilegesResource{}
}

func (r *DefaultPrivilegesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_default_privileges"
}

func (r *DefaultPrivilegesResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     0,
		Description: "Manages PostgreSQL default privileges: the privileges a role gets on objects another role creates in the future.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier: {owner}/{role}/{object_type}/{database}/{schema}.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"role": schema.StringAttribute{
				Description: "The role that receives the privileges.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"owner": schema.StringAttribute{
				Description: "The role whose future objects get the privileges (ALTER DEFAULT PRIVILEGES FOR ROLE).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database": schema.StringAttribute{
				Description: "The database the default privileges are set in. Defaults to the provider's database.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema": schema.StringAttribute{
				Description: "The schema the default privileges apply in. Without it, they apply to objects the owner creates in any schema.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"object_type": schema.StringAttribute{
				Description: "The type of future objects: table, sequence, function, type, or schema. schema is allowed only without schema.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("table", "sequence", "function", "type", "schema"),
				},
			},
			"privileges": schema.SetAttribute{
				Description: "The privileges to grant (e.g. SELECT, INSERT, USAGE, EXECUTE). Case-insensitive; ALL stands for every privilege of the object type.",
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(
						stringvalidator.LengthAtLeast(1),
					),
				},
			},
			"with_grant_option": schema.BoolAttribute{
				Description: "Whether the role can grant the same privileges to others.",
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

func (r *DefaultPrivilegesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DefaultPrivilegesResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg DefaultPrivilegesResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.ObjectType.ValueString() == "schema" && !cfg.Schema.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("schema"), "Unexpected schema",
			"object_type schema sets defaults for new schemas, so it is allowed only without schema.")
	}
	if common.IsSet(cfg.Privileges) {
		if _, err := common.NormalizePrivileges(common.StringSetToSlice(ctx, cfg.Privileges)); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("privileges"), "Invalid privileges", err.Error())
		}
	}
}

// BuildDefaultPrivilegesGrant returns the ALTER DEFAULT PRIVILEGES ... GRANT statement.
func BuildDefaultPrivilegesGrant(owner, schemaName, objectType, role string, privileges []string, withGrantOption bool) string {
	stmt := fmt.Sprintf("%s GRANT %s ON %s TO %s", defaultPrivilegesPrefix(owner, schemaName),
		strings.Join(privileges, ", "), defaultObjectTypes[objectType].keyword, pq.QuoteIdentifier(role))
	if withGrantOption {
		stmt += " WITH GRANT OPTION"
	}
	return stmt
}

// BuildDefaultPrivilegesRevoke returns the ALTER DEFAULT PRIVILEGES ... REVOKE ALL statement.
func BuildDefaultPrivilegesRevoke(owner, schemaName, objectType, role string) string {
	return fmt.Sprintf("%s REVOKE ALL ON %s FROM %s", defaultPrivilegesPrefix(owner, schemaName),
		defaultObjectTypes[objectType].keyword, pq.QuoteIdentifier(role))
}

func defaultPrivilegesPrefix(owner, schemaName string) string {
	prefix := "ALTER DEFAULT PRIVILEGES FOR ROLE " + pq.QuoteIdentifier(owner)
	if schemaName != "" {
		prefix += " IN SCHEMA " + pq.QuoteIdentifier(schemaName)
	}
	return prefix
}

func defaultPrivilegesID(m *DefaultPrivilegesResourceModel) string {
	return strings.Join([]string{m.Owner.ValueString(), m.Role.ValueString(), m.ObjectType.ValueString(),
		m.Database.ValueString(), m.Schema.ValueString()}, "/")
}

// grantStatement validates the plan, then connects: nothing reaches the server with an invalid privilege.
func (r *DefaultPrivilegesResource) grantStatement(ctx context.Context, plan *DefaultPrivilegesResourceModel) (common.DBTX, string, error) {
	privileges, err := common.NormalizePrivileges(common.StringSetToSlice(ctx, plan.Privileges))
	if err != nil {
		return nil, "", err
	}
	conn, err := common.OnDatabase(ctx, r.DB, plan.Database.ValueString())
	if err != nil {
		return nil, "", err
	}
	return conn, BuildDefaultPrivilegesGrant(plan.Owner.ValueString(), plan.Schema.ValueString(),
		plan.ObjectType.ValueString(), plan.Role.ValueString(), privileges, plan.WithGrantOption.ValueBool()), nil
}

func (r *DefaultPrivilegesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DefaultPrivilegesResourceModel
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

	conn, grant, err := r.grantStatement(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Error preparing ALTER DEFAULT PRIVILEGES", err.Error())
		return
	}
	logStatements(ctx, []string{grant})
	if err := common.ExecInTx(ctx, conn, []string{grant}); err != nil {
		resp.Diagnostics.AddError("Error executing ALTER DEFAULT PRIVILEGES", err.Error())
		return
	}

	plan.ID = types.StringValue(defaultPrivilegesID(&plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// DefaultPrivileges is what a role gets by default on the future objects of an owner.
type DefaultPrivileges struct {
	// Privileges held, sorted.
	Privileges []string
	// Grantable is true when every one of Privileges is held WITH GRANT OPTION.
	Grantable bool
	// TargetExists is false when the owner, the role or the schema is gone.
	TargetExists bool
}

const defaultTargetQuery = `
	SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = $1)
	   AND EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = $2)
	   AND ($3 = '' OR EXISTS (SELECT 1 FROM pg_catalog.pg_namespace WHERE nspname = $3))`

// Built-in defaults have no row here; only the grants ALTER DEFAULT PRIVILEGES added do.
const defaultACLQuery = `
	SELECT a.privilege_type, a.is_grantable
	FROM pg_catalog.pg_default_acl d
	CROSS JOIN LATERAL aclexplode(d.defaclacl) a
	WHERE d.defaclrole = (SELECT oid FROM pg_catalog.pg_roles WHERE rolname = $1)
	  AND d.defaclnamespace = CASE WHEN $2 = '' THEN 0
	        ELSE (SELECT oid FROM pg_catalog.pg_namespace WHERE nspname = $2) END
	  AND d.defaclobjtype = $3
	  AND a.grantee = (SELECT oid FROM pg_catalog.pg_roles WHERE rolname = $4)`

// ReadDefaultPrivileges reads from pg_default_acl what role gets on the future objects of owner.
func ReadDefaultPrivileges(ctx context.Context, db common.DBTX, owner, role, objectType, schemaName string) (DefaultPrivileges, error) {
	t, ok := defaultObjectTypes[objectType]
	if !ok {
		return DefaultPrivileges{}, fmt.Errorf("unsupported object type: %s", objectType)
	}
	var exists bool
	if err := db.QueryRowContext(ctx, defaultTargetQuery, owner, role, schemaName).Scan(&exists); err != nil || !exists {
		return DefaultPrivileges{}, err
	}

	rows, err := db.QueryContext(ctx, defaultACLQuery, owner, schemaName, t.catalog, role)
	if err != nil {
		return DefaultPrivileges{}, err
	}
	defer rows.Close() //nolint:errcheck

	held := map[string]bool{}
	for rows.Next() {
		var privilege string
		var grantable bool
		if err := rows.Scan(&privilege, &grantable); err != nil {
			return DefaultPrivileges{}, err
		}
		held[privilege] = held[privilege] || grantable
	}
	if err := rows.Err(); err != nil {
		return DefaultPrivileges{}, err
	}

	result := DefaultPrivileges{TargetExists: true, Grantable: len(held) > 0}
	for p, g := range held {
		result.Privileges = append(result.Privileges, p)
		result.Grantable = result.Grantable && g
	}
	sort.Strings(result.Privileges)
	return result, nil
}

// DefaultFullPrivileges lists what ALL grants for an object type on this server.
func DefaultFullPrivileges(ctx context.Context, db common.DBTX, objectType string) ([]string, error) {
	if objectType == "type" {
		return []string{"USAGE"}, nil
	}
	return FullPrivileges(ctx, db, objectType)
}

func (r *DefaultPrivilegesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DefaultPrivilegesResourceModel
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

	conn, err := common.OnDatabase(ctx, r.DB, state.Database.ValueString())
	if isGone(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to database", err.Error())
		return
	}

	objectType := state.ObjectType.ValueString()
	held, err := ReadDefaultPrivileges(ctx, conn, state.Owner.ValueString(), state.Role.ValueString(),
		objectType, state.Schema.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading default privileges", err.Error())
		return
	}
	// An import of defaults that hold nothing has nothing to import.
	if !held.TargetExists || (state.Privileges.IsNull() && len(held.Privileges) == 0) {
		tflog.Warn(ctx, "Default privileges no longer exist, removing from state", map[string]interface{}{
			"owner": state.Owner.ValueString(),
			"role":  state.Role.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}

	full, err := DefaultFullPrivileges(ctx, conn, objectType)
	if err != nil {
		resp.Diagnostics.AddError("Error reading default privileges", err.Error())
		return
	}
	desired := ExpandPrivileges(common.StringSetToSlice(ctx, state.Privileges), full)
	if state.Privileges.IsNull() || !slices.Equal(desired, held.Privileges) {
		privileges := make([]attr.Value, 0, len(held.Privileges))
		for _, p := range held.Privileges {
			privileges = append(privileges, types.StringValue(p))
		}
		set, diags := types.SetValue(types.StringType, privileges)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Privileges = set
	}
	if len(held.Privileges) > 0 || state.WithGrantOption.IsNull() {
		state.WithGrantOption = types.BoolValue(held.Grantable)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DefaultPrivilegesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DefaultPrivilegesResourceModel
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

	conn, grant, err := r.grantStatement(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Error preparing ALTER DEFAULT PRIVILEGES", err.Error())
		return
	}
	statements := []string{
		BuildDefaultPrivilegesRevoke(plan.Owner.ValueString(), plan.Schema.ValueString(),
			plan.ObjectType.ValueString(), plan.Role.ValueString()),
		grant,
	}
	logStatements(ctx, statements)
	if err := common.ExecInTx(ctx, conn, statements); err != nil {
		resp.Diagnostics.AddError("Error updating default privileges", err.Error())
		return
	}

	plan.ID = types.StringValue(defaultPrivilegesID(&plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DefaultPrivilegesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DefaultPrivilegesResourceModel
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

	conn, err := common.OnDatabase(ctx, r.DB, state.Database.ValueString())
	if isGone(err) {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to database", err.Error())
		return
	}

	revoke := BuildDefaultPrivilegesRevoke(state.Owner.ValueString(), state.Schema.ValueString(),
		state.ObjectType.ValueString(), state.Role.ValueString())
	logStatements(ctx, []string{revoke})
	_, err = conn.ExecContext(ctx, revoke)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && goneCodes[pqErr.Code] {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error executing ALTER DEFAULT PRIVILEGES", err.Error())
	}
}

func (r *DefaultPrivilegesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: owner/role/object_type/database[/schema]
	parts := strings.Split(req.ID, "/")
	if len(parts) < 4 || len(parts) > 5 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected format: owner/role/object_type/database[/schema]. Got: %s", req.ID),
		)
		return
	}
	objectType := strings.ToLower(parts[2])
	if _, ok := defaultObjectTypes[objectType]; !ok {
		resp.Diagnostics.AddError("Invalid Import ID",
			fmt.Sprintf("object_type must be table, sequence, function, type or schema. Got: %s", parts[2]))
		return
	}

	model := DefaultPrivilegesResourceModel{
		Owner:           types.StringValue(parts[0]),
		Role:            types.StringValue(parts[1]),
		ObjectType:      types.StringValue(objectType),
		Database:        types.StringNull(),
		Schema:          types.StringNull(),
		Privileges:      types.SetNull(types.StringType),
		WithGrantOption: types.BoolValue(false),
		Timeouts:        timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{"create": types.StringType, "read": types.StringType, "update": types.StringType, "delete": types.StringType})},
	}
	if parts[3] != "" {
		model.Database = types.StringValue(parts[3])
	}
	if len(parts) == 5 && parts[4] != "" {
		model.Schema = types.StringValue(parts[4])
	}
	model.ID = types.StringValue(defaultPrivilegesID(&model))
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
