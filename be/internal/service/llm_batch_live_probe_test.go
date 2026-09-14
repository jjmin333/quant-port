package service

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

// 실 DB에 붙어 **대상 선정만** 돌려 본다. LLM은 부르지 않는다.
//
//	go test ./internal/service/ -run TestLiveBatchSelection -v -count=1
//	  (PROBE_MYSQL_DSN 이 있어야 돈다)
func TestLiveBatchSelection(t *testing.T) {
	dsn := os.Getenv("PROBE_MYSQL_DSN")
	if dsn == "" {
		t.Skip("PROBE_MYSQL_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	svc := &StockSummaryService{
		stocks:        repository.NewStockRepo(db),
		rankings:      repository.NewMarketRankingsRepo(db),
		scoreRankings: repository.NewStockScoreRankingRepo(db),
		narratives:    repository.NewStockSummaryNarrativeRepo(db),
		holdings:      repository.NewUserHoldingsRepo(db),
		watchlist:     repository.NewWatchlistRepo(db),
	}

	ctx := context.Background()
	for _, market := range []toss.MarketCountry{toss.MarketKR, toss.MarketUS} {
		selection, err := svc.selectBatchTargets(ctx, market, []string{"005930", "000660"})
		if err != nil {
			t.Fatalf("%s: %v", market, err)
		}
		t.Logf("%s: considered=%d targets=%d bySource=%v excluded=%v",
			market, selection.Considered, len(selection.Targets), selection.BySource, selection.Excluded)

		firstOfSource := map[string]string{}
		for _, target := range selection.Targets {
			if _, ok := firstOfSource[target.Source]; !ok {
				firstOfSource[target.Source] = target.Symbol + " " + target.Name
			}
		}
		t.Logf("%s: 층별 첫 종목 %v", market, firstOfSource)

		head := selection.Targets
		if len(head) > 40 {
			head = head[:40]
		}
		for i, target := range head {
			t.Logf("  %3d %-8s %-10s %s", i+1, target.Symbol, target.Source, target.Name)
		}

		// 같은 종목이 두 번 들어가면 그만큼 쿼터가 새는 것이다.
		seen := map[string]bool{}
		for _, target := range selection.Targets {
			if seen[target.Symbol] {
				t.Fatalf("%s: duplicate target %s", market, target.Symbol)
			}
			seen[target.Symbol] = true
		}
		// 예산이 상한에서 잘릴 자리를 확인한다 — 꼬리는 회전 큐로 넘어간다.
		dailyCap := model.DefaultLLMSettings().DailyCap
		if len(selection.Targets) > dailyCap {
			cut := selection.Targets[dailyCap-1]
			t.Logf("%s: %d번째(마지막 계획) = %s %s", market, dailyCap, cut.Symbol, cut.Source)
		}

		// 관리자 화면이 보는 것과 같은 경로 — 저장된 설정으로 예산까지 적용한다.
		// LLM 풀이 없으므로 헤드룸은 무제한이고 DailyCap만 걸린다.
		svc.settings = NewLLMSettingsStore(repository.NewLLMSettingsRepo(db))
		preview, err := svc.PreviewBatchTargets(ctx, market)
		if err != nil {
			t.Fatalf("%s preview: %v", market, err)
		}
		t.Logf("%s preview: targets=%d bySource=%v excluded=%v",
			market, len(preview.Targets), preview.BySource, preview.Excluded)
	}
}
