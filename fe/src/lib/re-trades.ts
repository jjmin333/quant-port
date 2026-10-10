/**
 * 단지 실거래 묶기·이동평균 (2026-10-03 R1). 순수 함수만 — 시험은 `re-trades.test.ts`.
 */

/**
 * 평형 = 전용면적의 정수 부분. 84.39·84.6·84.89㎡는 모두 「84㎡형」이다(분양 표기 84A·84B가 같은 84).
 * 예전엔 반올림이라 84.39는 84, 84.6은 85로 갈라졌다.
 */
export function areaType(area: number): number {
  return Math.floor(area + 1e-9);
}

/** 공급면적 기준 평 — 전용 × 1.33 ÷ 3.3058의 근사(아파트 평균 전용률 약 75%). 84㎡ → 34평, 59㎡ → 24평. */
export function supplyPyeong(area: number): number {
  return Math.round((area * 1.33) / 3.3058);
}

export type AreaGroup = { key: number; area: number; n: number };

/** 거래들을 평형으로 묶는다. area는 그 평형에서 가장 흔한 전용면적(추정가 조회에 쓴다). */
export function groupAreas(trades: { area: number }[]): AreaGroup[] {
  const m = new Map<number, Map<number, number>>();
  for (const t of trades) {
    const k = areaType(t.area);
    const counts = m.get(k) ?? new Map<number, number>();
    counts.set(t.area, (counts.get(t.area) ?? 0) + 1);
    m.set(k, counts);
  }
  return [...m.entries()]
    .map(([key, counts]) => {
      let area = key;
      let best = -1;
      let n = 0;
      for (const [a, c] of counts) {
        n += c;
        if (c > best) [area, best] = [a, c];
      }
      return { key, area, n };
    })
    .sort((a, b) => a.key - b.key);
}

const DAY = 864e5;

/**
 * 이동평균선 — 달마다(그달 15일) 직전 windowDays 안 거래의 평균. 그 창에 거래가 없는 달은 건너뛴다.
 * 입력 순서는 상관없다.
 */
export function movingAverage(points: { t: number; price: number }[], windowDays = 90): { t: number; price: number }[] {
  if (points.length === 0) return [];
  const sorted = [...points].sort((a, b) => a.t - b.t);
  const first = new Date(sorted[0].t);
  const last = sorted[sorted.length - 1].t;
  const out: { t: number; price: number }[] = [];
  for (let y = first.getUTCFullYear(), m = first.getUTCMonth(); ; m++) {
    const at = Date.UTC(y, m, 15);
    if (at - 15 * DAY > last) break;
    const t = Math.min(at, last);
    const inWin = sorted.filter((p) => p.t <= t && p.t > t - windowDays * DAY);
    if (inWin.length > 0) out.push({ t, price: Math.round(inWin.reduce((s, p) => s + p.price, 0) / inWin.length) });
  }
  return out;
}
