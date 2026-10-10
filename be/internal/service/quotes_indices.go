package service

import (
	"fmt"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
)

type indexSource string

const (
	indexSourceToss      indexSource = "toss"
	indexSourceFinnhub   indexSource = "finnhub"
	indexSourceFRED      indexSource = "fred"
	indexSourceUpbit     indexSource = "upbit"
	indexSourceGoldPrice indexSource = "goldprice"
	// 한국 금리(2026-10-01) — worker-asset의 ecos_rates가 채운 macro_rates 표를 읽는다(API는 외부 호출 없음).
	indexSourceEcos indexSource = "ecos"
)

type indexCatalogEntry struct {
	id       string
	name     string
	currency model.QuoteCurrency
	source   indexSource
	symbol   string
	badge    string
}

// marketIndexCatalog — global market overview entries (multi-provider).
var marketIndexCatalog = []indexCatalogEntry{
	{id: "kospi", name: "KOSPI", currency: model.CurrencyKRW, source: indexSourceToss, symbol: "KOSPI"},
	{id: "kosdaq", name: "KOSDAQ", currency: model.CurrencyKRW, source: indexSourceToss, symbol: "KOSDAQ"},
	{id: "usd-krw", name: "USD/KRW", currency: model.CurrencyKRW, source: indexSourceToss, symbol: "USD/KRW"},
	{id: "nasdaq", name: "나스닥", currency: model.CurrencyUSD, source: indexSourceFRED, symbol: "NASDAQCOM"},
	{id: "nasdaq100", name: "나스닥 100", currency: model.CurrencyUSD, source: indexSourceFRED, symbol: "NASDAQ100"},
	{id: "nq100", name: "나스닥100 선물", currency: model.CurrencyUSD, source: indexSourceFinnhub, symbol: "QQQ", badge: "QQQ"},
	{id: "sp500", name: "S&P 500", currency: model.CurrencyUSD, source: indexSourceFRED, symbol: "SP500"},
	{id: "dow", name: "다우존스", currency: model.CurrencyUSD, source: indexSourceFRED, symbol: "DJIA"},
	/*
		지수 추종 ETF SPY·DIA(10-01 추가)는 **뺐다**(2026-10-07 사용자 결정 — 지수와 ETF가 같은 시장을 두 번
		보였다). 출처 표기는 지수 모달 안에 있다. 카탈로그 길이가 바뀌어도 Redis 옛 스냅샷은
		`alignIndicesToCatalog`가 id로 다시 맞춘다(10-01 패닉의 수정).
	*/
	{id: "vix", name: "VIX", currency: model.CurrencyUSD, source: indexSourceFRED, symbol: "VIXCLS"},
	{id: "us10y", name: "미 국채 10년", currency: model.CurrencyUSD, source: indexSourceFRED, symbol: "DGS10"},
	// 한국 금리(값은 %, 등락은 %p) — 일별(ECOS 다음 날 공표)·기준금리는 월별.
	{id: "kr-base", name: "한국 기준금리", currency: model.CurrencyKRW, source: indexSourceEcos, symbol: "722Y001/0101000"},
	/*
		국고채 3·10년은 **토스 지표 API로 장중 실시간**이다(2026-10-07 — 그전엔 ECOS 다음 날 공표값).
		KOSPI·KOSDAQ와 같은 /market-indicators/prices 한 번에 같이 받는다(추가 요청 없음).
		금리 화면·자산 AI가 쓰는 ECOS 일별 표(macro_rates)는 그대로다.
	*/
	{id: "ktb3", name: "국고채 3년", currency: model.CurrencyKRW, source: indexSourceToss, symbol: "KR_BOND_3Y"},
	{id: "ktb10", name: "국고채 10년", currency: model.CurrencyKRW, source: indexSourceToss, symbol: "KR_BOND_10Y"},
	{id: "sox", name: "필라델피아 반도체", currency: model.CurrencyUSD, source: indexSourceFinnhub, symbol: "SOXX", badge: "SOXX"},
	{id: "btc", name: "비트코인", currency: model.CurrencyKRW, source: indexSourceUpbit, symbol: "KRW-BTC"},
	{id: "gold", name: "금", currency: model.CurrencyUSD, source: indexSourceGoldPrice, symbol: "XAU"},
}

const (
	quotesIndexKeyFmt    = "quotes:index:v6:%s"
	indexGoodCacheTTL    = 45 * 24 * time.Hour
	indexSparklinePoints = 20
)

type cachedMarketIndex struct {
	Index     model.MarketIndex `json:"index"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

func emptyIndicesFromCatalog() []model.MarketIndex {
	out := make([]model.MarketIndex, len(marketIndexCatalog))
	for i, idx := range marketIndexCatalog {
		out[i] = emptyMarketIndex(idx.id, idx.name, idx.currency)
	}
	return out
}

// alignIndicesToCatalog rewrites a snapshot into current catalog order and
// length. Redis may still hold a shorter list from before an index was added;
// indexing that slice by catalog position would pin the wrong meta on sox/금.
func alignIndicesToCatalog(indices []model.MarketIndex) []model.MarketIndex {
	out := emptyIndicesFromCatalog()
	if len(indices) == 0 {
		return out
	}
	byID := make(map[string]model.MarketIndex, len(indices))
	for i, idx := range indices {
		if idx.ID == "" && i < len(marketIndexCatalog) {
			idx.ID = marketIndexCatalog[i].id
		}
		if idx.ID != "" {
			byID[idx.ID] = idx
		}
	}
	for i, entry := range marketIndexCatalog {
		if idx, ok := byID[entry.id]; ok {
			out[i] = idx
			if out[i].Name == "" {
				out[i].Name = entry.name
			}
			if out[i].ID == "" {
				out[i].ID = entry.id
			}
		}
	}
	return out
}

func batchMinIntervalForIndex(indexID string) time.Duration {
	for _, batch := range indexBatches {
		for _, id := range batch.indexIDs {
			if id == indexID {
				return batch.minInterval
			}
		}
	}
	var provider apibudget.Provider
	_, entry, ok := catalogEntryByID(indexID)
	if !ok {
		return 0
	}
	switch entry.source {
	case indexSourceFinnhub:
		provider = apibudget.ProviderFinnhub
	case indexSourceFRED:
		provider = apibudget.ProviderFRED
	case indexSourceUpbit:
		provider = apibudget.ProviderUpbit
	case indexSourceGoldPrice:
		provider = apibudget.ProviderGoldPrice
	default:
		return 0
	}
	if lim, ok := apibudget.DefaultLimits[provider]; ok {
		return lim.MinInterval
	}
	return 0
}

func indexFromQuote(entry indexCatalogEntry, q marketdata.Quote, at time.Time) model.MarketIndex {
	idx := model.MarketIndex{
		ID:         entry.id,
		Name:       entry.name,
		Value:      roundIndexValue(q.Value),
		Change:     roundIndexValue(q.Change),
		ChangeRate: q.ChangeRate,
		Currency:   entry.currency,
		Badge:      entry.badge,
	}
	if len(q.Sparkline) >= 2 {
		idx.Sparkline = q.Sparkline
	}
	applyIndexMeta(&idx, entry, q, at)
	return idx
}

func catalogEntryByID(id string) (int, indexCatalogEntry, bool) {
	for i, entry := range marketIndexCatalog {
		if entry.id == id {
			return i, entry, true
		}
	}
	return 0, indexCatalogEntry{}, false
}

func indexCacheKey(id string) string {
	return fmt.Sprintf(quotesIndexKeyFmt, id)
}
