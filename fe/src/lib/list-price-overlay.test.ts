import { describe, expect, it } from "vitest";

import { liveOverlayFor, PRICE_AS_OF_LIVE } from "@/lib/list-price-overlay";
import type { ListPriceEntry, ListPriceMap } from "@/lib/quote-stream-state";

/**
 * 이 규칙은 **AI 추천 랭킹과 종목 스크리너가 함께 쓰는 단 하나**라, 갈리면 같은
 * 종목이 두 화면에서 다른 등락률이 됩니다 (2026-09-23 QA P0-01).
 */

function stream(entries: Record<string, number>): ListPriceMap {
  const map: ListPriceMap = new Map();
  for (const [symbol, price] of Object.entries(entries)) {
    map.set(symbol, { symbol, price } as ListPriceEntry);
  }
  return map;
}

describe("liveOverlayFor", () => {
  it("스트림에 없는 종목은 손대지 않는다", () => {
    expect(liveOverlayFor("005930", 78000, 77000, stream({ "000660": 184400 }))).toBeNull();
  });

  it("가격이 0 이하인 틱은 값이 아니다 — 0원짜리 행을 만들지 않는다", () => {
    expect(liveOverlayFor("005930", 78000, 77000, stream({ "005930": 0 }))).toBeNull();
  });

  it("보고 있는 값과 같은 틱이면 null이다 — 같은 객체를 유지해 다시 그리지 않는다", () => {
    expect(liveOverlayFor("005930", 78000, 77000, stream({ "005930": 78000 }))).toBeNull();
  });

  it("종목 코드의 대소문자는 여기서 맞춘다", () => {
    const got = liveOverlayFor("aapl", 190, 188, stream({ AAPL: 195 }));
    expect(got?.price).toBe(195);
  });

  it("기준가가 있으면 등락률을 다시 낸다", () => {
    const got = liveOverlayFor("005930", 78000, 77000, stream({ "005930": 79000 }));
    expect(got?.price).toBe(79000);
    expect(got?.changeRate).toBeCloseTo(((79000 - 77000) / 77000) * 100, 10);
  });

  it("기준가를 모르면 등락률은 null이다 — 0.00%(보합)로 때우지 않는다", () => {
    const got = liveOverlayFor("005930", 78000, undefined, stream({ "005930": 79000 }));
    expect(got?.price).toBe(79000);
    expect(got?.changeRate).toBeNull();
  });

  /**
   * 두 화면이 같은 입력에 **같은 답**을 내는지가 이 파일이 있는 이유입니다.
   * 필드 이름만 다르고(`closePrice` ↔ `close`) 규칙은 하나여야 합니다.
   */
  it("같은 종목·같은 틱이면 랭킹과 스크리너가 같은 값을 얻는다", () => {
    const ticks = stream({ "073570": 2131 });
    const rankingRow = { symbol: "073570", closePrice: 2095, basePrice: 2060 };
    const screenerRow = { symbol: "073570", close: 2095, basePrice: 2060 };

    const a = liveOverlayFor(
      rankingRow.symbol,
      rankingRow.closePrice,
      rankingRow.basePrice,
      ticks,
    );
    const b = liveOverlayFor(
      screenerRow.symbol,
      screenerRow.close,
      screenerRow.basePrice,
      ticks,
    );

    expect(a).toEqual(b);
    expect(a?.price).toBe(2131);
  });

  it("실시간으로 덮은 행이 말하는 시점은 서버가 쓰는 값과 같은 문자열이다", () => {
    // `model.ScorePriceAsOfLive` · `priceAsOfLabel()`이 이 문자열을 기다린다.
    expect(PRICE_AS_OF_LIVE).toBe("live");
  });
});
