package marketdata

import "testing"

func TestExtractPossessiveHeadlineSubject(t *testing.T) {
	got := ExtractPossessiveHeadlineSubject("Tesla's $99-a-Month FSD Plan Is on Pace")
	if got != "Tesla" {
		t.Fatalf("got %q want Tesla", got)
	}
}

func TestExtractCashtags(t *testing.T) {
	text := "IREN($IREN) surged while Coinbase ($COIN) fell."
	got := ExtractCashtags(text)
	if len(got) != 2 || got[0] != "IREN" || got[1] != "COIN" {
		t.Fatalf("ExtractCashtags() = %v", got)
	}
}

func TestExtractHeadlineTickers(t *testing.T) {
	got := ExtractHeadlineTickers("IREN 17.55% 급등...코인베이스는 7.73% 하락")
	if len(got) != 1 || got[0] != "IREN" {
		t.Fatalf("ExtractHeadlineTickers() = %v", got)
	}
}

func TestExtractExchangeTickers(t *testing.T) {
	text := "Coca-Cola (NYSE: KO) and Microsoft (NASDAQ: MSFT) moved."
	got := ExtractExchangeTickers(text)
	if len(got) != 2 || got[0] != "KO" || got[1] != "MSFT" {
		t.Fatalf("ExtractExchangeTickers() = %v", got)
	}
}

func TestParseRSSTickerList(t *testing.T) {
	got := ParseRSSTickerList("SBUX,SBUX,CMG")
	if len(got) != 2 || got[0] != "SBUX" || got[1] != "CMG" {
		t.Fatalf("ParseRSSTickerList() = %v", got)
	}
}
