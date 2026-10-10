package service

import "time"

type indexBatchID string

const (
	batchTossKR      indexBatchID = "toss:kr"
	batchTossFX      indexBatchID = "toss:fx"
	batchFREDUS      indexBatchID = "fred:us"
	batchFinnhubUS   indexBatchID = "finnhub:us"
	batchUpbitCrypto indexBatchID = "upbit:crypto"
	batchGoldSpot    indexBatchID = "goldprice:spot"
	batchEcosKR      indexBatchID = "ecos:kr"
)

type indexBatch struct {
	id          indexBatchID
	source      string
	minInterval time.Duration
	indexIDs    []string
	eodAfter    bool // run once after market close, then persist only
}

var indexBatches = []indexBatch{
	{
		// DB만 읽는다(macro_rates) — 간격은 표가 하루 한 번 바뀌는 것에 맞춘다.
		id:          batchEcosKR,
		source:      "ecos",
		minInterval: 30 * time.Minute,
		indexIDs:    []string{"kr-base"}, // 국고채는 10-07부터 토스 실시간(아래 toss:kr)
	},
	{
		id:          batchTossKR,
		source:      "toss",
		minInterval: tossIndexMinRefresh,
		indexIDs:    []string{"kospi", "kosdaq", "ktb3", "ktb10"},
		eodAfter:    true,
	},
	{
		id:          batchTossFX,
		source:      "toss",
		minInterval: tossIndexMinRefresh,
		indexIDs:    []string{"usd-krw"},
	},
	{
		id:          batchFREDUS,
		source:      "fred",
		minInterval: 3 * time.Minute,
		indexIDs:    []string{"nasdaq", "nasdaq100", "sp500", "dow", "vix", "us10y"},
		eodAfter:    true,
	},
	{
		id:          batchFinnhubUS,
		source:      "finnhub",
		/*
			2분(2026-10-07, 그전 5분). SPY·DIA는 화면에서 빠져(10-07) 더 받지 않는다 — 회당 4호출 → 2호출이라
			분당 호출은 0.8 → 1.0으로 거의 그대로이고 값은 2.5배 자주 바뀐다. Finnhub 무료 상한 60/분 대비 미미.
			S&P·다우·나스닥 지수 자체(FRED)는 **하루 한 번 바뀌는 일별 시리즈**라 더 자주 물어도 값이 안 바뀐다.
		*/
		minInterval: 2 * time.Minute,
		indexIDs:    []string{"nq100", "sox"},
	},
	{
		id:          batchUpbitCrypto,
		source:      "upbit",
		minInterval: 2 * time.Minute,
		indexIDs:    []string{"btc"},
	},
	{
		id:          batchGoldSpot,
		source:      "goldprice",
		minInterval: 90 * time.Minute,
		indexIDs:    []string{"gold"},
	},
}

func entriesForBatch(batch indexBatch) []indexCatalogEntry {
	var out []indexCatalogEntry
	for _, id := range batch.indexIDs {
		if _, entry, ok := catalogEntryByID(id); ok {
			out = append(out, entry)
		}
	}
	return out
}
