package service

import (
	"strings"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestBriefingHeadlinesRanksMarketOverGossip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	timeNowForBriefing = func() time.Time { return now }
	t.Cleanup(func() { timeNowForBriefing = time.Now })

	articles := []model.NewsArticle{
		{
			Headline:      "고위공직자 재산공개 명단 발표",
			PublisherName: "연합뉴스",
			PublishedAt:   now.Add(-time.Minute),
		},
		{
			Headline:      "출근길 팟캐스트 — 오늘의 이슈",
			PublisherName: "매경",
			PublishedAt:   now.Add(-2 * time.Minute),
		},
		{
			Headline:      "SK하이닉스, 반도체 실적 개선 전망",
			PublisherName: "한경",
			PublishedAt:   now.Add(-3 * time.Minute),
			Symbols:       []string{"000660"},
		},
		{
			Headline:      "[속보] 코스피, 외국인 순매수에 급등",
			PublisherName: "연합뉴스",
			PublishedAt:   now.Add(-30 * time.Second),
			Tags:          []string{"속보"},
		},
	}
	mentioned := []model.NewsBriefingMention{{Symbol: "000660", Name: "SK하이닉스"}}
	got := briefingHeadlines(articles, mentioned, "domestic")
	if len(got) < 4 {
		t.Fatalf("got %d headlines", len(got))
	}
	top := got[0].Title + " " + got[1].Title
	if !strings.Contains(top, "SK하이닉스") || !strings.Contains(top, "속보") {
		t.Fatalf("want market headlines on top, got %q then %q", got[0].Title, got[1].Title)
	}
	if strings.Contains(got[0].Title, "재산공개") || strings.Contains(got[0].Title, "팟캐스트") {
		t.Fatalf("gossip should not rank first: %q", got[0].Title)
	}
}

func TestNetEok(t *testing.T) {
	t.Parallel()
	// OpenAPI 예시: 매수 5.2조 − 매도 5.35조 = −1,500억.
	if got := netEok("5200000000000", "5350000000000"); got != -1500 {
		t.Fatalf("netEok = %d, want -1500", got)
	}
}
