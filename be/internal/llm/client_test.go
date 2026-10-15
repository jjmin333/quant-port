package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
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

// 시장 브리핑의 503 재시도는 같은 모델이 아니라 대체 모델로 간다. 주 모델의 503은
// 미국 낮 시간 내내 이어져, 4초 뒤 같은 모델은 거의 늘 같은 대답이었다.
func TestNewsBriefingRetries503OnFallbackModel(t *testing.T) {
	withFastUnavailableRetry(t)
	var models []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		models = append(models, body.Model)
		if body.Model == "primary" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`[{"error":{"status":"UNAVAILABLE"}}]`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"summary\":\"요약\",\"mood\":\"중립\"}"}}],"usage":{"total_tokens":9}}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(Config{APIKey: "test", BaseURL: srv.URL, Model: "primary", FallbackModel: "backup"}, nil)
	c.httpClient = srv.Client()

	out, err := c.GenerateNewsBriefing(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("fallback should answer: %v", err)
	}
	if out.Summary != "요약" || out.Model != "backup" {
		t.Fatalf("summary=%q model=%q, want 요약/backup", out.Summary, out.Model)
	}
	if len(models) != 2 || models[0] != "primary" || models[1] != "backup" {
		t.Fatalf("models=%v, want [primary backup]", models)
	}
}

// 대체 모델은 opt-in이다 — 브리핑 외 경로(번역·종목 요약)는 같은 모델로 한 번 더.
func TestChatCompletionRetries503OnSameModelWithoutOptIn(t *testing.T) {
	withFastUnavailableRetry(t)
	var models []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		models = append(models, body.Model)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(Config{APIKey: "test", BaseURL: srv.URL, Model: "primary", FallbackModel: "backup"}, nil)
	c.httpClient = srv.Client()

	if _, err := c.TranslateENToKO(context.Background(), "hello"); err == nil {
		t.Fatal("expected 503")
	}
	if len(models) != 2 || models[1] != "primary" {
		t.Fatalf("models=%v, want [primary primary]", models)
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

// 배치가 끝에서 다시 부르는 것은 503과 타임아웃뿐이다 — 예산 소진(429)은 다시 물어도 같은 대답이다.
func TestIsTransient(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer slow.Close()
	_, timeoutErr := (&http.Client{Timeout: 20 * time.Millisecond}).Get(slow.URL)

	cases := []struct {
		err  error
		want bool
	}{
		{fmt.Errorf("llm api 503: high demand"), true},
		{timeoutErr, true},
		{apibudget.Exhaustedf("llm api 429: quota"), false},
		{fmt.Errorf("llm api 400: bad request"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := IsTransient(c.err); got != c.want {
			t.Errorf("IsTransient(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
