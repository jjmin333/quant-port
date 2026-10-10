/**
 * 모바일 실시간 차트(/market)의 목록↔상세 전환 규칙.
 *
 * UI는 `page.tsx`에 있고, 여기는 분기만 모아 둔다 — push/replace·핀 상태·표시
 * 여부를 테스트로 고정해 회귀를 막는다.
 */

export type SymbolSelectNavMode = "push" | "replace";

/** compact에서 상세 패널(오버레이)을 그릴지. */
export function compactShowsDetail(
  isCompact: boolean,
  symbolParam: string,
  listPinned: boolean,
): boolean {
  return isCompact && symbolParam.trim().length > 0 && !listPinned;
}

/**
 * 종목 선택 시 주소를 push할지 replace할지.
 *
 * 데스크톱은 **언제나 push**다 (2026-10-09 사용자 결정) — 목록을 훑으며 종목을 바꿔 보다가 뒤로
 * 가기를 누르면 직전에 보던 종목이 패널에 다시 떠야 한다. 예전 replace는 뒤로 가기 한 번에 이
 * 화면을 떠났다. 모바일은 상세가 화면 전체라 「뒤로 = 목록」이 맞아 그대로 둔다.
 */
export function symbolSelectNavMode(
  isCompact: boolean,
  currentSymbolParam: string,
): SymbolSelectNavMode {
  if (!isCompact) return "push";
  return currentSymbolParam.trim().length > 0 ? "replace" : "push";
}

/** 헤더/물리 뒤로가기가 history.back()을 써야 하는지. */
export function shouldHistoryBackOnCompactBack(symbolParam: string): boolean {
  return symbolParam.trim().length > 0;
}

/** 시장·정렬·기간 필터가 바뀌면 목록을 다시 보여 준다(deep link는 첫 마운트 제외). */
export function shouldPinListOnParamsChange(
  prevParamsKey: string,
  nextParamsKey: string,
  isCompact: boolean,
): boolean {
  return isCompact && prevParamsKey !== nextParamsKey;
}
