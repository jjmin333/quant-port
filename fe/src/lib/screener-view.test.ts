import { describe, expect, it } from "vitest";
import { quickBounds, ruleChipLabel, rulesWithView, type ScreenCatalogField } from "./screener-api";

const per: ScreenCatalogField = {
  id: "per", label: "PER", group: "valuation", unit: "배", higherIsBetter: false, decimals: 1,
  count: 100, inMarket: 100, quartiles: [8.04, 12.4, 21.26],
};
const roe: ScreenCatalogField = { ...per, id: "roe", label: "ROE", unit: "%", higherIsBetter: true };

describe("rulesWithView", () => {
  it("keeps active rules first and adds view columns once", () => {
    const out = rulesWithView([{ field: "per", max: 12 }], "value");
    expect(out.map((r) => r.field)).toEqual(["per", "pbr", "psr", "targetUpside"]);
    expect(out[0]).toEqual({ field: "per", max: 12 });
    expect(out[1]).toEqual({ field: "pbr" }); // 열만 — 거르지 않는다
  });
  it("skips fields the market does not have and caps at 12", () => {
    const allowed = new Set(["roe"]);
    expect(rulesWithView([], "quality", allowed).map((r) => r.field)).toEqual(["roe"]);
    const many = Array.from({ length: 12 }, (_, i) => ({ field: `f${i}`, min: 1 }));
    expect(rulesWithView(many, "value")).toHaveLength(12);
  });
});

describe("chips", () => {
  it("labels value and column-only rules", () => {
    expect(ruleChipLabel({ field: "per", max: 12 }, per)).toBe("PER 12배 이하");
    expect(ruleChipLabel({ field: "per" }, per)).toBe("PER (열만)");
  });
  it("quick bounds tighten toward the better side", () => {
    expect(quickBounds(per)).toEqual([
      { op: "max", value: 21.3 },
      { op: "max", value: 12.4 },
      { op: "max", value: 8 },
    ]);
    expect(quickBounds(roe).map((b) => b.op)).toEqual(["min", "min", "min"]);
    expect(quickBounds({ ...per, quartiles: undefined })).toEqual([]);
  });
});
