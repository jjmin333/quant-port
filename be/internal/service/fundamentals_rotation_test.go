package service

import (
	"context"
	"testing"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
2026-09-10 사고의 재발 방지선을 시험으로 못 박는다 (2026-09-11 사용자 요청 6).

그날의 실패는 「전 종목 스윕이 DART 하루치 18,000을 다 썼고, 저녁 DART 잡 셋이
한 종목도 못 불렀다」였다. 그래서 지켜야 하는 성질은 둘이다 —
**한 회차의 크기에 상한이 있다**, 그리고 **예약분은 절대 건드리지 않는다**.
*/

func TestFundamentalsRotation_reserveSurvivesAFullRun(t *testing.T) {
	b := apibudget.NewManager(nil)
	b.SetLimits(apibudget.ProviderDART, apibudget.Limits{DailyQuota: 18000})
	ctx := context.Background()

	// 한 주기의 국내 상한(400)은 예산 가드를 통과해야 한다 — 안 그러면 이 갱신이
	// 영원히 아무것도 안 한다.
	allowed := backfillMaxSymbols(
		ctx, b, apibudget.ProviderDART,
		fundamentalsDARTReserve, fundamentalsDARTPerSymbol,
		fundamentalsRotationLimitDomestic,
	)
	if allowed != fundamentalsRotationLimitDomestic {
		t.Fatalf("빈 한도에서는 한 주기가 통째로 통과해야 한다: %d", allowed)
	}

	// 그리고 그 한 주기가 쓰는 최대 호출은 예약분을 남긴다.
	worstCase := fundamentalsRotationLimitDomestic * fundamentalsDARTPerSymbol
	if left := 18000 - worstCase; left < fundamentalsDARTReserve {
		t.Fatalf(
			"한 주기(%d종목 × %d호출 = %d)가 예약분 %d를 먹는다: 남는 것 %d",
			fundamentalsRotationLimitDomestic, fundamentalsDARTPerSymbol,
			worstCase, fundamentalsDARTReserve, left,
		)
	}
}

func TestFundamentalsRotation_stopsWhenOnlyTheReserveIsLeft(t *testing.T) {
	b := apibudget.NewManager(nil)
	b.SetLimits(apibudget.ProviderDART, apibudget.Limits{DailyQuota: 18000})
	ctx := context.Background()

	// 이미 9,100콜을 썼다 = 남은 8,900은 예약분(9,000)보다 적다.
	for i := 0; i < 9100; i++ {
		b.Record(ctx, apibudget.ProviderDART)
	}
	got := backfillMaxSymbols(
		ctx, b, apibudget.ProviderDART,
		fundamentalsDARTReserve, fundamentalsDARTPerSymbol,
		fundamentalsRotationLimitDomestic,
	)
	if got != 0 {
		t.Fatalf("예약분만 남았으면 한 종목도 돌지 않아야 한다: %d", got)
	}
}

func TestFundamentalsRotation_notWiredIsANoOp(t *testing.T) {
	// 배선이 빠졌을 때 「전 종목이라도 돌자」로 떨어지지 않는다 — 그게 사고 당일의 동작이다.
	svc := &FundamentalsService{}
	res, err := svc.RefreshRotation(context.Background(), "domestic", 400)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Attempted != 0 || res.Stopped == "" {
		t.Fatalf("배선 없으면 아무것도 시도하지 않고 이유를 남겨야 한다: %+v", res)
	}
}

func TestFundamentalsRotation_perSymbolCostCoversMeasuredAverage(t *testing.T) {
	// 실측 6.4(2026-09-10: 2,800종목 · 18,000호출)를 밑돌면 가드가 한도를 넘긴다.
	const measured = 6.4
	if float64(fundamentalsDARTPerSymbol) < measured {
		t.Fatalf(
			"종목당 호출 추정(%d)이 실측(%.1f)보다 낮다 — 가드가 한도를 넘긴다",
			fundamentalsDARTPerSymbol, measured,
		)
	}
}
