import { describe, expect, it } from "vitest";
import { STREAM_BACKGROUND_GRACE_MS, shouldKeepStream } from "./stream-awake";

const NOW = Date.UTC(2026, 8, 5, 6, 0, 0);
const HOUR = 60 * 60 * 1000;

describe("shouldKeepStream", () => {
  it("보이는 중이면 언제나 붙어 있는다", () => {
    expect(shouldKeepStream(null, NOW)).toBe(true);
  });

  it("막 숨었으면 붙어 있는다", () => {
    expect(shouldKeepStream(NOW, NOW)).toBe(true);
  });

  it("숨은 지 3시간 미만이면 붙어 있는다 — 요청의 핵심", () => {
    expect(shouldKeepStream(NOW - 2 * HOUR, NOW)).toBe(true);
    expect(shouldKeepStream(NOW - (3 * HOUR - 1), NOW)).toBe(true);
  });

  it("정확히 3시간이 되면 놓는다", () => {
    expect(shouldKeepStream(NOW - 3 * HOUR, NOW)).toBe(false);
  });

  it("3시간을 넘겼으면 놓는다", () => {
    expect(shouldKeepStream(NOW - 5 * HOUR, NOW)).toBe(false);
  });

  it("기본 유예는 3시간이다", () => {
    expect(STREAM_BACKGROUND_GRACE_MS).toBe(3 * HOUR);
  });

  it("유예를 넘기면 그 값으로 판정한다", () => {
    expect(shouldKeepStream(NOW - 90_000, NOW, 60_000)).toBe(false);
    expect(shouldKeepStream(NOW - 30_000, NOW, 60_000)).toBe(true);
  });

  it("시계가 뒤로 간 경우에도 붙어 있는다 — 놓을 근거가 없다", () => {
    // 노트북이 깨어나며 시각이 보정되면 now가 숨은 시각보다 앞설 수 있다.
    expect(shouldKeepStream(NOW + HOUR, NOW)).toBe(true);
  });
});
