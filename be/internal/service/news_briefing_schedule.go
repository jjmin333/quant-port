package service

import (
	"context"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/safego"
)

// 브리핑 생성 주기.
//
// 장중(+개장 직전 한 시간)에는 30분마다, 그 밖(마감 후·주말·휴장)에는 1시간마다 —
// 국내는 06:00, 07:00, 08:00, 08:30, 09:00, 09:30 … 처럼 걸린다. 슬롯을
// 벽시계에 맞추는 이유는 두 가지다: 「15:30 생성」 같은 표시가 예측 가능해지고,
// 재기동이 잦아도 생성 횟수가 늘지 않는다(같은 슬롯은 한 번만).
//
// 비용은 슬롯 수가 아니라 **새 기사 유무**로 정해진다. Generate가 직전 실행
// 이후 새 기사가 없으면 LLM을 부르지 않고 직전 브리핑을 그대로 둔다 — 기사가
// 거의 늘지 않는 야간·주말이 사실상 공짜가 되는 지점이다.
const (
	briefingSlotInSession  = 30 * time.Minute
	briefingSlotOffSession = time.Hour
	/*
		개장 몇 분 전부터 30분 주기로 올릴 것인가.

		**뉴스는 종을 기다리지 않는다.** 국내 08:00~09:00에는 전일 미국 증시 마감,
		시간외 특징주, 개장 전 시황이 몰려 들어오는데 브리핑은 그 시간대를 「장 밖」으로
		보고 한 시간에 한 번만 돌았다. 그래서 09:00에 처음 화면을 여는 사람이 보는
		문장이 **08:00 슬롯 것**, 즉 07:00 이후 기사만 반영한 것이었다.

		시장마다 자기 종에 붙는다 — 국내는 08:00 KST, 미국은 21:30 KST(09:30 ET)다.
		슬롯이 하루에 둘 늘 뿐이고, 그마저도 새 기사가 없으면 LLM을 부르지 않는다.
	*/
	briefingPreOpenLead = time.Hour
	// briefingTickInterval is how often the runner wakes to check whether the
	// wall-clock slot has advanced. Finer than the shortest slot so a slot is
	// never missed, coarse enough to cost nothing.
	briefingTickInterval = time.Minute
)

var briefingMarkets = []string{"domestic", "us"}

// briefingSlot is the wall-clock slot a moment belongs to, per market. Two runs
// inside the same slot are the same run.
func briefingSlot(market string, now time.Time) time.Time {
	step := briefingSlotOffSession
	if calendar.InRegularSessionWithLead(briefingScheduleFor(market), now, briefingPreOpenLead) {
		step = briefingSlotInSession
	}
	local := now.In(seoulLocation())
	minute := (local.Minute() / int(step.Minutes())) * int(step.Minutes())
	return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), minute, 0, 0, local.Location())
}

// Run drives scheduled generation until ctx is cancelled. It is the only
// automatic caller of the LLM for briefings.
func (s *NewsBriefingService) Run(ctx context.Context) {
	if s == nil {
		return
	}
	lastSlot := map[string]time.Time{}

	// 기동 직후 한 번 — 브리핑이 아예 없는 상태로 화면이 뜨는 것을 막는다.
	// 이미 있으면 새 기사 판정에 걸려 LLM을 부르지 않는다.
	s.runDueMarkets(ctx, lastSlot)

	ticker := time.NewTicker(briefingTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDueMarkets(ctx, lastSlot)
		}
	}
}

func (s *NewsBriefingService) runDueMarkets(ctx context.Context, lastSlot map[string]time.Time) {
	now := time.Now()
	spent := false
	for _, market := range briefingMarkets {
		slot := briefingSlot(market, now)
		if prev, ok := lastSlot[market]; ok && !slot.After(prev) {
			continue
		}
		lastSlot[market] = slot

		// 두 시장의 슬롯이 겹치면, 공유 키일 때만 최소 간격(기본 4초)을
		// 지킨다. 키가 국내/해외로 나뉘면 버킷이 달라서 연달아 호출해도 되고,
		// 기다리면 두 번째 시장만 늦어진다.
		if spent && s.llm.SharedBudget() && !s.waitForLLMInterval(ctx, market) {
			return
		}

		safego.Call("news-briefing-"+market, func() {
			/*
				슬롯마다 원장에 한 줄 남긴다 — **건너뛴 슬롯도 포함**해서.

				「새 기사가 없어 건너뜀」과 「스케줄러가 죽어서 안 돎」은 결과가 같아
				보이지만 완전히 다른 일이다. 생성한 것만 기록하면 조용한 주말과 죽은
				프로세스를 구분할 수 없다. 슬롯이 지나갔다는 사실 자체가 신호다.
			*/
			jobID, hasJob := briefingJobID(market)
			record := func(result map[string]any, message string, err error) {
				if !hasJob || s.runs == nil {
					return
				}
				RunScheduledJobResult(ctx, s.runs, jobID, result, message, err)
			}

			_, generated, err := s.Generate(ctx, market, true)
			if err != nil {
				log.Printf("news briefing %s: %v", market, err)
				record(nil, "", err)
				return
			}
			if generated {
				// 실제로 호출을 쓴 경우에만 다음 시장 앞에 간격을 둔다 — 조용해서
				// 건너뛴 시장 뒤에 기다리면 아무 이유 없이 늦어진다.
				spent = true
				log.Printf("news briefing %s: generated for slot %s", market, slot.Format("01-02 15:04"))
			}
			record(
				map[string]any{"slot": slot.Format(time.RFC3339), "generated": generated},
				briefingSlotMessage(slot, generated),
				nil,
			)
		})
	}
}

// briefingJobID maps a briefing market onto its 배치 관리 job id.
func briefingJobID(market string) (model.BatchJobID, bool) {
	switch market {
	case "domestic":
		return model.BatchNewsBriefingDomestic, true
	case "us":
		return model.BatchNewsBriefingOverseas, true
	}
	return "", false
}

func briefingSlotMessage(slot time.Time, generated bool) string {
	if generated {
		return "슬롯 " + slot.Format("01-02 15:04") + " 생성"
	}
	return "슬롯 " + slot.Format("01-02 15:04") + " — 새 기사 없음, 건너뜀"
}

// waitForLLMInterval sleeps out the provider's minimum spacing. Reports false
// when the context was cancelled while waiting.
func (s *NewsBriefingService) waitForLLMInterval(ctx context.Context, market string) bool {
	client := s.llm.For(market)
	if !client.Enabled() {
		return true
	}
	gap := client.Limits().MinInterval
	if gap <= 0 {
		return true
	}
	// 경계에 딱 붙지 않도록 여유를 둔다.
	timer := time.NewTimer(gap + time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
