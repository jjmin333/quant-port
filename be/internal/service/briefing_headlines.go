package service

import (
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// 재산공개·팟캐스트·출근길·연예는 시장 브리핑의 오늘의 핵심이 되면 안 된다.
// 관련성 필터가 기사를 목록에 남기더라도, 프롬프트 40건에서는 뒤로 보낸다.
var briefingHeadlineDemoteKR = []string{
	"재산공개", "고위공직자", "팟캐스트", "출근길", "연예", "배우", "가수", "아이돌",
}
var briefingHeadlineDemoteUS = []string{
	"podcast", "celebrity", "morning commute", "entertainment", "actor", "actress",
}

func briefingHeadlines(articles []model.NewsArticle, mentioned []model.NewsBriefingMention, market string) []model.NewsBriefingHeadline {
	if len(articles) == 0 {
		return nil
	}
	mentionKeys := make([]string, 0, len(mentioned)*2)
	for _, m := range mentioned {
		if m.Symbol != "" {
			mentionKeys = append(mentionKeys, strings.ToLower(m.Symbol))
		}
		if m.Name != "" {
			mentionKeys = append(mentionKeys, strings.ToLower(m.Name))
		}
	}
	boostTerms := concatTermLists(strongMarketTerms, corporateFinanceTerms, macroFinanceTerms)
	demote := briefingHeadlineDemoteKR
	if market == "us" {
		boostTerms = concatTermLists(usStrongMarketTerms, usCorporateFinanceTerms, usMacroFinanceTerms)
		demote = briefingHeadlineDemoteUS
	}

	type ranked struct {
		h     model.NewsBriefingHeadline
		score int
		idx   int
	}
	now := timeNowForBriefing()
	items := make([]ranked, 0, len(articles))
	for i, a := range articles {
		title := strings.TrimSpace(a.HeadlineKo)
		if title == "" {
			title = strings.TrimSpace(a.Headline)
		}
		if title == "" {
			continue
		}
		blob := newsTextBlob(title, "")
		score := 0
		if isBreakingArticle(a, now) {
			score += 3
		}
		if containsAnyTerm(blob, mentionKeys) {
			score += 4
		}
		if containsAnyTerm(blob, boostTerms) {
			score += 2
		}
		if containsAnyTerm(blob, demote) {
			score -= 5
		}
		items = append(items, ranked{
			h: model.NewsBriefingHeadline{
				Title:     title,
				At:        a.PublishedAt.In(seoulLocation()).Format("01-02 15:04"),
				Publisher: strings.TrimSpace(a.PublisherName),
				Breaking:  isBreakingArticle(a, now),
			},
			score: score,
			idx:   i, // 원래 목록이 최신순 — 동점이면 더 최근이 앞
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].idx < items[j].idx
	})

	perPublisher := map[string]int{}
	out := make([]model.NewsBriefingHeadline, 0, briefingHeadlineCount)
	for _, item := range items {
		if len(out) >= briefingHeadlineCount {
			break
		}
		pub := item.h.Publisher
		if perPublisher[pub] >= briefingHeadlinesPerPublisher {
			continue
		}
		perPublisher[pub]++
		out = append(out, item.h)
	}
	return out
}

// timeNowForBriefing is a seam so tests can freeze "now" for 속보 창.
var timeNowForBriefing = func() time.Time { return time.Now() }
