package service

import (
	"context"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

// TestBatchJobCatalogIncludesLLMSummaryJobs guards the whole point of wiring
// the AI 요약 batch into 배치 관리: the two per-market jobs must actually be
// registered, and marked rate_limited (they spend LLM quota) rather than
// exposing the since/limit controls that only make sense for the DB-backfill
// jobs — the batch always targets today's radar top list, nothing else.
func TestBatchJobCatalogIncludesLLMSummaryJobs(t *testing.T) {
	for _, id := range []model.BatchJobID{model.BatchLLMSummaryDomestic, model.BatchLLMSummaryOverseas} {
		spec, ok := batchJobSpec(id)
		if !ok {
			t.Fatalf("catalog missing %s", id)
		}
		if spec.Cost != model.CostRateLimited {
			t.Fatalf("%s cost = %s, want %s (it calls the LLM)", id, spec.Cost, model.CostRateLimited)
		}
		if spec.AcceptsSince || spec.AcceptsLimit {
			t.Fatalf("%s exposes since/limit controls it does not use", id)
		}
	}
}

func TestLLMBatchCoverageNilServiceReturnsNoCoverage(t *testing.T) {
	svc := NewBatchAdminService(nil, nil, nil, nil)
	if got := svc.llmBatchCoverage(context.Background(), toss.MarketKR); got != nil {
		t.Fatalf("expected nil coverage when the LLM summary service is unset, got %v", got)
	}
}
