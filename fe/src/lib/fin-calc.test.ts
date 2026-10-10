import { describe, expect, it } from "vitest";
import { depositInterest, loanRepayment, monthlyInterestGap, prefillQuery, readCalcPrefill, savingInterest } from "@/lib/fin-calc";

// 기대값은 손으로 푼 식(주석)이다 — 실제 상품 금리가 아니다.
describe("fin-calc", () => {
  it("예금 단리: 1천만원 × 3.5% × 12개월", () => {
    const r = depositInterest(10_000_000, 3.5, 12, "S");
    expect(r.interest).toBe(350_000);
    expect(r.tax).toBe(53_900); // 15.4%
    expect(r.maturity).toBe(10_296_100);
  });

  it("예금 월복리는 매달 원리금에 이자가 붙는 것과 같다", () => {
    let bal = 10_000_000;
    for (let m = 0; m < 12; m++) bal *= 1 + 0.035 / 12;
    expect(depositInterest(10_000_000, 3.5, 12, "M").interest).toBe(Math.round(bal - 10_000_000));
  });

  it("적금 단리: 월 50만원 × 3.5% × 12회 = 50만 × 0.035/12 × 78", () => {
    const r = savingInterest(500_000, 3.5, 12, "S");
    expect(r.principal).toBe(6_000_000);
    expect(r.interest).toBe(113_750);
    expect(r.afterTaxInterest).toBe(113_750 - 17_518);
    expect(savingInterest(500_000, 3.5, 12, "S", "exempt").tax).toBe(0);
  });

  it("적금 월복리는 납입분마다 남은 달만큼 복리", () => {
    let raw = 0;
    for (let k = 1; k <= 24; k++) raw += 300_000 * ((1 + 0.04 / 12) ** k - 1);
    expect(savingInterest(300_000, 4, 24, "M").interest).toBe(Math.round(raw));
  });

  it("원리금균등: 1억 × 5% × 30년 → 월 536,822원", () => {
    const r = loanRepayment(100_000_000, 5, 360, "equal_payment");
    expect(r.firstPayment).toBe(536_822);
    expect(r.rows.at(-1)!.balance).toBe(0);
    expect(Math.abs(r.totalInterest - (536_821.64 * 360 - 1e8))).toBeLessThan(400);
  });

  it("원금균등 총이자 ≈ P·i·(n+1)/2, 만기일시 = P·i·n", () => {
    const ep = loanRepayment(100_000_000, 5, 360, "equal_principal");
    expect(Math.abs(ep.totalInterest - (1e8 * (0.05 / 12) * 361) / 2)).toBeLessThan(400);
    expect(ep.rows.at(-1)!.balance).toBe(0);
    const b = loanRepayment(100_000_000, 5, 360, "bullet");
    expect(b.totalInterest).toBe(416_667 * 360);
    expect(b.lastPayment).toBe(100_000_000 + 416_667);
  });

  it("금리 0%도 나눗셈 없이 끝난다", () => {
    expect(loanRepayment(1_200_000, 0, 12, "equal_payment").firstPayment).toBe(100_000);
    expect(savingInterest(100_000, 0, 12, "M").interest).toBe(0);
  });
});

describe("calc prefill (2026-10-01 요청 5)", () => {
  // 계산기는 없앴지만(2026-10-09) 목록 → 상품 상세가 같은 주소 조각으로 금액·기간을 넘긴다.
  it("round-trips a product row into the detail page", () => {
    const qs = prefillQuery({ amount: 10_000_000, months: 12, rate: 3.123, type: "M" }, "deposit");
    expect(qs).toBe("tab=deposit&amount=10000000&months=12&rate=3.12&type=M");
    const q = new URLSearchParams(qs);
    expect(readCalcPrefill((k) => q.get(k))).toEqual({ amount: 10_000_000, months: 12, rate: 3.12, type: "M", method: undefined });
  });
  it("drops out-of-range or junk values", () => {
    const q = new URLSearchParams("amount=-5&months=abc&rate=99&type=X&method=bullet");
    expect(readCalcPrefill((k) => q.get(k))).toEqual({
      amount: undefined, months: undefined, rate: undefined, type: undefined, method: "bullet",
    });
  });
});

describe("monthlyInterestGap (F3)", () => {
  it("잔액 × 금리 차 ÷ 12, 양수 = 덜 낸다", () => {
    expect(monthlyInterestGap(300_000_000, 4.5, 4.0)).toBe(125_000);
    expect(monthlyInterestGap(100_000_000, 3.6, 4.2)).toBe(-50_000);
  });
});
