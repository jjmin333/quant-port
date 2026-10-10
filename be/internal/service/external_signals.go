package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
ExternalSignalService — 외부 신호 세 잡 (2026-09-28 사용자 결정, 요청 6·7). worker-filings만 부른다.

	ext_metrics_overseas  해외 등급제 로테이션 1,500종목/일 · 종목당 Finnhub 2호출(지표 + 추천 추이)
	                      · 3초 간격(finnhub_metrics 버킷) ≈ 2.5시간. 한 바퀴 약 5일.
	analyst_domestic      국내 보통주 전 종목 매일 · 네이버 1호출 · 1.2초 ≈ 55분.
	social_mentions       ApeWisdom 전 종목 목록(7페이지 남짓) 하루 한 번.

지표 보조는 **EDGAR 값이 없을 때만** 점수·화면에 쓰인다(`stock_score.go`의 loadUniverse,
`fundamentals.go`의 GetWithValuation). 비율은 본국 상장 기준이라 ADR에도 맞다.
*/
type ExternalSignalService struct {
	finnhub *marketdata.FinnhubClient
	naver   *marketdata.NaverStockClient
	ape     *marketdata.ApeWisdomClient
	repo    *repository.ExternalSignalRepo
	flows   *repository.InvestorFlowRepo
	sync    *repository.SyncStateRepo
	budget  *apibudget.Manager
	// reddit·translator — 커뮤니티 탭 글(2026-09-29, social_reddit.go). 없으면 social_posts 잡만 꺼진다.
	reddit     *marketdata.RedditClient
	translator Translator
}

func NewExternalSignalService(
	finnhub *marketdata.FinnhubClient, naver *marketdata.NaverStockClient, ape *marketdata.ApeWisdomClient,
	repo *repository.ExternalSignalRepo, flows *repository.InvestorFlowRepo, sync *repository.SyncStateRepo,
	budget *apibudget.Manager,
) *ExternalSignalService {
	return &ExternalSignalService{finnhub: finnhub, naver: naver, ape: ape, repo: repo, flows: flows, sync: sync, budget: budget}
}

const (
	extMetricsDailyLimit = 1500
	extMetricsStaleAfter = 5 * 24 * time.Hour
	// ExtMetricsMaxAge — 이보다 오래된 보조 값은 점수·화면에 쓰지 않는다(로테이션이 멈춘 경우의 안전판).
	ExtMetricsMaxAge = 21 * 24 * time.Hour
	extRetryPause    = 20 * time.Second
)

// pace sleeps the bucket's MinInterval — apibudget.Check는 기다려 주지 않는다.
func (s *ExternalSignalService) pace(ctx context.Context, p apibudget.Provider) error {
	d := time.Second
	if s.budget != nil {
		if iv := s.budget.LimitsFor(p).MinInterval; iv > 0 {
			d = iv + 50*time.Millisecond
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// RunOverseasMetrics — 해외 지표 보조 + 추천 추이 로테이션.
func (s *ExternalSignalService) RunOverseasMetrics(ctx context.Context, limit int) (map[string]any, string, error) {
	if s == nil || s.finnhub == nil || !s.finnhub.Enabled() {
		return nil, "", fmt.Errorf("ext metrics: finnhub not configured")
	}
	if limit <= 0 {
		limit = extMetricsDailyLimit
	}
	symbols, err := s.sync.SelectTargets(ctx, model.SyncKindExtMetrics,
		[]string{string(model.AssetUS)}, extMetricsStaleAfter, limit)
	if err != nil {
		return nil, "", fmt.Errorf("ext metrics: select targets: %w", err)
	}
	stats := newIngestStats()
	done := 0
	for i, sym := range symbols {
		if i > 0 && s.pace(ctx, apibudget.ProviderFinnhubMetrics) != nil {
			break
		}
		m, err := s.finnhub.Metrics(ctx, sym)
		if err != nil && isBudgetOr429(err) {
			if !sleepCtx(ctx, extRetryPause) {
				break
			}
			m, err = s.finnhub.Metrics(ctx, sym)
		}
		out := repository.SyncOutcome{Kind: model.SyncKindExtMetrics, Symbol: sym}
		switch {
		case errors.Is(err, marketdata.ErrExternalNotFound):
			out.Status = model.SyncEmpty
		case err != nil:
			out.Status, out.Message = model.SyncError, err.Error()
		default:
			if werr := s.repo.UpsertExtMetrics(ctx, repository.ExtMetrics{
				Symbol: sym, Source: "finnhub", PER: m.PER, PBR: m.PBR, PSR: m.PSR, ROE: m.ROE,
				OperatingMargin: m.OperatingMargin, NetMargin: m.NetMargin, RevenueGrowth: m.RevenueGrowth,
				EPSGrowth: m.EPSGrowth, CurrentRatio: m.CurrentRatio, DividendYield: m.DividendYield, Beta: m.Beta,
			}); werr != nil {
				out.Status, out.Message = model.SyncError, werr.Error()
				break
			}
			out.Status, out.RowsStored = model.SyncOK, 1
			// 지표가 있는 종목만 추천 추이를 묻는다 — 없는 종목(소형·신규)은 대개 애널리스트도 없다.
			if s.pace(ctx, apibudget.ProviderFinnhubMetrics) != nil {
				break
			}
			if recs, rerr := s.finnhub.Recommendation(ctx, sym); rerr == nil {
				rec := recs[0] // 최신 달
				c := repository.AnalystConsensus{Symbol: sym, Source: "finnhub", AsOf: rec.Period}
				c.StrongBuy, c.Buy, c.Hold, c.Sell, c.StrongSell = &rec.StrongBuy, &rec.Buy, &rec.Hold, &rec.Sell, &rec.StrongSell
				if r, ok := marketdata.RecommendationRating(rec); ok {
					c.Rating = &r
				}
				if werr := s.repo.UpsertConsensus(ctx, c); werr == nil {
					out.RowsStored++
				}
				// 이력(099): 응답에 온 달을 전부 남긴다 — 예전엔 최신 달만 쓰고 나머지를 버렸다.
				for _, m := range recs {
					if werr := s.repo.UpsertAnalystHistory(ctx, sym, finnhubHistoryRow(m)); werr == nil {
						out.RowsStored++
					}
				}
			}
		}
		stats.record(ctx, s.sync, out)
		done++
	}
	// 중간에 멈췄으면(재배포·취소) 실제로 처리한 수를 적는다 — 대상 수를 적으면 다 돈 것처럼 읽힌다.
	return stats.finish("ext_metrics", done)
}

// RunDomesticAnalyst — 국내 보통주 전 종목의 평균 목표가·투자의견.
func (s *ExternalSignalService) RunDomesticAnalyst(ctx context.Context, limit int) (map[string]any, string, error) {
	if s == nil || s.naver == nil {
		return nil, "", fmt.Errorf("analyst domestic: not configured")
	}
	common, err := s.flows.DomesticCommonSymbols(ctx)
	if err != nil {
		return nil, "", err
	}
	ordered, err := s.sync.SelectTargets(ctx, model.SyncKindAnalystNaver,
		[]string{string(model.AssetDomestic)}, 0, 6000)
	if err != nil {
		return nil, "", fmt.Errorf("analyst domestic: select targets: %w", err)
	}
	var symbols []string
	for _, sym := range ordered {
		if common[sym] {
			symbols = append(symbols, sym)
		}
	}
	if limit > 0 && len(symbols) > limit {
		symbols = symbols[:limit]
	}
	stats := newIngestStats()
	done := 0
	for i, sym := range symbols {
		if i > 0 && s.pace(ctx, apibudget.ProviderNaverStock) != nil {
			break
		}
		c, err := s.naver.Consensus(ctx, sym)
		if err != nil && isBudgetOr429(err) {
			if !sleepCtx(ctx, extRetryPause) {
				break
			}
			c, err = s.naver.Consensus(ctx, sym)
		}
		out := repository.SyncOutcome{Kind: model.SyncKindAnalystNaver, Symbol: sym}
		switch {
		case errors.Is(err, marketdata.ErrExternalNotFound):
			out.Status = model.SyncEmpty
		case err != nil:
			out.Status, out.Message = model.SyncError, err.Error()
		default:
			out.Status = model.SyncOK
			if c.Rating != nil || c.TargetPrice != nil {
				row := repository.AnalystConsensus{
					Symbol: sym, Source: "naver", AsOf: c.AsOf, Rating: c.Rating, TargetMean: c.TargetPrice,
				}
				if c.TargetPrice != nil {
					row.TargetCurrency = "KRW"
				}
				if werr := s.repo.UpsertConsensus(ctx, row); werr != nil {
					out.Status, out.Message = model.SyncError, werr.Error()
					break
				}
				out.RowsStored++
				// 이력(099): 의견·목표가가 **바뀐 날만** 남긴다. 실패해도 최신 값은 이미 저장됐다.
				if s.storeNaverHistory(ctx, sym, c) {
					out.RowsStored++
				}
			}
			if len(c.Reports) > 0 {
				reps := make([]repository.AnalystReport, 0, len(c.Reports))
				for _, r := range c.Reports {
					reps = append(reps, repository.AnalystReport{ID: r.ID, Source: "naver", Broker: r.Broker, Title: r.Title, Published: r.Date})
				}
				if werr := s.repo.UpsertAnalystReports(ctx, sym, reps); werr == nil {
					out.RowsStored += len(reps)
				}
			}
		}
		stats.record(ctx, s.sync, out)
		done++
	}
	return stats.finish("analyst_naver", done)
}

func finnhubHistoryRow(m marketdata.FinnhubRecommendation) repository.AnalystHistoryRow {
	row := repository.AnalystHistoryRow{
		AsOf: m.Period, Source: "finnhub",
		Counts: &[5]int{m.StrongBuy, m.Buy, m.Hold, m.Sell, m.StrongSell},
	}
	if r, ok := marketdata.RecommendationRating(m); ok {
		row.Rating = &r
	}
	return row
}

// storeNaverHistory — 마지막 이력과 값이 다를 때만 한 줄을 남긴다. 남겼으면 true.
func (s *ExternalSignalService) storeNaverHistory(ctx context.Context, sym string, c marketdata.NaverConsensus) bool {
	if c.AsOf == "" {
		return false
	}
	prev, err := s.repo.LatestAnalystHistory(ctx, sym, "naver")
	if err != nil || !analystValueChanged(prev, c.Rating, c.TargetPrice) {
		return false
	}
	row := repository.AnalystHistoryRow{AsOf: c.AsOf, Source: "naver", Rating: c.Rating, TargetMean: c.TargetPrice}
	if c.TargetPrice != nil {
		row.TargetCurrency = "KRW"
	}
	return s.repo.UpsertAnalystHistory(ctx, sym, row) == nil
}

// analystValueChanged — 이력이 없거나 의견(0.01)·목표가(0.1%)가 움직였으면 true.
func analystValueChanged(prev *repository.AnalystHistoryRow, rating, target *float64) bool {
	if prev == nil {
		return true
	}
	diff := func(a, b *float64, tol func(float64) float64) bool {
		if (a == nil) != (b == nil) {
			return true
		}
		if a == nil {
			return false
		}
		d := *a - *b
		if d < 0 {
			d = -d
		}
		return d > tol(*b)
	}
	return diff(rating, prev.Rating, func(float64) float64 { return 0.005 }) ||
		diff(target, prev.TargetMean, func(b float64) float64 { return b * 0.001 })
}

/*
socialTickerDenylist — 영어 낱말과 같은 티커. 「I think it's ON」 같은 문장이 종목 언급으로
세어진다(ApeWisdom은 티커로만 센다). 우리 화면에 「레딧 급증」으로 올리지 않는다.
*/
var socialTickerDenylist = map[string]bool{
	"A": true, "I": true, "AI": true, "IT": true, "ON": true, "ALL": true, "ARE": true, "BE": true,
	"NOW": true, "CAN": true, "DD": true, "GO": true, "SO": true, "ONE": true, "OR": true, "AM": true,
	"AN": true, "AT": true, "BIG": true, "EDIT": true, "EV": true, "FOR": true, "HAS": true,
	"LOVE": true, "NEW": true, "OUT": true, "PM": true, "REAL": true, "SEE": true, "TWO": true,
	"UP": true, "VS": true, "YOU": true, "CEO": true, "USA": true, "GDP": true, "ATH": true,
	"FUD": true, "IPO": true, "OPEN": true, "NICE": true, "FUN": true, "LIFE": true, "WELL": true,
	"PLAY": true, "RUN": true, "TOP": true, "LOW": true, "HOPE": true, "MOON": true, "CASH": true,
	// 2026-09-28 첫 수집 상위 12개 중 넷이 이런 잡음이었다: DTE(옵션 만기 일수) · IP(지식재산) · JUST(낱말) ·
	// BYD(중국 BYD를 말하는데 카탈로그의 BYD는 보이드 게이밍). 옵션 은어도 같이 뺀다.
	"DTE": true, "IP": true, "JUST": true, "BYD": true, "OTM": true, "ITM": true, "EOD": true, "IMO": true,
	"LOL": true, "WSB": true, "OP": true, "PT": true, "EPS": true, "PE": true,
}

// RunSocial — ApeWisdom 전 종목 목록을 오늘(KST) 날짜로 적재한다.
func (s *ExternalSignalService) RunSocial(ctx context.Context) (map[string]any, string, error) {
	if s == nil || s.ape == nil {
		return nil, "", fmt.Errorf("social: not configured")
	}
	rows, err := s.ape.AllStocks(ctx, func(ctx context.Context) error { return s.pace(ctx, apibudget.ProviderApeWisdom) })
	if err != nil && len(rows) == 0 {
		return nil, "", fmt.Errorf("social: %w", err)
	}
	known, kerr := s.repo.USListedSymbols(ctx)
	if kerr != nil {
		return nil, "", kerr
	}
	var keep []repository.SocialRow
	skipped := 0
	seen := map[string]bool{}
	for _, r := range rows {
		if socialTickerDenylist[r.Ticker] || known[r.Ticker] == "" {
			skipped++
			continue
		}
		// 페이지를 넘기는 사이 순위가 바뀌면 같은 티커가 두 번 온다(2026-09-28 첫 실행: TBI) — 앞(높은 순위)을 쓴다.
		if seen[r.Ticker] {
			continue
		}
		seen[r.Ticker] = true
		keep = append(keep, repository.SocialRow{
			Symbol: r.Ticker, Mentions: r.Mentions, MentionsPrev: r.MentionsPrev,
			Upvotes: r.Upvotes, Rank: r.Rank, RankPrev: r.RankPrev,
		})
	}
	date := time.Now().In(model.KST).Format("2006-01-02")
	if werr := s.repo.ReplaceSocialDay(ctx, date, keep); werr != nil {
		return nil, "", werr
	}
	// 언급 상위 종목 페이지(감성·언급 사용자·키워드) — 점수 배치(00:25)가 읽으므로 서브레딧보다 먼저.
	insights := s.fetchInsights(ctx, date, keep)
	// 서브레딧별 — 실패해도 전체 집계는 이미 저장됐다(다음 날 다시 받는다).
	subStored := 0
	for _, sub := range marketdata.ApeWisdomSubreddits {
		if s.pace(ctx, apibudget.ProviderApeWisdom) != nil {
			break
		}
		subRows, serr := s.ape.Filter(ctx, sub, func(ctx context.Context) error { return s.pace(ctx, apibudget.ProviderApeWisdom) })
		if serr != nil && len(subRows) == 0 {
			continue
		}
		var list []repository.SocialRow
		dup := map[string]bool{}
		for _, r := range subRows {
			if socialTickerDenylist[r.Ticker] || known[r.Ticker] == "" || dup[r.Ticker] {
				continue
			}
			dup[r.Ticker] = true
			list = append(list, repository.SocialRow{Symbol: r.Ticker, Mentions: r.Mentions, MentionsPrev: r.MentionsPrev,
				Upvotes: r.Upvotes, Rank: r.Rank, RankPrev: r.RankPrev})
		}
		if werr := s.repo.ReplaceSocialDaySource(ctx, date, "r/"+sub, list); werr == nil {
			subStored += len(list)
		}
	}
	partial := ""
	if err != nil {
		partial = " (일부 페이지 실패: " + err.Error() + ")"
	}
	msg := fmt.Sprintf("social_mentions: %s — %d종목 저장 / 제외 %d(카탈로그 밖·낱말 티커) · 종목 요약 %d · 서브레딧 %d행%s",
		date, len(keep), skipped, insights, subStored, partial)
	return map[string]any{"date": date, "stored": len(keep), "skipped": skipped, "fetched": len(rows),
		"insights": insights, "subredditRows": subStored}, msg, nil
}

// 한 번 기다렸다 다시 걸 만한 실패 — 간격·분당 한도·429. 일 한도는 기다려도 소용없다.
func isBudgetOr429(err error) bool {
	if errors.Is(err, apibudget.ErrPacing) || strings.Contains(err.Error(), "429") {
		return true
	}
	return errors.Is(err, apibudget.ErrExhausted) && strings.Contains(err.Error(), "per-minute")
}
