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
  /**
   * 좁은 화면에서 입력줄 대신 **돋보기 아이콘**으로 접어 둔다(누르면 펼쳐진다).
   *
   * 검색은 여전히 어느 화면에서나 닿을 수 있어야 한다(아래 HIDDEN_PREFIXES 주석).
   * 다만 이미 한 종목을 열어 놓은 화면에서는 입력줄이 늘 펼쳐져 있을 이유가 약하고,
   * 그 43px이 비싸다 — 종목 상세는 헤더·탭바만으로 375×812의 33%를 쓰고 있었다.
   */
  collapsible?: boolean;
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

  /*
   * 뉴스 검색창의 문구는 **무엇을 넣어야 하는지**가 아니라 무엇을 해 주는지를 적는다.
   *
   * 「국내 뉴스 검색 (종목명, 키워드, 005930…)」은 375px에서 275px을 써서 칸을
   * 거의 가득 채웠다. 예시를 괄호로 나열하는 만큼 길어지는데, 그 예시가 알려 주는
   * 것("종목명도 되고 키워드도 된다")은 한 번 써 보면 바로 아는 종류다.
   */
  if (pathname === "/news/domestic" || pathname.startsWith("/news/domestic/")) {
    return {
      mode: "news-domestic",
      placeholder: "관심 키워드를 똑똑하게 찾아드릴게요",
      debounceMs: 400,
    };
  }

  if (pathname === "/news/us" || pathname.startsWith("/news/us/")) {
    return {
      mode: "news-us",
      placeholder: "관심 키워드를 똑똑하게 찾아드릴게요",
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

  if (pathname === "/stocks" || pathname.startsWith("/stocks/")) {
    return {
      mode: "stock",
      placeholder: "종목명 · 심볼 검색…",
      debounceMs: 200,
      collapsible: true,
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
