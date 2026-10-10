package repository

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNumberedDisplayName(t *testing.T) {
	if got := numberedDisplayName("투자왕", 2); got != "투자왕2" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("가", 100)
	got := numberedDisplayName(long, 12)
	if n := utf8.RuneCountInString(got); n != displayNameColumnMax {
		t.Fatalf("칸(100자)을 넘으면 안 된다: %d", n)
	}
	if !strings.HasSuffix(got, "12") {
		t.Fatalf("번호가 끝에 붙어야 한다: %q", got)
	}
}

func TestDuplicateUserErrPicksTheKey(t *testing.T) {
	nick := errors.New("Error 1062 (23000): Duplicate entry '투자왕' for key 'users." + displayNameUniqueIndex + "'")
	if !errors.Is(duplicateUserErr(nick), ErrDisplayNameTaken) {
		t.Fatal("닉네임 인덱스 충돌은 닉네임 중복이다")
	}
	id := errors.New("Error 1062 (23000): Duplicate entry 'abcd' for key 'users.email'")
	if !errors.Is(duplicateUserErr(id), ErrEmailTaken) {
		t.Fatal("아이디 칸 충돌은 아이디 중복이다")
	}
}

func TestClipRunesCountsCharacters(t *testing.T) {
	if got := clipRunes("가나다라", 2); got != "가나" {
		t.Fatalf("got %q", got)
	}
	if got := clipRunes("abc", 10); got != "abc" {
		t.Fatalf("짧으면 그대로: %q", got)
	}
}
