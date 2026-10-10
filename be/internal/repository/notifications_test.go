package repository

import "testing"

func TestEokLabel(t *testing.T) {
	for in, want := range map[int64]string{57700: "5.77억", 114000: "11.4억", 100000: "10억", 9800: "9800만"} {
		if got := eokLabel(in); got != want {
			t.Errorf("eokLabel(%d) = %q, want %q", in, got, want)
		}
	}
}
