package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	// BudgetProvider is the apibudget bucket this client charges. Empty
	// means ProviderOpenAI (shared/legacy key). Split KR/US keys use
	// ProviderLLMKR / ProviderLLMUS so each API key has its own RPD.
	BudgetProvider apibudget.Provider
}

func (c Config) Enabled() bool {
	return strings.TrimSpace(c.APIKey) != ""
}

type Client struct {
	cfg            Config
	httpClient     *http.Client
	budget         *apibudget.Manager
	budgetProvider apibudget.Provider
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
	bp := cfg.BudgetProvider
	if bp == "" {
		bp = apibudget.ProviderOpenAI
	}
	return &Client{
		cfg:            Config{Provider: cfg.Provider, APIKey: cfg.APIKey, BaseURL: base, Model: model, BudgetProvider: bp},
		httpClient:     &http.Client{Timeout: 45 * time.Second},
		budget:         budget,
		budgetProvider: bp,
	}
}

// Provider reports which LLM backend is configured ("openai", "gemini", or
// "" when disabled) — used for admin monitoring, not request behavior.
func (c *Client) Provider() string {
	if c == nil {
		return ""
	}
	return c.cfg.Provider
}

// Model reports the configured chat-completions model id, for admin display.
func (c *Client) Model() string {
	if c == nil {
		return ""
	}
	return c.cfg.Model
}

// BudgetProvider reports which apibudget bucket this client charges.
func (c *Client) BudgetProvider() apibudget.Provider {
	if c == nil {
		return ""
	}
	return c.budgetProvider
}

// Headroom reports how much of today's provider allowance is still unspent.
// Batch callers size their work list from this instead of discovering the
// ceiling one rejected call at a time.
func (c *Client) Headroom(ctx context.Context) apibudget.Headroom {
	if c == nil || c.budget == nil {
		return apibudget.Headroom{Calls: -1, DayTokens: -1, MinuteTokens: -1}
	}
	return c.budget.Headroom(ctx, c.budgetProvider)
}

// Limits reports the enforced provider quotas, for pacing decisions and admin
// display.
func (c *Client) Limits() apibudget.Limits {
	if c == nil || c.budget == nil {
		return apibudget.Limits{}
	}
	return c.budget.LimitsFor(c.budgetProvider)
}

func (c *Client) Enabled() bool {
	return c != nil && c.cfg.Enabled()
}

// llmUnavailableRetryWait is how long we pause after a 503 before the one
// allowed retry. Tests shorten this; production stays at the provider's 4s
// spacing so a retry cannot trip our own min-interval or hammer an overloaded
// model.
var llmUnavailableRetryWait = 4 * time.Second

func isLLMUnavailable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "llm api 503")
}

// IsBudgetExhausted reports the failures that mean "the allowance is spent" —
// our own day/minute quotas, the token ceilings, and the provider's 429 (plus
// the block it puts us in). A caller must **not** retry these: the answer will
// be the same, and if it were not, the retry would be spending an allowance we
// deliberately withheld.
func IsBudgetExhausted(err error) bool {
	return errors.Is(err, apibudget.ErrExhausted)
}

// IsUnavailable reports the provider-capacity 503. chatCompletion already
// retried it once on the caller's behalf, so a caller-level retry on top would
// make it four attempts for one slot.
func IsUnavailable(err error) bool { return isLLMUnavailable(err) }

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
		if d := c.budget.MinInterval(c.budgetProvider); d > wait {
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
		if err := c.budget.CheckTokens(ctx, c.budgetProvider, estimate); err != nil {
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
			c.budget.BlockFor(ctx, c.budgetProvider, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return "", 0, apibudget.Exhaustedf("llm api 429: %s", strings.TrimSpace(string(body)))
	}
	if res.StatusCode >= 400 {
		return "", 0, fmt.Errorf("llm api %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	if c.budget != nil {
		c.budget.Record(ctx, c.budgetProvider)
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
		c.budget.RecordTokens(ctx, c.budgetProvider, used)
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
	// Overview is the 개요 탭 headline — written from all four sections at
	// once, in the same call, so the summary a user sees first is not just the
	// valuation paragraph wearing a different label.
	Overview       string                  `json:"overview"`
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
// five Korean paragraphs (개요 + four sections) plus one sentiment entry per
// article, which is by far the largest completion this client asks for.
//
// 1,200 → 1,400: the 개요 문단이 2~3문장 늘었다. 한국어는 음절당 토큰이라 한
// 문단이 대략 120~150 토큰이고, 여유를 보태 200을 올린다. 이 값은 **예산
// 사전 점검에만** 쓰이므로 넉넉한 쪽이 안전하다 — 모자라면 남은 한도를 실제보다
// 크게 보고 배치가 한도 끝에서 거절당한다.
const (
	stockSummaryOutputTokens = 1_400
	translationOutputRatio   = 2
)

// GenerateStockSummary produces the 밸류에이션/기술적분석/내부자거래/뉴스 AI
// 요약 blocks for one symbol from a single call — callers must NOT invoke
// this once per tab; assemble every fact the four sections need into
// factsJSON and cache the result (see StockSummaryService).
func (c *Client) GenerateStockSummary(ctx context.Context, factsJSON, scoreLine string) (*StockSummaryLLMOutput, error) {
	_, out, err := c.GenerateStockSummaryRaw(ctx, factsJSON, scoreLine)
	return out, err
}

// GenerateStockSummaryRaw returns the provider completion before parsing, plus
// the decoded payload. Callers that only need the prose can use
// GenerateStockSummary; admin/dry-run tooling uses the raw string for auditing.
func (c *Client) GenerateStockSummaryRaw(ctx context.Context, factsJSON, scoreLine string) (raw string, out *StockSummaryLLMOutput, err error) {
	content, used, err := c.chatCompletion(
		ctx,
		stockSummarySystemPrompt,
		stockSummaryUserPrompt(factsJSON, scoreLine),
		0.2,
		true,
		stockSummaryOutputTokens,
	)
	if err != nil {
		return "", nil, err
	}
	var parsed StockSummaryLLMOutput
	if err := decodeLLMJSON(content, &parsed); err != nil {
		return content, nil, err
	}
	parsed.TotalTokens = used
	return content, &parsed, nil
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

Every peer figure in the facts — "peerTopPercent" and each "...TopPercent" — is already expressed as
상위 N% among comparable companies: 1 is the best of the peer group and 100 the worst, so a LOWER number
means better. Write each one in Korean exactly as given ("상위 6%"): never subtract it from 100, never
re-rank it, and never turn 상위 6% into 상위 94% or the reverse. Prefer these over raw ratios when judging
cheap/expensive — "PER 24.9" alone says nothing without the peer context.

"dataGaps" lists what could not be measured. Stay silent on those areas rather than hedging throughout.

Respond with valid JSON only, this exact shape:
{
  "overview": "위 네 갈래(밸류에이션·기술적 분석·내부자 거래·뉴스)를 **모두 고려한** 종합 요약 2~3문장. 개요 화면의 첫 문단이므로, 한 축만 골라 말하지 말고 지금 이 종목의 상태를 가장 크게 좌우하는 것부터 적고 축들이 엇갈리면 그 긴장을 드러낼 것",
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
