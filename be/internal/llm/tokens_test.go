package llm

import (
	"strings"
	"testing"
)

func TestEstimateTokensScalesWithScript(t *testing.T) {
	// Same character count, different scripts: Hangul must cost markedly
	// more than Latin, or a KR-heavy batch would be under-booked and blow
	// through the per-minute token ceiling. Long inputs, so the flat request
	// overhead does not dominate the comparison.
	latin := EstimateTokens(strings.Repeat("abcdefghij", 40))
	hangul := EstimateTokens(strings.Repeat("가나다라마바사아자차", 40))

	if latin >= hangul {
		t.Fatalf("latin %d should cost fewer tokens than hangul %d for equal length", latin, hangul)
	}
	if hangul < latin*2 {
		t.Fatalf("hangul %d should cost at least twice latin %d", hangul, latin)
	}
}

func TestEstimateTokensIsAnUpperBound(t *testing.T) {
	// A real tokenizer puts this English sentence near 10 tokens; the
	// estimate must not come in under that, since the whole point is to
	// reject a call before it crosses a ceiling rather than after.
	got := EstimateTokens("The quick brown fox jumps over the lazy dog every single morning.")
	if got < 15 {
		t.Fatalf("estimate %d is too low to be a safe upper bound", got)
	}
}

func TestEstimateTokensEmpty(t *testing.T) {
	if got := EstimateTokens(); got != 0 {
		t.Fatalf("EstimateTokens() = %d, want 0", got)
	}
	if got := EstimateTokens("", ""); got != 0 {
		t.Fatalf("EstimateTokens(\"\", \"\") = %d, want 0", got)
	}
}

func TestEstimateTokensSumsParts(t *testing.T) {
	// Splitting a prompt across arguments must not change its cost, or the
	// pre-flight estimate would depend on how the caller assembled it.
	joined := EstimateTokens("삼성전자 밸류에이션 해석 abcdefgh")
	split := EstimateTokens("삼성전자 밸류에이션 ", "해석 abcdefgh")
	if joined != split {
		t.Fatalf("joined %d != split %d", joined, split)
	}
}

func TestEstimateStockSummaryTokensCoversSystemPrompt(t *testing.T) {
	c := NewClient(Config{APIKey: "x"}, nil)
	// Even with empty facts the estimate must account for the system prompt
	// plus the expected completion — a near-zero number here would let the
	// batch plan far more symbols than the token budget can pay for.
	got := c.EstimateStockSummaryTokens("", "")
	if got < stockSummaryOutputTokens+500 {
		t.Fatalf("estimate %d does not cover the system prompt + completion", got)
	}

	withFacts := c.EstimateStockSummaryTokens(`{"name":"삼성전자","per":35.6}`, "62점 B+")
	if withFacts <= got {
		t.Fatalf("facts must add cost: %d <= %d", withFacts, got)
	}
}
