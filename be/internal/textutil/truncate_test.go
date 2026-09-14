package textutil

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateRunesKeepsHangulWhole(t *testing.T) {
	// 「가나다라마」는 15바이트다. 바이트로 10을 자르면 네 번째 글자가 반으로
	// 쪼개지지만, 글자로 자르면 딱 넷이 남는다.
	const name = "가나다라마"
	got := TruncateRunes(name, 4)
	if got != "가나다라" {
		t.Fatalf("TruncateRunes(%q, 4) = %q", name, got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("truncated %q is not valid UTF-8", got)
	}
}

func TestTruncateRunesShorterThanLimit(t *testing.T) {
	for _, s := range []string{"", "abc", "한글", "한 글 자"} {
		if got := TruncateRunes(s, 10); got != s {
			t.Fatalf("TruncateRunes(%q, 10) = %q, want unchanged", s, got)
		}
	}
}

func TestTruncateRunesAtDBColumnWidth(t *testing.T) {
	// symbol_sync_state.message는 VARCHAR(255) — MySQL도 문자로 센다.
	msg := strings.Repeat("종목", 200) // 400자 · 1,200바이트
	got := TruncateRunes(msg, 255)
	if n := utf8.RuneCountInString(got); n != 255 {
		t.Fatalf("rune count = %d, want 255", n)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("truncated message is not valid UTF-8")
	}
}

func TestTruncateRunesNonPositiveLimit(t *testing.T) {
	if got := TruncateRunes("한글", 0); got != "" {
		t.Fatalf("TruncateRunes(_, 0) = %q, want empty", got)
	}
	if got := TruncateRunes("한글", -1); got != "" {
		t.Fatalf("TruncateRunes(_, -1) = %q, want empty", got)
	}
}

func TestTruncateRunesMixedWidths(t *testing.T) {
	// 영문·한글·이모지가 섞여도 경계는 언제나 글자 경계다.
	s := "ab한글🙂cd"
	for n := 0; n <= utf8.RuneCountInString(s)+2; n++ {
		got := TruncateRunes(s, n)
		if !utf8.ValidString(got) {
			t.Fatalf("TruncateRunes(%q, %d) = %q is not valid UTF-8", s, n, got)
		}
		if !strings.HasPrefix(s, got) {
			t.Fatalf("TruncateRunes(%q, %d) = %q is not a prefix", s, n, got)
		}
	}
}
