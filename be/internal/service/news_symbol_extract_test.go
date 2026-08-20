package service

import (
	"context"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/repository"
)

// 실제 기사 문장으로 국내 종목 추출을 확인한다. 재태깅 뒤에도 "삼성전자"가 붙지
// 않던 기사가 있어, 인덱스가 아니라 매칭이 문제인지 가리기 위한 테스트다.
func TestExtractSymbolsDomesticRealSentence(t *testing.T) {
	st := &NewsStore{
		nameIndex: map[string]string{
			"삼성전자":   "005930",
			"sk하이닉스": "000660",
			"현대차":    "005380",
			"kb금융":   "105560",
		},
	}

	text := `[심층분석] "외인·기관 쌍끌이 매수"…대형 반도체주 중심 수급 쏠림 ` +
		`기관은 SK하이닉스와 삼성전자 외에도 현대차, KB금융 등 실적 개선세가 견조한 대형주를 집중 매수했다.`

	got := st.extractSymbols(context.Background(), text)
	set := map[string]bool{}
	for _, s := range got {
		set[s] = true
	}

	for _, want := range []string{"005930", "000660", "005380", "105560"} {
		if !set[want] {
			t.Errorf("expected %s in %v", want, got)
		}
	}
}

// 종목명 인덱스는 소문자 키다. 기사 원문이 대문자를 섞어 써도(SK하이닉스) 걸려야
// 한다.
func TestExtractSymbolsLowercasesIndexLookup(t *testing.T) {
	st := &NewsStore{nameIndex: map[string]string{"sk하이닉스": "000660"}}
	for _, text := range []string{"SK하이닉스가 올랐다", "sk하이닉스가 올랐다", "Sk하이닉스 급등"} {
		got := st.extractSymbols(context.Background(), text)
		if len(got) != 1 || got[0] != "000660" {
			t.Errorf("extractSymbols(%q) = %v, want [000660]", text, got)
		}
	}
}

// 최장 일치 — 더 긴 상호가 같이 걸리면 짧은 쪽은 그 머리였을 뿐이다.
func TestExtractSymbolsPrefersLongestName(t *testing.T) {
	st := &NewsStore{nameIndex: map[string]string{
		"두산":      "000150",
		"두산에너빌리티": "034020",
		"삼성전자":    "005930",
	}}
	got := st.extractSymbols(context.Background(), "두산에너빌리티 주가 강세…삼성전자도 상승")
	set := map[string]bool{}
	for _, s := range got {
		set[s] = true
	}
	if set["000150"] {
		t.Errorf("두산(000150) should be suppressed by 두산에너빌리티: %v", got)
	}
	if !set["034020"] || !set["005930"] {
		t.Errorf("extractSymbols = %v, want 034020 and 005930", got)
	}
}

func TestExtractSymbolsRejectsAmbiguousWithoutCode(t *testing.T) {
	st := &NewsStore{nameIndex: map[string]string{"대상": "001680"}}

	if got := st.extractSymbols(context.Background(), "인수 대상은 아직 정해지지 않았다"); len(got) != 0 {
		t.Errorf("ambiguous name matched without code: %v", got)
	}
	if got := st.extractSymbols(context.Background(), "대상(001680) 주가 상승"); len(got) != 1 || got[0] != "001680" {
		t.Errorf("ambiguous name with code = %v, want [001680]", got)
	}
}

func TestRetagRecordMirrorsIngest(t *testing.T) {
	st := &NewsStore{nameIndex: map[string]string{"삼성전자": "005930"}}
	got := strings.Join(st.extractSymbols(context.Background(), "삼성전자, 파운드리 수주"), ",")
	if got != "005930" {
		t.Errorf("got %q, want 005930", got)
	}
}

// 국내 6자리 코드가 티커 병합에서 사라지던 회귀.
//
// marketdata.MergeTickerSymbols는 넘겨받은 **모든** 값을 "1~5자 영문 대문자"
// 규칙으로 거른다. 이미 해석된 국내 심볼을 거기에 통과시키면 목록이 통째로
// 비었고, 그래서 국내 기사에 종목이 붙지 않았다.
func TestMergeTickerCandidatesKeepsDomesticCodes(t *testing.T) {
	got := mergeTickerCandidates([]string{"005930", "000660"}, "NVDA")
	want := map[string]bool{"005930": true, "000660": true, "NVDA": true}
	if len(got) != len(want) {
		t.Fatalf("mergeTickerCandidates = %v, want %v entries", got, len(want))
	}
	for _, s := range got {
		if !want[s] {
			t.Errorf("unexpected symbol %q in %v", s, got)
		}
	}
}

func TestMergeTickerCandidatesFiltersNewCandidatesOnly(t *testing.T) {
	// 새 후보는 티커 규칙을 지킨다 — 소문자는 대문자로 올려서 받고, 6자 이상은
	// 버린다. 기존 국내 코드는 그 규칙과 무관하게 남는다.
	got := mergeTickerCandidates([]string{"005930"}, "nvda", "TOOLONGTICKER", "AMD")
	if len(got) != 3 || got[0] != "005930" || got[1] != "NVDA" || got[2] != "AMD" {
		t.Errorf("mergeTickerCandidates = %v, want [005930 NVDA AMD]", got)
	}
}

func TestRetagRecordKeepsDomesticSymbol(t *testing.T) {
	st := &NewsStore{nameIndex: map[string]string{"삼성전자": "005930"}}
	got, _ := st.retagRecord(context.Background(), repository.NewsArticleRecord{
		Market:   "domestic",
		Headline: "[속보]삼성전자, 2Q 확정실적 발표",
		Summary:  "DS부문 영업이익률 69.96%를 기록했다.",
	})
	if len(got) != 1 || got[0] != "005930" {
		t.Errorf("retagRecord = %v, want [005930]", got)
	}
}
