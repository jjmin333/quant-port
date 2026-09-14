package service

import (
	"math"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func day(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		panic(err)
	}
	return t
}

// kstDay mimics MySQL DATE scanned with DSN loc=Asia/Seoul (KST midnight).
func kstDay(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", s, model.KST)
	if err != nil {
		panic(err)
	}
	return t
}

// 8/10(월) ~ 8/14(금) 거래일 종가. 8/15·8/16은 주말이라 없다.
var closes = []repository.DailyClose{
	{Date: day("2026-08-10"), Close: 100},
	{Date: day("2026-08-11"), Close: 110},
	{Date: day("2026-08-12"), Close: 120},
	{Date: day("2026-08-13"), Close: 90},
	{Date: day("2026-08-14"), Close: 99},
}

// **세션을 보지 않으면 반응을 하루 잘못 읽는다.** 장후 발표는 다음 거래일에,
// 장전 발표는 그날 장에 반영된다 — 같은 날짜·같은 종가에서 서로 다른 답이 나와야
// 맞다.
func TestReactionForSessionSplit(t *testing.T) {
	base := model.EarningsEvent{
		Symbol:     "TEST",
		Date:       "2026-08-12",
		DateStatus: model.EarningsDateConfirmed,
	}

	t.Run("장후(amc) — 다음 거래일이 첫 반응", func(t *testing.T) {
		e := base
		e.Session = model.EarningsSessionAfterClose
		r := reactionFor(e, closes)
		if r == nil {
			t.Fatal("reaction = nil")
		}
		// 기준 8/12 120 → 반응 8/13 90 = −25%
		if r.BaseDate != "2026-08-12" || r.Date != "2026-08-13" {
			t.Fatalf("dates = %s → %s, want 08-12 → 08-13", r.BaseDate, r.Date)
		}
		if r.Basis != "nextDay" {
			t.Errorf("basis = %q, want nextDay", r.Basis)
		}
		if got := pct1(r.Pct); got != -25 {
			t.Errorf("pct = %v, want -25", got)
		}
	})

	t.Run("장전(bmo) — 그날 종가가 첫 반응", func(t *testing.T) {
		e := base
		e.Session = model.EarningsSessionBeforeOpen
		r := reactionFor(e, closes)
		if r == nil {
			t.Fatal("reaction = nil")
		}
		// 기준 8/11 110 → 반응 8/12 120 = +9.1%
		if r.BaseDate != "2026-08-11" || r.Date != "2026-08-12" {
			t.Fatalf("dates = %s → %s, want 08-11 → 08-12", r.BaseDate, r.Date)
		}
		if r.Basis != "sameDay" {
			t.Errorf("basis = %q, want sameDay", r.Basis)
		}
		if got := pct1(r.Pct); got != 9.1 {
			t.Errorf("pct = %v, want 9.1", got)
		}
	})

	t.Run("장중(dmh) — 그날 종가가 첫 반응", func(t *testing.T) {
		// 장중에 나온 뉴스는 그 세션 종가에 이미 들어 있다. 장후처럼 다음날을
		// 세면 소화가 끝난 뒤의 하루를 발표 반응이라고 부르게 된다.
		e := base
		e.Session = model.EarningsSessionDuringHours
		r := reactionFor(e, closes)
		if r == nil {
			t.Fatal("reaction = nil")
		}
		if r.BaseDate != "2026-08-11" || r.Date != "2026-08-12" {
			t.Fatalf("dates = %s → %s, want 08-11 → 08-12", r.BaseDate, r.Date)
		}
		if r.Basis != "sameDay" {
			t.Errorf("basis = %q, want sameDay", r.Basis)
		}
	})

	t.Run("국내(시각 없음) — 장후와 같게 본다", func(t *testing.T) {
		// 국내 잠정실적은 통상 마감 후 공시되고 시각 데이터 자체가 없다.
		r := reactionFor(base, closes)
		if r == nil || r.Basis != "nextDay" {
			t.Fatalf("reaction = %+v, want nextDay basis", r)
		}
	})
}

// 가장 최근 발표는 다음 거래일 종가가 아직 없다. 그때 0%로 그리면 "반응이 없었다"는
// 거짓말이 되므로 nil이어야 하고, 화면이 「집계 중」으로 말한다.
func TestReactionForMissingNextClose(t *testing.T) {
	e := model.EarningsEvent{Symbol: "TEST", Date: "2026-08-14", DateStatus: model.EarningsDateConfirmed}
	if r := reactionFor(e, closes); r != nil {
		t.Fatalf("reaction = %+v, want nil — 다음 거래일 종가가 없으면 0%%이 아니라 미집계다", r)
	}
}

// 발표일이 휴장일이면(주말 공시·데이터 어긋남) 그 직전 종가를 기준으로 삼는다.
func TestReactionForHolidayAnnouncement(t *testing.T) {
	e := model.EarningsEvent{Symbol: "TEST", Date: "2026-08-15", DateStatus: model.EarningsDateConfirmed}
	// 8/15는 종가가 없다 → 기준은 8/14, 반응은 그 뒤 첫 종가인데 목록에 없으므로 nil.
	if r := reactionFor(e, closes); r != nil {
		t.Fatalf("reaction = %+v, want nil (8/15 이후 종가가 없다)", r)
	}

	extended := append(append([]repository.DailyClose{}, closes...),
		repository.DailyClose{Date: day("2026-08-17"), Close: 108})
	r := reactionFor(e, extended)
	if r == nil {
		t.Fatal("reaction = nil")
	}
	// 기준 8/14 99 → 반응 8/17 108 = +9.1%
	if r.BaseDate != "2026-08-14" || r.Date != "2026-08-17" {
		t.Fatalf("dates = %s → %s, want 08-14 → 08-17", r.BaseDate, r.Date)
	}
}

// **커버리지 구멍을 7일치 등락으로 메우지 않는다.**
//
// stock_daily_closes는 최근 세션의 종목 커버리지가 얇다(실측: 08-07까지 16,190
// 종목/일 → 08-10부터 200~700). 가드가 없으면 08-13 발표에 08-14가 아니라 다음
// 주 종가가 잡혀 「다음 거래일 +28.2%」라고 적히는데, 실제로는 한 주 등락이다.
func TestReactionForRejectsDistantCloses(t *testing.T) {
	sparse := []repository.DailyClose{
		{Date: day("2026-08-07"), Close: 11140},
		{Date: day("2026-08-14"), Close: 14280},
	}
	e := model.EarningsEvent{Symbol: "237880", Date: "2026-08-13", DateStatus: model.EarningsDateConfirmed}
	if r := reactionFor(e, sparse); r != nil {
		t.Fatalf("reaction = %+v (%.1f%%), want nil — 기준 종가가 6일 전이라 「다음 세션」이 아니다",
			r, r.Pct)
	}

	// 같은 발표에 인접 종가가 있으면 정상적으로 낸다.
	dense := append(append([]repository.DailyClose{}, sparse...),
		repository.DailyClose{Date: day("2026-08-12"), Close: 12000},
		repository.DailyClose{Date: day("2026-08-13"), Close: 12500})
	sortByDate(dense)
	r := reactionFor(e, dense)
	if r == nil {
		t.Fatal("reaction = nil with adjacent closes")
	}
	if r.BaseDate != "2026-08-13" || r.Date != "2026-08-14" {
		t.Fatalf("dates = %s → %s, want 08-13 → 08-14", r.BaseDate, r.Date)
	}
}

func sortByDate(points []repository.DailyClose) {
	for i := 1; i < len(points); i++ {
		for j := i; j > 0 && points[j].Date.Before(points[j-1].Date); j-- {
			points[j], points[j-1] = points[j-1], points[j]
		}
	}
}

func TestReactionForGuards(t *testing.T) {
	e := model.EarningsEvent{Symbol: "TEST", Date: "2026-08-12", DateStatus: model.EarningsDateConfirmed}
	if r := reactionFor(e, nil); r != nil {
		t.Error("no closes must yield nil")
	}
	if r := reactionFor(e, closes[:1]); r != nil {
		t.Error("a single close cannot make a pair")
	}
	// 기준가 0으로 나누면 +Inf가 나가 화면이 깨진다.
	zero := []repository.DailyClose{
		{Date: day("2026-08-12"), Close: 0},
		{Date: day("2026-08-13"), Close: 50},
	}
	if r := reactionFor(e, zero); r != nil {
		t.Error("zero base close must yield nil, not Inf")
	}
	bad := model.EarningsEvent{Symbol: "TEST", Date: "not-a-date", DateStatus: model.EarningsDateConfirmed}
	if r := reactionFor(bad, closes); r != nil {
		t.Error("unparseable date must yield nil")
	}
}

func pct1(v float64) float64 {
	return math.Round(v*10) / 10
}

// DSN loc=Asia/Seoul scans DATE as KST midnight; the event date is UTC midnight.
// Equal/Before then miss the announcement-day close (장전·장중 전부 「집계 중」)
// and lastCloseBefore can pick that same day as the baseline (0%).
func TestReactionForKSTScannedCloses(t *testing.T) {
	kstCloses := []repository.DailyClose{
		{Date: kstDay("2026-08-10"), Close: 100},
		{Date: kstDay("2026-08-11"), Close: 110},
		{Date: kstDay("2026-08-12"), Close: 120},
		{Date: kstDay("2026-08-13"), Close: 90},
		{Date: kstDay("2026-08-14"), Close: 99},
	}
	base := model.EarningsEvent{
		Symbol:     "TGT",
		Date:       "2026-08-12",
		DateStatus: model.EarningsDateConfirmed,
	}

	t.Run("장전 — 당일 종가를 찾고 전일을 기준으로", func(t *testing.T) {
		e := base
		e.Session = model.EarningsSessionBeforeOpen
		r := reactionFor(e, kstCloses)
		if r == nil {
			t.Fatal("reaction = nil (KST DATE vs UTC 발표일이 Equal에 실패하던 경로)")
		}
		if r.BaseDate != "2026-08-11" || r.Date != "2026-08-12" {
			t.Fatalf("dates = %s → %s, want 08-11 → 08-12", r.BaseDate, r.Date)
		}
		if got := pct1(r.Pct); got != 9.1 {
			t.Errorf("pct = %v, want 9.1 — 0%%이면 발표일 종가를 기준으로 집은 것", got)
		}
	})

	t.Run("장중 — 장전과 같은 당일 짝", func(t *testing.T) {
		e := base
		e.Session = model.EarningsSessionDuringHours
		r := reactionFor(e, kstCloses)
		if r == nil || r.BaseDate != "2026-08-11" || r.Date != "2026-08-12" {
			t.Fatalf("reaction = %+v, want 08-11 → 08-12", r)
		}
	})

	t.Run("장후 — 다음 달력일이 첫 반응", func(t *testing.T) {
		e := base
		e.Session = model.EarningsSessionAfterClose
		r := reactionFor(e, kstCloses)
		if r == nil {
			t.Fatal("reaction = nil")
		}
		if r.BaseDate != "2026-08-12" || r.Date != "2026-08-13" {
			t.Fatalf("dates = %s → %s, want 08-12 → 08-13", r.BaseDate, r.Date)
		}
		if got := pct1(r.Pct); got != -25 {
			t.Errorf("pct = %v, want -25", got)
		}
	})
}

func TestGapDaysUsesCivilDate(t *testing.T) {
	// Hours()/24는 KST 0시 vs UTC 0시에서 9시간을 잃어 달력 5일을 4로 센다.
	if got := gapDays(kstDay("2026-08-18"), day("2026-08-13")); got != 5 {
		t.Fatalf("gapDays = %d, want 5", got)
	}
	if got := gapDays(day("2026-08-12"), day("2026-08-12")); got != 0 {
		t.Fatalf("same UTC day = %d, want 0", got)
	}
	if got := gapDays(kstDay("2026-08-12"), day("2026-08-12")); got != 0 {
		t.Fatalf("same civil day different loc = %d, want 0", got)
	}
}
