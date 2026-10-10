import { describe, expect, it } from "vitest";
import type { EarningsTabData } from "@/lib/earnings-types";
import { answerEarnings, answerPeers, answerWhy, type StockQA } from "@/lib/stock-qa";

const qa: StockQA = { symbol: "005930", changes: [], peers: [] };

describe("answerWhy", () => {
  it("says plainly when nothing was detected", () => {
    const a = answerWhy({ ...qa, lastMove: { date: "2026-09-23", changeRate: 1.127 } });
    expect(a.lines[0]).toBe("마지막 거래일(9/23) +1.13%.");
    expect(a.lines[1]).toContain("큰 변화");
  });

  it("flags an unexplained price move", () => {
    const a = answerWhy({
      ...qa,
      changes: [
        { date: "2026-09-22", kind: "price_move", direction: -1, title: "급락 −6.2%", explained: false },
        { date: "2026-09-21", kind: "volume_surge", direction: 0, title: "거래량 5.1배" },
      ],
    });
    expect(a.lines).toEqual([
      "9/22 급락 −6.2%",
      "9/21 거래량 5.1배",
      "9/22 가격 움직임은 같은 날 뉴스·공시에서 뚜렷한 원인을 찾지 못했습니다.",
    ]);
  });
});

describe("answerEarnings", () => {
  it("reads the upcoming date and last surprise with the right particle", () => {
    const tab = {
      symbol: "005930",
      assetType: "domestic",
      calendar: [
        {
          date: "2026-10-14",
          label: "2026 3Q 실적발표",
          source: "fnguide",
          kind: "upcoming",
          dateStatus: "estimated",
          estimateBasis: "작년 3Q는 10/14 발표",
        },
      ],
      announcements: [
        {
          date: "2026-07-07",
          fiscalYear: 2026,
          fiscalPeriod: "2Q",
          surprise: { metric: "opIncome", estimate: 1, actual: 1.05, pct: 5.49 },
        },
      ],
      quarterly: [],
      consensus: { available: false },
    } as unknown as EarningsTabData;
    expect(answerEarnings(tab).lines).toEqual([
      "다음 발표: 10/14 2026 3Q 실적발표(추정) — 작년 3Q는 10/14 발표",
      "지난 발표: 7/7 2026 2Q — 영업이익이 예상을 5.5% 상회",
    ]);
  });
});

describe("answerPeers", () => {
  it("explains why there are no peers", () => {
    expect(answerPeers(qa).lines[0]).toContain("섹터 분류가 아직 없어");
  });
});
