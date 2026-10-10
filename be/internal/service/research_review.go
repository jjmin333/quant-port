package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
투자 리포트 「AI 검토 노트」 (2026-09-27 사용자 결정 C ④).

공개 리포트가 게시되거나 본문이 고쳐지면 다음 회차(30분 간격)에 한 번 만든다. 리포트 하나에
노트 하나 — 조회 때 만들지 않는다(llm_spend_rule). 기능 버킷 `llm_feat_research`(하루 30콜 ·
8만 토큰)가 떨어지면 규칙 노트(점수 방향·목표가 거리·이후 가격)로 채운다.

지키는 선: 판정·채점하지 않는다(리포트의 성적은 기한이 끝나면 따로 매겨진다), 작성자를
줄 세우지 않는다(리포트 방침 2026-08-10), 링크 공개·비공개 리포트는 모델에 보내지 않는다.
*/

const (
	researchReviewWindow   = 30 * 24 * time.Hour
	researchReviewPerRun   = 10
	researchReviewSlotStep = 30 * time.Minute
)

type ResearchReviewService struct {
	repo   *repository.ResearchReviewRepo
	llm    *llm.Pool
	budget *apibudget.Manager
	runs   *repository.BatchJobRunRepo
	now    func() time.Time
	sleep  func(context.Context, time.Duration) bool
}

func NewResearchReviewService(repo *repository.ResearchReviewRepo, pool *llm.Pool, budget *apibudget.Manager) *ResearchReviewService {
	return &ResearchReviewService{repo: repo, llm: pool, budget: budget, now: time.Now, sleep: sleepCtx}
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
	var nLLM, nRule, nFail int
	var tokens int64
	lastCall := time.Time{}
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
		factsJSON, _ := json.Marshal(facts)
		out := RuleResearchReview(facts)
		source, modelName := "rule", ""
		var used int64

		// 리포트 종목의 시장 키로 청구한다(국내 리포트는 국내 키).
		market := "domestic"
		if !isDomesticSymbol(facts.Symbol) {
			market = "us"
		}
		var client *llm.Client
		if s.llm != nil {
			client = s.llm.For(market)
		}
		if client != nil && client.Enabled() {
			if wait := client.Limits().MinInterval - time.Since(lastCall); !lastCall.IsZero() && wait > 0 {
				if !s.sleep(ctx, wait) {
					break
				}
			}
			est := client.EstimateResearchReviewTokens(string(factsJSON))
			if err := s.budget.CheckTokens(ctx, apibudget.ProviderLLMFeatResearch, est); err != nil {
				log.Printf("research review: feature budget — %v", err)
			} else {
				lastCall = time.Now()
				res, err := client.GenerateResearchReview(ctx, string(factsJSON))
				if err != nil {
					log.Printf("research review %d llm: %v", p.CallID, err)
				} else {
					s.budget.Record(ctx, apibudget.ProviderLLMFeatResearch)
					s.budget.RecordTokens(ctx, apibudget.ProviderLLMFeatResearch, res.TotalTokens)
					if len(res.Checks) >= 2 && res.Summary != "" {
						out, source, modelName, used = *res, "llm", res.Model, res.TotalTokens
					}
				}
			}
		}
		body, _ := json.Marshal(map[string]any{"checks": out.Checks, "summary": out.Summary})
		if err := s.repo.Upsert(ctx, p.CallID, facts.BodyRev, body, source, modelName, used); err != nil {
			nFail++
			continue
		}
		tokens += used
		if source == "llm" {
			nLLM++
		} else {
			nRule++
		}
	}
	msg := fmt.Sprintf("리포트 %d건 — AI %d · 규칙 %d · 실패 %d (토큰 %d)", len(pending), nLLM, nRule, nFail, tokens)
	return map[string]any{"pending": len(pending), "llm": nLLM, "rule": nRule, "failed": nFail, "tokens": tokens}, msg, nil
}

func isDomesticSymbol(sym string) bool {
	if len(sym) != 6 {
		return false
	}
	for _, r := range sym[:1] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
