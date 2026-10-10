package service

import "testing"

// TSM: 같은 값을 TWD와 USD 환산치로 두 번 싣는다 — 많이 쓰인 TWD 하나만 남긴다.
func TestPrimaryMonetaryUnit(t *testing.T) {
	counts := map[string]int{"TWD": 900, "USD": 400, "shares": 50, "TWD/shares": 30, "USD/shares": 10, "pure": 5}
	p := primaryMonetaryUnit(counts)
	if p != "TWD" {
		t.Fatalf("primary=%q", p)
	}
	for unit, want := range map[string]bool{"TWD": true, "USD": false, "TWD/shares": true, "USD/shares": false, "shares": true, "pure": true} {
		if got := keepFactUnit(unit, p); got != want {
			t.Errorf("keep %s = %v, want %v", unit, got, want)
		}
	}
	if primaryMonetaryUnit(map[string]int{"USD": 10, "EUR": 10}) != "USD" {
		t.Error("tie must prefer USD")
	}
	if !keepFactUnit("EUR", "") {
		t.Error("no primary → keep all")
	}
}
