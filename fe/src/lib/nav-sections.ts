/**
 * 사이드바 한 칸 = 여러 화면 (2026-10-10 사용자 결정 A — 메뉴 12개 → 7개).
 *
 * 스크리너·오늘의 변화·종목 모음·AI 점수 순위가 「종목을 고르는 목록」이라는 같은 일을 해서 한 메뉴
 * 「종목 찾기」로 묶었다. 뉴스·실적도 국내/해외를 한 칸으로. **주소는 그대로다** — 각 화면이 따로
 * 색인돼 있어서다. 묶인 화면들 위에는 `SectionTabs`가 같은 탭 줄을 그린다.
 */
export type NavTab = {
  href: string;
  label: string;
  /** 이 탭에 속하는 하위 주소 접두어. 없으면 `href`와 그 아래 전부. */
  match?: string[];
};

export type NavSection = { id: string; label: string; tabs: NavTab[] };

export const NAV_SECTIONS: NavSection[] = [
  {
    id: "find",
    label: "종목 찾기",
    tabs: [
      { href: "/analysis", label: "AI 점수 순위" },
      { href: "/screener", label: "조건 검색" },
      // `/stocks/<종목>`은 종목 화면이라 탭을 그리지 않는다 — 업종·스타일 목록만 이 탭이다.
      { href: "/stocks", label: "업종·스타일", match: ["/stocks/sector/", "/stocks/style/"] },
      { href: "/changes", label: "오늘의 변화" },
    ],
  },
  {
    id: "news",
    label: "뉴스",
    tabs: [
      { href: "/news/domestic", label: "국내" },
      { href: "/news/us", label: "해외" },
    ],
  },
  {
    id: "earnings",
    label: "실적",
    tabs: [
      { href: "/earnings/domestic", label: "국내 일정" },
      { href: "/earnings/us", label: "해외 일정" },
      { href: "/earnings/surprises", label: "서프라이즈" },
    ],
  },
];

function tabMatches(tab: NavTab, pathname: string): boolean {
  if (pathname === tab.href) return true;
  if (tab.match) return tab.match.some((p) => pathname.startsWith(p));
  return pathname.startsWith(`${tab.href}/`);
}

/** 지금 주소가 속한 묶음과 탭. 어디에도 없으면 null. */
export function navSectionOf(pathname: string): { section: NavSection; tab: NavTab } | null {
  for (const section of NAV_SECTIONS) {
    const tab = section.tabs.find((t) => tabMatches(t, pathname));
    if (tab) return { section, tab };
  }
  return null;
}
