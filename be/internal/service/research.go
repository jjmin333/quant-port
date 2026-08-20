package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// Research calls — publish, freeze, judge (docs/RESEARCH_CALL_DESIGN.md).
//
// The service's job is to make sure the author never supplies a fact. Prices,
// scores, valuation, news and the rule-based briefing are all collected here,
// server-side, at publish time. What the author supplies is the claim: a
// direction, a target, a horizon, a conviction and prose.

const (
	researchChartBars   = 200
	researchNewsMax     = 5
	researchDiscloseMax = 5
	researchTitleMax    = 200
	researchBodyMax     = 50000
	researchTagsMax     = 5
	researchTagMaxLen   = 20

	// researchDailyCap bounds how many calls one author can publish per day.
	// Volume is the cheapest way to fake a track record.
	researchDailyCap = 5
	// researchSymbolCooldownDays stops the same symbol being re-called until
	// the previous call has had time to be wrong.
	researchSymbolCooldownDays = 30

	// Public-visibility floors (§14 종목 게이트). A call on a symbol too small
	// to absorb attention is a pump vector, not research.
	researchMinMarketCapKRW = 30_000_000_000 // 300억
	researchMinMarketCapUSD = 30_000_000     // $30M

	defaultResearchHorizonDays = 90
	defaultResearchConviction  = 50

	// researchOddsHistoryBars is how much daily history the target-odds
	// estimate reads. ~8 years is more than any accepted horizon needs.
	researchOddsHistoryBars = 2000
)

var (
	ErrResearchNoPrice     = errors.New("no price available for symbol")
	ErrResearchDailyCap    = errors.New("daily publish cap reached")
	ErrResearchNotEligible = errors.New("symbol not eligible for public publishing")
)

// ResearchInconsistentError explains a target/stop pair that contradicts the
// direction. Returning a specific message matters: the compose screen shows it
// inline next to the field the author has to fix.
type ResearchInconsistentError struct{ Reason string }

func (e *ResearchInconsistentError) Error() string { return e.Reason }

// ResearchSymbolCooldownError carries the wait so the UI can say when.
type ResearchSymbolCooldownError struct {
	Symbol       string
	DaysRemaining int
}

func (e *ResearchSymbolCooldownError) Error() string {
	return fmt.Sprintf("%s was called within %d days", e.Symbol, researchSymbolCooldownDays)
}

// ResearchPublishRequest is the client payload. Nothing factual is in here.
type ResearchPublishRequest struct {
	Symbol      string
	Title       string
	Direction   string
	TargetPrice *float64
	StopPrice   *float64
	HorizonDays int
	Conviction  int
	BodyMD       string
	ThumbnailURL string
	Tags         []string
	Visibility   string
	Interval     string
}

// ResearchUpdateRequest is the editable half — prose, cover and exposure only.
type ResearchUpdateRequest struct {
	Title        string
	BodyMD       string
	ThumbnailURL string
	Tags         []string
	Visibility   string
}

// ResearchPreviewRequest drives the compose screen's live panel.
type ResearchPreviewRequest struct {
	Symbol      string
	Direction   string
	TargetPrice *float64
	StopPrice   *float64
	HorizonDays int
	Conviction  int
	Interval    string
}

// ResearchService publishes and scores calls. Every collaborator is optional:
// a missing one degrades that part of the evidence into a dataGap rather than
// failing the publish.
type ResearchService struct {
	repo        *repository.ResearchRepo
	closes      *repository.StockDailyCloseRepo
	indexCloses *repository.IndexDailyCloseRepo
	stocks      *repository.StockRepo
	candles     *repository.CandleBarRepo

	quotes       *QuotesService
	scores       *ScoreService
	news         *NewsStore
	indexStore   *MarketIndexStore
	marketStore  *MarketDataStore
	stockSummary *StockSummaryService
}

func NewResearchService(
	repo *repository.ResearchRepo,
	closes *repository.StockDailyCloseRepo,
	indexCloses *repository.IndexDailyCloseRepo,
	stocks *repository.StockRepo,
	candles *repository.CandleBarRepo,
) *ResearchService {
	return &ResearchService{repo: repo, closes: closes, indexCloses: indexCloses, stocks: stocks, candles: candles}
}

func (s *ResearchService) SetQuotes(q *QuotesService)               { s.quotes = q }
func (s *ResearchService) SetScores(sc *ScoreService)               { s.scores = sc }
func (s *ResearchService) SetNews(n *NewsStore)                     { s.news = n }
func (s *ResearchService) SetIndexStore(i *MarketIndexStore)        { s.indexStore = i }
func (s *ResearchService) SetMarketStore(m *MarketDataStore)        { s.marketStore = m }
func (s *ResearchService) SetStockSummary(v *StockSummaryService)   { s.stockSummary = v }

// ---------------------------------------------------------------- publish

func (s *ResearchService) Publish(ctx context.Context, req ResearchPublishRequest) (model.ResearchView, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.ResearchView{}, err
	}
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		return model.ResearchView{}, errors.New("symbol required")
	}

	now := time.Now().UTC()
	if n, err := s.repo.CountSince(ctx, userID, now.Add(-24*time.Hour)); err != nil {
		return model.ResearchView{}, err
	} else if n >= researchDailyCap {
		return model.ResearchView{}, ErrResearchDailyCap
	}
	if last, ok, err := s.repo.LastForSymbol(ctx, userID, symbol); err != nil {
		return model.ResearchView{}, err
	} else if ok {
		cooldown := time.Duration(researchSymbolCooldownDays) * 24 * time.Hour
		if elapsed := now.Sub(last); elapsed < cooldown {
			days := int((cooldown-elapsed).Hours()/24) + 1
			return model.ResearchView{}, &ResearchSymbolCooldownError{Symbol: symbol, DaysRemaining: days}
		}
	}

	call, err := s.capture(ctx, symbol, now, req.Interval)
	if err != nil {
		return model.ResearchView{}, err
	}
	call.UserID = userID

	direction := normalizeDirection(req.Direction)
	target, stop, err := normalizeBounds(direction, call.BasePrice, req.TargetPrice, req.StopPrice)
	if err != nil {
		return model.ResearchView{}, err
	}
	call.Direction = direction
	call.TargetPrice = target
	call.StopPrice = stop
	call.HorizonDays = normalizeResearchHorizon(req.HorizonDays)
	call.Conviction = normalizeConviction(req.Conviction)
	call.Title = normalizeResearchTitle(req.Title, call.SymbolName, symbol, direction)
	call.BodyMD = truncateRunes(strings.TrimSpace(req.BodyMD), researchBodyMax)
	call.ThumbnailURL = normalizeThumbnailURL(req.ThumbnailURL)
	call.Tags = normalizeResearchTags(req.Tags)
	call.Visibility = normalizeVisibility(req.Visibility)
	call.Contrarian = isContrarian(direction, call.ScoreAtPub)

	if call.Visibility == model.VisibilityPublic {
		if eligible, reason := researchEligible(call.Currency, call.Evidence.MarketCap); !eligible {
			return model.ResearchView{}, fmt.Errorf("%w: %s", ErrResearchNotEligible, reason)
		}
	}

	created, err := s.repo.Create(ctx, call)
	if err != nil {
		return model.ResearchView{}, err
	}
	views, err := s.decorate(ctx, userID, []model.ResearchCall{created})
	if err != nil {
		return model.ResearchView{}, err
	}
	return views[0], nil
}

// capture collects the immutable evidence. It reads only paths the stock page
// itself already uses, so publishing costs no extra provider budget beyond the
// standard cold-miss backfill.
func (s *ResearchService) capture(
	ctx context.Context, symbol string, now time.Time, interval string,
) (model.ResearchCall, error) {
	call := model.ResearchCall{
		Symbol:      symbol,
		PublishedAt: now,
		Currency:    model.CurrencyKRW,
	}
	var gaps []string

	if s.stocks != nil {
		if st, err := s.stocks.GetBySymbol(ctx, symbol); err == nil && st != nil {
			call.SymbolName = st.Name
			call.AssetType = st.AssetType
			if st.Currency != "" {
				call.Currency = st.Currency
			}
			call.BenchmarkID = benchmarkForStock(st.AssetType, st.Market)
		}
	}

	var quote model.QuoteDetail
	haveQuote := false
	if s.quotes != nil {
		quote, haveQuote = s.quotes.GetQuote(ctx, symbol)
	}
	if haveQuote {
		call.BasePrice = quote.CurrentPrice
		if quote.Name != "" {
			call.SymbolName = quote.Name
		}
		if quote.Currency != "" {
			call.Currency = quote.Currency
		}
		if quote.AssetType != "" {
			call.AssetType = quote.AssetType
			if call.BenchmarkID == "" {
				call.BenchmarkID = benchmarkForStock(quote.AssetType, quote.Market)
			}
		}
		if quote.PrevClose > 0 {
			prev := quote.PrevClose
			call.Evidence.PrevClose = &prev
		}
		rate := quote.ChangeRate
		call.Evidence.ChangeRate = &rate
		call.Evidence.Volume = quote.Volume
		call.Evidence.MarketSession = quote.MarketSession
		call.Evidence.Open = quote.Open
		call.Evidence.High = quote.High
		call.Evidence.Low = quote.Low
		call.Evidence.Week52High = quote.Week52High
		call.Evidence.Week52Low = quote.Week52Low
		call.Evidence.MarketCap = quote.MarketCap
		call.Evidence.TradingValue = quote.TradingValue
		call.Evidence.PER = quote.PER
		call.Evidence.PBR = quote.PBR
	} else {
		gaps = append(gaps, "quote")
	}

	var closeRow *repository.StockDailyCloseRow
	if s.closes != nil {
		if rows, err := s.closes.List(ctx, symbol, 1); err == nil && len(rows) > 0 {
			closeRow = &rows[0]
		}
	}
	if call.BasePrice <= 0 && closeRow != nil {
		call.BasePrice = closeRow.Close
		if closeRow.PrevClose.Valid {
			prev := closeRow.PrevClose.Float64
			call.Evidence.PrevClose = &prev
		}
		if closeRow.ChangeRate.Valid {
			rate := closeRow.ChangeRate.Float64
			call.Evidence.ChangeRate = &rate
		}
		call.Evidence.Volume = closeRow.Volume
	}
	// After the close the live quote reports zero volume, which would freeze
	// "거래량 —" onto a session that actually traded.
	if call.Evidence.Volume == 0 && closeRow != nil {
		call.Evidence.Volume = closeRow.Volume
	}
	if call.BasePrice <= 0 {
		return model.ResearchCall{}, ErrResearchNoPrice
	}

	call.TradeDate = marketTradeDate(now, call.AssetType)
	if call.BenchmarkID == "" {
		call.BenchmarkID = benchmarkForStock(call.AssetType, "")
	}

	if s.scores != nil {
		if view, err := s.scores.Get(ctx, symbol); err == nil && view != nil {
			call.ScoreAtPub = view.Score
			call.GradeAtPub = view.Grade
			call.Evidence.Breakdown = view.Breakdown
			call.Evidence.Coverage = view.Coverage
			call.Evidence.PeerGroup = view.PeerGroup
			call.Evidence.PeerPct = view.PeerPct
			call.Evidence.ScoreKind = view.ScoreKind
		}
	}
	if call.ScoreAtPub == nil {
		gaps = append(gaps, "score")
	}

	// The rule-based briefing is what the draft is built from. It costs no LLM
	// call — see stock_briefing.go.
	if s.stockSummary != nil {
		if view, err := s.stockSummary.Get(ctx, symbol); err == nil && view != nil {
			call.Evidence.Briefing = view.Briefing
			call.Context.AIKeywords = view.Keywords
			if view.Valuation != "" || view.Technical != "" {
				call.Context.AISummary = strings.TrimSpace(view.Technical)
			}
		}
	}
	if call.Evidence.Briefing == nil {
		gaps = append(gaps, "briefing")
	}

	call.Context.News = s.captureNews(ctx, symbol, call.AssetType)
	call.Context.Disclosures = s.captureDisclosures(ctx, symbol)
	call.Context.Indices = s.captureIndices(ctx)
	if closeRow != nil && closeRow.AISummary != "" {
		call.Context.AISummary = closeRow.AISummary
	}
	if call.Context.AISummary == "" {
		gaps = append(gaps, "aiSummary")
	}
	if len(call.Context.News) == 0 {
		gaps = append(gaps, "news")
	}
	// Disclosures have no historical table — a cold cache means there is
	// nothing to copy and nothing to recover later (STAMP_DESIGN §5.2).
	if len(call.Context.Disclosures) == 0 {
		gaps = append(gaps, "disclosures")
	}

	call.Chart = s.captureChart(ctx, symbol, interval)
	if len(call.Chart.Bars) == 0 {
		gaps = append(gaps, "chart")
	}

	call.Context.DataGaps = gaps
	return call, nil
}

func (s *ResearchService) captureNews(ctx context.Context, symbol string, assetType model.QuoteAssetType) []model.ResearchNews {
	out := make([]model.ResearchNews, 0, researchNewsMax)
	if s.news == nil {
		return out
	}
	market := "domestic"
	if assetType == model.AssetUS || assetType == model.AssetETF {
		market = "us"
	}
	articles, _ := s.news.List(ctx, model.NewsListFilter{Market: market, Symbol: symbol, Limit: researchNewsMax})
	for _, a := range articles {
		headline := a.HeadlineKo
		if headline == "" {
			headline = a.Headline
		}
		out = append(out, model.ResearchNews{
			ID: a.ID, Headline: headline, URL: a.URL,
			PublisherName: a.PublisherName, PublishedAt: a.PublishedAt,
		})
		if len(out) >= researchNewsMax {
			break
		}
	}
	return out
}

func (s *ResearchService) captureDisclosures(ctx context.Context, symbol string) []model.BriefingEvent {
	out := make([]model.BriefingEvent, 0, researchDiscloseMax)
	if s.marketStore == nil {
		return out
	}
	// Cache-only read on purpose — EnsureTodayDisclosures would call DART.
	for _, ev := range s.marketStore.GetTodayDisclosures(ctx, []string{symbol}) {
		out = append(out, ev)
		if len(out) >= researchDiscloseMax {
			break
		}
	}
	return out
}

// researchIndexIDs is the market backdrop frozen with every call — two
// domestic, two US, plus FX.
var researchIndexIDs = []string{"kospi", "kosdaq", "sp500", "nasdaq", "usd-krw"}

func (s *ResearchService) captureIndices(ctx context.Context) []model.ResearchIndex {
	out := make([]model.ResearchIndex, 0, len(researchIndexIDs))
	if s.indexStore == nil {
		return out
	}
	wanted := map[string]struct{}{}
	for _, id := range researchIndexIDs {
		wanted[id] = struct{}{}
	}
	indices, _ := s.indexStore.List(ctx)
	for _, idx := range indices {
		if _, ok := wanted[idx.ID]; !ok {
			continue
		}
		out = append(out, model.ResearchIndex{
			ID: idx.ID, Name: idx.Name, Value: idx.Value, ChangeRate: idx.ChangeRate,
		})
	}
	return out
}

func (s *ResearchService) captureChart(ctx context.Context, symbol, interval string) model.ResearchChart {
	iv := model.CandleInterval(strings.TrimSpace(interval))
	switch iv {
	case model.Interval1m, model.Interval1d:
	default:
		iv = model.Interval1d
	}
	chart := model.ResearchChart{Interval: string(iv), Bars: []model.Candle{}}
	if s.candles == nil {
		return chart
	}
	bars, _, err := s.candles.ListBefore(ctx, symbol, iv, "", researchChartBars)
	if err != nil || len(bars) == 0 {
		return chart
	}
	chart.Bars = bars
	return chart
}

// ---------------------------------------------------------------- preview

// Preview assembles everything the compose screen shows without storing
// anything: the evidence card, the auto-generated draft, the snappable price
// levels, and — when a target is supplied — how often this symbol has actually
// moved that far in that time.
func (s *ResearchService) Preview(ctx context.Context, req ResearchPreviewRequest) (model.ResearchPreview, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.ResearchPreview{}, err
	}
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		return model.ResearchPreview{}, errors.New("symbol required")
	}

	call, err := s.capture(ctx, symbol, time.Now().UTC(), req.Interval)
	if err != nil {
		return model.ResearchPreview{}, err
	}
	direction := normalizeDirection(req.Direction)
	horizon := normalizeResearchHorizon(req.HorizonDays)

	out := model.ResearchPreview{
		Symbol:      call.Symbol,
		SymbolName:  call.SymbolName,
		AssetType:   call.AssetType,
		Currency:    call.Currency,
		BasePrice:   call.BasePrice,
		BenchmarkID: call.BenchmarkID,
		Score:       call.ScoreAtPub,
		Grade:       call.GradeAtPub,
		Contrarian:  isContrarian(direction, call.ScoreAtPub),
		Evidence:    call.Evidence,
		Context:     call.Context,
		DraftMD:     buildResearchDraft(call, direction),
		DataMD:      buildResearchDataBlock(call),
		Levels:      briefingLevels(call.Evidence.Briefing),
	}
	out.Eligible, out.IneligibleReason = func() (bool, string) {
		ok, reason := researchEligible(call.Currency, call.Evidence.MarketCap)
		return ok, reason
	}()

	if req.TargetPrice != nil && *req.TargetPrice > 0 && s.candles != nil {
		points, err := s.candles.DailyCloseSeries(ctx, symbol, researchOddsHistoryBars)
		if err == nil {
			out.Odds = researchTargetOdds(points, call.BasePrice, *req.TargetPrice, horizon)
		}
	}

	// Expected value uses the same rules the published call will, so the number
	// the author sees before publishing is the number they get after.
	probe := call
	probe.Direction = direction
	probe.HorizonDays = horizon
	probe.Conviction = normalizeConviction(req.Conviction)
	if target, stop, err := normalizeBounds(direction, call.BasePrice, req.TargetPrice, req.StopPrice); err == nil {
		probe.TargetPrice = target
		probe.StopPrice = stop
	}
	out.Expected = researchExpectedValue(probe, s.authorHitRate(ctx, userID))
	return out, nil
}

// briefingLevels flattens both scenarios' levels into one snap list. Every one
// of them is a real derived price (moving average, swing extreme, 52-week
// bound) — the compose screen must never offer an invented round number.
func briefingLevels(b *model.MarketBriefing) []model.PriceLevel {
	out := make([]model.PriceLevel, 0, 6)
	if b == nil {
		return out
	}
	if b.Upside != nil {
		out = append(out, b.Upside.Levels...)
	}
	if b.Downside != nil {
		out = append(out, b.Downside.Levels...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Price > out[j].Price })
	return out
}

// ---------------------------------------------------------------- read

func (s *ResearchService) List(ctx context.Context, f repository.ResearchListFilter) ([]model.ResearchView, error) {
	viewerID, _ := authctx.UserID(ctx)
	calls, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, err
	}
	views, err := s.decorate(ctx, viewerID, calls)
	if err != nil {
		return nil, err
	}
	return trimChartsForList(views), nil
}

// Mine lists the caller's own calls, including retracted and hidden ones.
func (s *ResearchService) Mine(ctx context.Context, f repository.ResearchListFilter) ([]model.ResearchView, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	f.UserID = userID
	f.IncludeHidden = true
	f.PublicOnly = false
	calls, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, err
	}
	views, err := s.decorate(ctx, userID, calls)
	if err != nil {
		return nil, err
	}
	return trimChartsForList(views), nil
}

/*
목록 응답의 캔들을 스파크라인이 실제로 쓰는 만큼만 남긴다.

발행 시점 차트는 200봉짜리 OHLCV라 **리포트 한 건에 19KB**다. 세 건짜리 목록
응답 72KB 중 57KB가 이것이었다. 그런데 목록·퀵룩 카드가 이 배열에서 읽는 값은
`time`과 `close` 둘뿐이고(`ResearchSparkline`), 폭 320px짜리 선에 200점을 찍으면
한 점이 1.6px를 차지한다 — 눈에 보이는 그림은 60점과 같다.

시가·고가·저가·거래량은 **상세 응답에 그대로 남는다**. 캔들스틱을 그리는 화면은
거기에 있고, 이 함수는 목록 경로에서만 불린다.

발행 지점 표시가 어긋나지 않도록 **처음과 마지막 봉은 반드시 포함**한다. 마지막
봉을 버리면 「발행 후」 선이 실제 마지막 종가보다 앞에서 끝난다.
*/
const researchListChartPoints = 60

func trimChartsForList(views []model.ResearchView) []model.ResearchView {
	for i := range views {
		views[i].Chart.Closes = downsampleCloses(views[i].Chart.Bars, researchListChartPoints)
		views[i].Chart.Bars = nil
	}
	return views
}

func downsampleCloses(bars []model.Candle, want int) []model.ResearchClosePoint {
	if len(bars) == 0 || want < 2 {
		return nil
	}
	point := func(b model.Candle) model.ResearchClosePoint {
		return model.ResearchClosePoint{Time: b.Time, Close: b.Close}
	}
	if len(bars) <= want {
		out := make([]model.ResearchClosePoint, len(bars))
		for i, b := range bars {
			out[i] = point(b)
		}
		return out
	}
	out := make([]model.ResearchClosePoint, 0, want)
	// 마지막 봉은 아래에서 따로 붙이므로 여기서는 want-1개를 고르게 흩는다.
	step := float64(len(bars)-1) / float64(want-1)
	for i := 0; i < want-1; i++ {
		out = append(out, point(bars[int(float64(i)*step)]))
	}
	return append(out, point(bars[len(bars)-1]))
}

func (s *ResearchService) Get(ctx context.Context, id int64) (model.ResearchView, error) {
	viewerID, _ := authctx.UserID(ctx)
	call, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.ResearchView{}, err
	}
	if !researchReadable(call, viewerID) {
		// Same answer as a missing row — a private call must not be
		// distinguishable from one that never existed.
		return model.ResearchView{}, repository.ErrResearchNotFound
	}
	views, err := s.decorate(ctx, viewerID, []model.ResearchCall{call})
	if err != nil {
		return model.ResearchView{}, err
	}
	if n, err := s.repo.RevisionCount(ctx, id); err == nil {
		views[0].Revisions = n
	}
	// The rating panel is detail-only, so it is not part of decorate.
	s.attachRating(ctx, viewerID, &views[0])
	return views[0], nil
}

func (s *ResearchService) Revisions(ctx context.Context, id int64) ([]model.ResearchRevision, error) {
	viewerID, _ := authctx.UserID(ctx)
	call, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !researchReadable(call, viewerID) {
		return nil, repository.ErrResearchNotFound
	}
	return s.repo.Revisions(ctx, id)
}

// Summary is the caller's scorecard.
func (s *ResearchService) Summary(ctx context.Context) (model.ResearchSummary, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.ResearchSummary{}, err
	}
	calls, err := s.repo.ListAllForUser(ctx, userID)
	if err != nil {
		return model.ResearchSummary{}, err
	}
	return summarizeResearch(calls), nil
}

func (s *ResearchService) SymbolSummary(ctx context.Context, symbol string) (model.ResearchSymbolSummary, error) {
	return s.repo.SymbolSummary(ctx, symbol)
}

// StatusCounts backs the 배치 관리 coverage card for the verdict job.
func (s *ResearchService) StatusCounts(ctx context.Context) (map[string]any, error) {
	counts, err := s.repo.StatusCounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(counts))
	for k, v := range counts {
		out[k] = v
	}
	return out, nil
}

// authorHitRate is the realised hit rate used for the "historical" expected
// value. Nil below the minimum sample — a rate off three calls is a coin flip
// wearing a percentage sign.
func (s *ResearchService) authorHitRate(ctx context.Context, userID int64) *float64 {
	if userID <= 0 {
		return nil
	}
	calls, err := s.repo.ListAllForUser(ctx, userID)
	if err != nil {
		return nil
	}
	summary := summarizeResearch(calls)
	if summary.Resolved < researchMinSample || summary.HitRate == nil {
		return nil
	}
	return summary.HitRate
}

// decorate resolves prices once per distinct symbol/benchmark, then computes
// every horizon in memory.
func (s *ResearchService) decorate(
	ctx context.Context, viewerID int64, calls []model.ResearchCall,
) ([]model.ResearchView, error) {
	views := make([]model.ResearchView, 0, len(calls))
	if len(calls) == 0 {
		return views, nil
	}

	earliestBySymbol := map[string]string{}
	benchmarks := map[string]string{}
	for _, c := range calls {
		if cur, ok := earliestBySymbol[c.Symbol]; !ok || c.TradeDate < cur {
			earliestBySymbol[c.Symbol] = c.TradeDate
		}
		if c.BenchmarkID != "" {
			if cur, ok := benchmarks[c.BenchmarkID]; !ok || c.TradeDate < cur {
				benchmarks[c.BenchmarkID] = c.TradeDate
			}
		}
	}

	priceSeries := map[string]closeSeries{}
	if s.closes != nil {
		for symbol, since := range earliestBySymbol {
			points, err := s.closes.ClosesSince(ctx, symbol, since)
			if err != nil {
				continue
			}
			priceSeries[symbol] = newCloseSeries(points)
		}
	}
	benchSeries := map[string]closeSeries{}
	if s.indexCloses != nil {
		for id, since := range benchmarks {
			points, err := s.indexCloses.ClosesSince(ctx, id, since)
			if err != nil {
				continue
			}
			benchSeries[id] = newCloseSeries(points)
		}
	}

	live := s.livePrices(ctx, sortedKeys(earliestBySymbol))
	hitRate := s.authorHitRate(ctx, viewerID)
	now := time.Now().UTC()
	for _, c := range calls {
		view := buildResearchView(
			c, now, live[c.Symbol], priceSeries[c.Symbol], benchSeries[c.BenchmarkID], hitRate,
		)
		view.CanEdit = viewerID > 0 && viewerID == c.UserID
		views = append(views, view)
	}
	// Fixed number of queries regardless of page size.
	s.attachSocial(ctx, viewerID, views)
	return views, nil
}

func (s *ResearchService) livePrices(ctx context.Context, symbols []string) map[string]float64 {
	out := map[string]float64{}
	if s.quotes == nil || len(symbols) == 0 {
		return out
	}
	for _, q := range s.quotes.BatchQuoteSummaries(ctx, symbols) {
		if q.CurrentPrice > 0 {
			out[strings.ToUpper(q.Symbol)] = q.CurrentPrice
		}
	}
	return out
}

// ---------------------------------------------------------------- mutate

// Update writes the only editable fields. Every call here bumps the public
// revision counter when the prose changed — see ResearchRepo.UpdateBody.
func (s *ResearchService) Update(ctx context.Context, id int64, req ResearchUpdateRequest) (model.ResearchView, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.ResearchView{}, err
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.ResearchView{}, err
	}
	if existing.UserID != userID {
		return model.ResearchView{}, repository.ErrResearchNotFound
	}

	visibility := normalizeVisibility(req.Visibility)
	if visibility == model.VisibilityPublic {
		if ok, reason := researchEligible(existing.Currency, existing.Evidence.MarketCap); !ok {
			return model.ResearchView{}, fmt.Errorf("%w: %s", ErrResearchNotEligible, reason)
		}
	}
	title := normalizeResearchTitle(req.Title, existing.SymbolName, existing.Symbol, existing.Direction)
	body := truncateRunes(strings.TrimSpace(req.BodyMD), researchBodyMax)

	updated, err := s.repo.UpdateBody(ctx, userID, id, title, body,
		normalizeThumbnailURL(req.ThumbnailURL), normalizeResearchTags(req.Tags), string(visibility))
	if err != nil {
		return model.ResearchView{}, err
	}
	views, err := s.decorate(ctx, userID, []model.ResearchCall{updated})
	if err != nil {
		return model.ResearchView{}, err
	}
	return views[0], nil
}

// Retract withdraws a call. It is not a delete: the row keeps counting toward
// the author's retract rate, which is the only reason the retract rate means
// anything.
func (s *ResearchService) Retract(ctx context.Context, id int64) (model.ResearchView, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.ResearchView{}, err
	}
	updated, err := s.repo.Retract(ctx, userID, id)
	if err != nil {
		return model.ResearchView{}, err
	}
	views, err := s.decorate(ctx, userID, []model.ResearchCall{updated})
	if err != nil {
		return model.ResearchView{}, err
	}
	return views[0], nil
}

// Report files a moderation report. Reporting exists before any social feature
// does, because the pump incentive arrives with the first public call.
func (s *ResearchService) Report(ctx context.Context, id int64, reason, detail string) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return err
	}
	reason = strings.TrimSpace(strings.ToLower(reason))
	switch reason {
	case "spam", "manipulation", "misinformation", "abuse", "other":
	default:
		reason = "other"
	}
	return s.repo.AddReport(ctx, id, userID, reason, truncateRunes(strings.TrimSpace(detail), 500))
}

// SetHidden is the admin moderation switch.
func (s *ResearchService) SetHidden(ctx context.Context, id int64, hidden bool) error {
	return s.repo.SetHidden(ctx, id, hidden)
}

// ---------------------------------------------------------------- verdict batch

// RunVerdicts judges every open call. It reads only stored closes, so it costs
// no external quota and must run *after* the daily-close workers have landed
// the session — judging before that would resolve calls against yesterday.
func (s *ResearchService) RunVerdicts(ctx context.Context) (map[string]any, string, error) {
	calls, err := s.repo.ListOpen(ctx, 1000)
	if err != nil {
		return nil, "", err
	}
	result := map[string]any{"scanned": len(calls)}
	if len(calls) == 0 {
		return result, "판정 대상 없음", nil
	}

	// One query per distinct symbol/benchmark, not per call.
	earliestBySymbol := map[string]string{}
	benchmarks := map[string]string{}
	for _, c := range calls {
		if cur, ok := earliestBySymbol[c.Symbol]; !ok || c.TradeDate < cur {
			earliestBySymbol[c.Symbol] = c.TradeDate
		}
		if c.BenchmarkID != "" {
			if cur, ok := benchmarks[c.BenchmarkID]; !ok || c.TradeDate < cur {
				benchmarks[c.BenchmarkID] = c.TradeDate
			}
		}
	}
	priceSeries := map[string]closeSeries{}
	if s.closes != nil {
		for symbol, since := range earliestBySymbol {
			points, err := s.closes.ClosesSince(ctx, symbol, since)
			if err != nil {
				continue
			}
			priceSeries[symbol] = newCloseSeries(points)
		}
	}
	benchSeries := map[string]closeSeries{}
	if s.indexCloses != nil {
		for id, since := range benchmarks {
			points, err := s.indexCloses.ClosesSince(ctx, id, since)
			if err != nil {
				continue
			}
			benchSeries[id] = newCloseSeries(points)
		}
	}

	now := time.Now().UTC()
	counts := map[string]int{}
	resolved := 0
	for _, c := range calls {
		if err := ctx.Err(); err != nil {
			return result, "", err
		}
		j := judgeResearchCall(c, now, priceSeries[c.Symbol], benchSeries[c.BenchmarkID])
		if !j.Changed {
			continue
		}
		if err := s.repo.SetVerdict(ctx, c.ID, j.Status, j.Date, j.Return, j.Alpha); err != nil {
			return result, "", err
		}
		counts[string(j.Status)]++
		resolved++
	}
	result["resolved"] = resolved
	for k, v := range counts {
		result[k] = v
	}
	return result, fmt.Sprintf("%d건 스캔 · %d건 판정", len(calls), resolved), nil
}

// ---------------------------------------------------------------- helpers

// researchReadable decides whether viewer may see this call. Private calls are
// author-only, unlisted ones need the id (which the caller already has).
func researchReadable(c model.ResearchCall, viewerID int64) bool {
	if viewerID > 0 && viewerID == c.UserID {
		return true
	}
	if c.HiddenAt != nil {
		return false
	}
	return c.Visibility != model.VisibilityPrivate
}

func normalizeDirection(v string) model.ResearchDirection {
	switch model.ResearchDirection(strings.ToLower(strings.TrimSpace(v))) {
	case model.DirectionSell:
		return model.DirectionSell
	case model.DirectionHold:
		return model.DirectionHold
	default:
		return model.DirectionBuy
	}
}

// normalizeThumbnailURL accepts a same-origin path (what /api/v1/media/images
// hands back) or an absolute http(s) URL, and drops everything else.
//
// The value is rendered straight into an <img src>, so a scheme like
// `javascript:` or `data:` must never survive the round trip — a report is
// public, and its cover is the one field a reader sees before deciding to
// trust the author.
func normalizeThumbnailURL(raw string) string {
	url := strings.TrimSpace(raw)
	if url == "" {
		return ""
	}
	if len(url) > 512 {
		return ""
	}
	// "//evil.example" is protocol-relative, not a local path.
	if strings.HasPrefix(url, "/") && !strings.HasPrefix(url, "//") {
		return url
	}
	lower := strings.ToLower(url)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return url
	}
	return ""
}

func normalizeVisibility(v string) model.ResearchVisibility {
	switch model.ResearchVisibility(strings.ToLower(strings.TrimSpace(v))) {
	case model.VisibilityPrivate:
		return model.VisibilityPrivate
	case model.VisibilityUnlisted:
		return model.VisibilityUnlisted
	default:
		return model.VisibilityPublic
	}
}

func normalizeResearchHorizon(days int) int {
	for _, h := range model.ResearchHorizons {
		if days == h {
			return days
		}
	}
	return defaultResearchHorizonDays
}

func normalizeConviction(v int) int {
	if v <= 0 {
		return defaultResearchConviction
	}
	if v > 100 {
		return 100
	}
	return v
}

// normalizeBounds rejects a target/stop pair that contradicts the direction.
// An inconsistent pair does not just look odd — it makes the expected value and
// the risk/reward ratio produce numbers with the wrong sign, and those numbers
// are published as if the server vouched for them.
func normalizeBounds(
	direction model.ResearchDirection, base float64, target, stop *float64,
) (*float64, *float64, error) {
	if direction == model.DirectionHold {
		// Neither bound is directional for a 관망 call, so nothing is stored and
		// nothing auto-resolves early.
		return nil, nil, nil
	}
	clean := func(v *float64) *float64 {
		if v == nil || *v <= 0 {
			return nil
		}
		out := *v
		return &out
	}
	t, sp := clean(target), clean(stop)
	if base <= 0 {
		return t, sp, nil
	}
	switch direction {
	case model.DirectionBuy:
		if t != nil && *t <= base {
			return nil, nil, &ResearchInconsistentError{Reason: "매수 콜의 목표가는 현재가보다 높아야 합니다"}
		}
		if sp != nil && *sp >= base {
			return nil, nil, &ResearchInconsistentError{Reason: "매수 콜의 손절가는 현재가보다 낮아야 합니다"}
		}
	case model.DirectionSell:
		if t != nil && *t >= base {
			return nil, nil, &ResearchInconsistentError{Reason: "매도 콜의 목표가는 현재가보다 낮아야 합니다"}
		}
		if sp != nil && *sp <= base {
			return nil, nil, &ResearchInconsistentError{Reason: "매도 콜의 손절가는 현재가보다 높아야 합니다"}
		}
	}
	return t, sp, nil
}

// isContrarian marks a call that runs against our own score. Aggregating these
// separately is the only way to tell "beat the market" from "agreed with the
// model and the model was right".
func isContrarian(direction model.ResearchDirection, score *int) bool {
	if score == nil {
		return false
	}
	switch direction {
	case model.DirectionBuy:
		return *score <= 40
	case model.DirectionSell:
		return *score >= 70
	}
	return false
}

// researchEligible applies the public-publishing floor (§14). When market cap
// is unknown we allow it rather than silently blocking — reporting "too small"
// for a symbol we simply failed to measure would be a lie.
func researchEligible(currency model.QuoteCurrency, marketCap float64) (bool, string) {
	if marketCap <= 0 {
		return true, ""
	}
	floor := float64(researchMinMarketCapKRW)
	unit := "300억원"
	if currency == model.CurrencyUSD {
		floor = researchMinMarketCapUSD
		unit = "3천만 달러"
	}
	if marketCap < floor {
		return false, fmt.Sprintf("시가총액이 공개 발행 기준(%s) 미만입니다. 비공개로는 저장할 수 있습니다", unit)
	}
	return true, ""
}

func normalizeResearchTitle(title, symbolName, symbol string, direction model.ResearchDirection) string {
	t := truncateRunes(strings.TrimSpace(title), researchTitleMax)
	if t != "" {
		return t
	}
	name := symbolName
	if name == "" {
		name = symbol
	}
	label := "매수"
	switch direction {
	case model.DirectionSell:
		label = "매도"
	case model.DirectionHold:
		label = "관망"
	}
	return fmt.Sprintf("%s %s 콜", name, label)
}

func normalizeResearchTags(tags []string) []string {
	cleaned := make([]string, 0, researchTagsMax)
	seen := map[string]struct{}{}
	for _, t := range tags {
		t = truncateRunes(strings.TrimSpace(t), researchTagMaxLen)
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		cleaned = append(cleaned, t)
		if len(cleaned) >= researchTagsMax {
			break
		}
	}
	return cleaned
}

func truncateRunes(s string, max int) string {
	if runes := []rune(s); len(runes) > max {
		return string(runes[:max])
	}
	return s
}
