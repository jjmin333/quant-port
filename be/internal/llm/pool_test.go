package llm

import (
	"testing"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

func TestRegionOf(t *testing.T) {
	cases := map[string]string{
		"KR":        RegionKR,
		"kr":        RegionKR,
		"domestic":  RegionKR,
		"":          RegionKR,
		"unknown":   RegionKR,
		"US":        RegionUS,
		"us":        RegionUS,
		"overseas":  RegionUS,
	}
	for in, want := range cases {
		if got := RegionOf(in); got != want {
			t.Errorf("RegionOf(%q)=%q want %q", in, got, want)
		}
	}
}

func TestPoolSplitKeysUseSeparateBudgetProviders(t *testing.T) {
	p := NewPoolFromKeys(Config{Provider: "gemini", Model: "test"}, "key-kr", "key-us", nil)
	if !p.Split() {
		t.Fatal("distinct keys should Split")
	}
	if p.SharedBudget() {
		t.Fatal("distinct keys should not share a budget")
	}
	kr := p.For("KR")
	us := p.For("US")
	if kr == nil || us == nil {
		t.Fatal("both regions should have a client")
	}
	if kr == us {
		t.Fatal("split keys must not share a client pointer")
	}
	if kr.BudgetProvider() != apibudget.ProviderLLMKR {
		t.Errorf("KR bucket=%q want llm_kr", kr.BudgetProvider())
	}
	if us.BudgetProvider() != apibudget.ProviderLLMUS {
		t.Errorf("US bucket=%q want llm_us", us.BudgetProvider())
	}
	if p.For("domestic").BudgetProvider() != apibudget.ProviderLLMKR {
		t.Error("news briefing market 'domestic' should charge the KR key")
	}
	if p.For("us").BudgetProvider() != apibudget.ProviderLLMUS {
		t.Error("news briefing market 'us' should charge the US key")
	}
}

func TestPoolSameKeySharesBucket(t *testing.T) {
	p := NewPoolFromKeys(Config{Provider: "gemini"}, "same", "same", nil)
	if p.Split() {
		t.Fatal("identical keys must not Split — Google counts them as one quota")
	}
	if p.For("KR") != p.For("US") {
		t.Fatal("identical keys should share one client")
	}
	if p.For("KR").BudgetProvider() != apibudget.ProviderOpenAI {
		t.Errorf("shared bucket=%q want openai", p.For("KR").BudgetProvider())
	}
}

func TestPoolSingleKeyFallsBackToShared(t *testing.T) {
	onlyKR := NewPoolFromKeys(Config{}, "only-kr", "", nil)
	if onlyKR.Split() {
		t.Fatal("one key is shared, not split")
	}
	if !onlyKR.For("US").Enabled() {
		t.Fatal("US should fall back to the only configured key")
	}
	onlyUS := NewPoolFromKeys(Config{}, "", "only-us", nil)
	if !onlyUS.For("KR").Enabled() {
		t.Fatal("KR should fall back to the only configured key")
	}
}

func TestPoolEmptyIsDisabled(t *testing.T) {
	p := NewPoolFromKeys(Config{}, "", "", nil)
	if p.Enabled() {
		t.Fatal("no keys → disabled")
	}
	if p.For("KR").Enabled() || p.For("US").Enabled() {
		t.Fatal("empty pool clients must not be enabled")
	}
}
