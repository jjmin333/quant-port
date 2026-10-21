package handler

import "testing"

func TestClientErrorFieldGroupsVariableParts(t *testing.T) {
	a := clientErrorField("window", "/stocks/005930", "Minified React error #418; visit https://react.dev/errors/418")
	b := clientErrorField("window", "/stocks/NVDA", "Minified React error #418; visit https://react.dev/errors/418")
	if a != b {
		t.Fatalf("same error on two stocks should share a field:\n%s\n%s", a, b)
	}
	if want := "window|/stocks/:x|Minified React error #N; visit https://react.dev/errors/N"; a != want {
		t.Fatalf("got %q, want %q", a, want)
	}
	if got := clientErrorField("render", "/news/domestic", "x"); got != "render|/news/domestic|x" {
		t.Fatalf("fixed path changed: %q", got)
	}
}
