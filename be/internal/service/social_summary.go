package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
SocialSummaryService — 종목 커뮤니티 탭 「AI 핵심 요약」 배치 (2026-09-29).

매일 04:20 KST — 글 적재(00:30~02:00) 뒤이고 Gemini 밤 503 구간(22~03시)을 피한다. 언급 상위 40종목,
기능 버킷 llm_feat_social. 같은 스냅샷 날짜로 AI 글이 이미 있으면 다시 쓰지 않는다. 한도가 없으면
규칙 문장으로 저장한다(점수·숫자는 늘 서버 값). 화면은 저장분만 읽는다(llm_spend_rule).
*/
type SocialSummaryService struct {
	repo   *repository.ExternalSignalRepo
	llm    *llm.Pool
	budget *apibudget.Manager
	runs   *repository.BatchJobRunRepo
	now    func() time.Time
}

const (
	socialSummaryMax        = 40
	socialSummaryPosts      = 10
	socialSummarySlotHour   = 4
	socialSummarySlotMinute = 20
)

func NewSocialSummaryService(repo *repository.ExternalSignalRepo, pool *llm.Pool, budget *apibudget.Manager) *SocialSummaryService {
	return &SocialSummaryService{repo: repo, llm: pool, budget: budget, now: time.Now}
}

func (s *SocialSummaryService) SetBatchRuns(r *repository.BatchJobRunRepo) { s.runs = r }

// Run enqueues the daily slot (the queue worker in server.go runs it).
func (s *SocialSummaryService) Run(ctx context.Context) {
	if s.runs == nil {
		return
	}
	gate := NewSlotGate()
	for {
		k := s.now().In(kstLoc())
		slot := time.Date(k.Year(), k.Month(), k.Day(), socialSummarySlotHour, socialSummarySlotMinute, 0, 0, kstLoc())
		if !slot.After(k) {
			slot = slot.AddDate(0, 0, 1)
		}
		if !WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		id := string(model.BatchSocialSummary)
		if busy, err := s.runs.HasPending(ctx, id); err == nil && !busy {
			if _, err := s.runs.EnqueueAs(ctx, id, repository.TriggerSchedule, nil); err != nil {
				log.Printf("social summary: enqueue: %v", err)
			}
		}
	}
}

type socialFactsPost struct {
	Subreddit string `json:"subreddit"`
	Date      string `json:"date"`
	Title     string `json:"title"`
	Excerpt   string `json:"excerpt,omitempty"`
}

type socialFacts struct {
	Symbol          string            `json:"symbol"`
	Name            string            `json:"name"`
	Date            string            `json:"date"`
	CommunityScore  int               `json:"communityScore"`
	Stance          string            `json:"stance"`
	Mentions24h     int               `json:"mentions24h"`
	MentionsPrev24h int               `json:"mentionsPrev24h"`
	Rank            int               `json:"rank"`
	RankPrev        *int              `json:"rankPrev,omitempty"`
	Upvotes         int               `json:"upvotes"`
	Users           int               `json:"users"`
	SentimentPct    int               `json:"sentimentPct"`
	WSBSentimentPct *int              `json:"wsbSentimentPct,omitempty"`
	Keywords        []string          `json:"keywords,omitempty"`
	Subreddits      map[string]int    `json:"subredditMentions,omitempty"`
	Posts           []socialFactsPost `json:"posts,omitempty"`
}

// SocialSummaryBody is what the tab shows.
type SocialSummaryBody struct {
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
	Bull    []string `json:"bull,omitempty"`
	Bear    []string `json:"bear,omitempty"`
	Stance  string   `json:"stance"`
}

// RunBatchNow writes summaries for the latest snapshot's top symbols.
func (s *SocialSummaryService) RunBatchNow(ctx context.Context) (map[string]any, string, error) {
	date, err := s.repo.LatestSocialDate(ctx)
	if err != nil || date == "" {
		return nil, "", fmt.Errorf("social summary: no snapshot (%v)", err)
	}
	top, err := s.repo.SocialTopOn(ctx, date, 120)
	if err != nil {
		return nil, "", err
	}
	var client *llm.Client
	if s.llm != nil {
		client = s.llm.For("us")
	}
	llmOn := client != nil && client.Enabled()
	var interval time.Duration
	if llmOn {
		interval = client.Limits().MinInterval
	}
	var nLLM, nRule, nSkip, nFail, targets int
	var tokens int64
	var lastCall time.Time
	// process — 한 항목(종목 또는 시장). false면 회차를 멈춘다(취소).
	process := func(facts socialFacts) bool {
		prev, err := s.repo.SocialSummaryOf(ctx, facts.Symbol)
		if err != nil {
			nFail++
			return true
		}
		if prev != nil && prev.SnapDate == date && prev.Source == "llm" {
			nSkip++
			return true
		}
		factsJSON, _ := json.Marshal(facts)
		sum := sha256.Sum256(factsJSON)
		body := RuleSocialSummary(facts)
		source, modelName := "rule", ""
		var used int64
		if llmOn {
			if wait := interval - time.Since(lastCall); !lastCall.IsZero() && wait > 0 && !sleepCtx(ctx, wait) {
				return false
			}
			if err := s.budget.CheckTokens(ctx, apibudget.ProviderLLMFeatSocial, client.EstimateSocialSummaryTokens(string(factsJSON))); err != nil {
				log.Printf("social summary: feature budget — %v (나머지는 규칙 문장)", err)
				llmOn = false
			} else {
				lastCall = time.Now()
				res, err := client.GenerateSocialSummary(ctx, string(factsJSON))
				// 키 버킷 간격은 뉴스 브리핑 등과 나눠 쓴다 — 남이 방금 불렀으면 한 번 기다렸다 다시(09-29 첫 실행: 21건 중 20건이 이것).
				if err != nil && isBudgetOr429(err) && sleepCtx(ctx, interval+time.Second) {
					lastCall = time.Now()
					res, err = client.GenerateSocialSummary(ctx, string(factsJSON))
				}
				switch {
				case err != nil:
					log.Printf("social summary %s llm: %v", facts.Symbol, err)
					if llm.IsBudgetExhausted(err) {
						llmOn = false
					}
				default:
					s.budget.Record(ctx, apibudget.ProviderLLMFeatSocial)
					s.budget.RecordTokens(ctx, apibudget.ProviderLLMFeatSocial, res.TotalTokens)
					if ValidSocialSummary(res) {
						body = SocialSummaryBody{Summary: res.Summary, Tags: res.Tags, Bull: res.Bull, Bear: res.Bear, Stance: facts.Stance}
						source, modelName, used = "llm", res.Model, res.TotalTokens
					} else {
						log.Printf("social summary %s: 형식이 맞지 않아 규칙 문장으로", facts.Symbol)
					}
				}
			}
		}
		raw, _ := json.Marshal(body)
		if err := s.repo.UpsertSocialSummary(ctx, repository.SocialSummary{
			Symbol: facts.Symbol, SnapDate: date, Score: facts.CommunityScore, Body: raw, Source: source,
			Model: modelName, InputHash: hex.EncodeToString(sum[:]), Tokens: used,
		}); err != nil {
			nFail++
			return true
		}
		tokens += used
		if source == "llm" {
			nLLM++
		} else {
			nRule++
		}
		return true
	}

	// 시장 「미국 개미 분위기」 한 줄 먼저(2026-10-01 요청 3) — 하루 1콜, 종목 40개 몫과 별개로 센다.
	market := 0
	if m, err := BuildMarketMood(ctx, s.repo); err == nil && m != nil && m.Date == date {
		if mf, ok := marketFacts(m); ok {
			market = 1
			if !process(mf) {
				return nil, "", ctx.Err()
			}
		}
	}
	for _, t := range top {
		if targets >= socialSummaryMax || ctx.Err() != nil {
			break
		}
		if socialTickerDenylist[t.Symbol] {
			continue
		}
		facts, ok := s.facts(ctx, t, date)
		if !ok {
			continue
		}
		targets++
		if !process(facts) {
			break
		}
	}
	msg := fmt.Sprintf("social_summary: %s 기준 %d종목 + 시장 %d — AI %d · 규칙 %d · 그대로 %d · 실패 %d (토큰 %d)",
		date, targets, market, nLLM, nRule, nSkip, nFail, tokens)
	return map[string]any{"date": date, "targets": targets, "market": market, "llm": nLLM, "rule": nRule, "skipped": nSkip,
		"failed": nFail, "tokens": tokens}, msg, nil
}

// facts builds one symbol's table. 커뮤니티 점수를 못 매기면(언급 사용자 5명 미만) 건너뛴다.
func (s *SocialSummaryService) facts(ctx context.Context, t repository.SocialTop, date string) (socialFacts, bool) {
	ins, err := s.repo.SocialInsightLatest(ctx, t.Symbol)
	if err != nil || ins == nil || ins.SnapDate != date {
		return socialFacts{}, false
	}
	score, ok := CommunityScore(ins.SentimentPct, ins.Users)
	if !ok {
		return socialFacts{}, false
	}
	f := socialFacts{
		Symbol: t.Symbol, Name: t.Name, Date: date, CommunityScore: score, Stance: CommunityStance(score),
		Mentions24h: t.Mentions, MentionsPrev24h: t.MentionsPrev, Rank: t.Rank, RankPrev: t.RankPrev,
		Upvotes: t.Upvotes, Users: *ins.Users, SentimentPct: *ins.SentimentPct, WSBSentimentPct: ins.WSBPct,
	}
	var kws []struct {
		Word string `json:"word"`
	}
	if json.Unmarshal(ins.Keywords, &kws) == nil {
		for _, k := range kws {
			f.Keywords = append(f.Keywords, k.Word)
		}
	}
	if subs, err := s.repo.SocialBySubreddit(ctx, t.Symbol); err == nil && len(subs) > 0 {
		f.Subreddits = map[string]int{}
		for _, sub := range subs {
			f.Subreddits[sub.Source] = sub.Mentions
		}
	}
	posts, _ := s.repo.SocialPosts(ctx, t.Symbol, socialSummaryPosts)
	for _, p := range posts {
		f.Posts = append(f.Posts, socialFactsPost{
			Subreddit: p.Subreddit, Date: p.PublishedAt.Format("2006-01-02"), Title: p.Title, Excerpt: cutText(p.Snippet, 200),
		})
	}
	return f, true
}

func cutText(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// ValidSocialSummary rejects replies the tab cannot show.
func ValidSocialSummary(o *llm.SocialSummaryLLMOutput) bool {
	if o == nil {
		return false
	}
	n := utf8.RuneCountInString(o.Summary)
	return n >= 20 && n <= 400 && len(o.Tags) > 0
}

// RuleSocialSummary is the no-LLM text — 숫자를 문장 틀에.
func RuleSocialSummary(f socialFacts) SocialSummaryBody {
	trend := "비슷했습니다"
	switch {
	case f.MentionsPrev24h > 0 && f.Mentions24h >= f.MentionsPrev24h*2:
		trend = "크게 늘었습니다"
	case f.Mentions24h > f.MentionsPrev24h:
		trend = "늘었습니다"
	case f.Mentions24h < f.MentionsPrev24h:
		trend = "줄었습니다"
	}
	text := fmt.Sprintf("Reddit 주식 커뮤니티에서 최근 24시간 %d회 언급되어 전일(%d회)보다 %s. 언급한 %d명의 긍정 비율은 %d%%로, 커뮤니티 점수는 %+d점(%s)입니다.",
		f.Mentions24h, f.MentionsPrev24h, trend, f.Users, f.SentimentPct, f.CommunityScore, f.Stance)
	if f.Users < 30 {
		text += " 표본이 작아 해석에 주의가 필요합니다."
	}
	tags := []string{f.Symbol}
	for _, k := range f.Keywords {
		if len(tags) >= 5 {
			break
		}
		tags = append(tags, k)
	}
	return SocialSummaryBody{Summary: text, Tags: tags, Stance: f.Stance}
}
