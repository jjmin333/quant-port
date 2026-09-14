package service

import (
	"context"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/marketdata"
)

func TestExtractUSSymbols_micronAndAliases(t *testing.T) {
	st := &NewsStore{}
	st.mu.Lock()
	st.usNameIndex = map[string]string{
		"micron":            "MU",
		"micron technology": "MU",
		"마이크론":              "MU",
	}
	st.mu.Unlock()
	ctx := context.Background()

	cases := []struct {
		text string
		want string
	}{
		{text: "Micron beats earnings expectations", want: "MU"},
		{text: "마이크론, 실적 전망 상향", want: "MU"},
		{text: "NASDAQ: MU rises after earnings", want: "MU"},
	}
	for _, tc := range cases {
		got := st.extractUSSymbols(ctx, tc.text)
		found := false
		for _, sym := range got {
			if sym == tc.want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("extractUSSymbols(%q) = %v, want %s", tc.text, got, tc.want)
		}
	}
}

func TestExtractUSSymbols_exchangeTicker(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{}}
	got := st.extractUSSymbols(context.Background(), "NASDAQ: MU rises after earnings")
	found := false
	for _, sym := range got {
		if sym == "MU" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected MU from exchange ticker, got %v", got)
	}
}

func TestUSNewsNameAliases_registered(t *testing.T) {
	// 별칭은 이제 `stock_aliases` 표에서 온다(migration 070). DB가 없는 시험에서는
	// 씨앗이 그대로 나오므로 이 검사는 「씨앗에 남아 있나」를 센다 — 표로 옮기면서
	// 실제로 한 번 빠뜨렸던 값이다(토스 상장명이 「마이크론 테크놀로지」라 짧은
	// 통칭은 이름 색인만으로는 닿지 않는다).
	if usNewsNameAliases()["마이크론"] != "MU" {
		t.Fatal("expected 마이크론 alias for MU")
	}
	if !strings.Contains(strings.Join(defaultUSPollSymbols, ","), "MU") {
		t.Fatal("expected MU in defaultUSPollSymbols")
	}
}

func TestExtractPossessiveMicron(t *testing.T) {
	subject := marketdata.ExtractPossessiveHeadlineSubject("Micron's HBM revenue jumps")
	if subject != "Micron" {
		t.Fatalf("subject = %q, want Micron", subject)
	}
}

/*
회사명 인덱스가 **일반 영어 단어를 종목으로 만들면 안 된다.**

실측(2026-08-24, 최근 3일): HERE 49건 · BOND 39 · OPEN 27 · GOLD 26 ·
HONG 14 · FIVE 11이 오탐이었다. 「금값」·「채권」·「홍콩」 기사에
GOLD.COM INC · PIMCO ACTIVE BOND ETF · LEVERAGE SHARES 2X HON이 붙었다.

원인이 둘이었다:

	① 주석은 「티커 자체는 이름 인덱스에 넣지 않는다」고 적혀 있는데 코드는 넣었다.
	② 회사명에서 도메인·접미사를 떼면 "gold.com inc" → "gold"가 됐다.
*/
// 티커는 **이름 인덱스가 아니라 티커 인덱스**로 간다. 이름 인덱스는 소문자 본문에
// 맞추므로, 거기 티커를 넣으면 「stocks fall amid …」가 AMID 기사가 된다.
func TestAddUSNameIndexKeys_tickerGoesToTickerIndexNotNameIndex(t *testing.T) {
	idx := map[string]string{}
	tickers := map[string]string{}
	addUSNameIndexKeys(idx, tickers, "CRWD", "CROWDSTRIKE HOLDINGS INC", "")

	if tickers["CRWD"] != "CRWD" {
		t.Fatalf("티커가 티커 인덱스에 없다: %v", tickers)
	}
	if sym, ok := idx["crwd"]; ok {
		t.Fatalf("티커가 소문자 이름 인덱스에 들어갔다 (%q) — 소문자 낱말과 부딪힌다", sym)
	}
	if idx["crowdstrike"] != "CRWD" {
		t.Fatalf("회사명이 인덱스에 없다: %v", idx)
	}
}

/*
흔한 영어 낱말과 같은 티커는 **자유 본문에서 아예 안 잡힌다.**

실측(2026-09-03, 최근 US 기사 1,000건)에서 이 종류가 스물두 개 나왔다 —
MOVE 30 · AMID 20 · ALTO 18 · PALO 18 · HIGH 17 · LIVE 15 · TECH 14 · GAIN 13 …
「stocks fall amid rate fears」가 ARGENT MID CAP ETF 기사로 집계되고 있었다.
*/
func TestAddUSNameIndexKeys_wordLikeTickersStayOutOfBothIndexes(t *testing.T) {
	idx := map[string]string{}
	tickers := map[string]string{}
	addUSNameIndexKeys(idx, tickers, "AMID", "ARGENT MID CAP ETF", "")
	addUSNameIndexKeys(idx, tickers, "HIGH", "SIMPLIFY ENHANCED INCOME ETF", "")
	addUSNameIndexKeys(idx, tickers, "MOVE", "MOVANO INC", "")
	addUSNameIndexKeys(idx, tickers, "TECH", "BIO-TECHNE CORP", "")

	for _, sym := range []string{"AMID", "HIGH", "MOVE", "TECH"} {
		if got, ok := tickers[sym]; ok {
			t.Errorf("낱말 티커 %q가 자유 본문 매칭에 들어갔다 (%q)", sym, got)
		}
		if got, ok := idx[strings.ToLower(sym)]; ok {
			t.Errorf("낱말 티커 %q가 이름 인덱스에 들어갔다 (%q)", sym, got)
		}
	}
	// 회사명 쪽은 그대로다 — Movano 기사는 여전히 MOVE로 잡혀야 한다.
	if idx["movano"] != "MOVE" {
		t.Fatalf("회사명이 사라졌다: %v", idx)
	}
}

func TestAddUSNameIndexKeys_blocksCommonWords(t *testing.T) {
	idx := map[string]string{}
	addUSNameIndexKeys(idx, nil, "GOLD", "GOLD.COM INC", "")
	addUSNameIndexKeys(idx, nil, "BOND", "PIMCO ACTIVE BOND EXCHANGE-TRADED FUND", "")
	addUSNameIndexKeys(idx, nil, "HERE", "HERE GP LTD", "")

	for _, word := range []string{"gold", "bond", "here", "hong", "open", "five"} {
		if sym, ok := idx[word]; ok {
			t.Fatalf("불용어 %q가 %s로 인덱싱됐다 — 그 단어가 든 모든 기사에 붙는다", word, sym)
		}
	}
}

// 불용어를 막는다고 진짜 회사명까지 막으면 안 된다.
func TestAddUSNameIndexKeys_keepsRealCompanyNames(t *testing.T) {
	idx := map[string]string{}
	addUSNameIndexKeys(idx, nil, "NVDA", "NVIDIA CORP", "")
	addUSNameIndexKeys(idx, nil, "AMZN", "AMAZON.COM INC", "")

	if idx["nvidia"] != "NVDA" {
		t.Fatalf("nvidia가 없다: %v", idx)
	}
	if idx["amazon"] != "AMZN" {
		t.Fatalf("amazon이 없다 — 도메인 꼬리 제거가 깨졌다: %v", idx)
	}
}

/*
이름 칸이 티커와 같으면 **자유 본문 매치에서 빠진다** (2026-09-13).

실측으로 잡았다 — AMID(ARGENT MID CAP ETF)는 `stocks.name`도 「AMID」라서
이름 인덱스에 `amid`가 들어갔고, "OpenAI delaying IPO amid AI safety concerns"
기사에 그 ETF가 붙었다(09-12~13 이틀에 20건). 티커 인덱스는 이런 낱말 티커를
이미 빼 두는데, 이름 칸이 **같은 방어를 우회하는 두 번째 문**이었다.
*/
func TestUSNameIndex_wordLikeTickerIsNotNameIndexed(t *testing.T) {
	idx := map[string]string{}
	tickers := map[string]string{}
	addUSNameIndexKeys(idx, tickers, "AMID", "AMID", "AMID")

	if sym, ok := idx["amid"]; ok {
		t.Fatalf("낱말 티커가 이름 인덱스에 들어갔다: amid → %s", sym)
	}
	// 티커 인덱스에도 없어야 한다 — 명시 표기($AMID) 경로로만 붙는다.
	if _, ok := tickers["AMID"]; ok {
		t.Fatal("낱말 티커가 자유 본문 티커 인덱스에 들어갔다")
	}

	// 진짜 이름이 있으면 그 이름은 그대로 색인된다.
	idx2 := map[string]string{}
	addUSNameIndexKeys(idx2, map[string]string{}, "AMID", "ARGENT MID CAP ETF", "")
	if idx2["argent mid cap etf"] != "AMID" {
		t.Fatalf("영문 이름이 색인되지 않았다: %v", idx2)
	}

	// 낱말이 아닌 티커는 이름 칸이 같아도 색인된다(NVDA 같은 것 — 잃을 이유가 없다).
	idx3 := map[string]string{}
	addUSNameIndexKeys(idx3, map[string]string{}, "NVDA", "NVDA", "NVDA")
	if idx3["nvda"] != "NVDA" {
		t.Fatalf("낱말이 아닌 티커까지 이름 인덱스에서 빠졌다: %v", idx3)
	}
}
