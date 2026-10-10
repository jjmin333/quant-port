import { describe, expect, it } from "vitest";
import { formatFinancialAmount, formatFinancialTick } from "@/lib/format";

describe("formatFinancialAmount", () => {
  it("writes the reporting currency code instead of $ (TSM files in TWD)", () => {
    expect(formatFinancialAmount(2_894_307_700_000, "TWD")).toBe("2.9T TWD");
    expect(formatFinancialAmount(-40_318_800_000, "JPY")).toBe("-40.3B JPY");
  });
  it("keeps KRW/USD as before", () => {
    expect(formatFinancialAmount(88_268_000_000, "USD")).toBe("$88.3B");
    expect(formatFinancialAmount(300_000_000_000, "KRW")).toBe("3000억원");
    expect(formatFinancialTick(2_894_307_700_000, "TWD")).toBe("2.9T");
  });
});
