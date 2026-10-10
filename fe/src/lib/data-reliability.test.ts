import { describe, expect, it } from "vitest";
import { fundamentalsProvenance } from "@/lib/data-reliability";

describe("fundamentalsProvenance", () => {
  it("names the supplement when it filled fields (TSM)", () => {
    const p = fundamentalsProvenance({
      dataSource: "edgar",
      fiscalYear: 2024,
      fiscalPeriod: "FY",
      supplement: { source: "finnhub", fields: ["per", "pbr"] },
    });
    expect(p.source).toMatch(/\+ Finnhub$/);
    expect(p.caveat).toBe("보조 값은 최근 12개월");
  });

  it("is unchanged without a supplement", () => {
    const p = fundamentalsProvenance({ dataSource: "dart", fiscalYear: 2025, fiscalPeriod: "FY" });
    expect(p.caveat).toBeUndefined();
    expect(p.source).not.toMatch(/Finnhub/);
  });
});
