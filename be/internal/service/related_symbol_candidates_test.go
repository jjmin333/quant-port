package service

import (
	"slices"
	"testing"
)

func TestRelatedSymbolCandidatesSNDK(t *testing.T) {
	got := relatedSymbolCandidates("SNDK")
	want := []string{"SNXX", "SNDQ", "SNDU", "SNDG", "SNDX", "SNDD", "SNDL"}
	for _, sym := range want {
		if !slices.Contains(got, sym) {
			t.Fatalf("missing %s in %v", sym, got)
		}
	}
}

func TestRelatedSymbolCandidatesQQQ(t *testing.T) {
	got := relatedSymbolCandidates("QQQ")
	for _, sym := range []string{"TQQQ", "SQQQ", "QLD", "PSQ"} {
		if !slices.Contains(got, sym) {
			t.Fatalf("missing %s in %v", sym, got)
		}
	}
}
