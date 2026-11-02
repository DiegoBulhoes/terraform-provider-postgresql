package resource

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/lib/pq"
)

var (
	_ resource.Resource                   = (*GrantResource)(nil)
	_ resource.ResourceWithImportState    = (*GrantResource)(nil)
	_ resource.ResourceWithValidateConfig = (*GrantResource)(nil)
)

type GrantResource struct {
	DB common.DBTX
}

type GrantResourceModel struct {
	ID              types.String   `tfsdk:"id"`
	Role            types.String   `tfsdk:"role"`
	Database        types.String   `tfsdk:"database"`
	Schema          types.String   `tfsdk:"schema"`
	ObjectType      types.String   `tfsdk:"object_type"`
	Objects         types.List     `tfsdk:"objects"`
	Privileges      types.Set      `tfsdk:"privileges"`
	WithGrantOption types.Bool     `tfsdk:"with_grant_option"`
	Timeouts        timeouts.Value `tfsdk:"timeouts"`
}

func NewGrantResource() resource.Resource {
	return &GrantResource{}
}

func (r *GrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_grant"
}

func (r *GrantResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     0,
		Description: "Manages PostgreSQL GRANT privileges on database objects.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The import ID: role/object_type/database[/schema[/object1,object2]].",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"role": schema.StringAttribute{
				Description: "The role to which privileges are granted.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database": schema.StringAttribute{
				Description: "The database of the grant. Required when object_type is database. Schema, table, sequence and function grants run in this database, which defaults to the provider's database.",
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
				Validators: []validator.String{
					stringvalidator.OneOf("database", "schema", "table", "sequence", "function"),
				},
			},
			"objects": schema.ListAttribute{
				Description: "Names of the tables, sequences or functions to grant on. A function named alone covers all its overloads; with argument types, as in f(integer), it covers one. If empty, the grant covers all objects of the type in the schema, and destroying it revokes the privileges on all of them.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"privileges": schema.SetAttribute{
				Description: "The set of privileges to grant (e.g. SELECT, INSERT, USAGE, CREATE). Case-insensitive; ALL stands for every privilege of the object type.",
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
				Description: "Whether the grantee can grant the same privileges to others.",
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

func (r *GrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *GrantResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg GrantResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || !common.IsSet(cfg.ObjectType) {
		return
	}
	objectType := strings.ToLower(cfg.ObjectType.ValueString())
	switch objectType {
	case "database":
		if cfg.Database.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("database"), "Missing database",
				"database is required when object_type is database.")
		}
	case "schema", "table", "sequence", "function":
		if cfg.Schema.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("schema"), "Missing schema",
				fmt.Sprintf("schema is required when object_type is %s.", objectType))
		}
	}
	if (objectType == "database" || objectType == "schema") && !cfg.Objects.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("objects"), "Unexpected objects",
			"objects applies only to table, sequence and function grants.")
	}
}

// GrantConn returns the connection a grant runs on: database grants use the
// provider's database, the others run in the database they name.
func GrantConn(ctx context.Context, db common.DBTX, objectType, database string) (common.DBTX, error) {
	if objectType == "database" {
		return db, nil
	}
	return common.OnDatabase(ctx, db, database)
}

func (r *GrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GrantResourceModel
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

	conn, grants, err := r.grantStatements(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Error preparing GRANT", err.Error())
		return
	}
	logStatements(ctx, grants)
	if err := common.ExecInTx(ctx, conn, grants); err != nil {
		resp.Diagnostics.AddError("Error executing GRANT", err.Error())
		return
	}

	plan.ID = types.StringValue(grantID(&plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// grantStatements validates the plan and returns the connection and the GRANT statements.
func (r *GrantResource) grantStatements(ctx context.Context, plan *GrantResourceModel) (common.DBTX, []string, error) {
	objectType := strings.ToLower(plan.ObjectType.ValueString())
	privileges, err := common.NormalizePrivileges(common.StringSetToSlice(ctx, plan.Privileges))
	if err != nil {
		return nil, nil, err
	}
	conn, err := GrantConn(ctx, r.DB, objectType, plan.Database.ValueString())
	if err != nil {
		return nil, nil, err
	}
	objects, err := GrantObjects(ctx, conn, objectType, plan.Schema.ValueString(), common.StringListToSlice(ctx, plan.Objects))
	if err != nil {
		return nil, nil, err
	}
	var grantOptionClause string
	if plan.WithGrantOption.ValueBool() {
		grantOptionClause = " WITH GRANT OPTION"
	}
	return conn, BuildGrantStatements(strings.Join(privileges, ", "), objectType, plan.Database.ValueString(),
		plan.Schema.ValueString(), plan.Role.ValueString(), objects, grantOptionClause), nil
}

func logStatements(ctx context.Context, statements []string) {
	for _, stmt := range statements {
		tflog.Debug(ctx, "Executing SQL", map[string]interface{}{"sql": stmt})
	}
}

// grantID is the import ID: role/object_type/database[/schema[/object1,object2]].
// The old form joined the parts with "_", which role and database names also use.
func grantID(m *GrantResourceModel) string {
	parts := []string{m.Role.ValueString(), strings.ToLower(m.ObjectType.ValueString()), m.Database.ValueString()}
	objects := common.StringListToSlice(context.Background(), m.Objects)
	if m.Schema.ValueString() != "" || len(objects) > 0 {
		parts = append(parts, m.Schema.ValueString())
	}
	if len(objects) > 0 {
		parts = append(parts, strings.Join(objects, ","))
	}
	return strings.Join(parts, "/")
}

func (r *GrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GrantResourceModel
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

	objectType := strings.ToLower(state.ObjectType.ValueString())
	conn, err := GrantConn(ctx, r.DB, objectType, state.Database.ValueString())
	if isGone(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to database", err.Error())
		return
	}

	granted, err := ReadGrantedPrivileges(ctx, conn, state.Role.ValueString(), objectType,
		state.Database.ValueString(), state.Schema.ValueString(), common.StringListToSlice(ctx, state.Objects))
	if err != nil {
		resp.Diagnostics.AddError("Error reading privileges", err.Error())
		return
	}
	if !granted.TargetExists {
		tflog.Warn(ctx, "Grant target no longer exists, removing from state", map[string]interface{}{
			"role":        state.Role.ValueString(),
			"object_type": objectType,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	// With no object to look at (an empty schema), there is nothing to compare.
	if granted.Objects > 0 {
		full, err := FullPrivileges(ctx, conn, objectType)
		if err != nil {
			resp.Diagnostics.AddError("Error reading privileges", err.Error())
			return
		}
		desired := ExpandPrivileges(common.StringSetToSlice(ctx, state.Privileges), full)
		if state.Privileges.IsNull() || !slices.Equal(desired, granted.Privileges) {
			privSet, diags := types.SetValueFrom(ctx, types.StringType, granted.Privileges)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			state.Privileges = privSet
		}
		state.WithGrantOption = types.BoolValue(granted.Grantable)
	}
	if state.WithGrantOption.IsNull() {
		state.WithGrantOption = types.BoolValue(false)
	}
	state.ID = types.StringValue(grantID(&state))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// isGone reports a connection error caused by a database that no longer exists.
func isGone(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "3D000"
}

// GrantedPrivileges is what a role holds on the objects of a grant.
type GrantedPrivileges struct {
	// Privileges held on every object, sorted.
	Privileges []string
	// Grantable is true when every one of Privileges is held WITH GRANT OPTION.
	Grantable bool
	// Objects is how many objects were compared, missing named objects included.
	Objects int
	// TargetExists is false when the database or schema of the grant is gone.
	TargetExists bool
}

var aclQueries = map[string]string{
	"database": `
		SELECT d.datname, a.privilege_type, a.is_grantable
		FROM pg_catalog.pg_database d
		LEFT JOIN LATERAL aclexplode(coalesce(d.datacl, acldefault('d', d.datdba))) a
		  ON a.grantee = (SELECT oid FROM pg_catalog.pg_roles WHERE rolname = $2)
		WHERE d.datname = $1`,
	"schema": `
		SELECT n.nspname, a.privilege_type, a.is_grantable
		FROM pg_catalog.pg_namespace n
		LEFT JOIN LATERAL aclexplode(coalesce(n.nspacl, acldefault('n', n.nspowner))) a
		  ON a.grantee = (SELECT oid FROM pg_catalog.pg_roles WHERE rolname = $2)
		WHERE n.nspname = $1`,
	"table": `
		SELECT c.relname, a.privilege_type, a.is_grantable
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN LATERAL aclexplode(coalesce(c.relacl, acldefault('r', c.relowner))) a
		  ON a.grantee = (SELECT oid FROM pg_catalog.pg_roles WHERE rolname = $2)
		WHERE n.nspname = $1 AND c.relkind IN ('r', 'v', 'm', 'f', 'p')
		  AND (cardinality($3::text[]) = 0 OR c.relname = ANY($3::text[]))`,
	"sequence": `
		SELECT c.relname, a.privilege_type, a.is_grantable
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN LATERAL aclexplode(coalesce(c.relacl, acldefault('s', c.relowner))) a
		  ON a.grantee = (SELECT oid FROM pg_catalog.pg_roles WHERE rolname = $2)
		WHERE n.nspname = $1 AND c.relkind = 'S'
		  AND (cardinality($3::text[]) = 0 OR c.relname = ANY($3::text[]))`,
	// ALL FUNCTIONS IN SCHEMA covers functions, aggregates and window functions, not procedures.
	"function": `
		SELECT format('%I.%I(%s)', n.nspname, p.proname, pg_get_function_identity_arguments(p.oid)),
		       a.privilege_type, a.is_grantable
		FROM pg_catalog.pg_proc p
		JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
		LEFT JOIN LATERAL aclexplode(coalesce(p.proacl, acldefault('f', p.proowner))) a
		  ON a.grantee = (SELECT oid FROM pg_catalog.pg_roles WHERE rolname = $2)
		WHERE n.nspname = $1
		  AND CASE WHEN cardinality($3::text[]) = 0 THEN p.prokind IN ('f', 'a', 'w')
		           ELSE format('%I.%I(%s)', n.nspname, p.proname, pg_get_function_identity_arguments(p.oid)) = ANY($3::text[]) END`,
}

// ReadGrantedPrivileges reads, from the catalog, the privileges role holds on
// every object of a grant. Named objects that do not exist hold nothing.
func ReadGrantedPrivileges(ctx context.Context, db common.DBTX, role, objectType, database, schemaName string, objects []string) (GrantedPrivileges, error) {
	query, ok := aclQueries[objectType]
	if !ok {
		return GrantedPrivileges{}, fmt.Errorf("unsupported object type: %s", objectType)
	}

	missing := 0
	if objectType == "function" && len(objects) > 0 {
		var err error
		objects, missing, err = ResolveFunctions(ctx, db, schemaName, objects)
		if err != nil {
			return GrantedPrivileges{}, err
		}
	}

	var args []any
	switch objectType {
	case "database":
		args = []any{database, role}
	case "schema":
		args = []any{schemaName, role}
	default:
		exists, err := schemaExists(ctx, db, schemaName)
		if err != nil || !exists {
			return GrantedPrivileges{}, err
		}
		if objects == nil {
			objects = []string{}
		}
		args = []any{schemaName, role, pq.Array(objects)}
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return GrantedPrivileges{}, err
	}
	defer rows.Close() //nolint:errcheck

	held := map[string]map[string]bool{}
	for rows.Next() {
		var object string
		var privilege sql.NullString
		var grantable sql.NullBool
		if err := rows.Scan(&object, &privilege, &grantable); err != nil {
			return GrantedPrivileges{}, err
		}
		if held[object] == nil {
			held[object] = map[string]bool{}
		}
		if privilege.Valid {
			// Grants from several grantors repeat a privilege.
			held[object][privilege.String] = held[object][privilege.String] || grantable.Bool
		}
	}
	if err := rows.Err(); err != nil {
		return GrantedPrivileges{}, err
	}

	if objectType == "database" || objectType == "schema" {
		if len(held) == 0 {
			return GrantedPrivileges{}, nil
		}
	} else if len(objects) > 0 {
		missing += len(objects) - len(held)
	}

	result := GrantedPrivileges{Objects: len(held) + missing, TargetExists: true}
	if missing > 0 || len(held) == 0 {
		return result, nil
	}
	var shared map[string]bool
	for _, privs := range held {
		if shared == nil {
			shared = maps.Clone(privs)
			continue
		}
		for p, g := range shared {
			if pg, ok := privs[p]; ok {
				shared[p] = g && pg
			} else {
				delete(shared, p)
			}
		}
	}
	result.Grantable = len(shared) > 0
	for p, g := range shared {
		result.Privileges = append(result.Privileges, p)
		result.Grantable = result.Grantable && g
	}
	sort.Strings(result.Privileges)
	return result, nil
}

func schemaExists(ctx context.Context, db common.DBTX, schemaName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_namespace WHERE nspname = $1)`, schemaName).Scan(&exists)
	return exists, err
}

var aclTypes = map[string]string{"database": "d", "schema": "n", "table": "r", "sequence": "s", "function": "f"}

// FullPrivileges lists every privilege the server knows for an object type,
// which is what ALL grants (MAINTAIN, for instance, exists only on PostgreSQL 17).
func FullPrivileges(ctx context.Context, db common.DBTX, objectType string) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT privilege_type FROM aclexplode(acldefault($1::"char", 10)) WHERE grantee = 10`,
		aclTypes[objectType])
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var privileges []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		privileges = append(privileges, p)
	}
	return privileges, rows.Err()
}

// ExpandPrivileges spells privileges as the catalog does: upper case, ALL
// expanded to full and TEMP as TEMPORARY. The result is sorted and unique.
func ExpandPrivileges(privileges, full []string) []string {
	seen := map[string]bool{}
	for _, p := range privileges {
		switch p = strings.ToUpper(strings.TrimSpace(p)); p {
		case "ALL", "ALL PRIVILEGES":
			for _, f := range full {
				seen[f] = true
			}
		case "TEMP":
			seen["TEMPORARY"] = true
		default:
			seen[p] = true
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// GrantObjects returns objects as BuildGrantStatements takes them: functions
// resolved to their signatures, other names unchanged.
func GrantObjects(ctx context.Context, db common.DBTX, objectType, schemaName string, objects []string) ([]string, error) {
	if objectType != "function" || len(objects) == 0 {
		return objects, nil
	}
	signatures, missing, err := ResolveFunctions(ctx, db, schemaName, objects)
	if err != nil {
		return nil, err
	}
	if missing > 0 {
		return nil, fmt.Errorf("function not found in schema %q: %s", schemaName, strings.Join(missingFunctions(ctx, db, schemaName, objects), ", "))
	}
	return signatures, nil
}

const resolveFunctionQuery = `
	SELECT format('%I.%I(%s)', n.nspname, p.proname, pg_get_function_identity_arguments(p.oid))
	FROM pg_catalog.pg_proc p
	JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
	WHERE CASE WHEN $3 = '' THEN n.nspname = $1 AND p.proname = $2
	           ELSE p.oid = to_regprocedure(format('%I.%I', $1::text, $2::text) || $3) END
	ORDER BY 1`

// ResolveFunctions turns function names into signatures the server rendered:
// a bare name gives every overload, name(args) gives one. It also counts the
// names that match nothing.
func ResolveFunctions(ctx context.Context, db common.DBTX, schemaName string, objects []string) ([]string, int, error) {
	var signatures []string
	missing := 0
	for _, obj := range objects {
		name, args := splitSignature(obj)
		rows, err := db.QueryContext(ctx, resolveFunctionQuery, schemaName, name, args)
		if err != nil {
			return nil, 0, err
		}
		found := 0
		for rows.Next() {
			var sig string
			if err := rows.Scan(&sig); err != nil {
				_ = rows.Close()
				return nil, 0, err
			}
			signatures = append(signatures, sig)
			found++
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, 0, err
		}
		if found == 0 {
			missing++
		}
	}
	return signatures, missing, nil
}

func missingFunctions(ctx context.Context, db common.DBTX, schemaName string, objects []string) []string {
	var out []string
	for _, obj := range objects {
		if _, missing, err := ResolveFunctions(ctx, db, schemaName, []string{obj}); err == nil && missing > 0 {
			out = append(out, obj)
		}
	}
	return out
}

// splitSignature splits "f(integer)" into "f" and "(integer)".
func splitSignature(obj string) (string, string) {
	obj = strings.TrimSpace(obj)
	if i := strings.Index(obj, "("); i > 0 {
		return strings.TrimSpace(obj[:i]), obj[i:]
	}
	return obj, ""
}

func (r *GrantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan GrantResourceModel
	var state GrantResourceModel
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

	// Everything is validated before the REVOKE, and both run in one transaction,
	// so a failed update leaves the old privileges in place.
	conn, grants, err := r.grantStatements(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Error preparing GRANT", err.Error())
		return
	}
	objectType := strings.ToLower(plan.ObjectType.ValueString())
	objects, err := GrantObjects(ctx, conn, objectType, plan.Schema.ValueString(), common.StringListToSlice(ctx, plan.Objects))
	if err != nil {
		resp.Diagnostics.AddError("Error preparing REVOKE", err.Error())
		return
	}
	revokes := BuildRevokeStatements(objectType, plan.Database.ValueString(), plan.Schema.ValueString(), plan.Role.ValueString(), objects)
	statements := slices.Concat(revokes, grants)
	logStatements(ctx, statements)
	if err := common.ExecInTx(ctx, conn, statements); err != nil {
		resp.Diagnostics.AddError("Error updating grant", err.Error())
		return
	}

	plan.ID = types.StringValue(grantID(&plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// goneCodes are the errors of a REVOKE whose target no longer exists: there is nothing left to revoke.
var goneCodes = map[pq.ErrorCode]bool{
	"3D000": true, // invalid_catalog_name
	"3F000": true, // invalid_schema_name
	"42P01": true, // undefined_table
	"42883": true, // undefined_function
	"42704": true, // undefined_object
}

func (r *GrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GrantResourceModel
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

	objectType := strings.ToLower(state.ObjectType.ValueString())
	conn, err := GrantConn(ctx, r.DB, objectType, state.Database.ValueString())
	if isGone(err) {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to database", err.Error())
		return
	}
	objects := common.StringListToSlice(ctx, state.Objects)
	if objectType == "function" && len(objects) > 0 {
		// Functions dropped since are skipped; if none is left there is nothing to revoke.
		if objects, _, err = ResolveFunctions(ctx, conn, state.Schema.ValueString(), objects); err != nil {
			resp.Diagnostics.AddError("Error preparing REVOKE", err.Error())
			return
		}
		if len(objects) == 0 {
			return
		}
	}

	revokeStatements := BuildRevokeStatements(objectType, state.Database.ValueString(), state.Schema.ValueString(), state.Role.ValueString(), objects)
	for _, stmt := range revokeStatements {
		tflog.Debug(ctx, "Executing REVOKE", map[string]interface{}{"sql": stmt})
		_, err := conn.ExecContext(ctx, stmt)
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && goneCodes[pqErr.Code] {
			continue
		}
		if err != nil {
			resp.Diagnostics.AddError("Error executing REVOKE", fmt.Sprintf("SQL: %s\nError: %s", stmt, err.Error()))
			return
		}
	}
}

func (r *GrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: role/object_type/database[/schema[/object1,object2]]
	parts := strings.Split(req.ID, "/")

	if len(parts) < 3 || len(parts) > 5 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected format: role/object_type/database[/schema[/object1,object2]]. Got: %s", req.ID),
		)
		return
	}

	model := GrantResourceModel{
		Role:            types.StringValue(parts[0]),
		ObjectType:      types.StringValue(strings.ToLower(parts[1])),
		Database:        types.StringValue(parts[2]),
		Schema:          types.StringNull(),
		Objects:         types.ListNull(types.StringType),
		Privileges:      types.SetNull(types.StringType),
		WithGrantOption: types.BoolValue(false),
		Timeouts:        timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{"create": types.StringType, "read": types.StringType, "update": types.StringType, "delete": types.StringType})},
	}
	if parts[2] == "" {
		model.Database = types.StringNull()
	}
	if len(parts) >= 4 && parts[3] != "" {
		model.Schema = types.StringValue(parts[3])
	}
	if len(parts) == 5 && parts[4] != "" {
		objects, diags := types.ListValueFrom(ctx, types.StringType, strings.Split(parts[4], ","))
		resp.Diagnostics.Append(diags...)
		model.Objects = objects
	}
	model.ID = types.StringValue(grantID(&model))
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// BuildGrantStatements constructs GRANT SQL statements for the given object type.
func BuildGrantStatements(privList, objectType, database, schemaName, role string, objects []string, grantOptionClause string) []string {
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
			// Objects are signatures from GrantObjects; ROUTINE also covers procedures.
			for _, obj := range objects {
				stmt := fmt.Sprintf("GRANT %s ON ROUTINE %s TO %s%s",
					privList, obj, quotedRole, grantOptionClause)
				statements = append(statements, stmt)
			}
		}
	}

	return statements
}

// BuildRevokeStatements constructs REVOKE ALL SQL statements for the given object type.
func BuildRevokeStatements(objectType, database, schemaName, role string, objects []string) []string {
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
				stmt := fmt.Sprintf("REVOKE ALL PRIVILEGES ON ROUTINE %s FROM %s", obj, quotedRole)
				statements = append(statements, stmt)
			}
		}
	}

	return statements
}
