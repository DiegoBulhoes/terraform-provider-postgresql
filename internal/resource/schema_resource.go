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
	_ resource.Resource                = (*SchemaResource)(nil)
	_ resource.ResourceWithImportState = (*SchemaResource)(nil)
)

type SchemaResource struct {
	DB common.DBTX
}

type SchemaResourceModel struct {
	Name        types.String   `tfsdk:"name"`
	Database    types.String   `tfsdk:"database"`
	Owner       types.String   `tfsdk:"owner"`
	IfNotExists types.Bool     `tfsdk:"if_not_exists"`
	DropCascade types.Bool     `tfsdk:"drop_cascade"`
	Timeouts    timeouts.Value `tfsdk:"timeouts"`
}

func NewSchemaResource() resource.Resource {
	return &SchemaResource{}
}

func (r *SchemaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema"
}

func (r *SchemaResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     0,
		Description: "Manages a PostgreSQL schema.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the schema.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 63),
				},
			},
			"database": schema.StringAttribute{
				Description: "The database where the schema resides. Defaults to the provider's configured database. Changing this forces a new resource.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"owner": schema.StringAttribute{
				Description: "The role that owns the schema.",
				Optional:    true,
				Computed:    true,
			},
			"if_not_exists": schema.BoolAttribute{
				Description: "If true, use IF NOT EXISTS when creating the schema. An existing schema is adopted, given the configured owner, and dropped on destroy.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"drop_cascade": schema.BoolAttribute{
				Description: "If true, destroy drops the schema with CASCADE, deleting every object in it. Default: false.",
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

func (r *SchemaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SchemaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SchemaResourceModel
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

	schemaName := plan.Name.ValueString()

	sqlStmt := "CREATE SCHEMA "
	if plan.IfNotExists.ValueBool() {
		sqlStmt += "IF NOT EXISTS "
	}
	sqlStmt += pq.QuoteIdentifier(schemaName)

	if common.IsSet(plan.Owner) {
		sqlStmt += " AUTHORIZATION " + pq.QuoteIdentifier(plan.Owner.ValueString())
	}

	tflog.Debug(ctx, "Executing SQL", map[string]interface{}{"sql": sqlStmt})

	if _, err := db.ExecContext(ctx, sqlStmt); err != nil {
		resp.Diagnostics.AddError(
			"Error creating schema",
			fmt.Sprintf("Could not create schema %q: %s", schemaName, err.Error()),
		)
		return
	}

	// IF NOT EXISTS skips AUTHORIZATION when the schema already exists.
	if plan.IfNotExists.ValueBool() && common.IsSet(plan.Owner) {
		sqlStmt := fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s",
			pq.QuoteIdentifier(schemaName), pq.QuoteIdentifier(plan.Owner.ValueString()))
		if _, err := db.ExecContext(ctx, sqlStmt); err != nil {
			resp.Diagnostics.AddError(
				"Error changing schema owner",
				fmt.Sprintf("Could not change owner of schema %q: %s", schemaName, err.Error()),
			)
			return
		}
	}

	if !common.IsSet(plan.Database) {
		var currentDB string
		if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&currentDB); err != nil {
			resp.Diagnostics.AddError("Error reading current database", err.Error())
			return
		}
		plan.Database = types.StringValue(currentDB)
	}

	owner, err := ReadSchemaOwner(ctx, db, schemaName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading schema after creation",
			fmt.Sprintf("Could not read schema %q: %s", schemaName, err.Error()),
		)
		return
	}
	plan.Owner = types.StringValue(owner)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// ReadSchemaOwner reads from pg_namespace, which, unlike information_schema,
// also lists schemas the current user has no privilege on.
func ReadSchemaOwner(ctx context.Context, db common.DBTX, schemaName string) (string, error) {
	var owner string
	err := db.QueryRowContext(ctx,
		"SELECT pg_get_userbyid(nspowner) FROM pg_catalog.pg_namespace WHERE nspname = $1",
		schemaName,
	).Scan(&owner)
	return owner, err
}

func (r *SchemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SchemaResourceModel
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

	db, err := common.OnDatabase(ctx, r.DB, state.Database.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to database", err.Error())
		return
	}

	schemaName := state.Name.ValueString()

	owner, err := ReadSchemaOwner(ctx, db, schemaName)
	if errors.Is(err, sql.ErrNoRows) {
		tflog.Info(ctx, "Schema not found, removing from state", map[string]interface{}{
			"schema": schemaName,
		})
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading schema",
			fmt.Errorf("read schema %q: %w", schemaName, err).Error(),
		)
		return
	}

	state.Owner = types.StringValue(owner)

	if !common.IsSet(state.Database) {
		var currentDB string
		if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&currentDB); err != nil {
			resp.Diagnostics.AddError("Error reading current database", err.Error())
			return
		}
		state.Database = types.StringValue(currentDB)
	}
	if state.DropCascade.IsNull() {
		state.DropCascade = types.BoolValue(false)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SchemaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SchemaResourceModel
	var state SchemaResourceModel

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

	oldName := state.Name.ValueString()
	newName := plan.Name.ValueString()

	var statements []string
	if oldName != newName {
		statements = append(statements, fmt.Sprintf("ALTER SCHEMA %s RENAME TO %s",
			pq.QuoteIdentifier(oldName), pq.QuoteIdentifier(newName)))
	}
	if common.IsSet(plan.Owner) && plan.Owner.ValueString() != state.Owner.ValueString() {
		statements = append(statements, fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s",
			pq.QuoteIdentifier(newName), pq.QuoteIdentifier(plan.Owner.ValueString())))
	}
	logStatements(ctx, statements)
	if err := common.ExecInTx(ctx, db, statements); err != nil {
		resp.Diagnostics.AddError(
			"Error updating schema",
			fmt.Sprintf("Could not update schema %q: %s", oldName, err.Error()),
		)
		return
	}

	owner, err := ReadSchemaOwner(ctx, db, newName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading schema after update",
			fmt.Sprintf("Could not read schema %q: %s", newName, err.Error()),
		)
		return
	}
	plan.Owner = types.StringValue(owner)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SchemaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SchemaResourceModel
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

	schemaName := state.Name.ValueString()

	sqlStmt := fmt.Sprintf("DROP SCHEMA IF EXISTS %s", pq.QuoteIdentifier(schemaName))
	if state.DropCascade.ValueBool() {
		sqlStmt += " CASCADE"
	}
	tflog.Debug(ctx, "Executing SQL", map[string]interface{}{"sql": sqlStmt})

	if _, err := db.ExecContext(ctx, sqlStmt); err != nil {
		detail := fmt.Sprintf("Could not drop schema %q: %s", schemaName, err.Error())
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "2BP01" {
			detail += "\nSet drop_cascade = true to drop the objects in it as well."
		}
		resp.Diagnostics.AddError("Error deleting schema", detail)
		return
	}
}

func (r *SchemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.SplitN(req.ID, "/", 2)

	if len(idParts) == 2 {
		// Format: database/schema_name
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), idParts[0])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), idParts[1])...)
	} else {
		// Format: schema_name
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
	}

	// Set the defaults on import
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("if_not_exists"), false)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("drop_cascade"), false)...)
}
