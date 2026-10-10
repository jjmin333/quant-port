package service

import (
	"context"
	"encoding/json"
	"math"
	"sort"

	"github.com/enact/quant-port/be/internal/repository"
)

/*
「미국 개미 분위기」 — Reddit 시장 단위 지표 (2026-10-01 요청 3, 결정: 홈 카드 + AI 한 줄).

이미 매일 받는 재료만 모은다 — 새 외부 호출은 시장 인기 글 RSS 하루 1회(social_posts 잡)와
LLM 하루 1콜(social_summary 잡의 첫 항목)뿐이다. 화면은 저장분만 읽는다.

분위기 점수는 **규칙**이다(종목 커뮤니티 점수와 같은 55% 기준·×2.2): 지수 ETF(SPY·QQQ·IWM·DIA)
감성 60% + 언급 상위 종목 전체(사용자 5명↑, 사용자 가중) 40%. 종목 AI 점수에는 넣지 않는다.
*/

// SocialMarketSymbol — 시장 인기 글·AI 한 줄을 종목 표(social_posts·social_summaries)에 얹는 가짜 심볼.
const SocialMarketSymbol = "_MARKET"

var marketMoodETFs = []string{"SPY", "QQQ", "IWM", "DIA"}

const (
	marketMoodDays     = 14
	marketMoodMinUsers = 50 // 전체 표본이 이보다 작으면 점수 없음
	marketMoodETFMin   = 20 // 지수 ETF 표본이 이보다 작으면 전체만
	marketMoodSurgeMin = 20 // 급증은 오늘 언급 20회 이상만
)

// MarketMoodScore — (점수 -100..100, 섞은 감성 %, 있음).
func MarketMoodScore(etfSU, etfU, allSU, allU int64) (int, float64, bool) {
	if allU < marketMoodMinUsers {
		return 0, 0, false
	}
	pct := float64(allSU) / float64(allU)
	if etfU >= marketMoodETFMin {
		pct = 0.6*float64(etfSU)/float64(etfU) + 0.4*pct
	}
	s := math.Max(-100, math.Min(100, (pct-communityNeutralPct)*communityScale))
	return int(math.Round(s)), math.Round(pct*10) / 10, true
}

// MarketMoodStance — 매수·매도 권유로 읽히지 않게 분위기 낱말만.
func MarketMoodStance(score int) string {
	switch {
	case score >= 40:
		return "강한 낙관"
	case score >= 15:
		return "약한 낙관"
	case score > -15:
		return "중립"
	case score > -40:
		return "약한 비관"
	}
	return "강한 비관"
}

type MarketMoodETF struct {
	Symbol       string `json:"symbol"`
	Mentions     int    `json:"mentions"`
	Users        *int   `json:"users,omitempty"`
	SentimentPct *int   `json:"sentimentPct,omitempty"`
}

type MarketMoodSurge struct {
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Mentions     int    `json:"mentions"`
	MentionsPrev int    `json:"mentionsPrev"`
}

type MarketMoodDay struct {
	Date     string `json:"date"`
	Mentions int    `json:"mentions"`
	Score    *int   `json:"score,omitempty"`
}

// MarketMood is GET /social/market-mood.
type MarketMood struct {
	Date         string                    `json:"date"`
	Score        *int                      `json:"score,omitempty"`
	Stance       string                    `json:"stance,omitempty"`
	SentimentPct *float64                  `json:"sentimentPct,omitempty"`
	Users        int64                     `json:"users"` // 감성을 잰 사용자 수(언급 상위 종목 · 5명↑ 합)
	Mentions     int                       `json:"mentions"`
	MentionsPrev int                       `json:"mentionsPrev"`
	Bulls        int                       `json:"bulls"`
	Bears        int                       `json:"bears"`
	ETFs         []MarketMoodETF           `json:"etfs"`
	Surges       []MarketMoodSurge         `json:"surges"`
	Subreddits   []repository.SocialSource `json:"subreddits"`
	History      []MarketMoodDay           `json:"history"`
	Posts        []repository.SocialPost   `json:"posts"`
	Summary      *SocialSummaryBody        `json:"summary,omitempty"`
	SummaryDate  string                    `json:"summaryDate,omitempty"`
	SummaryAI    bool                      `json:"summaryAi,omitempty"`
}

// BuildMarketMood reads the stored snapshots (LLM·외부 호출 없음). 스냅샷이 없으면 nil.
func BuildMarketMood(ctx context.Context, repo *repository.ExternalSignalRepo) (*MarketMood, error) {
	days, err := repo.SocialMarketDays(ctx, marketMoodETFs, marketMoodDays)
	if err != nil || len(days) == 0 {
		return nil, err
	}
	today := days[0]
	out := &MarketMood{Date: today.Date, Mentions: today.Mentions, Users: today.AllUsers,
		ETFs: []MarketMoodETF{}, Surges: []MarketMoodSurge{}, History: []MarketMoodDay{}, Posts: []repository.SocialPost{}}
	if len(days) > 1 {
		out.MentionsPrev = days[1].Mentions
	}
	if s, pct, ok := MarketMoodScore(today.ETFSentUsers, today.ETFUsers, today.AllSentUsers, today.AllUsers); ok {
		out.Score, out.SentimentPct, out.Stance = &s, &pct, MarketMoodStance(s)
	}
	for i := len(days) - 1; i >= 0; i-- { // 차트는 오래된 것부터
		d := MarketMoodDay{Date: days[i].Date, Mentions: days[i].Mentions}
		if s, _, ok := MarketMoodScore(days[i].ETFSentUsers, days[i].ETFUsers, days[i].AllSentUsers, days[i].AllUsers); ok {
			d.Score = &s
		}
		out.History = append(out.History, d)
	}

	insights, err := repo.SocialInsightsBetween(ctx, today.Date, today.Date)
	if err != nil {
		return nil, err
	}
	for _, in := range insights {
		if s, ok := CommunityScore(in.SentimentPct, in.Users); ok {
			switch CommunityStance(s) {
			case "낙관":
				out.Bulls++
			case "비관":
				out.Bears++
			}
		}
	}
	top, err := repo.SocialTopOn(ctx, today.Date, 400)
	if err != nil {
		return nil, err
	}
	out.ETFs, out.Surges = marketETFsAndSurges(top, insights)

	if out.Subreddits, err = repo.SocialSubredditTotals(ctx, today.Date); err != nil {
		return nil, err
	}
	// 모은 것 전부(8) — 홈 카드 오른쪽 칸을 인기 글로 채운다(2026-10-09 요청 6).
	if out.Posts, err = repo.SocialPosts(ctx, SocialMarketSymbol, socialMarketPostsKeep); err != nil {
		return nil, err
	}
	if out.Posts == nil {
		out.Posts = []repository.SocialPost{}
	}
	if sum, err := repo.SocialSummaryOf(ctx, SocialMarketSymbol); err == nil && sum != nil {
		var body SocialSummaryBody
		if json.Unmarshal(sum.Body, &body) == nil && body.Summary != "" {
			out.Summary, out.SummaryDate, out.SummaryAI = &body, sum.SnapDate, sum.Source == "llm"
		}
	}
	return out, nil
}

// marketETFsAndSurges — 지수 ETF 줄과 급증 상위 5(오늘 20회 이상 · 전일 대비 배수 순, ETF·잡음 티커 제외).
func marketETFsAndSurges(top []repository.SocialTop, insights map[string]repository.SocialInsight) ([]MarketMoodETF, []MarketMoodSurge) {
	isETF := map[string]bool{}
	for _, e := range marketMoodETFs {
		isETF[e] = true
	}
	etfs := []MarketMoodETF{}
	surges := []MarketMoodSurge{}
	for _, t := range top {
		if isETF[t.Symbol] {
			e := MarketMoodETF{Symbol: t.Symbol, Mentions: t.Mentions}
			if in, ok := insights[t.Symbol]; ok {
				e.Users, e.SentimentPct = in.Users, in.SentimentPct
			}
			etfs = append(etfs, e)
			continue
		}
		if t.Mentions >= marketMoodSurgeMin && !socialTickerDenylist[t.Symbol] && t.Mentions >= 2*max(t.MentionsPrev, 1) {
			surges = append(surges, MarketMoodSurge{Symbol: t.Symbol, Name: t.Name, Mentions: t.Mentions, MentionsPrev: t.MentionsPrev})
		}
	}
	ratio := func(s MarketMoodSurge) float64 { return float64(s.Mentions) / float64(max(s.MentionsPrev, 1)) }
	sort.SliceStable(surges, func(i, j int) bool { return ratio(surges[i]) > ratio(surges[j]) })
	if len(surges) > 5 {
		surges = surges[:5]
	}
	return etfs, surges
}

// marketFacts — AI 한 줄의 사실표. 종목 요약과 같은 모양이라 같은 프롬프트·검증·규칙 문장을 쓴다.
func marketFacts(m *MarketMood) (socialFacts, bool) {
	if m == nil || m.Score == nil || m.SentimentPct == nil {
		return socialFacts{}, false
	}
	f := socialFacts{
		Symbol: SocialMarketSymbol, Name: "미국 증시 전반(Reddit 주식 커뮤니티 전체)", Date: m.Date,
		CommunityScore: *m.Score, Stance: m.Stance, Mentions24h: m.Mentions, MentionsPrev24h: m.MentionsPrev,
		SentimentPct: int(math.Round(*m.SentimentPct)), Users: int(m.Users), Subreddits: map[string]int{},
	}
	for _, e := range m.ETFs {
		if e.SentimentPct != nil {
			f.Keywords = append(f.Keywords, e.Symbol+" 감성 "+itoa(*e.SentimentPct)+"%")
		}
	}
	for _, s := range m.Surges {
		f.Keywords = append(f.Keywords, s.Symbol+" 언급 급증")
	}
	for _, s := range m.Subreddits {
		f.Subreddits[s.Source] = s.Mentions
	}
	for _, p := range m.Posts {
		f.Posts = append(f.Posts, socialFactsPost{Subreddit: p.Subreddit, Date: p.PublishedAt.Format("2006-01-02"),
			Title: p.Title, Excerpt: cutText(p.Snippet, 200)})
	}
	return f, true
}
