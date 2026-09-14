package model

import (
	"testing"
	"time"
)

func d(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		panic(err)
	}
	return t
}

func q(year int, period, periodEnd string) QuarterToMatch {
	return QuarterToMatch{FiscalYear: year, FiscalPeriod: period, PeriodEnd: d(periodEnd)}
}

func cand(filed string) AnnounceCandidate {
	return AnnounceCandidate{Filed: d(filed), Session: "amc"}
}

// 실측 AAPL — 회계연도가 9월에 끝나 기간말 뒤 약 30일에 발표한다.
func TestMatchAnnouncements_calendarFiscalYear(t *testing.T) {
	got := MatchAnnouncements(
		[]QuarterToMatch{
			q(2026, "3Q", "2026-06-30"),
			q(2026, "2Q", "2026-03-31"),
			q(2026, "1Q", "2025-12-31"),
			q(2025, "4Q", "2025-09-30"),
		},
		[]AnnounceCandidate{
			cand("2026-07-30"), cand("2026-04-30"), cand("2026-01-29"), cand("2025-10-30"),
		},
	)
	if len(got) != 4 {
		t.Fatalf("matched = %d, want 4", len(got))
	}
	want := []string{"2026-07-30", "2026-04-30", "2026-01-29", "2025-10-30"}
	for i, w := range want {
		if got[i].Filed.Format("2006-01-02") != w {
			t.Fatalf("%d: %s, want %s", i, got[i].Filed.Format("2006-01-02"), w)
		}
		if got[i].GapDays <= 0 {
			t.Fatalf("%d: gap = %d, want 양수 (발표가 기간말 뒤)", i, got[i].GapDays)
		}
	}
}

// 실측 NVDA — 회계연도가 1월에 끝나 Finnhub의 반올림이 발표보다 **뒤**에 온다.
// 이 경우 gap이 음수이므로, 부호를 보고 버리면 안 된다.
func TestMatchAnnouncements_offCalendarFiscalYearGivesNegativeGap(t *testing.T) {
	got := MatchAnnouncements(
		[]QuarterToMatch{
			q(2027, "1Q", "2026-06-30"),
			q(2026, "4Q", "2026-03-31"),
			q(2026, "3Q", "2025-12-31"),
			q(2026, "2Q", "2025-09-30"),
		},
		[]AnnounceCandidate{
			cand("2026-05-20"), cand("2026-02-25"), cand("2025-11-19"), cand("2025-08-27"),
		},
	)
	if len(got) != 4 {
		t.Fatalf("matched = %d, want 4 (음수 gap도 유효하다)", len(got))
	}
	if got[0].GapDays != -41 {
		t.Fatalf("gap = %d, want -41", got[0].GapDays)
	}
}

// AMAT 실측 — 48일이라 45일 허용오차에서는 놓쳤다. 60일이면 잡힌다.
func TestMatchAnnouncements_toleranceCovers48Days(t *testing.T) {
	got := MatchAnnouncements(
		[]QuarterToMatch{q(2026, "3Q", "2026-09-30")},
		[]AnnounceCandidate{cand("2026-11-17")}, // +48일
	)
	if len(got) != 1 {
		t.Fatalf("matched = %d, want 1 (48일은 허용 범위)", len(got))
	}
}

// 허용오차를 넘으면 그 분기만 포기한다 — 나머지는 살린다(부분 매칭).
func TestMatchAnnouncements_partialWhenOneQuarterHasNoFiling(t *testing.T) {
	got := MatchAnnouncements(
		[]QuarterToMatch{
			q(2026, "2Q", "2026-06-30"),
			q(2026, "1Q", "2026-03-31"),
		},
		[]AnnounceCandidate{cand("2026-07-28")}, // 1분기용 8-K가 없다
	)
	if len(got) != 1 {
		t.Fatalf("matched = %d, want 1 (부분 매칭)", len(got))
	}
	if got[0].Quarter.FiscalPeriod != "2Q" {
		t.Fatalf("matched %s, want 2Q", got[0].Quarter.FiscalPeriod)
	}
}

// **이 테스트가 가장 중요하다.** 한 칸 밀리면 모든 분기가 그럴듯한 날짜를 갖고,
// 화면에서는 「과거 캘린더가 채워졌다」로만 보인다. 단조성 위반이면 통째로 버린다.
func TestMatchAnnouncements_rejectsWholeSymbolWhenOrderInverts(t *testing.T) {
	// 최근 분기가 더 오래된 8-K를 집고, 오래된 분기가 더 최근 8-K를 집는 배치.
	got := MatchAnnouncements(
		[]QuarterToMatch{
			q(2026, "2Q", "2026-06-30"),
			q(2026, "1Q", "2026-06-20"), // 기간말이 비정상적으로 붙어 있다
		},
		[]AnnounceCandidate{cand("2026-06-10"), cand("2026-07-25")},
	)
	if got != nil {
		for i, m := range got {
			t.Logf("  %d: %s -> %s", i, m.Quarter.FiscalPeriod, m.Filed.Format("2006-01-02"))
		}
		t.Fatal("순서가 뒤집힌 매칭이 통과했다 — 단조성 가드가 깨졌다")
	}
}

// 같은 8-K를 두 분기가 나눠 가질 수 없다.
func TestMatchAnnouncements_oneFilingPerQuarter(t *testing.T) {
	got := MatchAnnouncements(
		[]QuarterToMatch{
			q(2026, "2Q", "2026-06-30"),
			q(2026, "1Q", "2026-03-31"),
		},
		[]AnnounceCandidate{cand("2026-05-15")}, // 둘 다에서 60일 안
	)
	if len(got) != 1 {
		t.Fatalf("matched = %d, want 1 (8-K 하나는 한 분기만)", len(got))
	}
}

func TestMatchAnnouncements_emptyInputs(t *testing.T) {
	if got := MatchAnnouncements(nil, []AnnounceCandidate{cand("2026-01-01")}); got != nil {
		t.Fatal("분기가 없으면 nil이어야 한다")
	}
	if got := MatchAnnouncements([]QuarterToMatch{q(2026, "1Q", "2026-03-31")}, nil); got != nil {
		t.Fatal("8-K가 없으면 nil이어야 한다")
	}
}

// 분기 간격(90일)보다 허용오차가 작아야 옆 분기를 침범하지 않는다.
func TestAnnounceMatchTolerance_staysBelowQuarterSpacing(t *testing.T) {
	if AnnounceMatchToleranceDays >= 90 {
		t.Fatalf("허용오차 %d일 — 분기 간격 90일 이상이면 옆 분기의 발표를 집어간다",
			AnnounceMatchToleranceDays)
	}
}
