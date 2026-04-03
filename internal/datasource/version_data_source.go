package datasource

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*versionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*versionDataSource)(nil)
)

type versionDataSource struct {
	db common.DBTX
}

type versionDataSourceModel struct {
	Version          types.String `tfsdk:"version"`
	Major            types.Int64  `tfsdk:"major"`
	Minor            types.Int64  `tfsdk:"minor"`
	ServerVersionNum types.Int64  `tfsdk:"server_version_num"`
}

func NewVersionDataSource() datasource.DataSource {
	return &versionDataSource{}
}

func (d *versionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_version"
}

func (d *versionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Exposes the PostgreSQL server version information.",
		Attributes: map[string]schema.Attribute{
			"version": schema.StringAttribute{
				Description: "The full version string returned by the server (e.g. PostgreSQL 16.2).",
				Computed:    true,
			},
			"major": schema.Int64Attribute{
				Description: "The major version number (e.g. 16).",
				Computed:    true,
			},
			"minor": schema.Int64Attribute{
				Description: "The minor version number (e.g. 2).",
				Computed:    true,
			},
			"server_version_num": schema.Int64Attribute{
				Description: "The numeric server version (e.g. 160002).",
				Computed:    true,
			},
		},
	}
}

func (d *versionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	db, err := common.ConfigureDB(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", err.Error())
		return
	}
	d.db = db
}

var versionRegexp = regexp.MustCompile(`(\d+)\.(\d+)`)

func (d *versionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state versionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var versionStr string
	err := d.db.QueryRowContext(ctx, `SELECT version()`).Scan(&versionStr)
	if err != nil {
		resp.Diagnostics.AddError("Error querying version", fmt.Sprintf("Could not query server version: %s", err.Error()))
		return
	}
	state.Version = types.StringValue(versionStr)

	matches := versionRegexp.FindStringSubmatch(versionStr)
	if len(matches) >= 3 {
		major, _ := strconv.ParseInt(matches[1], 10, 64)
		minor, _ := strconv.ParseInt(matches[2], 10, 64)
		state.Major = types.Int64Value(major)
		state.Minor = types.Int64Value(minor)
	} else {
		state.Major = types.Int64Value(0)
		state.Minor = types.Int64Value(0)
	}

	var serverVersionNum string
	err = d.db.QueryRowContext(ctx, `SHOW server_version_num`).Scan(&serverVersionNum)
	if err != nil {
		resp.Diagnostics.AddError("Error querying server_version_num", fmt.Sprintf("Could not query server_version_num: %s", err.Error()))
		return
	}

	num, err := strconv.ParseInt(serverVersionNum, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Error parsing server_version_num", fmt.Sprintf("Could not parse server_version_num %q: %s", serverVersionNum, err.Error()))
		return
	}
	state.ServerVersionNum = types.Int64Value(num)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
