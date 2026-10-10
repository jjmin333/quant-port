package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

/*
재처리 허용 목록에 든 잡은 **카탈로그에 있고 감시 대상이어야** 한다.

허용 목록만 남고 잡이 사라지면 영영 안 도는 이름을 큐에 넣으려 들고, 감시 대상이
아닌 잡(`manual`·`partOf`)은 애초에 `late` 판정을 못 받아 허용해 봐야 죽은 줄이다.
*/
func TestCatchUpAllowlistIsCataloguedAndManaged(t *testing.T) {
	for id := range batchCatchUpAllowed {
		found := false
		for _, spec := range batchJobCatalog {
			if spec.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s는 카탈로그에 없다 — 허용 목록에 죽은 이름이 남아 있다", id)
		}

		exp, ok := batchExpectations[id]
		if !ok {
			t.Errorf("%s에 기대가 없다 — late 판정을 못 받으므로 재처리도 안 된다", id)
			continue
		}
		if !exp.managed() {
			t.Errorf("%s는 감시 대상이 아니다(manual/partOf) — 재처리 허용이 의미 없다", id)
		}
	}
}

// 허용 목록의 사유는 비어 있으면 안 된다. 「왜 이 잡은 자동으로 다시 돌려도 되는가」가
// 남아 있지 않으면 다음 사람이 판단을 재현할 수 없다.
func TestCatchUpAllowlistHasReasons(t *testing.T) {
	for id, reason := range batchCatchUpAllowed {
		if len(reason) < 10 {
			t.Errorf("%s의 재처리 사유가 비어 있거나 너무 짧다: %q", id, reason)
		}
	}
}

/*
**자동으로 다시 돌리면 안 되는 잡**은 허용 목록에 들어오면 안 된다.

이 테스트가 이 파일의 핵심이다. 위의 표는 사람이 손으로 고치는 것이고, 언젠가
「늦었으니 이것도 넣자」가 들어온다. 그때 걸려야 하는 자리다. 각 이유는
batch_catchup.go 머리글에.
*/
func TestCatchUpNeverIncludesUnsafeJobs(t *testing.T) {
	forbidden := map[model.BatchJobID]string{
		model.BatchLLMSummaryDomestic:   "LLM 비용 — 창 밖에서 돌면 다음 날 예산을 먹는다. 사람이 정한다",
		model.BatchLLMSummaryOverseas:   "위와 같다",
		model.BatchNewsBriefingDomestic: "슬롯이 지나면 의미가 없다 — 다음 슬롯이 덮는데 LLM만 태운다",
		model.BatchNewsBriefingOverseas: "위와 같다",
		model.BatchDailyCloseDomestic:   "자기 catch-up(catchUpMissedSessions)이 이미 있다 — 두 번 한다",
		model.BatchDailyCloseOverseas:   "위와 같다",
		model.BatchWatchdog:             "자기 자신을 큐에 넣는 셈이다",
	}
	for id, why := range forbidden {
		if _, ok := batchCatchUpAllowed[id]; ok {
			t.Errorf("%s가 재처리 허용 목록에 있다 — 들어가면 안 된다: %s", id, why)
		}
	}
}

/*
late인 것만, 허용된 것만, 한 회차 상한까지만 고른다.

`failed`·`stuck`을 빼는 것이 특히 중요하다 — 원인을 모른 채 다시 넣으면 같은 실패를
반복하거나 아직 붙잡혀 있는 실행과 겹친다.
*/
func TestCatchUpSelection(t *testing.T) {
	report := model.BatchHealthReport{Jobs: []model.BatchJobHealth{
		{JobID: model.BatchWarnings, Health: model.BatchHealthLate},           // 허용 · late
		{JobID: model.BatchStockScores, Health: model.BatchHealthLate},        // 허용 · late
		{JobID: model.BatchDataRetention, Health: model.BatchHealthLate},      // 허용 · late (상한에 걸림)
		{JobID: model.BatchLLMSummaryDomestic, Health: model.BatchHealthLate}, // 금지
		{JobID: model.BatchDisclosures, Health: model.BatchHealthFailed},      // 허용이지만 failed
		{JobID: model.BatchInsiderDomestic, Health: model.BatchHealthStuck},   // 허용이지만 stuck
		{JobID: model.BatchCandleRollup, Health: model.BatchHealthOK},         // 정상
	}}

	picked := selectCatchUpCandidates(report)

	if len(picked) != batchCatchUpPerTick {
		t.Fatalf("고른 개수 %d — 상한 %d 여야 한다: %v", len(picked), batchCatchUpPerTick, picked)
	}
	// 이름 순이므로 data_retention · stock_scores 가 앞선다(warnings_sync 는 뒤).
	want := []model.BatchJobID{model.BatchDataRetention, model.BatchStockScores}
	for i := range want {
		if picked[i] != want[i] {
			t.Fatalf("고른 잡 %v — 이름 순 상한까지여야 한다 (기대 %v)", picked, want)
		}
	}
	for _, id := range picked {
		if id == model.BatchLLMSummaryDomestic {
			t.Fatalf("금지된 LLM 잡이 뽑혔다")
		}
	}
}

// late 가 하나도 없으면 아무것도 고르지 않는다 — 평소에 큐가 더러워지지 않아야 한다.
func TestCatchUpSelectsNothingWhenHealthy(t *testing.T) {
	report := model.BatchHealthReport{Jobs: []model.BatchJobHealth{
		{JobID: model.BatchWarnings, Health: model.BatchHealthOK},
		{JobID: model.BatchStockScores, Health: model.BatchHealthPending},
		{JobID: model.BatchCandleRollup, Health: model.BatchHealthUnmanaged},
	}}
	if picked := selectCatchUpCandidates(report); len(picked) != 0 {
		t.Fatalf("정상인데 %v 를 골랐다", picked)
	}
}

/*
**잘린 실행은 다시 돌린다** (2026-09-20).

배포·재기동으로 도중에 죽은 실행은 `MarkStaleRunsFailed`가 error 로 닫고, 그러면
판정이 `failed`가 되어 예전에는 재처리에서 빠졌다. 실측 한 달에 69건 · 8개 잡 ·
20일이 그렇게 조용히 버려졌다 — 그 하루치 데이터는 아무도 다시 받지 않는다.

「돌았는데 깨졌다」와 「돌다가 프로세스가 사라졌다」는 다음 행동이 정반대다.
*/
func TestCatchUpIncludesInterruptedFailures(t *testing.T) {
	report := model.BatchHealthReport{Jobs: []model.BatchJobHealth{
		{JobID: model.BatchWarnings, Health: model.BatchHealthFailed, LastInterrupted: true},
		{JobID: model.BatchStockScores, Health: model.BatchHealthOK},
	}}

	picked := selectCatchUpCandidates(report)
	if len(picked) != 1 || picked[0] != model.BatchWarnings {
		t.Fatalf("잘린 실행을 다시 안 골랐다: %v", picked)
	}
}

// 진짜 실패는 여전히 사람 몫이다 — 표식이 없으면 안 고른다.
func TestCatchUpSkipsGenuineFailures(t *testing.T) {
	report := model.BatchHealthReport{Jobs: []model.BatchJobHealth{
		{JobID: model.BatchWarnings, Health: model.BatchHealthFailed},
		{JobID: model.BatchDisclosures, Health: model.BatchHealthFailed, LastInterrupted: false},
	}}
	if picked := selectCatchUpCandidates(report); len(picked) != 0 {
		t.Fatalf("원인 모를 실패를 다시 골랐다: %v", picked)
	}
}

/*
`stuck`은 표식이 있어도 안 고른다 — 「지금 붙잡혀 있다」는 아직 도는 중일 수 있고,
겹쳐 넣으면 같은 잡이 둘이 된다. 잘렸다는 표식은 **지난** 실행의 성질이다.
*/
func TestCatchUpNeverTakesStuckEvenWhenInterrupted(t *testing.T) {
	report := model.BatchHealthReport{Jobs: []model.BatchJobHealth{
		{JobID: model.BatchWarnings, Health: model.BatchHealthStuck, LastInterrupted: true},
	}}
	if picked := selectCatchUpCandidates(report); len(picked) != 0 {
		t.Fatalf("stuck 을 골랐다: %v", picked)
	}
}

// 허용 목록은 그대로다 — 잘렸다고 해서 금지된 잡이 돌면 안 된다(LLM 비용).
func TestCatchUpInterruptedStillRespectsAllowlist(t *testing.T) {
	report := model.BatchHealthReport{Jobs: []model.BatchJobHealth{
		{JobID: model.BatchLLMSummaryDomestic, Health: model.BatchHealthFailed, LastInterrupted: true},
		{JobID: model.BatchNewsBriefingOverseas, Health: model.BatchHealthFailed, LastInterrupted: true},
	}}
	if picked := selectCatchUpCandidates(report); len(picked) != 0 {
		t.Fatalf("금지된 잡을 잘렸다는 이유로 골랐다: %v", picked)
	}
}
