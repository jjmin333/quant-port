package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
봇 초안 LLM 다시 쓰기 (2026-10-19 사용자 요청) — 머리글은 bot_drafts.go.

검증이 이 파일의 핵심이다. 10-19 시험에서 모델은 사람이 쓴 것 같은 글을 썼지만, 서로 다른 날의 등락률을 섞고
사실에 없는 차트 묘사(「윗꼬리」)를 지어냈다. 프롬프트로 부탁하는 것만으로는 막지 못한다 — 출력의 숫자가 전부
입력 사실에 있는지 세고, 권유 표현을 찾고, 길이를 본다. 하나라도 걸리면 그 글은 버리고 템플릿을 둔다.
*/

const (
	// botRewriteWindow — 이 안에 만든 템플릿 초안을 이어 쓴다(밤 503 회차를 아침 회차가 메운다).
	botRewriteWindow = 12 * time.Hour
	// botRewriteMaxPerRun — 회차당 상한. 평소 7건 + 밀린 회차 몇 개. 기능 버킷(하루 80)이 최종 상한이다.
	botRewriteMaxPerRun = 25
)

type botRewriteResult struct {
	Rewritten, Rejected int
	Tokens              int64
	Stop                string // "" | no_llm | budget | unavailable
}

func (r botRewriteResult) stopText() string {
	switch r.Stop {
	case "no_llm":
		return " (LLM 키 없음 — 템플릿만)"
	case "budget":
		return " (예산 소진 — 나머지는 템플릿)"
	case "unavailable":
		return " (LLM 503·시간 초과 — 다음 회차가 이어 씀)"
	}
	return ""
}

func (s *BotDraftService) rewritePending(ctx context.Context, now time.Time) botRewriteResult {
	var res botRewriteResult
	if s.llm == nil || !s.llm.Enabled() || s.budget == nil {
		res.Stop = "no_llm"
		return res
	}
	pending, err := s.repo.PendingRewrites(ctx, now.Add(-botRewriteWindow), botRewriteMaxPerRun)
	if err != nil {
		log.Printf("bot drafts: pending rewrites: %v", err)
		return res
	}
	interval := s.llm.Limits().MinInterval
	var last time.Time
	for _, d := range pending {
		if wait := interval - time.Since(last); !last.IsZero() && wait > 0 && !s.sleep(ctx, wait) {
			break
		}
		system, input := botRewritePrompt(d)
		if err := s.budget.CheckTokens(ctx, apibudget.ProviderLLMFeatBot, llm.EstimateBotDraftTokens(system, input)); err != nil {
			res.Stop = "budget"
			break
		}
		out, used, err := s.llm.RewriteBotDraft(ctx, system, input)
		// 간격은 응답이 **끝난** 뒤부터 잰다 — 예산의 최소 간격(llm_sub 4초)이 마지막 기록 시각(응답 뒤)을 보기 때문.
		// 10-19 첫 실행에서 시작 시각으로 쟀더니 응답이 4초를 넘긴 다음 콜이 하나 걸러 ErrPacing으로 거부됐다.
		last = time.Now()
		if errors.Is(err, apibudget.ErrPacing) {
			// 간격에 걸린 것은 글의 문제가 아니다 — 메모 없이 두면 다음 회차가 다시 쓴다.
			continue
		}
		if used > 0 {
			s.budget.Record(ctx, apibudget.ProviderLLMFeatBot)
			s.budget.RecordTokens(ctx, apibudget.ProviderLLMFeatBot, used)
			res.Tokens += used
		}
		if err != nil {
			switch {
			case llm.IsBudgetExhausted(err):
				res.Stop = "budget"
			case llm.IsTransient(err):
				res.Stop = "unavailable"
			default:
				// 형식이 깨진 응답 — 같은 사실로 다시 불러도 비슷하다. 메모를 남겨 다시 시도하지 않는다.
				log.Printf("bot drafts: rewrite %d (%s %s): %v", d.ID, d.Kind, d.Length, err)
				_ = s.repo.SetNote(ctx, d.ID, "AI 응답 오류: "+truncateNote(err.Error()))
				res.Rejected++
				continue
			}
			break
		}
		body, note := validateBotRewrite(d, out)
		if note != "" {
			_ = s.repo.SetNote(ctx, d.ID, note)
			res.Rejected++
			continue
		}
		if err := s.repo.ApplyRewrite(ctx, d.ID, out.Title, body); err != nil {
			log.Printf("bot drafts: apply rewrite %d: %v", d.ID, err)
			continue
		}
		res.Rewritten++
	}
	return res
}

// botPickPersona — 아무 게시 계정이나 하나. 계정이 없으면 nil(게시할 때 고른다).
func botPickPersona(rnd *rand.Rand, personas []repository.BotPersona) *repository.BotPersona {
	if len(personas) == 0 {
		return nil
	}
	p := personas[rnd.IntN(len(personas))]
	return &p
}

// botRewriteRules — 모델에 그대로 간다. 둘째 줄(요일·시간대)은 10-20 실측: 평일 글이 「주말을 맞아」로 시작했다
// (유형 설명의 「주말에 공들여 쓰는」을 글 내용으로 옮겼다).
const botRewriteRules = `규칙:
- 「어떤 사람」 설명은 말투만 정한다. 요일·시간대·상황(주말, 점심시간, 출근길 등)은 사실에 없으면 글에 쓰지 않는다.
- 사실을 다 쓸 필요는 없다. 이 작성자라면 실제로 언급할 만큼만 골라 쓴다(한 줄 글이면 하나도 충분하다).
- 「사실」에 적힌 숫자·사건·날짜만 쓴다. 새 숫자(가격·비율·배수·날짜·순위·전망치)를 만들지 않는다. 숫자는 적힌 그대로 옮기고 반올림하지 않는다.
- 사실에 없는 차트 모양(꼬리·지지선 등)·호가·소문·개인 경험(샀다·팔았다·물렸다)을 지어내지 않는다.
- 날짜가 붙은 사건은 그 날의 일로 쓴다. 서로 다른 날의 숫자를 한 문장에 섞지 않는다.
- 매수·매도 권유, 목표가, 수익 보장, 「지금 사라/팔아라」 같은 표현을 쓰지 않는다. 의견은 「~로 보인다」, 「궁금하다」 정도로.
- AI가 쓴 티를 내지 않는다: 「~을 보여주고 있습니다」 반복, 「결론적으로」, 「요약하자면」, 이모지, 과한 칭찬, 모든 글을 질문으로 끝내기.
- 욕설·비하·특정인 공격은 쓰지 않는다.
- 출력은 JSON 하나: {"title": "...", "body": "..."}`

func botRewritePrompt(d repository.BotDraft) (system, input string) {
	a := botArchetypeOf(d.Kind, d.Archetype, d.Length)
	where := "주식 커뮤니티 게시판 글. 본문은 일반 텍스트(마크다운·#·@ 기호 금지)."
	if d.Kind == repository.BotDraftResearch {
		where = "투자 리포트 게시판 글. 본문은 마크다운. 매수·매도 방향과 목표가는 쓰지 않는다 — 게시하는 사람이 정한다."
	}
	system = "너는 한국 주식 커뮤니티의 수많은 사용자 중 한 명이다. 이번 글을 쓰는 사람은 아래와 같다. 주어진 사실만으로 그 사람이 실제로 쓸 법한 글을 쓴다.\n" +
		"어떤 사람: " + a.Who + "\n어디에: " + where + "\n무엇을 얼마나: " + a.Form + "\n\n" + botRewriteRules
	input = fmt.Sprintf("종목: %s (%s)\n고른 이유: %s\n\n사실:\n%s", d.SymbolName, d.Symbol, d.Reason, d.Facts)
	return system, input
}

var botNumberRe = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?`)

// botBannedPhrases — 권유·보장 표현. 사실 자체에 있으면(예: 공시 제목) 허용한다.
var botBannedPhrases = []string{
	"매수 추천", "매도 추천", "매수하세요", "매도하세요", "사세요", "파세요", "목표가", "목표 주가",
	"수익 보장", "무조건", "강력 매수", "적극 매수", "풀매수", "몰빵",
}

func botNumbers(s string) []string {
	out := []string{}
	for _, m := range botNumberRe.FindAllString(s, -1) {
		out = append(out, strings.TrimRight(strings.ReplaceAll(m, ",", ""), "."))
	}
	return out
}

/*
validateBotRewrite — LLM 출력을 받을지. 받으면 (정리한 본문, ""), 버리면 ("", 이유).

숫자: 출력의 모든 숫자가 입력(사실·고른 이유·종목명·코드)에 있어야 한다. 한 자리 정수만 봐준다(「3가지」, 「2번째」).
기호: 커뮤니티 본문의 #·@는 태그로 읽히니 전각으로 바꾸고, 리포트는 @만 바꾼다(## 소제목은 살린다).
*/
func validateBotRewrite(d repository.BotDraft, out llm.BotDraftRewrite) (string, string) {
	a := botArchetypeOf(d.Kind, d.Archetype, d.Length)
	minBody, maxBody, maxTitle := a.MinBody, a.MaxBody, 60
	if d.Kind == repository.BotDraftResearch {
		maxTitle = 100
	}
	if n := utf8.RuneCountInString(out.Title); n < 2 || n > maxTitle {
		return "", fmt.Sprintf("제목 길이 %d자", n)
	}
	if n := utf8.RuneCountInString(out.Body); n < minBody || n > maxBody {
		return "", fmt.Sprintf("본문 길이 %d자", n)
	}
	source := d.Facts + "\n" + d.Reason + "\n" + d.SymbolName + "\n" + d.Symbol
	known := map[string]bool{}
	for _, n := range botNumbers(source) {
		known[n] = true
	}
	for _, n := range botNumbers(out.Title + "\n" + out.Body) {
		if known[n] || (len(n) == 1) {
			continue
		}
		return "", "사실에 없는 숫자: " + n
	}
	for _, w := range botBannedPhrases {
		if strings.Contains(out.Title+out.Body, w) && !strings.Contains(source, w) {
			return "", "금지 표현: " + w
		}
	}
	body := out.Body
	if d.Kind == repository.BotDraftCommunity {
		body = botStripTags(body)
	} else {
		body = strings.ReplaceAll(body, "@", "＠")
	}
	return body, ""
}

func truncateNote(s string) string {
	if r := []rune(s); len(r) > 120 {
		return string(r[:120]) + "…"
	}
	return s
}
