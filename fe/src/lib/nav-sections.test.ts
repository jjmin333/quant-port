import { describe, expect, it } from "vitest";
import { navSectionOf } from "@/lib/nav-sections";

describe("navSectionOf", () => {
  it("묶인 화면은 그 탭, 종목 화면·홈은 null", () => {
    expect(navSectionOf("/screener")?.tab.label).toBe("조건 검색");
    expect(navSectionOf("/changes/2026-10-08")?.tab.href).toBe("/changes");
    expect(navSectionOf("/stocks")?.tab.href).toBe("/stocks");
    expect(navSectionOf("/stocks/sector/kr/IT")?.tab.href).toBe("/stocks");
    expect(navSectionOf("/stocks/005930")).toBeNull();
    expect(navSectionOf("/earnings/surprises")?.section.id).toBe("earnings");
    expect(navSectionOf("/")).toBeNull();
  });
});
