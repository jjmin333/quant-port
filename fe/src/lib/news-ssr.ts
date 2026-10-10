/**
 * 서버가 뉴스 목록을 미리 받아도 되는 주소인지 가른다.
 *
 * 서버는 **기본 정렬(최신) · 필터 없음**으로 한 벌만 받아 온다. 주소에 정렬이나
 * 검색·키워드가 붙어 있으면 그 목록은 **틀린 목록**이라, 미리 받지 않고 예전처럼
 * 클라이언트가 받게 둔다(로딩 표시도 그대로 뜬다).
 *
 * 크롤러는 언제나 맨 주소로 오므로 이 조건 때문에 크롤러를 놓치는 일은 없다.
 */
export type NewsSearchParams = Record<string, string | string[] | undefined>;

const FILTER_KEYS = ["sort", "q", "keyword", "symbol", "source", "article", "topic"] as const;

export function hasNewsFilterParams(params: NewsSearchParams): boolean {
  return FILTER_KEYS.some((key) => {
    const value = params[key];
    return Array.isArray(value) ? value.length > 0 : Boolean(value);
  });
}
