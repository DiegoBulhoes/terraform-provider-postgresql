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
	_ resource.Resource                = (*ExtensionResource)(nil)
	_ resource.ResourceWithImportState = (*ExtensionResource)(nil)
)

type ExtensionResource struct {
	DB common.DBTX
}

type ExtensionResourceModel struct {
	Name          types.String   `tfsdk:"name"`
	Database      types.String   `tfsdk:"database"`
	Schema        types.String   `tfsdk:"schema"`
	Version       types.String   `tfsdk:"version"`
	CreateCascade types.Bool     `tfsdk:"create_cascade"`
	DropCascade   types.Bool     `tfsdk:"drop_cascade"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func NewExtensionResource() resource.Resource {
	return &ExtensionResource{}
}

func (r *ExtensionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_extension"
}

func (r *ExtensionResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     0,
		Description: "Manages a PostgreSQL extension.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the extension. Changing this forces a new resource.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 63),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database": schema.StringAttribute{
				Description: "The database to install the extension in. Defaults to the provider's configured database. Changing this forces a new resource.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"schema": schema.StringAttribute{
				Description: "The schema that holds the extension's objects. Defaults to the first schema in the search path. A change runs ALTER EXTENSION ... SET SCHEMA.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"version": schema.StringAttribute{
				Description: "The version of the extension. Defaults to the version in the extension's control file. A change runs ALTER EXTENSION ... UPDATE TO.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"create_cascade": schema.BoolAttribute{
				Description: "If true, create also installs the extensions this one requires (CREATE EXTENSION ... CASCADE). Default: false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"drop_cascade": schema.BoolAttribute{
				Description: "If true, destroy drops the extension with CASCADE, deleting every object that depends on it. Default: false.",
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

func (r *ExtensionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// CreateExtensionStatement builds the CREATE EXTENSION statement of a plan.
func CreateExtensionStatement(plan *ExtensionResourceModel) string {
	stmt := "CREATE EXTENSION " + pq.QuoteIdentifier(plan.Name.ValueString())
	if common.IsSet(plan.Schema) {
		stmt += " WITH SCHEMA " + pq.QuoteIdentifier(plan.Schema.ValueString())
	}
	if common.IsSet(plan.Version) {
		stmt += " VERSION " + pq.QuoteLiteral(plan.Version.ValueString())
	}
	if plan.CreateCascade.ValueBool() {
		stmt += " CASCADE"
	}
	return stmt
}

// AlterExtensionStatements builds the statements that move state to plan.
func AlterExtensionStatements(plan, state *ExtensionResourceModel) []string {
	name := pq.QuoteIdentifier(plan.Name.ValueString())
	var statements []string
	if common.IsSet(plan.Schema) && plan.Schema.ValueString() != state.Schema.ValueString() {
		statements = append(statements, fmt.Sprintf("ALTER EXTENSION %s SET SCHEMA %s",
			name, pq.QuoteIdentifier(plan.Schema.ValueString())))
	}
	if common.IsSet(plan.Version) && plan.Version.ValueString() != state.Version.ValueString() {
		statements = append(statements, fmt.Sprintf("ALTER EXTENSION %s UPDATE TO %s",
			name, pq.QuoteLiteral(plan.Version.ValueString())))
	}
	return statements
}

// ReadExtension reads the schema and version of an installed extension.
func ReadExtension(ctx context.Context, db common.DBTX, name string) (string, string, error) {
	var schemaName, version string
	err := db.QueryRowContext(ctx,
		"SELECT n.nspname, e.extversion FROM pg_catalog.pg_extension e JOIN pg_catalog.pg_namespace n ON n.oid = e.extnamespace WHERE e.extname = $1",
		name,
	).Scan(&schemaName, &version)
	return schemaName, version, err
}

// readBack fills the computed attributes of model from the server.
func (r *ExtensionResource) readBack(ctx context.Context, db common.DBTX, model *ExtensionResourceModel) error {
	schemaName, version, err := ReadExtension(ctx, db, model.Name.ValueString())
	if err != nil {
		return err
	}
	model.Schema = types.StringValue(schemaName)
	model.Version = types.StringValue(version)
	if !common.IsSet(model.Database) {
		var currentDB string
		if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&currentDB); err != nil {
			return fmt.Errorf("read current database: %w", err)
		}
		model.Database = types.StringValue(currentDB)
	}
	return nil
}

func (r *ExtensionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ExtensionResourceModel
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

	db, err := common.OnDatabase(ctx, r.DB, plan.Database.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to database", err.Error())
		return
	}

	name := plan.Name.ValueString()
	sqlStmt := CreateExtensionStatement(&plan)
	tflog.Debug(ctx, "Executing SQL", map[string]interface{}{"sql": sqlStmt})
	if _, err := db.ExecContext(ctx, sqlStmt); err != nil {
		resp.Diagnostics.AddError(
			"Error creating extension",
			fmt.Sprintf("Could not create extension %q: %s", name, err.Error()),
		)
		return
	}

	if err := r.readBack(ctx, db, &plan); err != nil {
		resp.Diagnostics.AddError(
			"Error reading extension after creation",
			fmt.Sprintf("Could not read extension %q: %s", name, err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ExtensionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ExtensionResourceModel
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

	name := state.Name.ValueString()
	db, err := common.OnDatabase(ctx, r.DB, state.Database.ValueString())
	if err == nil {
		err = r.readBack(ctx, db, &state)
	}
	if errors.Is(err, sql.ErrNoRows) || isGone(err) {
		tflog.Info(ctx, "Extension not found, removing from state", map[string]interface{}{
			"extension": name,
			"database":  state.Database.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading extension",
			fmt.Errorf("read extension %q: %w", name, err).Error(),
		)
		return
	}

	if state.CreateCascade.IsNull() {
		state.CreateCascade = types.BoolValue(false)
	}
	if state.DropCascade.IsNull() {
		state.DropCascade = types.BoolValue(false)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ExtensionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ExtensionResourceModel
	var state ExtensionResourceModel

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

	db, err := common.OnDatabase(ctx, r.DB, state.Database.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to database", err.Error())
		return
	}

	name := plan.Name.ValueString()
	statements := AlterExtensionStatements(&plan, &state)
	logStatements(ctx, statements)
	if err := common.ExecInTx(ctx, db, statements); err != nil {
		resp.Diagnostics.AddError(
			"Error updating extension",
			fmt.Sprintf("Could not update extension %q: %s", name, err.Error()),
		)
		return
	}

	plan.Database = state.Database
	if err := r.readBack(ctx, db, &plan); err != nil {
		resp.Diagnostics.AddError(
			"Error reading extension after update",
			fmt.Sprintf("Could not read extension %q: %s", name, err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ExtensionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ExtensionResourceModel
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

	db, err := common.OnDatabase(ctx, r.DB, state.Database.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to database", err.Error())
		return
	}

	name := state.Name.ValueString()
	sqlStmt := "DROP EXTENSION IF EXISTS " + pq.QuoteIdentifier(name)
	if state.DropCascade.ValueBool() {
		sqlStmt += " CASCADE"
	}
	tflog.Debug(ctx, "Executing SQL", map[string]interface{}{"sql": sqlStmt})

	if _, err := db.ExecContext(ctx, sqlStmt); err != nil {
		detail := fmt.Sprintf("Could not drop extension %q: %s", name, err.Error())
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "2BP01" {
			detail += "\nSet drop_cascade = true to drop the objects that depend on it as well."
		}
		resp.Diagnostics.AddError("Error deleting extension", detail)
		return
	}
}

func (r *ExtensionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if database, name, ok := strings.Cut(req.ID, "/"); ok {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), database)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	} else {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("create_cascade"), false)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("drop_cascade"), false)...)
}
