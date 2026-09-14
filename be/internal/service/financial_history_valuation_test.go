package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func fhF(v float64) *float64 { return &v }
func fhI(v int64) *int64     { return &v }

func TestTTMEPSNeedsFourQuarters(t *testing.T) {
	periods := []model.FinancialHistoryPeriod{
		{EPSBasic: fhF(100)},
		{EPSBasic: fhF(200)},
		{EPSBasic: fhF(300)},
		{EPSBasic: fhF(400)},
		{EPSBasic: fhF(500)},
	}
	// 앞의 세 분기는 4개를 못 채운다 — 세 분기만 더해 TTM이라 부르면 PER이
	// 부풀어 「싸 보이는」 종목이 된다.
	for i := 0; i < 3; i++ {
		if got := ttmEPS(periods, i); got != nil {
			t.Fatalf("ttmEPS(%d) = %v, want nil", i, *got)
		}
	}
	if got := ttmEPS(periods, 3); got == nil || *got != 1000 {
		t.Fatalf("ttmEPS(3) = %v, want 1000", got)
	}
	if got := ttmEPS(periods, 4); got == nil || *got != 1400 {
		t.Fatalf("ttmEPS(4) = %v, want 1400", got)
	}
}

func TestTTMEPSRejectsHole(t *testing.T) {
	periods := []model.FinancialHistoryPeriod{
		{EPSBasic: fhF(100)},
		{EPSBasic: nil}, // 계정명을 못 읽은 분기
		{EPSBasic: fhF(300)},
		{EPSBasic: fhF(400)},
	}
	if got := ttmEPS(periods, 3); got != nil {
		t.Fatalf("ttmEPS = %v, want nil (구멍이 있으면 TTM을 만들지 않는다)", *got)
	}
}

func TestSharesFromEPS(t *testing.T) {
	p := model.FinancialHistoryPeriod{NetIncome: fhI(1_000_000), EPSBasic: fhF(100)}
	if got := sharesFromEPS(p); got == nil || *got != 10_000 {
		t.Fatalf("shares = %v, want 10000", got)
	}
	// EPS가 0에 가까우면 나눗셈이 폭발한다.
	if got := sharesFromEPS(model.FinancialHistoryPeriod{NetIncome: fhI(1000), EPSBasic: fhF(0)}); got != nil {
		t.Fatalf("shares = %v, want nil", *got)
	}
	// 적자 분기도 주식수는 양수로 나와야 한다.
	loss := model.FinancialHistoryPeriod{NetIncome: fhI(-1_000_000), EPSBasic: fhF(-100)}
	if got := sharesFromEPS(loss); got == nil || *got != 10_000 {
		t.Fatalf("shares(loss) = %v, want 10000", got)
	}
}

func TestBookValuePerSharePrefersPeriodShares(t *testing.T) {
	p := model.FinancialHistoryPeriod{
		TotalEquity: fhI(1_000_000),
		NetIncome:   fhI(100_000),
		EPSBasic:    fhF(10), // → 그 분기 주식수 10,000
	}
	// 저장된 현재 주식수(20,000)를 쓰면 BPS가 절반이 된다. 그 사이 증자를
	// 과거로 소급하지 않도록 분기 주식수가 이긴다.
	got := bookValuePerShareAt(p, fhI(20_000))
	if got == nil || *got != 100 {
		t.Fatalf("bps = %v, want 100 (1,000,000 / 10,000)", got)
	}
}

func TestBookValuePerShareFallsBackToStoredShares(t *testing.T) {
	p := model.FinancialHistoryPeriod{TotalEquity: fhI(1_000_000)} // EPS·순이익 없음
	got := bookValuePerShareAt(p, fhI(20_000))
	if got == nil || *got != 50 {
		t.Fatalf("bps = %v, want 50", got)
	}
	if got := bookValuePerShareAt(p, nil); got != nil {
		t.Fatalf("bps = %v, want nil (주식수를 어디서도 못 구하면 비운다)", *got)
	}
}

func TestBookValuePerShareRejectsNonPositiveEquity(t *testing.T) {
	// 자본잠식이면 주당순자산이 음수이고 PBR은 뜻을 잃는다.
	p := model.FinancialHistoryPeriod{TotalEquity: fhI(-500), NetIncome: fhI(100), EPSBasic: fhF(1)}
	if got := bookValuePerShareAt(p, fhI(100)); got != nil {
		t.Fatalf("bps = %v, want nil", *got)
	}
}
