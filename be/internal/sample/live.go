package sample

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// Quotes is the slice of the quote service the fixtures need to become real.
//
// Why an interface rather than importing the service: the fixtures describe
// **shape** (which symbols, how long ago they were added, what the note says)
// and nothing else. Everything with a number on it — 현재가, 담은 뒤, 전일, 전주,
// 전월 — is read from the same market data the signed-in screens read, through
// the same methods. Hand-written prices went stale the moment the market moved:
// the 삼성전자 fixture still said 74,200원 while the stock traded at 270,000원, so
// every derived column (담은 뒤 수익률, 목표까지 남은 거리) taught the visitor a
// number that could not happen.
type Quotes interface {
	BatchQuoteSummaries(ctx context.Context, symbols []string) []model.QuoteSummary
	WatchlistMetricsFor(ctx context.Context, item *model.QuoteSummary, addedAt time.Time) *model.WatchlistMetrics
	DailyCloseAtOrBefore(ctx context.Context, symbol string, cutoff time.Time) (float64, bool)
	DailyBars(ctx context.Context, symbol string, limit int) []model.Candle
}

// resolveQuotes fetches live quotes for the fixture symbols, indexed by symbol.
// A nil service (or a symbol the market data does not know) yields no entry and
// the caller drops that row rather than showing a priceless one.
func resolveQuotes(ctx context.Context, q Quotes, symbols []string) map[string]*model.QuoteSummary {
	if q == nil {
		return map[string]*model.QuoteSummary{}
	}
	ordered := symbolsOf(symbols)
	if len(ordered) == 0 {
		return map[string]*model.QuoteSummary{}
	}
	return quoteFor(q.BatchQuoteSummaries(ctx, ordered))
}

// barsSince returns the daily bars dated on or after `from`, oldest first.
func barsSince(ctx context.Context, q Quotes, symbol string, from time.Time) []model.Candle {
	if q == nil {
		return nil
	}
	// 400 covers the longest fixture window (about a quarter) with room for a
	// symbol whose history is sparse.
	bars := q.DailyBars(ctx, symbol, 400)
	if len(bars) == 0 {
		return nil
	}
	cutoff := from.In(model.KST)
	cutoff = time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, model.KST)

	out := make([]model.Candle, 0, len(bars))
	for _, bar := range bars {
		at, err := time.ParseInLocation("2006-01-02", firstTen(bar.Time), model.KST)
		if err != nil || at.Before(cutoff) {
			continue
		}
		out = append(out, bar)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time < out[j].Time })
	return out
}

func firstTen(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// rangeSince is the real high/low the symbol printed since the row was created.
// The target screen draws a 진행축 from these, so inventing them would leave the
// axis looking plausible while meaning nothing.
func rangeSince(
	ctx context.Context,
	q Quotes,
	symbol string,
	from time.Time,
	current float64,
) (high, low float64, ok bool) {
	bars := barsSince(ctx, q, symbol, from)
	if len(bars) == 0 {
		return 0, 0, false
	}
	for _, bar := range bars {
		if bar.High > high {
			high = bar.High
		}
		if bar.Low > 0 && (low == 0 || bar.Low < low) {
			low = bar.Low
		}
	}
	// The live price can sit outside a range built from completed bars.
	if current > 0 {
		if current > high {
			high = current
		}
		if low == 0 || current < low {
			low = current
		}
	}
	if high <= 0 || low <= 0 {
		return 0, 0, false
	}
	return high, low, true
}

// reachedDateFor is the first session whose high touched the target — the date
// the screen prints next to 「도달」. Empty when no bar ever got there.
func reachedDateFor(
	ctx context.Context,
	q Quotes,
	symbol string,
	from time.Time,
	target float64,
) string {
	if target <= 0 {
		return ""
	}
	for _, bar := range barsSince(ctx, q, symbol, from) {
		if bar.High >= target {
			return firstTen(bar.Time)
		}
	}
	return ""
}

// quoteFor indexes a batch response by symbol so each fixture row can find its
// own quote without rescanning the slice.
func quoteFor(qs []model.QuoteSummary) map[string]*model.QuoteSummary {
	out := make(map[string]*model.QuoteSummary, len(qs))
	for i := range qs {
		sym := strings.ToUpper(strings.TrimSpace(qs[i].Symbol))
		if sym == "" {
			continue
		}
		out[sym] = &qs[i]
	}
	return out
}

// symbolsOf keeps the order the fixtures declare so the batch call and the
// rendered list agree.
func symbolsOf(rows []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(rows))
	for _, raw := range rows {
		sym := strings.ToUpper(strings.TrimSpace(raw))
		if sym == "" {
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

// historicalCloses resolves the daily close for each (symbol, date) pair.
//
// This is what turns 「담은 뒤」·「등록가」·「작성 당시」 into real numbers: the fixture
// says *when*, the DB says *at what price*. Pairs are looked up concurrently
// because each is an independent indexed read and a sample response should not
// cost one round trip per row in series.
func historicalCloses(
	ctx context.Context,
	q Quotes,
	pairs map[string]time.Time,
) map[string]float64 {
	out := make(map[string]float64, len(pairs))
	if q == nil || len(pairs) == 0 {
		return out
	}
	type result struct {
		key   string
		price float64
	}
	results := make([]result, 0, len(pairs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for key, at := range pairs {
		wg.Add(1)
		go func(key string, at time.Time) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// The key is "SYMBOL@unix" so two rows on the same symbol but
			// different dates (a note and a target, say) stay separate.
			sym := key
			if idx := strings.LastIndex(key, "@"); idx > 0 {
				sym = key[:idx]
			}
			price, ok := q.DailyCloseAtOrBefore(ctx, sym, at)
			if !ok || price <= 0 {
				return
			}
			mu.Lock()
			results = append(results, result{key: key, price: price})
			mu.Unlock()
		}(key, at)
	}
	wg.Wait()
	for _, r := range results {
		out[r.key] = r.price
	}
	return out
}

func closeKey(symbol string, at time.Time) string {
	return strings.ToUpper(strings.TrimSpace(symbol)) + "@" + at.UTC().Format("20060102")
}

// endOfDay is the cutoff a "close on day D" lookup needs — the bar for D itself
// must be included, so the cutoff is the last instant of D, not its start.
func endOfDay(t time.Time) time.Time {
	d := t.In(model.KST)
	return time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, model.KST)
}
