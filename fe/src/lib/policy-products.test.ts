import { describe, expect, it } from "vitest";
import { YOUTH_FUTURE_SAVINGS as P } from "./policy-products";

describe("policy-products", () => {
  it("은행 14곳, 최고금리 ≥ 기본금리, 이름 중복 없음", () => {
    expect(P.banks).toHaveLength(14);
    expect(new Set(P.banks.map((b) => b.name)).size).toBe(14);
    for (const b of P.banks) expect(b.max).toBeGreaterThanOrEqual(b.base);
  });
});
