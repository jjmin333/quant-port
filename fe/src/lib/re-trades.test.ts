import { describe, expect, it } from "vitest";
import { areaType, groupAreas, movingAverage, supplyPyeong } from "@/lib/re-trades";

describe("re-trades", () => {
  it("84.39·84.6·84.89㎡는 한 평형", () => {
    expect([84.39, 84.6, 84.89, 84.99].map(areaType)).toEqual([84, 84, 84, 84]);
    const g = groupAreas([{ area: 84.39 }, { area: 84.6 }, { area: 84.6 }, { area: 59.97 }]);
    expect(g).toEqual([
      { key: 59, area: 59.97, n: 1 },
      { key: 84, area: 84.6, n: 3 },
    ]);
  });

  it("공급 평 근사", () => {
    expect(supplyPyeong(84.97)).toBe(34);
    expect(supplyPyeong(59.99)).toBe(24);
  });

  it("이동평균은 달마다 직전 90일 평균", () => {
    const d = (s: string) => Date.parse(s);
    const ma = movingAverage([
      { t: d("2026-01-10"), price: 100 },
      { t: d("2026-02-10"), price: 200 },
      { t: d("2026-06-01"), price: 400 },
    ]);
    expect(ma.map((p) => p.price)).toEqual([100, 150, 150, 200, 400]);
    // 4월 15일: 1/10은 90일 밖, 2/10만 → 200 · 5월 15일: 창에 거래 없음 → 빠짐 · 6월: 6/1까지
    expect(new Date(ma[ma.length - 1].t).toISOString().slice(0, 10)).toBe("2026-06-01");
    expect(movingAverage([])).toEqual([]);
  });
});
