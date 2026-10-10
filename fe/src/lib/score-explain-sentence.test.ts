import { describe, expect, it } from "vitest";
import { explainSummarySentence, subjectParticle, type ScoreExplainData } from "./score-explain-api";

const base = {
  score: 69,
  axes: [
    { axis: "technical", label: "추세", score: 99, weight: 30, weightShare: 30, contribution: 29.7, available: true },
    { axis: "valuation", label: "가치", score: 12, weight: 15, weightShare: 15, contribution: 1.8, available: true },
    { axis: "sentiment", label: "심리·수급", score: null, weight: 10, weightShare: 0, available: false },
  ],
  changes: [],
} as unknown as ScoreExplainData;

describe("explainSummarySentence", () => {
  it("names the strongest and the weakest axis", () => {
    expect(explainSummarySentence(base)).toBe(
      "종합 69점 — 추세(99점)가 가장 크게 받쳤고, 가치(12점)가 가장 낮습니다.",
    );
  });

  // 어제와 다른 이유가 축 결측이면 그것이 데이터 몫이라고 말한다.
  it("flags data-driven daily changes", () => {
    const withChange = {
      ...base,
      changes: [
        {
          window: "1d",
          available: true,
          delta: 5,
          drivers: [
            { kind: "availability", label: "축", detail: "가치 축이 빠졌습니다", effect: 4.2 },
            { kind: "axis", axis: "technical", label: "추세", detail: "", effect: 0.8 },
          ],
        },
      ],
    } as unknown as ScoreExplainData;
    const s = explainSummarySentence(withChange)!;
    expect(s).toContain("어제보다 +5점(가치 축 결측 +4.2 · 추세 변화 +0.8)");
    expect(s).toContain("데이터가 바뀐 몫");
  });

  it("says nothing without a score", () => {
    expect(explainSummarySentence({ ...base, score: null } as unknown as ScoreExplainData)).toBeNull();
  });
});

describe("subjectParticle", () => {
  it("picks 이/가 by the last syllable's final consonant", () => {
    expect(subjectParticle("추세")).toBe("가");
    expect(subjectParticle("수익성")).toBe("이");
    expect(subjectParticle("심리·수급")).toBe("이");
    expect(subjectParticle("가치")).toBe("가");
    expect(subjectParticle("EPS")).toBe("가");
  });
});
