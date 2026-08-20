package safego

import (
	"sync"
	"testing"
	"time"
)

func TestGoRecoversPanic(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	Go("test", func() {
		defer wg.Done()
		panic("boom")
	})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not finish — panic was not recovered")
	}
}

func TestCallRecoversPanic(t *testing.T) {
	if !Call("item", func() { panic("boom") }) {
		t.Fatal("Call should report panic")
	}
}
