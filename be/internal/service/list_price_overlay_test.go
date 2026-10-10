package service

import "testing"

/*
기준가 고르기는 **두 화면이 공유하는 단 하나의 규칙**이라 여기서 못 박는다
(2026-09-23 QA P0-01).

이 한 줄이 갈리면 미국 애프터마켓에서만 한 세션씩 밀리고, 그건 화면을 봐서는
안 갈린다 — 랭킹과 스크리너가 같은 종목에 다른 등락률을 적어도 둘 다 그럴듯해
보인다. 날짜 비교는 `time.Now()`를 쓰므로, 시험은 **오늘이 언제든 답이 정해지는**
입력만 쓴다(먼 과거 · 먼 미래).
*/
func TestListPrevCloseHint(t *testing.T) {
	cases := []struct {
		name string
		in   ListPriceInput
		want float64
	}{
		{
			name: "종가가 없으면 기준가도 없다 — 0으로 때우지 않는다",
			in:   ListPriceInput{Symbol: "005930", AssetType: "domestic", Close: nil},
			want: 0,
		},
		{
			name: "종가가 0 이하면 없는 것과 같다",
			in:   ListPriceInput{Symbol: "005930", AssetType: "domestic", Close: f64(0)},
			want: 0,
		},
		{
			name: "저장된 종가가 아직 오늘 것이 아니면 그 종가가 곧 직전 종가다",
			in: ListPriceInput{
				Symbol: "005930", AssetType: "domestic",
				Close: f64(2095), PrevClose: f64(2060), PriceAsOf: "2020-01-02",
			},
			want: 2095,
		},
		{
			name: "종가가 이미 오늘 것이면(미국 애프터마켓) 그 행의 prev_close가 직전 종가다",
			in: ListPriceInput{
				Symbol: "AAPL", AssetType: "us",
				Close: f64(190), PrevClose: f64(188), PriceAsOf: "2999-01-01",
			},
			want: 188,
		},
		{
			name: "오늘 것이어도 prev_close가 없으면 종가로 떨어진다",
			in: ListPriceInput{
				Symbol: "AAPL", AssetType: "us",
				Close: f64(190), PrevClose: nil, PriceAsOf: "2999-01-01",
			},
			want: 190,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := listPrevCloseHint(tc.in); got != tc.want {
				t.Errorf("listPrevCloseHint() = %v, want %v", got, tc.want)
			}
		})
	}
}

/*
서비스가 nil이거나 행이 없을 때 판정 배열의 **길이가 입력과 같아야** 한다.

호출자 둘이 모두 `for i, d := range ...`로 행에 자리 맞춰 적어 넣으므로, 길이가
어긋나면 **다른 종목의 가격이 다른 종목 줄에 앉는다.** 그 사고는 조용하다.
*/
func TestResolveListPricesKeepsInputOrder(t *testing.T) {
	in := []ListPriceInput{
		{Symbol: "005930", AssetType: "domestic", Close: f64(78000), PriceAsOf: "2020-01-02"},
		{Symbol: "000660", AssetType: "domestic", Close: f64(184400), PriceAsOf: "2020-01-02"},
		{Symbol: "AAPL", AssetType: "us", Close: nil},
	}

	// liveQuotes가 없는 서비스 — 기준가만 채우고 실시간은 건드리지 않는다.
	var s ScoreService
	got := s.ResolveListPrices(t.Context(), in)

	if len(got) != len(in) {
		t.Fatalf("판정 %d건, 입력 %d건 — 길이가 어긋나면 행이 밀린다", len(got), len(in))
	}
	if got[0].BasePrice != 78000 || got[1].BasePrice != 184400 {
		t.Errorf("기준가 = %v, %v; want 78000, 184400", got[0].BasePrice, got[1].BasePrice)
	}
	if got[2].BasePrice != 0 {
		t.Errorf("종가 없는 행의 기준가 = %v, want 0", got[2].BasePrice)
	}
	for i, d := range got {
		if d.Live {
			t.Errorf("행 %d: liveQuotes가 없는데 Live=true", i)
		}
	}
}
