package apibudget

import (
	"context"
	"testing"
)

/*
RecordN — 「한 논리적 작업에 상류 호출 여러 개」를 정직하게 센다
(2026-09-11 사용자 요청 7).

배경: FRED 배치가 Check·Record 를 한 번씩만 하면서 시리즈당 HTTP 2회를 냈다.
지수 6종이면 실제 12회인데 카운터에는 1이 적혔다 — 계기가 12분의 1을 말하고 있었다.
*/

func TestRecordN_countsEveryCall(t *testing.T) {
	// redis 없이 메모리 폴백만으로 센다.
	b := NewManager(nil)
	b.SetLimits(ProviderFRED, Limits{DailyQuota: 5000})
	ctx := context.Background()

	before := b.Headroom(ctx, ProviderFRED).Calls
	b.RecordN(ctx, ProviderFRED, 12)
	after := b.Headroom(ctx, ProviderFRED).Calls

	if before-after != 12 {
		t.Fatalf("12회를 기록했는데 헤드룸이 %d 만큼 줄었다", before-after)
	}
}

func TestRecordN_oneIsSameAsRecord(t *testing.T) {
	a := NewManager(nil)
	a.SetLimits(ProviderFRED, Limits{DailyQuota: 5000})
	b := NewManager(nil)
	b.SetLimits(ProviderFRED, Limits{DailyQuota: 5000})
	ctx := context.Background()

	a.Record(ctx, ProviderFRED)
	b.RecordN(ctx, ProviderFRED, 1)

	if a.Headroom(ctx, ProviderFRED).Calls != b.Headroom(ctx, ProviderFRED).Calls {
		t.Fatal("RecordN(1) 이 Record() 와 달라졌다")
	}
}

func TestRecordN_ignoresNonPositive(t *testing.T) {
	b := NewManager(nil)
	b.SetLimits(ProviderFRED, Limits{DailyQuota: 5000})
	ctx := context.Background()

	before := b.Headroom(ctx, ProviderFRED).Calls
	b.RecordN(ctx, ProviderFRED, 0)
	b.RecordN(ctx, ProviderFRED, -3)
	if b.Headroom(ctx, ProviderFRED).Calls != before {
		t.Fatal("0·음수 기록이 카운터를 움직였다")
	}
}
