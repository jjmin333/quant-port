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
	marketPriceKeyFmt = "market:price:v1:%s"
	marketPriceTTL    = 20 * time.Second
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
	holdings      *repository.UserHoldingsRepo
	watchlist     *repository.WatchlistRepo
	quoteTerminal *repository.QuoteTerminalRepo
	// disclosures — 공시 표. 스탬프·리포트가 발행 시점 스냅샷을 굳힐 때 읽는다.
	disclosures *repository.DisclosureRepo

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
	holdings *repository.UserHoldingsRepo,
	watchlist *repository.WatchlistRepo,
	quoteTerminal *repository.QuoteTerminalRepo,
) *MarketDataStore {
	return &MarketDataStore{
		toss:          tossClient,
		redis:         redisClient,
		earnings:      earnings,
		dart:          dart,
		holdings:      holdings,
		watchlist:     watchlist,
		quoteTerminal: quoteTerminal,
		active:        make(map[string]time.Time),
	}
}

// Run keeps the market calendars fresh.
//
// 예전에는 여기서 실적 캘린더(Finnhub)와 「인사이트 데이터」(종목별 Toss 경고 +
// 종목별 DART 공시)도 주기적으로 받았다. 그 셋은 전부 worker-filings·
// worker-earnings로 옮겼다 — **API 프로세스가 외부를 부르는 배경 작업을 갖는 것**
// 자체가 문제였다. 사용자 요청을 처리하는 프로세스가 남의 사이트 사정으로
// 느려지고, 여러 인스턴스를 띄우면 같은 일을 인스턴스 수만큼 반복한다.
//
// 장 개장/휴장 달력만 남았다. 이건 Toss 호출 한 번이고, 시세 경로가 요청마다
// 즉시 필요로 하는 값이라 표를 거치는 것이 오히려 느리다.
func (s *MarketDataStore) Run(ctx context.Context) {
	s.loadCalendarsFromRedis(ctx)
	s.refreshCalendars(ctx)

	calTicker := time.NewTicker(marketCalendarPoll)
	defer calTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-calTicker.C:
			s.refreshCalendars(ctx)
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

	if s.holdings != nil {
		symbols, err := s.holdings.ListAllSymbols(ctx)
		if err == nil {
			for _, sym := range symbols {
				add(sym)
			}
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
		if p, ok := parseMarketPriceRedis(raw); ok {
			sym := keyToSym[keys[i]]
			p.Symbol = sym
			out[sym] = p
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
			s.cacheMarketPrice(ctx, sym, cachedMarketPrice{
				LastPrice: p.LastPrice,
				Currency:  p.Currency,
				UpdatedAt: time.Now().UTC(),
			}, "db")
		}
	}
	return out
}

func parseMarketPriceRedis(raw string) (toss.PriceResponse, bool) {
	var cached cachedMarketPrice
	if at, _, ok := redisCacheGetEnvelopeFromBytes([]byte(raw), &cached); ok && cached.LastPrice != "" {
		if at.IsZero() && !cached.UpdatedAt.IsZero() {
			at = cached.UpdatedAt
		}
		_ = at
		return toss.PriceResponse{
			Symbol:    "",
			LastPrice: cached.LastPrice,
			Currency:  cached.Currency,
		}, true
	}
	if json.Unmarshal([]byte(raw), &cached) == nil && cached.LastPrice != "" {
		return toss.PriceResponse{
			LastPrice: cached.LastPrice,
			Currency:  cached.Currency,
		}, true
	}
	return toss.PriceResponse{}, false
}

func redisCacheGetEnvelopeFromBytes(raw []byte, dest any) (fetchedAt time.Time, enveloped bool, ok bool) {
	switch d := dest.(type) {
	case *cachedMarketPrice:
		v, at, env, ok := decodeCacheEnvelope[cachedMarketPrice](raw)
		if ok {
			*d = v
			return at, env, true
		}
	}
	return time.Time{}, false, false
}

func (s *MarketDataStore) cacheMarketPrice(ctx context.Context, symbol string, cached cachedMarketPrice, source string) {
	if s.redis == nil || cached.LastPrice == "" {
		return
	}
	if cached.UpdatedAt.IsZero() {
		cached.UpdatedAt = time.Now().UTC()
	}
	redisCacheSetEnvelope(ctx, s.redis, fmt.Sprintf(marketPriceKeyFmt, strings.ToUpper(strings.TrimSpace(symbol))), cached, source, marketPriceTTL)
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
			// Prefer the server's own Retry-After / X-RateLimit-Reset over
			// the fixed marketRateLimitBackoff — observed resets are as
			// short as 1s, so a flat 60s wait stalls prices far longer than
			// the bucket actually needs.
			if wait, ok := tossRateLimitWait(err); ok {
				s.mu.Lock()
				s.priceBackoff = time.Now().Add(wait)
				s.mu.Unlock()
				log.Printf("market price poller: rate limited, backing off %s", wait)
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
			s.cacheMarketPrice(refreshCtx, sym, cached, "toss")
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

func (s *MarketDataStore) GetTodayDisclosures(ctx context.Context, symbols []string) []model.BriefingEvent {
	if s.disclosures != nil && len(symbols) > 0 {
		rows, err := s.disclosures.ListForSymbolsOnDate(ctx, symbols, time.Now().In(model.KST))
		if err == nil {
			out := make([]model.BriefingEvent, 0, len(rows))
			for _, row := range rows {
				out = append(out, model.BriefingEvent{Date: "오늘", Symbol: row.Symbol, Event: row.Title})
			}
			return out
		}
	}
	if len(symbols) == 0 {
		return nil
	}
	dateKey := time.Now().In(model.KST).Format("20060102")
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

// SetDisclosures wires the 공시 표 reader.
//
// GetTodayDisclosures가 읽던 Redis 키는 이제 아무도 쓰지 않는다 — 종목별 DART
// 조회를 시장 전체 일별 스윕으로 바꿨기 때문이다. 스탬프·리포트가 발행 시점에
// 그날 공시를 스냅샷으로 굳히는 기능은 그대로 살아 있어야 하므로, 같은 이름의
// 읽기를 표 쪽으로 돌린다.
func (s *MarketDataStore) SetDisclosures(r *repository.DisclosureRepo) { s.disclosures = r }
