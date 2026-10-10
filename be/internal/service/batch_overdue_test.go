package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestPlanOverdue(t *testing.T) {
	h := func(id model.BatchJobID, health string) model.BatchJobHealth {
		return model.BatchJobHealth{JobID: id, Label: string(id), Health: health}
	}
	report := model.BatchHealthReport{Jobs: []model.BatchJobHealth{
		h(model.BatchReTrades, model.BatchHealthFailed),           // 실행
		h(model.BatchAssetScores, model.BatchHealthLate),          // 선행 re_trades 늦음 → 보류
		h(model.BatchAssetAI, model.BatchHealthLate),              // 선행 asset_scores 늦음 → 보류
		h(model.BatchNewsBriefingDomestic, model.BatchHealthLate), // 제외
		h(model.BatchWarnings, model.BatchHealthStuck),            // 제외(stuck)
		h(model.BatchOnbid, model.BatchHealthLate),                // 이미 대기 → 제외
		h(model.BatchStockScores, model.BatchHealthOK),            // 정상 → 계획에 없음
	}}
	plan := planOverdue(report, func(id model.BatchJobID) bool { return id == model.BatchOnbid })

	if len(plan.Run) != 1 || plan.Run[0].JobID != model.BatchReTrades {
		t.Fatalf("run = %+v, want only re_trades", plan.Run)
	}
	skipped := map[model.BatchJobID]bool{}
	for _, s := range plan.Skip {
		skipped[s.JobID] = true
		if s.Reason == "" {
			t.Errorf("%s skipped without reason", s.JobID)
		}
	}
	for _, id := range []model.BatchJobID{model.BatchAssetScores, model.BatchAssetAI, model.BatchNewsBriefingDomestic, model.BatchWarnings, model.BatchOnbid} {
		if !skipped[id] {
			t.Errorf("%s should be skipped", id)
		}
	}
	if len(plan.Run)+len(plan.Skip) != 6 {
		t.Errorf("healthy job leaked into plan: %+v", plan)
	}
}
