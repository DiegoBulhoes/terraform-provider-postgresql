package resource

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

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
				MarkdownDescription: "The database where the schema resides. Defaults to the provider's configured database. Changing this forces a new resource.\n\n~> **Note:** The provider uses its configured connection and does not open a separate connection to this database. Ensure the provider is configured to connect to the correct database.",
				Optional:            true,
				Computed:            true,
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
				Description: "If true, use IF NOT EXISTS when creating the schema.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
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

	createTimeout, d := plan.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	schemaName := plan.Name.ValueString()

	// Build CREATE SCHEMA statement
	sqlStmt := "CREATE SCHEMA "
	if plan.IfNotExists.ValueBool() {
		sqlStmt += "IF NOT EXISTS "
	}
	sqlStmt += pq.QuoteIdentifier(schemaName)

	if common.IsSet(plan.Owner) {
		sqlStmt += " AUTHORIZATION " + pq.QuoteIdentifier(plan.Owner.ValueString())
	}

	tflog.Debug(ctx, "Executing SQL", map[string]interface{}{"sql": sqlStmt})

	_, err := r.DB.ExecContext(ctx, sqlStmt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating schema",
			fmt.Sprintf("Could not create schema %q: %s", schemaName, err.Error()),
		)
		return
	}

	// If database was not specified, read the current database from the connection
	if plan.Database.IsNull() || plan.Database.IsUnknown() {
		var currentDB string
		err := r.DB.QueryRowContext(ctx, "SELECT current_database()").Scan(&currentDB)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error reading current database",
				err.Error(),
			)
			return
		}
		plan.Database = types.StringValue(currentDB)
	}

	// Read back the owner from the database to populate computed state
	var owner string
	err = r.DB.QueryRowContext(ctx,
		"SELECT schema_owner FROM information_schema.schemata WHERE schema_name = $1",
		schemaName,
	).Scan(&owner)
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

func (r *SchemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	schemaName := state.Name.ValueString()

	var owner string
	err := r.DB.QueryRowContext(ctx,
		"SELECT schema_owner FROM information_schema.schemata WHERE schema_name = $1",
		schemaName,
	).Scan(&owner)
	if err == sql.ErrNoRows {
		tflog.Warn(ctx, "Schema not found, removing from state", map[string]interface{}{
			"schema": schemaName,
		})
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading schema",
			fmt.Sprintf("Could not read schema %q: %s", schemaName, err.Error()),
		)
		return
	}

	state.Owner = types.StringValue(owner)

	// Ensure database is populated
	if state.Database.IsNull() || state.Database.IsUnknown() {
		var currentDB string
		err := r.DB.QueryRowContext(ctx, "SELECT current_database()").Scan(&currentDB)
		if err != nil {
			resp.Diagnostics.AddError("Error reading current database", err.Error())
			return
		}
		state.Database = types.StringValue(currentDB)
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

	updateTimeout, d := plan.Timeouts.Update(ctx, 5*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	oldName := state.Name.ValueString()
	newName := plan.Name.ValueString()

	// Handle rename
	if oldName != newName {
		sqlStmt := fmt.Sprintf(
			"ALTER SCHEMA %s RENAME TO %s",
			pq.QuoteIdentifier(oldName),
			pq.QuoteIdentifier(newName),
		)
		tflog.Debug(ctx, "Executing SQL", map[string]interface{}{"sql": sqlStmt})

		_, err := r.DB.ExecContext(ctx, sqlStmt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error renaming schema",
				fmt.Sprintf("Could not rename schema %q to %q: %s", oldName, newName, err.Error()),
			)
			return
		}
	}

	// Handle owner change
	if common.IsSet(plan.Owner) &&
		plan.Owner.ValueString() != state.Owner.ValueString() {
		sqlStmt := fmt.Sprintf(
			"ALTER SCHEMA %s OWNER TO %s",
			pq.QuoteIdentifier(newName),
			pq.QuoteIdentifier(plan.Owner.ValueString()),
		)
		tflog.Debug(ctx, "Executing SQL", map[string]interface{}{"sql": sqlStmt})

		_, err := r.DB.ExecContext(ctx, sqlStmt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error changing schema owner",
				fmt.Sprintf("Could not change owner of schema %q: %s", newName, err.Error()),
			)
			return
		}
	}

	// Read back the current state from the database
	var owner string
	err := r.DB.QueryRowContext(ctx,
		"SELECT schema_owner FROM information_schema.schemata WHERE schema_name = $1",
		newName,
	).Scan(&owner)
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

	deleteTimeout, d := state.Timeouts.Delete(ctx, 5*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	schemaName := state.Name.ValueString()

	sqlStmt := fmt.Sprintf("DROP SCHEMA %s", pq.QuoteIdentifier(schemaName))
	tflog.Debug(ctx, "Executing SQL", map[string]interface{}{"sql": sqlStmt})

	_, err := r.DB.ExecContext(ctx, sqlStmt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting schema",
			fmt.Sprintf("Could not drop schema %q: %s", schemaName, err.Error()),
		)
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

	// Set if_not_exists to default value on import
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("if_not_exists"), false)...)
}
