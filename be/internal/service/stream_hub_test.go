package service

import (
	"sync"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestStreamHubConcurrentPublishSubscribe(t *testing.T) {
	t.Parallel()
	h := newStreamHub()
	ev := model.StreamEvent{Symbol: "005930", Interval: model.Interval1m}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, cancel := h.Subscribe(ev.Symbol, ev.Interval)
			defer cancel()
			_ = ch
		}()
	}
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Publish(ev)
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out — possible deadlock")
	}
}

func TestQuoteStreamHubConcurrentPublishSubscribe(t *testing.T) {
	t.Parallel()
	h := newQuoteStreamHub()
	ev := model.QuoteStreamEvent{Symbol: "AAPL"}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, cancel := h.Subscribe(ev.Symbol)
			defer cancel()
			_ = ch
		}()
	}
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Publish(ev)
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out — possible deadlock")
	}
}
