package apibudget

import (
	"context"
	"testing"
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
