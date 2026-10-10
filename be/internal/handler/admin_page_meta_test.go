package handler

import (
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
이 자료는 **틀려도 화면에 오류가 안 뜬다.** `/markett`에 저장된 제목은 조용히 아무
화면에도 안 나오고, 관리자는 저장이 됐다는 것만 보고 돌아간다. 그래서 거절하는
경계를 시험이 붙들고 있어야 한다.
*/

func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		"/market":         "/market",
		"market":          "/market",
		"  /market/  ":    "/market",
		"/":               "/",
		"//":              "/",
		"":                "",
		"   ":             "",
		"/news/domestic/": "/news/domestic",
		/*
			대소문자는 **보존한다.** 내리면 FE 목록의 `/reports/authors/[userId]`가
			`[userid]`가 되어 영영 안 맞는다 — 저장은 되고 오류도 안 나는데 제목만
			안 바뀐다. 이 시험이 그 실수를 다시 못 하게 한다.
		*/
		"/reports/authors/[userId]": "/reports/authors/[userId]",
	}
	for in, want := range cases {
		if got := repository.NormalizePath(in); got != want {
			t.Errorf("NormalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecodePageMetaRejects(t *testing.T) {
	cases := []struct {
		name string
		in   pageMetaBody
		msg  string
	}{
		{"경로가 없다", pageMetaBody{Title: "제목"}, "path is required"},
		{"경로에 질의 문자열", pageMetaBody{Path: "/market?tab=1"}, "plain app path"},
		{"경로에 공백", pageMetaBody{Path: "/mar ket"}, "plain app path"},
		{
			"제목이 컬럼보다 길다",
			pageMetaBody{Path: "/market", Title: strings.Repeat("가", 100)},
			"title is too long",
		},
		{
			"설명이 컬럼보다 길다",
			pageMetaBody{Path: "/market", Description: strings.Repeat("가", 200)},
			"description is too long",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, msg := decodePageMeta(c.in)
			if msg == "" {
				t.Fatalf("받아 주면 안 되는 값인데 통과했다: %+v", c.in)
			}
			if !strings.Contains(msg, c.msg) {
				t.Errorf("오류 문구 = %q, %q 를 담아야 한다", msg, c.msg)
			}
		})
	}
}

/*
한글은 utf8mb4 에서 세 바이트다. 글자로 세면 255자까지 통과한 제목이 저장 직전에
MySQL 에서 터진다 — 컬럼 상한과 같은 잣대(바이트)로 재는지 확인한다.
*/
func TestDecodePageMetaCountsBytesNotRunes(t *testing.T) {
	// 85자 × 3바이트 = 255바이트 — 딱 상한이라 통과해야 한다.
	if _, msg := decodePageMeta(pageMetaBody{
		Path: "/market", Title: strings.Repeat("가", 85),
	}); msg != "" {
		t.Fatalf("255바이트는 통과해야 한다: %s", msg)
	}
	// 86자 = 258바이트 — 넘친다.
	if _, msg := decodePageMeta(pageMetaBody{
		Path: "/market", Title: strings.Repeat("가", 86),
	}); msg == "" {
		t.Fatal("258바이트는 거절해야 한다")
	}
}

func TestDecodePageMetaTrimsAndNormalizes(t *testing.T) {
	yes := true
	item, msg := decodePageMeta(pageMetaBody{
		Path:        " /news/domestic/ ",
		Title:       "  국내 주식 뉴스  ",
		Description: " 설명 ",
		NoIndex:     &yes,
	})
	if msg != "" {
		t.Fatalf("통과해야 한다: %s", msg)
	}
	if item.Path != "/news/domestic" {
		t.Errorf("path = %q", item.Path)
	}
	if item.Title != "국내 주식 뉴스" || item.Description != "설명" {
		t.Errorf("공백이 안 다듬어졌다: %+v", item)
	}
	if item.NoIndex == nil || !*item.NoIndex {
		t.Errorf("noindex 가 안 넘어왔다: %+v", item.NoIndex)
	}
}

/*
「아직 안 정했다」와 「색인해도 된다고 정했다」는 다른 상태다. 둘을 합치면 관리자가
한 번 켰다 끈 화면을 코드 기본값으로 되돌릴 방법이 사라진다.
*/
func TestPageMetaIsEmpty(t *testing.T) {
	if !(model.PageMeta{Path: "/market"}).IsEmpty() {
		t.Error("아무것도 안 적은 행은 빈 것이다 — 저장 대신 삭제로 간다")
	}
	no := false
	if (model.PageMeta{Path: "/market", NoIndex: &no}).IsEmpty() {
		t.Error("noindex=false 는 **정한 값**이라 빈 것이 아니다")
	}
	if (model.PageMeta{Path: "/market", OGImage: "/x.png"}).IsEmpty() {
		t.Error("og 이미지만 적어도 덮어쓰기다")
	}
}
