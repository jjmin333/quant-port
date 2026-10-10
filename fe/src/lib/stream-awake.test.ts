import { describe, expect, it } from "vitest";
import { STREAM_BACKGROUND_GRACE_MS, shouldKeepStream } from "./stream-awake";

const NOW = Date.UTC(2026, 8, 5, 6, 0, 0);
const MINUTE = 60 * 1000;
/*
  30분으로 잡았다가 사용자가 **1분으로 내렸다.** 30분은 요청이 겨냥한 상황을 하나도
  못 덮는 값이었다 — 탭을 오가며 쓰는 동안에는 어떤 탭도 30분 연속으로 숨어 있지
  않아서, 열 탭이 전부 붙어 있는 것과 같았다.
*/
const GRACE = 1 * MINUTE;

describe("shouldKeepStream", () => {
  it("보이는 중이면 언제나 붙어 있는다", () => {
    expect(shouldKeepStream(null, NOW)).toBe(true);
  });

  it("막 숨었으면 붙어 있는다", () => {
    expect(shouldKeepStream(NOW, NOW)).toBe(true);
  });

  it("숨은 지 1분 미만이면 붙어 있는다 — 알트탭 한 번은 여기 있다", () => {
    expect(shouldKeepStream(NOW - 30_000, NOW)).toBe(true);
    expect(shouldKeepStream(NOW - (GRACE - 1), NOW)).toBe(true);
  });

  it("정확히 1분이 되면 놓는다", () => {
    expect(shouldKeepStream(NOW - GRACE, NOW)).toBe(false);
  });

  it("1분을 넘겼으면 놓는다", () => {
    expect(shouldKeepStream(NOW - 3 * GRACE, NOW)).toBe(false);
  });

  it("기본 유예는 1분이다 (2026-09-15 사용자 지정 — 종전 3시간 → 30분 → 1분)", () => {
    expect(STREAM_BACKGROUND_GRACE_MS).toBe(GRACE);
  });

  it("유예를 넘기면 그 값으로 판정한다", () => {
    expect(shouldKeepStream(NOW - 90_000, NOW, 60_000)).toBe(false);
    expect(shouldKeepStream(NOW - 30_000, NOW, 60_000)).toBe(true);
  });

  it("시계가 뒤로 간 경우에도 붙어 있는다 — 놓을 근거가 없다", () => {
    // 노트북이 깨어나며 시각이 보정되면 now가 숨은 시각보다 앞설 수 있다.
    expect(shouldKeepStream(NOW + 60 * MINUTE, NOW)).toBe(true);
  });
});
