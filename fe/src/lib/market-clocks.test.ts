import { describe, expect, it } from "vitest";
import { openMarketDefault, type MarketClocks } from "@/lib/market-clocks";

const PAIR = { domestic: "domestic", us: "us" } as const;

function clocks(kr: string | undefined, us: string | undefined): MarketClocks {
  return { domestic: { marketSession: kr }, us: { marketSession: us } };
}

describe("openMarketDefault", () => {
  it("국내만 열렸으면 국내", () => {
    expect(openMarketDefault(clocks("regular", "closed"), PAIR)).toBe("domestic");
  });

  it("해외만 열렸으면 해외", () => {
    expect(openMarketDefault(clocks("closed", "regular"), PAIR)).toBe("us");
  });

  it("둘 다 열렸으면 국내 (사용자 결정)", () => {
    expect(openMarketDefault(clocks("regular", "regular"), PAIR)).toBe("domestic");
  });

  it("둘 다 닫혔으면 국내 (사용자 결정)", () => {
    expect(openMarketDefault(clocks("closed", "closed"), PAIR)).toBe("domestic");
  });

  it("정규장 밖 체결 세션도 '열림'으로 센다", () => {
    // pre/after/day 는 가격이 실제로 움직이는 창이다 — isLiveSession과 같은 선.
    expect(openMarketDefault(clocks("closed", "after"), PAIR)).toBe("us");
    expect(openMarketDefault(clocks("pre", "closed"), PAIR)).toBe("domestic");
  });

  it("세션을 모르면 국내로 떨어진다", () => {
    expect(openMarketDefault(clocks(undefined, undefined), PAIR)).toBe("domestic");
    expect(openMarketDefault(null, PAIR)).toBe("domestic");
  });

  it("모름 + 해외 열림이면 해외 (모름이 국내를 잠그지 않는다)", () => {
    expect(openMarketDefault(clocks(undefined, "regular"), PAIR)).toBe("us");
  });

  it("화면마다 다른 이름을 그대로 돌려준다", () => {
    expect(openMarketDefault(clocks("closed", "regular"), { domestic: "KR", us: "US" })).toBe("US");
    expect(
      openMarketDefault(clocks("closed", "regular"), { domestic: "domestic", us: "overseas" }),
    ).toBe("overseas");
  });
});
