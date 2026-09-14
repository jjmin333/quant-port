package service

import (
	"context"
	"fmt"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/model"
)

// NewsBriefingDryRun captures one LLM briefing generation without persisting.
type NewsBriefingDryRun struct {
	Market          string                      `json:"market"`
	MarketLabel     string                      `json:"marketLabel"`
	FactsJSON       string                      `json:"factsJson"`
	Raw             string                      `json:"rawCompletion"`
	Tokens          int64                       `json:"tokens"`
	Parsed          *llm.NewsBriefingLLMOutput  `json:"parsed"`
	StoredFields    newsBriefingStoredFields    `json:"storedFields"`
	CurrentlyStored *model.NewsMarketBriefing   `json:"currentlyStored,omitempty"`
}

type newsBriefingStoredFields struct {
	Source      string   `json:"source"`
	Summary     string   `json:"summary"`
	Mood        string   `json:"mood"`
	MoodReason  string   `json:"moodReason"`
	Highlights  []string `json:"highlights,omitempty"`
	WatchPoints []string `json:"watchPoints,omitempty"`
}

// DryRunBriefing assembles facts, calls the LLM once, and reports what would
// be stored — without writing MySQL or spending a request log entry.
func (s *NewsBriefingService) DryRunBriefing(ctx context.Context, market string) (*NewsBriefingDryRun, error) {
	if s == nil || s.news == nil {
		return nil, fmt.Errorf("news briefing service not configured")
	}
	market = normalizeBriefingMarket(market)
	if market == "" {
		return nil, fmt.Errorf("unknown market")
	}

	facts := s.buildFacts(ctx, market)
	if facts.ArticleCount == 0 {
		return nil, fmt.Errorf("%s 뉴스 기사가 없어 브리핑을 만들 수 없습니다", briefingMarketLabel(market))
	}

	client := s.llmForMarket(market)
	if client == nil || !client.Enabled() {
		return nil, fmt.Errorf("llm not configured")
	}

	var history []model.NewsBriefingHistoryEntry
	if s.repo != nil {
		history, _ = s.repo.History(ctx, market, briefingHistoryCount, briefingHistoryChars)
	}
	earnings := s.briefingEarnings(ctx, market)
	var flow *briefingFlowDTO
	if market != "us" {
		flow = s.briefingInvestorFlow(ctx)
	}
	factsJSON, err := s.promptJSON(market, facts, history, earnings, flow)
	if err != nil {
		return nil, err
	}

	raw, parsed, err := client.GenerateNewsBriefingRaw(ctx, factsJSON)
	if err != nil {
		return nil, err
	}

	out := &NewsBriefingDryRun{
		Market:      market,
		MarketLabel: briefingMarketLabel(market),
		FactsJSON:   factsJSON,
		Raw:         raw,
		Tokens:      parsed.TotalTokens,
		Parsed:      parsed,
		StoredFields: newsBriefingStoredFields{
			Source:      model.BriefingSourceLLM,
			Summary:     parsed.Summary,
			Mood:        parsed.Mood,
			MoodReason:  parsed.MoodReason,
			Highlights:  parsed.Highlights,
			WatchPoints: parsed.WatchPoints,
		},
	}
	if s.repo != nil {
		out.CurrentlyStored, _ = s.repo.Latest(ctx, market)
	}
	return out, nil
}

func (s *NewsBriefingService) llmForMarket(market string) *llm.Client {
	if s == nil || s.llm == nil {
		return nil
	}
	return s.llm.For(market)
}

func normalizeBriefingMarket(market string) string {
	switch market {
	case "domestic", "kr", "KR":
		return "domestic"
	case "us", "overseas", "US":
		return "us"
	default:
		return ""
	}
}
