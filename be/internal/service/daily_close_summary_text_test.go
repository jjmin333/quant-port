package service

import (
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

/*
이 글은 `stock_daily_closes.ai_summary`에 **한 번 쌓이면 되돌릴 수 없다** — 그날의
LLM 응답은 캐시가 지나면 사라진다. 그래서 무엇이 들어가고 무엇이 빠지는지를 시험으로
못 박는다 (2026-09-11 사용자 요청 2).
*/

func TestDailyCloseSummaryText_joinsAllFourSections(t *testing.T) {
	view := &model.StockSummaryView{
		Valuation: "PER 12배로 업종 평균보다 낮다.",
		Technical: "20일선을 위로 뚫었다.",
		Insider:   "대표이사가 1만 주를 샀다.",
		News:      "수주 공시가 두 건 있었다.",
	}
	got := dailyCloseSummaryText(view)
	for _, want := range []string{"PER 12배", "20일선", "대표이사", "수주 공시"} {
		if !strings.Contains(got, want) {
			t.Fatalf("네 갈래가 다 들어가야 한다: %q 없음 in %q", want, got)
		}
	}
	// 순서는 밸류에이션 → 기술적 → 내부자 → 뉴스다.
	if idx := strings.Index(got, "대표이사"); idx < strings.Index(got, "20일선") {
		t.Fatalf("섹션 순서가 어긋났다: %q", got)
	}
}

func TestDailyCloseSummaryText_dropsMissingSections(t *testing.T) {
	// 내부자 거래가 없는 종목 — 그 갈래는 통째로 빠진다(빈 문자열).
	view := &model.StockSummaryView{
		Valuation: "PER 12배.",
		Technical: "",
		Insider:   "",
		News:      "뉴스 세 건.",
	}
	got := dailyCloseSummaryText(view)
	if got != "PER 12배. 뉴스 세 건." {
		t.Fatalf("빈 섹션은 공백도 남기지 않아야 한다: %q", got)
	}
}

func TestDailyCloseSummaryText_dropsRuleBasedEmptySentences(t *testing.T) {
	// 규칙 기반이 내놓는 「없다」 문장은 이력에 쌓지 않는다 — composeOverview와 같은 필터.
	view := &model.StockSummaryView{
		Valuation: "PER 12배.",
		Technical: "기술적 지표를 계산할 캔들 데이터가 부족합니다.",
		Insider:   "최근 내부자 거래 내역이 없습니다.",
		News:      "최근 관련 뉴스가 없습니다.",
	}
	if got := dailyCloseSummaryText(view); got != "PER 12배." {
		t.Fatalf("「없다」 문장이 남았다: %q", got)
	}
}

func TestDailyCloseSummaryText_emptyWhenNothingGenerated(t *testing.T) {
	if got := dailyCloseSummaryText(&model.StockSummaryView{}); got != "" {
		t.Fatalf("생성된 글이 없으면 빈 문자열이어야 한다: %q", got)
	}
	if got := dailyCloseSummaryText(nil); got != "" {
		t.Fatalf("nil view 는 빈 문자열이어야 한다: %q", got)
	}
}
