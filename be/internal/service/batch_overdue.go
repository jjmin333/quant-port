package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
제때 돌지 않은 배치를 관리자가 한 번에 돌린다 — 다만 **한 번에 돌려도 되는 것만.**

워치독 catch-up(`batch_catchup.go`)은 사람이 없을 때 안전한 것만 자동으로 채운다.
이쪽은 사람이 누르므로 돈이 드는 잡(LLM)·실패한 잡도 받지만, 두 가지는 여전히
묶어 돌리면 안 된다.

  - **제외** — 일괄로 돌리면 해롭거나 의미가 없는 잡. 슬롯이 지나면 다음 슬롯이
    덮는 뉴스 브리핑, 감시자 자신, 지금 붙잡혀 있는(stuck) 잡.
  - **순서** — 선행 잡이 아직 늦었거나 돌고 있으면 그 뒤 잡은 **이번에는 넣지
    않는다.** 서버가 기다려 주지 않고, 선행이 끝난 뒤 다시 누르면 다음 단이 풀린다.
    (종가가 앉기 전에 1분봉을 접으면 종가 재료가 사라지고, 점수·AI 해설이 낡은
    입력으로 계산된다.)

같은 외부 소스를 쓰는 잡끼리는 워커가 이미 서로 기다리므로 여기서 직렬화하지 않는다.
*/

// batchOverdueExcluded — 일괄 실행에서 빼는 잡과 그 이유.
var batchOverdueExcluded = map[model.BatchJobID]string{
	model.BatchNewsBriefingDomestic: "슬롯이 지나면 다음 슬롯이 덮습니다 — 지금 돌려도 돈만 듭니다",
	model.BatchNewsBriefingOverseas: "슬롯이 지나면 다음 슬롯이 덮습니다 — 지금 돌려도 돈만 듭니다",
	model.BatchWatchdog:             "감시자 자신입니다",
}

// batchRunAfter — 잡 → 먼저 끝나 있어야 하는 잡. 선행이 늦었거나 돌고 있으면 뒤 잡은 보류한다.
var batchRunAfter = map[model.BatchJobID][]model.BatchJobID{
	// 종가가 앉은 뒤에 접고(1분봉 삭제), 판정하고, 점수를 매긴다.
	model.BatchCandleRollup:            {model.BatchDailyCloseDomestic, model.BatchDailyCloseOverseas},
	model.BatchResearchVerdict:         {model.BatchDailyCloseDomestic, model.BatchDailyCloseOverseas},
	model.BatchStockScores:             {model.BatchDailyCloseDomestic, model.BatchDailyCloseOverseas, model.BatchNewsSentiment},
	model.BatchChangeDetect:            {model.BatchStockScores},
	model.BatchScoreReportCard:         {model.BatchStockScores},
	model.BatchIndexNotes:              {model.BatchStockScores},
	model.BatchEarningsPreviewDomestic: {model.BatchEarningsDomestic},
	model.BatchEarningsPreviewOverseas: {model.BatchEarningsOverseas},
	// 자산: 적재 → 점수 → AI 해설.
	model.BatchAssetScores: {model.BatchReTrades, model.BatchReIndex, model.BatchEcosRates, model.BatchFinProducts, model.BatchOnbid},
	model.BatchAssetAI:     {model.BatchAssetScores},
	// 커뮤니티 요약은 글·언급이 먼저.
	model.BatchSocialSummary: {model.BatchSocialPosts, model.BatchSocialMentions},
}

// BatchOverdueItem — 계획의 한 줄. Reason은 실행 대상에는 비고, 제외·보류에만 있다.
type BatchOverdueItem struct {
	JobID  model.BatchJobID `json:"jobId"`
	Label  string           `json:"label"`
	Reason string           `json:"reason,omitempty"`
	// LLM — 실행하면 LLM 지출이 난다(확인 창에서 따로 알린다).
	LLM bool `json:"llm,omitempty"`
}

// BatchOverduePlan — 이번 일괄 실행이 넣을 것(Run)과 넣지 않을 것(Skip).
type BatchOverduePlan struct {
	Run  []BatchOverdueItem `json:"run"`
	Skip []BatchOverdueItem `json:"skip"`
}

// planOverdue — 보고서에서 계획을 세운다. DB를 보지 않는 순수 함수(pending만 주입).
func planOverdue(report model.BatchHealthReport, pending func(model.BatchJobID) bool) BatchOverduePlan {
	plan := BatchOverduePlan{Run: []BatchOverdueItem{}, Skip: []BatchOverdueItem{}}
	late := map[model.BatchJobID]bool{}
	for _, j := range report.Jobs {
		if model.BatchHealthNeedsAttention(j.Health) {
			late[j.JobID] = true
		}
	}
	for _, j := range report.Jobs {
		if !late[j.JobID] {
			continue
		}
		item := BatchOverdueItem{JobID: j.JobID, Label: j.Label}
		if spec, ok := batchJobSpec(j.JobID); ok {
			item.LLM = spec.Category == model.CategoryLLM
		}
		switch reason := overdueSkipReason(j, late, pending); {
		case reason != "":
			item.Reason = reason
			plan.Skip = append(plan.Skip, item)
		default:
			plan.Run = append(plan.Run, item)
		}
	}
	for _, list := range [][]BatchOverdueItem{plan.Run, plan.Skip} {
		sort.Slice(list, func(a, b int) bool { return list[a].JobID < list[b].JobID })
	}
	return plan
}

func overdueSkipReason(j model.BatchJobHealth, late map[model.BatchJobID]bool, pending func(model.BatchJobID) bool) string {
	if why, ok := batchOverdueExcluded[j.JobID]; ok {
		return why
	}
	if j.Health == model.BatchHealthStuck {
		return "실행이 붙잡혀 있습니다 — 원인을 확인한 뒤 개별로 다루세요"
	}
	if pending(j.JobID) {
		return "이미 대기 중이거나 실행 중입니다"
	}
	for _, pre := range batchRunAfter[j.JobID] {
		if late[pre] || pending(pre) {
			label := string(pre)
			if spec, ok := batchJobSpec(pre); ok {
				label = spec.Label
			}
			return fmt.Sprintf("선행 「%s」가 끝난 뒤에 돌려야 합니다 — 끝나면 다시 누르세요", label)
		}
	}
	return ""
}

// RunOverdue — 제때 돌지 않은 배치를 계획대로 큐에 넣는다. dryRun이면 계획만 돌려준다.
func (s *BatchAdminService) RunOverdue(ctx context.Context, dryRun bool) (BatchOverduePlan, error) {
	if s == nil || s.runs == nil || s.health == nil {
		return BatchOverduePlan{}, fmt.Errorf("batch service unavailable")
	}
	report, err := s.health.Report(ctx, time.Now())
	if err != nil {
		return BatchOverduePlan{}, err
	}
	// 조회 실패는 「바쁨」으로 본다 — 모르는데 겹쳐 넣는 쪽이 더 나쁘다.
	pending := func(id model.BatchJobID) bool {
		busy, err := s.runs.HasPending(ctx, string(id))
		return err != nil || busy
	}
	plan := planOverdue(report, pending)
	if dryRun {
		return plan, nil
	}
	for _, item := range plan.Run {
		if _, err := s.runs.EnqueueAs(ctx, string(item.JobID), repository.TriggerManual, nil); err != nil {
			log.Printf("batch run-overdue: %s 큐 등록 실패: %v", item.JobID, err)
			return plan, err
		}
	}
	return plan, nil
}
