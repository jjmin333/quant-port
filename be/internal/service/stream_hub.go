package service

import (
	"sync"

	"github.com/enact/quant-port/be/internal/model"
)

type streamHub struct {
	mu   sync.RWMutex
	subs map[string]map[chan model.StreamEvent]struct{}
}

func newStreamHub() *streamHub {
	return &streamHub{subs: make(map[string]map[chan model.StreamEvent]struct{})}
}

func streamKey(symbol string, interval model.CandleInterval) string {
	return symbol + "|" + string(interval)
}

func (h *streamHub) Subscribe(symbol string, interval model.CandleInterval) (<-chan model.StreamEvent, func()) {
	ch := make(chan model.StreamEvent, 8)
	key := streamKey(symbol, interval)

	h.mu.Lock()
	if h.subs[key] == nil {
		h.subs[key] = make(map[chan model.StreamEvent]struct{})
	}
	h.subs[key][ch] = struct{}{}
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		delete(h.subs[key], ch)
		if len(h.subs[key]) == 0 {
			delete(h.subs, key)
		}
		h.mu.Unlock()
		close(ch)
	}
	return ch, cancel
}

func (h *streamHub) Publish(ev model.StreamEvent) {
	key := streamKey(ev.Symbol, ev.Interval)
	h.mu.RLock()
	targets := h.subs[key]
	h.mu.RUnlock()
	if len(targets) == 0 {
		return
	}

	for ch := range targets {
		select {
		case ch <- ev:
		default:
		}
	}
}
