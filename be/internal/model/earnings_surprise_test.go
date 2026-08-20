package model

import "testing"

func f(v float64) *float64 { return &v }

// 부호 처리가 이 함수의 존재 이유이므로, 네 갈래를 전부 못 박는다. 순진한
// (actual-estimate)/|estimate|는 적자 예상 종목을 「+840% 서프라이즈」로 만들고,
// 서프라이즈 순 정렬에서 그런 행이 상단을 전부 차지한다.
func TestComputeSurprise(t *testing.T) {
	cases := []struct {
		name     string
		est, act *float64
		wantNil  bool
		wantPct  *float64
		wantLbl  string
	}{
		{
			// 삼성전자 2026 2Q 영업이익: 84.8조 기대 → 89.5조 발표.
			name: "정상 상회", est: f(848366.63), act: f(894924),
			wantPct: f(5.4878832280331435),
		},
		{name: "정상 하회", est: f(100), act: f(90), wantPct: f(-10)},
		{
			// 기준선이 0 이하면 퍼센트를 만들 수 없다 — 말로 답한다.
			name: "적자 예상 → 흑자", est: f(-50), act: f(10), wantLbl: "흑자전환",
		},
		{name: "적자 예상 → 적자", est: f(-50), act: f(-30), wantLbl: "적자지속"},
		{
			// 여기는 퍼센트도 의미가 있다(흑자 기대 대비 얼마나 빗나갔나).
			// 둘 중 하나만 주면 정보를 버리는 셈이라 함께 낸다.
			name: "흑자 예상 → 적자", est: f(100), act: f(-50),
			wantPct: f(-150), wantLbl: "적자전환",
		},
		{name: "기준선 0, 흑자", est: f(0), act: f(10), wantLbl: "흑자전환"},
		{name: "추정치 없음", est: nil, act: f(10), wantNil: true},
		{name: "발표치 없음", est: f(10), act: nil, wantNil: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeSurprise(EarningsMetricOpIncome, EarningsMetric{Estimate: tc.est, Actual: tc.act})
			if tc.wantNil {
				if got != nil {
					t.Fatalf("surprise = %+v, want nil (한쪽이 없으면 비교 자체가 없다)", got)
				}
				return
			}
			if got == nil {
				t.Fatal("surprise = nil, want a value")
			}
			if tc.wantPct == nil && got.Pct != nil {
				t.Errorf("pct = %v, want nil", *got.Pct)
			}
			if tc.wantPct != nil {
				if got.Pct == nil {
					t.Fatalf("pct = nil, want %v", *tc.wantPct)
				}
				if diff := *got.Pct - *tc.wantPct; diff > 1e-9 || diff < -1e-9 {
					t.Errorf("pct = %v, want %v", *got.Pct, *tc.wantPct)
				}
			}
			if got.Label != tc.wantLbl {
				t.Errorf("label = %q, want %q", got.Label, tc.wantLbl)
			}
		})
	}
}

// 시장마다 「어닝 서프라이즈」로 부르는 지표가 다르다. 국내 뉴스는 "영업이익
// 컨센서스 상회"라고 쓰고 미국은 "EPS beat"이라고 쓴다 — 한쪽 관행을 양쪽에
// 밀어붙이면 한 시장에서는 아무도 그 숫자로 이야기하지 않는다.
func TestHeadlineSurpriseMetricPerMarket(t *testing.T) {
	us := &EarningsEvent{
		Market: string(AssetUS),
		EPS:    EarningsMetric{Estimate: f(3.6761), Actual: f(3.65)},
	}
	if s := us.HeadlineSurprise(); s == nil || s.Metric != EarningsMetricEPS {
		t.Fatalf("US headline metric = %+v, want eps", s)
	}

	kr := &EarningsEvent{
		Market:   string(AssetDomestic),
		OpIncome: EarningsMetric{Estimate: f(2044), Actual: f(2565)},
	}
	if s := kr.HeadlineSurprise(); s == nil || s.Metric != EarningsMetricOpIncome {
		t.Fatalf("KR headline metric = %+v, want opIncome", s)
	}

	// 폴백: 국내 소스는 EPS를 안 주고 미국 소스는 영업이익을 안 준다. 주력 지표에
	// 짝이 없으면 다른 쪽으로 넘어가되, Metric에 어느 쪽인지 정확히 적어야
	// 화면이 「영업이익 기준」이라고 이름을 붙일 수 있다.
	fallback := &EarningsEvent{
		Market:   string(AssetUS),
		OpIncome: EarningsMetric{Estimate: f(100), Actual: f(110)},
	}
	s := fallback.HeadlineSurprise()
	if s == nil || s.Metric != EarningsMetricOpIncome {
		t.Fatalf("fallback metric = %+v, want opIncome", s)
	}

	if (&EarningsEvent{Market: string(AssetUS)}).HeadlineSurprise() != nil {
		t.Error("no metrics at all must yield nil, not a zero-valued surprise")
	}
}
