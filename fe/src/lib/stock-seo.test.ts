import { describe, expect, it } from "vitest";
import {
  STOCK_DESCRIPTION_MAX,
  stockPageTitle,
  stockSummarySentence,
  type StockSeoSummary,
  groupBySector,
  sectorPeers,
} from "@/lib/stock-seo";

const base: StockSeoSummary = {
  symbol: "005930",
  name: "삼성전자",
  market: "KOSPI",
  assetType: "domestic",
  sectorLabel: "정보기술",
  tradeDate: "2026-09-23",
  score: 69,
  grade: "B",
  axes: { technical: 99, quality: 70, growth: 60, valuation: 12, sentiment: null },
  close: 286500,
  changeRate: 1.127,
  currency: "KRW",
  recentChange: { date: "2026-09-22", title: "거래량 급증" },
  indexable: true,
};

describe("stockSummarySentence", () => {
  it("joins only the facts it has", () => {
    expect(stockSummarySentence(base)).toBe(
      "삼성전자(005930) · 코스피 · 정보기술 — 9/23 기준 AI 종합 69점(B) · 추세 99점이 가장 높고 " +
        "가치 12점이 가장 낮음 · 종가 286,500원 (+1.13%) · 최근 변화: 9/22 거래량 급증",
    );
  });

  it("formats US closes in dollars and drops missing parts", () => {
    const s = stockSummarySentence({
      symbol: "AAPL",
      name: "애플",
      market: "NASDAQ",
      assetType: "us",
      close: 339.74,
      changeRate: -0.5,
      currency: "USD",
      indexable: false,
    });
    expect(s).toBe("애플(AAPL) · 미국 — 종가 $339.74 (-0.50%)");
  });

  it("shortens only the news title to stay within the description limit", () => {
    const long = "가".repeat(200);
    const s = stockSummarySentence({ ...base, recentChange: { date: "2026-09-22", title: long } });
    expect([...s].length).toBeLessThanOrEqual(STOCK_DESCRIPTION_MAX);
    expect(s).toContain("종가 286,500원 (+1.13%) · 최근 변화: 9/22 가가");
    expect(s.endsWith("…")).toBe(true);
  });

  it("keeps page titles within 45 characters including the brand", () => {
    expect(stockPageTitle({ name: "애플", symbol: "AAPL" })).toBe("애플 (AAPL) 주가·AI 점수");
    const t = stockPageTitle({ name: "TIGER 미국배당다우존스타겟데일리커버드콜", symbol: "0008S0" });
    expect([...`${t} · Quant Port`].length).toBeLessThanOrEqual(45);
    expect(t.startsWith("TIGER 미국배당다우존스타겟데일리커버드콜 (0008S0)")).toBe(true);
  });

  it("dates the close when it is not the score's day", () => {
    // 추석 휴장: 점수는 9/24 배치, 국내 종가는 9/23.
    expect(stockSummarySentence({ ...base, recentChange: undefined, closeDate: "2026-09-22" })).toBe(
      "삼성전자(005930) · 코스피 · 정보기술 — 9/23 기준 AI 종합 69점(B) · 추세 99점이 가장 높고 " +
        "가치 12점이 가장 낮음 · 종가 286,500원 (+1.13%, 9/22)",
    );
    expect(stockSummarySentence({ ...base, recentChange: undefined, closeDate: "2026-09-23" })).toContain(
      "종가 286,500원 (+1.13%)",
    );
  });
});

describe("groupBySector / sectorPeers", () => {
  const rows = [
    { symbol: "A1", assetType: "domestic", sectorLabel: "IT" },
    { symbol: "A2", assetType: "domestic", sectorLabel: "금융" },
    { symbol: "A3", assetType: "domestic", sectorLabel: "IT" },
    { symbol: "A4", assetType: "domestic" },
    { symbol: "A5", assetType: "domestic", sectorLabel: "IT" },
    { symbol: "U1", assetType: "us", sectorLabel: "IT" },
  ];

  it("groups one market, biggest sector first, 기타 last, rank order kept", () => {
    const g = groupBySector(rows, "domestic");
    expect(g.map((x) => x.sector)).toEqual(["IT", "금융", "기타"]);
    expect(g[0].stocks.map((s) => s.symbol)).toEqual(["A1", "A3", "A5"]);
  });

  it("peers stay in the same market and sector, nearest rank first, never self", () => {
    expect(sectorPeers(rows, rows[2], 8).map((s) => s.symbol)).toEqual(["A1", "A5"]);
    expect(sectorPeers(rows, rows[0], 1).map((s) => s.symbol)).toEqual(["A3"]);
    expect(sectorPeers(rows, { symbol: "X", assetType: "domestic" })).toEqual([]);
  });
});
