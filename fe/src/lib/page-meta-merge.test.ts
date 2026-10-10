import { describe, expect, it } from "vitest";
import { mergePageMeta } from "@/lib/page-meta-merge";
import { PAGE_ROUTES, pageRoute, type PageRoute } from "@/lib/page-meta-routes";
import type { PageMetaOverride } from "@/lib/page-meta-types";
import { SITE_NAME } from "@/lib/seo";

/*
이 규칙이 틀리면 **검색 결과가 조용히 바뀐다.** 화면에는 아무 오류도 안 뜨고, 며칠
뒤에 구글 결과를 보고서야 알게 된다 — 그래서 경계를 시험이 붙들고 있어야 한다.
*/

const ROUTE: PageRoute = {
  path: "/market",
  label: "실시간 차트",
  title: "실시간 차트",
  description: "기본 설명입니다.",
  indexable: true,
};

function override(partial: Partial<PageMetaOverride>): PageMetaOverride {
  return {
    path: "/market",
    title: "",
    description: "",
    ogTitle: "",
    ogDescription: "",
    ogImage: "",
    ...partial,
  };
}

describe("mergePageMeta", () => {
  it("덮어쓰기가 없으면 코드 기본값 그대로다 — 표가 비어도 사이트는 그대로 돈다", () => {
    const out = mergePageMeta(ROUTE, null);
    expect(out.title).toBe("실시간 차트");
    expect(out.description).toBe("기본 설명입니다.");
    expect(out.indexable).toBe(true);
  });

  it("빈 칸은 덮어쓰지 않는다 — 비움이 곧 「기본값을 쓴다」다", () => {
    const out = mergePageMeta(ROUTE, override({ title: "   " }));
    expect(out.title).toBe("실시간 차트");
  });

  it("적힌 칸만 이긴다", () => {
    const out = mergePageMeta(ROUTE, override({ title: "실시간 시세" }));
    expect(out.title).toBe("실시간 시세");
    expect(out.description).toBe("기본 설명입니다.");
  });

  /*
    「아직 안 정했다」(undefined) · 「노출한다」(false) · 「뺀다」(true) 셋을 가른다.
    둘로 합치면 관리자가 한 번 뺐던 화면을 코드 기본값으로 되돌릴 수 없다.
  */
  it("noindex 는 세 값이다", () => {
    expect(mergePageMeta(ROUTE, override({})).indexable).toBe(true);
    expect(mergePageMeta(ROUTE, override({ noindex: true })).indexable).toBe(false);

    const hidden: PageRoute = { ...ROUTE, indexable: false };
    expect(mergePageMeta(hidden, override({})).indexable).toBe(false);
    // 기본이 「뺀다」인 화면을 관리자가 노출로 뒤집는다.
    expect(mergePageMeta(hidden, override({ noindex: false })).indexable).toBe(true);
  });

  it("SNS 카드 제목에는 브랜드가 붙는다 — 탭 제목에는 안 붙는다(템플릿이 붙인다)", () => {
    const out = mergePageMeta(ROUTE, null);
    expect(out.title).toBe("실시간 차트");
    expect(out.cardTitle).toBe(`실시간 차트 · ${SITE_NAME}`);
  });

  it("홈은 제목에 브랜드가 이미 있어 두 번 붙이지 않는다", () => {
    const home = pageRoute("/")!;
    const out = mergePageMeta(home, null);
    expect(out.cardTitle).toBe(out.title);
    expect(out.cardTitle).not.toContain(`· ${SITE_NAME}`);
  });

  it("OG 문구를 따로 적으면 본문 문구와 달라진다", () => {
    const out = mergePageMeta(
      ROUTE,
      override({ title: "실시간 시세", ogTitle: "지금 시장", ogDescription: "카드용 설명" }),
    );
    expect(out.title).toBe("실시간 시세");
    expect(out.cardTitle).toBe("지금 시장");
    expect(out.cardDescription).toBe("카드용 설명");
  });

  it("OG 제목만 비우면 본문 제목 + 브랜드를 따라간다", () => {
    const out = mergePageMeta(ROUTE, override({ title: "실시간 시세" }));
    expect(out.cardTitle).toBe(`실시간 시세 · ${SITE_NAME}`);
    expect(out.cardDescription).toBe("기본 설명입니다.");
  });

  it("OG 이미지를 안 적으면 빈 문자열 — 호출부가 기본 카드 이미지를 쓴다", () => {
    expect(mergePageMeta(ROUTE, null).cardImage).toBe("");
    expect(mergePageMeta(ROUTE, override({ ogImage: " /x.png " })).cardImage).toBe("/x.png");
  });
});

describe("PAGE_ROUTES", () => {
  it("경로가 겹치지 않는다 — 겹치면 관리 화면의 두 줄이 같은 행을 덮어쓴다", () => {
    const paths = PAGE_ROUTES.map((r) => r.path);
    expect(new Set(paths).size).toBe(paths.length);
  });

  it("모든 화면에 제목과 설명이 있다 — 기본값이 비면 덮어쓰기가 없을 때 빈 제목이 나간다", () => {
    for (const route of PAGE_ROUTES) {
      expect(route.title.trim(), route.path).not.toBe("");
      expect(route.description.trim(), route.path).not.toBe("");
    }
  });

  /*
    BE 는 저장 전에 경로를 다듬는다(`repository.NormalizePath`) — 앞 슬래시를 붙이고
    뒤 슬래시를 뗀다. 여기 적힌 경로가 그 결과와 다르면 저장은 되는데 **찾아지지
    않는다**: 오류는 안 나고 제목만 안 바뀐다.
  */
  it("경로가 BE 정규화 결과와 같은 모양이다", () => {
    for (const route of PAGE_ROUTES) {
      expect(route.path.startsWith("/"), route.path).toBe(true);
      expect(route.path === "/" || !route.path.endsWith("/"), route.path).toBe(true);
      expect(route.path.trim(), route.path).toBe(route.path);
    }
  });
});
