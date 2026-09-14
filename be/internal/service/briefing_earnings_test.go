package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestBriefingEarningsScoreMegaCapWithoutSurprise(t *testing.T) {
	t.Parallel()
	cap := 400e12 // 삼성 ~400조
	ev := model.EarningsEvent{
		Market:     string(model.AssetDomestic),
		MarketCap:  &cap,
		DateStatus: model.EarningsDateConfirmed,
	}
	imp, _ := briefingEarningsScore(ev, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	if imp < 80 {
		t.Fatalf("mega-cap confirmed: imp=%d, want >=80 from cap alone", imp)
	}
}

func TestBriefingEarningsScoreSurpriseBugGuard(t *testing.T) {
	t.Parallel()
	cap := 10e12
	pct := 5272.0
	ev := model.EarningsEvent{
		Market:     string(model.AssetDomestic),
		MarketCap:  &cap,
		DateStatus: model.EarningsDateConfirmed,
		Surprise:   &model.EarningsSurprise{Pct: &pct},
	}
	imp, _ := briefingEarningsScore(ev, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	// 시총 25 (5조+) + 서프 0. 가드가 없으면 40이 붙어 65.
	if imp != 25 {
		t.Fatalf("buggy surprise: imp=%d, want 25 (cap only)", imp)
	}
}

func TestBriefingEarningsScoreWMTAndNVDA(t *testing.T) {
	t.Parallel()
	today := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)

	wmtCap := 800e9
	eps := 8.1
	wmt := model.EarningsEvent{
		Market:     string(model.AssetUS),
		MarketCap:  &wmtCap,
		DateStatus: model.EarningsDateConfirmed,
		Surprise:   &model.EarningsSurprise{Pct: &eps},
	}
	wmtImp, _ := briefingEarningsScore(wmt, today)
	// cap 80 + surprise 16 = 96. rxn 없음.
	if wmtImp < 90 {
		t.Fatalf("WMT beat: imp=%d, want >=90", wmtImp)
	}

	nvdaCap := 4e12
	nvda := model.EarningsEvent{
		Market:     string(model.AssetUS),
		MarketCap:  &nvdaCap,
		DateStatus: model.EarningsDateScheduled,
		Date:       "2026-08-26",
		Session:    model.EarningsSessionAfterClose,
	}
	nvdaImp, _ := briefingEarningsScore(nvda, today)
	// cap 80 + soon(5d) 10 = 90
	if nvdaImp < 90 {
		t.Fatalf("NVDA upcoming: imp=%d, want >=90", nvdaImp)
	}
}

func TestBriefingEarningsScoreReactionBothDirections(t *testing.T) {
	t.Parallel()
	cap := 10e9
	ev := model.EarningsEvent{
		Market:     string(model.AssetUS),
		MarketCap:  &cap,
		DateStatus: model.EarningsDateConfirmed,
		Reaction:   &model.EarningsReaction{Pct: -9.2},
	}
	imp, _ := briefingEarningsScore(ev, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	// cap 25 ($5B+) + reaction ~14 = ~39
	if imp < 35 || imp > 45 {
		t.Fatalf("down-move reaction: imp=%d, want ~39", imp)
	}
}

func TestCompactBriefingEarningKR(t *testing.T) {
	t.Parallel()
	actual, est := 3.4e12, 3.1e12
	s := 9.7
	ev := model.EarningsEvent{
		Symbol:   "000660",
		Name:     "SK하이닉스",
		Market:   string(model.AssetDomestic),
		Date:     "2026-08-14",
		OpIncome: model.EarningsMetric{Actual: &actual, Estimate: &est},
		Surprise: &model.EarningsSurprise{Pct: &s},
		Reaction: &model.EarningsReaction{Pct: 4.1},
	}
	row := compactBriefingEarning(ev, 92)
	if row.D != "08-14" || row.Op != "3.4조/3.1조" || row.S == nil || *row.S != 9.7 {
		t.Fatalf("compact KR = %+v", row)
	}
	if row.EPS != "" || row.Name != "SK하이닉스" {
		t.Fatalf("KR row should carry name and op, not eps: %+v", row)
	}
}

func TestCompactBriefingEarningUSScheduledOmitsNumbers(t *testing.T) {
	t.Parallel()
	ev := model.EarningsEvent{
		Symbol:     "NVDA",
		Name:       "NVIDIA",
		Market:     string(model.AssetUS),
		Date:       "2026-08-26",
		DateStatus: model.EarningsDateScheduled,
		Session:    "amc",
	}
	row := compactBriefingEarning(ev, 90)
	if row.Op != "" || row.EPS != "" || row.Name != "" || row.Sess != "amc" {
		t.Fatalf("US scheduled should be ticker+date+session: %+v", row)
	}
}
