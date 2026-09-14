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

/** 종목 선택 시 주소를 push할지 replace할지. */
export function symbolSelectNavMode(
  isCompact: boolean,
  currentSymbolParam: string,
): SymbolSelectNavMode {
  if (!isCompact) return "replace";
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
