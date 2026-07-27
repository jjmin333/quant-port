package service

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
)

const (
	marketFastPollInterval = 2 * time.Second
	marketSlowPollInterval = 30 * time.Second
	marketActiveTTL        = 5 * time.Minute
	marketPinnedRefresh    = 1 * time.Minute
)

type marketPoller struct {
	quotes      *QuotesService
	mu          sync.Mutex
	active      map[string]time.Time
	pinned      []string
	pinnedAt    time.Time
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
	if time.Since(p.pinnedAt) < marketPinnedRefresh {
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

func (p *marketPoller) tickFast(ctx context.Context) {
	symbols := p.pollSymbols(ctx)
	if p.quotes.marketStore != nil && len(symbols) > 0 {
		p.quotes.marketStore.RefreshPrices(ctx, symbols)
	}
	for _, symbol := range symbols {
		if !p.quotes.shouldLivePollSymbol(ctx, symbol) {
			continue
		}
		p.quotes.refreshTerminalSnapshot(ctx, symbol)
		p.quotes.refreshFormingBars(ctx, symbol)
	}
}

func (p *marketPoller) pollSymbols(ctx context.Context) []string {
	p.refreshPinned(ctx)

	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()

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

	for _, sym := range p.pinned {
		add(sym)
	}
	for sym, seenAt := range p.active {
		if now.Sub(seenAt) > marketActiveTTL {
			delete(p.active, sym)
			continue
		}
		add(sym)
	}
	return out
}


func (p *marketPoller) tickSlow(ctx context.Context) {
	for _, symbol := range p.pollSymbols(ctx) {
		if !p.quotes.shouldLivePollSymbol(ctx, symbol) {
			continue
		}
		if err := p.quotes.refreshRawCandles(ctx, symbol); err != nil {
			log.Printf("market poller: refresh candles %s: %v", symbol, err)
		}
		p.quotes.triggerQuoteStatsRefresh(symbol)
	}
}
