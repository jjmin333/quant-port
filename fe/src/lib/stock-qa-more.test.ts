import { describe, expect, it } from "vitest";
import {
  answerAnalyst,
  answerFlows,
  answerMine,
  answerRisks,
  answerValuation,
  availableMoreQuestions,
  formatShares,
  type StockQAMore,
} from "@/lib/stock-qa-more";

const empty: StockQAMore = { risks: { warnings: [], riskDisclosures: [] }, disclosures: [] };

describe("availableMoreQuestions — 재료 없는 칩은 숨긴다", () => {
  it("해외 종목(수급·공시 없음)에는 외국인·기관·공시 칩이 없다", () => {
    const ids = availableMoreQuestions(
      { ...empty, estimate: { fiscalYear: 2027, fiscalPeriod: "Q1", now: 1.3, nowDate: "2026-09-26", before: 1.2, beforeDate: "2026-08-27" } },
      true,
    );
    expect(ids).toEqual(["analyst", "risks", "mine"]);
  });
  it("비로그인이면 「내 보유 기준으로」가 없다", () => {
    expect(availableMoreQuestions(empty, false)).toEqual(["risks"]);
  });
  it("재료 응답이 없으면(구버전 서버) 새 칩 없음", () => {
    expect(availableMoreQuestions(undefined, false)).toEqual([]);
  });
});

describe("answers", () => {
  it("추정치 상향 5% 이상은 한 줄 더 말한다", () => {
    const a = answerAnalyst({ fiscalYear: 2027, fiscalPeriod: "Q1", now: 1.32, nowDate: "2026-09-26", before: 1.2, beforeDate: "2026-08-27" });
    expect(a.lines[0]).toContain("+10.0%");
    expect(a.lines[0]).toContain("상향");
    expect(a.lines[1]).toContain("기대가 커지는");
  });

  it("수급: 주식 수 단위와 연속 일수", () => {
    const a = answerFlows({
      asOf: "2026-09-25", foreign5: 1_230_000, foreign20: -41_000, institution5: -5_000, institution20: 250_000_000,
      foreignStreak: 3, institutionStreak: -1, foreignHoldRate: 0.4664, foreignHoldRatePrev: 0.4671, days: 20,
    });
    expect(a.lines[0]).toBe("외국인: 5일 +123.0만 주 · 20일 −4.1만 주 — 3일 연속 순매수.");
    expect(a.lines[1]).toBe("기관: 5일 −5,000주 · 20일 +2.5억 주.");
    expect(a.lines[2]).toBe("외국인 지분율 46.64% (20거래일 전 46.71%).");
  });

  it("PER은 업종 중앙값과, 적자면 비교 불가를 말한다", () => {
    expect(answerValuation({ per: 15, sectorPer: 21.4, pbr: -1, sectorCount: 42 }).lines).toEqual([
      "PER 15.0배 — 같은 업종 중앙값 21.4배보다 30% 낮습니다(싼 편).",
      "PBR 값이 0 이하라(적자·자본잠식) 업종과 비교할 수 없습니다.",
    ]);
  });

  it("위험 신호가 없으면 없다고 답한다 — 실패가 아니라 답이다", () => {
    expect(answerRisks(empty.risks).lines[0]).toContain("없습니다");
    const a = answerRisks({ warnings: ["투자주의"], insider: { buys: 1, sells: 4, days: 90 }, riskDisclosures: [{ date: "2026-09-20", title: "유상증자결정" }] });
    expect(a.lines).toEqual(["시장 경고: 투자주의", "최근 90일 내부자 매도 4건 · 매수 1건 — 매도 우세.", "9/20 공시: 유상증자결정"]);
  });

  it("내 보유: 평단 대비·평가손익·목표가 거리", () => {
    const a = answerMine({ quantity: 10, avgPrice: 200_000, price: 250_000, currency: "KRW", targetPrice: 300_000 });
    expect(a.lines[0]).toBe("평단 200,000원 · 10주 → 지금 250,000원 (+25.0%), 평가손익 +500,000원.");
    expect(a.lines[1]).toBe("목표가 300,000원까지 +20.0%.");
    expect(answerMine({ price: 100 }).lines[0]).toBe("내 포트폴리오에 없는 종목입니다.");
  });

  it("formatShares", () => {
    expect(formatShares(0)).toBe("0주");
    expect(formatShares(12_345)).toBe("+1.2만 주");
  });
});
