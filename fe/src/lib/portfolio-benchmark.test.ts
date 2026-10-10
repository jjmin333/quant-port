import { describe, expect, it } from "vitest";
import { alignIndex, portfolioTWR } from "./portfolio-benchmark";
import type { DailyCloseRow } from "@/lib/daily-close-types";

describe("portfolioTWR", () => {
  // 추가 매수는 수익이 아니다 — 입금만 있는 날의 수익률은 0이어야 한다.
  it("strips out new money", () => {
    const s = portfolioTWR([
      { date: "d1", valueKrw: 100, costKrw: 100 },
      { date: "d2", valueKrw: 110, costKrw: 100 }, // +10%
      { date: "d3", valueKrw: 210, costKrw: 200 }, // 100 입금, 가격 변화 없음
    ]);
    expect(s[1]!.pct).toBeCloseTo(10);
    expect(s[2]!.pct).toBeCloseTo(10);
  });
});

describe("alignIndex", () => {
  const row = (tradeDate: string, close: number) => ({ tradeDate, close }) as DailyCloseRow;
  it("normalises to the first date and carries holidays forward", () => {
    const s = alignIndex(["2026-09-01", "2026-09-02", "2026-09-03"], [
      row("2026-09-03", 110),
      row("2026-09-01", 100),
    ])!;
    expect(s.map((p) => Math.round(p.pct))).toEqual([0, 0, 10]);
  });
  it("refuses without a base close", () => {
    expect(alignIndex(["2026-09-01"], [row("2026-09-05", 100)])).toBeNull();
  });
});
