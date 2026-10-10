package service

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/repository"
)

// 실제 DB로 관전 포인트 사실표 — ASSET_MENTION_TEST_DSN이 있을 때만(읽기만).
// 2026-10-10: stock_daily_closes의 열은 close_price인데 `SELECT close`라 사실표가 매번 실패했고, 관전 포인트가 0건이었다.
func TestEarningsPreviewFactsDB(t *testing.T) {
	dsn := os.Getenv("ASSET_MENTION_TEST_DSN")
	if dsn == "" {
		t.Skip("ASSET_MENTION_TEST_DSN 없음")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.NewEarningsPreviewRepo(db)
	ctx := context.Background()
	from, to := earningsPreviewWindow(time.Now())
	ts, err := repo.Targets(ctx, "us", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) == 0 {
		t.Skip("다가오는 해외 발표 없음")
	}
	if _, err := repo.Facts(ctx, ts[0], time.Now().In(kstLoc())); err != nil {
		t.Fatalf("사실표 %s: %v", ts[0].Symbol, err)
	}
}
