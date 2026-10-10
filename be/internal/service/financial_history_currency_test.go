package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

func TestForeignCurrencyFromCalcError(t *testing.T) {
	cases := []struct {
		in    string
		code  string
		mixed bool
	}{
		{"roe,debt_ratio,current_ratio,currency_twd_usd", "TWD", true},
		{"currency_jpy", "JPY", false},
		{"current_ratio", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		code, mixed := foreignCurrencyFromCalcError(c.in, "USD")
		if code != c.code || mixed != c.mixed {
			t.Errorf("%q → %q %v", c.in, code, mixed)
		}
	}
}

func TestDropMislabeledPeriods(t *testing.T) {
	at := func(y int, m time.Month, d int) *time.Time { v := time.Date(y, m, d, 0, 0, 0, 0, time.UTC); return &v }
	in := []model.FinancialHistoryPeriod{
		{FiscalYear: 2017, PeriodEnd: at(2015, 12, 31)}, // TSM: 2017 보고서의 2015 비교 수치
		{FiscalYear: 2024, PeriodEnd: at(2023, 12, 30)}, // 애플 FY2024 1분기 — 정상
		{FiscalYear: 2024, PeriodEnd: at(2024, 12, 31)},
	}
	out := dropMislabeledPeriods(in)
	if len(out) != 2 || out[0].FiscalYear != 2024 {
		t.Fatalf("got %+v", out)
	}
}
