/**
 * 차트 시간축의 **오른쪽 여백** 계산 — 캔버스 밖으로 라벨이 잘려 나가지 않게.
 *
 * 그림 그리는 라이브러리(lightweight-charts)와 떨어뜨려 둔다: 여기 있는 것은 산수뿐이라
 * 캔버스 없이 검증할 수 있고, 「축 폭을 아직 모를 때 어떻게 하나」라는 이 파일의 진짜
 * 주제가 컴포넌트 코드에 묻히지 않는다.
 */

/**
 * 오른쪽 끝에 남겨 두는 여백(px) — **라벨을 재지 못했을 때의 기본값**.
 *
 * # 왜 있나 (2026-09-03 사용자 신고)
 *
 * 모바일 일봉에서 맨 오른쪽 축 라벨이 **글자 중간에서 잘렸다** — 「2026-09」가
 * 「2026-0」까지만 보였다. 시간축 캔버스는 오른쪽 가격축(82px)을 뺀 나머지(실측
 * 375px 화면에서 234px)이고, 라벨은 눈금에 **가운데 정렬**로 그려진다. 마지막 눈금이
 * 축 오른쪽 끝 가까이 놓이면 라벨의 오른쪽 절반이 캔버스 밖으로 나간다.
 *
 * ⚠️ 처음에는 이 값을 **16px**로 뒀는데 그래도 잘렸다. 「라벨 반쪽(≈20px)보다 조금
 * 작게」라고 적어 두었지만 실제로 재 보니 11px Pretendard에서 「2026-09」는
 * **44.3px**이라 반쪽이 22.2px이다 — 16px은 6px 모자랐고, 그래서 사용자에게는
 * 「고쳤다는데 그대로」였다. 어림으로 적어 둔 수를 믿지 말고 **재는 것**이 이 파일이
 * `rightEdgePadPx`를 갖게 된 이유다. 기본값도 그 실측에 맞춰 올렸다.
 */
export const AXIS_RIGHT_PAD_PX = 26;

/**
 * 라벨이 잘리지 않으려면 오른쪽에 얼마나 필요한가.
 *
 * 라벨은 눈금 위에 **가운데 정렬**이므로 반쪽이 넘어간다. 거기에 3px을 더한다 —
 * 글자 끝이 캔버스 경계에 닿아 붙어 보이는 것까지 막는다.
 */
export function rightEdgePadPx(labelWidthPx: number): number {
  if (!(labelWidthPx > 0)) return AXIS_RIGHT_PAD_PX;
  return Math.ceil(labelWidthPx / 2) + 3;
}

/**
 * 그 간격에서 **가장 넓은 축 라벨**의 본보기.
 *
 * 실제로 무엇이 그려질지는 `formatAxisTickLabel`이 정하지만, 여백은 마지막 눈금
 * 하나가 아니라 「이 간격에서 나올 수 있는 가장 긴 라벨」에 맞춰야 안전하다 —
 * 확대·축소로 라벨 종류가 바뀌어도 다시 재지 않아도 된다.
 */
export function widestAxisLabelSample(interval: string): string {
  // 일·주·월봉은 「2026-09」 / 「2026」, 분·시간봉은 「00:00」.
  return interval.endsWith("m") || interval.endsWith("h") ? "00:00" : "2026-09";
}

/** 축 폭을 모를 때 쓰는 여백(봉 단위). 봉 하나의 절반. */
export const AXIS_RIGHT_PAD_FALLBACK_BARS = 0.5;

export type RightEdgePad = {
  /** 마지막 봉 오른쪽에 둘 여백(봉 단위). */
  bars: number;
  /**
   * 실제 축 폭으로 계산했는가.
   *
   * **false면 라벨이 잘린 채로 그려진다** — 첫 렌더에는 캔버스 폭이 아직 0이라
   * 봉 간격을 알 수 없고, 그때 쓰는 폴백(0.5봉)은 「2026-09」를 가리기에 모자란다.
   * 부르는 쪽은 이 값이 false면 **폭이 정해진 뒤 한 번 더 물어야** 한다
   * (2026-09-03 사용자 신고: 「개요 탭을 모바일에서 처음 열면 2026-09가 잘린다」 —
   * 여백 자체는 09-03 오전에 넣었지만, 첫 렌더에서만 이 폴백을 타고 있었다).
   */
  exact: boolean;
};

/**
 * 여백을 **픽셀로 정하고 봉 단위로 환산한다** — 봉 간격이 화면 폭·확대 배율에 따라
 * 달라지므로, 봉 수로 고정하면 좁은 화면에서는 모자라고 넓은 화면에서는 빈 칸이 커진다.
 *
 * @param axisWidth 시간축 캔버스 폭(px). 아직 배치되지 않았으면 0이 온다.
 * @param visibleBars 그 폭에 들어가는 봉 수.
 */
export function rightEdgePad(
  axisWidth: number,
  visibleBars: number,
  padPx: number = AXIS_RIGHT_PAD_PX,
): RightEdgePad {
  if (!(axisWidth > 0) || !(visibleBars > 0)) {
    return { bars: AXIS_RIGHT_PAD_FALLBACK_BARS, exact: false };
  }
  const barSpacing = axisWidth / visibleBars;
  if (!Number.isFinite(barSpacing) || barSpacing <= 0) {
    return { bars: AXIS_RIGHT_PAD_FALLBACK_BARS, exact: false };
  }
  return {
    bars: Math.max(AXIS_RIGHT_PAD_FALLBACK_BARS, padPx / barSpacing),
    exact: true,
  };
}

/**
 * 글자 폭을 재는 캔버스 하나를 만들어 두고 재사용한다. 서버·테스트에서는 잴 수
 * 없으므로 0을 돌려주고, 부르는 쪽은 기본값으로 떨어진다.
 */
let measureCanvas: HTMLCanvasElement | null = null;
const measured = new Map<string, number>();

export function measureAxisLabelWidth(text: string, font: string): number {
  if (typeof document === "undefined") return 0;
  const key = `${font}|${text}`;
  const hit = measured.get(key);
  if (hit != null) return hit;
  measureCanvas ??= document.createElement("canvas");
  const ctx = measureCanvas.getContext("2d");
  if (!ctx) return 0;
  ctx.font = font;
  const width = ctx.measureText(text).width;
  measured.set(key, width);
  return width;
}
