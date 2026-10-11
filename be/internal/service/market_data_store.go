package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

const (
	marketPriceKeyFmt   = "market:price:v1:%s"
	marketPriceTTL      = 20 * time.Second
	marketCalendarKRKey = "market:calendar:kr:v1"
	marketCalendarUSKey = "market:calendar:us:v1"
	marketCalendarTTL   = 26 * time.Hour
	marketCalendarPoll  = 24 * time.Hour
	/*
		marketCalendarRetryAfterStale — 달력이 「낡아 보일 때」의 재시도 간격.

		10분은 세션 경계를 늦지 않게 따라가면서(개장·마감은 분 단위로 정해져 있고
		달력은 하루 단위로 바뀐다) 휴장일에 상류를 두드리지 않는 값이다.
		자세한 배경은 MarketDataStore.calAttemptedAt 머리글.
	*/
	marketCalendarRetryAfterStale = 10 * time.Minute
	marketEarningsKey             = "market:earnings:calendar:v1"
	marketEarningsTTL             = 1 * time.Hour
	marketEarningsPoll            = 1 * time.Hour
	marketWarningsKeyFmt          = "market:warnings:v1:%s"
	marketWarningsTTL             = 6 * time.Hour
	marketDisclosuresKeyFmt       = "market:disclosures:v1:%s:%s"
	marketDisclosuresTTL          = 1 * time.Hour
	marketInsightsDataPoll        = 15 * time.Minute
	marketRateLimitBackoff        = 60 * time.Second
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
	quoteTerminal *repository.QuoteTerminalRepo
	// disclosures — 공시 표. 스탬프·리포트가 발행 시점 스냅샷을 굳힐 때 읽는다.
	disclosures *repository.DisclosureRepo

	mu           sync.RWMutex
	calendars    marketCalendars
	calendarsAt  time.Time
	priceBackoff time.Time
	calBackoff   time.Time
	/*
		calAttemptedAt — 달력을 **시도한** 시각(성공 여부와 무관).

		`calendarsAt`(성공 시각)만으로는 재조회를 못 막는 경우가 있다.
		`marketCalendarsDateStale`이 「달력의 Today.Date가 오늘과 다르면 낡았다」로
		판정하는데, **휴장일에는 그것이 영원히 참**이다 — 토스의 KR 달력은 토요일에
		오늘 날짜를 담지 않는다. 그래서 그 판정이 켜진 동안에는 `EnsureCalendars`
		호출마다 상류 호출 두 개가 나갔다.

		실측(2026-09-12 토요일 00:16 KST): `/market-calendar/KR` 3회 + `/US` 4회가
		**분당** 나가고 있었다(하루 약 10,000회). 그 자리를 늘린 것은 같은 날 넣은
		세션 판정 두 곳(환율 주기·순위 마감 스킵)이고, 판정 자체는 옳았지만
		**게이트가 낡음에 대해 재시도 간격을 두지 않았다.**

		이 값이 그 간격이다 — 낡아 보여도 `marketCalendarRetryAfterStale` 안에는
		다시 묻지 않는다. 낡은 달력으로 판정하는 비용은 「휴장일을 한 번 더 휴장일로
		읽는 것」이고, 반대쪽 비용은 분당 일곱 번의 상류 호출이다.
	*/
	calAttemptedAt time.Time
	// calRefreshing — 달력 재조회 단일 비행. 없으면 hot path 동시 호출이
	// 그대로 상류로 나간다 (refreshCalendars 헤더 참조).
	calRefreshing bool
	// lastWrittenPrices — 심볼별로 **DB에 마지막으로 쓴** 가격. 값이 그대로면
	// 다시 쓰지 않는다(RefreshPrices의 주석 참조). 캐시가 아니라 쓰기 중복 제거용
	// 기록이라, 읽기 경로는 이 맵을 보지 않는다.
	lastWrittenPrices map[string]lastWrittenPrice

	// badPriceSymbols — 상류가 `/prices` **묶음 전체를 400으로 거절**하게 만드는
	// 심볼과 그 격리 만료 시각. 값이 아니라 「보내면 안 되는 것」의 목록이다.
	// 왜 필요한지는 refreshPricesNow 주석 참조.
	badPriceSymbols map[string]time.Time

	activeMu sync.Mutex
	active   map[string]time.Time

	// priceRefresh — 같은 심볼 집합에 대한 동시 상류 갱신을 하나로 합친다.
	// 없으면 캐시가 식은 목록을 여러 사용자가 동시에 열 때 Toss 호출이
	// 사용자 수만큼 나간다 (RefreshPrices 헤더 참조).
	priceRefresh *quoteRefreshCoordinator

	/*
		listPrices — 목록 화면 SSE로 나갈 가격 방송(없을 수도 있다).

		여기 매다는 이유는 `cacheMarketPrice`가 **가격이 캐시에 써지는 유일한
		길목**이기 때문이다. 배치 폴러도 웹소켓 체결도 그 함수를 지난다. 발행을
		호출부마다 흩어 놓으면 다음에 세 번째 경로가 생겼을 때 조용히 빠진다.
	*/
	listPrices *listPriceHub

	/*
		lastPublished — 심볼별로 **목록 SSE에 마지막으로 내보낸** 가격.

		`lastWrittenPrices`(DB 쓰기 중복 제거)와 목적이 다르다. 저쪽은 쓰기를 아끼고
		이쪽은 **화면의 플래시**를 지킨다 — 값이 그대로인데 이벤트가 나가면 셀이
		깜빡여서 없는 체결을 있는 것처럼 말한다(publishListPrice 주석 참조).

		`sync.Map`인 이유: 발행은 폴러 고루틴들과 웹소켓 플러시가 **동시에** 하고,
		키 집합은 추적 종목이라 거의 고정이다. 그 모양(쓰는 키가 정해져 있고 경합이
		잦음)에 뮤텍스 하나를 걸면 2초 틱마다 수백 심볼이 한 자리에서 줄을 선다.
	*/
	lastPublished sync.Map
}

// AttachListPriceHub wires the list-price broadcast. Optional: with no hub the
// store behaves exactly as before, so tests and workers need not set one.
func (s *MarketDataStore) AttachListPriceHub(hub *listPriceHub) {
	if s == nil {
		return
	}
	s.listPrices = hub
}

type lastWrittenPrice struct {
	price    float64
	currency model.QuoteCurrency
	at       time.Time
}

/*
lastPriceRewriteInterval — 값이 그대로여도 이 주기마다 한 번은 다시 쓴다.

**자가 치유를 잃지 않기 위한 장치다.** 값 비교로 쓰기를 거르면 메모리의 기록과
DB가 어긋났을 때 스스로 복구하지 못한다 — 예전 코드는 1초마다 덮어썼기 때문에
어떤 이유로 행이 틀어져도 다음 틱에 맞춰졌다. 그 성질까지 버릴 이유는 없다.

10분이면 어긋난 상태가 남는 시간의 상한이 10분이고, 비용은 심볼 3,733개 기준
10분에 10문장(400행 청크) — 초당 0.017문장이라 사실상 0이다.
*/
const lastPriceRewriteInterval = 10 * time.Minute

func NewMarketDataStore(
	tossClient *toss.Client,
	redisClient *redis.Client,
	earnings *EarningsClient,
	dart *DartClient,
	quoteTerminal *repository.QuoteTerminalRepo,
) *MarketDataStore {
	return &MarketDataStore{
		toss:          tossClient,
		redis:         redisClient,
		earnings:      earnings,
		dart:          dart,
		quoteTerminal: quoteTerminal,
		active:        make(map[string]time.Time),
		priceRefresh:  newQuoteRefreshCoordinator(),
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

/*
WarmPriceSymbols — 가격을 따뜻하게 유지할 종목. **최근에 실제로 조회된 것만.**

# 관심·보유를 뺐다

예전 이름은 `AllTrackedSymbols`였고 **전 사용자의 보유 ∪ 관심**을 통째로 넣었다.
그래서 아무도 접속하지 않아도 그 종목 전부가 1초마다 갱신됐다.

그 값을 쓰는 곳은 **사용자가 그 화면을 열었을 때뿐**이고(이 함수의 소비자는
`refreshPinned` 하나다), 화면은 열리는 순간 어차피 새로 받아 1초 주기로 갱신한다.
즉 아무도 안 보는 동안의 갱신이 사는 것은 **첫 조회 한 번의 지연**뿐이었다.

비용은 그 반대로 컸다. 실측(2026-09-02): 사용자 4,652명의 관심·보유 distinct
종목이 **209개**로, pinned 305개 중 대부분이 이것이었다. 그리고 이 축은 사용자가
늘면 그대로 상류 호출로 이어진다 — 목록 화면과 달리 **캐시로 흡수되지 않는다.**

콜드 오픈 비용은 `RefreshPrices`의 단일 비행이 받아 준다: 동시에 몇 명이 열든
상류 호출은 1회다(그 함수 헤더의 실측 — 동시 30명 → 2회).

이름을 바꾼 이유: 「전부 추적한다」는 옛 이름이 지금 하는 일과 반대다. 이름과
실제가 어긋난 채로 두면 다음 사람이 그 이름을 믿고 잘못된 판단을 한다.
*/
func (s *MarketDataStore) WarmPriceSymbols() []string {
	seen := map[string]struct{}{}
	var out []string
	for _, sym := range s.activeSymbols() {
		up := strings.ToUpper(strings.TrimSpace(sym))
		if up == "" {
			continue
		}
		if _, ok := seen[up]; ok {
			continue
		}
		seen[up] = struct{}{}
		out = append(out, up)
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

/*
refreshCalendars — 장 운영 달력을 상류에서 다시 받는다.

**단일 비행이 필수다.** 이 함수는 24시간 티커만 부르는 게 아니라
`EnsureCalendars`를 통해 **요청 hot path에서도** 불린다 — `fetchQuoteStats`가
종목마다 부르고(`quotes_enrich.go`), 지수·수익률·기준가 계산도 각각 부른다.

전에는 backoff 검사만 있고 in-flight 가드가 없었다. RLock으로 검사하고 놓은 뒤
본문에 들어가므로, **동시에 통과한 goroutine 수만큼 상류 호출이 나갔다.**
그 결과 `MARKET_INFO` 그룹이 소진돼 SECONDARY로 failover하는 로그가
계속 찍혔다(실측 2026-08-24):

	toss api failover: GET /api/v1/market-calendar/KR group=MARKET_INFO primary exhausted
	market calendar poller: rate limited, backing off

달력은 **하루에 한 번 바뀌는 값**이다. 실패하면 needRefresh가 계속 true라
다음 호출자가 또 시도하는 구조여서, 한 번 막히면 스스로 증폭됐다.

가드를 걸면 재시도는 남되 **동시에 하나만** 나간다.
*/
func (s *MarketDataStore) refreshCalendars(ctx context.Context) {
	s.mu.Lock()
	if time.Now().Before(s.calBackoff) || s.calRefreshing {
		s.mu.Unlock()
		return
	}
	if !s.calendarsNeedRefreshLocked(time.Now()) {
		s.mu.Unlock()
		return
	}
	s.calRefreshing = true
	// **시도했다는 사실을 먼저 남긴다.** 성공 여부와 무관하게 이 시각이 재시도
	// 간격의 기준이다 — 실패만 반복하는 동안에도 간격이 지켜져야 한다.
	s.calAttemptedAt = time.Now()
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.calRefreshing = false
		s.mu.Unlock()
	}()

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
		// 한 번도 못 받았어도 시도 간격은 지킨다 — 상류가 죽어 있는 동안
		// 호출마다 재시도하면 그 자체가 부하가 된다.
		return s.calAttemptedAt.IsZero() ||
			now.Sub(s.calAttemptedAt) >= marketCalendarRetryAfterStale
	}
	if now.Sub(s.calendarsAt) >= marketCalendarPoll {
		return true
	}
	if !marketCalendarsDateStale(s.calendars, now) {
		return false
	}
	// 낡아 보인다 — 그런데 휴장일에는 그것이 영원히 참이다(calAttemptedAt 머리글).
	// 재시도 간격을 둔다.
	return s.calAttemptedAt.IsZero() ||
		now.Sub(s.calAttemptedAt) >= marketCalendarRetryAfterStale
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
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	redisCacheSetEnvelope(ctx, s.redis, fmt.Sprintf(marketPriceKeyFmt, sym), cached, source, marketPriceTTL)
	s.publishListPrice(sym, cached)
}

/*
publishListPrice — 방금 캐시에 쓴 가격을 목록 SSE로 흘려보낸다.

**구독자를 먼저 묻는다.** 이 함수는 추적 종목 전체에 대해 2초마다 불리는데(실측
pinned 수백 종목), 그중 목록 화면이 실제로 보고 있는 것은 몇 개뿐이다. 아무도 안
보는 종목의 이벤트를 만들어 빈 맵에 던지는 것은 파싱·할당만 남는 일이다.

값이 안 풀리면 조용히 지나간다 — 가격 캐시 쓰기 자체는 이미 끝났고, 이 방송은
그 위에 얹은 것이다. 여기서 실패한다고 캐시가 덜 써지면 안 된다.
*/
func (s *MarketDataStore) publishListPrice(symbol string, cached cachedMarketPrice) {
	if s.listPrices == nil || symbol == "" {
		return
	}
	if !s.listPrices.HasSubscribers(symbol) {
		return
	}
	price := parseDecimal(cached.LastPrice)
	if price <= 0 {
		return
	}
	/*
		**값이 그대로면 보내지 않는다** (2026-09-04 사용자 지적 3).

		이 함수는 가격이 캐시에 써질 때마다 불린다 — 즉 **폴러의 매 틱마다**, 값이
		움직였는지와 무관하게. 화면은 그 이벤트 하나하나를 「갱신」으로 세어 셀에
		플래시를 준다. 그래서 아무도 거래하지 않는 종목의 가격이 초마다 파랗고
		빨갛게 깜빡였다 — 사용자에게는 **없는 체결이 있는 것처럼** 보인다.

		중복을 화면에서 거르지 않고 여기서 막는 이유는 둘이다. 하나는 진짜 방송이
		한 번뿐이라 **모든 구독자에게 한꺼번에** 고쳐진다는 것. 다른 하나는 이것이
		네트워크로 나가는 프레임이라는 것 — 마감 직전처럼 값이 굳은 구간에서는
		이 한 줄이 이벤트의 대부분을 지운다.

		「보낸 값」은 **캐시가 아니라 여기**서 센다. 캐시를 되읽으면 방금 쓴 값이라
		언제나 같고, 그러면 아무것도 못 거른다. 심볼 수만큼의 float64라 추적 종목이
		수백이어도 몇 KB다.

		**구독자가 없는 동안의 변화는 여기 안 남는다** — 위의 `HasSubscribers`가 먼저
		끊기 때문이다(그 가드가 있는 이유는 추적 종목 대부분을 아무도 안 보기
		때문이고, 그걸 없애면 이 절약이 사라진다). 그래서 새 구독자의 첫 틱은 스냅샷과
		**같은 값**을 한 번 더 받을 수 있다. 화면 쪽 `applyListPriceTick`이 가격이
		같은 틱으로는 플래시를 세지 않으므로 그 한 번은 보이지 않는다.
	*/
	if last, seen := s.lastPublished.Load(symbol); seen {
		if v, okCast := last.(float64); okCast && v == price {
			return
		}
	}
	s.lastPublished.Store(symbol, price)
	s.listPrices.Publish(model.ListPriceEvent{
		Type:      "list_price",
		Symbol:    symbol,
		Price:     price,
		Currency:  string(modelCurrency(cached.Currency)),
		UpdatedAt: cached.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
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

/*
GetPriceAt — 배치 가격 캐시의 값과 **그 값이 써진 시각**.

`GetPrice`가 시각을 안 돌려준 것이 2026-09-04 「목록 $11.36 vs 상세 $11.39」의
원인이었다. 상세 화면은 체결 목록의 마지막 가격을 먼저 보는데, 프리·애프터 세션에는
그 캐시에 **정규장의 마지막 체결**이 남아 있다. 배치 가격 캐시는 그 사이에도 계속
갱신되므로 두 값이 갈리는데, 어느 쪽이 새것인지 **비교할 수단이 없었다** — 캐시
봉투의 시각을 `parseMarketPriceRedis`가 `_ = at`으로 버리고 있었다.

이제 시각을 함께 돌려주므로 `resolveBestLivePrice`가 둘 중 **새것**을 고를 수 있다.
값이 하나 더 필요했던 게 아니라, 이미 있던 두 값의 **순서를 정할 근거**가 없었다.

읽기는 심볼 하나짜리 GET이다 — 목록 경로는 이 함수를 타지 않는다(`listMode`는
`GetPrices`의 MGET 한 번으로 끝난다).
*/
func (s *MarketDataStore) GetPriceAt(ctx context.Context, symbol string) (float64, time.Time, bool) {
	if s.redis == nil {
		return 0, time.Time{}, false
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, time.Time{}, false
	}
	raw, err := s.redis.Get(ctx, fmt.Sprintf(marketPriceKeyFmt, sym)).Result()
	if err != nil || raw == "" {
		return 0, time.Time{}, false
	}
	var cached cachedMarketPrice
	at, _, ok := redisCacheGetEnvelopeFromBytes([]byte(raw), &cached)
	if !ok && json.Unmarshal([]byte(raw), &cached) != nil {
		return 0, time.Time{}, false
	}
	if at.IsZero() {
		at = cached.UpdatedAt
	}
	v := parseDecimal(cached.LastPrice)
	if v <= 0 {
		return 0, time.Time{}, false
	}
	return v, at.UTC(), true
}

/*
RefreshPrices — 주어진 심볼들의 가격을 상류에서 받아 캐시·DB에 채운다.

# 같은 집합에 대한 동시 호출은 하나로 합친다

이 함수는 두 곳에서 불린다: 폴러의 1초 틱(배경)과 **요청 경로**(`EnsurePrices`가
캐시에 없는 심볼을 발견했을 때). 후자가 문제였다 — 캐시가 식은 목록을 여러
사용자가 동시에 열면 **같은 심볼 집합에 대해 각자 Toss를 불렀다.** 사용자 수에
그대로 비례하는 유일한 상류 경로였다.

`doSync`가 그 창을 닫는다: 먼저 온 호출이 실제로 받아 오고, 같은 키의 나머지는
그 결과를 기다렸다가(최대 `quoteRefreshWait`) 캐시에서 읽는다. 키는 **정렬한
심볼 집합의 해시**다 — 같은 목록을 보는 사용자들은 같은 집합을 만들기 때문에
바로 이 경우가 합쳐진다. 집합이 다르면 키도 달라 서로 막지 않는다.

폴러의 틱은 추적 종목 전체라 키가 달라, 사용자 요청과 서로를 기다리지 않는다.
*/
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

	if s.priceRefresh != nil {
		s.priceRefresh.doSync(ctx, priceRefreshKey(batch), func(runCtx context.Context) {
			s.refreshPricesNow(runCtx, batch)
		})
		return
	}
	s.refreshPricesNow(ctx, batch)
}

/*
priceRefreshKey — 심볼 집합 하나를 가리키는 안정된 키.

정렬해서 해시하므로 **순서가 달라도 같은 집합이면 같은 키**다. 목록은 순위대로
심볼을 넘기는데 사용자마다 그 순서가 미묘하게 다를 수 있어(랭킹 갱신 타이밍),
정렬하지 않으면 같은 집합이 안 합쳐진다.
*/
func priceRefreshKey(batch []string) string {
	sorted := append([]string(nil), batch...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	return "prices:" + hex.EncodeToString(sum[:8])
}

/*
priceSymbolQuarantine — 상류가 거절한 심볼을 다시 보내 보기까지 기다리는 시간.

영구 제외가 아닌 이유: 상장·재개처럼 나중에 유효해지는 경우가 있다. 반대로 너무
짧으면 격리의 값이 없다 — 만료될 때마다 이등분 탐색을 다시 치른다. 6시간이면
하루 네 번이고, 그 비용은 심볼당 호출 몇 번이다.
*/
const priceSymbolQuarantine = 6 * time.Hour

/*
quarantinePriceSymbol — 이 심볼을 당분간 `/prices` 묶음에서 뺀다.

들어오는 김에 만료된 항목을 치운다. 맵이 커질 일은 없지만(불량 심볼 수가 상한),
치우지 않으면 폐장한 종목이 영영 남는다.
*/
func (s *MarketDataStore) quarantinePriceSymbol(symbol string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.badPriceSymbols == nil {
		s.badPriceSymbols = map[string]time.Time{}
	}
	now := time.Now()
	for sym, until := range s.badPriceSymbols {
		if now.After(until) {
			delete(s.badPriceSymbols, sym)
		}
	}
	s.badPriceSymbols[symbol] = now.Add(priceSymbolQuarantine)
}

// dropQuarantinedSymbols — 격리 중인 심볼을 뺀 목록을 돌려준다. 뺄 것이 없으면
// 받은 슬라이스를 그대로 돌려준다(호출부의 슬라이스는 절대 건드리지 않는다).
func (s *MarketDataStore) dropQuarantinedSymbols(batch []string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.badPriceSymbols) == 0 {
		return batch
	}
	now := time.Now()
	kept := make([]string, 0, len(batch))
	for _, sym := range batch {
		if until, bad := s.badPriceSymbols[sym]; bad && now.Before(until) {
			continue
		}
		kept = append(kept, sym)
	}
	return kept
}

/*
fetchPricesIsolatingBad — 한 묶음을 받아 오되, 상류가 **묶음 전체를 400으로**
거절하면 반씩 갈라 원인 심볼을 찾아 격리하고 **나머지는 살려서** 돌려준다.

이게 없으면 심볼 하나가 같은 묶음의 나머지를 전부 떨어뜨린다. 2026-09-08 실측:
사용자가 해외 종목 QXO를 열자 폴러 묶음(40종목)이 통째로 400을 받아 **국내 39종목의
시세가 그 틱마다 사라졌고**, 재시도가 아니라 같은 요청이 초당 2회씩 계속 나가
Toss `MARKET_DATA` 버킷을 태웠다. 400은 기다려도 낫지 않으므로 원인을 빼는 것이
유일한 출구다.

비용은 불량 심볼 k개에 대해 대략 O(k log n) 호출이고 **한 번만** 치른다 — 그 뒤로는
격리 목록이 앞에서 걸러 호출이 아예 나가지 않는다. 지금의 「영원히 초당 2회 실패」보다
싸다. 레이트리밋을 만나면 즉시 접고 올려보낸다(호출부가 backoff를 잡는다).
*/
func (s *MarketDataStore) fetchPricesIsolatingBad(ctx context.Context, chunk []string) ([]toss.PriceResponse, error) {
	prices, err := s.toss.GetPrices(ctx, chunk)
	if err == nil {
		return prices, nil
	}
	// 404 stock-not-found도 400과 같다 — 상대가 모르는 종목은 기다려도 안 생긴다.
	if !toss.IsInvalidRequestError(err) && !toss.IsStockNotFound(err) {
		return nil, err
	}
	if len(chunk) == 1 {
		s.quarantinePriceSymbol(chunk[0])
		log.Printf("market price poller: %s rejected by upstream, quarantined for %s (%v)", chunk[0], priceSymbolQuarantine, err)
		return nil, nil
	}
	mid := len(chunk) / 2
	left, err := s.fetchPricesIsolatingBad(ctx, chunk[:mid])
	if err != nil {
		return nil, err
	}
	right, err := s.fetchPricesIsolatingBad(ctx, chunk[mid:])
	if err != nil {
		return nil, err
	}
	return append(left, right...), nil
}

func (s *MarketDataStore) refreshPricesNow(ctx context.Context, batch []string) {
	refreshCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 격리된 심볼은 보내지 않는다 — 보내면 이 묶음이 또 통째로 400을 받는다.
	batch = s.dropQuarantinedSymbols(batch)

	for i := 0; i < len(batch); i += 200 {
		end := min(i+200, len(batch))
		prices, err := s.fetchPricesIsolatingBad(refreshCtx, batch[i:end])
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
				// 한도에 걸린 것은 지금 더 불러야 소용이 없다 — 다음 틱에서 이어 받는다.
				return
			}
			// **한 묶음의 실패로 나머지를 버리지 않는다.**
			//
			// 여기서 return하면 뒤쪽 종목은 그 틱에서 가격을 아예 못 받는다.
			// 200종목씩 19묶음인데 두 번째가 실패하면 3,400종목이 통째로 빠졌다 —
			// 실측(2026-08-23) 가격 커버리지가 60초에 146종목(pinned의 4%)이었던
			// 원인 중 하나다. 레이트리밋이 아닌 일시적 오류는 그 묶음만 건너뛴다.
			//
			// 400(요청 거절)은 여기까지 오지 않는다 — fetchPricesIsolatingBad가
			// 원인 심볼을 격리하고 나머지를 살려서 돌려준다.
			log.Printf("market price poller: batch %d-%d failed, continuing: %v", i, end, err)
			continue
		}
		now := time.Now()
		/*
			가격 쓰기는 **모아서 한 번에, 바뀐 것만** 한다.

			예전에는 이 루프가 심볼마다 `UpsertLastPrice`를 한 번씩 불렀다. 폴러가
			1초 주기라 pinned 크기만큼의 문장이 매초 나갔고, 장이 닫혀 가격이 그대로인
			동안에는 **한 행도 바꾸지 않는 쓰기**였다. 실측(2026-08-24, 유휴):
			`Com_insert 18/s` vs `Innodb_rows_inserted 0/s`.

			쓰지 않아도 되는 것을 거르는 기준은 `lastWrittenPrices`에 들고 있는
			직전 기록값이다. 프로세스 메모리라 재기동 후 첫 틱은 전부 쓰지만,
			그 뒤로는 실제로 움직인 종목만 쓴다. 맵 크기는 상장 종목 수(약 1.8만)가
			상한이라 무한히 자라지 않는다.

			주의: 이 맵은 **DB에 무엇을 썼는지**를 기억하는 것이지 캐시가 아니다.
			읽기는 여전히 DB/Redis에서 온다 — 여기서 값을 돌려주지 않는다.
		*/
		var pending []repository.LastPriceRow
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
			if s.quoteTerminal == nil {
				continue
			}
			price := parseDecimal(p.LastPrice)
			if price <= 0 {
				continue
			}
			cur := model.CurrencyKRW
			if p.Currency == toss.CurrencyUSD {
				cur = model.CurrencyUSD
			}
			if !s.lastPriceChanged(sym, price, cur) {
				continue
			}
			pending = append(pending, repository.LastPriceRow{
				Symbol: sym, LastPrice: price, Currency: cur,
			})
		}
		if len(pending) > 0 {
			if err := s.quoteTerminal.UpsertLastPrices(refreshCtx, pending); err != nil {
				// 쓰기에 실패했으면 기록값을 되돌린다 — 그대로 두면 다음 틱이
				// "이미 썼다"고 판단해 그 종목이 영영 저장되지 않는다.
				s.forgetLastPrices(pending)
				log.Printf("market price poller: persist %d prices: %v", len(pending), err)
			}
		}
	}
}

/*
lastPriceChanged — 이 심볼을 지금 DB에 써야 하는지 판단하고, 쓸 것이면 기록한다.

판단과 기록을 한 번의 잠금 안에서 하는 이유: 갈라 두면 두 틱이 겹칠 때 같은
심볼을 두 번 쓰거나(무해) **둘 다 "안 바뀌었다"고 보고 아무도 안 쓰는** 경우가
생긴다. 뒤쪽이 위험하다 — 가격이 영영 저장되지 않는다.
*/
func (s *MarketDataStore) lastPriceChanged(symbol string, price float64, cur model.QuoteCurrency) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastWrittenPrices == nil {
		s.lastWrittenPrices = map[string]lastWrittenPrice{}
	}
	prev, ok := s.lastWrittenPrices[symbol]
	unchanged := ok && prev.price == price && prev.currency == cur
	if unchanged && time.Since(prev.at) < lastPriceRewriteInterval {
		return false
	}
	s.lastWrittenPrices[symbol] = lastWrittenPrice{price: price, currency: cur, at: time.Now()}
	return true
}

// forgetLastPrices drops the recorded values for rows whose write failed.
func (s *MarketDataStore) forgetLastPrices(rows []repository.LastPriceRow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range rows {
		delete(s.lastWrittenPrices, row.Symbol)
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

// SetDisclosures wires the 공시 표 reader.
//
// GetTodayDisclosures가 읽던 Redis 키는 이제 아무도 쓰지 않는다 — 종목별 DART
// 조회를 시장 전체 일별 스윕으로 바꿨기 때문이다. 스탬프·리포트가 발행 시점에
// 그날 공시를 스냅샷으로 굳히는 기능은 그대로 살아 있어야 하므로, 같은 이름의
// 읽기를 표 쪽으로 돌린다.
func (s *MarketDataStore) SetDisclosures(r *repository.DisclosureRepo) { s.disclosures = r }

/*
applyStreamPrice — 웹소켓 체결가를 폴러가 쓰는 것과 **같은 자리**에 넣는다.

읽기 경로(`GetPrices` → Redis MGet → DB 폴백)를 하나도 안 바꾸려고 캐시 키와
DB 컬럼을 그대로 재사용한다. 그래야 구독이 켜지든 꺼지든 화면이 읽는 값의
모양과 출처가 같다.

`lastPriceChanged`를 그대로 통과시키는 것도 같은 이유다 — 값이 안 바뀌었으면
DB에 안 쓴다. 체결은 같은 가격으로도 계속 나므로 이 필터가 없으면 활발한 종목
하나가 초당 수십 건의 무변경 쓰기를 만든다.
*/
func (s *MarketDataStore) applyStreamPrice(ctx context.Context, symbol, price string, currency toss.Currency) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" || price == "" {
		return
	}

	s.cacheMarketPrice(ctx, sym, cachedMarketPrice{
		LastPrice: price,
		Currency:  currency,
		UpdatedAt: time.Now(),
	}, "stream")

	if s.quoteTerminal == nil {
		return
	}
	v := parseDecimal(price)
	if v <= 0 {
		return
	}
	cur := model.CurrencyKRW
	if currency == toss.CurrencyUSD {
		cur = model.CurrencyUSD
	}
	if !s.lastPriceChanged(sym, v, cur) {
		return
	}
	if err := s.quoteTerminal.UpsertLastPrices(ctx, []repository.LastPriceRow{
		{Symbol: sym, LastPrice: v, Currency: cur},
	}); err != nil {
		s.forgetLastPrices([]repository.LastPriceRow{{Symbol: sym, LastPrice: v, Currency: cur}})
	}
}
