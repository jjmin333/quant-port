package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func badgeKeys(bs []model.CommunityBadge) []string {
	out := []string{}
	for _, b := range bs {
		out = append(out, b.Key)
	}
	return out
}

func TestCommunityBadges(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	fresh := &repository.CommunityAuthorStats{JoinedAt: now.Add(-3 * 24 * time.Hour)}
	if got := badgeKeys(CommunityBadges(fresh, nil, now)); len(got) != 1 || got[0] != "newbie" {
		t.Fatalf("가입 2주 안 · 활동 없음 → 새내기: %v", got)
	}

	vet := &repository.CommunityAuthorStats{
		Role: model.RoleAdmin, JoinedAt: now.Add(-400 * 24 * time.Hour),
		Posts: 12, Comments: 60, LikesReceived: 150, Reads: 900, JudgedStamps: 11, Notes: 31,
	}
	got := badgeKeys(CommunityBadges(vet, []string{"005930"}, now))
	want := []string{"admin", "holder", "liked", "writer", "talker", "reader", "forecaster", "journaler"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("순서가 표시 우선순위다: got %v want %v", got, want)
		}
	}

	// 새내기는 다른 활동 뱃지가 있으면 붙지 않는다.
	busyNew := &repository.CommunityAuthorStats{JoinedAt: now.Add(-time.Hour), Posts: 10}
	for _, k := range badgeKeys(CommunityBadges(busyNew, nil, now)) {
		if k == "newbie" {
			t.Fatal("활동 뱃지가 있으면 새내기를 달지 않는다")
		}
	}
}

func TestCommunityBadgesHaveNoRankOrHitRate(t *testing.T) {
	// 방침(2026-08-10 · 09-27): 적중률·순위·보상 뱃지는 없다.
	st := &repository.CommunityAuthorStats{Posts: 999, Comments: 999, LikesReceived: 999, Reads: 999, JudgedStamps: 999, Notes: 999}
	for _, b := range CommunityBadges(st, nil, time.Now()) {
		for _, banned := range []string{"적중", "순위", "랭킹", "1위", "상위", "수익"} {
			if strings.Contains(b.Label, banned) {
				t.Fatalf("뱃지 %q에 성과·순위 말이 들어갔다", b.Label)
			}
		}
	}
}

func TestCommunityAuthorForPostKeepsOnlyThisPostsHolderBadges(t *testing.T) {
	a := model.CommunityAuthor{Badges: []model.CommunityBadge{
		{Key: "holder", Symbol: "005930"}, {Key: "holder", Symbol: "NVDA"}, {Key: "writer"},
	}}
	got := communityAuthorForPost(a, []model.CommunitySymbol{{Symbol: "NVDA"}})
	if len(got.Badges) != 2 || got.Badges[0].Symbol != "NVDA" || got.Badges[1].Key != "writer" {
		t.Fatalf("이 글의 종목만 주주 뱃지: %+v", got.Badges)
	}
}

func TestCommunityBlockedPhrase(t *testing.T) {
	for _, bad := range []string{"리딩방 오세요", "리 딩 방", "무료 추천주 드립니다", "오픈-채팅 링크", "ㅅㅂ 뭐야"} {
		if CommunityBlockedPhrase(bad) == "" {
			t.Errorf("%q는 막혀야 한다", bad)
		}
	}
	for _, ok := range []string{"새끼손가락만큼 올랐네", "삼성전자 3분기 실적 기대", "배당 권리 막차?", "병원 관련주"} {
		if p := CommunityBlockedPhrase(ok); p != "" {
			t.Errorf("%q가 %q로 오탐", ok, p)
		}
	}
}

func TestValidateCommunityText(t *testing.T) {
	if _, _, err := validateCommunityText("a", "본문"); err == nil {
		t.Fatal("제목 1자는 거절")
	}
	title, body, err := validateCommunityText("  월요일   코스피  ", "\r\n본문\r\n")
	if err != nil || title != "월요일 코스피" || body != "본문" {
		t.Fatalf("공백 정리: %q %q %v", title, body, err)
	}
	if _, _, err := validateCommunityText("제목", "리딩방 입장"); !errors.Is(err, ErrCommunityBlockedWords) {
		t.Fatalf("금칙어는 ErrCommunityBlockedWords: %v", err)
	}
}

func TestValidateCommunityImagesOnlyOwnUploads(t *testing.T) {
	if _, err := validateCommunityImages([]model.CommunityImage{{URL: "https://evil.example/p.gif"}}); err == nil {
		t.Fatal("외부 이미지는 거절")
	}
	if _, err := validateCommunityImages([]model.CommunityImage{{URL: "/api/v1/media/../../etc/passwd"}}); err == nil {
		t.Fatal("경로 올라가기 거절")
	}
	five := make([]model.CommunityImage, 5)
	for i := range five {
		five[i] = model.CommunityImage{URL: "/api/v1/media/ab/cd/x.jpg"}
	}
	if _, err := validateCommunityImages(five); err == nil {
		t.Fatal("5장은 거절")
	}
	if _, err := validateCommunityImages(five[:4]); err != nil {
		t.Fatalf("4장은 통과: %v", err)
	}
}

func TestParseCommunityCursor(t *testing.T) {
	if b, o := parseCommunityCursor("b123"); b != 123 || o != 0 {
		t.Fatal("b123")
	}
	if b, o := parseCommunityCursor("o40"); b != 0 || o != 40 {
		t.Fatal("o40")
	}
	for _, bad := range []string{"", "x1", "b-1", "o999999", "b"} {
		if b, o := parseCommunityCursor(bad); b != 0 || o != 0 {
			t.Fatalf("%q는 처음부터", bad)
		}
	}
}

func TestCommunityCooldownLeft(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	if got := communityCooldownLeft(now, now.Add(-20*time.Second), time.Minute); got != 40 {
		t.Fatalf("40초 남음: %d", got)
	}
	if got := communityCooldownLeft(now, now.Add(-2*time.Minute), time.Minute); got != 0 {
		t.Fatalf("지났으면 0: %d", got)
	}
}

func TestCommunityReadable(t *testing.T) {
	if !communityReadable(model.CommunityStatusVisible, 1, 0, false) {
		t.Fatal("공개 글은 비로그인도")
	}
	if communityReadable(model.CommunityStatusHidden, 1, 2, false) {
		t.Fatal("숨긴 글은 남에게 안 보인다")
	}
	if !communityReadable(model.CommunityStatusHidden, 1, 1, false) {
		t.Fatal("숨긴 글도 작성자에게는 보인다(왜 숨겨졌는지 알아야 한다)")
	}
	if communityReadable(model.CommunityStatusDeleted, 1, 1, true) {
		t.Fatal("지운 글은 아무에게도")
	}
}

func TestCommunityExcerpt(t *testing.T) {
	if got := CommunityExcerpt("줄\n\n바꿈   정리", 20); got != "줄 바꿈 정리" {
		t.Fatalf("%q", got)
	}
	if got := CommunityExcerpt("가나다라마바사", 3); got != "가나다…" {
		t.Fatalf("%q", got)
	}
}

func TestNormalizeCommunitySymbols(t *testing.T) {
	got := normalizeCommunitySymbols([]string{" 005930", "nvda", "005930", "", "bad sym", "BRK.B"})
	want := []string{"005930", "NVDA", "BRK.B"}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v", got)
		}
	}
}
