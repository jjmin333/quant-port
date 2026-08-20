package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

type Config struct {
	Provider string // "openai" | "gemini" | "" — informational, for admin display
	APIKey   string
	BaseURL  string
	Model    string
}

func (c Config) Enabled() bool {
	return c.APIKey != ""
}

type Client struct {
	cfg        Config
	httpClient *http.Client
	budget     *apibudget.Manager
}

func NewClient(cfg Config, budget *apibudget.Manager) *Client {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Client{
		cfg:        Config{Provider: cfg.Provider, APIKey: cfg.APIKey, BaseURL: base, Model: model},
		httpClient: &http.Client{Timeout: 45 * time.Second},
		budget:     budget,
	}
}

// Provider reports which LLM backend is configured ("openai", "gemini", or
// "" when disabled) — used for admin monitoring, not request behavior.
func (c *Client) Provider() string {
	return c.cfg.Provider
}

// Model reports the configured chat-completions model id, for admin display.
func (c *Client) Model() string {
	return c.cfg.Model
}

// Headroom reports how much of today's provider allowance is still unspent.
// Batch callers size their work list from this instead of discovering the
// ceiling one rejected call at a time.
func (c *Client) Headroom(ctx context.Context) apibudget.Headroom {
	if c == nil || c.budget == nil {
		return apibudget.Headroom{Calls: -1, DayTokens: -1, MinuteTokens: -1}
	}
	return c.budget.Headroom(ctx, apibudget.ProviderOpenAI)
}

// Limits reports the enforced provider quotas, for pacing decisions and admin
// display.
func (c *Client) Limits() apibudget.Limits {
	if c == nil || c.budget == nil {
		return apibudget.Limits{}
	}
	return c.budget.LimitsFor(apibudget.ProviderOpenAI)
}

func (c *Client) Enabled() bool {
	return c.cfg.Enabled()
}

// llmUnavailableRetryWait is how long we pause after a 503 before the one
// allowed retry. Tests shorten this; production stays at the provider's 4s
// spacing so a retry cannot trip our own min-interval or hammer an overloaded
// model.
var llmUnavailableRetryWait = 4 * time.Second

func isLLMUnavailable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "llm api 503")
}

// chatCompletion sends a single system+user message to the chat completions
// endpoint and returns the raw response content plus the tokens the call
// actually cost. Shared by every Client method that talks to the LLM —
// callers own prompt construction and response parsing.
//
// expectedOutput is the caller's ceiling for the completion length, in
// tokens. It only feeds the pre-flight budget check: the provider reports
// real usage afterwards, but a batch has to know *before* sending whether the
// call fits inside the remaining per-minute and per-day token allowance.
//
// A 503 (Gemini "high demand" / UNAVAILABLE) is retried once after a short
// wait. Google's docs treat 503 as capacity, not quota, and our apibudget
// only Record()s HTTP <400 — so neither local RPD nor token counters move on
// the failed attempt. 429 is not retried: that is the quota signal.
func (c *Client) chatCompletion(
	ctx context.Context,
	systemPrompt, userContent string,
	temperature float64,
	jsonMode bool,
	expectedOutput int64,
) (string, int64, error) {
	content, used, err := c.chatCompletionOnce(ctx, systemPrompt, userContent, temperature, jsonMode, expectedOutput)
	if err == nil || !isLLMUnavailable(err) {
		return content, used, err
	}
	wait := llmUnavailableRetryWait
	if c.budget != nil {
		if d := c.budget.MinInterval(apibudget.ProviderOpenAI); d > wait {
			wait = d
		}
	}
	select {
	case <-ctx.Done():
		return "", 0, err
	case <-time.After(wait):
	}
	return c.chatCompletionOnce(ctx, systemPrompt, userContent, temperature, jsonMode, expectedOutput)
}

func (c *Client) chatCompletionOnce(
	ctx context.Context,
	systemPrompt, userContent string,
	temperature float64,
	jsonMode bool,
	expectedOutput int64,
) (string, int64, error) {
	if !c.cfg.Enabled() {
		return "", 0, fmt.Errorf("llm not configured")
	}
	estimate := EstimateTokens(systemPrompt, userContent) + expectedOutput
	if c.budget != nil {
		if err := c.budget.CheckTokens(ctx, apibudget.ProviderOpenAI, estimate); err != nil {
			return "", 0, err
		}
	}

	reqBody := map[string]any{
		"model": c.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userContent},
		},
		"temperature": temperature,
	}
	if jsonMode {
		reqBody["response_format"] = map[string]string{"type": "json_object"}
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", 0, err
	}
	if res.StatusCode == http.StatusTooManyRequests {
		if c.budget != nil {
			c.budget.BlockFor(ctx, apibudget.ProviderOpenAI, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return "", 0, fmt.Errorf("llm api 429: %s", strings.TrimSpace(string(body)))
	}
	if res.StatusCode >= 400 {
		return "", 0, fmt.Errorf("llm api %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	if c.budget != nil {
		c.budget.Record(ctx, apibudget.ProviderOpenAI)
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", 0, err
	}
	// Charge the day's token budget with whatever the provider reported, and
	// with the estimate when it reported nothing — an unmetered call would
	// otherwise let a long batch run past the ceiling unnoticed.
	used := apiResp.Usage.TotalTokens
	if used <= 0 {
		used = estimate
	}
	if c.budget != nil {
		c.budget.RecordTokens(ctx, apibudget.ProviderOpenAI, used)
	}
	if len(apiResp.Choices) == 0 {
		return "", used, fmt.Errorf("llm empty response")
	}
	return apiResp.Choices[0].Message.Content, used, nil
}

// TranslateENToKO translates short financial news text for US feed display.
func (c *Client) TranslateENToKO(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}

	system := "Translate the following US financial news text from English to Korean. " +
		"Preserve company tickers and proper nouns where appropriate. Output only the Korean translation."

	// Korean output runs longer in tokens than the English source (one token
	// per syllable vs ~4 chars per token), so budget for a multiple of the
	// input rather than a flat ceiling.
	content, _, err := c.chatCompletion(ctx, system, text, 0.1, false, EstimateTokens(text)*translationOutputRatio)
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(content)
	if out == "" {
		return "", fmt.Errorf("llm empty translation")
	}
	return out, nil
}

// ArticleSentimentLabel is one news article's LLM-assigned sentiment. Counts
// and averages are computed by the caller from these labels — the LLM never
// reports aggregate numbers itself, so they always match the real article
// count fed into the prompt.
type ArticleSentimentLabel struct {
	ID        string `json:"id"`
	Sentiment string `json:"sentiment"` // "positive" | "negative" | "neutral"
}

// StockSummaryLLMOutput is the per-symbol AI 요약 payload: up to three
// sentences of narrative per section, plus per-article sentiment labels for
// the news section (never an LLM-invented aggregate count).
type StockSummaryLLMOutput struct {
	Valuation      string                  `json:"valuation"`
	Technical      string                  `json:"technical"`
	Insider        string                  `json:"insider"`
	News           string                  `json:"news"`
	NewsSentiments []ArticleSentimentLabel `json:"newsSentiments"`
	Keywords       []string                `json:"keywords"`

	// TotalTokens is what the provider charged for this call, filled in from
	// the response rather than the model's JSON. Batch callers feed it back
	// into their running token estimate.
	TotalTokens int64 `json:"-"`
}

// Expected completion sizes, in tokens, used only for the pre-flight budget
// check. Measured against real responses and rounded up: the AI 요약 emits
// four Korean paragraphs plus one sentiment entry per article, which is by
// far the largest completion this client asks for.
const (
	stockSummaryOutputTokens = 1_200
	translationOutputRatio   = 2
)

// GenerateStockSummary produces the 밸류에이션/기술적분석/내부자거래/뉴스 AI
// 요약 blocks for one symbol from a single call — callers must NOT invoke
// this once per tab; assemble every fact the four sections need into
// factsJSON and cache the result (see StockSummaryService).
func (c *Client) GenerateStockSummary(ctx context.Context, factsJSON, scoreLine string) (*StockSummaryLLMOutput, error) {
	content, used, err := c.chatCompletion(
		ctx,
		stockSummarySystemPrompt,
		stockSummaryUserPrompt(factsJSON, scoreLine),
		0.2,
		true,
		stockSummaryOutputTokens,
	)
	if err != nil {
		return nil, err
	}
	var out StockSummaryLLMOutput
	if err := decodeLLMJSON(content, &out); err != nil {
		return nil, err
	}
	out.TotalTokens = used
	return &out, nil
}

// EstimateStockSummaryTokens reports what one GenerateStockSummary call is
// expected to cost, input plus completion. The batch runner uses it to decide
// whether the next symbol still fits in the day's token budget — the system
// prompt lives here, so the estimate has to live here too.
func (c *Client) EstimateStockSummaryTokens(factsJSON, scoreLine string) int64 {
	return EstimateTokens(stockSummarySystemPrompt, stockSummaryUserPrompt(factsJSON, scoreLine)) +
		stockSummaryOutputTokens
}

func stockSummaryUserPrompt(factsJSON, scoreLine string) string {
	if scoreLine == "" {
		return factsJSON
	}
	// Anchoring the score ahead of the JSON keeps the prose consistent with
	// the number rendered next to it in the UI.
	return "서버 산출 점수: " + scoreLine + "\n" + factsJSON
}

const stockSummarySystemPrompt = `You are a Korean equity research assistant. Use ONLY the JSON facts provided — never invent
numbers, prices, or indicator values that are not present in the facts. If a fact needed for a section is
missing or null, write a shorter sentence that omits it rather than making one up.

The server has already done the arithmetic, classification, aggregation and peer ranking. Do NOT restate
the numbers as a list — the user can already see them. Your job is the part the server cannot do:

1. 해석 — what the figures mean together, not one by one.
2. 우선순위 — lead with whatever most changes the picture for this stock right now.
3. 인과관계 — connect the axes. When "score.axes" disagree (e.g. growth high while valuation is low, or
   strong fundamentals under a bearish trend), that tension IS the story: say which side is driving the
   price and why. "score.strengths"/"score.weaknesses" name the extremes for you.
4. 근거 있는 판단 — a clear closing view, framed as analysis of the current state, never as a
   recommendation to buy or sell and never as a prediction of future price.

Peer percentiles ("...PeerPct", "peerTopPercent") are the stock's rank among comparable companies, where a
higher number means better on that measure. Prefer them over raw ratios when judging cheap/expensive —
"PER 24.9" alone says nothing without the peer context.

"dataGaps" lists what could not be measured. Stay silent on those areas rather than hedging throughout.

Respond with valid JSON only, this exact shape:
{
  "valuation": "실적·수익성·밸류에이션 해석 3문장 이내. 동종 대비 어디에 서 있는지와 그 이유",
  "technical": "가격 흐름 해석 3문장 이내. 추세와 펀더멘털이 엇갈리면 그 이유를 우선 설명",
  "insider": "내부자 거래 동향 3문장 이내. 방향이 실적·주가와 어떻게 맞물리는지",
  "news": "최근 뉴스 흐름 2문장 이내 한국어 요약 (건수·감성 비율은 언급하지 말 것 — 별도로 계산됨)",
  "newsSentiments": [{"id":"<given article id>","sentiment":"positive|negative|neutral"}, ...],
  "keywords": ["종목명 또는 티커, 섹터/테마, 위 4개 요약에서 실제로 언급한 근거 기반 짧은 태그 등 최대 6개, 각 1~2단어"]
}

newsSentiments must contain exactly one entry per article id given in the facts' "newsArticles" list, using
the SAME ids — do not add, remove, or invent ids. keywords must be short (1-2 words each), drawn only from
the facts and the sentences you wrote above — never a theme or strategy not supported by the given data.
This is informational analysis only, not investment advice.`
