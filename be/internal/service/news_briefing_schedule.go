package service

import (
	"context"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/safego"
)

// 브리핑 생성 주기.
//
// 장중에는 30분마다, 그 밖(개장 전·마감 후·주말·휴장)에는 1시간마다 — 국내는
// 07:00, 08:00, 09:00, 09:30, 10:00 … 처럼 정시·정각 30분에 걸린다. 슬롯을
// 벽시계에 맞추는 이유는 두 가지다: 「15:30 생성」 같은 표시가 예측 가능해지고,
// 재기동이 잦아도 생성 횟수가 늘지 않는다(같은 슬롯은 한 번만).
//
// 비용은 슬롯 수가 아니라 **새 기사 유무**로 정해진다. Generate가 직전 실행
// 이후 새 기사가 없으면 LLM을 부르지 않고 직전 브리핑을 그대로 둔다 — 기사가
// 거의 늘지 않는 야간·주말이 사실상 공짜가 되는 지점이다.
const (
	briefingSlotInSession  = 30 * time.Minute
	briefingSlotOffSession = time.Hour
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
	if calendar.InRegularSession(briefingScheduleFor(market), now) {
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

		// 두 시장의 슬롯이 겹치면 호출이 연달아 나간다. 제공자 최소 간격(기본
		// 4초)을 안 지키면 두 번째 시장은 매번 예산 검사에서 거부되어 규칙
		// 기반으로 떨어진다 — 실제로 해외 브리핑이 그렇게 되고 있었다.
		if spent && !s.waitForLLMInterval(ctx) {
			return
		}

		safego.Call("news-briefing-"+market, func() {
			_, generated, err := s.Generate(ctx, market, true)
			if err != nil {
				log.Printf("news briefing %s: %v", market, err)
				return
			}
			if generated {
				// 실제로 호출을 쓴 경우에만 다음 시장 앞에 간격을 둔다 — 조용해서
				// 건너뛴 시장 뒤에 기다리면 아무 이유 없이 늦어진다.
				spent = true
				log.Printf("news briefing %s: generated for slot %s", market, slot.Format("01-02 15:04"))
			}
		})
	}
}

// waitForLLMInterval sleeps out the provider's minimum spacing. Reports false
// when the context was cancelled while waiting.
func (s *NewsBriefingService) waitForLLMInterval(ctx context.Context) bool {
	if s.llm == nil || !s.llm.Enabled() {
		return true
	}
	gap := s.llm.Limits().MinInterval
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
