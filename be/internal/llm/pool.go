package llm

import (
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

// Region identifies which Gemini/OpenAI API key a call should charge.
// New LLM features must pick a region (국내 vs 해외) rather than a shared key.
const (
	RegionKR = "kr"
	RegionUS = "us"
)

// Pool holds the per-market LLM clients. Call For(market) at every spend
// site — stock AI 요약, news briefings, and anything added later. When KR
// and US resolve to the same API key they share one Client and one
// apibudget bucket (ProviderOpenAI); distinct keys get llm_kr / llm_us.
type Pool struct {
	kr *Client
	us *Client
}

// NewPoolFromKeys wires one or two clients from the resolved regional keys.
// Empty keys disable that region. Identical (or only-one-set) keys share a
// client so Google's per-key quota is not double-counted locally.
func NewPoolFromKeys(cfg Config, keyKR, keyUS string, budget *apibudget.Manager) *Pool {
	keyKR = strings.TrimSpace(keyKR)
	keyUS = strings.TrimSpace(keyUS)
	makeClient := func(key string, bp apibudget.Provider) *Client {
		c := cfg
		c.APIKey = key
		c.BudgetProvider = bp
		return NewClient(c, budget)
	}
	if keyKR == "" && keyUS == "" {
		return &Pool{}
	}
	if keyKR != "" && keyUS != "" && keyKR != keyUS {
		return &Pool{
			kr: makeClient(keyKR, apibudget.ProviderLLMKR),
			us: makeClient(keyUS, apibudget.ProviderLLMUS),
		}
	}
	key := keyKR
	if key == "" {
		key = keyUS
	}
	shared := makeClient(key, apibudget.ProviderOpenAI)
	return &Pool{kr: shared, us: shared}
}

// RegionOf maps a market id onto KR or US. Accepts the values already in
// circulation: toss.MarketCountry ("KR"/"US"), news briefing ("domestic"/"us").
// Unknown values default to KR so a new caller cannot silently bill the US key.
func RegionOf(market string) string {
	switch strings.ToLower(strings.TrimSpace(market)) {
	case "us", "overseas":
		return RegionUS
	default:
		return RegionKR
	}
}

// For returns the client that should pay for this market. Nil-safe: a
// missing pool or disabled region yields a nil *Client (Enabled() == false).
func (p *Pool) For(market string) *Client {
	if p == nil {
		return nil
	}
	if RegionOf(market) == RegionUS {
		return p.us
	}
	return p.kr
}

func (p *Pool) Enabled() bool {
	return p != nil && (p.kr.Enabled() || p.us.Enabled())
}

func (p *Pool) Provider() string {
	if p == nil {
		return ""
	}
	if p.kr != nil && p.kr.Provider() != "" {
		return p.kr.Provider()
	}
	if p.us != nil {
		return p.us.Provider()
	}
	return ""
}

// Split reports whether KR and US charge distinct API keys (and therefore
// distinct apibudget buckets). Briefing slots that would otherwise wait on
// a shared 4s min-interval can skip that wait when Split is true.
func (p *Pool) Split() bool {
	return p != nil && p.kr != nil && p.us != nil && p.kr != p.us
}

// SharedBudget is the inverse of Split — two markets must not fire back to
// back against the same key without honouring MinInterval.
func (p *Pool) SharedBudget() bool {
	return p == nil || !p.Split()
}
