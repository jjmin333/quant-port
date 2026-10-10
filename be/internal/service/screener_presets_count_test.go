package service

import (
	"reflect"
	"testing"

	"github.com/enact/quant-port/be/internal/repository"
)

func TestNewcomersKeepsTodayOrderAndCap(t *testing.T) {
	today := []repository.ScreenHit{{Symbol: "A", Name: "에이"}, {Symbol: "B", Name: "비"}, {Symbol: "C"}, {Symbol: "D", Name: "디"}, {Symbol: "E", Name: "이"}}
	before := []repository.ScreenHit{{Symbol: "B"}, {Symbol: "X"}}
	if got, want := newcomers(today, before, 3), []string{"에이", "C", "디"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("newcomers = %v, want %v", got, want)
	}
	if got := newcomers(today[:1], today[:1], 3); got != nil {
		t.Fatalf("no change should give nil, got %v", got)
	}
}
