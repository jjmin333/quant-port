import { describe, expect, it } from "vitest";
import {
  parseScoreRankingParams,
  scoreRankingCaption,
  scoreRankingParamsKey,
  scoreRankingSearchParams,
  type ScoreRankingParams,
} from "@/lib/score-ranking-config";

const base: ScoreRankingParams = {
  market: "domestic",
  axis: "composite",
  duration: "realtime",
};

describe("parseScoreRankingParams", () => {
  it("빈 주소는 국내 · 종합 · 실시간이다", () => {
    expect(parseScoreRankingParams(new URLSearchParams())).toEqual(base);
  });

  it("주소에 적힌 값을 읽는다", () => {
    const q = new URLSearchParams("market=overseas&axis=growth&duration=1w");
    expect(parseScoreRankingParams(q)).toEqual({
      market: "overseas",
      axis: "growth",
      duration: "1w",
    });
  });

  it("모르는 값은 기본값으로 — 주소창을 손댄 요청이 빈 화면이 되면 안 된다", () => {
    const q = new URLSearchParams("market=mars&axis=vibes&duration=10y");
    expect(parseScoreRankingParams(q)).toEqual(base);
  });
});

describe("scoreRankingSearchParams", () => {
  it("기본값은 주소에 적지 않는다 — 뒤로가기가 빈 칸을 만들지 않게", () => {
    expect(scoreRankingSearchParams(base).toString()).toBe("");
  });

  it("기본이 아닌 값만 싣는다", () => {
    const q = scoreRankingSearchParams({
      market: "overseas",
      axis: "valuation",
      duration: "1mo",
    });
    expect(q.get("market")).toBe("overseas");
    expect(q.get("axis")).toBe("valuation");
    expect(q.get("duration")).toBe("1mo");
  });

  it("왕복해도 같은 값이 나온다", () => {
    const params: ScoreRankingParams = {
      market: "overseas",
      axis: "sentiment",
      duration: "1d",
    };
    expect(parseScoreRankingParams(scoreRankingSearchParams(params))).toEqual(params);
  });
});

describe("scoreRankingParamsKey", () => {
  it("세 손잡이가 하나라도 다르면 다른 키다", () => {
    expect(scoreRankingParamsKey(base)).toBe("domestic|composite|realtime");
    expect(scoreRankingParamsKey({ ...base, axis: "growth" })).not.toBe(
      scoreRankingParamsKey(base),
    );
  });
});

describe("scoreRankingCaption", () => {
  it("실시간은 그 날짜의 점수 순이라고 말한다", () => {
    expect(scoreRankingCaption(base, "2026-09-02")).toBe("2026-09-02 기준 · 종합 점수 순");
  });

  it("기간을 고르면 그 기간의 최고 점수 순이라고 말한다", () => {
    expect(scoreRankingCaption({ ...base, axis: "technical", duration: "1w" }, "2026-09-02")).toBe(
      "2026-09-02 기준 · 최근 1주 추세 최고 점수 순",
    );
  });

  it("날짜가 아직 없어도 문장이 성립한다", () => {
    expect(scoreRankingCaption(base, "")).toBe("최근 배치 기준 · 종합 점수 순");
  });
});
