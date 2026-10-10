import { describe, expect, it } from "vitest";
import { loanMarketStats, marketTopPct, savingsMarketStats, type FinProduct } from "./fin-products-api";

const prod = (id: number, group: string, opts: { save_trm: string; intr_rate: number; intr_rate2: number }[]): FinProduct => ({
  id, kind: "deposit", finCoNo: "", productCode: "", group, companyName: `은행${id}`, productName: `예금${id}`,
  dclsMonth: "202610", base: {}, options: opts, fetchedAt: "",
});

describe("marketTopPct", () => {
  it("ranks higher-is-better and lower-is-better with ties sharing a rank", () => {
    const all = [4.1, 4.0, 4.0, 3.5];
    expect(marketTopPct(4.1, all, true)).toBe(25);
    expect(marketTopPct(4.0, all, true)).toBe(50);
    expect(marketTopPct(3.5, all, false)).toBe(25); // 대출은 낮을수록
    expect(marketTopPct(1, [], true)).toBe(100);
  });
});

describe("market stats", () => {
  it("uses the 12-month option and splits bank / savings bank", () => {
    const s = savingsMarketStats([
      prod(1, "020000", [{ save_trm: "12", intr_rate: 3, intr_rate2: 3.5 }, { save_trm: "24", intr_rate: 9, intr_rate2: 9 }]),
      prod(2, "030300", [{ save_trm: "12", intr_rate: 3.8, intr_rate2: 4.1 }]),
      prod(3, "020000", [{ save_trm: "6", intr_rate: 5, intr_rate2: 5 }]),
    ]);
    expect(s.n).toBe(2);
    expect(s.bankAvgMax).toBe(3.5);
    expect(s.sbAvgMax).toBe(4.1);
    expect(s.top?.rate).toBe(4.1);
  });
  it("loan stats", () => {
    expect(loanMarketStats([4, 6])).toEqual({ n: 2, avg: 5, min: 4 });
    expect(loanMarketStats([])).toEqual({ n: 0, avg: null, min: null });
  });
});
