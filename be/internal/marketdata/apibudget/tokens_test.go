package apibudget

import (
	"context"
	"testing"
	"time"
)

// newTestManager builds a Redis-less manager (in-memory counters) with one
// provider's limits set for the test.
func newTestManager(lim Limits) *Manager {
	m := NewManager(nil)
	m.SetLimits(ProviderOpenAI, lim)
	return m
}

func TestCheckTokensRejectsCallThatWouldCrossDailyCeiling(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(Limits{DailyQuota: 500, TokensPerDay: 10_000})

	m.RecordTokens(ctx, ProviderOpenAI, 8_000)

	// Comfortably inside the remaining 2,000.
	if err := m.CheckTokens(ctx, ProviderOpenAI, 1_500); err != nil {
		t.Fatalf("1500 tokens should fit in the remaining 2000: %v", err)
	}
	// The point of the pre-flight check: reject the call that would cross
	// the line, not merely the one after it.
	if err := m.CheckTokens(ctx, ProviderOpenAI, 2_500); err == nil {
		t.Fatal("2500 tokens must be rejected against a remaining 2000")
	}
	// Check alone still passes, since nothing has actually overshot yet —
	// which is exactly why batch callers need CheckTokens.
	if err := m.Check(ctx, ProviderOpenAI); err != nil {
		t.Fatalf("Check should still pass below the ceiling: %v", err)
	}
}

func TestCheckTokensRejectsAgainstMinuteCeiling(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(Limits{TokensPerMinute: 5_000})

	m.RecordTokens(ctx, ProviderOpenAI, 4_000)

	if err := m.CheckTokens(ctx, ProviderOpenAI, 500); err != nil {
		t.Fatalf("500 tokens should fit in the remaining 1000: %v", err)
	}
	if err := m.CheckTokens(ctx, ProviderOpenAI, 2_000); err == nil {
		t.Fatal("2000 tokens must be rejected against a remaining 1000")
	}
}

func TestCheckBlocksOnceDailyTokensExhausted(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(Limits{TokensPerDay: 1_000})

	m.RecordTokens(ctx, ProviderOpenAI, 1_000)

	if err := m.Check(ctx, ProviderOpenAI); err == nil {
		t.Fatal("Check must fail once the daily token budget is spent")
	}
}

func TestHeadroomReportsRemainingAllowance(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(Limits{DailyQuota: 10, TokensPerDay: 10_000, TokensPerMinute: 4_000})

	m.Record(ctx, ProviderOpenAI)
	m.Record(ctx, ProviderOpenAI)
	m.RecordTokens(ctx, ProviderOpenAI, 2_500)

	head := m.Headroom(ctx, ProviderOpenAI)
	if head.Calls != 8 {
		t.Fatalf("Calls = %d, want 8", head.Calls)
	}
	if head.DayTokens != 7_500 {
		t.Fatalf("DayTokens = %d, want 7500", head.DayTokens)
	}
	if head.MinuteTokens != 1_500 {
		t.Fatalf("MinuteTokens = %d, want 1500", head.MinuteTokens)
	}
}

func TestHeadroomClampsAndReportsUnbounded(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(Limits{DailyQuota: 1})

	m.Record(ctx, ProviderOpenAI)
	m.Record(ctx, ProviderOpenAI)

	head := m.Headroom(ctx, ProviderOpenAI)
	if head.Calls != 0 {
		t.Fatalf("overspent Calls = %d, want clamp at 0", head.Calls)
	}
	// An unset ceiling must read as "unbounded", not as "nothing left" —
	// a batch sizing itself off this would otherwise refuse to run at all.
	if head.DayTokens != -1 {
		t.Fatalf("unset TokensPerDay = %d, want -1", head.DayTokens)
	}
}

func TestRecordTokensChargesDayAndMinute(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(Limits{TokensPerDay: 100_000, TokensPerMinute: 100_000})

	m.RecordTokens(ctx, ProviderOpenAI, 1_200)
	m.RecordTokens(ctx, ProviderOpenAI, 800)

	usage := m.Usage(ctx, ProviderOpenAI)
	if usage.DayTokens != 2_000 {
		t.Fatalf("DayTokens = %d, want 2000", usage.DayTokens)
	}
	if usage.MinuteTokens != 2_000 {
		t.Fatalf("MinuteTokens = %d, want 2000", usage.MinuteTokens)
	}
}

/*
월 허용량을 남은 날로 나눈 「오늘치」.

이 시험이 있는 이유는 실패 모양이다. 월 한도 하나만 있으면 한도를 다 쓴 순간부터
**남은 날이 통째로 0**이 된다 — 2026-09-13~14에 번역 버그가 이틀에 1.5M자를 태워
9월의 남은 열엿새가 그렇게 될 뻔했다. 고정 하루 한도도 그 상황을 못 고친다.
*/
func TestDailyShareSpreadsWhatIsLeftOverTheDaysThatRemain(t *testing.T) {
	loc := time.FixedZone("KST", 9*3600)
	const monthLimit = 2_000_000

	tests := []struct {
		name               string
		monthUsed, dayUsed int64
		day                int
		want               int64
	}{
		{
			// 평상시 — 9월 1일, 아직 아무것도 안 썼다. 2M / 30일.
			name: "달 첫날은 월 한도를 날수로 나눈 값",
			day:  1, want: 2_000_000 / 30,
		},
		{
			// 사고 직후 — 어제까지 426,939, 오늘 1,092,484를 썼다(실측).
			// 남은 480,577을 오늘 포함 17일로 나눈다.
			name:      "앞에서 과하게 쓴 달은 하루치가 저절로 줄어든다",
			monthUsed: 1_519_423, dayUsed: 1_092_484, day: 14,
			want: (2_000_000 - (1_519_423 - 1_092_484)) / 17,
		},
		{
			// 조용한 달을 보내면 남은 날의 하루치가 오히려 늘어난다.
			name:      "적게 쓴 달은 하루치가 늘어난다",
			monthUsed: 100_000, dayUsed: 0, day: 25,
			want: (2_000_000 - 100_000) / 6,
		},
		{
			name:      "말일에는 남은 것을 하루에 다 준다",
			monthUsed: 1_900_000, dayUsed: 0, day: 30,
			want: 2_000_000 - 1_900_000,
		},
		{
			name:      "달이 이미 한도를 넘었으면 0",
			monthUsed: 2_500_000, dayUsed: 0, day: 20,
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 9, tc.day, 12, 0, 0, 0, loc)
			got := dailyShare(monthLimit, tc.monthUsed, tc.dayUsed, now)
			if got != tc.want {
				t.Fatalf("dailyShare = %d, want %d", got, tc.want)
			}
		})
	}
}

// 오늘을 **포함해** 나눠야 한다 — 빼면 말일에 0으로 나눈다.
func TestDailyShareIncludesTodayAndNeverDividesByZero(t *testing.T) {
	loc := time.FixedZone("KST", 9*3600)
	for _, day := range []int{28, 29, 30} {
		now := time.Date(2026, 9, day, 23, 59, 0, 0, loc)
		if got := dailyShare(300, 0, 0, now); got <= 0 {
			t.Fatalf("9월 %d일의 하루치가 %d다", day, got)
		}
	}
	// 2월처럼 날수가 다른 달도 그 달의 실제 날수를 본다.
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, loc)
	if got := dailyShare(2_800, 0, 0, feb); got != 100 {
		t.Fatalf("2026년 2월 1일 하루치 = %d, want 100 (28일)", got)
	}
}

/*
과하게 쓴 **다음 날**에 하루치가 실제로 내려간다.

이 시험이 따로 있는 이유는 `dailyShare`가 「어제까지」를 기준으로 나누기 때문이다.
사고가 난 당일에는 하루치가 아직 안 내려가고(그날은 월 한도가 막는다) 다음 날부터
내려간다 — 실측 2026-09-14/15가 그 두 날이다.
*/
func TestDailyShareDropsTheDayAfterAnOverspend(t *testing.T) {
	loc := time.FixedZone("KST", 9*3600)
	const monthLimit = 2_000_000

	// 9/14 — 버그가 그날 1,092,484자를 태웠다. 어제까지는 426,939자뿐이라
	// 하루치는 아직 높지만, day(1,092,484)가 이미 그 위라 어차피 막힌다.
	today := dailyShare(monthLimit, 1_519_423, 1_092_484, time.Date(2026, 9, 14, 18, 0, 0, 0, loc))
	if today != (monthLimit-426_939)/17 {
		t.Fatalf("9/14 하루치 = %d", today)
	}
	if 1_092_484 <= today {
		t.Fatalf("9/14 에 이미 하루치를 넘겨야 한다: used=1092484 share=%d", today)
	}

	// 9/15 — 어제까지가 1,519,423 으로 올라간다. 남은 480,577 을 16일로 나눈다.
	tomorrow := dailyShare(monthLimit, 1_519_423, 0, time.Date(2026, 9, 15, 9, 0, 0, 0, loc))
	if want := int64(monthLimit-1_519_423) / 16; tomorrow != want {
		t.Fatalf("9/15 하루치 = %d, want %d", tomorrow, want)
	}
	if tomorrow >= today {
		t.Fatalf("다음 날 하루치가 안 내려갔다: %d → %d", today, tomorrow)
	}
}
