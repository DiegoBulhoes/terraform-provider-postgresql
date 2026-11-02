package common_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/mocks"
	"go.uber.org/mock/gomock"
)

// ---------------------------------------------------------------------------
// ExecInTx
// ---------------------------------------------------------------------------

func TestExecInTx_emptyRunsNothing(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	if err := common.ExecInTx(context.Background(), mockDB, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecInTx_beginError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(nil, errors.New("too many connections"))

	err := common.ExecInTx(context.Background(), mockDB, []string{"SELECT 1"})
	if err == nil || !strings.Contains(err.Error(), "too many connections") {
		t.Fatalf("expected begin error, got %v", err)
	}
}

func TestExecInTx_execErrorRollsBack(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	boom := errors.New("boom")
	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	// No STMT 3 and no Commit: the transaction stops at the first failure.
	gomock.InOrder(
		mockTx.EXPECT().ExecContext(gomock.Any(), "STMT 1").Return(noopResult{}, nil),
		mockTx.EXPECT().ExecContext(gomock.Any(), "STMT 2").Return(nil, boom),
		mockTx.EXPECT().Rollback().Return(nil),
	)

	err := common.ExecInTx(context.Background(), mockDB, []string{"STMT 1", "STMT 2", "STMT 3"})
	if !errors.Is(err, boom) {
		t.Fatalf("expected the exec error, got %v", err)
	}
}

func TestExecInTx_commits(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	gomock.InOrder(
		mockTx.EXPECT().ExecContext(gomock.Any(), "STMT 1").Return(noopResult{}, nil),
		mockTx.EXPECT().ExecContext(gomock.Any(), "STMT 2").Return(noopResult{}, nil),
		mockTx.EXPECT().Commit().Return(nil),
		mockTx.EXPECT().Rollback().Return(sql.ErrTxDone),
	)

	if err := common.ExecInTx(context.Background(), mockDB, []string{"STMT 1", "STMT 2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecInTx_commitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)
	mockTx := mocks.NewMockTx(ctrl)

	mockDB.EXPECT().BeginTx(gomock.Any(), gomock.Any()).Return(mockTx, nil)
	mockTx.EXPECT().ExecContext(gomock.Any(), "STMT 1").Return(noopResult{}, nil)
	mockTx.EXPECT().Commit().Return(errors.New("serialization failure"))
	mockTx.EXPECT().Rollback().Return(nil)

	err := common.ExecInTx(context.Background(), mockDB, []string{"STMT 1"})
	if err == nil || !strings.Contains(err.Error(), "serialization failure") {
		t.Fatalf("expected commit error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// DBWrapper.ForDatabase / ReleaseDatabase / Close
// ---------------------------------------------------------------------------

func TestDBWrapper_ForDatabase_sameWrapper(t *testing.T) {
	opened := 0
	open := func(context.Context, string) (*sql.DB, error) {
		opened++
		return nil, errors.New("must not open")
	}
	tests := []struct {
		name     string
		w        *common.DBWrapper
		database string
	}{
		{"empty name", &common.DBWrapper{Database: "postgres", Open: open}, ""},
		{"same database", &common.DBWrapper{Database: "postgres", Open: open}, "postgres"},
		{"nil Open", &common.DBWrapper{Database: "postgres"}, "other"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.w.ForDatabase(context.Background(), tc.database)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.w {
				t.Errorf("expected the same wrapper, got %v", got)
			}
		})
	}
	if opened != 0 {
		t.Errorf("expected Open not to be called, called %d times", opened)
	}
}

func TestDBWrapper_ForDatabase_openError(t *testing.T) {
	w := &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, errors.New("no such database")
	}}
	got, err := w.ForDatabase(context.Background(), "other")
	if err == nil {
		t.Fatal("expected error")
	}
	if got != nil {
		t.Errorf("expected nil connection, got %v", got)
	}
	if !strings.Contains(err.Error(), `"other"`) || !strings.Contains(err.Error(), "no such database") {
		t.Errorf("unexpected error: %q", err.Error())
	}
}

func TestDBWrapper_ForDatabase_cachesAndReleases(t *testing.T) {
	var opened []string
	w := &common.DBWrapper{DB: fakeDB(t), Database: "postgres",
		Open: func(_ context.Context, database string) (*sql.DB, error) {
			opened = append(opened, database)
			return fakeDB(t), nil
		}}
	ctx := context.Background()

	first, err := w.ForDatabase(ctx, "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p, ok := first.(*common.DBWrapper)
	if !ok || p == w {
		t.Fatalf("expected a new wrapper, got %T", first)
	}
	if p.Database != "app" {
		t.Errorf("expected database app, got %q", p.Database)
	}
	second, err := w.ForDatabase(ctx, "app")
	if err != nil || second != first {
		t.Fatalf("expected the cached pool, got %v, %v", second, err)
	}

	if err := w.ReleaseDatabase("app"); err != nil {
		t.Fatalf("unexpected release error: %v", err)
	}
	third, err := w.ForDatabase(ctx, "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if third == first {
		t.Error("expected a new pool after release")
	}
	if len(opened) != 2 {
		t.Errorf("expected 2 opens, got %v", opened)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if err := w.ReleaseDatabase("app"); err != nil {
		t.Errorf("expected release after close to be a no-op, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// OnDatabase / ReleaseDatabase
// ---------------------------------------------------------------------------

func TestOnDatabase_notSwitcher(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mocks.NewMockDBTX(ctrl)

	got, err := common.OnDatabase(context.Background(), mockDB, "other")
	if err != nil || got != mockDB {
		t.Fatalf("expected the same connection, got %v, %v", got, err)
	}
	if err := common.ReleaseDatabase(mockDB, "other"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOnDatabase_switcher(t *testing.T) {
	w := &common.DBWrapper{Database: "postgres", Open: func(context.Context, string) (*sql.DB, error) {
		return nil, errors.New("refused")
	}}
	if _, err := common.OnDatabase(context.Background(), w, "other"); err == nil {
		t.Fatal("expected the switcher's error")
	}
	if err := common.ReleaseDatabase(w, "other"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
