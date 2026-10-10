import { describe, expect, it } from "vitest";
import {
  parseFundamentalView,
  parseStockWorkspaceTab,
  STOCK_WORKSPACE_TABS,
} from "./stock-workspace-tabs";

// 2026-10-07 요청 10 — 기업 정보·밸류에이션·재무제표 한 탭. 10-09 요청 1 — 차트 탭은 어디서도 없다.
describe("stock workspace tabs", () => {
  it("합쳐진 탭의 옛 주소는 「기업·재무」의 그 보기로 연다", () => {
    expect(parseStockWorkspaceTab("valuation")).toBe("company");
    expect(parseStockWorkspaceTab("financials")).toBe("company");
    expect(parseFundamentalView("valuation")).toBe("valuation");
    expect(parseFundamentalView("financials")).toBe("financials");
    expect(parseFundamentalView("company")).toBe("company");
    expect(parseFundamentalView("news")).toBe("company");
    expect(parseStockWorkspaceTab("bogus")).toBe("overview");
  });

  it("차트 탭은 없고, 옛 ?tab=chart 주소는 개요로 연다", () => {
    expect(STOCK_WORKSPACE_TABS.some((t) => (t.id as string) === "chart")).toBe(false);
    expect(parseStockWorkspaceTab("chart")).toBe("overview");
    expect(parseStockWorkspaceTab("reddit")).toBe("reddit");
  });
});
