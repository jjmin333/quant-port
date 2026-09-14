package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
한도 소진을 사건 원장에 남긴다 — **호출 경로를 막지 않고**.

# 무엇을 막아야 하나

`apibudget.Check`는 아주 자주 불리고, **소진된 뒤에는 모든 호출이 이 훅으로 온다.**
2026-09-10 DART 소진 당시 그 상태가 7시간 이어졌다. 매 호출마다 DB에 upsert를 하면
기록이 원인보다 큰 부하가 된다.

그래서 둘을 지킨다.

  1. **같은 (provider, scope)는 `quotaIncidentThrottle`에 한 번만 쓴다.** 사건은
     상태이므로(model.Incident 머리글) 같은 소진을 1초에 100번 적을 이유가 없다.
  2. **쓰기는 고루틴으로 떼어 낸다.** 훅은 Check 안에서 불리고 Check는 요청 경로에
     있다 — DB가 느릴 때 시세 응답이 함께 느려지면 안 된다.

# 왜 자정에 저절로 닫히지 않나

한도는 창(하루·한 달)이 넘어가면 알아서 풀린다. 그런데 사건은 「풀렸다」를 **본 적이
있어야** 닫을 수 있고, 이 훅은 소진됐을 때만 불린다. 그래서 닫는 일은
`ResolveRecovered`가 맡고 워치독이 회차마다 그것을 부른다 — 남은 헤드룸이 있으면
열린 소진 사건을 닫는다.
*/
type QuotaIncidentReporter struct {
	budget    *apibudget.Manager
	incidents *repository.OpsIncidentRepo

	mu   sync.Mutex
	last map[string]time.Time
}

/*
quotaIncidentThrottle — 같은 소진을 다시 적기까지의 최소 간격.

5분은 「사람이 화면을 새로 고쳐 볼 만한 간격」이다. 더 짧게 잡아도 얻는 것이 없다 —
행은 하나고 occurrences 만 올라가므로, 그 수가 1분 단위로 정확해질 이유가 없다.
*/
const quotaIncidentThrottle = 5 * time.Minute

func NewQuotaIncidentReporter(
	budget *apibudget.Manager, incidents *repository.OpsIncidentRepo,
) *QuotaIncidentReporter {
	return &QuotaIncidentReporter{
		budget:    budget,
		incidents: incidents,
		last:      map[string]time.Time{},
	}
}

// Install wires the reporter into the budget manager. Call at wiring time.
func (r *QuotaIncidentReporter) Install() {
	if r == nil || r.budget == nil || r.incidents == nil {
		return
	}
	r.budget.SetExhaustedHook(r.onExhausted)
}

func (r *QuotaIncidentReporter) onExhausted(
	provider apibudget.Provider, scope string, used, limit int64,
) {
	if r == nil || r.incidents == nil {
		return
	}
	key := string(provider) + ":" + scope
	r.mu.Lock()
	if at, ok := r.last[key]; ok && time.Since(at) < quotaIncidentThrottle {
		r.mu.Unlock()
		return
	}
	r.last[key] = time.Now()
	r.mu.Unlock()

	subject := string(provider)
	msg := fmt.Sprintf("%s 한도 소진 — %d/%d (%s)", provider, used, limit, scope)
	detail := map[string]any{"scope": scope, "used": used, "limit": limit}

	// 요청 경로를 막지 않는다. 쓰기 실패는 로그로만 남는다 — 기록이 본업을 세우면 안 된다.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		opened, err := r.incidents.Observe(
			ctx, model.IncidentQuotaExhausted, subject,
			model.IncidentSeverityCritical, msg, detail,
		)
		if err != nil {
			log.Printf("quota incident %s: %v", subject, err)
			return
		}
		if opened {
			log.Printf("quota incident opened: %s", msg)
		}
	}()
}

/*
ResolveRecovered — 창이 넘어가 한도가 풀린 소진 사건을 닫는다.

워치독이 회차마다 부른다. 판단은 헤드룸 하나다 — `Headroom`이 남은 호출 수를 주고,
`-1`은 「그 한도가 없다」이므로 둘 다 닫아도 되는 상태다.

**열린 사건만 닫는다**(Resolve가 그렇게 되어 있다). 그래서 이 함수를 매 회차 불러도
닫힌 사건을 다시 건드리지 않는다.
*/
func (r *QuotaIncidentReporter) ResolveRecovered(ctx context.Context) {
	if r == nil || r.budget == nil || r.incidents == nil {
		return
	}
	for _, provider := range apibudget.AllProviders() {
		h := r.budget.Headroom(ctx, provider)
		// 하루·한 달 어느 쪽이든 아직 비어 있으면 사건을 열어 둔다.
		if h.Calls == 0 || h.DayTokens == 0 || h.MonthTokens == 0 {
			continue
		}
		if _, err := r.incidents.Resolve(ctx, model.IncidentQuotaExhausted, string(provider)); err != nil {
			log.Printf("quota incident resolve %s: %v", provider, err)
		}
	}
	// 스로틀도 비운다 — 안 그러면 복구 직후 다시 소진돼도 5분간 기록되지 않는다.
	r.mu.Lock()
	r.last = map[string]time.Time{}
	r.mu.Unlock()
}
