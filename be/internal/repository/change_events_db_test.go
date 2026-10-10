package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// 실제 DB로 「오늘의 변화」 목록 순서 — CHANGE_TEST_DSN이 있을 때만(읽기만).
// 시가총액 내림차순 · 같은 종목 줄이 이어짐 · limit이 종목 수라는 세 가지를 본다.
func TestChangeListMarketCapOrderDB(t *testing.T) {
	dsn := os.Getenv("CHANGE_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_TEST_DSN 없음 — DB 시험은 건너뛴다")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewChangeEventRepo(db)
	ctx := context.Background()
	dates, err := repo.Dates(ctx, 1)
	if err != nil || len(dates) == 0 {
		t.Skipf("변화 기록 없음 (%v)", err)
	}
	for _, market := range []string{"domestic", "us"} {
		items, err := repo.List(ctx, ChangeListQuery{Date: dates[0], AssetType: market, Limit: 5})
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		prevCap := -1.0
		for i, e := range items {
			if i > 0 && items[i-1].Symbol != e.Symbol {
				if seen[e.Symbol] {
					t.Fatalf("%s: %s 줄이 흩어져 있다", market, e.Symbol)
				}
			}
			if !seen[e.Symbol] {
				if prevCap >= 0 && e.MarketCap > prevCap {
					t.Fatalf("%s: 시총 순서가 아니다 %s %.0f > %.0f", market, e.Symbol, e.MarketCap, prevCap)
				}
				prevCap = e.MarketCap
			}
			seen[e.Symbol] = true
			if e.AssetType != market {
				t.Fatalf("%s 목록에 %s(%s)", market, e.Symbol, e.AssetType)
			}
		}
		if len(seen) > 5 {
			t.Fatalf("%s: limit 5인데 종목 %d개", market, len(seen))
		}
		t.Logf("%s %s: %d줄 %d종목", dates[0], market, len(items), len(seen))
	}
}
