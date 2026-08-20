const META_GAP_PX = 6; // gap-1.5
const CHIP_GAP_PX = 4; // gap-1 inside NewsArticleChips

function fitMetaChipCount(
  chipWidths: number[],
  timeWidth: number,
  plusWidthByHidden: ReadonlyMap<number, number>,
  availableWidth: number,
): number {
  const total = chipWidths.length;
  if (total === 0) return 0;

  for (let shown = total; shown >= 1; shown--) {
    const hidden = total - shown;
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

  return 1;
}

/** 숨김 측정 줄 DOM으로 메타 칩 몇 개까지 한 줄에 넣을지 계산한다. */
export function measureMetaChipLayout(
  row: HTMLElement,
  measure: HTMLElement,
  breaking: boolean,
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

  return fitMetaChipCount(chipWidths, timeWidth, plusWidthByHidden, available);
}
