package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/toss"
	"github.com/redis/go-redis/v9"
)

type MarketService struct {
	toss             *toss.Client
	redis            *redis.Client
	fx               *ExchangeRateService
	stocks           *repository.StockRepo
	marketStore      *MarketDataStore
	rankingSnapshots *repository.RankingSnapshotRepo
	rankingsRepo     *repository.MarketRankingsRepo
	dataSettings     *MarketDataSettingsStore
	quotes           *QuotesService
	// warnings — 종목 지정 상태 표. 배치가 채우고 여기서는 읽기만 한다.
	warnings *repository.WarningRepo
}

// SetWarnings wires the 종목 경고 reader.
func (s *MarketService) SetWarnings(r *repository.WarningRepo) { s.warnings = r }

func NewMarketService(
	tossClient *toss.Client,
	redisClient *redis.Client,
	fx *ExchangeRateService,
	stockRepo *repository.StockRepo,
	rankingRepo *repository.RankingSnapshotRepo,
	rankingsRepo *repository.MarketRankingsRepo,
) *MarketService {
	return &MarketService{
		toss:             tossClient,
		redis:            redisClient,
		fx:               fx,
		stocks:           stockRepo,
		rankingSnapshots: rankingRepo,
		rankingsRepo:     rankingsRepo,
	}
}

func (s *MarketService) SetMarketStore(store *MarketDataStore) {
	s.marketStore = store
}

func (s *MarketService) SetQuotesService(q *QuotesService) {
	s.quotes = q
}

func (s *MarketService) SetDataSettings(store *MarketDataSettingsStore) {
	s.dataSettings = store
}

func (s *MarketService) requireToss() (*toss.Client, error) {
	if s.toss == nil || !s.toss.Enabled() {
		return nil, fmt.Errorf("toss api is not configured")
	}
	return s.toss, nil
}

func (s *MarketService) ExchangeRate(ctx context.Context, p toss.GetExchangeRateParams) (*toss.ExchangeRateResponse, error) {
	if p.BaseCurrency == "" {
		p.BaseCurrency = toss.CurrencyUSD
	}
	if p.QuoteCurrency == "" {
		p.QuoteCurrency = toss.CurrencyKRW
	}
	if p.BaseCurrency == toss.CurrencyUSD && p.QuoteCurrency == toss.CurrencyKRW && s.fx != nil {
		if resp, ok := s.fx.ToResponse(ctx); ok {
			return resp, nil
		}
		return nil, fmt.Errorf("exchange rate not available yet")
	}
	return nil, fmt.Errorf("unsupported currency pair")
}

func (s *MarketService) KrCalendar(ctx context.Context, date string) (*toss.KrMarketCalendarResponse, error) {
	client, err := s.requireToss()
	if err != nil {
		return nil, err
	}
	return client.GetKrMarketCalendar(ctx, date)
}

func (s *MarketService) UsCalendar(ctx context.Context, date string) (*toss.UsMarketCalendarResponse, error) {
	client, err := s.requireToss()
	if err != nil {
		return nil, err
	}
	return client.GetUsMarketCalendar(ctx, date)
}

func (s *MarketService) Rankings(ctx context.Context, p toss.GetRankingsParams) (*RankingsAPIResult, error) {
	if !isSupportedRankingPollParams(p) {
		return &RankingsAPIResult{Rankings: []RankingAPIItem{}}, nil
	}

	storeParams := p
	storeParams.Count = rankingsPollFetchCount

	stored, ok := s.loadRankingsFromStore(ctx, storeParams)
	if !ok {
		return &RankingsAPIResult{
			MarketClock: s.rankingMarketClock(ctx, p.MarketCountry),
			Rankings:    []RankingAPIItem{},
		}, nil
	}
	result := stored.toAPIResult(rankingsAPIResultCount(p.Count))
	result.MarketClock = s.rankingMarketClock(ctx, p.MarketCountry)
	if s.quotes != nil && len(result.Rankings) > 0 {
		result.applyTossItems(s.quotes.EnrichRankingPrices(ctx, result.tossItems()))
	}
	s.attachRankingNames(ctx, result)
	return result, nil
}

/*
rankingMarketClock — 이 목록이 속한 시장의 세션 시계.

목록의 모든 행이 한 달력을 쓰므로 시장 하나로 물으면 된다(국내 탭 = KRX, 해외
탭 = 미국). ETF도 미국 달력이라 「해외」 하나로 묶인다 — 랭킹의 시장 구분이
`domesticAssetTypes` / `overseasAssetTypes`와 같은 선이다.
*/
func (s *MarketService) rankingMarketClock(ctx context.Context, country toss.MarketCountry) MarketClock {
	if s.quotes == nil {
		return MarketClock{}
	}
	return s.quotes.MarketClockFor(ctx, rankingAssetType(country))
}

// rankingMarketClockCached는 같은 판정을 **상류 호출 없이** 한다 — 주기적으로 도는
// 자리용이다(MarketClockCachedFor 머리글).
func (s *MarketService) rankingMarketClockCached(ctx context.Context, country toss.MarketCountry) MarketClock {
	if s.quotes == nil {
		return MarketClock{}
	}
	return s.quotes.MarketClockCachedFor(ctx, rankingAssetType(country))
}

func rankingAssetType(country toss.MarketCountry) string {
	if country == toss.MarketUS {
		return string(model.AssetUS)
	}
	return string(model.AssetDomestic)
}

/*
MarketClocks — **두 시장의 세션을 한 번에** 답한다 (2026-09-11 사용자 요청 9).

`rankingMarketClock`은 「이 목록의 시장」 하나만 답하므로, 화면이 「지금 어느 쪽이
열려 있나」를 물으려면 두 번 물어야 한다. 그런데 그 질문을 하는 자리(홈 카드의
국내/해외 기본값)는 **데이터를 받기 전에** 답을 알아야 한다 — 어느 쪽을 받을지
정하는 값이기 때문이다.

브라우저에서 다시 계산하지 않는 이유는 `lib/market-session`에 적힌 그대로다:
세션 창은 종목마다 다르고 미국은 서머타임에 창이 밀려, 화면이 판정하면 서버와
어긋나는 두 번째 진실이 생긴다. 달력은 `EnsureCalendars`가 캐시하므로 이 응답에
상류 호출은 없다.
*/
type MarketClocksView struct {
	Domestic MarketClock `json:"domestic"`
	US       MarketClock `json:"us"`
}

func (s *MarketService) MarketClocks(ctx context.Context) MarketClocksView {
	if s == nil || s.quotes == nil {
		return MarketClocksView{}
	}
	return MarketClocksView{
		Domestic: s.quotes.MarketClockFor(ctx, string(model.AssetDomestic)),
		US:       s.quotes.MarketClockFor(ctx, string(model.AssetUS)),
	}
}

/*
attachRankingNames — 카탈로그에서 종목명을 붙인다.

`ListBySymbols` 한 번(심볼 IN 조회)이고 상류 호출은 없다. 이름을 못 찾은 행은
`Name`이 빈 채로 나가고 화면이 심볼로 되돌린다 — 카탈로그에 없는 신규 상장이
목록에서 통째로 사라지는 것보다 낫다.

여기서 조용히 실패해도 목록은 그대로 나간다. 이름은 표시용이고, 없다고 해서
순위·가격·등락률이 틀리지는 않는다.
*/
func (s *MarketService) attachRankingNames(ctx context.Context, result *RankingsAPIResult) {
	if s.stocks == nil || result == nil || len(result.Rankings) == 0 {
		return
	}
	symbols := make([]string, 0, len(result.Rankings))
	for i := range result.Rankings {
		symbols = append(symbols, result.Rankings[i].Symbol)
	}
	rows, err := s.stocks.ListBySymbols(ctx, symbols)
	if err != nil {
		log.Printf("rankings: 종목명 조회 실패 (심볼 %d개): %v", len(symbols), err)
		return
	}
	bySymbol := make(map[string]model.Stock, len(rows))
	for _, st := range rows {
		bySymbol[strings.ToUpper(strings.TrimSpace(st.Symbol))] = st
	}
	for i := range result.Rankings {
		st, ok := bySymbol[strings.ToUpper(strings.TrimSpace(result.Rankings[i].Symbol))]
		if !ok {
			continue
		}
		result.Rankings[i].Name = st.Name
		result.Rankings[i].AssetType = st.AssetType
		result.Rankings[i].LogoURL = st.LogoURL
	}
}

func rankingsAPIResultCount(requested int) int {
	if requested <= 0 || requested > rankingsPollFetchCount {
		return rankingsPollFetchCount
	}
	return requested
}

func (s *MarketService) Stock(ctx context.Context, symbol string) (*toss.StockInfo, error) {
	items, err := s.Stocks(ctx, []string{symbol})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (s *MarketService) Stocks(ctx context.Context, symbols []string) ([]toss.StockInfo, error) {
	order := normalizeStockSymbols(symbols)
	if len(order) == 0 {
		return nil, nil
	}

	dbBySymbol := make(map[string]model.Stock, len(order))
	if s.stocks != nil {
		fromDB, err := s.stocks.ListBySymbols(ctx, order)
		if err != nil {
			return nil, err
		}
		for _, st := range fromDB {
			dbBySymbol[st.Symbol] = st
		}
	}

	needToss := make([]string, 0, len(order))
	for _, sym := range order {
		st, ok := dbBySymbol[sym]
		if !ok || stockNeedsTossReference(st) {
			needToss = append(needToss, sym)
		}
	}

	responseBySymbol := make(map[string]toss.StockInfo, len(order))
	for sym, st := range dbBySymbol {
		if !stockNeedsTossReference(st) {
			responseBySymbol[sym] = catalogStockToTossInfo(st)
		}
	}

	if len(needToss) > 0 {
		client, err := s.requireToss()
		if err != nil {
			if len(responseBySymbol) == 0 {
				return nil, err
			}
			return orderStockInfos(valuesFromMap(responseBySymbol), order), nil
		}

		for i := 0; i < len(needToss); i += 200 {
			end := min(i+200, len(needToss))
			fromToss, err := client.GetStocks(ctx, needToss[i:end])
			if err != nil {
				if len(responseBySymbol) == 0 {
					return nil, fmt.Errorf("toss get stocks: %w", err)
				}
				break
			}
			for _, info := range fromToss {
				sym := normalizeStockSymbols([]string{info.Symbol})[0]
				if sym == "" {
					continue
				}
				existing, hasExisting := dbBySymbol[sym]
				merged := s.persistTossReference(ctx, existing, hasExisting, info)
				dbBySymbol[sym] = merged
				responseBySymbol[sym] = catalogStockToTossInfo(merged)
			}
		}

		for _, sym := range needToss {
			if _, ok := responseBySymbol[sym]; ok {
				continue
			}
			if st, ok := dbBySymbol[sym]; ok {
				responseBySymbol[sym] = catalogStockToTossInfo(st)
			}
		}
	}

	if len(responseBySymbol) == 0 {
		return nil, nil
	}
	return orderStockInfos(valuesFromMap(responseBySymbol), order), nil
}

// StockWarnings reads 지정 상태 from the table the warnings batch fills.
//
// 예전에는 캐시가 비면 여기서 Toss를 직접 불렀다 — 종목 수에 비례해 호출이
// 늘어나는 경로였다. 경고는 하루 단위로 붙고 풀리므로 배치가 **전 종목 매일**
// 갱신한다(reference_ingest.go IngestWarnings).
func (s *MarketService) StockWarnings(ctx context.Context, symbol string) ([]toss.StockWarning, error) {
	if s.warnings == nil {
		return nil, nil
	}
	rows, err := s.warnings.ListForSymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	out := make([]toss.StockWarning, 0, len(rows))
	for _, w := range rows {
		out = append(out, toss.StockWarning{
			WarningType: w.WarningType,
			Exchange:    w.Exchange,
			StartDate:   w.StartDate,
			EndDate:     w.EndDate,
		})
	}
	return out, nil
}

func (s *MarketService) MarketIndicatorPrices(ctx context.Context, symbols []string) ([]toss.MarketIndicatorPriceResponse, error) {
	client, err := s.requireToss()
	if err != nil {
		return nil, err
	}
	return client.GetMarketIndicatorPrices(ctx, symbols)
}

func valuesFromMap(m map[string]toss.StockInfo) []toss.StockInfo {
	out := make([]toss.StockInfo, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

const sharesRefreshBatch = 200
const sharesRefreshPause = 150 * time.Millisecond

// RepairDomesticAssetTypes fixes KRW/KOSPI catalog rows mis-tagged as us (Toss market=KOSPI).
func (s *MarketService) RepairDomesticAssetTypes(ctx context.Context) (int, error) {
	if s.stocks == nil {
		return 0, nil
	}
	symbols, err := s.stocks.ListSymbols(ctx, "", 10000)
	if err != nil {
		return 0, err
	}
	repaired := 0
	for _, sym := range symbols {
		stock, err := s.stocks.GetBySymbol(ctx, sym)
		if err != nil || stock == nil {
			continue
		}
		want := assetTypeFromMarket(stock.Market, stock.SecurityType)
		if stock.Currency == model.CurrencyKRW || isDomesticKRSymbol(stock.Symbol) {
			want = model.AssetDomestic
		}
		if stock.AssetType == want {
			continue
		}
		stock.AssetType = want
		if err := s.stocks.Upsert(ctx, *stock); err != nil {
			continue
		}
		repaired++
	}
	return repaired, nil
}

// RefreshAllSharesOutstanding re-fetches Toss reference data (shares, list date) for catalog symbols.
func (s *MarketService) RefreshAllSharesOutstanding(ctx context.Context) (int, error) {
	if s.stocks == nil {
		return 0, nil
	}
	if n, err := s.RepairDomesticAssetTypes(ctx); err != nil {
		return 0, err
	} else if n > 0 {
		log.Printf("market: repaired domestic asset types for %d symbols", n)
	}
	client, err := s.requireToss()
	if err != nil {
		return 0, err
	}
	symbols, err := s.stocks.ListSymbols(ctx, "", 10000)
	if err != nil {
		return 0, err
	}
	updated := 0
	for i := 0; i < len(symbols); i += sharesRefreshBatch {
		end := min(i+sharesRefreshBatch, len(symbols))
		batch := symbols[i:end]
		fromToss, err := client.GetStocks(ctx, batch)
		if err != nil {
			return updated, err
		}
		for _, info := range fromToss {
			sym := strings.ToUpper(strings.TrimSpace(info.Symbol))
			if sym == "" {
				continue
			}
			existing, _ := s.stocks.GetBySymbol(ctx, sym)
			hasExisting := existing != nil
			var base model.Stock
			if hasExisting {
				base = *existing
			}
			_ = s.persistTossReference(ctx, base, hasExisting, info)
			updated++
		}
		if end < len(symbols) {
			select {
			case <-ctx.Done():
				return updated, ctx.Err()
			case <-time.After(sharesRefreshPause):
			}
		}
	}
	return updated, nil
}
