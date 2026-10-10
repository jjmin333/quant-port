import { describe, expect, it } from "vitest";
import { rankLoans, rankSavings, shortCompanyName, type FinProduct, type LoanQuery, type SavingsQuery } from "@/lib/fin-products-api";

// 전부 시험용 가짜 상품이다 — 실제 금리가 아니다.
function product(id: number, kind: FinProduct["kind"], base: Record<string, unknown>, options: Record<string, unknown>[]): FinProduct {
  return {
    id, kind, finCoNo: `T${id}`, productCode: `P${id}`, group: "020000", companyName: `은행${id}`,
    productName: `상품${id}`, dclsMonth: "202609", base, options, fetchedAt: "2026-09-29T00:00:00Z",
  };
}

const sq: SavingsQuery = {
  kind: "deposit", amount: 10_000_000, months: 12, tax: "normal", basis: "base", interestType: "all",
  rsrvType: "all", group: "all", openOnly: false, watchedOnly: false, watched: new Set(),
};

describe("rankSavings", () => {
  const items = [
    // 최고금리는 높지만 기본금리가 낮다 — 우대 없이는 3위.
    product(1, "deposit", { join_deny: 1 }, [{ save_trm: "12", intr_rate_type: "S", intr_rate: 2.0, intr_rate2: 5.0 }]),
    product(2, "deposit", { join_deny: 1 }, [{ save_trm: "12", intr_rate_type: "S", intr_rate: 3.2, intr_rate2: 3.3 }]),
    // 같은 금리의 월복리 — 단리보다 이자가 많다.
    product(3, "deposit", { join_deny: 3 }, [{ save_trm: "12", intr_rate_type: "M", intr_rate: 3.2, intr_rate2: null }]),
    product(4, "deposit", { max_limit: 5_000_000 }, [{ save_trm: "12", intr_rate_type: "S", intr_rate: 9, intr_rate2: 9 }]),
    product(5, "deposit", {}, [{ save_trm: "6", intr_rate_type: "S", intr_rate: 4, intr_rate2: 4 }]),
  ];

  it("기본은 세후 이자(기본금리) — 최고금리 숫자가 아니다", () => {
    const r = rankSavings(items, sq);
    expect(r.rows.map((x) => x.product.id)).toEqual([3, 2, 1]);
    expect(r.overLimit).toBe(1);
    expect(r.noTerm).toBe(1);
    expect(r.rows[1].maxRate).toBe(3.3);
    expect(r.rows[0].maxRate).toBe(3.2); // 최고금리 비어 있으면 기본금리
  });

  it("우대 전부 충족 기준으로 바꾸면 순서가 바뀐다", () => {
    expect(rankSavings(items, { ...sq, basis: "max" }).rows[0].product.id).toBe(1);
  });

  it("내 조건(F1) — 고른 우대조건 가산만 더해 순위를 매긴다", () => {
    const withAI = [
      { ...product(8, "deposit", {}, [{ save_trm: "12", intr_rate_type: "S", intr_rate: 3.0, intr_rate2: 3.6 }]), ai: { conds: [{ l: "급여이체", b: 0.5 }, { l: "비대면 가입", b: 0.1 }] } },
      product(9, "deposit", {}, [{ save_trm: "12", intr_rate_type: "S", intr_rate: 3.3, intr_rate2: 3.3 }]),
    ];
    const none = rankSavings(withAI, { ...sq, basis: "mine", myConds: new Set() });
    expect(none.rows.map((x) => x.product.id)).toEqual([9, 8]);
    const salary = rankSavings(withAI, { ...sq, basis: "mine", myConds: new Set(["salary"]) });
    expect(salary.rows.map((x) => x.product.id)).toEqual([8, 9]);
    expect(salary.rows[0].myRate).toBe(3.5);
  });

  it("가입 방법·대상(F2)", () => {
    const list = [
      product(10, "deposit", { join_way: "영업점", join_member: "제한없음" }, [{ save_trm: "12", intr_rate_type: "S", intr_rate: 3, intr_rate2: 3 }]),
      product(11, "deposit", { join_way: "스마트폰", join_member: "만 19세~34세 청년" }, [{ save_trm: "12", intr_rate_type: "S", intr_rate: 3, intr_rate2: 3 }]),
    ];
    expect(rankSavings(list, { ...sq, way: "online" }).rows.map((x) => x.product.id)).toEqual([11]);
    expect(rankSavings(list, { ...sq, target: "youth" }).rows.map((x) => x.product.id)).toEqual([11]);
    expect(rankSavings(list, { ...sq, way: "all", target: "all" }).rows.length).toBe(2);
  });

  it("가입 제한 없는 상품만", () => {
    expect(rankSavings(items, { ...sq, openOnly: true }).rows.map((x) => x.product.id)).toEqual([2, 1]);
  });
});

const lq: LoanQuery = {
  kind: "mortgage", amount: 300_000_000, months: 360, method: "equal_payment", basis: "avg", rpayType: "D",
  rateType: "all", mrtgType: "all", creditBand: "crdt_grad_4", creditType: "all", group: "all",
  watchedOnly: false, watched: new Set(),
};

describe("rankLoans", () => {
  const items = [
    product(1, "mortgage", {}, [{ rpay_type: "D", lend_rate_type: "C", mrtg_type: "A", lend_rate_min: 3.0, lend_rate_max: 6, lend_rate_avg: 4.8 }]),
    product(2, "mortgage", {}, [{ rpay_type: "D", lend_rate_type: "F", mrtg_type: "A", lend_rate_min: 3.9, lend_rate_max: 5, lend_rate_avg: 4.1 }]),
    product(3, "mortgage", {}, [{ rpay_type: "D", lend_rate_type: "F", mrtg_type: "A", lend_rate_min: 3.5, lend_rate_max: 5, lend_rate_avg: null }]),
    product(4, "mortgage", {}, [{ rpay_type: "S", lend_rate_type: "F", mrtg_type: "A", lend_rate_min: 1, lend_rate_max: 2, lend_rate_avg: 1.5 }]),
  ];

  it("실제 취급 평균금리 순위와 광고(최저) 금리 순위가 다르다", () => {
    const avg = rankLoans(items, lq);
    expect(avg.rows.map((x) => x.product.id)).toEqual([2, 1]);
    expect(avg.noRate).toBe(2); // 평균 없음 1 + 분할상환 옵션 없음 1
    expect(rankLoans(items, { ...lq, basis: "min" }).rows.map((x) => x.product.id)).toEqual([1, 3, 2]);
  });

  it("만기일시 상품은 bullet로 계산", () => {
    const r = rankLoans(items, { ...lq, rpayType: "S" });
    expect(r.rows[0].method).toBe("bullet");
    expect(r.rows[0].repay.firstPayment).toBe(Math.round(300_000_000 * 0.015 / 12));
  });

  it("신용대출은 대출금리(A) 행의 내 점수 구간 금리", () => {
    const credit = [
      product(7, "credit", { crdt_prdt_type: "1" }, [
        { crdt_lend_rate_type: "B", crdt_grad_4: 3.0, crdt_grad_avg: 3.0 },
        { crdt_lend_rate_type: "A", crdt_grad_4: 5.5, crdt_grad_avg: 7.1 },
      ]),
    ];
    const r = rankLoans(credit, { ...lq, kind: "credit", months: 12 });
    expect(r.rows[0].rate).toBe(5.5);
    expect(rankLoans(credit, { ...lq, kind: "credit", months: 12, creditBand: "crdt_grad_13" }).noRate).toBe(1);
  });
});

describe("shortCompanyName (2026-10-01 실데이터)", () => {
  it("drops corporate markers in any position", () => {
    expect(shortCompanyName("주식회사 케이뱅크")).toBe("케이뱅크");
    expect(shortCompanyName("농협은행주식회사")).toBe("농협은행");
    expect(shortCompanyName("토스뱅크 주식회사")).toBe("토스뱅크");
    expect(shortCompanyName("(주)우리은행")).toBe("우리은행");
    expect(shortCompanyName("우리저축은행")).toBe("우리저축은행");
  });
});
