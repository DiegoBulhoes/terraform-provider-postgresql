package resource_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/resource"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/mocks"
	"go.uber.org/mock/gomock"
)

func createDatabaseQuery(t *testing.T, tablespace string) string {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	var query string
	mockDB.EXPECT().
		ExecContext(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, q string, _ ...any) (any, error) {
			query = q
			return nil, fmt.Errorf("stop after capturing the statement")
		})

	ctx := context.Background()
	s := databaseResourceSchema()
	plan := databasePlanValue(ctx, s, "testdb", "postgres", "template0",
		"UTF8", "en_US.UTF-8", "en_US.UTF-8", tablespace, -1, true, false)
	req, resp := newCreateReqResp(ctx, s, plan)

	r := &resource.DatabaseResource{DB: mockDB}
	r.Create(ctx, req, resp)

	return query
}

func TestDatabaseResource_Create_omitsDefaultTablespace(t *testing.T) {
	query := createDatabaseQuery(t, "pg_default")

	if query == "" {
		t.Fatal("no CREATE DATABASE statement was captured")
	}
	if strings.Contains(query, "TABLESPACE") {
		t.Errorf("CREATE DATABASE must not name the default tablespace, got: %s", query)
	}
}

func TestDatabaseResource_Create_includesCustomTablespace(t *testing.T) {
	query := createDatabaseQuery(t, "fastdisk")

	if query == "" {
		t.Fatal("no CREATE DATABASE statement was captured")
	}
	if !strings.Contains(query, `TABLESPACE = "fastdisk"`) {
		t.Errorf("CREATE DATABASE must name a non-default tablespace, got: %s", query)
	}
}
