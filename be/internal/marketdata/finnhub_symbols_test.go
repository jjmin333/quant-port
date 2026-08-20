package marketdata

import "testing"

func TestIsUSExchangeTradedProduct(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"ETF", true},
		{"ETN", true},
		{"ETP", true},
		{"Common Stock", false},
		{"ADR", false},
		{"Leveraged ETF", true},
		{"", false},
	}
	for _, tc := range tests {
		if got := IsUSExchangeTradedProduct(tc.in); got != tc.want {
			t.Errorf("IsUSExchangeTradedProduct(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
