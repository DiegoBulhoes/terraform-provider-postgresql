package resource

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lib/pq"
)

var _ resource.Resource = (*grantResource)(nil)

type grantResource struct {
	db *sql.DB
}

type grantResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Role            types.String `tfsdk:"role"`
	Database        types.String `tfsdk:"database"`
	Schema          types.String `tfsdk:"schema"`
	ObjectType      types.String `tfsdk:"object_type"`
	Objects         types.List   `tfsdk:"objects"`
	Privileges      types.Set    `tfsdk:"privileges"`
	WithGrantOption types.Bool   `tfsdk:"with_grant_option"`
}

func NewGrantResource() resource.Resource {
	return &grantResource{}
}

func (r *grantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_grant"
}

func (r *grantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages PostgreSQL GRANT privileges on database objects.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier: {role}_{object_type}_{database}_{schema}.",
				Computed:    true,
			},
			"role": schema.StringAttribute{
				Description: "The role to which privileges are granted.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database": schema.StringAttribute{
				Description: "The database on which to grant privileges.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema": schema.StringAttribute{
				Description: "The schema on which to grant privileges.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"object_type": schema.StringAttribute{
				Description: "The object type to grant privileges on: database, schema, table, sequence, or function.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"objects": schema.ListAttribute{
				Description: "Specific object names to grant on. If empty, grants on all objects of the given type in the schema.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"privileges": schema.SetAttribute{
				Description: "The set of privileges to grant (e.g. SELECT, INSERT, USAGE, CREATE).",
				Required:    true,
				ElementType: types.StringType,
			},
			"with_grant_option": schema.BoolAttribute{
				Description: "Whether the grantee can grant the same privileges to others.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}

func (r *grantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *grantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan grantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := plan.Role.ValueString()
	objectType := strings.ToLower(plan.ObjectType.ValueString())
	database := plan.Database.ValueString()
	schemaName := plan.Schema.ValueString()
	withGrantOption := plan.WithGrantOption.ValueBool()

	privileges := common.StringSetToSlice(ctx, plan.Privileges)
	objects := common.StringListToSlice(ctx, plan.Objects)
	privList := strings.Join(privileges, ", ")

	var grantOptionClause string
	if withGrantOption {
		grantOptionClause = " WITH GRANT OPTION"
	}

	statements := buildGrantStatements(privList, objectType, database, schemaName, role, objects, grantOptionClause)

	for _, stmt := range statements {
		tflog.Debug(ctx, "Executing GRANT", map[string]interface{}{"sql": stmt})
		_, err := r.db.ExecContext(ctx, stmt)
		if err != nil {
			resp.Diagnostics.AddError("Error executing GRANT", fmt.Sprintf("SQL: %s\nError: %s", stmt, err.Error()))
			return
		}
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s_%s_%s_%s", role, objectType, database, schemaName))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *grantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state grantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := state.Role.ValueString()
	objectType := strings.ToLower(state.ObjectType.ValueString())
	database := state.Database.ValueString()
	schemaName := state.Schema.ValueString()
	objects := common.StringListToSlice(ctx, state.Objects)

	// Drift detection for database, schema, and specific object grants.
	// For "ALL objects" grants (empty objects list on table/sequence/function),
	// server-side verification is skipped as it would require checking every object.
	canDetectDrift := objectType == "database" || objectType == "schema" || len(objects) > 0

	if canDetectDrift {
		privileges, grantOption, err := r.readPrivileges(ctx, role, objectType, database, schemaName, objects)
		if err != nil {
			resp.Diagnostics.AddError("Error reading privileges", err.Error())
			return
		}

		if len(privileges) == 0 {
			tflog.Warn(ctx, "No privileges found for grant, removing from state", map[string]interface{}{
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

		state.Privileges = privSet
		state.WithGrantOption = types.BoolValue(grantOption)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// readPrivileges queries the PostgreSQL catalog to determine what privileges a role
// currently has on a given object.
func (r *grantResource) readPrivileges(ctx context.Context, role, objectType, database, schemaName string, objects []string) ([]string, bool, error) {
	var query string
	var args []interface{}

	switch objectType {
	case "database":
		query = `
			SELECT privilege_type, is_grantable
			FROM (
				SELECT (aclexplode(datacl)).grantee,
				       (aclexplode(datacl)).privilege_type,
				       (aclexplode(datacl)).is_grantable
				FROM pg_database
				WHERE datname = $1 AND datacl IS NOT NULL
			) AS acl
			JOIN pg_roles ON acl.grantee = pg_roles.oid
			WHERE pg_roles.rolname = $2
		`
		args = []interface{}{database, role}

	case "schema":
		query = `
			SELECT privilege_type, is_grantable
			FROM (
				SELECT (aclexplode(nspacl)).grantee,
				       (aclexplode(nspacl)).privilege_type,
				       (aclexplode(nspacl)).is_grantable
				FROM pg_namespace
				WHERE nspname = $1 AND nspacl IS NOT NULL
			) AS acl
			JOIN pg_roles ON acl.grantee = pg_roles.oid
			WHERE pg_roles.rolname = $2
		`
		args = []interface{}{schemaName, role}

	case "table":
		query = `
			SELECT privilege_type, is_grantable
			FROM (
				SELECT (aclexplode(relacl)).grantee,
				       (aclexplode(relacl)).privilege_type,
				       (aclexplode(relacl)).is_grantable
				FROM pg_class c
				JOIN pg_namespace n ON c.relnamespace = n.oid
				WHERE n.nspname = $1 AND c.relname = $2
				  AND c.relkind IN ('r', 'v', 'm', 'f', 'p')
				  AND c.relacl IS NOT NULL
			) AS acl
			JOIN pg_roles ON acl.grantee = pg_roles.oid
			WHERE pg_roles.rolname = $3
		`
		args = []interface{}{schemaName, objects[0], role}

	case "sequence":
		query = `
			SELECT privilege_type, is_grantable
			FROM (
				SELECT (aclexplode(relacl)).grantee,
				       (aclexplode(relacl)).privilege_type,
				       (aclexplode(relacl)).is_grantable
				FROM pg_class c
				JOIN pg_namespace n ON c.relnamespace = n.oid
				WHERE n.nspname = $1 AND c.relname = $2
				  AND c.relkind = 'S'
				  AND c.relacl IS NOT NULL
			) AS acl
			JOIN pg_roles ON acl.grantee = pg_roles.oid
			WHERE pg_roles.rolname = $3
		`
		args = []interface{}{schemaName, objects[0], role}

	case "function":
		query = `
			SELECT privilege_type, is_grantable
			FROM (
				SELECT (aclexplode(proacl)).grantee,
				       (aclexplode(proacl)).privilege_type,
				       (aclexplode(proacl)).is_grantable
				FROM pg_proc p
				JOIN pg_namespace n ON p.pronamespace = n.oid
				WHERE n.nspname = $1 AND p.proname = $2
				  AND p.proacl IS NOT NULL
			) AS acl
			JOIN pg_roles ON acl.grantee = pg_roles.oid
			WHERE pg_roles.rolname = $3
		`
		args = []interface{}{schemaName, objects[0], role}

	default:
		return nil, false, fmt.Errorf("unsupported object type: %s", objectType)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var privileges []string
	allGrantable := true
	hasRows := false

	for rows.Next() {
		hasRows = true
		var privType string
		var isGrantable bool
		if err := rows.Scan(&privType, &isGrantable); err != nil {
			return nil, false, err
		}
		privileges = append(privileges, privType)
		if !isGrantable {
			allGrantable = false
		}
	}

	if !hasRows {
		allGrantable = false
	}

	return privileges, allGrantable, rows.Err()
}

func (r *grantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan grantResourceModel
	var state grantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := plan.Role.ValueString()
	objectType := strings.ToLower(plan.ObjectType.ValueString())
	database := plan.Database.ValueString()
	schemaName := plan.Schema.ValueString()
	withGrantOption := plan.WithGrantOption.ValueBool()
	objects := common.StringListToSlice(ctx, plan.Objects)

	// Revoke old privileges first.
	revokeStatements := buildRevokeStatements(objectType, database, schemaName, role, objects)
	for _, stmt := range revokeStatements {
		tflog.Debug(ctx, "Executing REVOKE", map[string]interface{}{"sql": stmt})
		_, err := r.db.ExecContext(ctx, stmt)
		if err != nil {
			resp.Diagnostics.AddError("Error executing REVOKE", fmt.Sprintf("SQL: %s\nError: %s", stmt, err.Error()))
			return
		}
	}

	// Grant new privileges.
	privileges := common.StringSetToSlice(ctx, plan.Privileges)
	privList := strings.Join(privileges, ", ")

	var grantOptionClause string
	if withGrantOption {
		grantOptionClause = " WITH GRANT OPTION"
	}

	grantStatements := buildGrantStatements(privList, objectType, database, schemaName, role, objects, grantOptionClause)
	for _, stmt := range grantStatements {
		tflog.Debug(ctx, "Executing GRANT", map[string]interface{}{"sql": stmt})
		_, err := r.db.ExecContext(ctx, stmt)
		if err != nil {
			resp.Diagnostics.AddError("Error executing GRANT", fmt.Sprintf("SQL: %s\nError: %s", stmt, err.Error()))
			return
		}
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s_%s_%s_%s", role, objectType, database, schemaName))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *grantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state grantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := state.Role.ValueString()
	objectType := strings.ToLower(state.ObjectType.ValueString())
	database := state.Database.ValueString()
	schemaName := state.Schema.ValueString()
	objects := common.StringListToSlice(ctx, state.Objects)

	revokeStatements := buildRevokeStatements(objectType, database, schemaName, role, objects)
	for _, stmt := range revokeStatements {
		tflog.Debug(ctx, "Executing REVOKE", map[string]interface{}{"sql": stmt})
		_, err := r.db.ExecContext(ctx, stmt)
		if err != nil {
			resp.Diagnostics.AddError("Error executing REVOKE", fmt.Sprintf("SQL: %s\nError: %s", stmt, err.Error()))
			return
		}
	}
}

// buildGrantStatements constructs GRANT SQL statements for the given object type.
func buildGrantStatements(privList, objectType, database, schemaName, role string, objects []string, grantOptionClause string) []string {
	quotedRole := pq.QuoteIdentifier(role)
	var statements []string

	switch objectType {
	case "database":
		stmt := fmt.Sprintf("GRANT %s ON DATABASE %s TO %s%s",
			privList, pq.QuoteIdentifier(database), quotedRole, grantOptionClause)
		statements = append(statements, stmt)

	case "schema":
		stmt := fmt.Sprintf("GRANT %s ON SCHEMA %s TO %s%s",
			privList, pq.QuoteIdentifier(schemaName), quotedRole, grantOptionClause)
		statements = append(statements, stmt)

	case "table":
		if len(objects) == 0 {
			stmt := fmt.Sprintf("GRANT %s ON ALL TABLES IN SCHEMA %s TO %s%s",
				privList, pq.QuoteIdentifier(schemaName), quotedRole, grantOptionClause)
			statements = append(statements, stmt)
		} else {
			for _, obj := range objects {
				stmt := fmt.Sprintf("GRANT %s ON TABLE %s.%s TO %s%s",
					privList, pq.QuoteIdentifier(schemaName), pq.QuoteIdentifier(obj), quotedRole, grantOptionClause)
				statements = append(statements, stmt)
			}
		}

	case "sequence":
		if len(objects) == 0 {
			stmt := fmt.Sprintf("GRANT %s ON ALL SEQUENCES IN SCHEMA %s TO %s%s",
				privList, pq.QuoteIdentifier(schemaName), quotedRole, grantOptionClause)
			statements = append(statements, stmt)
		} else {
			for _, obj := range objects {
				stmt := fmt.Sprintf("GRANT %s ON SEQUENCE %s.%s TO %s%s",
					privList, pq.QuoteIdentifier(schemaName), pq.QuoteIdentifier(obj), quotedRole, grantOptionClause)
				statements = append(statements, stmt)
			}
		}

	case "function":
		if len(objects) == 0 {
			stmt := fmt.Sprintf("GRANT %s ON ALL FUNCTIONS IN SCHEMA %s TO %s%s",
				privList, pq.QuoteIdentifier(schemaName), quotedRole, grantOptionClause)
			statements = append(statements, stmt)
		} else {
			for _, obj := range objects {
				stmt := fmt.Sprintf("GRANT %s ON FUNCTION %s.%s TO %s%s",
					privList, pq.QuoteIdentifier(schemaName), pq.QuoteIdentifier(obj), quotedRole, grantOptionClause)
				statements = append(statements, stmt)
			}
		}
	}

	return statements
}

// buildRevokeStatements constructs REVOKE ALL SQL statements for the given object type.
func buildRevokeStatements(objectType, database, schemaName, role string, objects []string) []string {
	quotedRole := pq.QuoteIdentifier(role)
	var statements []string

	switch objectType {
	case "database":
		stmt := fmt.Sprintf("REVOKE ALL PRIVILEGES ON DATABASE %s FROM %s",
			pq.QuoteIdentifier(database), quotedRole)
		statements = append(statements, stmt)

	case "schema":
		stmt := fmt.Sprintf("REVOKE ALL PRIVILEGES ON SCHEMA %s FROM %s",
			pq.QuoteIdentifier(schemaName), quotedRole)
		statements = append(statements, stmt)

	case "table":
		if len(objects) == 0 {
			stmt := fmt.Sprintf("REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA %s FROM %s",
				pq.QuoteIdentifier(schemaName), quotedRole)
			statements = append(statements, stmt)
		} else {
			for _, obj := range objects {
				stmt := fmt.Sprintf("REVOKE ALL PRIVILEGES ON TABLE %s.%s FROM %s",
					pq.QuoteIdentifier(schemaName), pq.QuoteIdentifier(obj), quotedRole)
				statements = append(statements, stmt)
			}
		}

	case "sequence":
		if len(objects) == 0 {
			stmt := fmt.Sprintf("REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA %s FROM %s",
				pq.QuoteIdentifier(schemaName), quotedRole)
			statements = append(statements, stmt)
		} else {
			for _, obj := range objects {
				stmt := fmt.Sprintf("REVOKE ALL PRIVILEGES ON SEQUENCE %s.%s FROM %s",
					pq.QuoteIdentifier(schemaName), pq.QuoteIdentifier(obj), quotedRole)
				statements = append(statements, stmt)
			}
		}

	case "function":
		if len(objects) == 0 {
			stmt := fmt.Sprintf("REVOKE ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA %s FROM %s",
				pq.QuoteIdentifier(schemaName), quotedRole)
			statements = append(statements, stmt)
		} else {
			for _, obj := range objects {
				stmt := fmt.Sprintf("REVOKE ALL PRIVILEGES ON FUNCTION %s.%s FROM %s",
					pq.QuoteIdentifier(schemaName), pq.QuoteIdentifier(obj), quotedRole)
				statements = append(statements, stmt)
			}
		}
	}

	return statements
}
