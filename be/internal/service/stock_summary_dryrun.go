package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// StockSummaryLLMDryRun captures one LLM generation without persisting anything.
// Used by admin tooling to inspect raw completions against stored narratives.
type StockSummaryLLMDryRun struct {
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	ScoreLine string `json:"scoreLine"`
	FactsJSON string `json:"factsJson"`
	Raw       string `json:"rawCompletion"`
	Tokens    int64  `json:"tokens"`

	// Parsed is the JSON object decoded from Raw.
	Parsed *llm.StockSummaryLLMOutput `json:"parsed"`

	// StoredFields is what storeNarrative would write (prose + aggregated news).
	StoredFields repository.StockSummaryNarrative `json:"storedFields"`

	// ViewFields is what applyStockSummaryLLMOutput overlays onto StockSummaryView.
	ViewFields struct {
		Overview  string                      `json:"overview"`
		Valuation string                      `json:"valuation"`
		Technical string                      `json:"technical"`
		Insider   string                      `json:"insider"`
		News      string                      `json:"news"`
		Keywords  []string                    `json:"keywords,omitempty"`
		Sentiment *model.NewsSentimentSummary `json:"newsSentiment,omitempty"`
	} `json:"viewFields"`

	// CurrentlyStored is the narrative already on disk, unchanged by this call.
	CurrentlyStored *repository.StockSummaryNarrative `json:"currentlyStored,omitempty"`
}

// DryRunLLM assembles facts, calls the LLM once, and reports what would be
// stored — without writing Redis, MySQL, or the request log.
func (s *StockSummaryService) DryRunLLM(ctx context.Context, symbol string) (*StockSummaryLLMDryRun, error) {
	stock, err := s.stocks.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if stock == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownSymbol, symbol)
	}
	assetType := ResolveFundamentalsAssetType(stock)

	var fundamentals *model.FundamentalsView
	if s.fundamentals != nil {
		fundamentals, _ = s.fundamentals.GetWithValuation(ctx, stock.Symbol)
	}
	history := s.recentHistory(ctx, stock.Symbol)
	insiderTxns := s.recentInsider(ctx, stock.Symbol)
	technical := s.technicalIndicators(ctx, stock.Symbol)
	articles := s.recentArticles(ctx, stock, assetType)

	var scoreView *model.StockScoreView
	if s.scores != nil {
		scoreView, _ = s.scores.Get(ctx, stock.Symbol)
	}

	client := s.llmForStock(stock)
	if client == nil || !client.Enabled() {
		return nil, fmt.Errorf("llm not configured")
	}

	facts := buildStockSummaryFacts(stock, fundamentals, history, insiderTxns, technical, articles, scoreView)
	factsJSON, err := json.Marshal(facts)
	if err != nil {
		return nil, err
	}
	scoreLine := describeScoreForPrompt(facts)

	raw, parsed, err := client.GenerateStockSummaryRaw(ctx, string(factsJSON), scoreLine)
	if err != nil {
		return nil, err
	}

	out := &StockSummaryLLMDryRun{
		Symbol:    stock.Symbol,
		Name:      stock.Name,
		ScoreLine: scoreLine,
		FactsJSON: string(factsJSON),
		Raw:       raw,
		Tokens:    parsed.TotalTokens,
		Parsed:    parsed,
	}

	view := &model.StockSummaryView{}
	applyStockSummaryLLMOutput(view, parsed, articles)
	out.ViewFields.Overview = view.Overview
	out.ViewFields.Valuation = view.Valuation
	out.ViewFields.Technical = view.Technical
	out.ViewFields.Insider = view.Insider
	out.ViewFields.News = view.News
	out.ViewFields.Keywords = view.Keywords
	out.ViewFields.Sentiment = view.NewsSentiment

	out.StoredFields = repository.StockSummaryNarrative{
		Overview:      view.Overview,
		Valuation:     view.Valuation,
		Technical:     view.Technical,
		Insider:       view.Insider,
		News:          view.News,
		Keywords:      view.Keywords,
		NewsSentiment: view.NewsSentiment,
	}

	if s.narratives != nil {
		out.CurrentlyStored, _ = s.narratives.Get(ctx, symbol)
	}
	return out, nil
}
