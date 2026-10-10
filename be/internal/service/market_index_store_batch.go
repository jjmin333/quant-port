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
	"github.com/enact/quant-port/be/internal/repository"
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

	// 배치가 끝난 뒤에 한 번 훑는다 — 어느 배치가 무엇을 못 채웠는지 여기서만
	// 한눈에 보이고, 결과가 스냅샷에 실려 캐시되므로 읽기 경로는 DB를 안 탄다.
	st.fillGapsFromHistory(refreshCtx, out)

	if !indicesSnapshotEqual(before, out) {
		at := time.Now().UTC()
		st.mu.Lock()
		st.indices = out
		st.updatedAt = at
		st.mu.Unlock()
		st.persistSnapshotRedis(refreshCtx, out, at)
	}
	/*
		회차를 끝냈다는 표시 — 값이 안 바뀌어도 움직인다. 제한 시간 안에 못 끝낸 회차는
		적지 않는다(그건 「못 받아 왔다」에 가깝다).
	*/
	if refreshCtx.Err() == nil {
		st.mu.Lock()
		st.checkedAt = time.Now().UTC()
		st.mu.Unlock()
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
	case batchGoldSpot, batchEcosKR:
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
	case batchEcosKR:
		st.refreshEcosBatch(ctx, entries, out, at)
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
	symbols := make([]string, 0, len(entries))
	for _, entry := range entries {
		symbols = append(symbols, entry.symbol)
	}
	prices, err := st.toss.GetMarketIndicatorPrices(ctx, symbols)
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
		// 일봉 한 번으로 등락과 흐름선을 함께 만든다(예전엔 같은 일봉을 두 번 불렀다).
		closes := st.tossIndexDailyCloses(ctx, entry.symbol)
		if ch, cr, ok := changeFromCloses(closes); ok {
			idx.Change, idx.ChangeRate = ch, cr
		}
		idx.Sparkline = sparklineFromCloses(closes)
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
	// KST calendar day, not now's own zone (process-local, UTC in Docker) — a
	// "ran today" marker keyed by the wrong day boundary can let the EOD
	// persist fire twice in one KST trading day or skip one.
	return fmt.Sprintf(indexEODKeyFmt, groupID, now.In(model.KST).Format("20060102"))
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

// SetRates wires the macro_rates table for the 한국 금리 지수(ecos).
func (st *MarketIndexStore) SetRates(repo *repository.PublicDataRepo) { st.rates = repo }

// refreshEcosBatch reads the latest rates from macro_rates (DB만) — 값 %, 등락 %p, 스파크라인 최근 20개.
func (st *MarketIndexStore) refreshEcosBatch(ctx context.Context, entries []indexCatalogEntry, out []model.MarketIndex, at time.Time) {
	if st.rates == nil {
		st.applyBatchFallback(ctx, entries, out)
		return
	}
	for _, entry := range entries {
		pts, err := st.rates.RateSeries(ctx, entry.symbol, time.Now().AddDate(-2, 0, 0))
		if err != nil || len(pts) == 0 {
			st.applyIndexFallback(ctx, entry, out)
			continue
		}
		last := pts[len(pts)-1]
		q := marketdata.Quote{Value: last.Value}
		if len(pts) >= 2 {
			prev := pts[len(pts)-2].Value
			q.Change = last.Value - prev
			if prev != 0 {
				q.ChangeRate = q.Change / prev * 100
			}
		}
		start := max(0, len(pts)-20)
		for _, p := range pts[start:] {
			q.Sparkline = append(q.Sparkline, p.Value)
		}
		if t, err := time.ParseInLocation("2006-01-02", last.Date, time.FixedZone("KST", 9*3600)); err == nil {
			q.AsOf = t
		}
		st.applyIndexSuccess(ctx, entry, out, indexFromQuote(entry, q, at), at)
	}
}
