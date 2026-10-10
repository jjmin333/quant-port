import { describe, expect, it } from "vitest";
import {
  likeDiffs,
  quartileHint,
  REFERENCE_AXIS_SLACK,
  referenceRules,
  type ScreenCatalogField,
} from "@/lib/screener-api";

describe("referenceRules", () => {
  const fields = new Map([
    ["score", { higherIsBetter: true, decimals: 0 }],
    ["axis.technical", { higherIsBetter: true, decimals: 0 }],
    ["per", { higherIsBetter: false, decimals: 2 }],
    ["roe", { higherIsBetter: true, decimals: 1 }],
    ["debtRatio", { higherIsBetter: false, decimals: 1 }],
  ]);

  it("score ≥ reference, axes with slack, ratios as columns only (2026-10-01 결정 B)", () => {
    const rules = referenceRules({ score: 78, "axis.technical": 98.2, per: 101.004, roe: 7.46, debtRatio: 13.41 }, fields);
    expect(rules).toEqual([
      { field: "score", min: 78 },
      { field: "axis.technical", min: 98 - REFERENCE_AXIS_SLACK },
      { field: "per" },
      { field: "roe" },
      { field: "debtRatio" },
    ]);
  });

  it("never asks for a negative axis", () => {
    expect(referenceRules({ "axis.technical": 4 }, fields)).toEqual([{ field: "axis.technical", min: 0 }]);
  });

  it("skips missing values (적자라 PER 없음)", () => {
    expect(referenceRules({ score: 50 }, fields).map((r) => r.field)).toEqual(["score"]);
  });
});

describe("likeDiffs", () => {
  const labels = new Map([
    ["axis.growth", { label: "성장" }],
    ["axis.valuation", { label: "가치" }],
    ["axis.technical", { label: "추세" }],
  ]);
  it("keeps axis gaps ≥ 5, biggest first", () => {
    const cells = [
      { field: "score", value: 90 },
      { field: "axis.growth", value: 71 },
      { field: "axis.valuation", value: 40 },
      { field: "axis.technical", value: 62 },
      { field: "per", value: 10 },
    ];
    const ref = { score: 70, "axis.growth": 63, "axis.valuation": 52, "axis.technical": 60, per: 30 };
    expect(likeDiffs(cells, ref, labels)).toEqual([
      { field: "axis.valuation", label: "가치", diff: -12 },
      { field: "axis.growth", label: "성장", diff: 8 },
    ]);
  });
  it("skips missing values", () => {
    expect(likeDiffs([{ field: "axis.growth", value: null }], { "axis.growth": 10 }, labels)).toEqual([]);
  });
});

describe("quartileHint", () => {
  it("reads as a range", () => {
    const f = { decimals: 2, quartiles: [6.5, 11.8, 27.93] } as ScreenCatalogField;
    expect(quartileHint(f)).toBe("보통 6.5~27.9");
    expect(quartileHint({ decimals: 0, quartiles: [40, 51, 61] } as ScreenCatalogField)).toBe("보통 40~61");
    expect(quartileHint({ decimals: 0 } as ScreenCatalogField)).toBeNull();
    // 해외 시가총액(조) — 0~0으로 뭉개지지 않는다.
    expect(quartileHint({ decimals: 0, quartiles: [0.00031, 0.001, 0.0052] } as ScreenCatalogField)).toBe(
      "보통 0.00031~0.0052",
    );
  });
});
