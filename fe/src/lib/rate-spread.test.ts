import { describe, expect, it } from "vitest";
import { spreadByMonth } from "@/lib/rate-spread";

describe("spreadByMonth", () => {
  it("연-월로 맞춰 차를 낸다", () => {
    const ktb10 = [{ date: "2026-08-29", value: 4.4 }, { date: "2026-07-31", value: 4.2 }, { date: "2026-06-30", value: 4.0 }];
    const ktb3 = [{ date: "2026-08-29", value: 3.9 }, { date: "2026-07-31", value: 3.85 }];
    expect(spreadByMonth(ktb10, ktb3)).toEqual([
      { date: "2026-07-01", value: 0.35 },
      { date: "2026-08-01", value: 0.5 },
    ]);
    const mortgage = [{ date: "2026-08-01", value: 4.66 }];
    const deposit = [{ date: "2026-08-01", value: 3.39 }];
    expect(spreadByMonth(mortgage, deposit)[0].value).toBe(1.27);
  });
});
