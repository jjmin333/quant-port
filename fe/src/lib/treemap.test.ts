import { describe, expect, it } from "vitest";
import { squarify } from "./treemap";

describe("squarify", () => {
  it("fills the rect exactly, keeps input order and area ratios", () => {
    const values = [6, 6, 4, 3, 2, 2, 1];
    const rect = { x: 0, y: 0, w: 600, h: 400 };
    const out = squarify(values, rect);
    const area = out.reduce((s, r) => s + r.w * r.h, 0);
    expect(area).toBeCloseTo(600 * 400, 3);
    const total = values.reduce((a, b) => a + b, 0);
    out.forEach((r, i) => expect((r.w * r.h) / (600 * 400)).toBeCloseTo(values[i] / total, 6));
    for (const r of out) {
      expect(r.x).toBeGreaterThanOrEqual(-1e-6);
      expect(r.y).toBeGreaterThanOrEqual(-1e-6);
      expect(r.x + r.w).toBeLessThanOrEqual(600 + 1e-6);
      expect(r.y + r.h).toBeLessThanOrEqual(400 + 1e-6);
    }
  });

  it("returns empty rects for zero totals", () => {
    expect(squarify([0, 0], { x: 0, y: 0, w: 10, h: 10 }).every((r) => r.w === 0)).toBe(true);
  });
});
