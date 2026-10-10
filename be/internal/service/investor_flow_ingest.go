package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
)

/*
InvestorFlowIngestService — 국내 보통주의 투자자별 순매수(외국인·기관·개인)를 받아
`stock_investor_flows`에 둔다. 심리·수급 축의 수급 재료다(2026-09-24 사용자 결정).

# 호출량 (사용자에게 먼저 보고한 숫자)

	대상   국내 보통주 약 2,760 (ETF·ETN·우선주 제외 — 랭킹과 같은 규칙)
	매일   종목당 1호출 ≈ 2,760호출 · 0.5초 간격 ≈ 23분
	백필   처음 보는 종목은 한 호출로 60일을 받는다(호출 수는 매일과 같다)

버킷은 `STOCK_TRADING_TREND` — 사용자 시세가 쓰는 `MARKET_DATA`와도, 경고 스윕이 쓰는
`STOCK`과도 다르다. 그 버킷의 한도는 공표되지 않아 첫 호출의 헤더로 로그에 남긴다
(`toss.logFirstSeenLimit`). 0.5초 간격은 그 한도를 모르는 상태의 보수적인 값이다.

# 확정치만

장중 잠정 기록은 개인이 비어 있다. 개인 값이 있는 기록만 저장한다 — 잠정치를 넣으면
같은 날이 저녁 전후로 다른 값이 된다. 20:30에 돌리므로 보통 그날까지 확정이고, 못 받은
날은 다음 회차의 5일 창이 메운다.
*/
type InvestorFlowIngestService struct {
	toss *toss.Client
	repo *repository.InvestorFlowRepo
	sync *repository.SyncStateRepo
}

func NewInvestorFlowIngestService(tc *toss.Client, repo *repository.InvestorFlowRepo, syncState *repository.SyncStateRepo) *InvestorFlowIngestService {
	return &InvestorFlowIngestService{toss: tc, repo: repo, sync: syncState}
}

const (
	investorFlowPause       = 500 * time.Millisecond
	investorFlowRetryPause  = 5 * time.Second
	investorFlowBackfill    = 60
	investorFlowCatchUp     = 30
	investorFlowDaily       = 5
	investorFlowRetention   = 400 * 24 * time.Hour
	investorFlowSelectLimit = 6000
)

// Run fetches every target symbol. limit>0 caps the symbol count (manual runs).
func (s *InvestorFlowIngestService) Run(ctx context.Context, limit int) (map[string]any, string, error) {
	if s == nil || s.toss == nil || !s.toss.Enabled() {
		return nil, "", fmt.Errorf("investor flow ingest: toss not configured")
	}
	targets, err := s.repo.DomesticCommonSymbols(ctx)
	if err != nil {
		return nil, "", err
	}
	// 로테이션 커서(오래된 순 · not_found 30일 쿨다운 · 토스 미취급 제외)를 빌리고,
	// 보통주 집합으로 거른다.
	ordered, err := s.sync.SelectTargets(ctx, model.SyncKindInvestorFlow,
		[]string{string(model.AssetDomestic)}, 0, investorFlowSelectLimit)
	if err != nil {
		return nil, "", fmt.Errorf("investor flow ingest: select targets: %w", err)
	}
	symbols := make([]string, 0, len(targets))
	for _, sym := range ordered {
		if targets[sym] {
			symbols = append(symbols, sym)
		}
	}
	if limit > 0 && len(symbols) > limit {
		symbols = symbols[:limit]
	}
	latest, err := s.repo.LatestDates(ctx)
	if err != nil {
		return nil, "", err
	}

	stats := newIngestStats()
	now := time.Now()
	for i, symbol := range symbols {
		if ctx.Err() != nil {
			break
		}
		if i > 0 {
			select {
			case <-ctx.Done():
				return stats.finish("investor_flow", len(symbols))
			case <-time.After(investorFlowPause):
			}
		}
		s.ingestOne(ctx, symbol, flowFetchCount(latest[symbol], now), stats)
	}
	res, msg, err := stats.finish("investor_flow", len(symbols))
	cutoff := now.Add(-investorFlowRetention).Format("2006-01-02")
	if n, perr := s.repo.Prune(context.WithoutCancel(ctx), cutoff); perr == nil && res != nil {
		res["pruned"] = n
	}
	return res, msg, err
}

// flowFetchCount — 처음 보는 종목은 60일, 열흘 넘게 빈 종목은 30일, 나머지는 5일.
func flowFetchCount(latest string, now time.Time) int {
	if latest == "" {
		return investorFlowBackfill
	}
	d, err := time.Parse("2006-01-02", latest)
	if err != nil || now.Sub(d) > 10*24*time.Hour {
		return investorFlowCatchUp
	}
	return investorFlowDaily
}

func (s *InvestorFlowIngestService) ingestOne(ctx context.Context, symbol string, count int, stats *ingestStats) {
	resp, err := s.toss.GetStockInvestorTrading(ctx, symbol, count, "")
	if err != nil && isRateLimitError(err) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(investorFlowRetryPause):
		}
		resp, err = s.toss.GetStockInvestorTrading(ctx, symbol, count, "")
	}
	if err != nil {
		status := model.SyncError
		if toss.IsStockNotFound(err) || strings.Contains(err.Error(), "unsupported-market") {
			status = model.SyncNotFound
		}
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: model.SyncKindInvestorFlow, Symbol: symbol, Status: status, Message: err.Error(),
		})
		return
	}
	var records []toss.StockInvestorTradingRecord
	if resp != nil {
		records = resp.Records
	}
	rows := flowRowsFromRecords(records)
	written, err := s.repo.Upsert(ctx, symbol, rows)
	if err != nil {
		stats.record(ctx, s.sync, repository.SyncOutcome{
			Kind: model.SyncKindInvestorFlow, Symbol: symbol, Status: model.SyncError, Message: err.Error(),
		})
		return
	}
	status := model.SyncEmpty
	if written > 0 {
		status = model.SyncOK
	}
	stats.record(ctx, s.sync, repository.SyncOutcome{
		Kind: model.SyncKindInvestorFlow, Symbol: symbol, Status: status,
		RowsStored: written, ItemsSeen: len(records),
	})
}

// flowRowsFromRecords keeps confirmed days only (개인 값이 있는 기록) and parses
// the share counts. 숫자가 아닌 값이 든 기록은 통째로 버린다 — 0으로 읽으면
// 「순매수 없음」이라는 거짓 관측이 된다.
func flowRowsFromRecords(records []toss.StockInvestorTradingRecord) []repository.InvestorFlowRow {
	out := make([]repository.InvestorFlowRow, 0, len(records))
	for _, rec := range records {
		if rec.Individual == nil || rec.Date == "" {
			continue
		}
		fr, ok1 := parseShares(rec.Foreigner.NetBuyVolume)
		in, ok2 := parseShares(rec.Institution.NetBuyVolume)
		ind, ok3 := parseShares(rec.Individual.NetBuyVolume)
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		row := repository.InvestorFlowRow{TradeDate: rec.Date, ForeignNet: fr, InstitutionNet: in, IndividualNet: &ind}
		if rec.ForeignerHolding != nil {
			if v, err := strconv.ParseFloat(strings.TrimSpace(rec.ForeignerHolding.HoldingRate), 64); err == nil {
				row.ForeignHoldRate = &v
			}
		}
		out = append(out, row)
	}
	return out
}

func parseShares(s string) (int64, bool) {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return v, err == nil
}
