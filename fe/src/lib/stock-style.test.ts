import { describe, expect, it } from "vitest";
import { stockStyle } from "@/lib/stock-style";
import { comparePath, parseComparePair } from "@/lib/stock-seo";

describe("stockStyle", () => {
  it("첫 번째로 맞는 이름표 하나만 붙인다", () => {
    // 우량 성장이면서 저평가 우량도 맞지만 순서가 앞선 쪽.
    expect(stockStyle({ quality: 80, growth: 70, valuation: 70, technical: 50 })?.slug).toBe("quality-growth");
  });

  it("싸지만 수익성·추세가 나쁘면 가치 함정, 수익성만 괜찮으면 역발상", () => {
    expect(stockStyle({ quality: 20, growth: 50, valuation: 80, technical: 10 })?.slug).toBe("value-trap");
    expect(stockStyle({ quality: 50, growth: 50, valuation: 80, technical: 10 })?.slug).toBe("contrarian");
  });

  it("삼성전자(10-08) — 추세 56·가치 19는 이름표 없음", () => {
    expect(stockStyle({ technical: 56, quality: 76, growth: 63, valuation: 19, sentiment: 51 })).toBeNull();
  });

  it("빈 축은 어떤 조건도 만족하지 않는다", () => {
    expect(stockStyle({ technical: 90, valuation: null })?.slug).toBe("momentum");
    expect(stockStyle({ quality: 90, growth: null })).toBeNull();
    expect(stockStyle(undefined)).toBeNull();
  });
});

describe("comparePath", () => {
  it("순서와 무관하게 같은 주소, 되읽으면 같은 쌍", () => {
    expect(comparePath("005930", "000660")).toBe("/compare/000660-vs-005930");
    expect(comparePath("000660", "005930")).toBe("/compare/000660-vs-005930");
    expect(parseComparePair("000660-vs-005930")).toEqual(["000660", "005930"]);
    expect(parseComparePair("BRK.B-vs-AAPL")).toEqual(["BRK.B", "AAPL"]);
    expect(parseComparePair("AAPL-vs-AAPL")).toBeNull();
    expect(parseComparePair("AAPL")).toBeNull();
  });
});
