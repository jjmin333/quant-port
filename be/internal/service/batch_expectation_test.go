package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func at(t *testing.T, layout string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04", layout, seoulLocation())
	if err != nil {
		t.Fatalf("parse %q: %v", layout, err)
	}
	return parsed
}

func timePtr(t time.Time) *time.Time { return &t }

/*
카탈로그에 있는 잡은 전부 기대가 있어야 한다.

빠뜨리면 그 잡은 조용히 감시 밖으로 나간다 — 그리고 감시 밖이라는 사실 자체가
어디에도 안 보인다. 새 잡을 추가할 때 여기서 걸린다.
*/
func TestEveryCataloguedJobHasAnExpectation(t *testing.T) {
	for _, spec := range batchJobCatalog {
		if _, ok := batchExpectations[spec.ID]; !ok {
			t.Fatalf("%s has no expectation — batchExpectations에 추가해야 감시 대상이 된다", spec.ID)
		}
	}
	for id := range batchExpectations {
		found := false
		for _, spec := range batchJobCatalog {
			if spec.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s는 카탈로그에 없는 잡이다 — 기대만 남아 있으면 판정할 대상이 없다", id)
		}
	}
}

/*
2026-09-07은 Labor Day다.

이 테스트가 이 파일이 존재하는 이유다 — 09-07에 해외 AI 요약이 안 돈 것을 사람이
「사흘째 멈춤」으로 읽었고, 구분하려고 holidays_us.json을 직접 열어야 했다.
*/
func TestOverseasLLMExpectationSkipsUSHolidays(t *testing.T) {
	exp := batchExpectations[model.BatchLLMSummaryOverseas]
	// 2026-09-08(화) 12:00 KST — 미국은 09-07(월)이 Labor Day라 직전 거래일은 09-04(금).
	got, ok := expectedLastRunAt(exp, at(t, "2026-09-08 12:00"))
	if !ok {
		t.Fatal("기대 시각이 없다")
	}
	et := got.In(calendar.LocationFor(calendar.ScheduleUS))
	// 기대는 창의 시작(18:00 ET)이다 — 기한이 아닌 이유는 위 표의 주석 참조.
	if want := "2026-09-04 18:00"; et.Format("2006-01-02 15:04") != want {
		t.Fatalf("기대 시각 = %s ET, want %s (09-07은 Labor Day)", et.Format("2006-01-02 15:04"), want)
	}
}

// 그 상황을 그대로 판정하면 「정상」이어야 한다 — 09-04 성공 뒤로 아무 일이 없었지만
// 그 사이에 기대할 시각이 없었다.
func TestHolidayGapJudgesAsOK(t *testing.T) {
	exp := batchExpectations[model.BatchLLMSummaryOverseas]
	row := repository.BatchLedgerRow{
		LastStatus:    "done",
		LastRunAt:     timePtr(at(t, "2026-09-05 09:20")), // 09-04 세션 = 09-05 09:20 KST
		LastSuccessAt: timePtr(at(t, "2026-09-05 09:20")),
	}
	health, _, _ := judgeBatchHealth(exp, row, at(t, "2026-09-08 12:00"))
	if health != model.BatchHealthOK {
		t.Fatalf("health = %s, want ok — 09-07은 휴장이라 기대 자체가 없다", health)
	}
}

// 같은 공백이라도 거래일을 지나쳤으면 늦은 것이다.
func TestMissedTradingDayJudgesAsLate(t *testing.T) {
	exp := batchExpectations[model.BatchLLMSummaryOverseas]
	row := repository.BatchLedgerRow{
		LastStatus:    "done",
		LastRunAt:     timePtr(at(t, "2026-09-05 09:20")),
		LastSuccessAt: timePtr(at(t, "2026-09-05 09:20")),
	}
	// 09-09(수) 15:00 KST — 09-08(화)은 미국 정상 거래일이고 그 실행은 09-09 09:00 KST에
	// 끝났어야 한다. 유예 3시간도 지났다.
	health, expected, lateBy := judgeBatchHealth(exp, row, at(t, "2026-09-09 15:00"))
	if health != model.BatchHealthLate {
		t.Fatalf("health = %s, want late (기대 %v)", health, expected)
	}
	if lateBy <= 0 {
		t.Fatalf("lateBy = %v, want > 0", lateBy)
	}
}

// 유예 안이면 아직 경보를 내지 않는다 — 안 그러면 잡이 도는 동안 매일 경보가 난다.
func TestWithinGraceIsPending(t *testing.T) {
	exp := batchExpectations[model.BatchFinancialsDomestic] // 22:00 · 유예 5시간
	row := repository.BatchLedgerRow{
		LastStatus:    "done",
		LastRunAt:     timePtr(at(t, "2026-09-07 22:00")),
		LastSuccessAt: timePtr(at(t, "2026-09-07 22:00")),
	}
	health, _, _ := judgeBatchHealth(exp, row, at(t, "2026-09-08 23:30"))
	if health != model.BatchHealthPending {
		t.Fatalf("health = %s, want pending — 22:00 기대에 유예 5시간이다", health)
	}
}

/*
2026-09-08에 실제로 일어난 일 — financials_domestic이 21:59에 시작해 재기동에 죽었다.

「돌긴 했는데 깨졌다」는 「아무 일도 안 일어났다」와 원인이 다르므로 다르게 부른다.
*/
func TestErroredAfterExpectedIsFailed(t *testing.T) {
	exp := batchExpectations[model.BatchFinancialsDomestic]
	row := repository.BatchLedgerRow{
		LastStatus:    "error",
		LastRunAt:     timePtr(at(t, "2026-09-08 21:59")),
		LastSuccessAt: timePtr(at(t, "2026-09-07 22:00")),
	}
	health, _, _ := judgeBatchHealth(exp, row, at(t, "2026-09-09 05:00"))
	if health != model.BatchHealthFailed {
		t.Fatalf("health = %s, want failed", health)
	}
}

// running이 최대 실행시간을 넘기면 stuck — 죽은 프로세스가 남긴 행과 지금 도는 일을
// 구분하는 유일한 방법이다.
func TestLongRunningIsStuck(t *testing.T) {
	exp := batchExpectations[model.BatchStockScores] // maxRuntime 2시간
	row := repository.BatchLedgerRow{
		LastStatus:    "running",
		LastRunAt:     timePtr(at(t, "2026-09-08 00:25")),
		LastSuccessAt: timePtr(at(t, "2026-09-07 00:25")),
	}
	health, _, _ := judgeBatchHealth(exp, row, at(t, "2026-09-08 06:00"))
	if health != model.BatchHealthStuck {
		t.Fatalf("health = %s, want stuck", health)
	}
}

// 늦게라도 성공했으면 정상이다 — 기대는 시각이지 마감이 아니다.
func TestLateButSucceededIsOK(t *testing.T) {
	exp := batchExpectations[model.BatchDisclosures] // 19:30
	row := repository.BatchLedgerRow{
		LastStatus:    "done",
		LastRunAt:     timePtr(at(t, "2026-09-08 21:10")),
		LastSuccessAt: timePtr(at(t, "2026-09-08 21:10")),
	}
	health, _, _ := judgeBatchHealth(exp, row, at(t, "2026-09-08 23:00"))
	if health != model.BatchHealthOK {
		t.Fatalf("health = %s, want ok", health)
	}
}

// 수동 전용·부모 잡 포함은 판정하지 않는다.
func TestUnscheduledJobsAreUnmanaged(t *testing.T) {
	for _, id := range []model.BatchJobID{
		model.BatchIndexCloses, model.BatchDailyCloseFetchStale, model.BatchIndexNotes,
	} {
		health, expected, _ := judgeBatchHealth(batchExpectations[id], repository.BatchLedgerRow{}, time.Now())
		if health != model.BatchHealthUnmanaged {
			t.Fatalf("%s health = %s, want unmanaged", id, health)
		}
		if expected != nil {
			t.Fatalf("%s는 기대 시각이 없어야 한다: %v", id, expected)
		}
	}
}

// 주기형(뉴스 브리핑·워치독)은 벽시계 슬롯으로 기대를 잡는다.
func TestIntervalJobsExpectTheLastSlot(t *testing.T) {
	exp := batchExpectations[model.BatchNewsBriefingDomestic] // 60분
	got, ok := expectedLastRunAt(exp, at(t, "2026-09-08 14:37"))
	if !ok {
		t.Fatal("기대 시각이 없다")
	}
	if want := at(t, "2026-09-08 14:00"); !got.Equal(want) {
		t.Fatalf("기대 = %s, want %s", got.Format("15:04"), want.Format("15:04"))
	}

	// 워치독은 자기 주기로 자기를 판정한다.
	if batchExpectations[model.BatchWatchdog].everyMinutes != batchWatchdogIntervalMinutes {
		t.Fatal("워치독의 기대 주기와 실제 틱 주기가 갈렸다")
	}
}

// 한 번도 안 돈 잡은 late다 — 기록이 없는 것과 정상인 것은 다르다.
func TestNeverRanIsLate(t *testing.T) {
	exp := batchExpectations[model.BatchStockScores]
	health, _, _ := judgeBatchHealth(exp, repository.BatchLedgerRow{}, at(t, "2026-09-08 12:00"))
	if health != model.BatchHealthLate {
		t.Fatalf("health = %s, want late", health)
	}
}

/*
실제 트리거는 표의 시각보다 몇 초 이르다 — 그걸 늦음으로 보면 매일 거짓 경보다.

실측: 워커들이 `HH:MM:56`에 뜬다. daily_close_domestic 15:49:57 · financials_domestic
21:59:59. 이 테스트가 없으면 감시를 켠 다음 날 전 잡이 빨갛게 된다.
*/
func TestSecondsEarlyStillCountsAsOnTime(t *testing.T) {
	exp := batchExpectations[model.BatchDailyCloseDomestic] // 표에는 15:50
	row := repository.BatchLedgerRow{
		LastStatus:    "done",
		LastRunAt:     timePtr(at(t, "2026-09-08 15:49")),
		LastSuccessAt: timePtr(at(t, "2026-09-08 15:49")),
	}
	health, _, _ := judgeBatchHealth(exp, row, at(t, "2026-09-08 23:00"))
	if health != model.BatchHealthOK {
		t.Fatalf("health = %s, want ok — 1분 이른 성공은 그 슬롯의 성공이다", health)
	}
}

/*
AI 요약은 기한보다 한참 앞서 시작한다 — 그 시작 시각이 원장에 남는 값이다.

실측: 국내 배치가 17:54에 시작해 18:15에 끝났다(기한 18:30). 기대를 기한으로 잡으면
「제때 끝난 성공」이 기대보다 이르다는 이유로 매일 늦은 것이 된다. 백필로 실제 기록을
원장에 넣자마자 이 함정이 드러났다.
*/
func TestLLMSummaryJudgedFromWindowStartNotDeadline(t *testing.T) {
	exp := batchExpectations[model.BatchLLMSummaryDomestic]
	row := repository.BatchLedgerRow{
		LastStatus:    "done",
		LastRunAt:     timePtr(at(t, "2026-09-08 17:54")),
		LastSuccessAt: timePtr(at(t, "2026-09-08 17:54")),
	}
	health, expected, _ := judgeBatchHealth(exp, row, at(t, "2026-09-08 23:00"))
	if health != model.BatchHealthOK {
		t.Fatalf("health = %s (기대 %v), want ok — 17:54 시작은 그날 창 안의 실행이다", health, expected)
	}
}

// 해외도 같다 — 18:00 ET 창에서 시작해 20:00 ET 기한 전에 끝난다.
func TestOverseasLLMJudgedFromWindowStart(t *testing.T) {
	exp := batchExpectations[model.BatchLLMSummaryOverseas]
	row := repository.BatchLedgerRow{
		LastStatus: "done",
		// 09-04(금) 세션 실행 = 09-05 08:24 KST.
		LastRunAt:     timePtr(at(t, "2026-09-05 08:24")),
		LastSuccessAt: timePtr(at(t, "2026-09-05 08:24")),
	}
	// 09-08 12:00 KST — 직전 거래일은 09-04(09-07은 Labor Day).
	health, expected, _ := judgeBatchHealth(exp, row, at(t, "2026-09-08 12:00"))
	if health != model.BatchHealthOK {
		t.Fatalf("health = %s (기대 %v), want ok", health, expected)
	}
}
