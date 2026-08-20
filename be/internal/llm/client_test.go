package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func withFastUnavailableRetry(t *testing.T) {
	t.Helper()
	old := llmUnavailableRetryWait
	llmUnavailableRetryWait = time.Millisecond
	t.Cleanup(func() { llmUnavailableRetryWait = old })
}

func newTestLLM(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(Config{APIKey: "test", BaseURL: srv.URL, Model: "test-model"}, nil)
	c.httpClient = srv.Client()
	return c
}

func TestChatCompletionRetries503Once(t *testing.T) {
	withFastUnavailableRetry(t)
	var hits atomic.Int32
	c := newTestLLM(t, func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`[{"error":{"status":"UNAVAILABLE"}}]`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"안녕"}}],"usage":{"total_tokens":9}}`))
	})

	got, err := c.TranslateENToKO(context.Background(), "hello")
	if err != nil {
		t.Fatalf("retry should succeed: %v", err)
	}
	if got != "안녕" {
		t.Fatalf("got %q", got)
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("hits=%d want 2", n)
	}
}

func TestChatCompletionDoesNotRetry503Twice(t *testing.T) {
	withFastUnavailableRetry(t)
	var hits atomic.Int32
	c := newTestLLM(t, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`unavailable`))
	})

	_, err := c.TranslateENToKO(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected 503 after one retry")
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("hits=%d want 2 (original + one retry)", n)
	}
}

func TestChatCompletionDoesNotRetry429(t *testing.T) {
	withFastUnavailableRetry(t)
	var hits atomic.Int32
	c := newTestLLM(t, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`quota`))
	})

	_, err := c.TranslateENToKO(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected 429")
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("hits=%d want 1 (429 must not retry)", n)
	}
}
