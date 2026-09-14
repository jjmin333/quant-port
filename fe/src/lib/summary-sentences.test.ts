import { describe, expect, it } from "vitest";
import { firstSentences, hasMoreThan } from "@/lib/summary-sentences";

describe("firstSentences", () => {
  /*
    사용자가 보낸 화면 그대로다 (2026-09-14 요청 2). `line-clamp-3`이 「…효과를
    상쇄하」에서 끊었다 — 낱말 가운데. 문장 단위로 자르면 그 자리가 안 생긴다.
  */
  it("사용자가 지목한 문장에서 낱말을 쪼개지 않는다", () => {
    const full =
      "PER 135.4배와 PBR 10.1배로 동종 업계 대비 상위 98% 수준의 고평가 구간에 위치해 있습니다. " +
      "매출과 영업이익이 전 분기 대비 각각 7.7%, 57% 증가하며 실적 개선 흐름은 뚜렷합니다. " +
      "다만, 높은 밸류에이션 부담이 실적 개선의 효과를 상쇄하고 있습니다.";

    const brief = firstSentences(full, 2);
    expect(brief.endsWith("뚜렷합니다.")).toBe(true);
    expect(brief).not.toContain("상쇄하");
    expect(hasMoreThan(brief, full)).toBe(true);
  });

  /*
    숫자로 가득한 글이라 소수점을 문장 끝으로 보면 첫 문장이 「PER 135.」가 된다.
    마침표 **뒤에 공백이 오는지**를 함께 봐야 갈린다.
  */
  it("소수점을 문장 끝으로 착각하지 않는다", () => {
    expect(firstSentences("PER 135.4배로 고평가입니다. 그러나 성장률이 높습니다.", 1)).toBe(
      "PER 135.4배로 고평가입니다.",
    );
  });

  it("문장이 요청한 수보다 적으면 있는 만큼 돌려준다", () => {
    const one = "한 문장뿐입니다.";
    expect(firstSentences(one, 3)).toBe(one);
    expect(hasMoreThan(firstSentences(one, 3), one)).toBe(false);
  });

  it("마침표가 없어도 원문을 잃지 않는다", () => {
    // 요약을 못 만들었다고 빈 화면을 주면 읽는 사람에게 아무 도움이 안 된다.
    expect(firstSentences("마침표가 없는 한 줄", 2)).toBe("마침표가 없는 한 줄");
  });

  it("빈 값과 공백은 빈 문자열이다", () => {
    expect(firstSentences("", 2)).toBe("");
    expect(firstSentences("   \n ", 2)).toBe("");
    expect(firstSentences(null, 2)).toBe("");
    expect(firstSentences(undefined, 2)).toBe("");
  });

  it("줄바꿈도 문장 구분으로 인정한다", () => {
    // 일별 해설은 문단이 나뉘어 오기도 한다 — `\n`은 공백이다.
    expect(firstSentences("첫 문장입니다.\n둘째 문장입니다.", 1)).toBe("첫 문장입니다.");
  });

  it("글 끝의 마침표 하나로 원문이 잘리지 않는다", () => {
    const two = "첫 문장입니다. 둘째 문장입니다.";
    expect(firstSentences(two, 2)).toBe(two);
    expect(firstSentences(two, 5)).toBe(two);
  });
});
