package llm

import (
	"context"
	"net/http"
	"testing"
)

// 뉴스 브리핑은 실패하면 한 번 더 걸어 보되 **한도 초과일 때는 걸지 않는다**.
// 그 판정이 여기 두 함수에 걸려 있어서, 어느 실패가 어느 쪽인지를 못 박아 둔다.
// 예전에는 메시지 문자열을 뒤져 가려냈고, 그러면 문구 한 번 고칠 때마다 조용히
// 어긋난다.

func TestQuotaResponseIsBudgetExhausted(t *testing.T) {
	c := newTestLLM(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"status":"RESOURCE_EXHAUSTED"}}`))
	})

	_, err := c.GenerateNewsBriefing(context.Background(), `{"market":"domestic"}`)
	if err == nil {
		t.Fatal("429 should fail")
	}
	if !IsBudgetExhausted(err) {
		t.Fatalf("429 must be budget-exhausted, got %v", err)
	}
	if IsUnavailable(err) {
		t.Fatalf("429 is not the capacity 503: %v", err)
	}
}

func TestServerErrorIsRetryable(t *testing.T) {
	c := newTestLLM(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	})

	_, err := c.GenerateNewsBriefing(context.Background(), `{"market":"domestic"}`)
	if err == nil {
		t.Fatal("500 should fail")
	}
	// 500은 재시도 대상이다 — 예산 문제도 아니고, chatCompletion이 이미 다시
	// 걸어 본 503도 아니다.
	if IsBudgetExhausted(err) {
		t.Fatalf("500 must not be read as a spent allowance: %v", err)
	}
	if IsUnavailable(err) {
		t.Fatalf("500 must not be read as the 503 that was already retried: %v", err)
	}
}

func TestUnavailableIsMarkedAsAlreadyRetried(t *testing.T) {
	withFastUnavailableRetry(t)
	c := newTestLLM(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`unavailable`))
	})

	_, err := c.GenerateNewsBriefing(context.Background(), `{"market":"domestic"}`)
	if err == nil {
		t.Fatal("503 should fail")
	}
	if !IsUnavailable(err) {
		t.Fatalf("503 must be recognisable, got %v", err)
	}
	if IsBudgetExhausted(err) {
		t.Fatalf("503 is capacity, not quota: %v", err)
	}
}

func TestMalformedJSONIsRetryable(t *testing.T) {
	c := newTestLLM(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"not json at all"}}],"usage":{"total_tokens":5}}`))
	})

	_, err := c.GenerateNewsBriefing(context.Background(), `{"market":"domestic"}`)
	if err == nil {
		t.Fatal("unparsable content should fail")
	}
	if IsBudgetExhausted(err) || IsUnavailable(err) {
		t.Fatalf("a parse failure is retryable, got %v", err)
	}
}
