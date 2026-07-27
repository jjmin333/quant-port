package calendar

import (
	"time"

	"github.com/enact/quant-port/be/internal/toss"
)

// USAfterMarketEnded reports whether US after-market session has ended for today.
func USAfterMarketEnded(us *toss.UsMarketCalendarResponse, now time.Time) bool {
	if us == nil || us.Today.AfterMarket == nil {
		return false
	}
	end, ok := parseMarketTime(us.Today.AfterMarket.EndTime)
	if !ok {
		return false
	}
	return now.After(end)
}

// KRIntegratedAfterEnded reports whether KRX integrated after-market has ended.
func KRIntegratedAfterEnded(kr *toss.KrMarketCalendarResponse, now time.Time) bool {
	if kr == nil || kr.Today.Integrated == nil || kr.Today.Integrated.AfterMarket == nil {
		return false
	}
	end, ok := parseMarketTime(kr.Today.Integrated.AfterMarket.EndTime)
	if !ok {
		return false
	}
	return now.After(end)
}
