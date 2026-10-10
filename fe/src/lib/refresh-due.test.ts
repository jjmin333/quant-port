import { describe, expect, it } from "vitest";
import { isRefreshDue } from "@/lib/refresh-due";

const MIN = 60_000;

/*
판정이 **시각**이라는 것을 잠근다. 예전 규칙(「숨어 있는 동안 회차가 울렸는가」)은
얼어붙은 탭에서 영영 false였고, 그래서 몇 시간 뒤 돌아온 사용자가 어제 값을 봤다.
*/
describe("다시 부를 때인가", () => {
  it("한 주기가 지나지 않았으면 아니다", () => {
    expect(isRefreshDue(0, 4 * MIN, 5 * MIN)).toBe(false);
  });

  it("한 주기가 지났으면 맞다", () => {
    expect(isRefreshDue(0, 5 * MIN, 5 * MIN)).toBe(true);
  });

  it("몇 시간을 얼어 있었어도 한 번이면 된다 — 밀린 회차를 세지 않는다", () => {
    // 회차가 한 번도 안 울린 채 6시간이 지난 탭(freeze·절전·가려진 창)이 돌아온 순간.
    expect(isRefreshDue(0, 6 * 60 * MIN, 5 * MIN)).toBe(true);
  });

  it("타이머가 간발의 차로 이르게 울려도 그 회차를 버리지 않는다", () => {
    expect(isRefreshDue(0, 30_000 - 200, 30_000)).toBe(true);
  });

  it("여유보다 더 이르면 아직 아니다", () => {
    expect(isRefreshDue(0, 30_000 - 5_000, 30_000)).toBe(false);
  });

  it("주기가 0 이하면 언제도 아니다", () => {
    expect(isRefreshDue(0, 10 * MIN, 0)).toBe(false);
    expect(isRefreshDue(0, 10 * MIN, -1)).toBe(false);
  });
});
