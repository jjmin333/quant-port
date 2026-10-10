package service

import (
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/repository"
)

func TestSocialEvent(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	i := func(v int) *int { return &v }
	cases := []struct {
		name    string
		row     repository.ChangeSocialRow
		ok      bool
		subtype string
	}{
		{"too few", repository.ChangeSocialRow{Mentions: 19, Prev: 1, Rank: 5}, false, ""},
		{"steady", repository.ChangeSocialRow{Mentions: 40, Prev: 58, Rank: 1, RankPrev: i(2)}, false, ""},
		{"tripled", repository.ChangeSocialRow{Mentions: 27, Prev: 4, Rank: 3, RankPrev: i(63)}, true, "surge"},
		{"new at 0", repository.ChangeSocialRow{Mentions: 30, Prev: 0, Rank: 20}, true, "surge"},
		{"rank jump", repository.ChangeSocialRow{Mentions: 22, Prev: 12, Rank: 8, RankPrev: i(80)}, true, "surge"},
		{"overheat", repository.ChangeSocialRow{Mentions: 90, Prev: 10, Rank: 2, Return5d: f(22.5)}, true, "overheat"},
	}
	for _, c := range cases {
		e, ok := socialEvent(c.row)
		if ok != c.ok || (ok && e.Subtype != c.subtype) {
			t.Errorf("%s: ok=%v subtype=%q", c.name, ok, e.Subtype)
		}
		if ok && (e.Kind != "social" || !strings.Contains(e.Title, "레딧")) {
			t.Errorf("%s: %+v", c.name, e)
		}
	}
}
