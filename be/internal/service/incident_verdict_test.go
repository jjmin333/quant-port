package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

/*
판정 → 사건 종류 매핑을 못 박는다 (2026-09-11 사용자 요청).

이 매핑이 틀리면 원장이 조용히 거짓을 말한다 — pending(아직 시각이 안 됨)을 사건으로
적으면 매일 새벽마다 「사건」이 수십 개 열리고, late를 critical로 적으면 critical이
아무 뜻도 없어진다.
*/
func TestIncidentKindFor(t *testing.T) {
	cases := []struct {
		health   string
		wantKind model.IncidentKind
		wantSev  string
		wantIs   bool
	}{
		{model.BatchHealthStuck, model.IncidentBatchStuck, model.IncidentSeverityCritical, true},
		{model.BatchHealthFailed, model.IncidentBatchFailed, model.IncidentSeverityCritical, true},
		{model.BatchHealthLate, model.IncidentBatchMissed, model.IncidentSeverityWarn, true},
		// 사건이 아닌 것들 — 이 둘이 사건이 되면 원장이 쓸모없어진다.
		{model.BatchHealthOK, "", "", false},
		{model.BatchHealthPending, "", "", false},
	}
	for _, c := range cases {
		kind, sev, is := incidentKindFor(c.health)
		if is != c.wantIs {
			t.Fatalf("%s: isIncident=%v, want %v", c.health, is, c.wantIs)
		}
		if kind != c.wantKind || sev != c.wantSev {
			t.Fatalf("%s: got (%s,%s), want (%s,%s)", c.health, kind, sev, c.wantKind, c.wantSev)
		}
	}
}

func TestIncidentDuration_openMeasuresUntilNow(t *testing.T) {
	// 「6분 전에 열렸다」와 「6시간째 열려 있다」는 같은 open 이지만 사람이 할 일이 다르다.
	now := mustTime(t, "2026-09-11T12:00:00Z")
	open := model.Incident{FirstSeenAt: mustTime(t, "2026-09-11T06:00:00Z")}
	if got := open.Duration(now); got.Hours() != 6 {
		t.Fatalf("열린 사건은 지금까지를 재야 한다: %v", got)
	}
	if !open.Open() {
		t.Fatal("resolvedAt 이 없으면 열린 것이다")
	}

	done := mustTime(t, "2026-09-11T08:00:00Z")
	closed := model.Incident{FirstSeenAt: mustTime(t, "2026-09-11T06:00:00Z"), ResolvedAt: &done}
	if got := closed.Duration(now); got.Hours() != 2 {
		t.Fatalf("닫힌 사건은 닫힌 시각까지를 재야 한다: %v", got)
	}
	if closed.Open() {
		t.Fatal("resolvedAt 이 있으면 닫힌 것이다")
	}
}
