import { describe, expect, it } from "vitest";
import { analystTrend, type AnalystHistoryPoint } from "./analyst-api";

describe("analystTrend", () => {
  it("점이 하나면 변화가 없다(null) — 0으로 적지 않는다", () => {
    expect(analystTrend([{ asOf: "2026-10-01", source: "naver", rating: 4 }])).toBeNull();
  });

  it("국내: 90일 이상 앞선 마지막 점과 의견·목표가를 견준다", () => {
    const h: AnalystHistoryPoint[] = [
      { asOf: "2026-05-01", source: "naver", rating: 3.8, targetMean: 400_000 },
      { asOf: "2026-07-01", source: "naver", rating: 3.9, targetMean: 450_000 },
      { asOf: "2026-08-20", source: "naver", rating: 4.0, targetMean: 480_000 },
      { asOf: "2026-10-06", source: "naver", rating: 4.0, targetMean: 495_000 },
    ];
    const t = analystTrend(h)!;
    expect(t.sinceDate).toBe("2026-07-01");
    expect(t.ratingDelta).toBeCloseTo(0.1);
    expect(t.targetPct).toBeCloseTo(10);
    expect(t.buyShareDelta).toBeUndefined();
  });

  it("해외: 이력이 짧으면 가장 오래된 달, 매수 비중·인원 변화를 센다", () => {
    const h: AnalystHistoryPoint[] = [
      { asOf: "2026-08-01", source: "finnhub", counts: [5, 10, 5, 0, 0] },
      { asOf: "2026-10-01", source: "finnhub", counts: [8, 12, 4, 0, 0] },
    ];
    const t = analystTrend(h)!;
    expect(t.sinceDate).toBe("2026-08-01");
    expect(t.buyShareDelta).toBeCloseTo((20 / 24) * 100 - 75);
    expect(t.analystsDelta).toBe(4);
  });
});
