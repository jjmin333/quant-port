import { describe, expect, it } from "vitest";
import {
  recentStocksPanelOpen,
  shouldBlurAfterPick,
  stockSuggestionsOpen,
} from "@/lib/header-search-panel";

describe("stockSuggestionsOpen", () => {
  it("검색어가 있으면 연다 — 결과가 0건이어도", () => {
    expect(stockSuggestionsOpen("stock", "삼성")).toBe(true);
    expect(stockSuggestionsOpen("quotes", "005930")).toBe(true);
  });

  it("공백뿐인 입력은 검색어가 아니다", () => {
    expect(stockSuggestionsOpen("stock", "   ")).toBe(false);
  });

  it("종목 자동완성이 없는 화면에서는 열리지 않는다", () => {
    expect(stockSuggestionsOpen("news-domestic", "삼성")).toBe(false);
    expect(stockSuggestionsOpen("hidden", "삼성")).toBe(false);
  });
});

describe("recentStocksPanelOpen", () => {
  const base = { mode: "stock" as const, focused: true, inputValue: "", recentCount: 3 };

  it("빈 입력에 포커스가 있으면 최근 목록을 보여준다", () => {
    expect(recentStocksPanelOpen(base)).toBe(true);
  });

  it("자동완성과 동시에 뜨지 않는다", () => {
    expect(recentStocksPanelOpen({ ...base, inputValue: "삼성" })).toBe(false);
  });

  it("최근 본 종목이 없으면 뜨지 않는다", () => {
    expect(recentStocksPanelOpen({ ...base, recentCount: 0 })).toBe(false);
  });

  it("종목 자동완성이 없는 화면에서는 뜨지 않는다", () => {
    expect(recentStocksPanelOpen({ ...base, mode: "news-us" })).toBe(false);
  });
});

describe("종목을 고른 직후", () => {
  /*
    이 화면의 버그가 정확히 여기였다. `clearSearch()`가 입력만 비우고 포커스를 그대로
    두면 아래 두 번째 기대가 깨진다 — 자동완성 자리에 「최근 본 종목」이 그대로 올라온다.
  */
  it("입력을 비우면 자동완성은 닫힌다", () => {
    expect(stockSuggestionsOpen("quotes", "")).toBe(false);
  });

  it("포커스까지 떼야 최근 목록도 뜨지 않는다", () => {
    const afterClearOnly = {
      mode: "quotes" as const,
      focused: true,
      inputValue: "",
      recentCount: 5,
    };
    expect(recentStocksPanelOpen(afterClearOnly)).toBe(true);
    expect(recentStocksPanelOpen({ ...afterClearOnly, focused: false })).toBe(false);
  });

  it("자동완성이 있는 화면에서만 포커스를 뗀다", () => {
    expect(shouldBlurAfterPick("stock")).toBe(true);
    expect(shouldBlurAfterPick("quotes")).toBe(true);
    expect(shouldBlurAfterPick("news-domestic")).toBe(false);
  });
});
