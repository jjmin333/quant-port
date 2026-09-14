package apibudget

/*
한도 소진을 **밖에 알리는** 자리.

# 왜 훅인가

2026-09-10에 DART 일일 한도가 저녁이 되기 전에 비었고, 눈에 보인 것은 배치 셋의
실패였다. 원인(한도 소진)은 로그 한 줄로만 남았고 그 로그는 컨테이너 재시작으로
사라졌다 — 조사할 때 이미 없었다.

그래서 소진을 사건 원장에 남기기로 했는데, 이 패키지가 DB를 알면 계층이 뒤집힌다
(`apibudget`은 marketdata 아래고 repository를 모른다). 훅으로 두면 **누가 기록하는지는
배선하는 쪽이 정하고**, 이 패키지는 「소진됐다」만 말한다.

# 왜 day·month 뿐인가

분당 한도는 소진이 아니라 **박자**다. 4초 간격 제공자는 정상 동작 중에도 계속
걸리므로 사건으로 적으면 원장이 그것만으로 찬다. 하루·한 달 한도는 다르다 —
그 창이 끝날 때까지 **아무것도 못 한다**는 뜻이고, 사람이 알아야 한다.

훅은 Check 안에서 불리므로 **절대 막히면 안 된다.** 호출 빈도도 높다(소진된 뒤에는
모든 호출이 여기로 온다). 그 두 가지를 지키는 책임은 배선하는 쪽에 있고,
`service.QuotaIncidentReporter`가 그 자리에서 기록을 조절한다.
*/

// ExhaustedHook is called when a day/month budget is already spent.
// scope is "day" or "month".
type ExhaustedHook func(provider Provider, scope string, used, limit int64)

// SetExhaustedHook installs the reporter. Must be called at wiring time,
// before the manager is shared with request handlers.
func (m *Manager) SetExhaustedHook(hook ExhaustedHook) {
	if m == nil {
		return
	}
	m.exhausted = hook
}

func (m *Manager) reportExhausted(provider Provider, scope string, used, limit int64) {
	if m == nil || m.exhausted == nil {
		return
	}
	m.exhausted(provider, scope, used, limit)
}
