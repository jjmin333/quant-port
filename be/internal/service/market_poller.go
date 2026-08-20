package service

import (
	"context"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/enact/quant-port/be/internal/safego"
)

const (
	marketFastPollInterval = 1 * time.Second
	marketSlowPollInterval = 30 * time.Second
	marketActiveTTL        = 5 * time.Minute
	marketPinnedRefresh    = 1 * time.Minute

	// marketRefreshConcurrency bounds in-flight upstream refreshes per tick so a
	// tick completes inside its interval without bursting the Toss rate limit.
	marketRefreshConcurrency = 4

	// marketFastSymbolCap bounds how many symbols a single fast tick may refresh.
	marketFastSymbolCap = 40
)

type marketPoller struct {
	quotes *QuotesService

	mu       sync.Mutex
	active   map[string]time.Time
	pinned   []string
	pinnedAt time.Time

	fastBusy atomic.Bool
	slowBusy atomic.Bool
}

func newMarketPoller(quotes *QuotesService) *marketPoller {
	return &marketPoller{
		quotes: quotes,
		active: make(map[string]time.Time),
	}
}

func (p *marketPoller) Touch(symbol string) {
	if symbol == "" {
		return
	}
	p.mu.Lock()
	p.active[strings.ToUpper(symbol)] = time.Now()
	p.mu.Unlock()
}

func (p *marketPoller) Run(ctx context.Context) {
	fast := time.NewTicker(marketFastPollInterval)
	slow := time.NewTicker(marketSlowPollInterval)
	defer fast.Stop()
	defer slow.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-fast.C:
			p.tickFast(ctx)
		case <-slow.C:
			p.tickSlow(ctx)
		}
	}
}

func (p *marketPoller) refreshPinned(ctx context.Context) {
	p.mu.Lock()
	fresh := time.Since(p.pinnedAt) < marketPinnedRefresh
	p.mu.Unlock()
	if fresh {
		return
	}

	seen := map[string]struct{}{}
	var out []string
	add := func(sym string) {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			return
		}
		if _, ok := seen[sym]; ok {
			return
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}

	if p.quotes.marketStore != nil {
		for _, sym := range p.quotes.marketStore.AllTrackedSymbols(ctx) {
			add(sym)
		}
	}
	watch := p.quotes.watchlistSymbols(ctx)
	held := p.quotes.heldSymbols(ctx)
	for _, sym := range append(watch, held...) {
		add(sym)
	}
	p.mu.Lock()
	p.pinned = out
	p.pinnedAt = time.Now()
	p.mu.Unlock()
}

// tickFast refreshes prices for every tracked symbol (single batched upstream
// call) and does a full terminal refresh — stock/orderbook/trades plus the
// daily chart's forming bar — only for symbols someone is actively viewing
// right now (an open detail page or chart, tracked via Touch).
//
// Pinned symbols (watchlist/held, nobody currently viewing) get their price
// kept warm by the batched RefreshPrices call above at zero extra Toss cost;
// their orderbook/trades/forming bars are left alone until Touch promotes
// them to active. Nothing renders those without a live viewer — a watchlist
// *list* view reads batched prices, not the per-symbol terminal snapshot —
// so refreshing them was pure waste. This used to be the largest source of
// MARKET_DATA rate-limit pressure: every pinned symbol got a full refresh
// every 5s regardless of whether anyone was looking. It also means this cost
// no longer scales with watchlist/holdings size, only with how many symbols
// are actually being viewed concurrently — the relevant axis when traffic
// grows.
func (p *marketPoller) tickFast(ctx context.Context) {
	if !p.fastBusy.CompareAndSwap(false, true) {
		// Previous fast tick is still running — skip instead of piling up.
		return
	}
	defer p.fastBusy.Store(false)

	all, active := p.pollSymbols(ctx)
	if p.quotes.marketStore != nil && len(all) > 0 {
		p.quotes.marketStore.RefreshPrices(ctx, all)
	}

	targets := capActiveTargets(active, marketFastSymbolCap)

	// 장이 닫힌 종목은 체결·호가가 더 이상 바뀌지 않는다. 그런데도 이 틱은
	// 1초마다 대상 전부의 터미널 스냅샷을 다시 받아 왔다 — 대상 40개면
	// 체결 40 + 호가 40 = 초당 80 호출이고, Toss의 해당 한도는 각 15/s다.
	//
	// 실측(사용자 요청 없음, 국내장 마감): `rate-limit exceeded`가 분당 약 600건,
	// 분포는 /trades 145 · /orderbook 139. 그 잡음이 실제 시세 갱신과 같은
	// 한도를 다투고 있었다.
	//
	// 마감 종목을 빼도 화면은 그대로다 — 마감 중 상세는 이미 DB의 last-good을
	// 읽는다(loadClosedMarketQuoteDetail). 가격과 캔들은 아래 slow 틱과 배치
	// RefreshPrices가 계속 갱신한다.
	targets = p.quotes.filterLiveSessionSymbols(ctx, targets)
	if len(targets) == 0 {
		return
	}

	// One batched GetStocks call for every symbol this tick will refresh,
	// instead of each symbol's refreshTerminalSnapshot making its own —
	// /api/v1/stocks and GetStock share Toss's STOCK rate-limit bucket.
	p.quotes.prefetchStockInfo(ctx, targets)

	p.forEachSymbol(ctx, targets, func(symbol string) {
		// Share one resolved price between the header (quote_update) and the
		// daily chart's forming bar (bar_update) — see refreshFormingBars.
		snap, ok := p.quotes.refreshTerminalSnapshot(ctx, symbol)
		var price float64
		if ok {
			price = snap.Detail.CurrentPrice
		}
		p.quotes.refreshFormingBars(ctx, symbol, price)
	})
}

// capActiveTargets bounds the actively-viewed set a single fast tick will
// fully refresh, so a burst of concurrent viewers can't blow a tick past its
// 1s interval or the Toss rate limit.
func capActiveTargets(active map[string]struct{}, limit int) []string {
	targets := make([]string, 0, min(len(active), limit))
	for sym := range active {
		if len(targets) >= limit {
			break
		}
		targets = append(targets, sym)
	}
	return targets
}

// forEachSymbol runs fn over symbols with bounded concurrency, aborting early
// when ctx is cancelled.
func (p *marketPoller) forEachSymbol(ctx context.Context, symbols []string, fn func(string)) {
	concurrency := marketRefreshConcurrency
	if len(symbols) < concurrency {
		concurrency = len(symbols)
	}
	if concurrency <= 1 {
		for _, sym := range symbols {
			if ctx.Err() != nil {
				return
			}
			fn(sym)
		}
		return
	}

	work := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sym := range work {
				if ctx.Err() != nil {
					return
				}
				sym := sym
				safego.Call("market-poller-symbol", func() {
					fn(sym)
				})
			}
		}()
	}
	for _, sym := range symbols {
		if ctx.Err() != nil {
			break
		}
		work <- sym
	}
	close(work)
	wg.Wait()
}

// pollSymbols returns every tracked symbol plus the subset a user is actively
// viewing. Expired active entries are pruned.
func (p *marketPoller) pollSymbols(ctx context.Context) ([]string, map[string]struct{}) {
	p.refreshPinned(ctx)

	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()

	seen := map[string]struct{}{}
	var out []string
	add := func(sym string) string {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			return ""
		}
		if _, ok := seen[sym]; ok {
			return sym
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
		return sym
	}

	active := map[string]struct{}{}
	// Active symbols first so the fast-tick cap never starves them.
	for sym, seenAt := range p.active {
		if now.Sub(seenAt) > marketActiveTTL {
			delete(p.active, sym)
			continue
		}
		if norm := add(sym); norm != "" {
			active[norm] = struct{}{}
		}
	}
	for _, sym := range p.pinned {
		add(sym)
	}
	return out, active
}

func (p *marketPoller) tickSlow(ctx context.Context) {
	if !p.quotes.TossEnabled() {
		return
	}
	if !p.slowBusy.CompareAndSwap(false, true) {
		return
	}
	defer p.slowBusy.Store(false)

	symbols, _ := p.pollSymbols(ctx)
	p.forEachSymbol(ctx, symbols, func(symbol string) {
		if err := p.quotes.refreshRawCandles(ctx, symbol); err != nil {
			log.Printf("market poller: refresh candles %s: %v", symbol, err)
		}
		p.quotes.triggerQuoteStatsRefresh(symbol)
	})
}
