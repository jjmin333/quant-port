package config

import "testing"

/*
웹소켓과 REST가 **같은 OAuth 앱을 쓴다.**

한때는 갈라 놨다. 근거는 「WS 재연결이 REST 토큰을 죽인다」였는데, 2026-08-25
재실측(`cmd/probe-toss-coexist`)에서 반증됐다 — 연결을 닫아도 토큰은 살아 있고,
같은 토큰으로 재연결도 된다. 토큰을 죽인 진범은 재연결이 부르던 재발급이었고,
그건 `TokenManager.TokenAfter`가 없앴다.

그래서 REST는 SECONDARY failover를 되찾고, 구독은 두 앱을 다 써서 슬롯이 400이
된다. 자세한 근거는 `docs/TOSS_TOKEN_DESIGN.md`.
*/
func TestTossConfig_RESTKeepsSecondaryWhileStreaming(t *testing.T) {
	base := TossConfig{
		BaseURL: "https://x", ClientID: "p", ClientSecret: "ps",
		SecondaryClientID: "s", SecondaryClientSecret: "ss",
	}

	if got := base.RESTConfig(); !got.SecondaryEnabled() {
		t.Fatal("웹소켓이 꺼졌는데 SECONDARY가 빠졌다 — REST failover를 잃는다")
	}

	base.StreamEnabled = true
	if !base.StreamReady() {
		t.Fatal("StreamReady가 false — 자격증명이 있는데 준비 안 됐다고 본다")
	}
	if got := base.RESTConfig(); !got.SecondaryEnabled() {
		t.Fatal("웹소켓이 켜졌다고 SECONDARY를 뺐다 — 공유가 안전해진 뒤로는 failover를 버릴 이유가 없다")
	}
	if got := base.RESTConfig(); got.ClientID != "p" || !got.Enabled() {
		t.Fatal("PRIMARY가 손상됐다")
	}
}

/*
SECONDARY가 없어도 웹소켓은 켤 수 있어야 한다.

앱이 하나면 연결 2개·슬롯 200으로 돌면 그만이다. 예전에는 SECONDARY를 전용으로
써야 해서 없으면 조용히 폴링으로 남았는데, 그 제약이 사라졌다.
*/
func TestTossConfig_StreamRunsOnSingleCredential(t *testing.T) {
	cfg := TossConfig{ClientID: "p", ClientSecret: "ps", StreamEnabled: true}
	if !cfg.StreamReady() {
		t.Fatal("자격증명 하나로는 구독을 못 켠다고 한다 — 이제 그럴 이유가 없다")
	}

	// 자격증명이 아예 없으면 당연히 안 된다.
	none := TossConfig{StreamEnabled: true}
	if none.StreamReady() {
		t.Fatal("자격증명이 없는데 StreamReady가 true다")
	}

	// 스위치가 꺼져 있으면 켜지지 않는다 — 되돌릴 길은 남아 있어야 한다.
	off := TossConfig{ClientID: "p", ClientSecret: "ps"}
	if off.StreamReady() {
		t.Fatal("TOSS_STREAM_ENABLED가 꺼졌는데 구독이 켜진다")
	}
}
