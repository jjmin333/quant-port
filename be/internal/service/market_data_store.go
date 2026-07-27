package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

const (
	marketPriceKeyFmt        = "market:price:v1:%s"
	marketPriceLastKeyFmt    = "market:price:last:v1:%s"
	marketPriceTTL           = 20 * time.Second
	marketCalendarKRKey      = "market:calendar:kr:v1"
	marketCalendarUSKey      = "market:calendar:us:v1"
	marketCalendarTTL        = 26 * time.Hour
	marketCalendarPoll       = 24 * time.Hour
	marketEarningsKey        = "market:earnings:calendar:v1"
	marketEarningsTTL        = 1 * time.Hour
	marketEarningsPoll       = 1 * time.Hour
	marketWarningsKeyFmt     = "market:warnings:v1:%s"
	marketWarningsTTL        = 6 * time.Hour
	marketDisclosuresKeyFmt  = "market:disclosures:v1:%s:%s"
	marketDisclosuresTTL     = 1 * time.Hour
	marketInsightsDataPoll   = 15 * time.Minute
	marketRateLimitBackoff   = 60 * time.Second
)

type cachedMarketPrice struct {
	LastPrice string        `json:"lastPrice"`
	Currency  toss.Currency `json:"currency,omitempty"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type MarketDataStore struct {
	toss          *toss.Client
	redis         *redis.Client
	earnings      *EarningsClient
	dart          *DartClient
	portfolio     *PortfolioService
	watchlist     *repository.WatchlistRepo
	quoteTerminal *repository.QuoteTerminalRepo

	mu           sync.RWMutex
	calendars    marketCalendars
	calendarsAt  time.Time
	priceBackoff time.Time
	calBackoff   time.Time

	activeMu sync.Mutex
	active   map[string]time.Time
}

func NewMarketDataStore(
	tossClient *toss.Client,
	redisClient *redis.Client,
	earnings *EarningsClient,
	dart *DartClient,
	portfolio *PortfolioService,
	watchlist *repository.WatchlistRepo,
	quoteTerminal *repository.QuoteTerminalRepo,
) *MarketDataStore {
	return &MarketDataStore{
		toss:          tossClient,
		redis:         redisClient,
		earnings:      earnings,
		dart:          dart,
		portfolio:     portfolio,
		watchlist:     watchlist,
		quoteTerminal: quoteTerminal,
		active:        make(map[string]time.Time),
	}
}

func (s *MarketDataStore) Run(ctx context.Context) {
	s.loadCalendarsFromRedis(ctx)
	s.refreshCalendars(ctx)
	s.refreshEarnings(ctx)
	s.refreshInsightsData(ctx)

	calTicker := time.NewTicker(marketCalendarPoll)
	earningsTicker := time.NewTicker(marketEarningsPoll)
	insightsTicker := time.NewTicker(marketInsightsDataPoll)
	defer calTicker.Stop()
	defer earningsTicker.Stop()
	defer insightsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-calTicker.C:
			s.refreshCalendars(ctx)
		case <-earningsTicker.C:
			s.refreshEarnings(ctx)
		case <-insightsTicker.C:
			s.refreshInsightsData(ctx)
		}
	}
}

func (s *MarketDataStore) Touch(symbol string) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return
	}
	s.activeMu.Lock()
	s.active[sym] = time.Now()
	s.activeMu.Unlock()
}

func (s *MarketDataStore) activeSymbols() []string {
	now := time.Now()
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	var out []string
	for sym, seenAt := range s.active {
		if now.Sub(seenAt) > marketActiveTTL {
			delete(s.active, sym)
			continue
		}
		out = append(out, sym)
	}
	return out
}

func (s *MarketDataStore) EnsurePrice(ctx context.Context, symbol string) (float64, bool) {
	prices := s.EnsurePrices(ctx, []string{symbol})
	p, ok := prices[strings.ToUpper(strings.TrimSpace(symbol))]
	if !ok {
		return 0, false
	}
	v := parseDecimal(p.LastPrice)
	return v, v > 0
}

func (s *MarketDataStore) EnsurePrices(ctx context.Context, symbols []string) map[string]toss.PriceResponse {
	for _, sym := range symbols {
		s.Touch(sym)
	}
	out := s.GetPrices(ctx, symbols)
	var missing []string
	seen := map[string]struct{}{}
	for _, sym := range symbols {
		up := strings.ToUpper(strings.TrimSpace(sym))
		if up == "" {
			continue
		}
		if _, ok := out[up]; ok {
			continue
		}
		if _, ok := seen[up]; ok {
			continue
		}
		seen[up] = struct{}{}
		missing = append(missing, up)
	}
	if len(missing) > 0 {
		s.RefreshPrices(ctx, missing)
		for sym, p := range s.GetPrices(ctx, missing) {
			out[sym] = p
		}
	}
	return out
}

func (s *MarketDataStore) EnsureCalendars(ctx context.Context) (marketCalendars, bool) {
	if s.calendarsNeedRefresh(time.Now()) {
		s.refreshCalendars(ctx)
	}
	return s.GetCalendars(ctx)
}

func (s *MarketDataStore) AllTrackedSymbols(ctx context.Context) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(sym string) {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			return
		}
		if _, ok := seen[sym]; ok {
			return
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}

	if s.portfolio != nil {
		for _, sym := range s.portfolio.AllTrackedSymbols(ctx) {
			add(sym)
		}
	}
	if s.watchlist != nil {
		symbols, err := s.watchlist.ListAllSymbols(ctx)
		if err == nil {
			for _, sym := range symbols {
				add(sym)
			}
		}
	}
	for _, sym := range s.activeSymbols() {
		add(sym)
	}
	return out
}

func (s *MarketDataStore) GetCalendars(ctx context.Context) (marketCalendars, bool) {
	s.mu.RLock()
	if !s.calendarsAt.IsZero() {
		cals := s.calendars
		s.mu.RUnlock()
		return cals, cals.us != nil || cals.kr != nil
	}
	s.mu.RUnlock()

	if s.loadCalendarsFromRedis(ctx) {
		s.mu.RLock()
		cals := s.calendars
		s.mu.RUnlock()
		return cals, cals.us != nil || cals.kr != nil
	}
	return marketCalendars{}, false
}

func (s *MarketDataStore) loadCalendarsFromRedis(ctx context.Context) bool {
	if s.redis == nil {
		return false
	}
	var cals marketCalendars
	if raw, err := s.redis.Get(ctx, marketCalendarUSKey).Bytes(); err == nil && len(raw) > 0 {
		var us toss.UsMarketCalendarResponse
		if json.Unmarshal(raw, &us) == nil {
			cals.us = &us
		}
	}
	if raw, err := s.redis.Get(ctx, marketCalendarKRKey).Bytes(); err == nil && len(raw) > 0 {
		var kr toss.KrMarketCalendarResponse
		if json.Unmarshal(raw, &kr) == nil {
			cals.kr = &kr
		}
	}
	if cals.us == nil && cals.kr == nil {
		return false
	}
	s.mu.Lock()
	s.calendars = cals
	s.calendarsAt = time.Now()
	s.mu.Unlock()
	return true
}

func (s *MarketDataStore) refreshCalendars(ctx context.Context) {
	s.mu.RLock()
	if time.Now().Before(s.calBackoff) {
		s.mu.RUnlock()
		return
	}
	needRefresh := s.calendarsNeedRefreshLocked(time.Now())
	s.mu.RUnlock()
	if !needRefresh {
		return
	}

	if s.toss == nil || !s.toss.Enabled() {
		return
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var cals marketCalendars
	if us, err := s.toss.GetUsMarketCalendar(refreshCtx, ""); err == nil {
		cals.us = us
		if s.redis != nil {
			if payload, err := json.Marshal(us); err == nil {
				_ = s.redis.Set(refreshCtx, marketCalendarUSKey, payload, marketCalendarTTL).Err()
			}
		}
	} else if isRateLimitError(err) {
		s.mu.Lock()
		s.calBackoff = time.Now().Add(marketRateLimitBackoff)
		s.mu.Unlock()
		log.Printf("market calendar poller: rate limited, backing off")
		return
	}

	if kr, err := s.toss.GetKrMarketCalendar(refreshCtx, ""); err == nil {
		cals.kr = kr
		if s.redis != nil {
			if payload, err := json.Marshal(kr); err == nil {
				_ = s.redis.Set(refreshCtx, marketCalendarKRKey, payload, marketCalendarTTL).Err()
			}
		}
	} else if isRateLimitError(err) {
		s.mu.Lock()
		s.calBackoff = time.Now().Add(marketRateLimitBackoff)
		s.mu.Unlock()
		return
	}

	if cals.us != nil || cals.kr != nil {
		s.mu.Lock()
		s.calendars = cals
		s.calendarsAt = time.Now()
		s.mu.Unlock()
	}
}

func (s *MarketDataStore) calendarsNeedRefresh(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.calendarsNeedRefreshLocked(now)
}

func (s *MarketDataStore) calendarsNeedRefreshLocked(now time.Time) bool {
	if s.calendarsAt.IsZero() {
		return true
	}
	if now.Sub(s.calendarsAt) >= marketCalendarPoll {
		return true
	}
	return marketCalendarsDateStale(s.calendars, now)
}

func marketCalendarsDateStale(cals marketCalendars, now time.Time) bool {
	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		seoul = time.FixedZone("KST", 9*3600)
	}
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		ny = time.FixedZone("EST", -5*3600)
	}
	if cals.kr != nil && cals.kr.Today.Date != "" {
		if cals.kr.Today.Date != now.In(seoul).Format("2006-01-02") {
			return true
		}
	}
	if cals.us != nil && cals.us.Today.Date != "" {
		if cals.us.Today.Date != now.In(ny).Format("2006-01-02") {
			return true
		}
	}
	return false
}

func (s *MarketDataStore) GetPrices(ctx context.Context, symbols []string) map[string]toss.PriceResponse {
	out := make(map[string]toss.PriceResponse, len(symbols))
	if len(symbols) == 0 {
		return out
	}
	if s.redis == nil {
		return out
	}

	keys := make([]string, 0, len(symbols))
	keyToSym := make(map[string]string, len(symbols))
	for _, sym := range symbols {
		up := strings.ToUpper(strings.TrimSpace(sym))
		if up == "" {
			continue
		}
		key := fmt.Sprintf(marketPriceKeyFmt, up)
		keys = append(keys, key)
		keyToSym[key] = up
	}
	if len(keys) == 0 {
		return out
	}

	vals, err := s.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return out
	}
	for i, val := range vals {
		if val == nil {
			continue
		}
		raw, ok := val.(string)
		if !ok || raw == "" {
			continue
		}
		var cached cachedMarketPrice
		if json.Unmarshal([]byte(raw), &cached) != nil || cached.LastPrice == "" {
			continue
		}
		sym := keyToSym[keys[i]]
		out[sym] = toss.PriceResponse{
			Symbol:    sym,
			LastPrice: cached.LastPrice,
			Currency:  cached.Currency,
		}
	}

	for _, sym := range symbols {
		up := strings.ToUpper(strings.TrimSpace(sym))
		if up == "" || out[up].LastPrice != "" {
			continue
		}
		if p, ok := s.getLastGoodPrice(ctx, up); ok {
			out[up] = p
		}
	}

	if s.quoteTerminal != nil {
		var missing []string
		for _, sym := range symbols {
			up := strings.ToUpper(strings.TrimSpace(sym))
			if up == "" || out[up].LastPrice != "" {
				continue
			}
			missing = append(missing, up)
		}
		for sym, p := range s.quoteTerminal.GetLastPrices(ctx, missing) {
			out[sym] = p
			s.cacheLastGoodPrice(ctx, sym, cachedMarketPrice{
				LastPrice: p.LastPrice,
				Currency:  p.Currency,
				UpdatedAt: time.Now(),
			})
		}
	}
	return out
}

func (s *MarketDataStore) getLastGoodPrice(ctx context.Context, symbol string) (toss.PriceResponse, bool) {
	if s.redis == nil {
		return toss.PriceResponse{}, false
	}
	raw, err := s.redis.Get(ctx, fmt.Sprintf(marketPriceLastKeyFmt, symbol)).Bytes()
	if err != nil || len(raw) == 0 {
		return toss.PriceResponse{}, false
	}
	var cached cachedMarketPrice
	if json.Unmarshal(raw, &cached) != nil || cached.LastPrice == "" {
		return toss.PriceResponse{}, false
	}
	return toss.PriceResponse{
		Symbol:    symbol,
		LastPrice: cached.LastPrice,
		Currency:  cached.Currency,
	}, true
}

func (s *MarketDataStore) cacheLastGoodPrice(ctx context.Context, symbol string, cached cachedMarketPrice) {
	if s.redis == nil || cached.LastPrice == "" {
		return
	}
	payload, err := json.Marshal(cached)
	if err != nil {
		return
	}
	_ = s.redis.Set(ctx, fmt.Sprintf(marketPriceLastKeyFmt, symbol), payload,
		time.Duration(model.MarketDataLastGoodCacheSec)*time.Second).Err()
}

func (s *MarketDataStore) GetPrice(ctx context.Context, symbol string) (float64, bool) {
	prices := s.GetPrices(ctx, []string{symbol})
	p, ok := prices[strings.ToUpper(strings.TrimSpace(symbol))]
	if !ok {
		return 0, false
	}
	v := parseDecimal(p.LastPrice)
	return v, v > 0
}

func (s *MarketDataStore) RefreshPrices(ctx context.Context, symbols []string) {
	s.mu.RLock()
	if time.Now().Before(s.priceBackoff) {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	if s.toss == nil || !s.toss.Enabled() || len(symbols) == 0 {
		return
	}

	seen := map[string]struct{}{}
	var batch []string
	for _, sym := range symbols {
		up := strings.ToUpper(strings.TrimSpace(sym))
		if up == "" {
			continue
		}
		if _, ok := seen[up]; ok {
			continue
		}
		seen[up] = struct{}{}
		batch = append(batch, up)
	}
	if len(batch) == 0 {
		return
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for i := 0; i < len(batch); i += 200 {
		end := min(i+200, len(batch))
		prices, err := s.toss.GetPrices(refreshCtx, batch[i:end])
		if err != nil {
			if isRateLimitError(err) {
				s.mu.Lock()
				s.priceBackoff = time.Now().Add(marketRateLimitBackoff)
				s.mu.Unlock()
				log.Printf("market price poller: rate limited, backing off")
			}
			return
		}
		now := time.Now()
		for _, p := range prices {
			sym := strings.ToUpper(strings.TrimSpace(p.Symbol))
			if sym == "" {
				continue
			}
			cached := cachedMarketPrice{
				LastPrice: p.LastPrice,
				Currency:  p.Currency,
				UpdatedAt: now,
			}
			if s.redis != nil {
				if payload, err := json.Marshal(cached); err == nil {
					key := fmt.Sprintf(marketPriceKeyFmt, sym)
					_ = s.redis.Set(refreshCtx, key, payload, marketPriceTTL).Err()
					s.cacheLastGoodPrice(refreshCtx, sym, cached)
				}
			}
			if s.quoteTerminal != nil {
				price := parseDecimal(p.LastPrice)
				if price > 0 {
					cur := model.CurrencyKRW
					if p.Currency == toss.CurrencyUSD {
						cur = model.CurrencyUSD
					}
					_ = s.quoteTerminal.UpsertLastPrice(refreshCtx, sym, price, cur)
				}
			}
		}
	}
}

func (s *MarketDataStore) GetEarningsForSymbols(ctx context.Context, symbols []string) []model.BriefingEvent {
	if s.redis == nil || len(symbols) == 0 {
		return nil
	}
	raw, err := s.redis.Get(ctx, marketEarningsKey).Bytes()
	if err != nil || len(raw) == 0 {
		return nil
	}
	var all []model.BriefingEvent
	if json.Unmarshal(raw, &all) != nil {
		return nil
	}
	held := map[string]struct{}{}
	for _, sym := range symbols {
		held[strings.ToUpper(strings.TrimSpace(sym))] = struct{}{}
	}
	var out []model.BriefingEvent
	for _, ev := range all {
		if _, ok := held[strings.ToUpper(ev.Symbol)]; ok {
			out = append(out, ev)
		}
	}
	return out
}

func (s *MarketDataStore) EnsureEarningsForSymbols(ctx context.Context, symbols []string) []model.BriefingEvent {
	out := s.GetEarningsForSymbols(ctx, symbols)
	if s.earningsCacheReady(ctx) {
		return out
	}
	s.refreshEarnings(ctx)
	return s.GetEarningsForSymbols(ctx, symbols)
}

func (s *MarketDataStore) earningsCacheReady(ctx context.Context) bool {
	if s.redis == nil {
		return false
	}
	n, err := s.redis.Exists(ctx, marketEarningsKey).Result()
	return err == nil && n > 0
}

func (s *MarketDataStore) refreshEarnings(ctx context.Context) {
	if s.earnings == nil || !s.earnings.cfg.Enabled() {
		return
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	all, err := s.earnings.FetchCalendar(refreshCtx)
	if err != nil {
		if isRateLimitError(err) {
			log.Printf("earnings poller: rate limited")
		}
		return
	}
	if s.redis == nil {
		return
	}
	payload, err := json.Marshal(all)
	if err != nil {
		return
	}
	_ = s.redis.Set(refreshCtx, marketEarningsKey, payload, marketEarningsTTL).Err()
}

func (s *MarketDataStore) GetStockWarnings(ctx context.Context, symbol string) ([]toss.StockWarning, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" || s.redis == nil {
		return nil, false
	}
	key := fmt.Sprintf(marketWarningsKeyFmt, sym)
	raw, err := s.redis.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	var out []toss.StockWarning
	if json.Unmarshal(raw, &out) != nil {
		return nil, false
	}
	return out, true
}

func (s *MarketDataStore) EnsureStockWarnings(ctx context.Context, symbol string) ([]toss.StockWarning, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil, false
	}
	if warnings, ok := s.GetStockWarnings(ctx, sym); ok {
		return warnings, true
	}
	s.refreshStockWarning(ctx, sym)
	return s.GetStockWarnings(ctx, sym)
}

func (s *MarketDataStore) refreshStockWarning(ctx context.Context, symbol string) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" || s.toss == nil || !s.toss.Enabled() {
		return
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	warnings, err := s.toss.GetStockWarnings(refreshCtx, sym)
	if err != nil {
		return
	}
	if s.redis == nil {
		return
	}
	payload, err := json.Marshal(warnings)
	if err != nil {
		return
	}
	key := fmt.Sprintf(marketWarningsKeyFmt, sym)
	_ = s.redis.Set(refreshCtx, key, payload, marketWarningsTTL).Err()
}

func (s *MarketDataStore) GetTodayDisclosures(ctx context.Context, symbols []string) []model.BriefingEvent {
	if len(symbols) == 0 {
		return nil
	}
	dateKey := time.Now().Format("20060102")
	var out []model.BriefingEvent
	for _, sym := range symbols {
		sym = strings.TrimSpace(sym)
		if sym == "" {
			continue
		}
		if s.redis == nil {
			continue
		}
		key := fmt.Sprintf(marketDisclosuresKeyFmt, sym, dateKey)
		raw, err := s.redis.Get(ctx, key).Bytes()
		if err != nil || len(raw) == 0 {
			continue
		}
		var events []model.BriefingEvent
		if json.Unmarshal(raw, &events) != nil {
			continue
		}
		out = append(out, events...)
	}
	return out
}

func (s *MarketDataStore) EnsureTodayDisclosures(ctx context.Context, symbols []string) []model.BriefingEvent {
	dateKey := time.Now().Format("20060102")
	for _, sym := range domesticSymbolsFromList(symbols) {
		if s.disclosureCacheReady(ctx, sym, dateKey) {
			continue
		}
		s.refreshDisclosure(ctx, sym)
	}
	return s.GetTodayDisclosures(ctx, symbols)
}

func (s *MarketDataStore) disclosureCacheReady(ctx context.Context, symbol, dateKey string) bool {
	if s.redis == nil {
		return false
	}
	key := fmt.Sprintf(marketDisclosuresKeyFmt, strings.TrimSpace(symbol), dateKey)
	n, err := s.redis.Exists(ctx, key).Result()
	return err == nil && n > 0
}

func (s *MarketDataStore) refreshDisclosure(ctx context.Context, symbol string) {
	sym := strings.TrimSpace(symbol)
	if sym == "" || s.dart == nil || !s.dart.cfg.Enabled() {
		return
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	events, err := s.dart.TodayDisclosures(refreshCtx, []string{sym})
	if err != nil {
		return
	}
	if s.redis == nil {
		return
	}
	if events == nil {
		events = []model.BriefingEvent{}
	}
	payload, err := json.Marshal(events)
	if err != nil {
		return
	}
	dateKey := time.Now().Format("20060102")
	key := fmt.Sprintf(marketDisclosuresKeyFmt, sym, dateKey)
	_ = s.redis.Set(refreshCtx, key, payload, marketDisclosuresTTL).Err()
}

func (s *MarketDataStore) refreshInsightsData(ctx context.Context) {
	symbols := s.AllTrackedSymbols(ctx)
	if len(symbols) == 0 {
		return
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	if s.toss != nil && s.toss.Enabled() {
		for _, sym := range symbols {
			warnings, err := s.toss.GetStockWarnings(refreshCtx, sym)
			if err != nil {
				if isRateLimitError(err) {
					break
				}
				continue
			}
			if s.redis == nil {
				continue
			}
			payload, err := json.Marshal(warnings)
			if err != nil {
				continue
			}
			key := fmt.Sprintf(marketWarningsKeyFmt, strings.ToUpper(sym))
			_ = s.redis.Set(refreshCtx, key, payload, marketWarningsTTL).Err()
		}
	}

	if s.dart != nil && s.dart.cfg.Enabled() {
		dateKey := time.Now().Format("20060102")
		for _, sym := range domesticSymbolsFromList(symbols) {
			events, err := s.dart.TodayDisclosures(refreshCtx, []string{sym})
			if err != nil {
				continue
			}
			if s.redis == nil {
				continue
			}
			payload, err := json.Marshal(events)
			if err != nil {
				continue
			}
			key := fmt.Sprintf(marketDisclosuresKeyFmt, sym, dateKey)
			_ = s.redis.Set(refreshCtx, key, payload, marketDisclosuresTTL).Err()
		}
	}
}

func domesticSymbolsFromList(symbols []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, sym := range symbols {
		sym = strings.TrimSpace(sym)
		if len(sym) != 6 {
			continue
		}
		if _, ok := seen[sym]; ok {
			continue
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	return out
}
