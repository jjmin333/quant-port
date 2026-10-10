package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
Reddit 커뮤니티 탭 (2026-09-29 사용자 요청 — 「ApeWisdom 최대 활용 · 해외 종목 커뮤니티 탭 · 점수·AI 요약」).

	social_mentions 잡(00:05)  전체 목록 + 서브레딧 9개 + **언급 상위 150종목의 종목 페이지**
	                           (언급 사용자 · 감성 · WSB 감성 · 키워드) → social_insights
	social_posts 잡(00:30)     Reddit 검색 RSS로 이번 주 인기 글 → social_posts, 제목·앞부분은
	                           번역기(자체 모델/Azure)로 옮겨 둔다. 분당 1회라 하루 80종목 로테이션.
	social_summary 잡(04:20)   LLM 요약 — API 프로세스(social_summary.go).

커뮤니티 점수(-100..100)는 **규칙**이다: ApeWisdom 감성(긍정 비율)을 표본 크기로 줄인다.
LLM은 그 점수에 맞춰 글을 쓴다 — 점수를 LLM이 정하면 같은 재료로 날마다 다른 숫자가 나온다.
*/

const (
	socialInsightTop = 150
	socialPostsLimit = 80
	// socialPostsKeep — 한 종목에 남기는 글 수(화면은 5개씩 넘긴다).
	socialPostsKeep = 20
	// 상위 종목은 매일, 나머지는 사흘마다 글을 다시 받는다(등급제 로테이션).
	socialPostsTopDaily  = 40
	socialPostsStaleTop  = 20 * time.Hour
	socialPostsStaleRest = 72 * time.Hour
	// socialMarketPostsKeep — 시장 인기 글(카드는 5개, AI 사실표도 이것을 읽는다).
	socialMarketPostsKeep = 8
)

// Translator is the EN→KO engine (translate.Chain).
type Translator interface {
	Available() bool
	TranslateENtoKO(ctx context.Context, texts []string) ([]string, error)
}

// SetReddit wires the post source and the translator (worker-filings).
func (s *ExternalSignalService) SetReddit(c *marketdata.RedditClient, tr Translator) {
	s.reddit, s.translator = c, tr
}

/*
communityNeutralPct — ApeWisdom 감성의 「보통」. 2026-09-29 표본(SPY 57 · MU 56 · META 55 · MSFT 47 ·
AMZN 73 · AMD 80 · AAPL 27)의 가운데가 50이 아니라 55 근처다 — 50을 기준으로 두면 대부분이 낙관으로 읽힌다.
communityPriorUsers — 표본이 적을수록 보통 쪽으로 당긴다(TSLA 11명 100% → 71%).
*/
const (
	communityNeutralPct = 55.0
	communityPriorUsers = 20.0
	communityMinUsers   = 5
	communityScale      = 2.2
)

// CommunityScore — 커뮤니티 점수 -100..100. 언급한 사용자가 5명 미만이면 없음.
func CommunityScore(sentPct, users *int) (int, bool) {
	if sentPct == nil || users == nil || *users < communityMinUsers {
		return 0, false
	}
	n := float64(*users)
	adj := (float64(*sentPct)*n + communityNeutralPct*communityPriorUsers) / (n + communityPriorUsers)
	v := math.Max(-100, math.Min(100, (adj-communityNeutralPct)*communityScale))
	return int(math.Round(v)), true
}

// CommunityStance — 점수를 말로. 매수·매도 권유로 읽히지 않게 「분위기」 낱말만 쓴다.
func CommunityStance(score int) string {
	switch {
	case score >= 25:
		return "낙관"
	case score <= -25:
		return "비관"
	}
	return "관망"
}

// fetchInsights — 언급 상위 종목의 종목 페이지. 전체 목록 저장 뒤라 실패해도 잃는 것이 없다.
func (s *ExternalSignalService) fetchInsights(ctx context.Context, date string, rows []repository.SocialRow) int {
	stored := 0
	for i, r := range rows {
		if i >= socialInsightTop || s.pace(ctx, apibudget.ProviderApeWisdom) != nil {
			break
		}
		ins, err := s.ape.Insight(ctx, r.Symbol)
		if err != nil {
			if !errors.Is(err, marketdata.ErrExternalNotFound) {
				log.Printf("social insight %s: %v", r.Symbol, err)
			}
			if errors.Is(err, apibudget.ErrExhausted) {
				break
			}
			continue
		}
		row := repository.SocialInsight{Symbol: r.Symbol, SnapDate: date, Users: ins.Users, SentimentPct: ins.SentimentPct, WSBPct: ins.WSBPct}
		if len(ins.Keywords) > 0 {
			row.Keywords, _ = json.Marshal(ins.Keywords)
		}
		// 언급 0명인 날의 「0%」는 감성이 아니다.
		if ins.Users != nil && *ins.Users == 0 {
			row.SentimentPct, row.WSBPct = nil, nil
		}
		if s.repo.UpsertSocialInsight(ctx, row) == nil {
			stored++
		}
	}
	return stored
}

// RunSocialPosts — 언급 상위 종목의 이번 주 인기 Reddit 글과 번역.
func (s *ExternalSignalService) RunSocialPosts(ctx context.Context, limit int) (map[string]any, string, error) {
	if s == nil || s.reddit == nil {
		return nil, "", fmt.Errorf("social posts: not configured")
	}
	if limit <= 0 {
		limit = socialPostsLimit
	}
	date, err := s.repo.LatestSocialDate(ctx)
	if err != nil || date == "" {
		return nil, "", fmt.Errorf("social posts: no mentions snapshot yet (%v)", err)
	}
	top, err := s.repo.SocialTopOn(ctx, date, 400)
	if err != nil {
		return nil, "", err
	}
	targets := pickPostTargets(top, limit, time.Now())
	var fetched, stored, translated, failed int
	// 시장 인기 글 먼저(2026-10-01 요청 3 「미국 개미 분위기」) — 재배포로 잡이 끊겨도 이것은 남는다.
	marketPosts := 0
	if posts, err := s.reddit.MarketTop(ctx); err != nil {
		log.Printf("social posts market: %v", err)
	} else {
		if len(posts) > socialMarketPostsKeep {
			posts = posts[:socialMarketPostsKeep]
		}
		rows, n := s.translatePosts(ctx, SocialMarketSymbol, posts)
		translated += n
		if s.repo.ReplaceSocialPosts(ctx, SocialMarketSymbol, rows) == nil {
			marketPosts = len(rows)
		}
	}
	for _, t := range targets {
		if s.pace(ctx, apibudget.ProviderRedditRSS) != nil {
			break
		}
		posts, err := s.reddit.SearchTop(ctx, t.Symbol, t.EnglishName)
		if err != nil && isBudgetOr429(err) {
			if !sleepCtx(ctx, 65*time.Second) {
				break
			}
			posts, err = s.reddit.SearchTop(ctx, t.Symbol, t.EnglishName)
		}
		if err != nil {
			failed++
			log.Printf("social posts %s: %v", t.Symbol, err)
			if errors.Is(err, apibudget.ErrExhausted) && !isBudgetOr429(err) {
				break // 일 한도
			}
			continue
		}
		fetched++
		if len(posts) > socialPostsKeep {
			posts = posts[:socialPostsKeep]
		}
		rows, n := s.translatePosts(ctx, t.Symbol, posts)
		translated += n
		if err := s.repo.ReplaceSocialPosts(ctx, t.Symbol, rows); err != nil {
			failed++
			continue
		}
		stored += len(rows)
	}
	msg := fmt.Sprintf("social_posts: %s 기준 %d종목 중 %d종목 받음 · 글 %d · 시장 글 %d · 새 번역 %d · 실패 %d",
		date, len(targets), fetched, stored, marketPosts, translated, failed)
	return map[string]any{"date": date, "targets": len(targets), "fetched": fetched, "posts": stored,
		"marketPosts": marketPosts, "translated": translated, "failed": failed}, msg, nil
}

// pickPostTargets — 상위 40은 20시간, 나머지는 사흘 지난 것만. 언급 많은 순.
func pickPostTargets(top []repository.SocialTop, limit int, now time.Time) []repository.SocialTop {
	var out []repository.SocialTop
	for i, t := range top {
		if len(out) >= limit {
			break
		}
		if socialTickerDenylist[t.Symbol] || t.Mentions < 2 {
			continue
		}
		stale := socialPostsStaleRest
		if i < socialPostsTopDaily {
			stale = socialPostsStaleTop
		}
		if t.LastPostsAt != nil && now.Sub(*t.LastPostsAt) < stale {
			continue
		}
		out = append(out, t)
	}
	return out
}

// translatePosts keeps earlier translations and asks the engine only for new posts.
func (s *ExternalSignalService) translatePosts(ctx context.Context, symbol string, posts []marketdata.RedditPost) ([]repository.SocialPost, int) {
	prev, _ := s.repo.SocialPostTranslations(ctx, symbol)
	rows := make([]repository.SocialPost, len(posts))
	var texts []string
	var slots []*string
	for i, p := range posts {
		rows[i] = repository.SocialPost{PostID: p.ID, Subreddit: p.Subreddit, Title: p.Title, Snippet: p.Snippet,
			URL: p.URL, TopRank: p.Rank, PublishedAt: p.Published}
		if old, ok := prev[p.ID]; ok && old[0] != "" {
			rows[i].TitleKO, rows[i].SnippetKO = old[0], old[1]
			continue
		}
		texts = append(texts, p.Title)
		slots = append(slots, &rows[i].TitleKO)
		if p.Snippet != "" {
			texts = append(texts, p.Snippet)
			slots = append(slots, &rows[i].SnippetKO)
		}
	}
	if len(texts) == 0 || s.translator == nil || !s.translator.Available() {
		return rows, 0
	}
	out, err := s.translator.TranslateENtoKO(ctx, texts)
	if err != nil {
		log.Printf("social posts %s: translate: %v", symbol, err)
	}
	n := 0
	for i, ko := range out {
		if i < len(slots) && ko != "" {
			*slots[i] = ko
			n++
		}
	}
	return rows, n
}

// redditSentimentComponent — 심리 축의 Reddit 성분(미국 종목, 언급 사용자 5명 이상).
//
// 커뮤니티 점수(-100..100)를 뉴스·내부자와 같은 15..85 폭에 둔다. confidence는 사용자 50명에서 1.0.
func redditSentimentComponent(in *repository.SocialInsight) (float64, float64, bool) {
	if in == nil {
		return 0, 0, false
	}
	score, ok := CommunityScore(in.SentimentPct, in.Users)
	if !ok {
		return 0, 0, false
	}
	return 50 + float64(score)*0.35, math.Min(1, float64(*in.Users)/50), true
}

// socialSnapWindow — 점수 날짜 기준 [-2일, +1일]의 요약만 쓴다. 스냅샷은 KST 날짜로 찍혀 미국 세션보다
// 하루 늦을 수 있고(+1), 그보다 뒤의 것은 성적표(IC)에 미래를 섞는다.
func socialSnapWindow(tradeDate string) (string, string) {
	d, err := time.ParseInLocation("2006-01-02", tradeDate, model.KST)
	if err != nil {
		return tradeDate, tradeDate
	}
	return d.AddDate(0, 0, -2).Format("2006-01-02"), d.AddDate(0, 0, 1).Format("2006-01-02")
}
