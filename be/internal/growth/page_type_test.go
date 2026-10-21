package growth

import "testing"

func TestPageTypeHubsAreSections(t *testing.T) {
	cases := map[string]string{
		"https://www.quant-port.com/":                    "home",
		"https://www.quant-port.com/stocks/005930":       "stock",
		"https://www.quant-port.com/stocks/sector/kr/it": "section",
		"https://www.quant-port.com/stocks/style/value":  "section",
		"https://www.quant-port.com/reports/12":          "report",
		"https://www.quant-port.com/reports/authors/7":   "section",
	}
	for u, want := range cases {
		if got := pageType(u); got != want {
			t.Errorf("pageType(%s) = %s, want %s", u, got, want)
		}
	}
}
