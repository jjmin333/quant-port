package service

import (
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestBotNextSlot(t *testing.T) {
	kst := kstLoc()
	cases := map[string]string{
		"2026-10-16 00:00": "2026-10-16 03:00", // 경계 위면 다음 회차
		"2026-10-16 02:59": "2026-10-16 03:00",
		"2026-10-16 13:10": "2026-10-16 15:00",
		"2026-10-16 21:00": "2026-10-17 00:00",
		"2026-10-16 23:30": "2026-10-17 00:00",
	}
	for in, want := range cases {
		now, _ := time.ParseInLocation("2006-01-02 15:04", in, kst)
		if got := BotNextSlot(now).In(kst).Format("2006-01-02 15:04"); got != want {
			t.Errorf("BotNextSlot(%s) = %s, want %s", in, got, want)
		}
	}
	if got := botSlotOf(time.Date(2026, 10, 16, 13, 10, 0, 0, kst)).Hour(); got != 12 {
		t.Errorf("botSlotOf hour = %d, want 12", got)
	}
}

func TestBotPeerLabel(t *testing.T) {
	for in, want := range map[string]string{
		"domestic:KOSPI:cap3":             "국내 KOSPI 대형주",
		"us:NASDAQ:cap1":                  "미국 NASDAQ 소형주",
		"domestic:information_technology": "국내 IT",
		"*":                               "전체 시장",
	} {
		if got := botPeerLabel(in); got != want {
			t.Errorf("botPeerLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBotPickNoDuplicatesAndFilter(t *testing.T) {
	cands := []*botCandidate{
		{Symbol: "A", Weight: 10}, {Symbol: "B", Weight: 1}, {Symbol: "C", Weight: 5},
		{Symbol: "D", Weight: 0}, {Symbol: "E", Weight: 3},
	}
	rnd := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 50; i++ {
		got := botPick(rnd, cands, 3, func(c *botCandidate) bool { return c.Symbol != "E" })
		if len(got) != 3 {
			t.Fatalf("picked %d, want 3", len(got))
		}
		seen := map[string]bool{}
		for _, c := range got {
			if seen[c.Symbol] || c.Symbol == "D" || c.Symbol == "E" {
				t.Fatalf("bad pick %v", c.Symbol)
			}
			seen[c.Symbol] = true
		}
	}
	if got := botPick(rnd, cands, 10, func(*botCandidate) bool { return true }); len(got) != 4 {
		t.Fatalf("pool of 4 positive weights → %d", len(got))
	}
}

func TestBotDraftBodies(t *testing.T) {
	score, tech := 72, 81.0
	peer := 5
	c := &botCandidate{
		Symbol: "005930", Name: "삼성전자", Market: "domestic", Mentions: 34,
		Events:    []string{"점수 +12", "#1 거래대금"},
		ScoreJump: "점수 +12",
		Score: &model.StockScoreView{Score: &score, Grade: "B+", PeerGroup: "반도체", PeerPct: &peer,
			Breakdown: model.ScoreBreakdown{Technical: &tech}},
		CloseAI:   repository.BotCloseAI{TradeDate: "2026-10-15", ChangeRate: 3.2, Summary: "메모리 가격 반등 기대로 올랐다."},
		Narrative: &repository.StockSummaryNarrative{Overview: "첫 문장이다. 둘째 문장이다. 셋째 문장이다.", News: "@태그 뉴스"},
		Upcoming:  &botPreview{EventDate: "2026-10-17", Fiscal: "3Q", Headline: "HBM 비중", Points: []string{"마진"}, Risk: "환율"},
	}
	slot := time.Date(2026, 10, 16, 12, 0, 0, 0, kstLoc())
	for _, long := range []bool{false, true} {
		d := buildBotCommunityDraft(rand.New(rand.NewPCG(3, 4)), c, botMarketMood{Mood: "강세"}, long, slot)
		if strings.ContainsAny(d.Body, "#@") {
			t.Errorf("community body (long=%v) carries tag characters:\n%s", long, d.Body)
		}
		if d.Title == "" || d.Kind != repository.BotDraftCommunity || d.Symbol != "005930" {
			t.Errorf("community draft fields: %+v", d)
		}
		r := buildBotResearchDraft(rand.New(rand.NewPCG(5, 6)), c, long, slot)
		if !strings.Contains(r.Body, "## ") || r.Kind != repository.BotDraftResearch {
			t.Errorf("research draft (long=%v) body:\n%s", long, r.Body)
		}
		// 방향·목표가는 관리자 몫 — 초안이 매수·매도를 말하지 않는다.
		for _, w := range []string{"매수", "매도", "목표가"} {
			if strings.Contains(r.Body+d.Body, w) {
				t.Errorf("draft must not say %q", w)
			}
		}
	}
}
