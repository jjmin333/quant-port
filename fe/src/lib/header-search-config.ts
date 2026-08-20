export type HeaderSearchMode =
  | "hidden"
  | "news-domestic"
  | "news-us"
  | "quotes"
  | "stock";

export type HeaderSearchConfig = {
  mode: HeaderSearchMode;
  placeholder: string;
  debounceMs: number;
};

/**
 * 검색바를 아예 감추는 경로 — 로그인 전이거나 가입 흐름 중인 화면뿐이다.
 *
 * 예전에는 메모장·목표가·설정 등도 여기 있었는데, 그 화면들에서 종목이 궁금해지면
 * 사이드바로 나갔다가 다시 들어와야 했다. 검색은 화면의 기능이 아니라 앱의 기능이다 —
 * 어느 메뉴에 있든 종목을 찾을 수 있어야 한다.
 */
const HIDDEN_PREFIXES = ["/login", "/signup", "/onboarding"];

export function resolveHeaderSearchConfig(pathname: string): HeaderSearchConfig {
  if (HIDDEN_PREFIXES.some((p) => pathname === p || pathname.startsWith(`${p}/`))) {
    return { mode: "hidden", placeholder: "", debounceMs: 0 };
  }

  if (pathname === "/news/domestic" || pathname.startsWith("/news/domestic/")) {
    return {
      mode: "news-domestic",
      placeholder: "국내 뉴스 검색 (종목명, 키워드, 005930…)",
      debounceMs: 400,
    };
  }

  if (pathname === "/news/us" || pathname.startsWith("/news/us/")) {
    return {
      mode: "news-us",
      placeholder: "해외 뉴스 검색 (티커, 키워드…)",
      debounceMs: 400,
    };
  }

  if (pathname === "/analysis" || pathname.startsWith("/analysis/")) {
    return {
      mode: "quotes",
      placeholder: "종목명 · 심볼 검색…",
      debounceMs: 200,
    };
  }


  if (pathname === "/market" || pathname.startsWith("/market/")) {
    return {
      mode: "quotes",
      placeholder: "종목명 · 심볼 검색…",
      debounceMs: 150,
    };
  }

  // 나머지 전부 — 종목 검색. 고르면 AI 추천 랭킹(`/analysis`)에서 그 종목이 열린다.
  return {
    mode: "stock",
    placeholder: "종목명 · 심볼 검색…",
    debounceMs: 200,
  };
}

/** 종목 자동완성이 뜨는 모드. 나머지는 화면 안을 거르는 필터 입력이다. */
export function supportsStockSuggestions(mode: HeaderSearchMode): boolean {
  return mode === "stock" || mode === "quotes";
}

/** Extract a likely stock symbol from free-text search (header / quotes). */
export function parseStockSearchSymbol(query: string): string | undefined {
  const trimmed = query.trim();
  if (!trimmed) return undefined;

  const domestic = trimmed.match(/\b(\d{6})\b/);
  if (domestic) return domestic[1];

  const upper = trimmed.toUpperCase().replace(/\s+/g, "");
  if (/^[A-Z][A-Z0-9]{0,5}$/.test(upper)) {
    return upper;
  }

  return undefined;
}

/** Extract a domestic 6-digit code or US ticker from a news search query. */
export function parseNewsSearchSymbol(query: string, market: "domestic" | "us"): string | undefined {
  const trimmed = query.trim();
  if (!trimmed) return undefined;

  if (market === "domestic") {
    const m = trimmed.match(/\b(\d{6})\b/);
    return m?.[1];
  }

  const upper = trimmed.toUpperCase();
  if (/^[A-Z]{1,5}$/.test(upper)) {
    return upper;
  }
  const m = trimmed.match(/\b([A-Z]{1,5})\b/i);
  return m?.[1]?.toUpperCase();
}
