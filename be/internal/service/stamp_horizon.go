package service

import (
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
)

// Deadlines for stamps.
//
// The deadline is settled once, when the stamp is taken, and stored. It is not
// recomputed on read: the embedded holiday lists get corrected from time to
// time, and a correction must never move the deadline of a stamp that has
// already been judged.
//
// Every deadline lands on a session close rather than an arbitrary clock time —
// "1개월" means "the close a month from now", so the verdict always has a real
// settlement price behind it instead of a mid-session quote.

// scheduleForStamp maps an asset to the trading calendar that governs its
// closes. The US/ETF split matches benchmarkForStock and marketTradeDate — all
// three have to agree or a stamp's deadline, benchmark and trade date end up on
// different markets.
func scheduleForStamp(assetType model.QuoteAssetType) calendar.ScheduleID {
	switch assetType {
	case model.AssetUS, model.AssetETF:
		return calendar.ScheduleUS
	default:
		return calendar.ScheduleKRX
	}
}

// normalizeHorizonKind maps client input onto a known kind, defaulting to 1M.
func normalizeHorizonKind(kind string) model.StampHorizonKind {
	switch model.StampHorizonKind(strings.ToLower(strings.TrimSpace(kind))) {
	case model.HorizonSessionClose:
		return model.HorizonSessionClose
	case model.HorizonNextSessionClose:
		return model.HorizonNextSessionClose
	case model.Horizon1W:
		return model.Horizon1W
	case model.Horizon3M:
		return model.Horizon3M
	case model.HorizonCustom:
		return model.HorizonCustom
	default:
		return model.Horizon1M
	}
}

// nextTradingClose returns the first session close strictly after `after`.
// It walks forward day by day so weekends and holidays are skipped by the same
// calendar the pollers use.
func nextTradingClose(schedule calendar.ScheduleID, after time.Time) time.Time {
	loc := calendar.LocationFor(schedule)
	day := after.In(loc)
	// 21 days covers the longest KRX holiday cluster with room to spare; the
	// fallback below keeps a pathological calendar from returning a zero time.
	for i := 0; i < 21; i++ {
		candidate := day.AddDate(0, 0, i)
		if !calendar.IsTradingDay(schedule, candidate) {
			continue
		}
		closeAt := calendar.SessionCloseAt(schedule, candidate)
		if closeAt.After(after) {
			return closeAt
		}
	}
	return calendar.SessionCloseAt(schedule, day.AddDate(0, 0, 1))
}

// resolveDueAt turns the chosen horizon into a concrete deadline.
//
// customDay is only read for HorizonCustom and is interpreted in the market's
// own timezone — a US stamp due "on the 20th" means the 20th in New York.
func resolveDueAt(
	kind model.StampHorizonKind,
	assetType model.QuoteAssetType,
	stampedAt time.Time,
	customDay string,
) time.Time {
	schedule := scheduleForStamp(assetType)
	loc := calendar.LocationFor(schedule)
	local := stampedAt.In(loc)

	switch kind {
	case model.HorizonSessionClose:
		// "이번 장 종료" after the bell has already rung means the next one —
		// a deadline in the past would settle the stamp before it existed.
		if calendar.IsTradingDay(schedule, local) {
			if closeAt := calendar.SessionCloseAt(schedule, local); closeAt.After(stampedAt) {
				return closeAt
			}
		}
		return nextTradingClose(schedule, stampedAt)

	case model.HorizonNextSessionClose:
		return nextTradingClose(schedule, resolveDueAt(model.HorizonSessionClose, assetType, stampedAt, ""))

	case model.HorizonCustom:
		day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(customDay), loc)
		if err != nil {
			// An unparseable date is a client bug, not a reason to drop the
			// stamp — fall back to the default horizon.
			return resolveDueAt(model.Horizon1M, assetType, stampedAt, "")
		}
		if closeAt := calendar.SessionCloseAt(schedule, day); closeAt.After(stampedAt) {
			// A non-trading day settles at the next close that exists.
			if calendar.IsTradingDay(schedule, day) {
				return closeAt
			}
			return nextTradingClose(schedule, closeAt)
		}
		return nextTradingClose(schedule, stampedAt)

	case model.Horizon1W:
		return closeOnOrAfter(schedule, local.AddDate(0, 0, 7), stampedAt)
	case model.Horizon3M:
		return closeOnOrAfter(schedule, local.AddDate(0, 3, 0), stampedAt)
	default: // Horizon1M
		return closeOnOrAfter(schedule, local.AddDate(0, 1, 0), stampedAt)
	}
}

// closeOnOrAfter settles on `day`'s close, or the next trading close if that
// day is a holiday (or somehow already behind us).
func closeOnOrAfter(schedule calendar.ScheduleID, day, floor time.Time) time.Time {
	if calendar.IsTradingDay(schedule, day) {
		if closeAt := calendar.SessionCloseAt(schedule, day); closeAt.After(floor) {
			return closeAt
		}
	}
	return nextTradingClose(schedule, calendar.SessionCloseAt(schedule, day))
}

// HorizonLabels are the Korean names the client shows. Kept next to the kinds
// so a new horizon cannot ship with a missing label.
var HorizonLabels = map[model.StampHorizonKind]string{
	model.HorizonSessionClose:     "이번 장 종료",
	model.HorizonNextSessionClose: "다음 장 종료",
	model.Horizon1W:               "1주",
	model.Horizon1M:               "1개월",
	model.Horizon3M:               "3개월",
	model.HorizonCustom:           "직접 지정",
}
