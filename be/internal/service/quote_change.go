package service

import (
	"context"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

/*
quoteChangeUsesRegularClose reports whether today's regular close is already
fixed — the sessions where `regularClose`가 표시할 값으로 의미가 있다.

**등락률의 기준으로는 더 이상 쓰지 않는다**(computeQuoteChange 주석 참조).
남아 있는 용도는 `resolveRegularClose`가 「지금 정규장 종가를 구할 수 있는
세션인가」를 판단하는 것뿐이다.
*/
func quoteChangeUsesRegularClose(session string) bool {
	switch session {
	case "after", "closed":
		return true
	default:
		return false
	}
}

/*
computeQuoteChange — 등락률의 기준은 **언제나 직전 거래일 종가**다.

예전에는 마감·시간외 세션에서 「당일 정규장 종가」를 기준으로 삼았다. 그런데
현재가는 시간외까지 반영한 최종가라, 그 둘을 빼면 나오는 것은 **하루 등락률이
아니라 시간외 등락률**이다. 실측(2026-09-01 마감 후 삼성전자): 최종가 255,000 ·
당일 정규장 종가 260,500 → **−2.11%**로 표시됐지만, 그날 실제 움직임은
전일 258,000 대비 **−1.16%**였다.

게다가 기준이 화면마다 갈렸다 — 목록은 랭킹 기준가(260,000), 상세는 정규장
종가(260,500). 같은 종목의 같은 날 등락률이 두 값으로 보였다.

「시간외까지 마감된 값이 그날의 진짜 종가」라는 결정에 따라, 현재가는 최종가로
두고 기준만 전일 종가로 통일한다. 그러면 장중·시간외·마감이 **같은 기준을
공유**해서 마감 순간에 값이 튀지 않는다.

`regularClose`는 계산에서 빠지되 응답 필드로는 남는다 — 상세 화면이 「정규장
종가」를 따로 보여 준다.
*/
func computeQuoteChange(currentPrice, prevClose float64) (change, changeRate float64) {
	if prevClose <= 0 || currentPrice <= 0 {
		return 0, 0
	}
	change = currentPrice - prevClose
	changeRate = (change / prevClose) * 100
	return change, changeRate
}

/*
quoteAssetType — 이 종목이 국내인가 해외인가 ETF인가.

**캐시가 붙어 있다.** 이 함수는 목록 경로의 거의 모든 갈래에 있어서, 캐시가 없을
때는 100줄짜리 목록 하나가 `stocks`를 수백 번 읽었다 — 근거와 실측은
`quotes_catalog_cache.go` 헤더에 있다.
*/
func (s *QuotesService) quoteAssetType(ctx context.Context, symbol string) model.QuoteAssetType {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if cached, ok := s.assetTypes.get(sym); ok {
		return cached
	}
	resolved, fromCatalog := s.quoteAssetTypeUncached(ctx, symbol)
	// 카탈로그에 행이 없어 기본값으로 떨어진 경우는 캐시하지 않는다 — 아직 안 채워진
	// 종목이 TTL 동안 「국내」로 굳으면 해외 종목의 세션 판정이 그만큼 틀어진다.
	if fromCatalog {
		s.assetTypes.put(sym, resolved)
	}
	return resolved
}

// quoteAssetTypeUncached는 값과 함께 **카탈로그에서 나온 값인지**를 돌려준다.
// 두 번째 값이 false면 아래 기본값(국내)으로 떨어진 것이고, 캐시하면 안 된다.
func (s *QuotesService) quoteAssetTypeUncached(ctx context.Context, symbol string) (model.QuoteAssetType, bool) {
	if s.stocks != nil {
		if stock, err := s.stocks.GetBySymbol(ctx, symbol); err == nil && stock != nil {
			if at := assetTypeFromMarket(stock.Market, stock.SecurityType); at == model.AssetDomestic {
				return model.AssetDomestic, true
			}
			if stock.Currency == model.CurrencyKRW || isDomesticKRSymbol(stock.Symbol) {
				return model.AssetDomestic, true
			}
			if stock.AssetType != "" {
				return stock.AssetType, true
			}
			// 행은 있는데 자산 구분이 비어 있다. 값은 기본값이지만 조회를 되풀이할
			// 이유는 없으므로 캐시한다.
			return model.AssetDomestic, true
		}
	}
	return model.AssetDomestic, false
}

func (s *QuotesService) marketSessionFor(ctx context.Context, symbol string) string {
	assetType := s.quoteAssetType(ctx, symbol)
	var cals marketCalendars
	if s.marketStore != nil {
		cals, _ = s.marketStore.EnsureCalendars(ctx)
	}
	return detectMarketSession(string(assetType), cals, time.Now())
}

func (s *QuotesService) resolveRegularClose(ctx context.Context, symbol string, assetType model.QuoteAssetType, session string) float64 {
	if !quoteChangeUsesRegularClose(session) {
		return 0
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	/*
		**닫혀 있으면 그 세션의 확정 일봉 종가가 정규장 종가다** (2026-09-26 외부 방문자 점검).

		아래 경로들은 「오늘」의 정규장 구간(`regularSessionBounds`)에서 종가를 찾는다. 그래서
		주말·휴장일·평일 개장 전에는 늘 빈손이었고, 화면의 「정규장 종가」 줄이 사라졌다. 국내는
		현재가가 NXT 20:00 최종가라 그 줄이 없으면 종목 화면(SK하이닉스 1,862,000원)과 검색 결과
		요약의 「종가 1,863,000원」(KRX 정규장)이 설명 없이 다른 숫자로 보였다.
	*/
	if session == "closed" {
		loc := kst
		if assetType == model.AssetUS || assetType == model.AssetETF {
			loc = usEastern
		}
		if c, ok := s.closedSessionDailyBar(ctx, sym, s.quoteSessionStart(ctx, sym, loc)); ok && c.Close > 0 {
			return c.Close
		}
	}
	now := time.Now()
	var cals marketCalendars
	if s.marketStore != nil {
		cals, _ = s.marketStore.EnsureCalendars(ctx)
	}

	if s.redis != nil {
		if rc, ok := regularCloseFromRawCandles(ctx, s.redis, sym, string(assetType), cals, now); ok {
			return rc
		}
	}
	if s.candles != nil && s.candles.HasTable(ctx) {
		if rc, ok := s.regularCloseFromDBCandles(ctx, sym, string(assetType), cals, now); ok {
			return rc
		}
	}
	// 목록에서는 업스트림까지 내려가지 않는다.
	//
	// fetchRegularSessionClose는 종목당 `/candles` 호출이고, 마감 세션에서는
	// **모든 국내 종목이 이 분기를 탄다**(quoteChangeUsesRegularClose가
	// "closed"에 true다). 실측(2026-09-01 마감 후): 시세 배치 10종목이
	// 캔들 한도 경고를 6번 냈고, 그 원인이 여기였다 — 캐시·DB에 정규장 종가가
	// 없는 종목마다 한 번씩이다.
	//
	// 목록에서 0을 돌려주면 `computeQuoteChange`가 전일 종가를 기준으로
	// 되돌아간다. 마감 후 목록에서 그건 「오늘 얼마나 움직였나」라 오히려
	// 목록이 기대하는 값이고, 상세 화면은 목록 모드가 아니므로 그대로다.
	if !listMode(ctx) && s.toss != nil && s.toss.Enabled() {
		if rc, ok := fetchRegularSessionClose(ctx, s.toss, sym, string(assetType), cals, now); ok {
			return rc
		}
	}
	return 0
}

func (s *QuotesService) regularCloseFromDBCandles(
	ctx context.Context,
	symbol string,
	assetType string,
	cals marketCalendars,
	now time.Time,
) (float64, bool) {
	regularStart, regularEnd, ok := regularSessionBounds(assetType, cals)
	if !ok || now.Before(regularEnd) {
		return 0, false
	}
	intraday, _, err := s.candles.ListBefore(ctx, symbol, model.Interval1m, "", 480)
	if err != nil || len(intraday) == 0 {
		return 0, false
	}
	var lastClose float64
	var lastTs time.Time
	for _, c := range intraday {
		ts, tsOk := parseCandleTime(c.Time)
		if !tsOk || ts.Before(regularStart) || ts.After(regularEnd) {
			continue
		}
		if c.Close > 0 && (lastTs.IsZero() || !ts.Before(lastTs)) {
			lastClose = c.Close
			lastTs = ts
		}
	}
	return lastClose, lastClose > 0
}

func (s *QuotesService) applyQuoteChange(
	ctx context.Context,
	symbol string,
	currentPrice float64,
	prevClose float64,
	detail *model.QuoteDetail,
) {
	assetType := s.quoteAssetType(ctx, symbol)
	/*
		세션과 그 경계 시각을 **한 번에** 받는다.

		예전에는 `marketSessionFor`가 세션만 답했고, 화면은 마감 중에도 「응답을 받은
		시각」을 찍었다(「오전 12:33:08 기준」). 지금은 닫혀 있으면 **언제 닫혔는지**와
		**언제 다시 여는지**가 같이 나가고, 화면은 그 둘로 시각을 적고 폴링을 멈춘다
		(`market_session_clock.go`).
	*/
	clock := s.MarketClockFor(ctx, string(assetType))
	session := clock.Session
	// 정규장 종가는 **표시용**이다(상세 화면의 「정규장 종가」 줄). 등락률 계산에는
	// 쓰이지 않으므로, 그 줄을 그리지 않는 목록에서는 구하지 않는다 — 이 함수가
	// 종목마다 1분봉 480행을 읽던 경로다.
	regularClose := 0.0
	if !listMode(ctx) {
		regularClose = s.resolveRegularClose(ctx, symbol, assetType, session)
		if regularClose <= 0 && detail.RegularClose > 0 {
			regularClose = detail.RegularClose
		}
	}
	change, changeRate := computeQuoteChange(currentPrice, prevClose)
	detail.MarketSession = session
	detail.SessionClosedAt = clock.ClosedAt
	detail.NextSessionAt = clock.NextOpenAt
	detail.RegularClose = regularClose
	detail.PrevClose = prevClose
	detail.CurrentPrice = currentPrice
	detail.Change = change
	detail.ChangeRate = changeRate
	detail.AssetType = assetType
}

// rehydrateQuoteDetail recomputes live price + reference baseline on every read (guards stale persisted fields).
func (s *QuotesService) rehydrateQuoteDetail(ctx context.Context, detail *model.QuoteDetail) {
	s.rehydrateQuoteDetailWithSources(ctx, detail, nil, nil)
}

// finalizeQuoteDetail applies live price recompute and per-request user flags after any cache hit.
func (s *QuotesService) finalizeQuoteDetail(ctx context.Context, detail *model.QuoteDetail) {
	if detail == nil || strings.TrimSpace(detail.Symbol) == "" {
		return
	}
	detail.AssetType = s.quoteAssetType(ctx, detail.Symbol)
	s.ensurePriceMicrostructure(ctx, detail.Symbol)

	var trades []model.TradeTick
	if cached, ok := s.loadTradesPersisted(ctx, detail.Symbol); ok {
		trades = cached
	}
	var book *model.OrderBook
	if cached, ok := s.loadOrderBookPersisted(ctx, detail.Symbol); ok {
		book = &cached
	}
	s.rehydrateQuoteDetailWithSources(ctx, detail, trades, book)
	s.overlayUserQuoteFlags(ctx, detail)
}

func (s *QuotesService) rehydrateQuoteDetailWithSources(
	ctx context.Context,
	detail *model.QuoteDetail,
	trades []model.TradeTick,
	book *model.OrderBook,
) {
	if detail == nil || strings.TrimSpace(detail.Symbol) == "" {
		return
	}
	detail.AssetType = s.quoteAssetType(ctx, detail.Symbol)
	ref, ok := s.resolvePrevClose(ctx, detail.Symbol)
	if !ok || ref <= 0 {
		return
	}
	fallback := detail.CurrentPrice
	price := s.resolveBestLivePrice(ctx, detail.Symbol, fallback, trades, book)
	if price <= 0 {
		return
	}
	session := s.marketSessionFor(ctx, detail.Symbol)
	if quoteSessionHasLiveMicrostructure(session) && isSamePrice(price, ref) {
		if p, ok := s.directTossPrice(ctx, detail.Symbol); ok && !isSamePrice(p, ref) {
			price = p
		} else if p, ok := lookupRankingLastPrice(ctx, s.redis, s.rankingsRepo, detail.Symbol); ok && !isSamePrice(p, ref) {
			price = p
		} else if fallback > 0 && !isSamePrice(fallback, ref) {
			price = fallback
		}
	}
	if price <= 0 {
		return
	}
	s.applyQuoteChange(ctx, detail.Symbol, price, ref, detail)
	detail.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (s *QuotesService) finalizeTerminalSnapshot(ctx context.Context, snap *model.QuoteTerminalSnapshot) {
	if snap == nil {
		return
	}
	sym := strings.TrimSpace(snap.Detail.Symbol)
	if sym != "" {
		s.ensurePriceMicrostructure(ctx, sym)
		if len(snap.Trades) == 0 {
			if trades, ok := s.loadTradesPersisted(ctx, sym); ok {
				snap.Trades = trades
			}
		}
		if len(snap.OrderBook.Asks) == 0 && len(snap.OrderBook.Bids) == 0 {
			if book, ok := s.loadOrderBookPersisted(ctx, sym); ok {
				snap.OrderBook = book
			}
		}
	}
	s.rehydrateQuoteDetailWithSources(ctx, &snap.Detail, snap.Trades, &snap.OrderBook)
	s.overlayUserQuoteFlags(ctx, &snap.Detail)
}
