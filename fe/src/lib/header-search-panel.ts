import { supportsStockSuggestions, type HeaderSearchMode } from "@/lib/header-search-config";

/*
헤더 검색 아래에 **무엇이 떠 있어야 하는가**.

두 패널이 같은 자리를 쓴다 — 검색어가 있으면 자동완성이, 없으면 「최근 본 종목」이다.
규칙이 두 파일(`header-search-provider`가 자동완성, `stock-search-field`가 최근 목록)에
나뉘어 있어서 **둘 다 닫혀야 하는 순간**을 아무도 책임지지 않았고, 그 틈이 버그가 됐다:

  종목을 고르면 `clearSearch()`가 입력을 비워 자동완성은 닫히는데, 옵션 버튼이
  `onMouseDown`에서 기본동작을 막아 입력이 **포커스를 유지**한다. 그래서 빈 입력 +
  포커스 = 「최근 본 종목」 조건이 그대로 성립하고, 방금 고른 종목이 맨 위에 얹힌
  목록이 같은 자리에 다시 떴다. 사용자 눈에는 "검색 결과가 안 사라진다"이다
  (모바일에서는 자판까지 그대로 올라와 있다).

그래서 두 규칙을 여기 한 곳에 두고 아래 불변식을 테스트로 못박는다 —
**입력이 비어 있고 포커스도 없으면 어떤 패널도 떠 있지 않다.**
*/

/** 자동완성(검색 결과) 패널이 떠 있어야 하나. 결과 0건이어도 연다 — 「없음」도 답이다. */
export function stockSuggestionsOpen(mode: HeaderSearchMode, inputValue: string): boolean {
  return supportsStockSuggestions(mode) && inputValue.trim().length > 0;
}

export type RecentPanelInput = {
  mode: HeaderSearchMode;
  /** 입력칸이 포커스를 쥐고 있나. 종목을 고르면 화면이 직접 떼어 낸다. */
  focused: boolean;
  inputValue: string;
  recentCount: number;
};

/** 「최근 본 종목」 패널이 떠 있어야 하나. 자동완성과 **동시에** 뜨지 않는다. */
export function recentStocksPanelOpen({
  mode,
  focused,
  inputValue,
  recentCount,
}: RecentPanelInput): boolean {
  if (!supportsStockSuggestions(mode)) return false;
  if (stockSuggestionsOpen(mode, inputValue)) return false;
  return focused && inputValue.trim().length === 0 && recentCount > 0;
}

/**
 * 종목을 고른 뒤 입력칸의 포커스를 떼야 하나.
 *
 * 자동완성이 있는 화면(`stock`·`quotes`)에서 종목을 고르는 것은 **검색의 끝**이다.
 * 반대로 화면 안을 거르는 검색(뉴스 등)에서는 계속 타이핑하는 중이므로 떼지 않는다.
 */
export function shouldBlurAfterPick(mode: HeaderSearchMode): boolean {
  return supportsStockSuggestions(mode);
}
