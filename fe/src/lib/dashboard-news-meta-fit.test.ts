import { describe, expect, it } from "vitest";
import { __testing, metaPlusCandidates } from "./dashboard-news-meta-fit";

const { fitMetaChipCount } = __testing;

/*
홈 뉴스 줄에서 칩이 「코…」로 뭉개지던 버그의 회귀 시험 (2026-09-05 사용자 지적 2).

폭 숫자는 실측에서 가져왔다 — 375px 폰에서 이 줄의 안쪽 폭이 321px, 종목 칩 하나가
68~96px, 「11분 전」이 45px 남짓이다. 칩 폭을 80으로 잡으면 셋(80·4·80·4·80 = 248)에
시간(6+45)까지 299px로 아슬아슬하게 들어가는데, 「+8」(18px)이 붙는 순간 321을 넘긴다.
그 18px이 예전 계산에서 빠져 있던 값이다.
*/
const CHIP = 80;
const TIME = 45;
const PLUS = new Map([
  [1, 14],
  [2, 14],
  [8, 18],
  [9, 18],
  [10, 20],
  [11, 20],
]);

describe("fitMetaChipCount", () => {
  it("총 개수가 잰 개수와 같으면 +N 없이 셋 다 들어간다", () => {
    // 80*3 + 4*2 + 6 + 45 = 299
    expect(fitMetaChipCount([CHIP, CHIP, CHIP], 3, TIME, PLUS, 321)).toBe(3);
  });

  it("총 개수가 더 많으면 +N 폭까지 세어 하나를 줄인다", () => {
    /*
      이것이 버그가 살던 자리다. 예전 계산은 shown === 잰 개수일 때 숨김을 0으로 보고
      「+8」을 빼먹어 3을 돌려줬고, 화면에서는 그 18px만큼 칩들이 눌려 「코…」가 됐다.
      299 + 4 + 18 = 321 은 경계라 통과하지만, 한 칸이라도 좁으면 둘로 내려간다.
    */
    expect(fitMetaChipCount([CHIP, CHIP, CHIP], 11, TIME, PLUS, 320)).toBe(2);
  });

  it("경계에 정확히 맞으면 줄이지 않는다", () => {
    expect(fitMetaChipCount([CHIP, CHIP, CHIP], 11, TIME, PLUS, 321)).toBe(3);
  });

  it("하나도 안 들어가면 0 — 뭉갠 칩 하나를 남기지 않는다", () => {
    // 폭 100: 칩 하나(80) + 「+10」(4+20) + 시간(6+45) 어느 조합도 못 들어간다.
    expect(fitMetaChipCount([CHIP, CHIP, CHIP], 11, TIME, PLUS, 100)).toBe(0);
  });

  it("칩이 없으면 0", () => {
    expect(fitMetaChipCount([], 0, TIME, PLUS, 321)).toBe(0);
  });

  it("총 개수가 0이면 잰 것이 있어도 0", () => {
    expect(fitMetaChipCount([CHIP], 0, TIME, PLUS, 321)).toBe(0);
  });

  it("넓으면 잰 개수 전부", () => {
    expect(fitMetaChipCount([CHIP, CHIP, CHIP], 30, TIME, PLUS, 2000)).toBe(3);
  });
});

describe("metaPlusCandidates", () => {
  it("0개~상한까지 보여줄 때 각각 남는 개수를 낸다", () => {
    // 열하나 중 0·1·2·3개를 보여주면 숨김은 11·10·9·8.
    expect(metaPlusCandidates(11, 3)).toEqual([11, 10, 9, 8]);
  });

  it("총 개수가 상한 이하면 0이 되는 값은 뺀다", () => {
    expect(metaPlusCandidates(3, 3)).toEqual([3, 2, 1]);
  });

  it("칩이 없으면 후보도 없다", () => {
    expect(metaPlusCandidates(0, 3)).toEqual([]);
  });
});
