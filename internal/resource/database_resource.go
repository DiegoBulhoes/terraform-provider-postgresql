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
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lib/pq"
)

var (
	_ resource.Resource                = (*DatabaseResource)(nil)
	_ resource.ResourceWithImportState = (*DatabaseResource)(nil)
)

type DatabaseResource struct {
	DB common.DBTX
}

type DatabaseResourceModel struct {
	Name             types.String   `tfsdk:"name"`
	Owner            types.String   `tfsdk:"owner"`
	Template         types.String   `tfsdk:"template"`
	Encoding         types.String   `tfsdk:"encoding"`
	LcCollate        types.String   `tfsdk:"lc_collate"`
	LcCtype          types.String   `tfsdk:"lc_ctype"`
	TablespaceName   types.String   `tfsdk:"tablespace_name"`
	ConnectionLimit  types.Int64    `tfsdk:"connection_limit"`
	AllowConnections types.Bool     `tfsdk:"allow_connections"`
	IsTemplate       types.Bool     `tfsdk:"is_template"`
	OID              types.Int64    `tfsdk:"oid"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func NewDatabaseResource() resource.Resource {
	return &DatabaseResource{}
}

func (r *DatabaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database"
}

func (r *DatabaseResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     0,
		Description: "Manages a PostgreSQL database.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the database.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 63),
				},
			},
			"owner": schema.StringAttribute{
				Description: "The role name of the owner of the database.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"template": schema.StringAttribute{
				Description: "The name of the template database from which to create the new database.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("template0"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"encoding": schema.StringAttribute{
				Description: "Character set encoding to use in the new database.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("UTF8"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"lc_collate": schema.StringAttribute{
				Description: "Collation order (LC_COLLATE) to use in the new database.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"lc_ctype": schema.StringAttribute{
				Description: "Character classification (LC_CTYPE) to use in the new database.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tablespace_name": schema.StringAttribute{
				Description: "The name of the tablespace that will be associated with the new database.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("pg_default"),
			},
			"connection_limit": schema.Int64Attribute{
				Description: "How many concurrent connections can be made to this database. -1 means no limit.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(-1),
			},
			"allow_connections": schema.BoolAttribute{
				Description: "If false then no one can connect to this database.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"is_template": schema.BoolAttribute{
				Description: "If true, this database can be cloned by any user with CREATEDB privileges.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"oid": schema.Int64Attribute{
				Description: "The OID of the database.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
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

func (r *DatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseResourceModel
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

	dbName := plan.Name.ValueString()

	var opts []string

	if common.IsSet(plan.Owner) {
		opts = append(opts, fmt.Sprintf("OWNER = %s", pq.QuoteIdentifier(plan.Owner.ValueString())))
	}

	if common.IsSet(plan.Template) {
		opts = append(opts, fmt.Sprintf("TEMPLATE = %s", pq.QuoteIdentifier(plan.Template.ValueString())))
	}

	if common.IsSet(plan.Encoding) {
		opts = append(opts, fmt.Sprintf("ENCODING = %s", pq.QuoteLiteral(plan.Encoding.ValueString())))
	}

	if common.IsSet(plan.LcCollate) {
		opts = append(opts, fmt.Sprintf("LC_COLLATE = %s", pq.QuoteLiteral(plan.LcCollate.ValueString())))
	}

	if common.IsSet(plan.LcCtype) {
		opts = append(opts, fmt.Sprintf("LC_CTYPE = %s", pq.QuoteLiteral(plan.LcCtype.ValueString())))
	}

	if common.IsSet(plan.TablespaceName) {
		opts = append(opts, fmt.Sprintf("TABLESPACE = %s", pq.QuoteIdentifier(plan.TablespaceName.ValueString())))
	}

	if common.IsSet(plan.ConnectionLimit) {
		opts = append(opts, fmt.Sprintf("CONNECTION LIMIT = %d", plan.ConnectionLimit.ValueInt64()))
	}

	if common.IsSet(plan.AllowConnections) {
		opts = append(opts, fmt.Sprintf("ALLOW_CONNECTIONS = %t", plan.AllowConnections.ValueBool()))
	}

	if common.IsSet(plan.IsTemplate) {
		opts = append(opts, fmt.Sprintf("IS_TEMPLATE = %t", plan.IsTemplate.ValueBool()))
	}

	query := fmt.Sprintf("CREATE DATABASE %s", pq.QuoteIdentifier(dbName))
	if len(opts) > 0 {
		query += " WITH " + strings.Join(opts, " ")
	}

	_, err := r.DB.ExecContext(ctx, query)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating database",
			fmt.Sprintf("Could not create database %s: %s", dbName, err.Error()),
		)
		return
	}

	// Read back the database to populate computed attributes
	diags := r.ReadDatabase(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags := r.ReadDatabase(ctx, &state)
	if diags.HasError() {
		// If the database no longer exists, remove it from state
		for _, d := range diags {
			if d.Summary() == "Database not found" {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatabaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DatabaseResourceModel
	var state DatabaseResourceModel
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

	dbName := pq.QuoteIdentifier(plan.Name.ValueString())

	// Update owner
	if !plan.Owner.Equal(state.Owner) && common.IsSet(plan.Owner) {
		query := fmt.Sprintf("ALTER DATABASE %s OWNER TO %s", dbName, pq.QuoteIdentifier(plan.Owner.ValueString()))
		_, err := r.DB.ExecContext(ctx, query)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating database owner",
				fmt.Sprintf("Could not update owner for database %s: %s", plan.Name.ValueString(), err.Error()),
			)
			return
		}
	}

	// Update tablespace
	if !plan.TablespaceName.Equal(state.TablespaceName) && common.IsSet(plan.TablespaceName) {
		query := fmt.Sprintf("ALTER DATABASE %s SET TABLESPACE %s", dbName, pq.QuoteIdentifier(plan.TablespaceName.ValueString()))
		_, err := r.DB.ExecContext(ctx, query)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating database tablespace",
				fmt.Sprintf("Could not update tablespace for database %s: %s", plan.Name.ValueString(), err.Error()),
			)
			return
		}
	}

	// Update connection_limit, allow_connections, is_template via ALTER DATABASE ... WITH
	var withOpts []string
	if !plan.ConnectionLimit.Equal(state.ConnectionLimit) {
		withOpts = append(withOpts, fmt.Sprintf("CONNECTION LIMIT = %d", plan.ConnectionLimit.ValueInt64()))
	}
	if !plan.AllowConnections.Equal(state.AllowConnections) {
		withOpts = append(withOpts, fmt.Sprintf("ALLOW_CONNECTIONS = %t", plan.AllowConnections.ValueBool()))
	}
	if !plan.IsTemplate.Equal(state.IsTemplate) {
		withOpts = append(withOpts, fmt.Sprintf("IS_TEMPLATE = %t", plan.IsTemplate.ValueBool()))
	}

	if len(withOpts) > 0 {
		query := fmt.Sprintf("ALTER DATABASE %s WITH %s", dbName, strings.Join(withOpts, " "))
		_, err := r.DB.ExecContext(ctx, query)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating database",
				fmt.Sprintf("Could not update database %s: %s", plan.Name.ValueString(), err.Error()),
			)
			return
		}
	}

	// Read back the database to populate computed attributes
	diags := r.ReadDatabase(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseResourceModel
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

	dbName := state.Name.ValueString()

	// If the database is a template, we must unset that first
	if state.IsTemplate.ValueBool() {
		query := fmt.Sprintf("ALTER DATABASE %s WITH IS_TEMPLATE = false", pq.QuoteIdentifier(dbName))
		_, err := r.DB.ExecContext(ctx, query)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error disabling template on database",
				fmt.Sprintf("Could not disable template on database %s before dropping: %s", dbName, err.Error()),
			)
			return
		}
	}

	query := fmt.Sprintf("DROP DATABASE %s", pq.QuoteIdentifier(dbName))
	_, err := r.DB.ExecContext(ctx, query)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting database",
			fmt.Sprintf("Could not drop database %s: %s", dbName, err.Error()),
		)
		return
	}
}

func (r *DatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *DatabaseResource) ReadDatabase(ctx context.Context, model *DatabaseResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	dbName := model.Name.ValueString()

	var oid int64
	var owner string
	var encoding string
	var lcCollate string
	var lcCtype string
	var allowConnections bool
	var connectionLimit int64
	var isTemplate bool
	var tablespaceName string

	query := `
		SELECT
			d.oid,
			r.rolname AS owner,
			pg_catalog.pg_encoding_to_char(d.encoding) AS encoding,
			d.datcollate AS lc_collate,
			d.datctype AS lc_ctype,
			d.datallowconn AS allow_connections,
			d.datconnlimit AS connection_limit,
			d.datistemplate AS is_template,
			t.spcname AS tablespace_name
		FROM pg_catalog.pg_database d
		JOIN pg_catalog.pg_roles r ON d.datdba = r.oid
		JOIN pg_catalog.pg_tablespace t ON d.dattablespace = t.oid
		WHERE d.datname = $1
	`

	err := r.DB.QueryRowContext(ctx, query, dbName).Scan(
		&oid,
		&owner,
		&encoding,
		&lcCollate,
		&lcCtype,
		&allowConnections,
		&connectionLimit,
		&isTemplate,
		&tablespaceName,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			diags.AddError("Database not found", fmt.Sprintf("Database %s does not exist.", dbName))
			return diags
		}
		diags.AddError(
			"Error reading database",
			fmt.Sprintf("Could not read database %s: %s", dbName, err.Error()),
		)
		return diags
	}

	model.OID = types.Int64Value(oid)
	model.Owner = types.StringValue(owner)
	model.Encoding = types.StringValue(encoding)
	model.LcCollate = types.StringValue(lcCollate)
	model.LcCtype = types.StringValue(lcCtype)
	model.AllowConnections = types.BoolValue(allowConnections)
	model.ConnectionLimit = types.Int64Value(connectionLimit)
	model.IsTemplate = types.BoolValue(isTemplate)
	model.TablespaceName = types.StringValue(tablespaceName)

	// Template is not stored in pg_database directly; preserve the plan/state value.
	// If it's unknown or null (e.g., during import), set the default.
	if model.Template.IsNull() || model.Template.IsUnknown() {
		model.Template = types.StringValue("template0")
	}

	return diags
}
