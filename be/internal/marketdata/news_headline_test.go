package marketdata

import "testing"

func TestNormalizeNewsHeadlineKey_stripsPublisherSuffix(t *testing.T) {
	raw := "US embassies issue travel warnings across Middle East - Fox News"
	got := NormalizeNewsHeadlineKey(raw)
	want := NormalizeNewsHeadlineKey("US embassies issue travel warnings across Middle East")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeNewsHeadlineKey_stripsFinancialJuicePrefix(t *testing.T) {
	raw := "FinancialJuice: US embassies issue travel warnings"
	got := NormalizeNewsHeadlineKey(raw)
	want := NormalizeNewsHeadlineKey("US embassies issue travel warnings")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNewsHeadlineMatchKeys_bilingual(t *testing.T) {
	keys := NewsHeadlineMatchKeys(
		"US embassies issue travel warnings - Fox News",
		"미국 대사관들, 중동 전역에 여행 경보 발령",
	)
	if len(keys) != 2 {
		t.Fatalf("NewsHeadlineMatchKeys() = %v, want 2 keys", keys)
	}
}
