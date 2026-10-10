package service

import (
	"strings"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestRankEarningsPreviewTargetsHeldFirstThenCap(t *testing.T) {
	ts := []repository.EarningsPreviewTarget{
		{Symbol: "BIG", MarketCap: 1e12},
		{Symbol: "MINE", MarketCap: 1e9, Held: true},
		{Symbol: "MID", MarketCap: 5e11},
		{Symbol: "WATCH", MarketCap: 1e8, Watched: true},
	}
	got := RankEarningsPreviewTargets(ts, 3)
	want := []string{"MINE", "WATCH", "BIG"}
	for i, w := range want {
		if got[i].Symbol != w {
			t.Fatalf("순서: got %v want %v", got, want)
		}
	}
}

func TestPreviewNeedsRegen(t *testing.T) {
	cases := []struct {
		name                  string
		exists                bool
		oldHash, newHash, src string
		age                   time.Duration
		days                  int
		want                  bool
	}{
		{"처음", false, "", "h", "", 0, 5, true},
		{"같은 사실표 — 굽지 않는다", true, "h", "h", "llm", 100 * time.Hour, 5, false},
		{"사실 바뀜, AI 글 3일 안 — 유지", true, "a", "b", "llm", 24 * time.Hour, 5, false},
		{"사실 바뀜, 3일 지남 — 새로", true, "a", "b", "llm", 73 * time.Hour, 5, true},
		{"발표 전날, 하루 지난 글 — 마지막 확인", true, "a", "b", "llm", 21 * time.Hour, 1, true},
		{"규칙 문장이었음 — AI로 다시 시도", true, "a", "b", "rule", time.Hour, 5, true},
		{"규칙 문장 + 같은 사실표 — 그래도 AI로 다시", true, "h", "h", "rule", time.Hour, 5, true},
	}
	for _, c := range cases {
		if got := PreviewNeedsRegen(c.exists, c.oldHash, c.newHash, c.src, c.age, c.days); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestRuleEarningsPreview(t *testing.T) {
	score, delta := 69, -1
	ret := 12.5
	f := &repository.EarningsPreviewFacts{
		Name: "삼성전자", Fiscal: "2026 3Q", DaysToEvent: 3,
		Surprises:   []repository.PreviewSurprise{{Pct: 5}, {Pct: -2}, {Pct: 8}},
		EPSRevision: &repository.PreviewRevision{Pct: 4.2},
		Score:       &score, ScoreDelta: &delta, Return20d: &ret,
		Headlines: []string{"삼성전자 3분기 배당 30조"},
	}
	out := RuleEarningsPreview(f)
	if out.Headline != "삼성전자 2026 3Q 실적 발표 D-3" {
		t.Fatalf("headline %q", out.Headline)
	}
	if len(out.Points) != 3 || !strings.Contains(out.Points[0], "지난 3번 중 2번") || !strings.Contains(out.Points[1], "+4.2%") {
		t.Fatalf("points %v", out.Points)
	}
	if !strings.Contains(out.Risk, "+12.5%") {
		t.Fatalf("risk %q", out.Risk)
	}
	if !ValidEarningsPreview(&out) {
		t.Fatal("규칙 문장도 화면이 그릴 수 있는 모양이어야 한다")
	}
}

func TestValidEarningsPreview(t *testing.T) {
	if ValidEarningsPreview(&llm.EarningsPreviewLLMOutput{Headline: "x", Points: []string{"a"}}) {
		t.Fatal("요점 하나는 부족")
	}
	if ValidEarningsPreview(&llm.EarningsPreviewLLMOutput{Headline: "", Points: []string{"a", "b"}}) {
		t.Fatal("제목 없음")
	}
	if !ValidEarningsPreview(&llm.EarningsPreviewLLMOutput{Headline: "기대 상향 뒤의 발표", Points: []string{"a", "b", "c"}, Risk: "r"}) {
		t.Fatal("정상")
	}
}

func TestNextEarningsPreviewSlot(t *testing.T) {
	loc := kstLoc()
	cases := []struct {
		now  time.Time
		want time.Time
		ids  []model.BatchJobID
	}{
		{time.Date(2026, 9, 27, 6, 0, 0, 0, loc), time.Date(2026, 9, 27, 7, 10, 0, 0, loc),
			[]model.BatchJobID{model.BatchEarningsPreviewDomestic, model.BatchEarningsPreviewOverseas}},
		// 07:10 정각이면 이미 지난 슬롯 — 다음은 18:50 국내만(2026-10-09 AI 실적 요약 슬롯).
		{time.Date(2026, 9, 27, 7, 10, 0, 0, loc), time.Date(2026, 9, 27, 18, 50, 0, 0, loc),
			[]model.BatchJobID{model.BatchEarningsPreviewDomestic}},
		{time.Date(2026, 9, 27, 19, 0, 0, 0, loc), time.Date(2026, 9, 27, 21, 30, 0, 0, loc),
			[]model.BatchJobID{model.BatchEarningsPreviewOverseas}},
		{time.Date(2026, 9, 27, 22, 0, 0, 0, loc), time.Date(2026, 9, 28, 7, 10, 0, 0, loc),
			[]model.BatchJobID{model.BatchEarningsPreviewDomestic, model.BatchEarningsPreviewOverseas}},
	}
	for _, c := range cases {
		got, ids := NextEarningsPreviewSlot(c.now)
		if !got.Equal(c.want) || len(ids) != len(c.ids) || ids[0] != c.ids[0] {
			t.Fatalf("%v → %v %v, want %v %v", c.now, got, ids, c.want, c.ids)
		}
	}
}

// 사실표에 없는 숫자는 걸러야 한다 — 소수·32 이상만 보고, 작은 정수(곳 수·날짜)는 통과.
func TestDigestNumbersGrounded(t *testing.T) {
	facts := []byte(`{"recent":[{"name":"삼성전자","est":"10.6조","act":"10.7조","pct":1.3}],"released":3,"avgPct":36.3}`)
	if !DigestNumbersGrounded("삼성전자 영업이익 10.7조, 예상 1.3% 상회 — 3곳 중 2곳 상회(평균 36.3%), 10월 14일", facts) {
		t.Fatal("사실표 숫자인데 걸렸다")
	}
	if DigestNumbersGrounded("LG에너지솔루션은 예상의 2.3배", facts) {
		t.Fatal("계산한 배수가 통과했다")
	}
	if DigestNumbersGrounded("영업이익 7,560억", facts) {
		t.Fatal("없는 숫자가 통과했다")
	}
}

func TestRuleEarningsDigest(t *testing.T) {
	pct := 1.3
	f := &EarningsDigestFacts{Released: 3, Beats: 2, Misses: 1,
		Recent:   []EarningsDigestEvent{{Name: "삼성전자", Metric: "영업이익", Est: "10.6조", Act: "10.7조", Pct: &pct}},
		Upcoming: []EarningsDigestEvent{{Name: "SK하이닉스", Date: "2026-10-23", Status: "estimated"}}}
	out := RuleEarningsDigest(f)
	if out.Headline != "삼성전자 영업이익 10.7조(예상 10.6조) — 예상 +1.3% 상회" {
		t.Fatalf("headline %q", out.Headline)
	}
	if out.Watch != "다가오는 대형 발표: SK하이닉스(10/23 추정)" {
		t.Fatalf("watch %q", out.Watch)
	}
	empty := RuleEarningsDigest(&EarningsDigestFacts{})
	if empty.Headline == "" || len(empty.Points) == 0 {
		t.Fatal("빈 사실표도 자리는 채운다")
	}
}

func TestLatestPreviewPerSymbol(t *testing.T) {
	now := time.Now()
	rows := []repository.EarningsPreviewRow{
		{Symbol: "005930", EventDate: "2026-10-14", GeneratedAt: now.Add(-48 * time.Hour)},
		{Symbol: "005930", EventDate: "2026-10-10", GeneratedAt: now},
		{Symbol: "000660", EventDate: "2026-10-23", GeneratedAt: now},
	}
	got := LatestPreviewPerSymbol(rows)
	if len(got) != 2 || got[0].EventDate != "2026-10-10" || got[1].Symbol != "000660" {
		t.Fatalf("추정일 행은 확정일 행에 밀려야 한다: %+v", got)
	}
}
