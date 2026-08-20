package service

import (
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

func TestResearchRatableOnlyWhileOpen(t *testing.T) {
	if !researchRatable(model.ResearchCall{Status: model.StatusOpen}) {
		t.Fatal("an open report is ratable")
	}
	for _, s := range []model.ResearchStatus{
		model.StatusTargetHit, model.StatusStopHit, model.StatusExpiredHit,
		model.StatusExpiredMiss, model.StatusRetracted,
	} {
		if researchRatable(model.ResearchCall{Status: s}) {
			t.Fatalf("%s must not be ratable", s)
		}
	}
}

// The viewer key must not be reversible to an address, and must separate
// logged-in readers from anonymous ones sharing an IP.
func TestViewerKeyIsHashedAndScoped(t *testing.T) {
	anon := ViewerKey(0, "203.0.113.9:5312", "Mozilla/5.0")
	same := ViewerKey(0, "203.0.113.9:44100", "Mozilla/5.0")
	if anon != same {
		t.Fatal("the same client on a new source port is the same viewer")
	}
	if len(anon) != 64 {
		t.Fatalf("want a sha256 hex digest, got %d chars", len(anon))
	}
	for _, raw := range []string{"203.0.113.9", "Mozilla"} {
		if strings.Contains(anon, raw) {
			t.Fatalf("viewer key must not carry %q", raw)
		}
	}
	if ViewerKey(7, "203.0.113.9:5312", "Mozilla/5.0") == anon {
		t.Fatal("a logged-in reader is a different viewer from an anonymous one")
	}
}
