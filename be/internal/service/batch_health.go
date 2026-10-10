package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/metrics"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
배치 감시 — 「돌았어야 했는데 안 돌았다」를 판정하고 남긴다.

# 왜 별도 컨테이너가 아닌가

감시자를 새 프로세스로 떼면 **그 감시자를 감시할 것이 또 필요해진다.** 대신 판정은
가장 가벼운 워커(worker-scores)에 잡 하나로 얹고, API는 그 결과의 **나이**를 읽는다.
둘이 서로를 본다:

	워커 워치독 → 잡들이 늦었는지 판정해 batch_health에 남긴다 (API가 죽어도 돈다)
	API        → batch_health.computed_at이 오래되면 워치독의 침묵을 화면에 띄운다

# 판정은 왜 API에서도 다시 하나

`Report`는 저장된 보고서를 읽지 않고 **그 자리에서 다시 계산한다.** 화면이 15분 묵은
숫자를 보여줄 이유가 없고, 워치독이 죽었을 때 화면이 「마지막으로 정상이었던 세상」을
계속 보여주면 그게 가장 나쁜 실패다. 저장된 보고서는 오직 **워치독이 살아 있는지**를
말하는 데 쓴다.
*/

// BatchWatchdogInterval — 워치독 주기. 워커가 이 값으로 틱을 돌고, 감시가 자기
// 자신을 판정할 때도 같은 값을 쓴다(batchExpectations).
const BatchWatchdogInterval = batchWatchdogIntervalMinutes * time.Minute

const (
	// batchWatchdogIntervalMinutes — 워치독 주기. batchExpectations의 자기
	// 항목과 같은 값이어야 한다.
	batchWatchdogIntervalMinutes = 15
	// batchHealthStaleAfter — 보고서가 이보다 낡으면 워치독 자체를 의심한다.
	batchHealthStaleAfter = 45 * time.Minute
	// batchLedgerLookback — 원장을 얼마나 거슬러 읽을까. 이보다 오래 안 돈 잡은
	// 「한 번도 없음」과 같게 다룬다.
	batchLedgerLookback = 30 * 24 * time.Hour
	// batchRunRetention — 원장 보존 기간. 워치독이 자기 회차마다 정리한다.
	batchRunRetention = 90 * 24 * time.Hour
	/*
		incidentRetention — **닫힌** 사건을 남겨 두는 기간.

		원장(batch_job_runs)과 같은 90일로 맞춘다 — 두 표를 나란히 보고 「그때 무엇이
		함께 죽었나」를 재구성하는 것이 이 표의 용도이므로, 한쪽이 먼저 사라지면
		그 재구성이 반쪽이 된다.

		열린 사건은 이 값과 무관하게 남는다.
	*/
	incidentRetention = 90 * 24 * time.Hour
	// batchHealthMetricsInterval — API가 `/metrics` 게이지를 다시 계산하는 주기.
	batchHealthMetricsInterval = 5 * time.Minute
)

// batchLedgerRow is the health judge's view of one job's last row.
type batchLedgerRow = repository.BatchLedgerRow

// BatchHealthService judges every catalogued job against its expectation.
type BatchHealthService struct {
	runs      *repository.BatchJobRunRepo
	health    *repository.BatchHealthRepo
	incidents *repository.OpsIncidentRepo
	// quota는 풀린 한도 소진 사건을 닫는 쪽이다. nil이면 그 정리를 건너뛴다.
	quota *QuotaIncidentReporter
}

func NewBatchHealthService(
	runs *repository.BatchJobRunRepo,
	health *repository.BatchHealthRepo,
) *BatchHealthService {
	return &BatchHealthService{runs: runs, health: health}
}

/*
SetIncidents — 판정 결과를 사건 원장에도 남긴다 (2026-09-11 사용자 요청).

배선이 없으면 예전처럼 로그만 찍는다. 판정 자체는 이 배선과 무관하게 돈다 —
기록이 감시를 막으면 안 된다.
*/
func (s *BatchHealthService) SetIncidents(repo *repository.OpsIncidentRepo) {
	if s != nil {
		s.incidents = repo
	}
}

// SetQuotaReporter lets the watchdog close quota incidents whose window rolled
// over. Optional — without it those rows stay open until someone looks.
func (s *BatchHealthService) SetQuotaReporter(q *QuotaIncidentReporter) {
	if s != nil {
		s.quota = q
	}
}

// incidentKindFor maps a health verdict onto the ledger's vocabulary.
// OK·pending 은 사건이 아니다 — pending 은 「아직 시각이 안 됐다」다.
func incidentKindFor(health string) (model.IncidentKind, string, bool) {
	switch health {
	case model.BatchHealthStuck:
		return model.IncidentBatchStuck, model.IncidentSeverityCritical, true
	case model.BatchHealthFailed:
		return model.IncidentBatchFailed, model.IncidentSeverityCritical, true
	case model.BatchHealthLate:
		return model.IncidentBatchMissed, model.IncidentSeverityWarn, true
	}
	return "", "", false
}

/*
recordIncidents — 이번 회차의 판정을 원장에 반영한다.

**열고 닫는 것을 같은 자리에서 한다.** 여는 것만 하면 고쳐진 사건이 영원히 열린 채로
남고, 그러면 「열린 사건 수」가 아무 의미도 없어진다. 그래서 주의가 필요 없는 잡은
그 잡의 세 종류를 모두 닫는다 — late 로 열렸다가 failed 로 바뀐 사건이 late 쪽에
남는 것을 막는다.

기록 실패는 삼킨다. 이 함수는 감시의 부산물이고, 원장에 못 쓴다고 판정과 재처리가
멈추면 감시가 감시 대상보다 약해진다.
*/
func (s *BatchHealthService) recordIncidents(ctx context.Context, report model.BatchHealthReport) {
	if s == nil {
		return
	}
	if s.incidents == nil {
		// 원장이 배선되지 않았으면 예전 동작 — 주의가 필요한 잡을 매 회차 찍는다.
		// 조용해지는 것보다 시끄러운 편이 낫다.
		for _, job := range report.Jobs {
			if model.BatchHealthNeedsAttention(job.Health) {
				log.Printf("batch watchdog: %s %s — 기대 %s · 마지막 성공 %s",
					job.JobID, job.Health,
					formatBatchTime(job.ExpectedAt), formatBatchTime(job.LastSuccessAt))
			}
		}
		return
	}
	allKinds := []model.IncidentKind{
		model.IncidentBatchMissed, model.IncidentBatchFailed, model.IncidentBatchStuck,
	}
	for _, job := range report.Jobs {
		subject := string(job.JobID)
		kind, severity, isIncident := incidentKindFor(job.Health)
		if !isIncident {
			for _, k := range allKinds {
				if _, err := s.incidents.Resolve(ctx, k, subject); err != nil {
					log.Printf("batch watchdog: resolve incident %s/%s: %v", k, subject, err)
				}
			}
			continue
		}
		detail := map[string]any{"health": job.Health}
		if job.ExpectedAt != nil {
			detail["expectedAt"] = job.ExpectedAt.UTC().Format(time.RFC3339)
		}
		if job.LastSuccessAt != nil {
			detail["lastSuccessAt"] = job.LastSuccessAt.UTC().Format(time.RFC3339)
		}
		msg := fmt.Sprintf("기대 %s · 마지막 성공 %s",
			formatBatchTime(job.ExpectedAt), formatBatchTime(job.LastSuccessAt))
		opened, err := s.incidents.Observe(ctx, kind, subject, severity, msg, detail)
		if err != nil {
			log.Printf("batch watchdog: observe incident %s/%s: %v", kind, subject, err)
		}
		// **새로 열린 사건만 찍는다.** 이어지는 사건은 원장의 occurrences 가 센다.
		if opened {
			log.Printf("batch watchdog: %s %s — %s", subject, job.Health, msg)
		}
		// 종류가 바뀌었으면(late → failed) 이전 종류는 닫는다.
		for _, k := range allKinds {
			if k == kind {
				continue
			}
			if _, err := s.incidents.Resolve(ctx, k, subject); err != nil {
				log.Printf("batch watchdog: resolve incident %s/%s: %v", k, subject, err)
			}
		}
	}
}

/*
Report computes every job's state right now.

`labelFor`가 nil이면 라벨 없이 잡 id만 담는다 — 워커 프로세스에는 카탈로그 라벨이
필요 없고, 화면에 나가는 쪽(API)만 채운다.
*/
func (s *BatchHealthService) Report(ctx context.Context, now time.Time) (model.BatchHealthReport, error) {
	report := model.BatchHealthReport{
		ComputedAt:        now,
		Jobs:              []model.BatchJobHealth{},
		StaleAfterSeconds: int64(batchHealthStaleAfter.Seconds()),
	}
	if s == nil || s.runs == nil {
		return report, nil
	}
	rows, err := s.runs.LedgerRows(ctx, batchLedgerLookback)
	if err != nil {
		return report, err
	}

	for _, spec := range batchJobCatalog {
		exp := batchExpectations[spec.ID]
		row := rows[string(spec.ID)]
		health, expectedAt, lateBy := judgeBatchHealth(exp, row, now)
		entry := model.BatchJobHealth{
			JobID:         spec.ID,
			Label:         spec.Label,
			Health:        health,
			ExpectedAt:    expectedAt,
			LastSuccessAt: row.LastSuccessAt,
			LastRunAt:     row.LastRunAt,
			LastStatus:      row.LastStatus,
			Message:         row.LastMessage,
			LastInterrupted: row.LastInterrupted,
		}
		if lateBy > 0 {
			entry.LateBySeconds = int64(lateBy.Seconds())
		}
		if model.BatchHealthNeedsAttention(health) {
			report.Unhealthy++
		}
		report.Jobs = append(report.Jobs, entry)
	}

	// 나쁜 것부터, 그 안에서는 더 오래 늦은 것부터. 화면이 위에서부터 읽으면 된다.
	sort.SliceStable(report.Jobs, func(i, j int) bool {
		a, b := report.Jobs[i], report.Jobs[j]
		if rank := batchHealthRank(a.Health) - batchHealthRank(b.Health); rank != 0 {
			return rank < 0
		}
		if a.LateBySeconds != b.LateBySeconds {
			return a.LateBySeconds > b.LateBySeconds
		}
		return a.JobID < b.JobID
	})
	return report, nil
}

func batchHealthRank(health string) int {
	switch health {
	case model.BatchHealthStuck:
		return 0
	case model.BatchHealthFailed:
		return 1
	case model.BatchHealthLate:
		return 2
	case model.BatchHealthPending:
		return 3
	case model.BatchHealthOK:
		return 4
	}
	return 5
}

/*
RunWatchdog — 워치독 한 회차.

하는 일은 셋뿐이고 전부 DB 안에서 끝난다: 판정하고, 남기고, 늦은 것을 로그로 외친다.
외부 호출도 LLM도 없다 — 감시가 감시 대상보다 무거우면 안 된다.

정리(오래된 원장 행 삭제)를 여기에 붙인 이유는 주기가 맞아서다. 예약 실행이 전부
원장에 남게 되면서 행이 하루 160개씩 늘어난다.
*/
func (s *BatchHealthService) RunWatchdog(ctx context.Context) (map[string]any, string, error) {
	now := time.Now()
	report, err := s.Report(ctx, now)
	if err != nil {
		return nil, "", err
	}

	attention := make([]string, 0, 4)
	for _, job := range report.Jobs {
		if !model.BatchHealthNeedsAttention(job.Health) {
			continue
		}
		attention = append(attention, string(job.JobID)+"="+job.Health)
		// 로그는 화면을 열지 않은 사람에게 닿는 유일한 경로였다. 이제 사건 원장이
		// 그 자리를 맡으므로 **로그는 사건이 열리거나 바뀔 때만** 찍는다 — 15분마다
		// 같은 줄을 찍으면 그 로그를 아무도 안 읽는다(실측: financials_domestic 한
		// 건이 같은 줄을 18번 넘게 찍었다). 원장이 없을 때는 예전처럼 매번 찍는다.
	}

	/*
		**원장을 먼저 쓰고, 그 결과로 로그를 정한다.**

		순서가 중요하다 — Observe 가 「처음 열렸나」를 알려 주므로, 그것을 알기 전에
		로그를 찍으면 새 사건과 계속 열려 있는 사건을 구분할 수 없다.
	*/
	s.recordIncidents(ctx, report)

	if s.health != nil {
		if err := s.health.Save(ctx, report); err != nil {
			// 저장 실패는 판정 자체를 무효로 만들지 않는다 — 로그에는 이미 나갔다.
			log.Printf("batch watchdog: save report: %v", err)
		}
	}

	// 판정만으로는 아무것도 안 채워진다. 다운으로 놓친 슬롯 중 **채워도 되는 것**을
	// 여기서 큐에 넣는다. 무엇이 되고 왜 안 되는지는 batch_catchup.go 머리글에.
	caughtUp := s.enqueueCatchUps(ctx, report)

	// 창이 넘어가 풀린 한도 소진 사건을 닫는다. 소진 훅은 「비었다」만 말하므로
	// 닫는 일은 여기서 한다(QuotaIncidentReporter.ResolveRecovered 머리글).
	if s.quota != nil {
		s.quota.ResolveRecovered(ctx)
	}

	purged, err := s.runs.DeleteRunsOlderThan(ctx, batchRunRetention)
	if err != nil {
		log.Printf("batch watchdog: purge old runs: %v", err)
	}
	// 닫힌 사건 정리. 열린 것은 아무리 오래됐어도 지우지 않는다 — 오래됐다는
	// 사실이 그 행의 값이다(OpsIncidentRepo.DeleteResolvedOlderThan).
	if s.incidents != nil {
		if n, e := s.incidents.DeleteResolvedOlderThan(ctx, incidentRetention); e != nil {
			log.Printf("batch watchdog: purge incidents: %v", e)
		} else if n > 0 {
			log.Printf("batch watchdog: 사건 원장 %d행 정리", n)
		}
	}

	message := fmt.Sprintf("정상 %d개", len(report.Jobs)-report.Unhealthy)
	if report.Unhealthy > 0 {
		message = fmt.Sprintf("주의 %d개 — %s", report.Unhealthy, strings.Join(attention, ", "))
	}
	if len(caughtUp) > 0 {
		message += fmt.Sprintf(" · 재처리 큐 %s", strings.Join(caughtUp, ", "))
	}
	if purged > 0 {
		message += fmt.Sprintf(" · 원장 %d행 정리", purged)
	}
	return map[string]any{
		"unhealthy": report.Unhealthy,
		"jobs":      len(report.Jobs),
		"attention": attention,
		"catchup":   caughtUp,
		"purged":    purged,
	}, message, nil
}

/*
WatchdogAge — 마지막 보고서가 얼마나 묵었나. ok=false는 보고서가 아예 없다는 뜻이다.

이것이 **워치독의 죽음을 보는 유일한 창**이다. 판정은 API가 자기 손으로 다시 하므로
보고서의 내용은 화면에 쓰지 않는다 — 나이만 쓴다.
*/
func (s *BatchHealthService) WatchdogAge(ctx context.Context, now time.Time) (time.Duration, bool) {
	if s == nil || s.health == nil {
		return 0, false
	}
	at, ok, err := s.health.LastComputedAt(ctx)
	if err != nil || !ok {
		return 0, false
	}
	return now.Sub(at), true
}

/*
RunMetricsLoop — `/metrics`의 배치 게이지를 주기적으로 갱신한다.

관리자 화면을 열 때만 갱신하면, 아무도 안 열었을 때 게이지가 옛날 값에 얼어붙는다 —
그리고 그 순간이 정확히 감시가 필요한 때다. API 프로세스에서 도는 이유는 `/metrics`가
여기 있기 때문이고, 판정 자체는 워커의 워치독이 독립적으로 계속한다.

한 회차는 질의 두 방이라 5분 주기가 부담이 되지 않는다.
*/
func (s *BatchHealthService) RunMetricsLoop(ctx context.Context) {
	if s == nil || s.runs == nil {
		return
	}
	tick := func() {
		now := time.Now()
		report, err := s.Report(ctx, now)
		if err != nil {
			log.Printf("batch health metrics: %v", err)
			return
		}
		age := int64(-1)
		if d, ok := s.WatchdogAge(ctx, now); ok {
			age = int64(d.Seconds())
		}
		metrics.SetBatchHealth(report.Unhealthy, age)
	}
	tick()

	ticker := time.NewTicker(batchHealthMetricsInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}

func formatBatchTime(t *time.Time) string {
	if t == nil {
		return "없음"
	}
	return t.In(seoulLocation()).Format("01-02 15:04")
}
