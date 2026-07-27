package service

import "time"

type indexBatchID string

const (
	batchTossKR       indexBatchID = "toss:kr"
	batchTossFX       indexBatchID = "toss:fx"
	batchFREDUS       indexBatchID = "fred:us"
	batchFinnhubUS    indexBatchID = "finnhub:us"
	batchUpbitCrypto  indexBatchID = "upbit:crypto"
	batchGoldSpot     indexBatchID = "goldprice:spot"
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
		id:          batchTossKR,
		source:      "toss",
		minInterval: tossIndexMinRefresh,
		indexIDs:    []string{"kospi", "kosdaq"},
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
		indexIDs:    []string{"nasdaq", "nasdaq100", "sp500", "dow", "vix"},
		eodAfter:    true,
	},
	{
		id:          batchFinnhubUS,
		source:      "finnhub",
		minInterval: 5 * time.Minute,
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
