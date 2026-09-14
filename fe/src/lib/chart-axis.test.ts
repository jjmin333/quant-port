import { describe, expect, it } from "vitest";
import {
  AXIS_RIGHT_PAD_FALLBACK_BARS,
  AXIS_RIGHT_PAD_PX,
  rightEdgePad,
  rightEdgePadPx,
  widestAxisLabelSample,
} from "@/lib/chart-axis";

describe("rightEdgePad", () => {
  it("픽셀 여백을 봉 단위로 환산한다 — 좁은 화면일수록 더 많은 봉을 띄운다", () => {
    // 234px에 60봉 → 봉 간격 3.9px → 26px은 약 6.7봉.
    const narrow = rightEdgePad(234, 60, 26);
    expect(narrow.exact).toBe(true);
    expect(narrow.bars).toBeCloseTo(26 / (234 / 60), 5);

    // 같은 봉 수라도 넓은 화면에서는 봉 간격이 커서 더 적은 봉이면 충분하다.
    const wide = rightEdgePad(1200, 60, 26);
    expect(wide.bars).toBeLessThan(narrow.bars);
  });

  it("여백을 안 주면 기본값을 쓴다", () => {
    expect(rightEdgePad(234, 60).bars).toBeCloseTo(AXIS_RIGHT_PAD_PX / (234 / 60), 5);
  });

  it("축 폭을 아직 모르면 폴백을 쓰되 **모른다고 말한다**", () => {
    // 첫 렌더가 여기 걸린다 — 부르는 쪽은 폭이 정해진 뒤 다시 물어야 한다.
    expect(rightEdgePad(0, 60)).toEqual({ bars: AXIS_RIGHT_PAD_FALLBACK_BARS, exact: false });
    expect(rightEdgePad(282, 0)).toEqual({ bars: AXIS_RIGHT_PAD_FALLBACK_BARS, exact: false });
    expect(rightEdgePad(Number.NaN, 60).exact).toBe(false);
  });

  it("봉이 아주 넓어도 최소 반 봉은 띄운다", () => {
    // 봉 간격 200px이면 26px은 0.13봉 — 그래도 예전 동작(0.5봉)보다 좁히지 않는다.
    expect(rightEdgePad(1000, 5).bars).toBe(AXIS_RIGHT_PAD_FALLBACK_BARS);
  });
});

describe("rightEdgePadPx", () => {
  it("잰 라벨의 반쪽 + 3px — 11px Pretendard에서 「2026-09」는 44.3px이라 26px이 필요하다", () => {
    // 예전 상수 16px은 이 값에 10px 모자랐고, 그래서 「2026-09」의 끝이 잘렸다.
    expect(rightEdgePadPx(44.27)).toBe(26);
    expect(rightEdgePadPx(44.27)).toBeGreaterThan(16);
  });

  it("재지 못했으면(0) 기본값으로 떨어진다", () => {
    expect(rightEdgePadPx(0)).toBe(AXIS_RIGHT_PAD_PX);
    expect(rightEdgePadPx(Number.NaN)).toBe(AXIS_RIGHT_PAD_PX);
  });

  it("기본값도 「2026-09」를 덮는다 — 예전 16px은 6px 모자랐다", () => {
    expect(AXIS_RIGHT_PAD_PX).toBeGreaterThanOrEqual(44.27 / 2);
  });
});

describe("widestAxisLabelSample", () => {
  it("분·시간봉은 시:분, 그 밖은 연-월", () => {
    expect(widestAxisLabelSample("1m")).toBe("00:00");
    expect(widestAxisLabelSample("30m")).toBe("00:00");
    expect(widestAxisLabelSample("1h")).toBe("00:00");
    expect(widestAxisLabelSample("1d")).toBe("2026-09");
    expect(widestAxisLabelSample("1wk")).toBe("2026-09");
    expect(widestAxisLabelSample("1mo")).toBe("2026-09");
  });
});
