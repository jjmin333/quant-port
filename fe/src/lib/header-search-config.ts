export type HeaderSearchMode =
  | "hidden"
  | "news-domestic"
  | "news-us"
  | "quotes"
  | "stock"
  | "transactions";

export type HeaderSearchConfig = {
  mode: HeaderSearchMode;
  placeholder: string;
  debounceMs: number;
};

const HIDDEN_PREFIXES = [
  "/settings",
  "/onboarding",
  "/journal",
  "/backtest",
  "/targets",
  "/",
  "/risk",
  "/portfolio",
  "/login",
  "/signup",
];

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
      placeholder: "미국 뉴스 검색 (티커, 키워드…)",
      debounceMs: 400,
    };
  }

  if (pathname === "/quotes" || pathname.startsWith("/quotes/")) {
    return {
      mode: "quotes",
      placeholder: "종목 검색…",
      debounceMs: 150,
    };
  }

  if (pathname === "/transactions" || pathname.startsWith("/transactions/")) {
    return {
      mode: "transactions",
      placeholder: "종목 · 주문 검색…",
      debounceMs: 200,
    };
  }

  if (
    pathname === "/analysis" ||
    pathname.startsWith("/analysis/") ||
    pathname === "/watchlist" ||
    pathname === "/holdings" ||
    pathname.startsWith("/stocks/")
  ) {
    return {
      mode: "stock",
      placeholder: "종목명 · 심볼 검색…",
      debounceMs: 200,
    };
  }

  return { mode: "hidden", placeholder: "", debounceMs: 0 };
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
