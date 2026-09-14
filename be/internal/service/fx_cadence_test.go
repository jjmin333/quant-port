package service

import (
	"context"
	"testing"
	"time"
)

/*
환율 폴러의 주기가 세션을 따라가는지 못 박는다 (2026-09-11 사용자 요청 7).

배경 실측: 15초 주기는 하루 5,760호출인데 60초 간격 세 표본의 변동이
1342.85 → 1343.00 → 1342.92, 즉 분당 약 0.011%였다. 그 정밀도를 사는 데
`MARKET_INFO` 버킷의 거의 전부를 쓰고 있었다.
*/

func TestFxPollInterval_followsSession(t *testing.T) {
	live := &ExchangeRateService{}
	live.SetSessionProbe(func(context.Context) bool { return true })
	if got := live.pollInterval(context.Background()); got != fxPollInterval {
		t.Fatalf("열린 시장이 있으면 %v 여야 한다: %v", fxPollInterval, got)
	}

	closed := &ExchangeRateService{}
	closed.SetSessionProbe(func(context.Context) bool { return false })
	if got := closed.pollInterval(context.Background()); got != fxPollIntervalClosed {
		t.Fatalf("둘 다 닫혔으면 %v 여야 한다: %v", fxPollIntervalClosed, got)
	}
}

func TestFxPollInterval_unwiredStaysFast(t *testing.T) {
	// 세션을 모를 때 10분으로 늦추면 보이는 손해가 된다 — 모르면 1분이다.
	s := &ExchangeRateService{}
	if got := s.pollInterval(context.Background()); got != fxPollInterval {
		t.Fatalf("배선 없으면 %v 여야 한다: %v", fxPollInterval, got)
	}
}

func TestFxPollInterval_isNotFasterThanBefore(t *testing.T) {
	// 실수로 주기를 다시 줄이는 회귀를 막는다. 15초로 돌아가면 하루 5,760호출이다.
	if fxPollInterval < time.Minute {
		t.Fatalf("주기가 1분보다 짧다(%v) — MARKET_INFO 버킷을 혼자 태운다", fxPollInterval)
	}
	if fxPollIntervalClosed < fxPollInterval {
		t.Fatalf("마감 주기(%v)가 장중 주기(%v)보다 짧다", fxPollIntervalClosed, fxPollInterval)
	}
}
