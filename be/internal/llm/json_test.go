package llm

import (
	"encoding/json"
	"testing"
)

// trailingBrace is the shape that failed the 2026-08-19 overseas batch
// (EHGO, AEON): a valid object plus one extra closing brace.
const trailingBrace = `{"valuation":"싸게 보임","technical":"상승 추세","insider":"공시 없음","news":"혼조","newsSentiments":[],"keywords":["반도체"]}}`

func TestDecodeLLMJSONIgnoresTrailingBrace(t *testing.T) {
	var discarded StockSummaryLLMOutput
	if err := json.Unmarshal([]byte(trailingBrace), &discarded); err == nil {
		t.Fatal("json.Unmarshal must reject the trailing brace we saw in production")
	}

	var out StockSummaryLLMOutput
	if err := decodeLLMJSON(trailingBrace, &out); err != nil {
		t.Fatalf("decodeLLMJSON: %v", err)
	}
	if out.Valuation != "싸게 보임" || out.Technical != "상승 추세" {
		t.Fatalf("got valuation=%q technical=%q", out.Valuation, out.Technical)
	}
}

func TestDecodeLLMJSONTable(t *testing.T) {
	const body = `{"valuation":"해석","technical":"추세","insider":"없음","news":"중립","newsSentiments":[{"id":"a1","sentiment":"neutral"}],"keywords":["삼성"]}`

	tests := []struct {
		name    string
		content string
		wantVal string
		wantErr bool
	}{
		{name: "clean", content: body, wantVal: "해석"},
		{name: "trailing braces", content: body + "}}", wantVal: "해석"},
		{name: "trailing prose", content: body + "\n이상입니다.", wantVal: "해석"},
		{name: "leading prose", content: "Here is the JSON:\n" + body, wantVal: "해석"},
		{name: "markdown fence", content: "```json\n" + body + "\n```", wantVal: "해석"},
		{name: "brace inside string", content: `{"valuation":"PER 24}배","technical":"x","insider":"x","news":"x"}`, wantVal: "PER 24}배"},
		{name: "empty", content: "   ", wantErr: true},
		{name: "not json", content: "모델이 JSON을 내지 않음", wantErr: true},
		{name: "truncated", content: `{"valuation":"잘림"`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out StockSummaryLLMOutput
			err := decodeLLMJSON(tt.content, &out)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeLLMJSON: %v", err)
			}
			if out.Valuation != tt.wantVal {
				t.Fatalf("valuation=%q want %q", out.Valuation, tt.wantVal)
			}
		})
	}
}

func TestDecodeLLMJSONBriefing(t *testing.T) {
	raw := `{"summary":"오늘 강세","highlights":["반도체"],"volatility":"높음"}}`
	var out struct {
		Summary    string   `json:"summary"`
		Highlights []string `json:"highlights"`
		Volatility string   `json:"volatility"`
	}
	if err := decodeLLMJSON(raw, &out); err != nil {
		t.Fatalf("decodeLLMJSON: %v", err)
	}
	if out.Summary != "오늘 강세" || out.Volatility != "높음" {
		t.Fatalf("got %+v", out)
	}
}
