package model

import "time"

/*
운영 사건 — 「무엇이 깨졌고, 언제부터, 아직 깨져 있나」.

# 왜 배치 원장으로는 안 됐나

`batch_job_runs`는 **실행된 것만** 안다. 안 돌아간 슬롯은 아무 흔적도 남기지 않으므로
「돌았어야 했는데 안 돌았다」가 그 표에 없다. `batch_health`는 단 한 행이라 지금
상태만 있고 이력이 없다. 2026-09-10 DART 한도 소진을 조사할 때 「언제부터 깨져
있었나」를 컨테이너 로그에서 찾아야 했고, 그 로그는 재시작으로 이미 사라져 있었다.

# 사건은 줄이 아니라 상태다

워치독은 15분마다 돈다. 늦은 잡 하나가 회차마다 한 행을 만들면 하루 96행이 쌓이고
「몇 건인가」가 「몇 번 쳐다봤나」를 뜻하게 된다 — 실측으로 그 로그가 그랬다
(`financials_domestic` 한 건이 같은 줄을 18번 넘게 찍었다).

그래서 (kind, subject)당 **열린 사건은 최대 하나**고, 같은 일이 다시 보이면
`occurrences`와 `last_seen_at`만 올라간다. 조건이 풀리면 닫고, 또 깨지면 **새 행**이
열린다 — 그래야 재발이 세어진다.
*/

// IncidentKind는 사건의 종류다. DB의 kind 컬럼과 같은 값을 쓴다.
type IncidentKind string

const (
	// 돌았어야 했는데 안 돌았다 (워치독의 late 판정).
	IncidentBatchMissed IncidentKind = "batch_missed"
	// 돌았고 실패했다.
	IncidentBatchFailed IncidentKind = "batch_failed"
	// 시작은 했는데 최대 실행 시간을 넘겼다.
	IncidentBatchStuck IncidentKind = "batch_stuck"
	/*
		외부 API 일일·월간 한도를 다 썼다.

		이 종류가 따로 있는 이유는 **원인과 결과가 다른 자리에 나타나기** 때문이다.
		2026-09-10에 눈에 보인 것은 배치 셋의 실패였지만 원인은 한도 소진이었고, 그
		둘을 같은 종류로 적으면 조사가 다시 로그로 내려간다.
	*/
	IncidentQuotaExhausted IncidentKind = "quota_exhausted"
	// 상류가 살아 있지만 정상보다 나쁘다 (양쪽 키 429, 회로 열림 등).
	IncidentUpstreamDegraded IncidentKind = "upstream_degraded"
)

// 심각도. 화면 정렬과 알림 문턱에만 쓴다.
const (
	IncidentSeverityWarn     = "warn"
	IncidentSeverityCritical = "critical"
)

// Incident is one row of the ops ledger.
type Incident struct {
	ID          uint64         `json:"id"`
	Kind        IncidentKind   `json:"kind"`
	Subject     string         `json:"subject"`
	Severity    string         `json:"severity"`
	Message     string         `json:"message"`
	Detail      map[string]any `json:"detail,omitempty"`
	Occurrences int            `json:"occurrences"`
	FirstSeenAt time.Time      `json:"firstSeenAt"`
	LastSeenAt  time.Time      `json:"lastSeenAt"`
	ResolvedAt  *time.Time     `json:"resolvedAt,omitempty"`
}

// Open reports whether the incident is still unresolved.
func (i Incident) Open() bool { return i.ResolvedAt == nil }

/*
Duration — 사건이 얼마나 이어졌나.

열려 있으면 **지금까지**다. 「6분 전에 열렸다」와 「6시간째 열려 있다」는 같은 open
상태지만 사람이 해야 할 일이 다르다.
*/
func (i Incident) Duration(now time.Time) time.Duration {
	if i.ResolvedAt != nil {
		return i.ResolvedAt.Sub(i.FirstSeenAt)
	}
	return now.Sub(i.FirstSeenAt)
}
