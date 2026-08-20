package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

const (
	// stampCooldown blocks re-stamping the same symbol — a stamp is a moment,
	// and two of them a minute apart record the same moment twice.
	stampCooldown = 5 * time.Minute
	// stampMaxPerUser bounds storage (each stamp carries a chart snapshot).
	stampMaxPerUser = 5000

	stampChartBars   = 120
	stampNewsMax     = 5
	stampDiscloseMax = 5
	stampNoteMaxLen  = 1000
	stampTagsMax     = 5
	stampTagMaxLen   = 20

	// stampPredictionGrace is how long a fresh stamp's target and deadline stay
	// editable. It exists for typos only: past it, revising the prediction after
	// watching the price would make the hit rate meaningless. Same length as the
	// cooldown, so "you cannot re-stamp yet" and "you can still fix it" cover the
	// same window.
	stampPredictionGrace = stampCooldown

	// stampTargetMaxMultiple bounds a target relative to the captured price. A
	// fat-fingered extra digit is a data-entry error, not a 900% call.
	stampTargetMaxMultiple = 10.0
)

var (
	ErrStampLimitReached     = errors.New("stamp limit reached")
	ErrStampNoPrice          = errors.New("no price available for symbol")
	ErrStampTargetRequired   = errors.New("target price required")
	ErrStampTargetOutOfRange = errors.New("target price out of range")
	// ErrStampPredictionLocked is returned when the grace window has closed.
	ErrStampPredictionLocked = errors.New("prediction locked")
)

// StampCooldownError carries the remaining wait so the button can count down
// instead of just failing.
type StampCooldownError struct {
	RemainingSeconds int
}

func (e *StampCooldownError) Error() string {
	return fmt.Sprintf("same symbol stamped within %d minutes", int(stampCooldown.Minutes()))
}

// StampCreateRequest is the client payload. Everything factual is collected
// server-side — the client cannot dictate the captured price. The prediction
// (target price + deadline) is the one thing that must come from the user, and
// TargetPrice has no default: a stamp without one cannot be scored.
//
// Stance is absent on purpose — it is derived from TargetPrice vs the captured
// price so the two can never contradict each other.
type StampCreateRequest struct {
	Symbol      string
	TargetPrice float64
	HorizonKind string
	// CustomDueDate is YYYY-MM-DD, read only when HorizonKind is "custom".
	CustomDueDate string
	Conviction    int
	Note          string
	Tags          []string
	Interval      string
}

// StampAnnotateRequest edits the parts that stay open for the life of a stamp.
type StampAnnotateRequest struct {
	Conviction int
	Note       string
	Tags       []string
}

// StampPredictionRequest rewrites the scored fields, allowed only inside
// stampPredictionGrace.
type StampPredictionRequest struct {
	TargetPrice   float64
	HorizonKind   string
	CustomDueDate string
}

// StampService captures and replays stamps. Every collaborator is optional:
// a missing one degrades that part of the snapshot into a dataGap rather than
// failing the stamp.
type StampService struct {
	repo        *repository.StampRepo
	closes      *repository.StockDailyCloseRepo
	indexCloses *repository.IndexDailyCloseRepo
	stocks      *repository.StockRepo
	candles     *repository.CandleBarRepo

	quotes      *QuotesService
	scores      *ScoreService
	news        *NewsStore
	indexStore  *MarketIndexStore
	marketStore *MarketDataStore
	holdings    *UserHoldingsService
}

func NewStampService(
	repo *repository.StampRepo,
	closes *repository.StockDailyCloseRepo,
	indexCloses *repository.IndexDailyCloseRepo,
	stocks *repository.StockRepo,
	candles *repository.CandleBarRepo,
) *StampService {
	return &StampService{repo: repo, closes: closes, indexCloses: indexCloses, stocks: stocks, candles: candles}
}

func (s *StampService) SetQuotes(q *QuotesService)         { s.quotes = q }
func (s *StampService) SetScores(sc *ScoreService)         { s.scores = sc }
func (s *StampService) SetNews(n *NewsStore)               { s.news = n }
func (s *StampService) SetIndexStore(i *MarketIndexStore)  { s.indexStore = i }
func (s *StampService) SetMarketStore(m *MarketDataStore)  { s.marketStore = m }
func (s *StampService) SetHoldings(h *UserHoldingsService) { s.holdings = h }

// ---------------------------------------------------------------- create

func (s *StampService) Create(ctx context.Context, req StampCreateRequest) (model.StampView, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.StampView{}, err
	}
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		return model.StampView{}, errors.New("symbol required")
	}

	now := time.Now().UTC()
	if last, ok, err := s.repo.LastStampedAt(ctx, userID, symbol); err != nil {
		return model.StampView{}, err
	} else if ok {
		if elapsed := now.Sub(last); elapsed < stampCooldown {
			remaining := int((stampCooldown - elapsed).Seconds())
			if remaining < 1 {
				remaining = 1
			}
			return model.StampView{}, &StampCooldownError{RemainingSeconds: remaining}
		}
	}
	if total, err := s.repo.CountForUser(ctx, userID); err != nil {
		return model.StampView{}, err
	} else if total >= stampMaxPerUser {
		return model.StampView{}, ErrStampLimitReached
	}

	stamp, err := s.capture(ctx, symbol, now, req.Interval)
	if err != nil {
		return model.StampView{}, err
	}
	// Validated against the captured price, not a price the client sent.
	if err := validateTargetPrice(stamp.Price, req.TargetPrice); err != nil {
		return model.StampView{}, err
	}
	stamp.TargetPrice = req.TargetPrice
	stamp.Stance = model.StanceForTarget(stamp.Price, req.TargetPrice)
	stamp.HorizonKind = normalizeHorizonKind(req.HorizonKind)
	stamp.DueAt = resolveDueAt(stamp.HorizonKind, stamp.AssetType, stamp.StampedAt, req.CustomDueDate)
	stamp.Conviction, stamp.Note, stamp.Tags = normalizeAnnotation(req.Conviction, req.Note, req.Tags)

	created, err := s.repo.Create(ctx, userID, stamp)
	if err != nil {
		return model.StampView{}, err
	}
	return s.decorate(ctx, []model.Stamp{created})[0], nil
}

// validateTargetPrice keeps the prediction inside sane bounds. The upper bound
// catches a mistyped extra digit, which is otherwise indistinguishable from a
// deliberate moonshot and would skew the average-error stat forever.
func validateTargetPrice(price, target float64) error {
	if target <= 0 {
		return ErrStampTargetRequired
	}
	if price > 0 && (target > price*stampTargetMaxMultiple || target < price/stampTargetMaxMultiple) {
		return ErrStampTargetOutOfRange
	}
	return nil
}

// capture collects the immutable snapshot. It reads only paths the stock page
// itself already uses (terminal cache → MySQL), so stamping costs no extra
// provider budget beyond the standard cold-miss backfill.
func (s *StampService) capture(ctx context.Context, symbol string, now time.Time, interval string) (model.Stamp, error) {
	stamp := model.Stamp{
		Symbol:    symbol,
		StampedAt: now,
		Currency:  model.CurrencyKRW,
	}
	var gaps []string

	if s.stocks != nil {
		if st, err := s.stocks.GetBySymbol(ctx, symbol); err == nil && st != nil {
			stamp.SymbolName = st.Name
			stamp.AssetType = st.AssetType
			if st.Currency != "" {
				stamp.Currency = st.Currency
			}
			stamp.BenchmarkID = benchmarkForStock(st.AssetType, st.Market)
		}
	}

	var quote model.QuoteDetail
	haveQuote := false
	if s.quotes != nil {
		quote, haveQuote = s.quotes.GetQuote(ctx, symbol)
	}
	if haveQuote {
		stamp.Price = quote.CurrentPrice
		stamp.Volume = quote.Volume
		stamp.MarketSession = quote.MarketSession
		if quote.Name != "" {
			stamp.SymbolName = quote.Name
		}
		if quote.Currency != "" {
			stamp.Currency = quote.Currency
		}
		if quote.AssetType != "" && stamp.BenchmarkID == "" {
			stamp.BenchmarkID = benchmarkForStock(quote.AssetType, quote.Market)
		}
		if quote.AssetType != "" {
			stamp.AssetType = quote.AssetType
		}
		if quote.PrevClose > 0 {
			prev := quote.PrevClose
			stamp.PrevClose = &prev
		}
		rate := quote.ChangeRate
		stamp.ChangeRate = &rate
		stamp.Snapshot = model.StampSnapshot{
			Open:         quote.Open,
			High:         quote.High,
			Low:          quote.Low,
			Week52High:   quote.Week52High,
			Week52Low:    quote.Week52Low,
			MarketCap:    quote.MarketCap,
			TradingValue: quote.TradingValue,
			PER:          quote.PER,
			PBR:          quote.PBR,
			Watchlisted:  quote.IsWatchlisted,
			Held:         quote.IsHeld,
		}
	} else {
		gaps = append(gaps, "quote")
	}

	// Latest stored close doubles as the price fallback and the AI-summary source.
	var closeRow *repository.StockDailyCloseRow
	if s.closes != nil {
		if rows, err := s.closes.List(ctx, symbol, 1); err == nil && len(rows) > 0 {
			closeRow = &rows[0]
		}
	}
	if stamp.Price <= 0 && closeRow != nil {
		stamp.Price = closeRow.Close
		if closeRow.PrevClose.Valid {
			prev := closeRow.PrevClose.Float64
			stamp.PrevClose = &prev
		}
		if closeRow.ChangeRate.Valid {
			rate := closeRow.ChangeRate.Float64
			stamp.ChangeRate = &rate
		}
		stamp.Volume = closeRow.Volume
	}
	// Outside trading hours the live quote reports 0 volume, which would record
	// "거래량 —" for a session that actually traded. The stored close holds the
	// session total, so fall back to it rather than freezing a zero.
	if stamp.Volume == 0 && closeRow != nil {
		stamp.Volume = closeRow.Volume
	}
	if stamp.Price <= 0 {
		return model.Stamp{}, ErrStampNoPrice
	}

	stamp.TradeDate = marketTradeDate(now, stamp.AssetType)
	if stamp.BenchmarkID == "" {
		stamp.BenchmarkID = benchmarkForStock(stamp.AssetType, "")
	}

	if s.scores != nil {
		if view, err := s.scores.Get(ctx, symbol); err == nil && view != nil {
			stamp.Score = view.Score
			stamp.Grade = view.Grade
			stamp.Snapshot.Breakdown = view.Breakdown
			stamp.Snapshot.Coverage = view.Coverage
			stamp.Snapshot.PeerGroup = view.PeerGroup
			stamp.Snapshot.PeerPct = view.PeerPct
			stamp.Snapshot.ScoreKind = view.ScoreKind
		}
	}
	if stamp.Score == nil {
		gaps = append(gaps, "score")
	}

	stamp.Context = model.StampContext{
		News:        s.captureNews(ctx, symbol, stamp.AssetType),
		Disclosures: s.captureDisclosures(ctx, symbol),
		Indices:     s.captureIndices(ctx),
	}
	if closeRow != nil && closeRow.AISummary != "" {
		stamp.Context.AISummary = closeRow.AISummary
	} else {
		gaps = append(gaps, "aiSummary")
	}
	if len(stamp.Context.News) == 0 {
		gaps = append(gaps, "news")
	}
	// Disclosures have no historical table (docs/STAMP_DESIGN.md §5.2): if the
	// day's cache is cold there is nothing to copy and nothing to recover later.
	if len(stamp.Context.Disclosures) == 0 {
		gaps = append(gaps, "disclosures")
	}

	stamp.Chart = s.captureChart(ctx, symbol, interval)
	if len(stamp.Chart.Bars) == 0 {
		gaps = append(gaps, "chart")
	}

	s.attachHolding(ctx, &stamp)

	stamp.Context.DataGaps = gaps
	return stamp, nil
}

func (s *StampService) captureNews(ctx context.Context, symbol string, assetType model.QuoteAssetType) []model.StampNews {
	out := make([]model.StampNews, 0, stampNewsMax)
	if s.news == nil {
		return out
	}
	market := "domestic"
	if assetType == model.AssetUS || assetType == model.AssetETF {
		market = "us"
	}
	articles, _ := s.news.List(ctx, model.NewsListFilter{Market: market, Symbol: symbol, Limit: stampNewsMax})
	for _, a := range articles {
		headline := a.HeadlineKo
		if headline == "" {
			headline = a.Headline
		}
		out = append(out, model.StampNews{
			ID:            a.ID,
			Headline:      headline,
			URL:           a.URL,
			PublisherName: a.PublisherName,
			PublishedAt:   a.PublishedAt,
		})
		if len(out) >= stampNewsMax {
			break
		}
	}
	return out
}

func (s *StampService) captureDisclosures(ctx context.Context, symbol string) []model.BriefingEvent {
	out := make([]model.BriefingEvent, 0, stampDiscloseMax)
	if s.marketStore == nil {
		return out
	}
	// Cache-only read on purpose — EnsureTodayDisclosures would call DART.
	for _, ev := range s.marketStore.GetTodayDisclosures(ctx, []string{symbol}) {
		out = append(out, ev)
		if len(out) >= stampDiscloseMax {
			break
		}
	}
	return out
}

// stampIndexIDs is the market backdrop kept with every stamp — two domestic,
// two US, plus FX, which is what the dashboard header shows.
var stampIndexIDs = []string{"kospi", "kosdaq", "sp500", "nasdaq", "usd-krw"}

func (s *StampService) captureIndices(ctx context.Context) []model.StampIndex {
	out := make([]model.StampIndex, 0, len(stampIndexIDs))
	if s.indexStore == nil {
		return out
	}
	wanted := map[string]struct{}{}
	for _, id := range stampIndexIDs {
		wanted[id] = struct{}{}
	}
	indices, _ := s.indexStore.List(ctx)
	for _, idx := range indices {
		if _, ok := wanted[idx.ID]; !ok {
			continue
		}
		out = append(out, model.StampIndex{
			ID: idx.ID, Name: idx.Name, Value: idx.Value, ChangeRate: idx.ChangeRate,
		})
	}
	return out
}

func (s *StampService) captureChart(ctx context.Context, symbol, interval string) model.StampChart {
	iv := model.CandleInterval(strings.TrimSpace(interval))
	switch iv {
	case model.Interval1m, model.Interval1d:
	default:
		iv = model.Interval1d
	}
	chart := model.StampChart{Interval: string(iv), Bars: []model.Candle{}}
	if s.candles == nil {
		return chart
	}
	bars, _, err := s.candles.ListBefore(ctx, symbol, iv, "", stampChartBars)
	if err != nil || len(bars) == 0 {
		return chart
	}
	chart.Bars = bars
	return chart
}

func (s *StampService) attachHolding(ctx context.Context, stamp *model.Stamp) {
	if s.holdings == nil {
		return
	}
	rows, err := s.holdings.List(ctx)
	if err != nil {
		return
	}
	for _, h := range rows {
		if !strings.EqualFold(h.Symbol, stamp.Symbol) {
			continue
		}
		stamp.Snapshot.Held = true
		stamp.Snapshot.HoldingQuantity = h.Quantity
		stamp.Snapshot.HoldingAvgPrice = h.AvgPrice
		return
	}
}

// ---------------------------------------------------------------- read

func (s *StampService) List(ctx context.Context, f repository.StampListFilter) ([]model.StampView, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	stamps, err := s.repo.List(ctx, userID, f)
	if err != nil {
		return nil, err
	}
	return s.decorate(ctx, stamps), nil
}

func (s *StampService) Get(ctx context.Context, id int64) (model.StampView, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.StampView{}, err
	}
	stamp, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return model.StampView{}, err
	}
	return s.decorate(ctx, []model.Stamp{stamp})[0], nil
}

// Summary aggregates every stamp the user owns (not just the current page).
func (s *StampService) Summary(ctx context.Context) (model.StampSummary, error) {
	views, err := s.List(ctx, repository.StampListFilter{Limit: 500})
	if err != nil {
		return model.StampSummary{}, err
	}
	return summarizeStamps(views), nil
}

// State drives the stamp button: how many stamps this symbol has and how long
// the cooldown still blocks a new one.
func (s *StampService) State(ctx context.Context, symbol string) (model.StampState, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.StampState{}, err
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	out := model.StampState{Symbol: sym}
	count, err := s.repo.CountForSymbol(ctx, userID, sym)
	if err != nil {
		return model.StampState{}, err
	}
	out.Count = count
	last, ok, err := s.repo.LastStampedAt(ctx, userID, sym)
	if err != nil {
		return model.StampState{}, err
	}
	if ok {
		at := last
		out.LastStampedAt = &at
		if elapsed := time.Since(last); elapsed < stampCooldown {
			out.CooldownRemaining = int((stampCooldown - elapsed).Seconds())
		}
	}
	return out, nil
}

// decorate resolves prices once per distinct symbol/benchmark, then computes
// every horizon in memory.
func (s *StampService) decorate(ctx context.Context, stamps []model.Stamp) []model.StampView {
	views := make([]model.StampView, 0, len(stamps))
	if len(stamps) == 0 {
		return views
	}

	earliestBySymbol := map[string]string{}
	benchmarks := map[string]string{}
	for _, st := range stamps {
		if cur, ok := earliestBySymbol[st.Symbol]; !ok || st.TradeDate < cur {
			earliestBySymbol[st.Symbol] = st.TradeDate
		}
		if st.BenchmarkID != "" {
			if cur, ok := benchmarks[st.BenchmarkID]; !ok || st.TradeDate < cur {
				benchmarks[st.BenchmarkID] = st.TradeDate
			}
		}
	}

	priceSeries := map[string]closeSeries{}
	for symbol, since := range earliestBySymbol {
		if s.closes == nil {
			break
		}
		points, err := s.closes.ClosesSince(ctx, symbol, since)
		if err != nil {
			continue
		}
		priceSeries[symbol] = newCloseSeries(points)
	}
	benchSeries := map[string]closeSeries{}
	for id, since := range benchmarks {
		if s.indexCloses == nil {
			break
		}
		points, err := s.indexCloses.ClosesSince(ctx, id, since)
		if err != nil {
			continue
		}
		benchSeries[id] = newCloseSeries(points)
	}

	symbols := sortedKeys(earliestBySymbol)
	live := s.livePrices(ctx, symbols)
	current := s.currentScores(ctx, symbols)
	// One range query per symbol, covering that symbol's oldest stamp — each
	// stamp then slices the shared series by its own dates.
	extremes := s.dailyExtremes(ctx, earliestBySymbol)
	now := time.Now().UTC()
	for _, st := range stamps {
		view := buildStampView(
			st, now, live[st.Symbol], priceSeries[st.Symbol], benchSeries[st.BenchmarkID],
			extremesSince(extremes[st.Symbol], st.TradeDate),
		)
		// The 당시 vs 현재 score comparison is the whole point of keeping the
		// breakdown in the snapshot, so pair it with today's score here.
		if score, ok := current[st.Symbol]; ok && view.Current != nil {
			view.Current.Score = score.Score
			view.Current.Grade = score.Grade
			view.Current.Breakdown = score.Breakdown
			if view.Current.TradeDate == "" {
				view.Current.TradeDate = score.TradeDate
			}
		}
		views = append(views, view)
	}
	return views
}

// dailyExtremes loads daily highs/lows per symbol from that symbol's earliest
// stamp onward. `to` is left open — the caller's own deadline bounds the scan,
// and a single open-ended range serves every stamp on the symbol.
func (s *StampService) dailyExtremes(
	ctx context.Context, earliestBySymbol map[string]string,
) map[string][]repository.DailyExtreme {
	out := map[string][]repository.DailyExtreme{}
	if s.candles == nil {
		return out
	}
	for symbol, since := range earliestBySymbol {
		rows, err := s.candles.DailyExtremesBetween(ctx, symbol, since, "")
		if err != nil || len(rows) == 0 {
			continue
		}
		out[symbol] = rows
	}
	return out
}

// extremesSince trims the shared series to the days strictly after a stamp's
// own trade date. The stamp day is dropped because its high/low may have been
// set before the stamp was taken.
func extremesSince(rows []repository.DailyExtreme, tradeDate string) []repository.DailyExtreme {
	for i, r := range rows {
		if r.Date > tradeDate {
			return rows[i:]
		}
	}
	return nil
}

func (s *StampService) currentScores(ctx context.Context, symbols []string) map[string]*model.StockScoreView {
	out := map[string]*model.StockScoreView{}
	if s.scores == nil {
		return out
	}
	for _, sym := range symbols {
		view, err := s.scores.Get(ctx, sym)
		if err != nil || view == nil {
			continue
		}
		out[sym] = view
	}
	return out
}

func (s *StampService) livePrices(ctx context.Context, symbols []string) map[string]float64 {
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

// Annotate edits conviction, note and tags — the fields that carry no claim
// about the future and so stay open forever.
func (s *StampService) Annotate(ctx context.Context, id int64, req StampAnnotateRequest) (model.StampView, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.StampView{}, err
	}
	conviction, note, tags := normalizeAnnotation(req.Conviction, req.Note, req.Tags)
	updated, err := s.repo.UpdateAnnotation(ctx, userID, id, conviction, note, tags)
	if err != nil {
		return model.StampView{}, err
	}
	return s.decorate(ctx, []model.Stamp{updated})[0], nil
}

// Revise rewrites the target price and deadline, but only while the stamp is
// still fresh. A prediction the user can move after seeing where the price went
// is not a prediction, and the hit rate would stop meaning anything.
func (s *StampService) Revise(ctx context.Context, id int64, req StampPredictionRequest) (model.StampView, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.StampView{}, err
	}
	existing, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return model.StampView{}, err
	}
	if time.Since(existing.StampedAt) > stampPredictionGrace {
		return model.StampView{}, ErrStampPredictionLocked
	}
	if err := validateTargetPrice(existing.Price, req.TargetPrice); err != nil {
		return model.StampView{}, err
	}

	kind := normalizeHorizonKind(req.HorizonKind)
	updated, err := s.repo.UpdatePrediction(ctx, userID, id,
		string(model.StanceForTarget(existing.Price, req.TargetPrice)),
		req.TargetPrice, string(kind),
		resolveDueAt(kind, existing.AssetType, existing.StampedAt, req.CustomDueDate),
	)
	if err != nil {
		return model.StampView{}, err
	}
	return s.decorate(ctx, []model.Stamp{updated})[0], nil
}

func (s *StampService) Open(ctx context.Context, id int64) (model.StampView, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.StampView{}, err
	}
	if err := s.repo.MarkOpened(ctx, userID, id); err != nil {
		return model.StampView{}, err
	}
	return s.Get(ctx, id)
}

func (s *StampService) Delete(ctx context.Context, id int64) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, userID, id)
}

// ---------------------------------------------------------------- helpers

func normalizeAnnotation(
	conviction int, note string, tags []string,
) (int, string, []string) {
	// 1..5 since v2 — the form shows five steps, and clamping to the old 1..3
	// would quietly file every "4" and "5" as the default.
	if conviction < 1 || conviction > 5 {
		conviction = 3
	}
	note = strings.TrimSpace(note)
	if runes := []rune(note); len(runes) > stampNoteMaxLen {
		note = string(runes[:stampNoteMaxLen])
	}

	cleaned := make([]string, 0, stampTagsMax)
	seen := map[string]struct{}{}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if runes := []rune(t); len(runes) > stampTagMaxLen {
			t = string(runes[:stampTagMaxLen])
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		cleaned = append(cleaned, t)
		if len(cleaned) >= stampTagsMax {
			break
		}
	}
	return conviction, note, cleaned
}

// Series/benchmark/trade-date helpers live in close_series.go — they are market
// mechanics shared with other features, not stamp behaviour.
