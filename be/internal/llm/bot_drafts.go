package llm

import (
	"context"
	"strings"
)

/*
봇 초안 다시 쓰기 (2026-10-19 사용자 요청) — 템플릿 초안(=사실 목록)을 게시 계정의 말투로 사람이 쓴 글처럼 다시 쓴다.
키는 자산 전용(GEMINI_SUB_API_KEY, 버킷 llm_sub) + 기능 버킷 llm_feat_bot. 프롬프트는 service/bot_drafts.go가 만든다 —
사실 밖 숫자·권유 금지는 프롬프트로 부탁하고, 지켰는지는 서비스가 출력에서 다시 검사한다(모델을 믿지 않는다).
*/

// botDraftOutputTokens — 시험 실측 275~665(10-19). 리포트 긴 글(1,200~2,500자)이 상한에 잘리면 JSON이 깨지므로
// 기대치를 넉넉히 두고, 상한은 그 두 배.
const botDraftOutputTokens = 2200

type BotDraftRewrite struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// RewriteBotDraft — 한 건. 온도는 높게(같은 사실도 글마다 다르게 읽혀야 한다).
func (c *Client) RewriteBotDraft(ctx context.Context, system, input string) (BotDraftRewrite, int64, error) {
	var out BotDraftRewrite
	content, used, err := c.chatCompletion(WithMaxTokens(ctx, botDraftOutputTokens*2), system, input, 0.9, true, botDraftOutputTokens)
	if err != nil {
		return out, used, err
	}
	err = decodeLLMJSON(content, &out)
	out.Title, out.Body = strings.TrimSpace(out.Title), strings.TrimSpace(out.Body)
	return out, used, err
}

func EstimateBotDraftTokens(system, input string) int64 {
	return EstimateTokens(system, input) + botDraftOutputTokens
}
