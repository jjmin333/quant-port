package service

import (
	"context"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

/*
장이 닫혀 있는 동안 화면이 물어보는 두 가지 — **언제 닫혔나 · 언제 다시 여나**.

# 왜 서버가 답하나 (2026-09-04 사용자 지적)

두 화면이 마감 중에도 시각을 「지금」으로 찍고 있었다. 종목 상세 머리줄의
「오전 12:33:08 기준」이 그것이다 — 새벽 0시 33분에 만들어진 값이 아니라
**전날 20:00에 멈춘 값**인데, 화면은 자기가 응답을 받은 시각을 적었다.

그리고 그 시각을 계속 새로 받고 있었다: 상세 화면이 2초마다
`/quotes/{sym}/terminal-snapshot`을 불렀다. 가격이 다음 개장까지 바뀔 수 없는
구간이라 그 왕복은 전부 값이 없다(실측 로그: 마감 후 005930 한 종목이 2초마다).

화면이 스스로 계산하지 않는 이유는 `lib/market-session`에 이미 적혀 있다 —
세션 창은 종목마다 다르고(넥스트레이드 08:00, 미국은 서머타임에 창이 밀린다),
브라우저에서 다시 계산하면 서버 판정과 어긋나는 두 번째 진실이 생긴다. 그래서
세션을 판정하는 자리(`detectMarketSession`)가 그 경계 시각까지 함께 답한다.
*/

// MarketClock is one market's session plus, when that session is "closed", the
// two instants a screen needs: when it last closed and when it opens next.
//
// 열려 있는 동안 두 시각은 비어 있다 — 그때 화면이 알아야 할 것은 「지금」이고,
// 빈 값이 곧 「지금 움직이는 중」이라는 뜻이다.
type MarketClock struct {
	Session    string `json:"marketSession,omitempty"`
	ClosedAt   string `json:"sessionClosedAt,omitempty"`
	NextOpenAt string `json:"nextSessionAt,omitempty"`
}

// Closed reports whether prices cannot move until NextOpenAt.
func (c MarketClock) Closed() bool { return c.Session == "closed" }

// sessionWindow is one contiguous stretch during which a market trades.
type sessionWindow struct{ start, end time.Time }

func appendWindow(out []sessionWindow, startRaw, endRaw string) []sessionWindow {
	start, ok1 := parseCandleTime(startRaw)
	end, ok2 := parseCandleTime(endRaw)
	if !ok1 || !ok2 || !end.After(start) {
		return out
	}
	return append(out, sessionWindow{start: start, end: end})
}

func krDayWindows(out []sessionWindow, day toss.KrMarketDay) []sessionWindow {
	h := day.Integrated
	if h == nil {
		return out
	}
	if h.PreMarket != nil {
		out = appendWindow(out, h.PreMarket.StartTime, h.PreMarket.EndTime)
	}
	if h.RegularMarket != nil {
		out = appendWindow(out, h.RegularMarket.StartTime, h.RegularMarket.EndTime)
	}
	if h.AfterMarket != nil {
		out = appendWindow(out, h.AfterMarket.StartTime, h.AfterMarket.EndTime)
	}
	return out
}

func usDayWindows(out []sessionWindow, day toss.UsMarketDay) []sessionWindow {
	if day.PreMarket != nil {
		out = appendWindow(out, day.PreMarket.StartTime, day.PreMarket.EndTime)
	}
	if day.DayMarket != nil {
		out = appendWindow(out, day.DayMarket.StartTime, day.DayMarket.EndTime)
	}
	if day.RegularMarket != nil {
		out = appendWindow(out, day.RegularMarket.StartTime, day.RegularMarket.EndTime)
	}
	if day.AfterMarket != nil {
		out = appendWindow(out, day.AfterMarket.StartTime, day.AfterMarket.EndTime)
	}
	return out
}

/*
marketSessionWindows — 어제·오늘·내일 세 영업일의 거래 창을 모두 모은다.

세 날을 다 보는 이유는 `detectUsSessionAcrossDays`와 같다: 미국 정규장은 KST
자정을 넘고, Toss 달력의 `today`는 KST 기준이다. 지금 돌아가고 있는(또는 방금
끝난) 창이 `previousBusinessDay` 쪽에 들어 있는 구간이 하루에 4시간 30분 있다.

창은 오프셋이 붙은 절대 시각이라 세 날을 다 봐도 겹치거나 어긋나지 않는다.
*/
func marketSessionWindows(assetType string, cals marketCalendars) []sessionWindow {
	out := make([]sessionWindow, 0, 12)
	if assetType == string(model.AssetUS) || assetType == string(model.AssetETF) {
		if cals.us == nil {
			return nil
		}
		out = usDayWindows(out, cals.us.PreviousBusinessDay)
		out = usDayWindows(out, cals.us.Today)
		return usDayWindows(out, cals.us.NextBusinessDay)
	}
	if cals.kr == nil {
		return nil
	}
	out = krDayWindows(out, cals.kr.PreviousBusinessDay)
	out = krDayWindows(out, cals.kr.Today)
	return krDayWindows(out, cals.kr.NextBusinessDay)
}

/*
marketSessionEdges — now를 기준으로 **마지막으로 닫힌 시각**과 **다음으로 열리는
시각**을 고른다.

「다음으로 열리는 시각」은 정규장이 아니라 **가장 이른 창의 시작**이다 — 국내는
프리마켓 08:00, 미국은 프리마켓이 그것이다. 사용자 요구가 정확히 그것이었다:
「국장 기준 오전 8시와 같이 장이 일부 열리는 시점부터는 호출을 해서 업데이트」.

못 구한 값은 제로 시각으로 나간다. 부르는 쪽은 그때 예전처럼 폴링을 이어 간다 —
달력이 없어서 모르는 것을 「닫혀 있다」로 단정하면 장중에 화면이 멈춘다.
*/
func marketSessionEdges(assetType string, cals marketCalendars, now time.Time) (lastClose, nextOpen time.Time) {
	for _, w := range marketSessionWindows(assetType, cals) {
		if !w.end.After(now) && w.end.After(lastClose) {
			lastClose = w.end
		}
		if w.start.After(now) && (nextOpen.IsZero() || w.start.Before(nextOpen)) {
			nextOpen = w.start
		}
	}
	return lastClose, nextOpen
}

func marketClockAt(assetType string, cals marketCalendars, now time.Time) MarketClock {
	clock := MarketClock{Session: detectMarketSession(assetType, cals, now)}
	if !clock.Closed() {
		return clock
	}
	lastClose, nextOpen := marketSessionEdges(assetType, cals, now)
	if !lastClose.IsZero() {
		clock.ClosedAt = lastClose.UTC().Format(time.RFC3339)
	}
	if !nextOpen.IsZero() {
		clock.NextOpenAt = nextOpen.UTC().Format(time.RFC3339)
	}
	return clock
}

// MarketClockFor answers for a whole market ("domestic" / "us" / "etf"), which
// is what a leaderboard needs — its rows all share one calendar.
func (s *QuotesService) MarketClockFor(ctx context.Context, assetType string) MarketClock {
	if s == nil {
		return MarketClock{}
	}
	var cals marketCalendars
	if s.marketStore != nil {
		cals, _ = s.marketStore.EnsureCalendars(ctx)
	}
	return marketClockAt(assetType, cals, time.Now())
}

/*
MarketClockCachedFor — 같은 판정을 **상류 호출 없이** 한다.

# 왜 갈라야 했나 (2026-09-12 실측)

`MarketClockFor`는 `EnsureCalendars`를 부르고, 그 함수는 달력이 낡았다고 보면 상류에서
다시 받는다. 상세 화면 한 번처럼 **드물게 부르는 자리**에는 맞다.

그런데 주기적으로 도는 자리(환율 주기 판정·순위 마감 스킵)가 그것을 매 틱 부르자
휴장일에 분당 일곱 번의 달력 호출이 나갔다 — 그 판정에는 「지금 캐시에 있는 달력」이면
충분한데 「최신 달력」을 요구한 셈이다.

게이트 쪽도 함께 고쳤지만(`calAttemptedAt`), 그것은 **빈도를 줄이는** 수정이고 이쪽은
**아예 부르지 않는** 수정이다. 주기적인 자리에는 뒤쪽이 맞다 — 달력을 따뜻하게 두는
일은 이미 주인이 있다(`marketStore`의 폴러).

달력이 캐시에 아예 없으면 빈 `MarketClock`이 나오고, 그때 호출부는 「모른다」로 다뤄야
한다 — 두 호출부가 모두 그렇게 되어 있다(모르면 예전처럼 받는다).
*/
func (s *QuotesService) MarketClockCachedFor(ctx context.Context, assetType string) MarketClock {
	if s == nil {
		return MarketClock{}
	}
	var cals marketCalendars
	if s.marketStore != nil {
		cals, _ = s.marketStore.GetCalendars(ctx)
	}
	return marketClockAt(assetType, cals, time.Now())
}
