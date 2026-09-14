package service

import (
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
)

/*
「이 잡은 지금쯤 돌았어야 한다」를 계산한다.

# 왜 필요한가

원장(`batch_job_runs`)은 **실행된 것만** 안다. 실행되지 않은 것은 아무 흔적도 남기지
않으므로, 기대 시각을 따로 계산하지 않으면 화면에서 이 둘이 똑같이 보인다:

	해외 AI 요약 마지막 기록 09-04  ← 09-07이 Labor Day라 정상
	해외 AI 요약 마지막 기록 09-04  ← 프로세스가 죽어서 사흘째 안 돎

2026-09-08에 실제로 이 둘을 헷갈렸고, 구분하려고 사람이 `holidays_us.json`을 직접
열어야 했다. 그 판단을 코드로 옮긴 것이 이 파일이다.

# 시각은 어디서 왔나

`batchJobCatalog`의 `Schedule` 문자열과 **같은 사실을 두 번 적는다.** 하나는 사람이
읽고 하나는 기계가 읽는다. 이상적이지 않지만, 실제 트리거는 각 `cmd/worker-*`
안에 흩어져 있어서 어느 쪽도 진짜 원본이 아니다 — 셋이 갈라지지 않게 하는 것은
`docs/BATCH_SCHEDULE.md`와 이 표를 함께 고치는 습관뿐이다(AGENTS.md의 규칙).

값은 문서가 아니라 **원장 실측**으로 맞췄다(2026-09-01~09-08 `trigger_kind='schedule'`).
그래서 「매일 도는가 · 거래일에만 도는가」가 문서의 서술이 아니라 관측이다: 워커 잡은
주말·휴일에도 전부 돌고(빈 결과를 남길 뿐), 거래일 판정이 스케줄러에 들어 있는 것은
AI 요약 둘뿐이다.
*/
type batchExpectation struct {
	/*
		schedule이 있으면 **그 시장의 거래일에만** 기대한다. 빈 값이면 매일이다.

		워커 잡 대부분이 빈 값인 것은 게으름이 아니라 관측 결과다 — 그것들은 휴장일에도
		돌아서 「오늘은 받을 게 없다」를 확인하고 끝난다. 거기에 거래일 조건을 걸면
		실제로 죽은 휴일을 감시가 놓친다.
	*/
	schedule calendar.ScheduleID
	// at은 기대 시각("HH:MM") 목록. schedule이 있으면 그 시장의 현지 시각,
	// 없으면 KST다. 하루에 여러 번 도는 잡은 마지막으로 지난 시각이 기준이 된다.
	at []string
	// everyMinutes는 벽시계 시각이 아니라 주기로 도는 잡(뉴스 브리핑·워치독).
	// at과 함께 쓰지 않는다.
	everyMinutes int
	/*
		grace — 기대 시각 이후 이만큼까지는 「늦었다」고 하지 않는다.

		실행 시간이 긴 잡일수록 넉넉해야 한다. 판정은 **시작이 아니라 성공**을 보므로,
		2시간 걸리는 잡에 1시간 유예를 주면 매일 늦은 것으로 나온다.
	*/
	grace time.Duration
	/*
		maxRuntime — running 상태가 이보다 오래면 stuck으로 본다.

		프로세스가 죽으면 행이 running으로 남는데(`MarkStaleRunsFailed`가 기동 때
		쓸어 담지만 그 기동이 오지 않으면 그대로다), 그 상태가 「지금 열심히 돌고 있음」과
		구분되지 않는다.
	*/
	maxRuntime time.Duration
	// partOf는 다른 잡 **안에서** 도는 잡이다. 독립 판정하지 않는다 — 부모가 돌면
	// 같이 돈 것이고, 부모가 안 돌면 부모 쪽에서 이미 보인다.
	partOf model.BatchJobID
	// manual은 예약이 없는 잡. 「오래 안 돌았다」가 이상 신호가 아니다.
	manual bool
}

// managed reports whether this job can be judged at all.
func (e batchExpectation) managed() bool {
	return !e.manual && e.partOf == "" && (len(e.at) > 0 || e.everyMinutes > 0)
}

func (e batchExpectation) graceOrDefault() time.Duration {
	if e.grace > 0 {
		return e.grace
	}
	return 2 * time.Hour
}

func (e batchExpectation) maxRuntimeOrDefault() time.Duration {
	if e.maxRuntime > 0 {
		return e.maxRuntime
	}
	return 3 * time.Hour
}

/*
batchExpectations — 잡별 기대. 새 잡을 추가하면 **여기도 추가한다.**

빠뜨리면 조용히 감시 밖에 놓인다(`unmanaged`). 그래서 catalog에 있는 잡이 이 표에
없으면 테스트가 실패하도록 해 두었다(batch_expectation_test.go).
*/
var batchExpectations = map[model.BatchJobID]batchExpectation{
	// ── 매일 도는 워커 잡 (휴장일에도 돈다 · 실측) ──────────────────────────
	model.BatchTossUniverse: {
		// 마켓 일곱 × (호출 1 + 간격 2초) — 실측 수 초. grace 는 워커 재기동을 덮는다.
		at: []string{"01:00"}, grace: 3 * time.Hour, maxRuntime: 20 * time.Minute,
	},
	model.BatchWarnings: {
		at: []string{"01:30"}, grace: 5 * time.Hour, maxRuntime: 4 * time.Hour,
	}, // 전 종목 17,900개 · 실측 약 2시간
	model.BatchStockScores: {
		at: []string{"00:25"}, grace: 3 * time.Hour, maxRuntime: 2 * time.Hour,
	},
	model.BatchDataRetention: {
		at: []string{"04:30"}, grace: 3 * time.Hour,
	},
	model.BatchCandleRollup: {
		at: []string{"05:30"}, grace: 3 * time.Hour,
	},
	model.BatchEarningsOverseas: {
		at: []string{"06:00"}, grace: 3 * time.Hour,
	},
	model.BatchInsiderOverseas: {
		at: []string{"07:00"}, grace: 3 * time.Hour,
	},
	model.BatchResearchVerdict: {
		at: []string{"07:00"}, grace: 3 * time.Hour,
	},
	model.BatchFinancialsOverseas: {
		at: []string{"09:00"}, grace: 3 * time.Hour,
	},
	model.BatchEarningsDomestic: {
		// 시즌에는 10:00부터 2시간마다 돌지만, 비시즌 18:30 하나만 기대로 둔다 —
		// 시즌 슬롯을 전부 적으면 비시즌마다 거짓 경보가 난다.
		at: []string{"18:30"}, grace: 6 * time.Hour,
	},
	model.BatchDisclosures: {
		at: []string{"19:30"}, grace: 3 * time.Hour,
	},
	model.BatchInsiderDomestic: {
		at: []string{"20:00"}, grace: 4 * time.Hour, maxRuntime: 2 * time.Hour,
	},
	model.BatchFinancialsDomestic: {
		// 400종목 × 20호출 × 1초 = 실측 약 2.5시간. 유예가 그보다 짧으면 매일 늦다.
		at: []string{"22:00"}, grace: 5 * time.Hour, maxRuntime: 4 * time.Hour,
	},
	model.BatchDailyCloseDomestic: {
		// KRX 마감 +20분. 실측 15:49~16:15에 적재가 끝난다.
		at: []string{"15:50"}, grace: 4 * time.Hour, maxRuntime: 2 * time.Hour,
	},
	model.BatchDailyCloseOverseas: {
		// US 마감 +20분 = 05:20 KST(서머타임 기준). 겨울에는 한 시간 늦으므로
		// 유예로 흡수한다 — 이 잡은 KST 벽시계로 도는 워커에 매달려 있다.
		at: []string{"05:20"}, grace: 5 * time.Hour, maxRuntime: 2 * time.Hour,
	},

	// ── 거래일에만 도는 잡 ────────────────────────────────────────────────
	/*
		AI 요약 두 잡은 **창의 시작**을 기대 시각으로 쓴다 — 기한(18:30 / 20:00 ET)이
		아니다.

		원장이 기록하는 시각은 실행의 **시작**이고, 이 배치는 계획한 종목 수만큼
		기한에서 거꾸로 빼서 시작한다(stock_summary_schedule.go). 상한 350이면 국내가
		17:10에 시작해 18:15에 끝난다. 기한을 기대로 잡으면 「17:10에 시작해 성공한
		실행」이 기대(18:30)보다 이르다는 이유로 매일 늦은 것이 된다 — 실측으로 확인했다.

		창의 시작(16:30 / 18:00 ET)은 종가 적재가 끝난 시각이라 그보다 이른 실행은
		애초에 없다. 유예는 기한 이후까지 덮도록 넉넉히 둔다.
	*/
	model.BatchLLMSummaryDomestic: {
		schedule: calendar.ScheduleKRX,
		at:       []string{"16:30"}, grace: 4 * time.Hour, maxRuntime: 3 * time.Hour,
	},
	model.BatchLLMSummaryOverseas: {
		schedule: calendar.ScheduleUS,
		// ET로 적어 두면 서머타임 전환을 달력이 알아서 따라간다.
		at: []string{"18:00"}, grace: 4 * time.Hour, maxRuntime: 3 * time.Hour,
	},

	// ── 주기로 도는 잡 ────────────────────────────────────────────────────
	model.BatchNewsBriefingDomestic: {
		// 장중 30분·그 외 1시간. 느슨한 쪽(1시간)을 기대로 두고 유예를 넉넉히 —
		// 여기서 보려는 것은 문장의 신선도가 아니라 **스케줄러가 살아 있는가**다.
		everyMinutes: 60, grace: 2 * time.Hour,
	},
	model.BatchNewsBriefingOverseas: {
		everyMinutes: 60, grace: 2 * time.Hour,
	},
	model.BatchWatchdog: {
		everyMinutes: batchWatchdogIntervalMinutes, grace: 45 * time.Minute, maxRuntime: 10 * time.Minute,
	},

	// ── 예약이 없는 잡 ────────────────────────────────────────────────────
	model.BatchIndexCloses:            {manual: true},
	model.BatchDailyCloseFetchStale:   {manual: true},
	model.BatchDailyCloseFetchMissing: {manual: true},
	model.BatchDailyCloseFetchHistory: {manual: true},
	// 지수 해설은 점수 배치 **안에서** 돈다 — 따로 예약이 없다.
	model.BatchIndexNotes: {partOf: model.BatchStockScores},
}

// batchExpectationLookbackDays bounds the walk backwards when every recent day
// is a holiday. 한국·미국 모두 최장 연휴가 이보다 짧다.
const batchExpectationLookbackDays = 12

/*
batchExpectationJitter — 기대 시각을 **이만큼 앞당겨** 비교한다.

이 표의 시각은 사람이 반올림한 값이고 실제 트리거는 그보다 몇 초 이르다. 실측하면
워커들이 `HH:MM:56`에 뜬다 — `daily_close_domestic`이 15:49:57, `financials_domestic`이
21:59:59다(틱 정렬 때문이다). 그대로 비교하면 **성공이 기대보다 1초 이르다는 이유로
매일 「늦음」이 뜬다.** 감시가 매일 거짓말을 하면 아무도 안 본다.

15분인 이유: 몇 초면 충분하지만, 표의 시각이 문서에서 옮겨 적은 근사값이라 분 단위로
어긋날 여지를 남긴다. 이보다 크게 잡으면 진짜 지연을 놓치기 시작한다.
*/
const batchExpectationJitter = 15 * time.Minute

/*
expectedLastRunAt — 지금 기준으로 **가장 최근에 지나간 기대 시각**.

ok=false는 「판정하지 않는다」는 뜻이다: 예약이 없거나(수동·부모 포함), 달력상 최근
며칠이 전부 휴장이라 기대할 시각이 없거나.
*/
func expectedLastRunAt(exp batchExpectation, now time.Time) (time.Time, bool) {
	if !exp.managed() {
		return time.Time{}, false
	}
	if exp.everyMinutes > 0 {
		step := time.Duration(exp.everyMinutes) * time.Minute
		return now.Truncate(step), true
	}

	loc := seoulLocation()
	if exp.schedule != "" {
		loc = calendar.LocationFor(exp.schedule)
	}
	local := now.In(loc)
	for back := 0; back <= batchExpectationLookbackDays; back++ {
		day := local.AddDate(0, 0, -back)
		if exp.schedule != "" && !calendar.IsTradingDay(exp.schedule, day) {
			continue
		}
		// 하루 안에서는 늦은 시각부터 본다 — 오늘 10:00과 18:00이 예정이고 지금이
		// 19시면 기준은 18:00이다.
		best := time.Time{}
		for _, hhmm := range exp.at {
			at, ok := parseClock(day, hhmm, loc)
			if !ok || at.After(local) {
				continue
			}
			if best.IsZero() || at.After(best) {
				best = at
			}
		}
		if !best.IsZero() {
			return best, true
		}
	}
	return time.Time{}, false
}

// parseClock builds "HH:MM" on the given local day.
func parseClock(day time.Time, hhmm string, loc *time.Location) (time.Time, bool) {
	parts := strings.SplitN(strings.TrimSpace(hhmm), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, false
	}
	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return time.Time{}, false
	}
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, loc), true
}

/*
judgeBatchHealth — 원장의 마지막 기록 한 줄과 기대 시각으로 상태를 정한다.

순서가 의미를 만든다:

	성공이 기대 이후   → ok        (늦게 돌았어도 돌았으면 ok다)
	running이 너무 오래 → stuck     (죽은 프로세스가 남긴 행)
	아직 유예 안       → pending   (경보를 내면 매일 낸다)
	기대 이후에 실패    → failed    (돌긴 했는데 깨졌다 — late와 원인이 다르다)
	그 밖              → late      (아무 일도 일어나지 않았다)
*/
func judgeBatchHealth(exp batchExpectation, row batchLedgerRow, now time.Time) (string, *time.Time, time.Duration) {
	expected, ok := expectedLastRunAt(exp, now)
	if !ok {
		return model.BatchHealthUnmanaged, nil, 0
	}
	// 비교는 앞당긴 시각으로, 표시는 원래 시각으로 — 위 batchExpectationJitter 참조.
	matchAt := expected.Add(-batchExpectationJitter)
	if row.LastSuccessAt != nil && !row.LastSuccessAt.Before(matchAt) {
		return model.BatchHealthOK, &expected, 0
	}
	if row.LastStatus == "running" && row.LastRunAt != nil &&
		now.Sub(*row.LastRunAt) > exp.maxRuntimeOrDefault() {
		return model.BatchHealthStuck, &expected, now.Sub(*row.LastRunAt)
	}
	deadline := expected.Add(exp.graceOrDefault())
	if now.Before(deadline) {
		return model.BatchHealthPending, &expected, 0
	}
	if row.LastStatus == "error" && row.LastRunAt != nil && !row.LastRunAt.Before(matchAt) {
		return model.BatchHealthFailed, &expected, now.Sub(deadline)
	}
	return model.BatchHealthLate, &expected, now.Sub(deadline)
}
