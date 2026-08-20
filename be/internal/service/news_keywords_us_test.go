package service

import (
	"context"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/marketdata"
)

func TestExtractUSSymbols_micronAndAliases(t *testing.T) {
	st := &NewsStore{}
	st.mu.Lock()
	st.usNameIndex = map[string]string{
		"micron":            "MU",
		"micron technology": "MU",
		"마이크론":              "MU",
	}
	st.mu.Unlock()
	ctx := context.Background()

	cases := []struct {
		text string
		want string
	}{
		{text: "Micron beats earnings expectations", want: "MU"},
		{text: "마이크론, 실적 전망 상향", want: "MU"},
		{text: "NASDAQ: MU rises after earnings", want: "MU"},
	}
	for _, tc := range cases {
		got := st.extractUSSymbols(ctx, tc.text)
		found := false
		for _, sym := range got {
			if sym == tc.want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("extractUSSymbols(%q) = %v, want %s", tc.text, got, tc.want)
		}
	}
}

func TestExtractUSSymbols_exchangeTicker(t *testing.T) {
	st := &NewsStore{usNameIndex: map[string]string{}}
	got := st.extractUSSymbols(context.Background(), "NASDAQ: MU rises after earnings")
	found := false
	for _, sym := range got {
		if sym == "MU" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected MU from exchange ticker, got %v", got)
	}
}

func TestUSNewsNameAliases_registered(t *testing.T) {
	if usNewsNameAliases["마이크론"] != "MU" {
		t.Fatal("expected 마이크롼 alias for MU")
	}
	if !strings.Contains(strings.Join(defaultUSPollSymbols, ","), "MU") {
		t.Fatal("expected MU in defaultUSPollSymbols")
	}
}

func TestExtractPossessiveMicron(t *testing.T) {
	subject := marketdata.ExtractPossessiveHeadlineSubject("Micron's HBM revenue jumps")
	if subject != "Micron" {
		t.Fatalf("subject = %q, want Micron", subject)
	}
}
