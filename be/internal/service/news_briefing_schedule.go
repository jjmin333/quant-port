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
	/*
		슬롯이 AI 문장 없이 끝났을 때 **그 슬롯 안에서** 다시 물어보는 간격.

		장 밖 슬롯은 한 시간이라, 예전에는 503 한 번이 곧 한 시간짜리 규칙·승계
		문장이었다. 503은 몇 시간 이어지지만 **그 사이 잠깐씩 뚫린다**(09-25 01:14 국내가
		재시도에서 주 모델 성공) — 자주 물을수록 그 틈을 잡는다. 5분이면 장 밖 슬롯에서
		열한 번, 장중 30분 슬롯에서 다섯 번 더 묻는다.

		우리 예산에는 잡히지 않는다(apibudget.Record는 HTTP <400에서만 돈다 — 호출 수 ·
		분당 · 최소 간격 전부). **Google 무료 한도가 503을 세는지는 문서에 없다** — 그래서
		하루 상한(briefingRetryDailyCap)을 둔다. 세지 않으면 상한은 안 걸리고, 센다면
		종목 AI 요약의 하루 몫을 지킨다. 한도 초과는 다시 묻지 않는다(needsRetry).
	*/
	briefingSlotRetryInterval = 5 * time.Minute
	// briefingRetryDailyCap bounds in-slot retries per market per briefing day. 관측된
	// 503 구간(22:00~03:00, 5시간)을 5분 간격으로 덮는 60회 — 재시도 한 번은 주 모델 +
	// 대체 모델 두 요청이라 최악에도 키당 하루 120요청(각 모델 60)이다.
	briefingRetryDailyCap = 60
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

// briefingRunState is the runner's memory between ticks: which slot each market
// last ran, and when (if at all) the current slot should be tried again.
type briefingRunState struct {
	lastSlot map[string]time.Time
	retryAt  map[string]time.Time
	// retries counts in-slot retries per market for retryDay (briefing day,
	// "2006-01-02"); briefingRetryDailyCap bounds it.
	retries  map[string]int
	retryDay map[string]string
}

func newBriefingRunState() *briefingRunState {
	return &briefingRunState{
		lastSlot: map[string]time.Time{},
		retryAt:  map[string]time.Time{},
		retries:  map[string]int{},
		retryDay: map[string]string{},
	}
}

// retriesToday reports how many retries this market has used in now's briefing
// day, resetting the count when the day turns. 날은 서울 자정이 아니라
// briefingDay(개장 준비 시각)로 센다 — 자정은 503 구간(22:00~03:00) 한가운데라,
// 거기서 새로 세면 한 밤에 상한을 두 번 쓴다.
func (st *briefingRunState) retriesToday(market string, now time.Time) int {
	day := briefingDay(market, now).Format("2006-01-02")
	if st.retryDay[market] != day {
		st.retryDay[market] = day
		st.retries[market] = 0
	}
	return st.retries[market]
}

// briefingAction is what a tick should do for one market.
type briefingAction int

const (
	briefingIdle    briefingAction = iota
	briefingSlotRun                // the wall-clock slot advanced
	briefingRetry                  // same slot, its narrative failed and the retry is due
)

// due decides the tick for one market and advances the state for a new slot.
func (st *briefingRunState) due(market string, now time.Time) (briefingAction, time.Time) {
	slot := briefingSlot(market, now)
	if prev, ok := st.lastSlot[market]; !ok || slot.After(prev) {
		st.lastSlot[market] = slot
		delete(st.retryAt, market)
		return briefingSlotRun, slot
	}
	if at, ok := st.retryAt[market]; ok && !now.Before(at) {
		st.retriesToday(market, now)
		st.retries[market]++
		return briefingRetry, slot
	}
	return briefingIdle, slot
}

/*
seedFromStored — 기동할 때 「이번 슬롯은 이미 돌았다」를 저장된 브리핑에서 되살린다.

**재배포마다 LLM을 한 번 더 불렀다** (2026-09-26 실측). 상태가 메모리에만 있어 기동 직후
현재 슬롯을 무조건 다시 돌렸고, 「새 기사 없으면 건너뜀」은 막아 주지 못했다 — 뉴스는
20분이면 늘 새로 들어온다. 원장에 「슬롯 09-26 18:00 생성」이 18:00·18:20 두 번 찍혔다.

AI 문장으로 끝난 행만 되살린다. 규칙·승계 행이면 그 슬롯은 아직 AI 문장을 못 얻은
것이라 기동 실행이 곧 재시도다(예전 동작 그대로).
*/
func (st *briefingRunState) seedFromStored(market string, prev *model.NewsMarketBriefing, now time.Time) bool {
	if prev == nil || prev.Source != model.BriefingSourceLLM || prev.GeneratedAt.IsZero() {
		return false
	}
	slot := briefingSlot(market, now)
	if !briefingSlot(market, prev.GeneratedAt).Equal(slot) {
		return false
	}
	st.lastSlot[market] = slot
	return true
}

// settle schedules the next in-slot retry after a run, or clears it.
func (st *briefingRunState) settle(market string, now time.Time, run briefingRun) {
	if run.needsRetry() && st.retriesToday(market, now) < briefingRetryDailyCap {
		st.retryAt[market] = now.Add(briefingSlotRetryInterval)
		return
	}
	if run.needsRetry() {
		log.Printf("news briefing %s: %d retries used this briefing day — none until the next pre-open", market, briefingRetryDailyCap)
	}
	delete(st.retryAt, market)
}

// Run drives scheduled generation until ctx is cancelled. It is the only
// automatic caller of the LLM for briefings.
func (s *NewsBriefingService) Run(ctx context.Context) {
	if s == nil {
		return
	}
	state := newBriefingRunState()

	// 이번 슬롯을 이미 AI 문장으로 끝냈으면 기동 실행을 건너뛴다(seedFromStored).
	if s.repo != nil {
		now := time.Now()
		for _, market := range briefingMarkets {
			prev, err := s.repo.Latest(ctx, market)
			if err != nil {
				continue
			}
			if state.seedFromStored(market, prev, now) {
				log.Printf("news briefing %s: slot %s already has an AI narrative — startup run skipped",
					market, briefingSlot(market, now).Format("01-02 15:04"))
			}
		}
	}

	// 기동 직후 한 번 — 브리핑이 아예 없는 상태로 화면이 뜨는 것을 막는다.
	s.runDueMarkets(ctx, state)

	ticker := time.NewTicker(briefingTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDueMarkets(ctx, state)
		}
	}
}

func (s *NewsBriefingService) runDueMarkets(ctx context.Context, state *briefingRunState) {
	now := time.Now()
	spent := false
	for _, market := range briefingMarkets {
		action, slot := state.due(market, now)
		if action == briefingIdle {
			continue
		}

		// 두 시장의 슬롯이 겹치면, 공유 키일 때만 최소 간격(기본 4초)을
		// 지킨다. 키가 국내/해외로 나뉘면 버킷이 달라서 연달아 호출해도 되고,
		// 기다리면 두 번째 시장만 늦어진다.
		if spent && s.llm.SharedBudget() && !s.waitForLLMInterval(ctx, market) {
			return
		}

		if action == briefingRetry {
			safego.Call("news-briefing-retry-"+market, func() {
				/*
					슬롯 안 재시도. 원장에는 남기지 않는다 — 원장은 「슬롯이 돌았나」를
					보는 곳이고 그건 슬롯 첫 실행이 이미 적었다. 실패한 재시도는 행도
					쓰지 않는다(upgradeOnly): 카드에는 슬롯 첫 실행의 승계·규칙 문장이
					그대로 있다.
				*/
				run, err := s.generate(ctx, market, briefingGenOptions{upgradeOnly: true})
				if err != nil {
					log.Printf("news briefing %s: retry: %v", market, err)
				}
				state.settle(market, time.Now(), run)
				if run.generated {
					spent = true
					log.Printf("news briefing %s: retry recovered the AI narrative for slot %s", market, slot.Format("01-02 15:04"))
				}
			})
			continue
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

			run, err := s.generate(ctx, market, briefingGenOptions{skipWhenQuiet: true})
			state.settle(market, time.Now(), run)
			if err != nil {
				log.Printf("news briefing %s: %v", market, err)
				record(nil, "", err)
				return
			}
			if run.generated {
				// 실제로 호출을 쓴 경우에만 다음 시장 앞에 간격을 둔다 — 조용해서
				// 건너뛴 시장 뒤에 기다리면 아무 이유 없이 늦어진다.
				spent = true
				log.Printf("news briefing %s: generated for slot %s", market, slot.Format("01-02 15:04"))
			}
			if _, pending := state.retryAt[market]; pending {
				log.Printf("news briefing %s: no AI narrative for slot %s — retrying in %s",
					market, slot.Format("01-02 15:04"), briefingSlotRetryInterval)
			}
			record(
				map[string]any{"slot": slot.Format(time.RFC3339), "generated": run.generated},
				briefingSlotMessage(slot, run.generated),
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
