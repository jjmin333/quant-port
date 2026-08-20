package service

import (
	"context"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestDecideInferredNewsMarket(t *testing.T) {
	cases := []struct {
		name     string
		us, kr   float64
		fallback string
		want     string
	}{
		{
			name:     "clear us",
			us:       6,
			kr:       0,
			fallback: "domestic",
			want:     "us",
		},
		{
			name:     "clear kr",
			us:       0,
			kr:       3,
			fallback: "domestic",
			want:     "domestic",
		},
		{
			name:     "ambiguous leans default",
			us:       1,
			kr:       1,
			fallback: "domestic",
			want:     "domestic",
		},
		{
			name:     "mixed leans domestic",
			us:       2,
			kr:       2,
			fallback: "domestic",
			want:     "domestic",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decideInferredNewsMarket(tc.us, tc.kr, tc.fallback)
			if got != tc.want {
				t.Fatalf("decideInferredNewsMarket(%v, %v, %q) = %q, want %q", tc.us, tc.kr, tc.fallback, got, tc.want)
			}
		})
	}
}

func TestInferNewsMarket_IRENHeadline(t *testing.T) {
	st := &NewsStore{
		usNameIndex: map[string]string{
			"coinbase": "COIN",
			"코인베이스":   "COIN",
		},
	}
	ctx := context.Background()

	title := "IREN 17.55% 급등...코인베이스는 7.73% 하락"
	summary := "미국 증시에서 가상자산 관련주가 장중 대체로 상승했다. IREN($IREN)은 17.55% 급등한 반면 코인베이스($COIN)..."

	got := st.inferNewsMarket(ctx, title, summary, "", "domestic")
	if got != "us" {
		t.Fatalf("inferNewsMarket() = %q, want us", got)
	}
}

func TestInferNewsMarket_kospiStaysDomestic(t *testing.T) {
	st := &NewsStore{nameIndex: map[string]string{"삼성전자": "005930"}}
	ctx := context.Background()

	got := st.inferNewsMarket(ctx,
		"코스피, 외국인 매수에 2700선 회복",
		"증시가 반등했습니다. 삼성전자가 1.2% 올랐다.",
		"",
		"domestic",
	)
	if got != "domestic" {
		t.Fatalf("inferNewsMarket() = %q, want domestic", got)
	}
}

func TestInferNewsMarket_NvidiaKoreanHeadline(t *testing.T) {
	st := &NewsStore{
		usNameIndex: map[string]string{
			"엔비디아": "NVDA",
			"nvidia": "NVDA",
		},
	}
	ctx := context.Background()

	got := st.inferNewsMarket(ctx,
		"중국 헤지펀드, 엔비디아·AI 인프라 비중 줄였다",
		"중국 헤지펀드들이 엔비디아와 AI 인프라 관련 포지션을 줄였다.",
		"",
		"domestic",
	)
	if got != "us" {
		t.Fatalf("inferNewsMarket() = %q, want us", got)
	}
}

func TestSymbolsMentionedInTitle_KoreanAlias(t *testing.T) {
	st := &NewsStore{
		usNameIndex: map[string]string{"엔비디아": "NVDA"},
	}
	ctx := context.Background()

	got := st.symbolsMentionedInTitle(ctx, "중국 헤지펀드, 엔비디아·AI 인프라 비중 줄였다")
	if _, ok := got["NVDA"]; !ok {
		t.Fatalf("symbolsMentionedInTitle() = %v, want NVDA", got)
	}
}

func TestScoreSymbolMarketEvidence(t *testing.T) {
	us, kr := scoreSymbolMarketEvidence([]symbolMarketEvidence{
		{symbol: "IREN", currency: model.CurrencyUSD, inTitle: true, known: true},
		{symbol: "COIN", currency: model.CurrencyUSD, inTitle: false, known: true},
	})
	if us < 4 || kr != 0 {
		t.Fatalf("scoreSymbolMarketEvidence() = (%v, %v), want us>=4 kr=0", us, kr)
	}
	if decideInferredNewsMarket(us, kr, "domestic") != "us" {
		t.Fatal("expected IREN-style evidence to route to us")
	}
}
