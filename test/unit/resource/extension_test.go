package resource_test

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/resource"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/mocks"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/lib/pq"
	"go.uber.org/mock/gomock"
)

const readExtensionSQL = "SELECT n.nspname, e.extversion FROM pg_catalog.pg_extension e JOIN pg_catalog.pg_namespace n ON n.oid = e.extnamespace WHERE e.extname = $1"

func extensionResourceSchema() rschema.Schema {
	r := &resource.ExtensionResource{}
	sresp := &fwresource.SchemaResponse{}
	r.Schema(context.Background(), fwresource.SchemaRequest{}, sresp)
	return sresp.Schema
}

// extensionValue builds a plan or state; nil leaves an attribute null.
func extensionValue(ctx context.Context, s rschema.Schema, database, schemaName, version any, createCascade, dropCascade any) tftypes.Value {
	return tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
		"name":           tftypes.NewValue(tftypes.String, "hstore"),
		"database":       tftypes.NewValue(tftypes.String, database),
		"schema":         tftypes.NewValue(tftypes.String, schemaName),
		"version":        tftypes.NewValue(tftypes.String, version),
		"create_cascade": tftypes.NewValue(tftypes.Bool, createCascade),
		"drop_cascade":   tftypes.NewValue(tftypes.Bool, dropCascade),
		"timeouts":       tftypes.NewValue(timeoutsObjectType, nil),
	})
}

func expectReadExtension(ctrl *gomock.Controller, db *mocks.MockDBTX, schemaName, version string, err error) {
	scanner := mocks.NewMockScanner(ctrl)
	db.EXPECT().QueryRowContext(gomock.Any(), readExtensionSQL, "hstore").Return(scanner)
	scanner.EXPECT().Scan(gomock.Any(), gomock.Any()).DoAndReturn(func(dest ...any) error {
		if err != nil {
			return err
		}
		*dest[0].(*string) = schemaName
		*dest[1].(*string) = version
		return nil
	})
}

func expectCurrentDatabase(ctrl *gomock.Controller, db *mocks.MockDBTX, name string, err error) {
	scanner := mocks.NewMockScanner(ctrl)
	db.EXPECT().QueryRowContext(gomock.Any(), "SELECT current_database()").Return(scanner)
	scanner.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest ...any) error {
		if err != nil {
			return err
		}
		*dest[0].(*string) = name
		return nil
	})
}

func refusingWrapper(code pq.ErrorCode) *common.DBWrapper {
	return &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, &pq.Error{Code: code, Message: "refused"}
	}}
}

func TestExtensionResource_MetadataAndConfigure(t *testing.T) {
	r := &resource.ExtensionResource{}
	mresp := &fwresource.MetadataResponse{}
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "postgresql"}, mresp)
	if mresp.TypeName != "postgresql_extension" {
		t.Errorf("expected postgresql_extension, got %s", mresp.TypeName)
	}

	for name, data := range map[string]any{"nil": nil, "wrapper": common.NewDBWrapper(&sql.DB{})} {
		resp := &fwresource.ConfigureResponse{}
		r.Configure(context.Background(), fwresource.ConfigureRequest{ProviderData: data}, resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("%s: unexpected error: %v", name, resp.Diagnostics.Errors())
		}
	}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(context.Background(), fwresource.ConfigureRequest{ProviderData: "not a db"}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("expected error for wrong provider data type")
	}
}

func TestCreateExtensionStatement(t *testing.T) {
	tests := []struct {
		name  string
		model resource.ExtensionResourceModel
		want  string
	}{
		{"minimal", resource.ExtensionResourceModel{Name: types.StringValue("hstore"), Schema: types.StringUnknown(), Version: types.StringUnknown()},
			`CREATE EXTENSION "hstore"`},
		{"all", resource.ExtensionResourceModel{
			Name: types.StringValue("earthdistance"), Schema: types.StringValue("geo"),
			Version: types.StringValue("1.1'"), CreateCascade: types.BoolValue(true),
		}, `CREATE EXTENSION "earthdistance" WITH SCHEMA "geo" VERSION '1.1''' CASCADE`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resource.CreateExtensionStatement(&tc.model); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAlterExtensionStatements(t *testing.T) {
	state := resource.ExtensionResourceModel{Name: types.StringValue("hstore"), Schema: types.StringValue("public"), Version: types.StringValue("1.7")}
	tests := []struct {
		name            string
		schema, version types.String
		want            []string
	}{
		{"unchanged", types.StringValue("public"), types.StringValue("1.7"), nil},
		{"unknown", types.StringUnknown(), types.StringUnknown(), nil},
		{"both", types.StringValue("ext"), types.StringValue("1.8"), []string{
			`ALTER EXTENSION "hstore" SET SCHEMA "ext"`,
			`ALTER EXTENSION "hstore" UPDATE TO '1.8'`,
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := state
			plan.Schema, plan.Version = tc.schema, tc.version
			if got := resource.AlterExtensionStatements(&plan, &state); !slices.Equal(got, tc.want) {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtensionResource_Create_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockDB.EXPECT().ExecContext(gomock.Any(), `CREATE EXTENSION "hstore" CASCADE`).Return(mockResult{}, nil)
	expectReadExtension(ctrl, mockDB, "public", "1.8", nil)
	expectCurrentDatabase(ctrl, mockDB, "postgres", nil)

	ctx := context.Background()
	s := extensionResourceSchema()
	unknown := tftypes.UnknownValue
	req, resp := newCreateReqResp(ctx, s, extensionValue(ctx, s, unknown, unknown, unknown, true, false))

	r := &resource.ExtensionResource{DB: mockDB}
	r.Create(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.ExtensionResourceModel
	resp.State.Get(ctx, &got)
	if got.Database.ValueString() != "postgres" || got.Schema.ValueString() != "public" || got.Version.ValueString() != "1.8" {
		t.Errorf("unexpected state: %+v", got)
	}
}

func TestExtensionResource_Create_errors(t *testing.T) {
	tests := []struct {
		name   string
		expect func(*gomock.Controller, *mocks.MockDBTX)
		want   string
	}{
		{"exec", func(_ *gomock.Controller, db *mocks.MockDBTX) {
			db.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("could not open extension control file"))
		}, "Error creating extension"},
		{"read back", func(ctrl *gomock.Controller, db *mocks.MockDBTX) {
			db.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil)
			expectReadExtension(ctrl, db, "", "", fmt.Errorf("connection reset"))
		}, "Error reading extension after creation"},
		{"current database", func(ctrl *gomock.Controller, db *mocks.MockDBTX) {
			db.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(mockResult{}, nil)
			expectReadExtension(ctrl, db, "public", "1.8", nil)
			expectCurrentDatabase(ctrl, db, "", fmt.Errorf("connection reset"))
		}, "Error reading extension after creation"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mocks.NewMockDBTX(ctrl)
			tc.expect(ctrl, mockDB)

			ctx := context.Background()
			s := extensionResourceSchema()
			req, resp := newCreateReqResp(ctx, s, extensionValue(ctx, s, nil, nil, nil, false, false))

			r := &resource.ExtensionResource{DB: mockDB}
			r.Create(ctx, req, resp)

			if !hasDiag(resp.Diagnostics, tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, resp.Diagnostics.Errors())
			}
		})
	}
}

func TestExtensionResource_Read_success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	other := mocks.NewMockDBTX(ctrl)
	expectReadExtension(ctrl, other, "ext", "1.8", nil)

	ctx := context.Background()
	s := extensionResourceSchema()
	state := extensionValue(ctx, s, "otherdb", "public", "1.7", nil, nil)
	req, resp := newReadReqResp(ctx, s, state)

	r := &resource.ExtensionResource{DB: &switchingDB{MockDBTX: mockDB, others: map[string]common.DBTX{"otherdb": other}}}
	r.Read(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
	}
	var got resource.ExtensionResourceModel
	resp.State.Get(ctx, &got)
	if got.Schema.ValueString() != "ext" || got.Version.ValueString() != "1.8" || got.Database.ValueString() != "otherdb" {
		t.Errorf("expected drift to be read, got %+v", got)
	}
	if got.CreateCascade.IsNull() || got.DropCascade.IsNull() {
		t.Errorf("expected cascade defaults, got %+v", got)
	}
}

func TestExtensionResource_Read_removedWhenGone(t *testing.T) {
	ctx := context.Background()
	s := extensionResourceSchema()
	state := extensionValue(ctx, s, "otherdb", "public", "1.8", false, false)

	t.Run("extension", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		expectReadExtension(ctrl, mockDB, "", "", sql.ErrNoRows)
		req, resp := newReadReqResp(ctx, s, state)
		(&resource.ExtensionResource{DB: mockDB}).Read(ctx, req, resp)
		if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
			t.Fatalf("expected removal, got %v", resp.Diagnostics.Errors())
		}
	})
	t.Run("database", func(t *testing.T) {
		req, resp := newReadReqResp(ctx, s, state)
		(&resource.ExtensionResource{DB: refusingWrapper("3D000")}).Read(ctx, req, resp)
		if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
			t.Fatalf("expected removal, got %v", resp.Diagnostics.Errors())
		}
	})
}

func TestExtensionResource_Read_queryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	expectReadExtension(ctrl, mockDB, "", "", fmt.Errorf("permission denied"))

	ctx := context.Background()
	s := extensionResourceSchema()
	req, resp := newReadReqResp(ctx, s, extensionValue(ctx, s, "postgres", "public", "1.8", false, false))
	(&resource.ExtensionResource{DB: mockDB}).Read(ctx, req, resp)

	if !hasDiag(resp.Diagnostics, "Error reading extension") {
		t.Fatalf("expected 'Error reading extension', got %v", resp.Diagnostics.Errors())
	}
}

func TestExtensionResource_Update(t *testing.T) {
	statements := []string{
		`ALTER EXTENSION "hstore" SET SCHEMA "ext"`,
		`ALTER EXTENSION "hstore" UPDATE TO '1.8'`,
	}
	tests := []struct {
		name   string
		expect func(*gomock.Controller, *mocks.MockDBTX)
		want   string
	}{
		{"success", func(ctrl *gomock.Controller, db *mocks.MockDBTX) {
			expectTx(ctrl, db, statements, -1)
			expectReadExtension(ctrl, db, "ext", "1.8", nil)
		}, ""},
		{"version fails", func(ctrl *gomock.Controller, db *mocks.MockDBTX) {
			expectTx(ctrl, db, statements, 1)
		}, "Error updating extension"},
		{"read back", func(ctrl *gomock.Controller, db *mocks.MockDBTX) {
			expectTx(ctrl, db, statements, -1)
			expectReadExtension(ctrl, db, "", "", fmt.Errorf("connection reset"))
		}, "Error reading extension after update"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mocks.NewMockDBTX(ctrl)
			tc.expect(ctrl, mockDB)

			ctx := context.Background()
			s := extensionResourceSchema()
			plan := extensionValue(ctx, s, "postgres", "ext", "1.8", false, true)
			state := extensionValue(ctx, s, "postgres", "public", "1.7", false, false)
			req, resp := newUpdateReqResp(ctx, s, plan, state)
			(&resource.ExtensionResource{DB: mockDB}).Update(ctx, req, resp)

			if tc.want == "" {
				if resp.Diagnostics.HasError() {
					t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
				}
				var got resource.ExtensionResourceModel
				resp.State.Get(ctx, &got)
				if got.Schema.ValueString() != "ext" || got.Version.ValueString() != "1.8" || !got.DropCascade.ValueBool() {
					t.Errorf("unexpected state: %+v", got)
				}
				return
			}
			if !hasDiag(resp.Diagnostics, tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, resp.Diagnostics.Errors())
			}
		})
	}
}

func TestExtensionResource_Delete_statement(t *testing.T) {
	for cascade, want := range map[bool]string{
		false: `DROP EXTENSION IF EXISTS "hstore"`,
		true:  `DROP EXTENSION IF EXISTS "hstore" CASCADE`,
	} {
		ctrl := gomock.NewController(t)
		mockDB := mocks.NewMockDBTX(ctrl)
		mockDB.EXPECT().ExecContext(gomock.Any(), want).Return(mockResult{}, nil)

		ctx := context.Background()
		s := extensionResourceSchema()
		req, resp := newDeleteReqResp(ctx, s, extensionValue(ctx, s, "postgres", "public", "1.8", false, cascade))
		(&resource.ExtensionResource{DB: mockDB}).Delete(ctx, req, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("cascade=%v: unexpected error: %v", cascade, resp.Diagnostics.Errors())
		}
	}
}

func TestExtensionResource_Delete_errors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantHint bool
	}{
		{"dependent objects", &pq.Error{Code: "2BP01", Message: "cannot drop extension hstore"}, true},
		{"other", fmt.Errorf("permission denied"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mocks.NewMockDBTX(ctrl)
			mockDB.EXPECT().ExecContext(gomock.Any(), gomock.Any()).Return(nil, tc.err)

			ctx := context.Background()
			s := extensionResourceSchema()
			req, resp := newDeleteReqResp(ctx, s, extensionValue(ctx, s, "postgres", "public", "1.8", false, false))
			(&resource.ExtensionResource{DB: mockDB}).Delete(ctx, req, resp)

			if !hasDiag(resp.Diagnostics, "Error deleting extension") {
				t.Fatalf("expected 'Error deleting extension', got %v", resp.Diagnostics.Errors())
			}
			if got := strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "drop_cascade = true"); got != tc.wantHint {
				t.Errorf("hint shown = %v, want %v", got, tc.wantHint)
			}
		})
	}
}

func TestExtensionResource_ConnectErrors(t *testing.T) {
	ctx := context.Background()
	s := extensionResourceSchema()
	state := extensionValue(ctx, s, "otherdb", "public", "1.8", false, false)
	r := &resource.ExtensionResource{DB: refusingWrapper("08006")}

	createReq, createResp := newCreateReqResp(ctx, s, state)
	r.Create(ctx, createReq, createResp)
	readReq, readResp := newReadReqResp(ctx, s, state)
	r.Read(ctx, readReq, readResp)
	updReq, updResp := newUpdateReqResp(ctx, s, state, state)
	r.Update(ctx, updReq, updResp)
	delReq, delResp := newDeleteReqResp(ctx, s, state)
	r.Delete(ctx, delReq, delResp)

	for name, tc := range map[string]struct {
		diags diag.Diagnostics
		want  string
	}{
		"create": {createResp.Diagnostics, "Error connecting to database"},
		"read":   {readResp.Diagnostics, "Error reading extension"},
		"update": {updResp.Diagnostics, "Error connecting to database"},
		"delete": {delResp.Diagnostics, "Error connecting to database"},
	} {
		if !hasDiag(tc.diags, tc.want) {
			t.Errorf("%s: expected %q, got %v", name, tc.want, tc.diags.Errors())
		}
	}
}

func TestExtensionResource_ImportState(t *testing.T) {
	for id, wantDB := range map[string]string{"hstore": "", "otherdb/hstore": "otherdb"} {
		ctx := context.Background()
		s := extensionResourceSchema()
		req, resp := newImportReqResp(ctx, s, id)
		(&resource.ExtensionResource{}).ImportState(ctx, req, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("%s: unexpected error: %v", id, resp.Diagnostics.Errors())
		}
		var got resource.ExtensionResourceModel
		resp.State.Get(ctx, &got)
		if got.Name.ValueString() != "hstore" || got.Database.ValueString() != wantDB || got.DropCascade.ValueBool() || got.CreateCascade.IsNull() {
			t.Errorf("%s: unexpected state: %+v", id, got)
		}
	}
}
