package service

import "testing"

func TestMatchDashboardIndexIDs(t *testing.T) {
	cases := []struct {
		name    string
		title   string
		summary string
		want    []string
	}{
		{
			name:  "kospi",
			title: "코스피, 외국인 매수에 2700선 회복",
			want:  []string{"kospi"},
		},
		{
			name:  "usd-krw",
			title: "원달러 환율, 1450원 돌파",
			want:  []string{"usd-krw"},
		},
		{
			name:  "nasdaq100 aliases",
			title: "Nasdaq 100 hits record high",
			want:  []string{"nasdaq", "nasdaq100", "nq100"},
		},
		{
			name:  "btc crypto",
			title: "비트코인 1억 돌파",
			want:  []string{"btc"},
		},
		{
			name:  "us10y",
			title: "미 국채 10년 금리가 4.3%로 올랐다",
			want:  []string{"us10y"},
		},
		{
			name:  "gold standalone",
			title: "Gold hits record high on safe-haven demand",
			want:  []string{"gold"},
		},
		{
			name:  "goldman not gold",
			title: "Goldman Sachs raises price target",
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchDashboardIndexIDs(tc.title, tc.summary)
			if len(got) != len(tc.want) {
				t.Fatalf("MatchDashboardIndexIDs() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("MatchDashboardIndexIDs() = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestDashboardIndexTags(t *testing.T) {
	got := DashboardIndexTags([]string{"kospi", "btc"})
	want := []string{"index:kospi", "index:btc"}
	if len(got) != len(want) {
		t.Fatalf("DashboardIndexTags() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DashboardIndexTags() = %v, want %v", got, want)
		}
	}
}
