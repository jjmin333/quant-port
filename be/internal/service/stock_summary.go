package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

const (
	stockSummaryCacheKeyPrefix = "stock:summary:v3:"
	stockSummaryStaleAfter     = 6 * time.Hour
	stockSummaryCacheTTL       = 24 * time.Hour
	stockSummaryNewsLimit      = 12

	// llmRequestLogKey/Max — capped, newest-first log of every
	// GenerateStockSummary call (input facts + success/failure), for the
	// LLM 모델 관리 admin page's "요청 INPUT 기록" section.
	llmRequestLogKey = "llm:requests:v1:log"
	llmRequestLogMax = 300
	llmRequestLogTTL = 7 * 24 * time.Hour
)

func stockSummaryCacheKey(symbol string) string {
	return stockSummaryCacheKeyPrefix + strings.ToUpper(strings.TrimSpace(symbol))
}

// StockSummaryService assembles the per-symbol AI 요약 shown atop the
// 재무제표/실적/전망/내부자거래/차트/뉴스 tabs. One LLM call produces all four
// text sections at once (see llm.Client.GenerateStockSummary) — this service
// never calls the LLM per tab. Results are cached per symbol (not per user)
// with a stale-while-revalidate cache.
//
// LLM spend has exactly two entry points: the scheduled per-market batch
// (stock_summary_batch.go) and an explicit admin refresh. Ordinary reads never
// call the LLM, however stale their cache is — that is what makes a day's cost
// equal to the number of symbols the batch planned, and therefore predictable
// against a free-tier quota. A read that finds stale data still rebuilds the
// numbers; it just carries the previous run's narrative forward instead of
// paying for a new one.
type StockSummaryService struct {
	fundamentals     *FundamentalsService
	financialHistory *FinancialHistoryService
	earnings         *EarningsTabService
	insider          *InsiderTabService
	news             *NewsStore
	candles          *repository.CandleBarRepo
	stocks           *repository.StockRepo
	llm              *llm.Pool
	redis            *redis.Client

	// rankings supplies 마켓 레이더's stored 1일 기준 top 100, which the batch
	// draws its targets from. Optional — without it the batch falls back to
	// the pinned symbol list only.
	rankings *repository.MarketRankingsRepo

	// settings holds the admin-editable batch knobs and always-include
	// symbol list (see /admin/llm — LLM 모델 관리). Re-read on every check,
	// so admin edits apply immediately.
	settings *LLMSettingsStore

	// dailyCloses persists each generated summary onto that trading day's
	// 일별 종가 row (score + AI 요약 text). Optional — nil simply skips it.
	dailyCloses *DailyCloseService

	/*
		narratives는 **마지막 LLM 글의 영구 보관소**다 (migrations/065).

		캐시(24시간)만으로 글을 이어 받던 동안에는, 하루 동안 아무도 열지 않은 종목의
		글이 조용히 사라졌다 — 다음 방문자는 규칙 문장을 봤다. 개요 탭이 「LLM 글이
		없으면 카드를 안 그린다」로 바뀐 뒤로는 그 손실이 카드가 사라지는 것으로 보인다.

		nil이면 예전처럼 캐시만 본다.
	*/
	narratives *repository.StockSummaryNarrativeRepo

	/*
		배치 대상 선정의 2·3순위 입력 (llm_batch_targets.go).

		scoreRankings는 AI 추천 랭킹의 사전 계산 표(00:25 배치), holdings/watchlist는
		사용자가 직접 담은 종목이다. 셋 다 선택 사항이다 — nil이면 그 층이 비고
		1순위(레이더)만으로 돈다.
	*/
	scoreRankings *repository.StockScoreRankingRepo
	holdings      *repository.UserHoldingsRepo
	watchlist     *repository.WatchlistRepo

	/*
		batchRuns — 예약 실행을 **배치 원장에 남기기 위한** 것뿐이다.

		이 배치는 전용 워커가 없어 API 프로세스 안에서 도는데, 그래서 오래도록
		`batch_job_runs`에 한 줄도 안 남겼다. 수동 실행만 큐 워커를 지나가므로
		관리자 「배치 관리」는 **마지막 실행을 08-20으로 알고 있었다** — 그 사이
		매일 돌고 있었는데도. 감시가 원장을 보는 이상, 원장에 없는 잡은 감시되지
		않는다.

		nil이면 예전처럼 Redis 기록만 남는다.
	*/
	batchRuns *repository.BatchJobRunRepo

	// scores supplies the server-computed 종합 점수 and its axis breakdown to
	// the prompt. Without it the prose and the number shown beside it would be
	// reasoning from different inputs.
	scores *ScoreService

	// mu guards bgCtx, manualJob and lastBatchDate — written by the admin
	// manual refresh path (stock_summary_manual.go) and the batch runner,
	// read by HTTP handlers.
	mu        sync.RWMutex
	bgCtx     context.Context
	manualJob *model.LLMManualRefreshJob
	// lastBatchDate is the Redis-less fallback for "has this market already
	// run today", keyed by market.
	lastBatchDate map[toss.MarketCountry]string
	// runningMarkets guards against the scheduled batch and a 배치 관리 manual
	// trigger (stock_summary_batch.go RunBatchNow) executing for the same
	// market at once — both would write the same Redis run/history keys and
	// check the same LLM budget independently, so overlap could double-spend
	// the daily quota rather than just race on the record.
	runningMarkets map[toss.MarketCountry]bool

	/*
		build — 심볼 하나의 요약 재구성을 **한 번으로 묶는** 조정자.

		없을 때 무슨 일이 났나(docs/PERFORMANCE_AUDIT.md P0-2):

		  · stale 경로는 읽을 때마다 `go s.refreshOne(symbol)`을 가드 없이 띄웠다.
		    동시 독자 N명이면 전체 빌드가 N개 뜬다.
		  · 캐시 미스 경로는 **동기** build였다. 요청 슬롯을 잡은 채 돈다.

		한 번의 빌드가 하는 일은 fundamentals·재무이력·실적·내부자·뉴스·캔들·점수를
		전부 모으는 것이고, 실측 콜드 159~406ms다(웜 2ms). 게다가
		`/stocks/{symbol}/summary`는 **비로그인 공개 경로**라 심볼 17,912개를
		순회하면 방어막이 IP 레이트리밋뿐이었다.

		시세 쪽에서 쓰던 조정자를 그대로 쓴다 — `runOnce`는 비동기로 중복을 접고,
		`doSync`는 소유자 하나만 만들고 나머지는 done 채널에서 기다린다. 대기는
		quoteRefreshWait(5초)에서 끊기므로 빌드가 길어져도 요청이 묶여 있지 않는다.
	*/
	buildOnce   *quoteRefreshCoordinator
	buildOnceMu sync.Mutex
}

// buildCoordinator lazily creates the per-symbol build coordinator.
//
// 생성자가 여럿이라(테스트·배선) 필드 초기화를 한곳으로 모은다.
func (s *StockSummaryService) buildCoordinator() *quoteRefreshCoordinator {
	s.buildOnceMu.Lock()
	defer s.buildOnceMu.Unlock()
	if s.buildOnce == nil {
		s.buildOnce = newQuoteRefreshCoordinator()
	}
	return s.buildOnce
}

// SetRankings wires 마켓 레이더's stored rankings so the scheduled batch can
// pick its targets from the 1일 기준 top 100.
func (s *StockSummaryService) SetRankings(repo *repository.MarketRankingsRepo) {
	s.rankings = repo
}

// SetDailyCloses wires the 일별 종가 store so generated summaries are recorded
// against the trading day they analysed.
func (s *StockSummaryService) SetDailyCloses(svc *DailyCloseService) {
	s.dailyCloses = svc
}

// SetScores wires the 종합 점수 service so the AI 요약 can explain the score
// rather than restate raw figures alongside it.
func (s *StockSummaryService) SetScores(svc *ScoreService) {
	s.scores = svc
}

// SetNarratives wires the durable store for LLM prose, so a symbol keeps the
// text it was written even after its Redis cache expires.
func (s *StockSummaryService) SetNarratives(repo *repository.StockSummaryNarrativeRepo) {
	s.narratives = repo
}

// SetBatchRuns wires the batch ledger so scheduled runs are recorded there
// alongside every worker job (see the field comment).
func (s *StockSummaryService) SetBatchRuns(repo *repository.BatchJobRunRepo) {
	s.batchRuns = repo
}

// SetScoreRankings wires AI 추천 랭킹's precomputed leaderboards so the batch's
// 2순위 layer can rotate through them.
func (s *StockSummaryService) SetScoreRankings(repo *repository.StockScoreRankingRepo) {
	s.scoreRankings = repo
}

// SetUserInterest wires 보유/관심 종목 so the batch's 3순위 layer can fill in
// the symbols users actually put on their own screens.
func (s *StockSummaryService) SetUserInterest(
	holdings *repository.UserHoldingsRepo,
	watchlist *repository.WatchlistRepo,
) {
	s.holdings = holdings
	s.watchlist = watchlist
}

func NewStockSummaryService(
	fundamentals *FundamentalsService,
	financialHistory *FinancialHistoryService,
	earnings *EarningsTabService,
	insider *InsiderTabService,
	news *NewsStore,
	candles *repository.CandleBarRepo,
	stocks *repository.StockRepo,
	llmClient *llm.Pool,
	redisClient *redis.Client,
	settings *LLMSettingsStore,
) *StockSummaryService {
	return &StockSummaryService{
		fundamentals:     fundamentals,
		financialHistory: financialHistory,
		earnings:         earnings,
		insider:          insider,
		news:             news,
		candles:          candles,
		stocks:           stocks,
		llm:              llmClient,
		redis:            redisClient,
		settings:         settings,
	}
}

// Provider reports the configured LLM backend ("openai"/"gemini"/"") and
// whether it's enabled, for admin monitoring.
func (s *StockSummaryService) Provider() (provider string, enabled bool) {
	if s.llm == nil {
		return "", false
	}
	return s.llm.Provider(), s.llm.Enabled()
}

// SplitKeys reports whether 국내 and 해외 charge distinct API keys.
func (s *StockSummaryService) SplitKeys() bool {
	return s.llm != nil && s.llm.Split()
}

func (s *StockSummaryService) llmForMarket(market toss.MarketCountry) *llm.Client {
	if s == nil || s.llm == nil {
		return nil
	}
	return s.llm.For(string(market))
}

func (s *StockSummaryService) llmForStock(stock *model.Stock) *llm.Client {
	if stock == nil {
		return s.llmForMarket(toss.MarketKR)
	}
	return s.llmForMarket(marketForStock(*stock))
}

// PilotSymbols returns the always-include symbols, in admin-defined order.
func (s *StockSummaryService) PilotSymbols(ctx context.Context) []string {
	if s.settings == nil {
		return nil
	}
	return s.settings.PilotSymbols(ctx)
}

// PilotStatus reads the cached summary for each pilot symbol without
// triggering a build — for admin monitoring only, never forces generation.
func (s *StockSummaryService) PilotStatus(ctx context.Context) []model.StockSummaryPilotStatus {
	symbols := s.PilotSymbols(ctx)
	out := make([]model.StockSummaryPilotStatus, 0, len(symbols))
	for _, symbol := range symbols {
		status := model.StockSummaryPilotStatus{Symbol: symbol}
		if s.redis != nil {
			if cached, err := s.redis.Get(ctx, stockSummaryCacheKey(symbol)).Bytes(); err == nil && len(cached) > 0 {
				var view model.StockSummaryView
				if json.Unmarshal(cached, &view) == nil {
					status.Cached = true
					status.Source = view.Source
					status.GeneratedAt = view.GeneratedAt
				}
			}
		}
		out = append(out, status)
	}
	return out
}

func (s *StockSummaryService) Get(ctx context.Context, symbol string) (*model.StockSummaryView, error) {
	cacheKey := stockSummaryCacheKey(symbol)
	readCache := func() (*model.StockSummaryView, bool) {
		if s.redis == nil {
			return nil, false
		}
		cached, err := s.redis.Get(ctx, cacheKey).Bytes()
		if err != nil || len(cached) == 0 {
			return nil, false
		}
		var out model.StockSummaryView
		if json.Unmarshal(cached, &out) != nil {
			return nil, false
		}
		/*
			개요 문단은 **캐시에서 읽어 온 것에도** 채운다.

			`Overview`는 §8에서 새로 생긴 필드다. 이미 저장돼 있는 요약(TTL 24시간)에는
			그 칸이 없고, 그 값들은 아직 신선하므로 재빌드가 돌지 않는다 — 그대로
			두면 개요 탭이 하루 동안 예전처럼 밸류에이션 한 갈래만 말한다.

			`composeOverview`는 네 섹션에서 엮을 뿐 새로 재지 않으므로 읽기 경로에서
			불러도 값싸다. 캐시 자체는 고치지 않는다 — 다음 재빌드가 자연스럽게
			덮어쓴다.
		*/
		if strings.TrimSpace(out.Overview) == "" {
			out.Overview = composeOverview(&out)
		}
		return &out, true
	}

	if out, ok := readCache(); ok {
		if s.isFresh(out) {
			return out, nil
		}
		// 낡았지만 값은 있다 — 그대로 주고 갱신은 뒤로 보낸다. runOnce가
		// 같은 심볼의 중복 갱신을 접는다(위 buildOnce 주석 참조).
		s.buildCoordinator().runOnce("summary:"+strings.ToUpper(strings.TrimSpace(symbol)),
			func(context.Context) { s.refreshOne(symbol) })
		return out, nil
	}

	/*
		캐시 미스. 여기서 그냥 build하면 같은 심볼에 대한 동시 요청 수만큼
		전체 빌드가 돈다 — 공개 경로라 그게 곧 부하 증폭 통로였다.

		소유자 하나만 만들고 나머지는 기다렸다가 캐시에서 읽는다. 대기가
		끊긴 경우(빌드가 5초를 넘김)에는 아래 폴백으로 직접 만든다 —
		요청을 실패시키지 않는다.
	*/
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	s.buildCoordinator().doSync(ctx, "summary:"+sym, func(buildCtx context.Context) {
		// 앞선 소유자가 이미 채웠으면 다시 만들지 않는다.
		if _, ok := readCache(); ok {
			return
		}
		built, _, err := s.build(buildCtx, symbol, false)
		if err != nil {
			return
		}
		s.store(buildCtx, symbol, built)
	})

	if out, ok := readCache(); ok {
		return out, nil
	}

	out, _, err := s.build(ctx, symbol, false)
	if err != nil {
		return nil, err
	}
	s.store(ctx, symbol, out)
	return out, nil
}

func (s *StockSummaryService) isFresh(out *model.StockSummaryView) bool {
	if out == nil || out.GeneratedAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, out.GeneratedAt)
	if err != nil {
		return false
	}
	return time.Since(t) < stockSummaryStaleAfter
}

// refreshOne rebuilds and re-caches one symbol with its own bounded context,
// without spending an LLM call — the async stale-cache path. Any narrative a
// previous run generated is carried forward by build().
func (s *StockSummaryService) refreshOne(symbol string) {
	if _, _, err := s.refreshSymbol(context.Background(), symbol, false); err != nil {
		log.Printf("stock summary refresh %s: %v", symbol, err)
	}
}

// refreshSymbol rebuilds one symbol under its own bounded context and returns
// the stored view plus what the build cost. useLLM is set only by the
// scheduled batch and by an explicit admin refresh.
func (s *StockSummaryService) refreshSymbol(parent context.Context, symbol string, useLLM bool) (*model.StockSummaryView, buildStats, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	out, stats, err := s.build(ctx, symbol, useLLM)
	if err != nil {
		return nil, stats, err
	}
	s.store(ctx, symbol, out)
	return out, stats, nil
}

func (s *StockSummaryService) store(ctx context.Context, symbol string, out *model.StockSummaryView) {
	if s.redis == nil || out == nil {
		return
	}
	if payload, err := json.Marshal(out); err == nil {
		_ = s.redis.Set(ctx, stockSummaryCacheKey(symbol), payload, stockSummaryCacheTTL).Err()
	}
}

// buildStats reports what one build spent, so the batch runner can track its
// token consumption and tell a failed LLM call apart from a build that never
// attempted one.
type buildStats struct {
	llmAttempted bool
	llmTokens    int64
	llmErr       error
}

func (s *StockSummaryService) build(ctx context.Context, symbol string, useLLM bool) (*model.StockSummaryView, buildStats, error) {
	var stats buildStats
	stock, err := s.stocks.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, stats, err
	}
	if stock == nil {
		// 오타·오래된 링크로 흔히 닿는 자리다. 「그런 종목이 없다」는 404이지
		// 서버 장애가 아니다 — 500으로 나가면 화면이 재시도할 문제로 오해하고
		// 로딩에 갇힌다(FE_FUNCTIONAL_QA BUG-05).
		return nil, stats, fmt.Errorf("%w: %s", ErrUnknownSymbol, symbol)
	}
	assetType := ResolveFundamentalsAssetType(stock)

	var fundamentals *model.FundamentalsView
	if s.fundamentals != nil {
		fundamentals, _ = s.fundamentals.GetWithValuation(ctx, stock.Symbol)
	}
	history := s.recentHistory(ctx, stock.Symbol)
	insiderTxns := s.recentInsider(ctx, stock.Symbol)
	technical := s.technicalIndicators(ctx, stock.Symbol)
	articles := s.recentArticles(ctx, stock, assetType)

	out := ruleBasedStockSummary(fundamentals, history, insiderTxns, technical, len(articles))
	out.Symbol = stock.Symbol
	out.AssetType = assetType
	out.Keywords = ruleBasedKeywords(stock, fundamentals, technical)

	// Fetched once and shared by the briefing and the LLM facts: both must
	// reason from the same score the UI displays.
	var scoreView *model.StockScoreView
	if s.scores != nil {
		scoreView, _ = s.scores.Get(ctx, stock.Symbol)
	}
	out.Briefing = buildMarketBriefing(stock, fundamentals, history, insiderTxns, technical, scoreView)

	client := s.llmForStock(stock)
	if useLLM && client.Enabled() {
		stats.llmAttempted = true
		facts := buildStockSummaryFacts(stock, fundamentals, history, insiderTxns, technical, articles, scoreView)
		factsJSON, err := json.Marshal(facts)
		if err != nil {
			stats.llmErr = err
		} else {
			llmOut, llmErr := client.GenerateStockSummary(ctx, string(factsJSON), describeScoreForPrompt(facts))
			entry := model.LLMRequestLogEntry{
				Symbol:    stock.Symbol,
				Name:      stock.Name,
				Timestamp: time.Now().Format(time.RFC3339),
				Success:   llmErr == nil,
				FactsJSON: string(factsJSON),
				Manual:    true,
			}
			if llmErr != nil {
				stats.llmErr = llmErr
				entry.Error = llmErr.Error()
				log.Printf("stock summary llm fallback %s: %v", stock.Symbol, llmErr)
			} else {
				stats.llmTokens = llmOut.TotalTokens
				applyStockSummaryLLMOutput(out, llmOut, articles)
				out.Source = "llm"
				// 방금 돈을 쓴 글이다 — 캐시가 만료돼도 잃지 않게 여기서 남긴다.
				s.storeNarrative(ctx, stock.Symbol, out)
			}
			s.logLLMRequest(ctx, entry)
		}
	}

	// A rebuild that did not produce fresh narrative keeps the last one the
	// batch paid for. Without this, every stale-cache read would silently
	// downgrade an AI 요약 to rule-based text a few hours after it was
	// generated — the numbers would stay current and the prose would vanish.
	if out.Source != "llm" {
		s.carryOverNarrative(ctx, stock.Symbol, out)
	}

	/*
		개요 문단은 **네 섹션이 다 정해진 뒤에** 채운다.

		LLM이 켜져 있으면 같은 호출이 써 준 것이 이미 들어와 있다. 비어 있는
		경우는 셋이다: LLM이 꺼져 있을 때, 호출이 실패했을 때, 그리고 이 필드가
		생기기 전에 만든 캐시를 이어 받았을 때. 그때는 서버가 네 섹션에서 엮는다 —
		개요가 밸류에이션 한 갈래만 말하는 상태로 돌아가지 않게(§8).
	*/
	if strings.TrimSpace(out.Overview) == "" {
		out.Overview = composeOverview(out)
	}

	out.GeneratedAt = time.Now().Format(time.RFC3339)
	s.recordDailyAnalysis(ctx, stock, assetType, out)
	/*
		심리·수급 축을 이 종목의 저장된 점수에 얹는다 (`stock_score_sentiment.go`).

		재료(뉴스 감성 · 내부자 거래)는 방금 이 빌드가 모은 것이고, 점수 배치는
		전 종목에 그것을 돌릴 수 없어 그 축을 비워 둔다. **LLM 호출은 늘지 않는다** —
		프롬프트에 넣던 값을 한 번 더 쓸 뿐이다. 그래서 LLM 없이 도는 재빌드에서도
		부른다(감성은 `carryOverNarrative`가 이어 받는다): 배치가 덮어쓴 축이 다음
		읽기에서 스스로 복구된다.

		실패는 로그만 남긴다 — 요약 자체는 이미 완성됐고, 점수 한 축 때문에 화면이
		비면 안 된다.
	*/
	if s.scores != nil {
		if err := s.scores.ApplySentiment(ctx, stock.Symbol, out.NewsSentiment, insiderTxns); err != nil {
			log.Printf("sentiment axis %s: %v", stock.Symbol, err)
		} else if updated, err := s.scores.Get(ctx, stock.Symbol); err == nil && updated != nil {
			/*
				시장 분석을 **바뀐 점수로 다시 쓴다.**

				브리핑은 위에서 이미 한 번 만들어졌는데, 그때의 `scoreView`는 심리
				축이 붙기 **전** 값이다. 그대로 두면 같은 화면에서 점수 카드는 66,
				바로 아래 「투자 포인트」 첫 줄은 69를 말한다 — §6에서 고친 것과
				똑같은 종류의 어긋남이 이 축을 더하면서 새로 생긴다(실측 000660).

				브리핑 조립은 순수 함수라 다시 부르는 비용이 조회 한 번뿐이다.
				LLM 프롬프트는 다시 만들지 않는다 — 이미 호출이 끝났고, 그 글은
				「어느 쪽 숫자로 썼는가」가 아니라 해석이라 한 축의 차이로 무효가
				되지 않는다.
			*/
			out.Briefing = buildMarketBriefing(stock, fundamentals, history, insiderTxns, technical, updated)
		}
	}
	return out, stats, nil
}

/*
carryOverNarrative — 새로 계산한 숫자 위에 **지난 LLM 글**을 얹는다.

찾는 순서가 둘이다:

	1. Redis 캐시 (24시간). 대개 여기서 끝난다.
	2. `stock_summary_narratives` 표 (영구).

2번이 필요한 이유: 하루 동안 아무도 열지 않은 종목은 캐시가 비고, 그러면 예전에는
돈 주고 만든 글이 **영영 사라졌다**. 개요 탭이 「LLM 글이 없으면 카드를 안 그린다」로
바뀐 뒤로는 그 손실이 카드가 사라지는 것으로 보인다.

숫자는 건드리지 않는다 — 글만 옮긴다.
*/
func (s *StockSummaryService) carryOverNarrative(ctx context.Context, symbol string, out *model.StockSummaryView) {
	if out == nil {
		return
	}
	if s.carryOverFromCache(ctx, symbol, out) {
		return
	}
	s.carryOverFromStore(ctx, symbol, out)
}

func (s *StockSummaryService) carryOverFromCache(ctx context.Context, symbol string, out *model.StockSummaryView) bool {
	if s.redis == nil {
		return false
	}
	raw, err := s.redis.Get(ctx, stockSummaryCacheKey(symbol)).Bytes()
	if err != nil || len(raw) == 0 {
		return false
	}
	var prev model.StockSummaryView
	if json.Unmarshal(raw, &prev) != nil || prev.Source != "llm" {
		return false
	}
	out.Overview = prev.Overview
	out.Valuation = prev.Valuation
	out.Technical = prev.Technical
	out.Insider = prev.Insider
	out.News = prev.News
	if len(prev.Keywords) > 0 {
		out.Keywords = prev.Keywords
	}
	// The sentiment tally is only ever produced by the LLM, from the same
	// article set the news sentence describes. Carrying the prose without it
	// would blank the sentiment badge and drop the news axis out of the
	// daily close score between two batches.
	if prev.NewsSentiment != nil {
		out.NewsSentiment = prev.NewsSentiment
	}
	out.Source = prev.Source
	// GeneratedAt is overwritten by the caller with "now", which would claim
	// the carried-over prose is fresh. AnalyzedAt keeps the honest timestamp
	// of the run that actually produced it.
	if prev.AnalyzedAt != "" {
		out.AnalyzedAt = prev.AnalyzedAt
	} else {
		out.AnalyzedAt = prev.GeneratedAt
	}
	/*
		캐시에서 되살린 글도 **표에 한 번 더 남긴다.**

		이 기능이 생기기 전에 만들어진 캐시에는 대응하는 표 행이 없다. 그대로 두면
		그 종목들은 캐시가 만료되는 순간 예전과 똑같이 글을 잃는다 — 읽는 김에
		채워 두면 하루 안에 자연히 이관된다. 쓰기는 이미 있는 행을 덮어쓸 뿐이다.
	*/
	s.storeNarrative(ctx, symbol, out)
	return true
}

func (s *StockSummaryService) carryOverFromStore(ctx context.Context, symbol string, out *model.StockSummaryView) {
	if s.narratives == nil {
		return
	}
	prev, err := s.narratives.Get(ctx, symbol)
	if err != nil {
		log.Printf("stock summary narrative %s: %v", symbol, err)
		return
	}
	// 한 번도 LLM이 쓴 적 없는 종목이다. 규칙 글이 그대로 남고, 화면은 그때
	// 「AI 핵심 요약」 카드를 그리지 않는다.
	if prev == nil || strings.TrimSpace(prev.Overview+prev.Valuation+prev.Technical) == "" {
		return
	}
	out.Overview = prev.Overview
	out.Valuation = prev.Valuation
	out.Technical = prev.Technical
	out.Insider = prev.Insider
	out.News = prev.News
	if len(prev.Keywords) > 0 {
		out.Keywords = prev.Keywords
	}
	if prev.NewsSentiment != nil {
		out.NewsSentiment = prev.NewsSentiment
	}
	out.Source = "llm"
	out.AnalyzedAt = prev.AnalyzedAt
}

/*
storeNarrative — 글만 표에 남긴다. 실패는 로그로 끝낸다.

숫자를 함께 넣지 않는 이유는 저장소 주석에 있다: 다음에 되살릴 때 오래된 숫자가
새 글 옆에 함께 살아나면 안 된다.
*/
func (s *StockSummaryService) storeNarrative(ctx context.Context, symbol string, out *model.StockSummaryView) {
	if s.narratives == nil || out == nil || out.Source != "llm" {
		return
	}
	analyzedAt := out.AnalyzedAt
	if analyzedAt == "" {
		analyzedAt = out.GeneratedAt
	}
	at, err := time.Parse(time.RFC3339, analyzedAt)
	if err != nil {
		at = time.Now()
		analyzedAt = at.Format(time.RFC3339)
	}
	err = s.narratives.Save(ctx, symbol, repository.StockSummaryNarrative{
		Overview:      out.Overview,
		Valuation:     out.Valuation,
		Technical:     out.Technical,
		Insider:       out.Insider,
		News:          out.News,
		Keywords:      out.Keywords,
		NewsSentiment: out.NewsSentiment,
		AnalyzedAt:    analyzedAt,
	}, at)
	if err != nil {
		log.Printf("stock summary narrative save %s: %v", symbol, err)
	}
}

// recordDailyAnalysis attaches this summary to the trading day it actually
// analysed. The trade date is the market's last *confirmed* close, computed per
// market: at 00:00 KST the domestic session closed yesterday, while the US
// session is still mid-day, so a naive "today − 1" would file US rows against a
// date whose close does not exist yet.
func (s *StockSummaryService) recordDailyAnalysis(
	ctx context.Context,
	stock *model.Stock,
	assetType model.QuoteAssetType,
	view *model.StockSummaryView,
) {
	if s.dailyCloses == nil || view == nil {
		return
	}

	schedule := calendar.ScheduleKRX
	if assetType == model.AssetUS {
		schedule = calendar.ScheduleUS
	}
	lastClose := calendar.LastSessionCloseAt(schedule, time.Now())
	tradeDate := lastClose.In(calendar.LocationFor(schedule)).Format("2006-01-02")

	/*
		**점수는 쓰지 않는다** (2026-09-03 §6 점수 통일).

		예전에는 여기서 `stockSummaryScore(view, technical)`로 이 파이프라인만의
		점수를 만들어 같은 컬럼에 써 넣었고, 점수 배치가 쓰는 값과 번갈아 덮어쓰며
		한 종목·한 날짜에 두 개의 「종합 점수」를 만들어 냈다. 이제 이 함수는 그날의
		**글만** 남긴다 — 점수는 `stock_scores`에서 조인해 온다.
	*/
	analysis := model.DailyCloseAnalysis{
		Symbol:    stock.Symbol,
		TradeDate: tradeDate,
		AISummary: dailyCloseSummaryText(view),
		Source:    view.Source,
	}
	if ok, err := s.dailyCloses.AttachAnalysis(ctx, analysis); err != nil {
		log.Printf("daily close analysis %s %s: %v", stock.Symbol, tradeDate, err)
	} else if !ok {
		// No close row yet for that date — the price ingestion has not run.
		// Not an error: the next ingestion plus the next summary will fill it.
		log.Printf("daily close analysis %s: no close row for %s yet", stock.Symbol, tradeDate)
	}
}

/*
dailyCloseSummaryText — 날짜별 한 줄 아래에 붙는 요약 **한 문단**.

# 네 갈래를 다 담는다 (2026-09-11 사용자 요청 2)

예전에는 `Valuation`과 `Technical` 둘만 이었다. 그런데 이 글이 쌓이는 자리는
`stock_daily_closes.ai_summary`, 즉 **되돌릴 수 없는 이력**이다 — 그날의 LLM 응답은
캐시가 지나면 사라지고 남는 것은 이 컬럼이다. 그 이력이 네 갈래 중 둘만 담고 있으면
「그날 내부자가 팔았다」·「그날 뉴스가 어땠다」는 나중에 어디서도 복구할 수 없다.

# 없는 갈래는 **빠진다**

종목마다 있는 재료가 다르다(내부자 거래가 아예 없는 종목이 흔하다). 그래서 빈
문자열뿐 아니라 **「최근 내부자 거래 내역이 없습니다」류의 규칙 기반 공백 문장까지**
걸러 낸다(`isEmptySectionText`) — 그 문장을 이력에 쌓으면 한 문단의 절반이 「없다」로
채워진다. 개요 한 문단을 엮는 `composeOverview`가 같은 이유로 같은 필터를 쓴다.

# `composeOverview`와 다른 점

이쪽은 각 섹션을 **통째로** 잇는다. 개요는 카드 네 줄에 잘리므로 첫 문장만 모으지만,
이 글은 그런 상한이 없고 지나간 날의 근거가 자세할수록 값이 크다.

과거 행은 다시 쓰지 않는다(2026-09-11 사용자 결정) — 그날의 원문이 없으므로 이 함수가
소급 적용될 자리가 없다. 오늘부터 쌓이는 행이 네 갈래를 담는다.
*/
func dailyCloseSummaryText(view *model.StockSummaryView) string {
	if view == nil {
		return ""
	}
	sections := []string{view.Valuation, view.Technical, view.Insider, view.News}
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		trimmed := strings.TrimSpace(section)
		if trimmed == "" || isEmptySectionText(trimmed) {
			continue
		}
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, " ")
}

func (s *StockSummaryService) logLLMRequest(ctx context.Context, entry model.LLMRequestLogEntry) {
	if s.redis == nil {
		return
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return
	}
	pipe := s.redis.Pipeline()
	pipe.LPush(ctx, llmRequestLogKey, payload)
	pipe.LTrim(ctx, llmRequestLogKey, 0, llmRequestLogMax-1)
	pipe.Expire(ctx, llmRequestLogKey, llmRequestLogTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("stock summary: log llm request: %v", err)
	}
}

// RecentRequests returns the most recent GenerateStockSummary calls
// (newest-first), for admin auditing of what input was actually sent.
func (s *StockSummaryService) RecentRequests(ctx context.Context) []model.LLMRequestLogEntry {
	if s.redis == nil {
		return nil
	}
	raw, err := s.redis.LRange(ctx, llmRequestLogKey, 0, -1).Result()
	if err != nil {
		return nil
	}
	out := make([]model.LLMRequestLogEntry, 0, len(raw))
	for _, item := range raw {
		var entry model.LLMRequestLogEntry
		if json.Unmarshal([]byte(item), &entry) == nil {
			out = append(out, entry)
		}
	}
	return out
}

// RequestLogPage filters the full request log (onlyFailed narrows to
// success == false) and slices out one page (1-indexed), newest-first. total
// is the count after filtering, before pagination, so callers can render a
// page count.
func (s *StockSummaryService) RequestLogPage(ctx context.Context, onlyFailed bool, page, pageSize int) (entries []model.LLMRequestLogEntry, total int) {
	all := s.RecentRequests(ctx)
	if onlyFailed {
		filtered := make([]model.LLMRequestLogEntry, 0, len(all))
		for _, entry := range all {
			if !entry.Success {
				filtered = append(filtered, entry)
			}
		}
		all = filtered
	}
	total = len(all)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 30
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []model.LLMRequestLogEntry{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total
}

func (s *StockSummaryService) recentHistory(ctx context.Context, symbol string) []model.FinancialHistoryPeriod {
	if s.financialHistory == nil {
		return nil
	}
	view, err := s.financialHistory.Get(ctx, symbol, 1)
	if err != nil || view == nil {
		return nil
	}
	return view.Periods
}

func (s *StockSummaryService) recentInsider(ctx context.Context, symbol string) []model.InsiderTransaction {
	if s.insider == nil {
		return nil
	}
	view, err := s.insider.Get(ctx, symbol)
	if err != nil || view == nil {
		return nil
	}
	return view.Transactions
}

func (s *StockSummaryService) technicalIndicators(ctx context.Context, symbol string) *TechnicalIndicators {
	if s.candles == nil {
		return nil
	}
	candles, _, err := s.candles.ListBefore(ctx, symbol, model.Interval1d, "", 200)
	if err != nil || len(candles) == 0 {
		return nil
	}
	return ComputeTechnicalIndicators(candles)
}

func (s *StockSummaryService) recentArticles(ctx context.Context, stock *model.Stock, assetType model.QuoteAssetType) []model.NewsArticle {
	if s.news == nil {
		return nil
	}
	market := "domestic"
	if assetType == model.AssetUS {
		market = "us"
	}
	articles, _ := s.news.List(ctx, model.NewsListFilter{
		Market: market,
		Sort:   "latest",
		Symbol: stock.Symbol,
		Limit:  stockSummaryNewsLimit,
	})
	return articles
}

// Start records the process-wide background context and launches the
// scheduler. Manual admin refreshes (see stock_summary_manual.go) derive
// their context from the same root, so a shutdown cancels them too.
func (s *StockSummaryService) Start(ctx context.Context) {
	s.mu.Lock()
	s.bgCtx = ctx
	s.mu.Unlock()
	go s.RunScheduler(ctx)
}

func (s *StockSummaryService) backgroundContext() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.bgCtx == nil {
		return context.Background()
	}
	return s.bgCtx
}

// kstZone matches the calendar package's fallback handling — the container
// image may not ship tzdata, in which case KST is a fixed +09:00 offset
// anyway (Korea observes no DST).
var kstZone = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}()

// llmRefreshSpacing is the floor on the gap between LLM calls in a sequential
// run. It must stay above apibudget's ProviderOpenAI MinInterval (4s) —
// calling back-to-back would otherwise trip our own rate limit and silently
// fall back to the rule-based summary for every symbol after the first. The
// batch runner stretches it further when token pacing demands it (see
// batchSpacing).
const llmRefreshSpacing = 6 * time.Second
