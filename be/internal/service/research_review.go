package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
투자 리포트 「AI 검토 노트」 (2026-09-27 사용자 결정 C ④).

공개 리포트가 게시되거나 본문이 고쳐지면 다음 회차(30분 간격)에 한 번 만든다. 리포트 하나에
노트 하나 — 조회 때 만들지 않는다.

**LLM을 쓰지 않는다** (2026-10-12 사용자 결정). 노트는 규칙 노트(점수 방향·목표가 거리·이후 가격)뿐이다.
예전에는 기능 버킷 `llm_feat_research`로 모델을 부르고 한도가 떨어질 때만 규칙 노트였다. 그때 저장된
`source = 'llm'` 행은 그대로 남고, 본문을 고치면 규칙 노트로 다시 만들어진다.

지키는 선: 판정·채점하지 않는다(리포트의 성적은 기한이 끝나면 따로 매겨진다), 작성자를
줄 세우지 않는다(리포트 방침 2026-08-10), 링크 공개·비공개 리포트는 모델에 보내지 않는다.
*/

const (
	researchReviewWindow   = 30 * 24 * time.Hour
	researchReviewPerRun   = 10
	researchReviewSlotStep = 30 * time.Minute
)

type ResearchReviewService struct {
	repo  *repository.ResearchReviewRepo
	runs  *repository.BatchJobRunRepo
	now   func() time.Time
	sleep func(context.Context, time.Duration) bool
}

func NewResearchReviewService(repo *repository.ResearchReviewRepo) *ResearchReviewService {
	return &ResearchReviewService{repo: repo, now: time.Now, sleep: sleepCtx}
}

func (s *ResearchReviewService) SetBatchRuns(r *repository.BatchJobRunRepo) { s.runs = r }

// Get returns the stored note (nil when none yet).
func (s *ResearchReviewService) Get(ctx context.Context, callID int64) (*repository.ResearchReviewRow, error) {
	return s.repo.Get(ctx, callID)
}

// RunBatchNow reviews up to researchReviewPerRun pending reports.
func (s *ResearchReviewService) RunBatchNow(ctx context.Context) (map[string]any, string, error) {
	pending, err := s.repo.Pending(ctx, s.now().Add(-researchReviewWindow), researchReviewPerRun)
	if err != nil {
		return nil, "", err
	}
	var nRule, nFail int
	for _, p := range pending {
		if ctx.Err() != nil {
			break
		}
		facts, err := s.repo.Facts(ctx, p.CallID)
		if err != nil {
			log.Printf("research review %d facts: %v", p.CallID, err)
			nFail++
			continue
		}
		out := RuleResearchReview(facts)
		body, _ := json.Marshal(map[string]any{"checks": out.Checks, "summary": out.Summary})
		if err := s.repo.Upsert(ctx, p.CallID, facts.BodyRev, body, "rule", "", 0); err != nil {
			nFail++
			continue
		}
		nRule++
	}
	msg := fmt.Sprintf("리포트 %d건 — 규칙 노트 %d · 실패 %d", len(pending), nRule, nFail)
	return map[string]any{"pending": len(pending), "rule": nRule, "failed": nFail}, msg, nil
}

// RuleResearchReview is the no-LLM note — structured data only.
func RuleResearchReview(f *repository.ResearchReviewFacts) llm.ResearchReviewLLMOutput {
	var out llm.ResearchReviewLLMOutput
	buy := strings.EqualFold(f.Direction, "buy")
	dirWord := "매수"
	if !buy {
		dirWord = "매도"
	}
	if f.ScoreNow != nil {
		v := "근거 약함"
		switch {
		case buy && *f.ScoreNow >= 60, !buy && *f.ScoreNow < 40:
			v = "일치"
		case buy && *f.ScoreNow < 40, !buy && *f.ScoreNow >= 60:
			v = "반대"
		}
		out.Checks = append(out.Checks, llm.ResearchReviewCheck{
			Claim: fmt.Sprintf("%s 의견", dirWord), Data: fmt.Sprintf("지금 AI 점수 %d점", *f.ScoreNow), Verdict: v,
		})
	}
	if f.TargetPrice != nil && f.BasePrice > 0 {
		up := (*f.TargetPrice - f.BasePrice) / f.BasePrice * 100
		v := "확인 불가"
		data := fmt.Sprintf("발행가 대비 목표 %+.1f%%, 기한 %d일", up, f.HorizonDays)
		if f.ReturnSincePub != nil {
			data += fmt.Sprintf(" · 발행 뒤 %+.1f%%", *f.ReturnSincePub)
			switch {
			case (buy && *f.ReturnSincePub > 0) || (!buy && *f.ReturnSincePub < 0):
				v = "일치"
			default:
				v = "반대"
			}
		}
		out.Checks = append(out.Checks, llm.ResearchReviewCheck{Claim: "목표가까지의 움직임", Data: data, Verdict: v})
	}
	if len(f.HeadlinesSince) > 0 {
		out.Checks = append(out.Checks, llm.ResearchReviewCheck{
			Claim: "발행 뒤 새로 나온 소식", Data: f.HeadlinesSince[0], Verdict: "확인 불가",
		})
	}
	out.Summary = "AI 문장 없이 데이터만으로 만든 노트입니다 — 점수 근거와 발행 뒤 기사를 함께 확인해 보세요."
	return out
}

// Run enqueues a review pass every 30 minutes (:05 · :35 KST).
func (s *ResearchReviewService) Run(ctx context.Context) {
	if s.runs == nil {
		return
	}
	gate := NewSlotGate()
	for {
		slot := NextResearchReviewSlot(s.now())
		if !WaitForSlot(ctx, slot) {
			return
		}
		if !gate.Allow(slot) {
			continue
		}
		if busy, err := s.runs.HasPending(ctx, string(model.BatchResearchReview)); err == nil && !busy {
			if _, err := s.runs.EnqueueAs(ctx, string(model.BatchResearchReview), repository.TriggerSchedule, nil); err != nil {
				log.Printf("research review: enqueue: %v", err)
			}
		}
	}
}

// NextResearchReviewSlot is the next :05 or :35 after now.
func NextResearchReviewSlot(now time.Time) time.Time {
	base := now.Truncate(researchReviewSlotStep).Add(5 * time.Minute)
	for !base.After(now) {
		base = base.Add(researchReviewSlotStep)
	}
	return base
}
