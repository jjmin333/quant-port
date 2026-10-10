import { describe, expect, it } from "vitest";
import { applyWhatIf, computePortfolioRisk, sectorShares, type Close } from "@/lib/portfolio-risk";

function day(i: number): string {
  const d = new Date(Date.UTC(2026, 0, 1 + i));
  return d.toISOString().slice(0, 10);
}

/** 수익률 배열 → 종가. */
function closesFrom(returns: number[], start = 100): Close[] {
  const out: Close[] = [{ d: day(0), c: start }];
  let c = start;
  returns.forEach((r, i) => {
    c *= 1 + r;
    out.push({ d: day(i + 1), c });
  });
  return out;
}

const wave = (n: number, amp: number, phase = 0) =>
  Array.from({ length: n }, (_, i) => amp * Math.sin(i * 0.9 + phase));

describe("computePortfolioRisk", () => {
  it("measures volatility, drawdown and beta against the index", () => {
    const idx = wave(60, 0.01);
    const res = computePortfolioRisk(
      [{ symbol: "A", name: "A", weight: 100 }],
      { A: closesFrom(idx.map((r) => 2 * r)) },
      closesFrom(idx),
    );
    expect(res).not.toBeNull();
    expect(res!.beta).toBeCloseTo(2, 5);
    expect(res!.volatilityPct).toBeGreaterThan(0);
    expect(res!.maxDrawdownPct).toBeLessThan(0);
  });

  it("flags pairs that move together, not ones that move apart", () => {
    const base = wave(60, 0.01);
    const res = computePortfolioRisk(
      [
        { symbol: "A", name: "가", weight: 40 },
        { symbol: "B", name: "나", weight: 40 },
        { symbol: "C", name: "다", weight: 20 },
      ],
      {
        A: closesFrom(base),
        B: closesFrom(base.map((r) => r * 1.1)),
        C: closesFrom(base.map((r) => -r)),
      },
      null,
    );
    expect(res!.pairs).toHaveLength(1);
    expect(res!.pairs[0]).toMatchObject({ a: "A", b: "B" });
    expect(res!.pairs[0]!.weight).toBeCloseTo(80, 5);
    expect(res!.beta).toBeNull();
  });

  it("skips holdings without enough closes instead of treating them as flat", () => {
    const res = computePortfolioRisk(
      [
        { symbol: "A", name: "A", weight: 50 },
        { symbol: "NEW", name: "신규", weight: 50 },
      ],
      { A: closesFrom(wave(60, 0.01)), NEW: closesFrom(wave(5, 0.01)) },
      null,
    );
    expect(res!.skipped).toEqual(["NEW"]);
  });

  it("returns null when nothing has enough history", () => {
    expect(
      computePortfolioRisk([{ symbol: "A", name: "A", weight: 1 }], { A: closesFrom([0.01]) }, null),
    ).toBeNull();
  });
});


describe("applyWhatIf", () => {
  const base = [
    { symbol: "A", name: "가", weight: 600, sector: "정보기술" },
    { symbol: "B", name: "나", weight: 400, sector: "금융" },
  ];

  it("adds to an existing holding and to a new one", () => {
    expect(applyWhatIf(base, { symbol: "A", name: "가", deltaKRW: 400 })[0]!.weight).toBe(1000);
    const added = applyWhatIf(base, { symbol: "C", name: "다", deltaKRW: 500, sector: "헬스케어" });
    expect(added.map((h) => h.symbol)).toEqual(["A", "B", "C"]);
  });

  it("drops a holding sold down to zero and never goes short", () => {
    expect(applyWhatIf(base, { symbol: "B", name: "나", deltaKRW: -999 }).map((h) => h.symbol)).toEqual([
      "A",
    ]);
    expect(applyWhatIf(base, { symbol: "Z", name: "없음", deltaKRW: -100 })).toHaveLength(2);
  });

  it("reads sector shares before and after", () => {
    expect(sectorShares(base)).toEqual([
      { sector: "정보기술", pct: 60 },
      { sector: "금융", pct: 40 },
    ]);
    const after = sectorShares(applyWhatIf(base, { symbol: "C", name: "다", deltaKRW: 1000 }));
    expect(after[0]).toEqual({ sector: "미분류", pct: 50 });
  });
});
