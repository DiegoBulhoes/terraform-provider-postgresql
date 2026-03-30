package resource

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lib/pq"
)

var (
	_ resource.Resource              = (*defaultPrivilegesResource)(nil)
	_ resource.ResourceWithConfigure = (*defaultPrivilegesResource)(nil)
)

// defaultPrivObjTypeChars maps object type names to pg_default_acl.defaclobjtype characters.
var defaultPrivObjTypeChars = map[string]string{
	"table":    "r",
	"sequence": "S",
	"function": "f",
	"type":     "T",
}

var objectTypePlural = map[string]string{
	"table":    "TABLES",
	"sequence": "SEQUENCES",
	"function": "FUNCTIONS",
	"type":     "TYPES",
}

type defaultPrivilegesResource struct {
	db *sql.DB
}

type defaultPrivilegesResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Owner      types.String `tfsdk:"owner"`
	Role       types.String `tfsdk:"role"`
	Database   types.String `tfsdk:"database"`
	Schema     types.String `tfsdk:"schema"`
	ObjectType types.String `tfsdk:"object_type"`
	Privileges types.Set    `tfsdk:"privileges"`
}

func NewDefaultPrivilegesResource() resource.Resource {
	return &defaultPrivilegesResource{}
}

func (r *defaultPrivilegesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_default_privileges"
}

func (r *defaultPrivilegesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages PostgreSQL default privileges using ALTER DEFAULT PRIVILEGES.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier: {owner}_{role}_{database}_{schema}_{object_type}.",
				Computed:    true,
			},
			"owner": schema.StringAttribute{
				Description: "Role that owns future objects.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Description: "Grantee role that will receive the default privileges.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database": schema.StringAttribute{
				Description: "Target database where default privileges are applied.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema": schema.StringAttribute{
				Description: "Target schema where default privileges are applied. If omitted, defaults apply database-wide.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"object_type": schema.StringAttribute{
				Description: "Object type for which default privileges are set. Valid values: table, sequence, function, type.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("table", "sequence", "function", "type"),
				},
			},
			"privileges": schema.SetAttribute{
				Description: "Set of privileges to grant as default privileges.",
				Required:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *defaultPrivilegesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *defaultPrivilegesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data defaultPrivilegesResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	privileges := common.PrivilegesToSlice(ctx, data.Privileges)

	plural := objectTypePlural[data.ObjectType.ValueString()]
	query := r.buildGrantSQL(data.Owner.ValueString(), data.Role.ValueString(), data.Schema, privileges, plural)

	tflog.Debug(ctx, "Executing default privileges grant", map[string]interface{}{"query": query})

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		resp.Diagnostics.AddError("Error granting default privileges", err.Error())
		return
	}

	data.ID = types.StringValue(r.compositeID(data))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *defaultPrivilegesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data defaultPrivilegesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	owner := data.Owner.ValueString()
	role := data.Role.ValueString()
	objectType := data.ObjectType.ValueString()
	objTypeChar := defaultPrivObjTypeChars[objectType]

	var query string
	var args []interface{}

	if common.IsSet(data.Schema) {
		query = `
			SELECT privilege_type, is_grantable
			FROM (
				SELECT (aclexplode(defaclacl)).grantee,
				       (aclexplode(defaclacl)).privilege_type,
				       (aclexplode(defaclacl)).is_grantable
				FROM pg_default_acl da
				JOIN pg_namespace n ON da.defaclnamespace = n.oid
				WHERE da.defaclrole = (SELECT oid FROM pg_roles WHERE rolname = $1)
				  AND da.defaclobjtype = $2
				  AND n.nspname = $3
			) AS acl
			JOIN pg_roles ON acl.grantee = pg_roles.oid
			WHERE pg_roles.rolname = $4
		`
		args = []interface{}{owner, objTypeChar, data.Schema.ValueString(), role}
	} else {
		query = `
			SELECT privilege_type, is_grantable
			FROM (
				SELECT (aclexplode(defaclacl)).grantee,
				       (aclexplode(defaclacl)).privilege_type,
				       (aclexplode(defaclacl)).is_grantable
				FROM pg_default_acl da
				WHERE da.defaclrole = (SELECT oid FROM pg_roles WHERE rolname = $1)
				  AND da.defaclobjtype = $2
				  AND da.defaclnamespace = 0
			) AS acl
			JOIN pg_roles ON acl.grantee = pg_roles.oid
			WHERE pg_roles.rolname = $3
		`
		args = []interface{}{owner, objTypeChar, role}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		resp.Diagnostics.AddError("Error reading default privileges", err.Error())
		return
	}
	defer rows.Close()

	var privileges []string
	hasRows := false

	for rows.Next() {
		hasRows = true
		var privType string
		var isGrantable bool
		if err := rows.Scan(&privType, &isGrantable); err != nil {
			resp.Diagnostics.AddError("Error scanning default privileges", err.Error())
			return
		}
		privileges = append(privileges, privType)
	}
	if err := rows.Err(); err != nil {
		resp.Diagnostics.AddError("Error iterating default privileges", err.Error())
		return
	}

	if !hasRows {
		tflog.Warn(ctx, "No default privileges found, removing from state", map[string]interface{}{
			"owner":       owner,
			"role":        role,
			"object_type": objectType,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	privElements := make([]attr.Value, len(privileges))
	for i, p := range privileges {
		privElements[i] = types.StringValue(p)
	}
	privSet, diags := types.SetValue(types.StringType, privElements)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Privileges = privSet
	data.ID = types.StringValue(r.compositeID(data))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *defaultPrivilegesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan defaultPrivilegesResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state defaultPrivilegesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plural := objectTypePlural[plan.ObjectType.ValueString()]

	// Revoke old defaults first.
	revokeQuery := r.buildRevokeAllSQL(state.Owner.ValueString(), state.Role.ValueString(), state.Schema, plural)
	tflog.Debug(ctx, "Revoking old default privileges", map[string]interface{}{"query": revokeQuery})

	_, err := r.db.ExecContext(ctx, revokeQuery)
	if err != nil {
		resp.Diagnostics.AddError("Error revoking old default privileges", err.Error())
		return
	}

	// Grant new defaults.
	privileges := common.PrivilegesToSlice(ctx, plan.Privileges)

	grantQuery := r.buildGrantSQL(plan.Owner.ValueString(), plan.Role.ValueString(), plan.Schema, privileges, plural)
	tflog.Debug(ctx, "Granting new default privileges", map[string]interface{}{"query": grantQuery})

	_, err = r.db.ExecContext(ctx, grantQuery)
	if err != nil {
		resp.Diagnostics.AddError("Error granting new default privileges", err.Error())
		return
	}

	plan.ID = types.StringValue(r.compositeID(plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *defaultPrivilegesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data defaultPrivilegesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plural := objectTypePlural[data.ObjectType.ValueString()]
	query := r.buildRevokeAllSQL(data.Owner.ValueString(), data.Role.ValueString(), data.Schema, plural)

	tflog.Debug(ctx, "Revoking all default privileges", map[string]interface{}{"query": query})

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		resp.Diagnostics.AddError("Error revoking default privileges", err.Error())
		return
	}
}

// buildGrantSQL constructs an ALTER DEFAULT PRIVILEGES ... GRANT statement.
func (r *defaultPrivilegesResource) buildGrantSQL(owner, role string, schemaAttr types.String, privileges []string, objectTypePlural string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s", pq.QuoteIdentifier(owner)))

	if common.IsSet(schemaAttr) {
		b.WriteString(fmt.Sprintf(" IN SCHEMA %s", pq.QuoteIdentifier(schemaAttr.ValueString())))
	}

	b.WriteString(fmt.Sprintf(" GRANT %s ON %s TO %s",
		strings.Join(privileges, ", "),
		objectTypePlural,
		pq.QuoteIdentifier(role),
	))

	return b.String()
}

// buildRevokeAllSQL constructs an ALTER DEFAULT PRIVILEGES ... REVOKE ALL statement.
func (r *defaultPrivilegesResource) buildRevokeAllSQL(owner, role string, schemaAttr types.String, objectTypePlural string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s", pq.QuoteIdentifier(owner)))

	if common.IsSet(schemaAttr) {
		b.WriteString(fmt.Sprintf(" IN SCHEMA %s", pq.QuoteIdentifier(schemaAttr.ValueString())))
	}

	b.WriteString(fmt.Sprintf(" REVOKE ALL ON %s FROM %s",
		objectTypePlural,
		pq.QuoteIdentifier(role),
	))

	return b.String()
}

// compositeID builds the resource ID from its identifying attributes.
func (r *defaultPrivilegesResource) compositeID(data defaultPrivilegesResourceModel) string {
	schemaVal := ""
	if common.IsSet(data.Schema) {
		schemaVal = data.Schema.ValueString()
	}

	return fmt.Sprintf("%s_%s_%s_%s_%s",
		data.Owner.ValueString(),
		data.Role.ValueString(),
		data.Database.ValueString(),
		schemaVal,
		data.ObjectType.ValueString(),
	)
}
