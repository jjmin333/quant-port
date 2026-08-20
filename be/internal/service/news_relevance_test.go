package service

import (
	"context"
	"testing"

	"github.com/enact/quant-port/be/internal/marketdata"
)

func TestIsStockRelated_financeSignals(t *testing.T) {
	st := &NewsStore{nameIndex: map[string]string{"삼성전자": "005930"}}
	ctx := context.Background()

	cases := []struct {
		name    string
		title   string
		summary string
		terms   []string
		want    bool
	}{
		{
			name:    "kospi headline",
			title:   "코스피, 외국인 매수에 2700선 회복",
			summary: "증시가 반등했습니다.",
			want:    true,
		},
		{
			name:    "ai politics excluded",
			title:   "李 대통령 지지율, 여론조사에서 40%대로 하락",
			summary: "AI 서밋 외교 성과가 반영되지 않았다.",
			terms:   []string{"AI"},
			want:    false,
		},
		{
			name:    "ai with market context",
			title:   "AI株 급등, 증시 테마주 강세",
			summary: "코스닥 AI 관련 종목이 올랐다.",
			terms:   []string{"AI"},
			want:    true,
		},
		{
			name:    "prison excluded",
			title:   "천안교도소 탈주 수용자 140명",
			summary: "법무부가 조사를 의뢰했다.",
			terms:   []string{"기관"},
			want:    false,
		},
		{
			name:    "foreign tourists excluded",
			title:   "인천공항 이름 바꾸나? 승객 22억 명",
			summary: "외국인 관광객 증가 전망",
			terms:   []string{"외국인"},
			want:    false,
		},
		{
			name:    "foreign investor flows allowed",
			title:   "외국인, 코스피 순매수 전환",
			summary: "증시 자금 유입",
			terms:   []string{"외국인"},
			want:    true,
		},
		{
			name:    "economy rss policy excluded",
			title:   "정부, 산업 정책 발표",
			summary: "새로운 지원책을 내놨다.",
			want:    false,
		},
		{
			name:    "education semiconductor excluded",
			title:   "동일미래과학고, AI·반도체 인재양성 메카로 거듭",
			summary: "지역 산업계와 교육을 연결한다.",
			want:    false,
		},
		{
			name:    "sector with industry outlook allowed",
			title:   "반도체 산업 전망",
			summary: "올해 기업 투자가 확대될 전망이다.",
			want:    true,
		},
		{
			name:    "health topic excluded",
			title:   "HPV 백신 접종률 82%",
			summary: "보건당국이 권고했다.",
			want:    false,
		},
		{
			name:    "trump loose finance excluded",
			title:   "트럼프, 대선 전 금융 지원 공약",
			summary: "정치권이 관심을 보였다.",
			terms:   []string{"트럼프"},
			want:    false,
		},
		{
			name:    "bio health keyword excluded",
			title:   "국내 바이오 신약, 임상 2상 진입",
			summary: "병원과 제약사가 협력한다.",
			terms:   []string{"바이오"},
			want:    false,
		},
		{
			name:    "keyword body only excluded",
			title:   "지역 축제 개최",
			summary: "코스피 관련 행사는 없었다.",
			terms:   []string{"코스피"},
			want:    false,
		},
		{
			name:    "macro headline rss allowed",
			title:   "원달러 환율, 1450원 돌파",
			summary: "외환시장 변동성이 커졌다.",
			want:    true,
		},
		{
			name:    "company name gossip excluded",
			title:   "'35억 건물주' 권성준의 비굴함, \"삼성전자 마케팅팀에 전화해 무조건 달라고 요구\"",
			summary: "influencer 논란이 확산하고 있다.",
			terms:   []string{"삼성전자"},
			want:    false,
		},
		{
			name:    "building owner false positive blocked",
			title:   "35억 건물주 권성준 논란",
			summary: "삼성전자 마케팅팀에 항의 전화를 걸었다.",
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isStockRelated(ctx, st, tc.title, tc.summary, "", tc.terms, nil)
			if got != tc.want {
				t.Fatalf("isStockRelated() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsStockRelated_trackedSymbol(t *testing.T) {
	st := &NewsStore{
		nameIndex: map[string]string{
			"삼성전자": "005930",
		},
	}
	ctx := context.Background()

	got := isStockRelated(ctx, st,
		"삼성전자, HBM3E 양산 확대",
		"반도체 부문 투자를 늘린다.",
		"005930",
		[]string{"삼성전자"},
		nil,
	)
	if !got {
		t.Fatal("expected tracked company article to pass")
	}
}

func TestIsStockRelated_companyNameWithoutFinance(t *testing.T) {
	st := &NewsStore{
		nameIndex: map[string]string{
			"삼성전자": "005930",
		},
	}
	ctx := context.Background()

	got := isStockRelated(ctx, st,
		"'35억 건물주' 권성준, 삼성전자 마케팅팀에 전화해 무조건 달라고 요구",
		"막장 논란이 커지고 있다.",
		"005930",
		[]string{"삼성전자"},
		[]string{"005930"},
	)
	if got {
		t.Fatal("expected gossip article with company name only to be excluded")
	}
}

func TestPassesStockRelevance_coinreadersSource(t *testing.T) {
	st := &NewsStore{}
	ctx := context.Background()

	item := marketdata.RawNewsItem{
		SourceID:    "coinreaders",
		Title:       "비트코인, 1억5000만원 돌파",
		Description: "가상자산 시장이 강세다.",
	}
	if st.passesStockRelevance(ctx, item, nil, "") {
		t.Fatal("expected pure crypto coinreaders article to be excluded")
	}

	item = marketdata.RawNewsItem{
		SourceID:    "coinreaders",
		Title:       "비트마인, 연내 17회 매주 배당 예정",
		Description: "이더리움 축적 기업이 현금 배당을 발표했다.",
	}
	if st.passesStockRelevance(ctx, item, nil, "") {
		t.Fatal("expected crypto dividend article without stock anchor to be excluded")
	}

	item = marketdata.RawNewsItem{
		SourceID:    "coinreaders",
		Title:       "코스피, 외국인 매수에 2%대 상승",
		Description: "증시가 강세를 보였다.",
	}
	if !st.passesStockRelevance(ctx, item, nil, "") {
		t.Fatal("expected stock-related coinreaders article to pass")
	}
}

func TestIsStockRelated_cryptoOnlyBlocked(t *testing.T) {
	st := &NewsStore{}
	ctx := context.Background()

	got := isStockRelated(ctx, st,
		"점프크립토, $1800만 BTC 바이낸스 추가 입금",
		"286.83 BTC를 바이낸스에 입금했다.",
		"", nil, nil,
	)
	if got {
		t.Fatal("expected pure crypto transfer article to be excluded")
	}
}

func TestPassesStockRelevance_cryptoSource(t *testing.T) {
	st := &NewsStore{}
	ctx := context.Background()

	item := marketdata.RawNewsItem{
		SourceID:    "tokenpost",
		Title:       "비트코인, 1억5000만원 돌파",
		Description: "가상자산 시장이 강세다.",
	}
	if st.passesStockRelevance(ctx, item, nil, "") {
		t.Fatal("expected pure crypto RSS article to be excluded")
	}

	item = marketdata.RawNewsItem{
		SourceID:    "tokenpost",
		Title:       "연예인 결혼식 화제",
		Description: "축제 분위기가 이어졌다.",
	}
	if st.passesStockRelevance(ctx, item, nil, "") {
		t.Fatal("expected non-finance crypto RSS article to be excluded")
	}
}
