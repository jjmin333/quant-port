package service

import (
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/toss"
)

/*
When each market's AI 요약 batch runs.

**마감 직후에 돈다.** 예전에는 다음 개장에서 거꾸로 세어 국내 08:00 · 미국 08:30 ET에
돌렸다. 근거는 "종을 치기 전에 열어 본 사람이 직전 세션 분석을 보게 한다"였는데, 그
배치가 읽는 값은 마지막으로 끝난 세션의 종가·지표·재무제표뿐이라(`buildStockSummaryFacts`)
**마감 직후에 만들든 다음 날 아침에 만들든 내용이 같다.** 같은 값을 열여섯 시간 늦게
만들고 있었던 셈이다.

그동안 손해가 실재했다. 국내 종가는 15:50에 들어오는데 분석은 다음 날 08:00에야 갱신돼,
**장 마감 후 저녁 내내 어제 종가 분석이 떠 있었다.** 마감 직후로 옮기면 그 구간이 사라지고,
"오늘 장이 어땠나"를 그날 저녁에 볼 수 있다. 호출 수는 그대로다.

시각은 **종가 적재가 끝난 뒤**로 잡는다. 그 전에 돌면 어제 값으로 오늘 분석을 만든다.

	국내  마감 15:30 → daily_close 15:50~16:15 (실측)  → 마감 시각 18:30 KST
	미국  마감 16:00 ET → 05:20~06:37 KST = 16:20~17:37 ET (실측) → 20:00 ET

각 시장은 *마감 기한*을 갖고, 시작 시각은 계획한 일감에서 거꾸로 뺀다. `earliestHour`는
그 뺄셈이 적재 완료보다 앞서지 못하게 막는 바닥이다 — 없으면 DailyCap이 커질 때
`batchMaxWindow`(2시간)만큼 당겨져 종가가 들어오기 전에 시작한다.
*/

type batchSchedule struct {
	market   toss.MarketCountry
	calendar calendar.ScheduleID
	// deadline is a wall-clock time in the market's own timezone: KST for
	// KRX, ET for the US session. Expressing it locally is what makes the
	// US window track daylight saving automatically.
	deadlineHour   int
	deadlineMinute int
	/*
		earliestHour/Minute — 이 시각 전에는 시작하지 않는다(시장 현지 시각).

		그날 종가가 표에 들어온 뒤여야 한다. 기한에서 거꾸로 뺀 시작 시각이 이보다
		이르면 여기로 끌어올린다.
	*/
	earliestHour   int
	earliestMinute int
}

var batchSchedules = []batchSchedule{
	/*
		국내: 15:30 마감, 종가 적재가 16:15까지 (실측). 16:30부터 18:30 사이에 돈다.
	*/
	{
		market: toss.MarketKR, calendar: calendar.ScheduleKRX,
		deadlineHour: 18, deadlineMinute: 30,
		earliestHour: 16, earliestMinute: 30,
	},
	/*
		미국: 16:00 ET 마감, 적재가 17:37 ET까지 (실측 05:20~06:37 KST). 18:00~20:00 ET면
		서머타임 여부와 무관하게 그 뒤다 — 시각을 ET로 적어 두면 전환을 알아서 따라간다.
	*/
	{
		market: toss.MarketUS, calendar: calendar.ScheduleUS,
		deadlineHour: 20, deadlineMinute: 0,
		earliestHour: 18, earliestMinute: 0,
	},
}

const (
	// batchStartMargin is slack between the planned finish and the deadline,
	// absorbing slow LLM responses and per-symbol data assembly.
	batchStartMargin = 10 * time.Minute

	/*
		llmBatchPacePerSymbol — 한 종목에 **실제로** 걸리는 벽시계 시간.

		시작 시각을 `llmRefreshSpacing`(6초)만으로 거꾸로 세면 절반을 놓친다. 간격은
		호출 사이에만 쉬는 시간이고, 그 앞에 요약 빌드(재무·실적·내부자·뉴스·캔들
		수집)와 LLM 응답 대기가 붙기 때문이다. 실측:

			국내 2026-09-08  106종목  20분15초 → 11.5초/종목
			해외 2026-09-04  135종목  22분19초 →  9.9초/종목

		상한이 250이던 동안에도 실제 계획이 106~135라 창이 남아 티가 안 났다. 350으로
		올리면 6초 가정은 45분짜리 창을 잡고 한 시간짜리 일을 넣는 셈이 되어, 매일
		기한에서 잘린 꼬리가 이월된다. 느린 쪽에 맞춰 12초로 센다 — 일찍 끝나면
		그냥 일찍 끝나는 것이고, 시작이 `earliestOn`(종가 적재 완료)보다 앞으로
		당겨지지는 않는다.
	*/
	llmBatchPacePerSymbol = 12 * time.Second
	// batchMaxWindow caps how early a run may begin, so a large DailyCap
	// cannot push the KR run back into the middle of the night where a
	// failure would go unnoticed until the open.
	batchMaxWindow = 2 * time.Hour
)

// batchWindow is how long before its deadline a run should start, given how
// many symbols it plans to process.
func batchWindow(planned int) time.Duration {
	if planned < 1 {
		planned = 1
	}
	window := time.Duration(planned)*llmBatchPacePerSymbol + batchStartMargin
	if window > batchMaxWindow {
		return batchMaxWindow
	}
	return window
}

// deadlineOn returns the schedule's cutoff for the market-local day
// containing `day`.
func (b batchSchedule) deadlineOn(day time.Time) time.Time {
	loc := calendar.LocationFor(b.calendar)
	local := day.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), b.deadlineHour, b.deadlineMinute, 0, 0, loc)
}

// earliestOn is the floor a run may start at on that market-local day — after
// the session's closes have landed. See the schedule table above.
func (b batchSchedule) earliestOn(day time.Time) time.Time {
	loc := calendar.LocationFor(b.calendar)
	local := day.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), b.earliestHour, b.earliestMinute, 0, 0, loc)
}

// tradeDateOn is the market-local calendar date a run belongs to — the key
// under which its record is stored, and the session it is preparing for.
func (b batchSchedule) tradeDateOn(day time.Time) string {
	return day.In(calendar.LocationFor(b.calendar)).Format("2006-01-02")
}

// isTradingDay reports whether the market trades on the day containing `at`.
// Holidays and weekends are read from the embedded exchange calendars, so a
// closed session simply produces no run — nothing is spent on a day whose
// prices will not have moved.
func (b batchSchedule) isTradingDay(at time.Time) bool {
	return calendar.IsTradingDay(b.calendar, at)
}

// batchRun is one scheduled occurrence: when to start, when to be done, and
// which market it covers.
type batchRunWindow struct {
	schedule batchSchedule
	startAt  time.Time
	deadline time.Time
	/*
		ledger — 이 실행을 `batch_job_runs`에 남길까.

		예약 실행만 true다. 수동 실행(`RunBatchNow`)은 이미 큐 워커가 감싸고 있어서
		여기서 또 남기면 한 번의 실행이 두 줄이 된다.
	*/
	ledger bool
	// asOf pins the run to a specific calendar day instead of deriving it
	// from wall-clock time at execution. Zero means "use time.Now()", which
	// is what the scheduled path wants (see runMarketBatch's midnight-crossed
	// safety check). A manual trigger fired on a closed session sets this to
	// the last completed close so the run still has a trading day to attach
	// its results to.
	asOf time.Time
}

// nextBatchWindow returns the earliest window to act on, scanning today and
// the following days until a trading day is found for some market. A window
// whose start has already passed but whose deadline has not is returned with
// startAt == now: that is the restart-catch-up case, where the process came
// up mid-window and the day's run has not happened yet.
//
// alreadyRan reports whether a market's run for a given trade date is already
// recorded, so a restart cannot spend the quota twice.
func nextBatchWindow(
	now time.Time,
	planned int,
	alreadyRan func(market toss.MarketCountry, tradeDate string) bool,
) (batchRunWindow, bool) {
	window := batchWindow(planned)

	var best batchRunWindow
	found := false
	// Seven days is enough to clear the longest exchange holiday run
	// (Korea's 추석/설 stretches) without looping unbounded.
	for dayOffset := 0; dayOffset <= 7 && !found; dayOffset++ {
		day := now.AddDate(0, 0, dayOffset)
		for _, sched := range batchSchedules {
			if !sched.isTradingDay(day) {
				continue
			}
			deadline := sched.deadlineOn(day)
			if !deadline.After(now) {
				continue
			}
			if alreadyRan != nil && alreadyRan(sched.market, sched.tradeDateOn(day)) {
				continue
			}
			startAt := deadline.Add(-window)
			// 종가가 들어오기 전으로는 당기지 않는다 — 그 전에 돌면 어제 값으로
			// 오늘 분석을 만든다.
			if earliest := sched.earliestOn(day); startAt.Before(earliest) {
				startAt = earliest
			}
			if startAt.Before(now) {
				startAt = now
			}
			candidate := batchRunWindow{schedule: sched, startAt: startAt, deadline: deadline, ledger: true}
			if !found || candidate.startAt.Before(best.startAt) {
				best = candidate
				found = true
			}
		}
	}
	return best, found
}
