import { describe, expect, it } from "vitest";
import { allocation, liabilityBalance, ltv, money, type AssetView, type UserLiability } from "@/lib/wealth-api";

describe("wealth-api helpers", () => {
  it("allocation drops empty classes and sorts by value", () => {
    const rows = allocation({ stock: 100, real_estate: 300, cash: 0, crypto: 100 });
    expect(rows.map((r) => r.cls)).toEqual(["real_estate", "stock", "crypto"]);
    expect(rows[0].pct).toBeCloseTo(60);
  });

  it("money reads 억·만 with sign", () => {
    expect(money(1_630_000_000)).toBe("16.3억");
    expect(money(-450_000_000)).toBe("−4.5억");
    expect(money(12_340_000)).toBe("1,234만");
    expect(money(null)).toBe("-");
  });

  it("ltv uses the user's balance, falls back to scheduled, then principal", () => {
    const asset = { id: 7, myValue: 1_000_000_000 } as AssetView;
    const l = (over: Partial<UserLiability>) => ({ id: 1, kind: "mortgage", name: "", principal: 600_000_000, balance: 0, rateType: "", repaymentType: "", memo: "", assetId: 7, ...over }) as UserLiability;
    expect(liabilityBalance(l({ balance: 500_000_000 }))).toBe(500_000_000);
    expect(liabilityBalance(l({ scheduledBalance: 550_000_000 }))).toBe(550_000_000);
    expect(ltv([l({ balance: 400_000_000 }), l({ assetId: 9, balance: 1 })], asset)).toBeCloseTo(40);
  });
});
