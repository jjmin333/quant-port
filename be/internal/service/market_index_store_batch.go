package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

const indexEODKeyFmt = "indices:eod:v1:%s:%s"

func (st *MarketIndexStore) refreshAll(ctx context.Context, warmup bool) {
	refreshCtx, cancel := context.WithTimeout(ctx, indicesRefreshTimeout)
	defer cancel()

	out := st.snapshotOrCatalog()
	before := make([]model.MarketIndex, len(out))
	copy(before, out)

	now := time.Now()
	var krCal *toss.KrMarketCalendarResponse
	var usCal *toss.UsMarketCalendarResponse
	if st.marketStore != nil {
		if cals, ok := st.marketStore.EnsureCalendars(refreshCtx); ok {
			krCal = cals.kr
			usCal = cals.us
		}
	}

	for _, batch := range indexBatches {
		run, markEOD := st.shouldRunBatch(refreshCtx, batch, now, krCal, usCal, warmup)
		if !run {
			continue
		}
		if !warmup && !st.shouldRefreshBatch(refreshCtx, batch) {
			continue
		}
		st.refreshBatch(refreshCtx, batch, out, warmup)
		if markEOD {
			st.markEODDone(refreshCtx, batch.id, now)
		}
	}

	if !indicesSnapshotEqual(before, out) {
		at := time.Now().UTC()
		st.mu.Lock()
		st.indices = out
		st.updatedAt = at
		st.mu.Unlock()
		st.persistSnapshotRedis(refreshCtx, out, at)
	}
}

func (st *MarketIndexStore) shouldRunBatch(
	ctx context.Context,
	batch indexBatch,
	now time.Time,
	krCal *toss.KrMarketCalendarResponse,
	usCal *toss.UsMarketCalendarResponse,
	warmup bool,
) (run bool, markEOD bool) {
	if warmup {
		return true, false
	}

	switch batch.id {
	case batchTossKR:
		if calendar.ShouldPoll(calendar.ScheduleKRX, now, krCal, usCal) {
			return true, false
		}
		if batch.eodAfter && calendar.KRIntegratedAfterEnded(krCal, now) && !st.eodDone(ctx, batch.id, now) {
			return true, true
		}
	case batchTossFX:
		if calendar.ShouldPoll(calendar.ScheduleWeekdays, now, krCal, usCal) {
			return true, false
		}
	case batchFREDUS, batchFinnhubUS:
		if calendar.ShouldPoll(calendar.ScheduleUS, now, krCal, usCal) {
			return true, false
		}
		if batch.eodAfter && calendar.USAfterMarketEnded(usCal, now) && !st.eodDone(ctx, batch.id, now) {
			return true, true
		}
	case batchUpbitCrypto:
		if calendar.ShouldPoll(calendar.ScheduleAlways, now, krCal, usCal) {
			return true, false
		}
	case batchGoldSpot:
		return true, false
	}
	return false, false
}

func (st *MarketIndexStore) shouldRefreshBatch(ctx context.Context, batch indexBatch) bool {
	last := st.groupLastRefresh(ctx, string(batch.id))
	if last.IsZero() {
		return true
	}
	return time.Since(last) >= batch.minInterval
}

func (st *MarketIndexStore) refreshBatch(ctx context.Context, batch indexBatch, out []model.MarketIndex, warmup bool) {
	at := time.Now().UTC()
	entries := entriesForBatch(batch)

	switch batch.id {
	case batchTossKR:
		st.refreshTossKRBatch(ctx, batch, entries, out, at)
	case batchTossFX:
		st.refreshTossFXBatch(ctx, batch, entries, out, at)
	case batchFREDUS:
		st.refreshFREDBatch(ctx, batch, entries, out, at)
	case batchFinnhubUS:
		st.refreshFinnhubBatch(ctx, batch, entries, out, at)
	case batchUpbitCrypto:
		st.refreshUpbitBatch(ctx, batch, entries, out, at)
	case batchGoldSpot:
		st.refreshGoldBatch(ctx, batch, entries, out, at)
	default:
		return
	}

	st.noteGroupRefresh(string(batch.id), at, batch.source)
	if warmup {
		log.Printf("market indices warmup: %s ok", batch.id)
	}
}

func (st *MarketIndexStore) refreshTossKRBatch(
	ctx context.Context,
	batch indexBatch,
	entries []indexCatalogEntry,
	out []model.MarketIndex,
	at time.Time,
) {
	if st.toss == nil || !st.toss.Enabled() {
		st.applyBatchFallback(ctx, entries, out)
		return
	}
	prices, err := st.toss.GetMarketIndicatorPrices(ctx, []string{"KOSPI", "KOSDAQ"})
	if err != nil {
		log.Printf("market indices batch %s: %v", batch.id, err)
		st.applyBatchFallback(ctx, entries, out)
		return
	}
	priceBySymbol := map[string]string{}
	for _, p := range prices {
		priceBySymbol[p.Symbol] = p.LastPrice
	}
	for _, entry := range entries {
		last, ok := priceBySymbol[entry.symbol]
		if !ok {
			st.applyIndexFallback(ctx, entry, out)
			continue
		}
		idx := emptyMarketIndex(entry.id, entry.name, entry.currency)
		idx.Value = parseDecimal(last)
		if ch, cr, ok := st.indexChangeFromCandles(ctx, entry.symbol); ok {
			idx.Change, idx.ChangeRate = ch, cr
		}
		idx.Sparkline = st.tossIndexSparkline(ctx, entry.symbol)
		applyIndexMeta(&idx, entry, marketdata.Quote{}, at)
		st.applyIndexSuccess(ctx, entry, out, idx, at)
	}
}

func (st *MarketIndexStore) refreshTossFXBatch(
	ctx context.Context,
	batch indexBatch,
	entries []indexCatalogEntry,
	out []model.MarketIndex,
	at time.Time,
) {
	for _, entry := range entries {
		val := st.usdKrwRate(ctx)
		if val <= 0 {
			st.applyIndexFallback(ctx, entry, out)
			continue
		}
		idx := emptyMarketIndex(entry.id, entry.name, entry.currency)
		idx.Value = val
		applyIndexMeta(&idx, entry, marketdata.Quote{}, at)
		st.applyIndexSuccess(ctx, entry, out, idx, at)
	}
}

func (st *MarketIndexStore) refreshFREDBatch(
	ctx context.Context,
	batch indexBatch,
	entries []indexCatalogEntry,
	out []model.MarketIndex,
	at time.Time,
) {
	if st.external == nil || st.external.FRED == nil || !st.external.FRED.Enabled() {
		st.applyBatchFallback(ctx, entries, out)
		return
	}
	series := make([]string, 0, len(entries))
	symbolToEntry := map[string]indexCatalogEntry{}
	for _, entry := range entries {
		series = append(series, entry.symbol)
		symbolToEntry[entry.symbol] = entry
	}
	quotes, err := st.external.FRED.QuoteBatch(ctx, series, indexSparklinePoints)
	if err != nil {
		log.Printf("market indices batch %s: %v", batch.id, err)
		st.applyBatchFallback(ctx, entries, out)
		return
	}
	var failed []string
	for _, entry := range entries {
		q, ok := quotes[entry.symbol]
		if !ok {
			failed = append(failed, entry.id)
			st.applyIndexFallback(ctx, entry, out)
			continue
		}
		idx := indexFromQuote(entry, q, at)
		st.applyIndexSuccess(ctx, entry, out, idx, at)
	}
	if len(failed) > 0 {
		log.Printf("market indices batch %s: partial %d/%d failed: %s",
			batch.id, len(failed), len(entries), strings.Join(failed, ","))
	}
}

func (st *MarketIndexStore) refreshFinnhubBatch(
	ctx context.Context,
	batch indexBatch,
	entries []indexCatalogEntry,
	out []model.MarketIndex,
	at time.Time,
) {
	if st.external == nil || st.external.Finnhub == nil || !st.external.Finnhub.Enabled() {
		st.applyBatchFallback(ctx, entries, out)
		return
	}
	symbols := make([]string, 0, len(entries))
	for _, entry := range entries {
		symbols = append(symbols, entry.symbol)
	}
	quotes, err := st.external.Finnhub.QuoteBatch(ctx, symbols)
	if err != nil {
		log.Printf("market indices batch %s: %v", batch.id, err)
		st.applyBatchFallback(ctx, entries, out)
		return
	}
	var failed []string
	for _, entry := range entries {
		q, ok := quotes[entry.symbol]
		if !ok {
			failed = append(failed, entry.id)
			st.applyIndexFallback(ctx, entry, out)
			continue
		}
		idx := indexFromQuote(entry, q, at)
		st.applyIndexSuccess(ctx, entry, out, idx, at)
	}
	if len(failed) > 0 {
		log.Printf("market indices batch %s: partial %d/%d failed: %s",
			batch.id, len(failed), len(entries), strings.Join(failed, ","))
	}
}

func (st *MarketIndexStore) refreshUpbitBatch(
	ctx context.Context,
	batch indexBatch,
	entries []indexCatalogEntry,
	out []model.MarketIndex,
	at time.Time,
) {
	if st.external == nil || st.external.Upbit == nil || !st.external.Upbit.Enabled() {
		st.applyBatchFallback(ctx, entries, out)
		return
	}
	for _, entry := range entries {
		q, err := st.external.Upbit.TickerWithSparkline(ctx, entry.symbol, indexSparklinePoints)
		if err != nil {
			log.Printf("market indices batch %s %s: %v", batch.id, entry.id, err)
			st.applyIndexFallback(ctx, entry, out)
			continue
		}
		idx := indexFromQuote(entry, q, at)
		st.applyIndexSuccess(ctx, entry, out, idx, at)
	}
}

func (st *MarketIndexStore) refreshGoldBatch(
	ctx context.Context,
	batch indexBatch,
	entries []indexCatalogEntry,
	out []model.MarketIndex,
	at time.Time,
) {
	if st.external == nil || st.external.GoldPrice == nil || !st.external.GoldPrice.Enabled() {
		st.applyBatchFallback(ctx, entries, out)
		return
	}
	for _, entry := range entries {
		currency := "USD"
		if entry.currency == model.CurrencyKRW {
			currency = "KRW"
		}
		q, err := st.external.GoldPrice.Spot(ctx, entry.symbol, currency)
		if err != nil {
			log.Printf("market indices batch %s %s: %v", batch.id, entry.id, err)
			st.applyIndexFallback(ctx, entry, out)
			continue
		}
		q = enrichGoldQuote(ctx, st.redis, q)
		idx := indexFromQuote(entry, q, at)
		st.applyIndexSuccess(ctx, entry, out, idx, at)
	}
}

func (st *MarketIndexStore) applyIndexSuccess(
	ctx context.Context,
	entry indexCatalogEntry,
	out []model.MarketIndex,
	idx model.MarketIndex,
	at time.Time,
) {
	i, _, ok := catalogEntryByID(entry.id)
	if !ok || idx.Value <= 0 {
		return
	}
	out[i] = idx
	st.persistOne(ctx, entry, idx, at)
	st.noteRefresh(entry.id, at)
}

func (st *MarketIndexStore) applyIndexFallback(ctx context.Context, entry indexCatalogEntry, out []model.MarketIndex) {
	i, _, ok := catalogEntryByID(entry.id)
	if !ok {
		return
	}
	fresh := emptyMarketIndex(entry.id, entry.name, entry.currency)
	resolved := st.resolveIndex(ctx, entry, fresh)
	if resolved.Value > 0 {
		out[i] = resolved
	}
}

func (st *MarketIndexStore) applyBatchFallback(ctx context.Context, entries []indexCatalogEntry, out []model.MarketIndex) {
	for _, entry := range entries {
		st.applyIndexFallback(ctx, entry, out)
	}
}

func (st *MarketIndexStore) noteGroupRefresh(groupID string, at time.Time, source string) {
	st.groupMu.Lock()
	if st.groupRefresh == nil {
		st.groupRefresh = make(map[string]time.Time)
	}
	if st.groupMeta == nil {
		st.groupMeta = make(map[string]model.IndexGroupMeta)
	}
	st.groupRefresh[groupID] = at
	st.groupMeta[groupID] = model.IndexGroupMeta{
		UpdatedAt: at.Format(time.RFC3339),
		Source:    source,
	}
	st.groupMu.Unlock()
	st.noteRefresh("batch:"+groupID, at)
}

func (st *MarketIndexStore) groupLastRefresh(ctx context.Context, groupID string) time.Time {
	st.groupMu.RLock()
	if t, ok := st.groupRefresh[groupID]; ok {
		st.groupMu.RUnlock()
		return t
	}
	st.groupMu.RUnlock()
	return st.lastRefreshAt(ctx, "batch:"+groupID)
}

func (st *MarketIndexStore) GroupMeta() map[string]model.IndexGroupMeta {
	st.groupMu.RLock()
	defer st.groupMu.RUnlock()
	out := make(map[string]model.IndexGroupMeta, len(st.groupMeta))
	for k, v := range st.groupMeta {
		out[k] = v
	}
	return out
}

func (st *MarketIndexStore) eodKey(groupID indexBatchID, now time.Time) string {
	return fmt.Sprintf(indexEODKeyFmt, groupID, now.Format("20060102"))
}

func (st *MarketIndexStore) eodDone(ctx context.Context, groupID indexBatchID, now time.Time) bool {
	key := st.eodKey(groupID, now)
	if st.redis == nil {
		return false
	}
	n, err := st.redis.Exists(ctx, key).Result()
	return err == nil && n > 0
}

func (st *MarketIndexStore) markEODDone(ctx context.Context, groupID indexBatchID, now time.Time) {
	key := st.eodKey(groupID, now)
	if st.redis == nil {
		return
	}
	_ = st.redis.Set(ctx, key, "1", 48*time.Hour).Err()
}
