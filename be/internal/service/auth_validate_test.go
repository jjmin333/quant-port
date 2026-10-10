package service

import (
	"strings"
	"testing"
)

func TestValidateLoginID(t *testing.T) {
	ok := []string{"abcd", "a123", "quant2026", strings.Repeat("a", 20), "qae2e0123456789ab"}
	bad := []string{"", "abc", strings.Repeat("a", 21), "1abc", "Abcd", "ab_cd", "ab.cd", "user@mail.com", "한글아이디", "ab cd"}
	for _, id := range ok {
		if err := validateLoginID(id); err != nil {
			t.Errorf("%q는 통과해야 한다: %v", id, err)
		}
	}
	for _, id := range bad {
		if err := validateLoginID(id); err == nil {
			t.Errorf("%q는 거부돼야 한다", id)
		}
	}
}

func TestValidatePasswordLength(t *testing.T) {
	cases := map[string]bool{
		"1234567":          false, // 7
		"12345678":         true,  // 8
		"123456789012345":  true,  // 15
		"1234567890123456": false, // 16
		"비밀번호한글여덟자":        true,  // 8글자 — 바이트가 아니라 글자로 센다
	}
	for pw, want := range cases {
		if got := validatePassword(pw) == nil; got != want {
			t.Errorf("validatePassword(%q) ok=%v, want %v", pw, got, want)
		}
	}
}

func TestValidateDisplayNameLength(t *testing.T) {
	cases := map[string]bool{
		"a":           false,
		"ab":          true,
		"투자왕":         true,
		"열글자닉네임입니다요":  true,  // 10
		"열한글자닉네임입니다요": false, // 11
		"   ":         false,
	}
	for name, want := range cases {
		if got := validateDisplayName(name) == nil; got != want {
			t.Errorf("validateDisplayName(%q) ok=%v, want %v", name, got, want)
		}
	}
}
