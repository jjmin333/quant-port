import { describe, expect, it } from "vitest";
import { navSectionOf } from "@/lib/nav-sections";

describe("navSectionOf", () => {
  it("묶인 화면은 그 탭, 종목 화면·홈은 null", () => {
    // 「종목 찾기」 묶음은 풀었다(2026-10-12) — 각자 사이드바 칸이라 탭 줄이 없다.
    expect(navSectionOf("/screener")).toBeNull();
    expect(navSectionOf("/analysis")).toBeNull();
    expect(navSectionOf("/changes/2026-10-08")).toBeNull();
    expect(navSectionOf("/stocks/005930")).toBeNull();
    expect(navSectionOf("/news/us")?.tab.label).toBe("해외");
    expect(navSectionOf("/earnings/surprises")?.section.id).toBe("earnings");
    expect(navSectionOf("/")).toBeNull();
  });
});
