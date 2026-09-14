import { describe, expect, it } from "vitest";
import {
  earningsDayTotal,
  emptyMonthHint,
  emptyMonthMessage,
  isEarningsRangeEmpty,
} from "@/lib/earnings-empty";
import type { EarningsDaySummary } from "@/lib/earnings-types";

function day(over: Partial<EarningsDaySummary> = {}): EarningsDaySummary {
  return {
    date: "2026-09-01",
    confirmed: 0,
    scheduled: 0,
    estimated: 0,
    compared: 0,
    beats: 0,
    ...over,
  };
}

describe("earningsDayTotal", () => {
  it("확정·예정·추정을 합친다", () => {
    expect(earningsDayTotal(day({ confirmed: 2, scheduled: 3, estimated: 4 }))).toBe(9);
  });
});

describe("isEarningsRangeEmpty", () => {
  it("모든 날이 0이면 비었다", () => {
    expect(isEarningsRangeEmpty([day(), day({ date: "2026-09-02" })])).toBe(true);
  });

  it("추정 하나만 있어도 비지 않았다 — 추정도 화면에 그리는 일정이다", () => {
    expect(isEarningsRangeEmpty([day(), day({ estimated: 1 })])).toBe(false);
  });

  it("아직 안 받았으면(null) 비었다고 하지 않는다", () => {
    // 로딩 중에 「없습니다」를 띄우면 잠시 뒤 화면이 자기 말을 뒤집는다.
    expect(isEarningsRangeEmpty(null)).toBe(false);
  });

  it("빈 배열은 비었다 — 그 달에 행이 하나도 오지 않은 경우", () => {
    expect(isEarningsRangeEmpty([])).toBe(true);
  });
});

describe("emptyMonthMessage", () => {
  it("국내는 「9월에는 국내 기업 …」", () => {
    expect(emptyMonthMessage("2026-09-01", "domestic")).toBe(
      "9월에는 국내 기업 실적 발표 일정이 없습니다.",
    );
  });

  it("해외는 「해외 기업」", () => {
    expect(emptyMonthMessage("2026-09-01", "us")).toBe(
      "9월에는 해외 기업 실적 발표 일정이 없습니다.",
    );
  });

  it("한 자리 달에 0을 붙이지 않는다", () => {
    expect(emptyMonthMessage("2026-01-01", "domestic")).toMatch(/^1월에는/);
    expect(emptyMonthMessage("2026-12-01", "domestic")).toMatch(/^12월에는/);
  });
});

describe("emptyMonthHint", () => {
  it("국내에만 쏠림 안내가 붙는다", () => {
    expect(emptyMonthHint("domestic")).toContain("2 · 5 · 8 · 11월");
    expect(emptyMonthHint("us")).toBe("");
  });
});
