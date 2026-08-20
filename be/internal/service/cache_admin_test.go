package service

import (
	"encoding/json"
	"testing"
)

func TestCacheAdminKeyBlocked(t *testing.T) {
	if !cacheAdminKeyBlocked("toss:oauth:access_token") {
		t.Fatal("oauth token should be blocked")
	}
	if cacheAdminKeyBlocked("quotes:terminal:005930") {
		t.Fatal("quote key should be allowed")
	}
}

func TestCacheAdminKeyReadOnly(t *testing.T) {
	if !cacheAdminKeyReadOnly("apibudget:v1:naver:day") {
		t.Fatal("apibudget should be read-only")
	}
}

func TestCacheAdminScanPattern(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "*"},
		{"005930", "*005930*"},
		{"quotes:terminal:005930", "*quotes:terminal:005930*"},
		{"quotes:*", "quotes:*"},
		{"*005930*", "*005930*"},
	}
	for _, tc := range tests {
		if got := cacheAdminScanPattern(tc.in); got != tc.want {
			t.Fatalf("cacheAdminScanPattern(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatCacheValue_json(t *testing.T) {
	val, format := formatCacheValue(`{"a":1}`)
	if format != "json" {
		t.Fatalf("format = %q want json", format)
	}
	if !jsonValid(val) {
		t.Fatalf("expected pretty json, got %q", val)
	}
}

func jsonValid(s string) bool {
	var v any
	return json.Unmarshal([]byte(s), &v) == nil
}
