package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

const (
	referencePriceKeyFmt = "market:refprice:v3:%s"
	referencePriceTTL    = 26 * time.Hour
)

type cachedReferencePrice struct {
	BasePrice float64   `json:"basePrice"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func cacheReferencePrices(ctx context.Context, rdb *redis.Client, items []toss.RankingItem) {
	if rdb == nil || len(items) == 0 {
		return
	}
	for _, item := range items {
		bp := parseDecimal(item.Price.BasePrice)
		if bp > 0 {
			cacheReferencePrice(ctx, rdb, item.Symbol, bp)
		}
	}
}

func cacheReferencePrice(ctx context.Context, rdb *redis.Client, symbol string, basePrice float64) {
	if rdb == nil || basePrice <= 0 {
		return
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return
	}
	payload, err := json.Marshal(cachedReferencePrice{
		BasePrice: basePrice,
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return
	}
	key := fmt.Sprintf(referencePriceKeyFmt, sym)
	_ = rdb.Set(ctx, key, payload, referencePriceTTL).Err()
}

func loadReferencePrice(ctx context.Context, rdb *redis.Client, symbol string) (float64, bool) {
	if rdb == nil {
		return 0, false
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, false
	}
	raw, err := rdb.Get(ctx, fmt.Sprintf(referencePriceKeyFmt, sym)).Bytes()
	if err != nil || len(raw) == 0 {
		return 0, false
	}
	var cached cachedReferencePrice
	if json.Unmarshal(raw, &cached) != nil || cached.BasePrice <= 0 {
		return 0, false
	}
	return cached.BasePrice, true
}

const (
	// 키에 거래일이 들어간다 — 날이 바뀌면 자동으로 빗나간다(prevCloseFromDailyBars 헤더).
	prevCloseCacheKeyFmt = "market:prevclose:v1:%s:%s"
	prevCloseCacheTTL    = 26 * time.Hour
)

type cachedPrevClose struct {
	// Found가 false면 「그날 기준 이 종목엔 쓸 만한 직전 종가가 없다」는 뜻이다.
	// 값 0과 구분해야 거래정지 종목이 매번 DB를 다시 읽지 않는다.
	Found     bool    `json:"found"`
	PrevClose float64 `json:"prevClose"`
}

func loadPrevCloseCache(ctx context.Context, rdb *redis.Client, symbol, day string) (float64, bool, bool) {
	if rdb == nil || day == "" {
		return 0, false, false
	}
	raw, err := rdb.Get(ctx, fmt.Sprintf(prevCloseCacheKeyFmt, symbol, day)).Bytes()
	if err != nil || len(raw) == 0 {
		return 0, false, false
	}
	var cached cachedPrevClose
	if json.Unmarshal(raw, &cached) != nil {
		return 0, false, false
	}
	return cached.PrevClose, cached.Found, true
}

func storePrevCloseCache(ctx context.Context, rdb *redis.Client, symbol, day string, value float64, found bool) {
	if rdb == nil || day == "" {
		return
	}
	payload, err := json.Marshal(cachedPrevClose{Found: found, PrevClose: value})
	if err != nil {
		return
	}
	_ = rdb.Set(ctx, fmt.Sprintf(prevCloseCacheKeyFmt, symbol, day), payload, prevCloseCacheTTL).Err()
}

func (s *QuotesService) resolvePrevClose(ctx context.Context, symbol string) (float64, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, false
	}

	/*
		**저장된 일봉의 「오늘이 아닌 가장 최근 종가」가 먼저다.**

		랭킹 basePrice는 「그 세션의 기준가」라, 정규장이 끝나면 **오늘 종가로
		갱신된다.** 장중에는 그것이 곧 전일 종가라 맞았지만, 마감 후에는 오늘
		종가가 기준이 되어 등락률이 하루치가 아니라 시간외 변동분만 남았다
		(실측 2026-09-01 삼성전자: 기준가 260,000 = 그날 정규장 종가).

		저장된 일봉은 그런 성질이 없다 — `prevCloseFromDailyBars`가 오늘 봉을
		건너뛰므로 장중이든 마감 후든 **언제나 직전 거래일 종가**다. 그래서
		기준이 하루 종일 움직이지 않고, 마감 순간에 값이 튀지 않는다.

		시계열이 낡은 종목(신규 상장·거래정지)은 여기서 빈손으로 나가고 아래
		기존 경로가 받는다.
	*/
	if v, ok := s.prevCloseFromDailyBars(ctx, sym); ok && v > 0 {
		return v, true
	}

	// Rankings basePrice (세션 기준가). 일봉이 없는 종목의 폴백이다.
	if v, ok := s.lookupReferencePrice(ctx, sym); ok {
		cacheReferencePrice(ctx, s.redis, sym, v)
		return v, true
	}

	/*
		**목록에서는 업스트림까지 내려가지 않는다.**

		`fetchPrevClose`는 전 거래일 정규장 구간의 **1분봉을 페이지네이션으로 전부**
		받아 마지막 종가를 고른다 — 종목당 `/candles` 여러 번이다. 상세 화면 한 종목에는
		맞는 값이지만, 목록은 항목 수만큼 이 경로를 타므로 스무 줄짜리 랭킹 한 번이
		캔들 예산을 통째로 태운다(실측: 랭킹에 실시간 등락률을 얹은 직후 API 로그가
		`/api/v1/candles group=MARKET_DATA_CHART primary exhausted`로 덮였다).

		대신 **이미 저장된 일봉**에서 직전 종가를 읽는다. 인덱스 조회 한 번이고
		업스트림을 부르지 않는다 — 목록이 등락률을 잃지 않으면서 예산도 지킨다.
	*/
	if listMode(ctx) {
		return s.prevCloseFromDailyBars(ctx, sym)
	}

	if s.toss != nil && s.toss.Enabled() {
		assetType := string(s.quoteAssetType(ctx, sym))
		var cals marketCalendars
		if s.marketStore != nil {
			cals, _ = s.marketStore.EnsureCalendars(ctx)
		}
		if v, err := fetchPrevClose(ctx, s.toss, sym, assetType, cals); err == nil && v > 0 {
			cacheReferencePrice(ctx, s.redis, sym, v)
			return v, true
		}
	}
	return 0, false
}

func (s *QuotesService) lookupReferencePrice(ctx context.Context, symbol string) (float64, bool) {
	if v, ok := lookupRankingBasePrice(ctx, s.redis, s.rankingsRepo, symbol); ok {
		return v, true
	}
	if s.redis != nil {
		if v, ok := loadReferencePrice(ctx, s.redis, symbol); ok {
			return v, true
		}
	}
	return 0, false
}

// prevCloseStaleWindow bounds how old the newest stored daily bar may be before
// prevCloseFromDailyBars gives up. Long weekends and holidays run to 5 days.
const prevCloseStaleWindow = 7 * 24 * time.Hour

/*
prevCloseFromDailyBars — 저장된 1일봉에서 **직전 세션 종가**를 읽는다. 캐시 앞단.

# 캐시가 붙어 있는 이유

이 값은 하루에 한 번만 바뀐다. 그런데 등락률을 그리는 목록은 **종목마다** 이걸
불렀고, 그때마다 `candle_bars`를 읽었다.

실측(2026-09-02, `Com_select` 전역 카운터, 10회 중 최솟값 — 전역 카운터라 배경
배치의 버스트가 섞이므로 최솟값이 요청 자체의 비용에 가장 가깝다):

	/rankings 100종목   캐시 없음 → 있음
	  KR(마감)              102 →   2 SELECT
	  US(장중)              658 →  53 SELECT

키에 **그 종목 시장의 오늘 날짜**를 넣으므로 날이 바뀌면 캐시가 스스로 빗나간다.
TTL(26시간)은 키가 쌓이는 것을 막는 뒷단속일 뿐, 정확성은 날짜 키가 책임진다.

**빈손(0,false)도 캐시한다.** 거래정지·신규 상장처럼 시계열이 낡은 종목은 이 함수가
빈손으로 나가는데, 그 판정 역시 하루 동안 안 바뀐다. 캐시하지 않으면 정확히 그
종목들만 폴링마다 DB를 계속 읽는다.
*/
func (s *QuotesService) prevCloseFromDailyBars(ctx context.Context, symbol string) (float64, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return 0, false
	}
	day := s.priceSessionDate(ctx, sym)
	if v, found, ok := loadPrevCloseCache(ctx, s.redis, sym, day); ok {
		return v, found
	}
	value, found := s.prevCloseFromDailyBarsUncached(ctx, sym, day)
	storePrevCloseCache(ctx, s.redis, sym, day, value, found)
	return value, found
}

/*
priceSessionDate — **지금 화면에 서 있는 현재가가 속한 세션의 날짜**(그 종목 시장
표준시 기준, YYYY-MM-DD).

# 왜 「오늘」이 아닌가 (2026-09-04 사용자 지적)

예전 이름은 `marketTodayFor`였고 언제나 그 시장의 오늘 날짜였다. 그래서 **자정과
개장 사이**에 기준가가 한 칸 밀렸다: KST 09-04 00:59, KRX는 닫혀 있고 현재가는
09-03의 종가인데 「오늘」은 09-04이므로 09-03 봉이 「직전 종가」로 뽑혔다 —
현재가와 기준가가 같은 값이 되어 등락률이 0.00%가 됐다(실측 089790 제이티:
상세 5,100원 0.00%, 같은 순간 랭킹 목록은 같은 종가에 −1.92%).

세션으로 물으면 그 구멍이 없다. 장이 돌고 있으면 현재가는 오늘 것이고, 닫혀
있으면 **마지막으로 닫힌 세션** 것이다. 달력을 못 구했을 때만 오늘로 되돌린다.

이 값은 캐시 키의 날짜 성분이기도 하다 — 답이 바뀌는 순간(개장)에 키도 함께
바뀌므로, 마감 중에 담아 둔 기준가가 장중까지 따라오지 않는다.
*/
func (s *QuotesService) priceSessionDate(ctx context.Context, symbol string) string {
	assetType := s.quoteAssetType(ctx, symbol)
	schedule := calendar.ScheduleKRX
	switch assetType {
	case model.AssetUS, model.AssetETF:
		schedule = calendar.ScheduleUS
	}
	loc := calendar.LocationFor(schedule)
	now := time.Now()

	var cals marketCalendars
	if s.marketStore != nil {
		cals, _ = s.marketStore.EnsureCalendars(ctx)
	}
	if marketClockAt(string(assetType), cals, now).Closed() {
		if lastClose, _ := marketSessionEdges(string(assetType), cals, now); !lastClose.IsZero() {
			return lastClose.In(loc).Format("2006-01-02")
		}
	}
	return now.In(loc).Format("2006-01-02")
}

/*
prevCloseFromDailyBarsUncached — 실제로 봉을 읽는 쪽.

목록 경로의 기준가 대안이다(`resolvePrevClose`의 listMode 분기). 업스트림을 부르지
않고 인덱스 조회 한 번으로 끝난다.

**현재가가 속한 세션의 봉부터 건너뛴다** — 그 날짜가 `sessionDate`다
(`priceSessionDate` 참조). 미국 애프터마켓처럼 정규장이 끝나고 그날 봉이 이미
확정된 구간에서는 그 봉이 「직전」이 아니라 「오늘」이고, 자정을 넘긴 마감
구간에서는 어제 봉이 그 자리다. 그걸 기준가로 쓰면 등락률이 시간외 변동분만
남거나(전자) 0.00%가 된다(후자).

**낡은 시계열은 돌려주지 않는다.** 이 함수가 `resolvePrevClose`의 1순위가 된 뒤로는
거래정지·신규상장처럼 봉이 오래 끊긴 종목에서 몇 달 전 종가가 기준으로 올라올 수
있다. 가장 최근 봉이 `prevCloseStaleWindow`보다 오래됐으면 빈손으로 나가고, 랭킹
기준가가 그 자리를 받는다. 연휴를 감안해 하루이틀이 아니라 한 주로 잡았다.
*/
func (s *QuotesService) prevCloseFromDailyBarsUncached(
	ctx context.Context,
	symbol string,
	sessionDate string,
) (float64, bool) {
	if s == nil || s.candles == nil || !s.candles.HasTable(ctx) {
		return 0, false
	}
	points, err := s.candles.DailyCloseSeries(ctx, symbol, 3)
	if err != nil || len(points) == 0 {
		return 0, false
	}

	schedule := calendar.ScheduleKRX
	switch s.quoteAssetType(ctx, symbol) {
	case model.AssetUS, model.AssetETF:
		schedule = calendar.ScheduleUS
	}
	now := time.Now().In(calendar.LocationFor(schedule))
	if sessionDate == "" {
		sessionDate = now.Format("2006-01-02")
	}

	// 시계열이 끊긴 종목은 여기서 빈손으로 나간다(위 주석 참조).
	if newest, err := time.ParseInLocation("2006-01-02", points[len(points)-1].Date, now.Location()); err == nil {
		if now.Sub(newest) > prevCloseStaleWindow {
			return 0, false
		}
	}

	// DailyCloseSeries는 오름차순이라 뒤에서부터 본다.
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Date >= sessionDate || points[i].Close <= 0 {
			continue
		}
		return points[i].Close, true
	}
	return 0, false
}
