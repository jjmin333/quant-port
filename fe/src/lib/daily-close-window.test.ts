import { describe, expect, it } from "vitest";
import {
  availableBarSteps,
  DAILY_CLOSE_MAX_BARS,
  resolveBarStep,
} from "@/lib/daily-close-window";

describe("availableBarSteps", () => {
  it("이력이 충분하면 20일 · 60일 · 전체 셋", () => {
    expect(availableBarSteps(300)).toEqual([
      { step: 20, label: "20일" },
      { step: 60, label: "60일" },
      { step: DAILY_CLOSE_MAX_BARS, label: "전체" },
    ]);
  });

  it("31행이면 「20일」과 「전체」 — 「60일」이라 적고 31봉을 그리지 않는다", () => {
    // 코스피 실측(2026-08-16): limit=66 을 줘도 31행만 온다.
    expect(availableBarSteps(31)).toEqual([
      { step: 20, label: "20일" },
      { step: 60, label: "전체" },
    ]);
  });

  it("20행 이하면 단계가 하나 — 부르는 쪽이 토글을 그리지 않는다", () => {
    expect(availableBarSteps(20)).toEqual([{ step: 20, label: "20일" }]);
    expect(availableBarSteps(10)).toEqual([{ step: 20, label: "전체" }]);
  });

  it("선 하나도 못 그리는 깊이에서는 아무 단계도 주지 않는다", () => {
    expect(availableBarSteps(1)).toEqual([]);
    expect(availableBarSteps(0)).toEqual([]);
  });

  it("정확히 60행이면 60일이 곧 전체라 둘로 겹치지 않는다", () => {
    expect(availableBarSteps(60)).toEqual([
      { step: 20, label: "20일" },
      { step: 60, label: "60일" },
    ]);
  });
});

describe("resolveBarStep", () => {
  it("고른 단계가 남아 있으면 그대로", () => {
    expect(resolveBarStep(availableBarSteps(300), 60)).toBe(60);
  });

  it("종목을 바꿔 이력이 얕아지면 남은 것 중 가장 긴 것으로 떨어진다", () => {
    // 300행에서 「전체」를 고른 뒤 31행짜리로 옮겨 가면 250 단계가 사라진다.
    expect(resolveBarStep(availableBarSteps(31), DAILY_CLOSE_MAX_BARS)).toBe(60);
  });

  it("단계가 하나도 없으면 최대치 — 자르기가 무의미하므로 전부를 넘긴다", () => {
    expect(resolveBarStep([], 20)).toBe(DAILY_CLOSE_MAX_BARS);
  });
});
