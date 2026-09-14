package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

func TestPreviousRegularSessionBounds_krCalendar(t *testing.T) {
	cals := marketCalendars{
		kr: &toss.KrMarketCalendarResponse{
			PreviousBusinessDay: toss.KrMarketDay{
				Integrated: &toss.IntegratedHour{
					RegularMarket: &toss.RegularMarketSession{
						StartTime: "2026-07-30T09:00:00+09:00",
						EndTime:   "2026-07-30T15:30:00+09:00",
					},
				},
			},
		},
	}
	start, end, ok := previousRegularSessionBounds("domestic", cals, time.Date(2026, 7, 31, 10, 0, 0, 0, kst))
	if !ok {
		t.Fatal("expected bounds")
	}
	if start.Hour() != 9 || end.Hour() != 15 || end.Minute() != 30 {
		t.Fatalf("bounds = %v ~ %v", start, end)
	}
}

func TestKrPreviousRegularSessionFallback_skipsWeekend(t *testing.T) {
	monday := time.Date(2026, 7, 27, 10, 0, 0, 0, kst) // Monday
	start, end, ok := krPreviousRegularSessionFallback(monday)
	if !ok {
		t.Fatal("expected fallback bounds")
	}
	if start.Weekday() != time.Friday {
		t.Fatalf("start weekday = %v want Friday", start.Weekday())
	}
	if end.Hour() != 15 || end.Minute() != 30 {
		t.Fatalf("end = %v want 15:30", end)
	}
}

func TestComputeQuoteChange_usesPrevClose(t *testing.T) {
	change, rate := computeQuoteChange(1_647_000, 1_322_000)
	if change != 325_000 {
		t.Fatalf("change = %v want 325000", change)
	}
	if rate < 24.5 || rate > 24.7 {
		t.Fatalf("rate = %.2f want ~24.58", rate)
	}
}

// 마감 후에도 기준은 전일 종가다 — 당일 정규장 종가를 기준으로 삼으면 하루
// 등락률이 아니라 시간외 등락률이 된다(실측 2026-09-01 삼성전자 −2.11% vs −1.16%).
func TestComputeQuoteChange_closedSessionStillUsesPrevClose(t *testing.T) {
	change, rate := computeQuoteChange(255_000, 258_000)
	if change != -3_000 {
		t.Fatalf("change = %v want -3000", change)
	}
	if rate < -1.17 || rate > -1.15 {
		t.Fatalf("rate = %.4f want ~-1.163", rate)
	}
}
