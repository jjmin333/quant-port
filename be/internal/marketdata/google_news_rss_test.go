package marketdata

import "testing"

func TestExtractPublisherFromGoogleTitle(t *testing.T) {
	title := "Oil prices rise on Middle East tensions - Reuters"
	got := extractPublisherFromGoogleTitle(title)
	if got != "Reuters" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractPublisherFromGoogleTitleNoDash(t *testing.T) {
	if got := extractPublisherFromGoogleTitle("Plain headline"); got != "" {
		t.Fatalf("got %q", got)
	}
}
