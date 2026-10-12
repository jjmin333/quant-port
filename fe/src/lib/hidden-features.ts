/*
노출을 끈 기능 (2026-10-10 사용자 결정 — `docs/BUSINESS_REVIEW.md` §9 W1).

사업화 검토의 결론은 「종목 점수·실적에 집중하고 나머지는 사용자가 생길 때까지 내린다」였다.
**데이터·API·화면 코드는 그대로 두고 들어가는 문만 닫는다** — 되살릴 때는 아래 목록에서
지우기만 하면 메뉴·버튼·사이트맵·주소 차단이 함께 풀린다.

화면 쪽은 경로가 아니라 아래 불리언을 본다. 「목표가 예측」처럼 한 기능이 여러 경로
(`/predictions`·`/stamps`·`/targets`)에 걸쳐 있어서다.
*/

/** 주소로 들어와도 홈으로 돌려보내는 경로(하위 포함). */
export const HIDDEN_ROUTE_PREFIXES = [
  // 부동산 시세·공매, 보유 화면의 부동산·금융 자산 상세
  "/real-estate",
  "/holdings/assets",
  // 금융상품 분석(예·적금·대출·금리 동향·상품 상세)
  "/finance",
  // 커뮤니티·투자 리포트는 2026-10-12 사용자 요청으로 다시 열었다.
  // 목표가 예측 = 스탬프 + 목표가
  "/predictions",
  "/stamps",
  "/targets",
] as const;

export function isHiddenRoute(pathname: string): boolean {
  return HIDDEN_ROUTE_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  );
}

/** 부동산·공매·금융상품(예·적금·대출·금리) — 홈 카드, 헤더 검색, 보유·관심의 자산 칸, 기사 자산 칩. */
export const ASSET_FEATURES_HIDDEN = isHiddenRoute("/real-estate");

/** 커뮤니티·리포트 — 종목 화면 이야기 카드·탭, 글쓰기·발행 버튼. 10-12에 다시 열어 지금은 false. */
export const COMMUNITY_FEATURES_HIDDEN = isHiddenRoute("/community");

/** 목표가 예측(스탬프·목표가) — 종목 화면 버튼, 보유 화면 목표가 카드, 내 기록 줄. */
export const PREDICTION_FEATURES_HIDDEN = isHiddenRoute("/predictions");
