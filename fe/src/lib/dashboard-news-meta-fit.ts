const META_GAP_PX = 6; // gap-1.5
const CHIP_GAP_PX = 4; // gap-1 inside NewsArticleChips

/**
 * 몇 개까지 한 줄에 넣을지.
 *
 * # 「+N」을 세지 않아 칩이 뭉개지던 버그 (2026-09-05 사용자 지적 2)
 *
 * 홈 뉴스 줄의 칩이 「코…」·「메리츠금…」처럼 **한두 글자로** 잘려, 무슨 종목인지
 * 못 읽는 상태로 서 있었다. 이름이 긴 종목이 잘리는 것(「LG에너지…」)은 의도지만,
 * 「코…」는 그냥 고장으로 읽힌다.
 *
 * 원인은 이 함수가 재던 폭과 실제로 그려지는 폭이 달랐던 것이다. 측정 줄은 칩을
 * **최대 세 개까지만** 그리는데(`META_CHIP_CAP`), 기사에 걸린 칩이 열한 개면 화면에는
 * 세 개 옆에 **「+8」이 함께** 그려진다. 그런데 예전 계산은 「보여줄 개수 = 잰 개수」인
 * 경우 숨김이 0이라고 보고 그 「+8」의 폭을 **빼먹었다**. 모자란 폭만큼 flex가 칩들을
 * 눌러 줄였고, 그 결과가 「코…」다.
 *
 * 그래서 이제 **숨김 수는 총 개수 기준**(`totalCount - shown`)으로 센다. 잰 개수는
 * 상한일 뿐이고, 옆에 적히는 숫자는 언제나 진짜 남은 개수다.
 *
 * # 하나도 안 들어가면 **0개** (같은 요청)
 *
 * 예전에는 마지막에 `return 1`로 한 개는 억지로 남겼다. 그 한 개가 정확히 위에서 말한
 * 뭉개진 칩이 된다. 「잘리게 되는 경우 생략하라」는 요청대로 0을 돌려준다 — 그 줄은
 * 「+11 · 11분 전」이 되어, 뭉개진 이름보다 **더 많은 것**을 말한다.
 */
function fitMetaChipCount(
  chipWidths: number[],
  totalCount: number,
  timeWidth: number,
  plusWidthByHidden: ReadonlyMap<number, number>,
  availableWidth: number,
): number {
  if (totalCount === 0 || chipWidths.length === 0) return 0;

  for (let shown = chipWidths.length; shown >= 1; shown--) {
    const hidden = totalCount - shown;
    let width = 0;
    for (let i = 0; i < shown; i++) {
      width += chipWidths[i]!;
      if (i > 0) width += CHIP_GAP_PX;
    }
    if (hidden > 0) {
      width += CHIP_GAP_PX + (plusWidthByHidden.get(hidden) ?? 0);
    }
    width += META_GAP_PX + timeWidth;
    if (width <= availableWidth) return shown;
  }

  return 0;
}

/**
 * 이 기사에서 「+N」이 될 수 있는 N들.
 *
 * 측정 줄이 이 숫자들의 폭을 미리 그려 둬야 위 계산이 참고할 수 있다. 「+8」과 「+1」은
 * 자릿수가 달라 폭도 다르므로, 있을 수 있는 값을 **그대로** 그린다(어림잡지 않는다).
 */
export function metaPlusCandidates(totalCount: number, measurableCount: number): number[] {
  const out: number[] = [];
  for (let shown = 0; shown <= measurableCount; shown++) {
    const hidden = totalCount - shown;
    if (hidden > 0) out.push(hidden);
  }
  return out;
}

/**
 * 숨김 측정 줄 DOM으로 메타 칩 몇 개까지 한 줄에 넣을지 계산한다.
 *
 * @param totalCount 이 기사에 걸린 칩의 **총** 개수. 측정 줄이 그리는 개수(상한)와
 *   다를 수 있고, 다를 때 화면에는 「+N」이 함께 그려진다 — 위 머리글 참조.
 */
export function measureMetaChipLayout(
  row: HTMLElement,
  measure: HTMLElement,
  breaking: boolean,
  totalCount: number,
): number {
  const chipWidths = [...measure.querySelectorAll("[data-meta-chip]")].map(
    (el) => el.getBoundingClientRect().width,
  );
  const timeWidth =
    measure.querySelector("[data-meta-time]")?.getBoundingClientRect().width ?? 0;

  const plusWidthByHidden = new Map<number, number>();
  for (const el of measure.querySelectorAll("[data-meta-plus]")) {
    const hidden = Number(el.getAttribute("data-meta-plus"));
    if (hidden > 0) plusWidthByHidden.set(hidden, el.getBoundingClientRect().width);
  }

  let available = row.clientWidth;
  if (breaking) {
    const badge = row.querySelector("[data-meta-breaking]");
    if (badge) {
      available -= badge.getBoundingClientRect().width + META_GAP_PX;
    }
  }

  return fitMetaChipCount(chipWidths, totalCount, timeWidth, plusWidthByHidden, available);
}

/** 계산만 떼어 시험하기 위한 출구 — DOM 없이 폭 숫자로 부른다. */
export const __testing = { fitMetaChipCount };
