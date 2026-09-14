package apibudget

import (
	"strings"
	"testing"
	"time"
)

// LLM 버킷의 하루는 태평양 자정에 시작한다 — 상대(Gemini 무료 티어)가 그렇게 세기
// 때문이다. 나머지 provider는 예전대로 KST다.
func TestQuotaZoneSplitsLLMFromTheRest(t *testing.T) {
	for _, provider := range []Provider{ProviderLLMKR, ProviderLLMUS, ProviderOpenAI} {
		if got := quotaZone(provider); got != pacific {
			t.Fatalf("%s zone = %v, want pacific", provider, got)
		}
	}
	for _, provider := range []Provider{ProviderDART, ProviderNaver, Provider("unknown_provider")} {
		if got := quotaZone(provider); got != kst {
			t.Fatalf("%s zone = %v, want KST", provider, got)
		}
	}
}

/*
같은 순간이라도 하루 키가 갈린다.

국내 배치가 도는 시각(16:30 KST)은 태평양으로는 **그날이 막 시작된 직후**이고 KST로는
하루의 끝자락이다. 이 차이가 헤드룸을 상대의 잔여와 맞추는 전부다.
*/
func TestCounterKeyUsesTheProvidersOwnDay(t *testing.T) {
	m := NewManager(nil)
	// 2026-09-08 16:30 KST = 2026-09-08 00:30 PDT.
	at := time.Date(2026, 9, 8, 16, 30, 0, 0, kst)

	llm := m.counterKey(ProviderLLMKR, "day", at)
	dart := m.counterKey(ProviderDART, "day", at)

	if !strings.HasSuffix(llm, "2026-09-08") {
		t.Fatalf("llm day key = %s, want the 09-08 Pacific day", llm)
	}
	if !strings.HasSuffix(dart, "2026-09-08") {
		t.Fatalf("dart day key = %s, want the 09-08 KST day", dart)
	}

	// 09:00 KST = 전날 17:00 PDT — 여기서 두 하루가 실제로 갈린다.
	morning := time.Date(2026, 9, 8, 9, 0, 0, 0, kst)
	if got := m.counterKey(ProviderLLMKR, "day", morning); !strings.HasSuffix(got, "2026-09-07") {
		t.Fatalf("llm day key = %s, want the 09-07 Pacific day", got)
	}
	if got := m.counterKey(ProviderDART, "day", morning); !strings.HasSuffix(got, "2026-09-08") {
		t.Fatalf("dart day key = %s, want the 09-08 KST day", got)
	}
}
