import type { ValuationPoint } from "@/lib/holding-trades-types";
import type { DailyCloseRow } from "@/lib/daily-close-types";

/**
 * 내 포트폴리오 vs 지수 (2026-09-24 사용자 결정).
 *
 * # 왜 평가금액을 그대로 비교하지 않나
 *
 * 평가금액 곡선에는 **추가 매수**가 섞인다. 100만 원을 더 사면 곡선이 뛰지만 그것은 수익이
 * 아니다 — 지수와 그대로 겹치면 입금이 많은 사람일수록 시장을 이긴 것처럼 보인다. 그래서
 * **시간가중수익률(TWR)**로 바꾼다: 매입금액의 변화를 그날의 현금 흐름으로 보고 뺀다.
 *
 *     r_t = (V_t − ΔC_t) / V_{t−1} − 1,   누적 = Π(1 + r_t) − 1
 *
 * 매입금액(원가)의 변화를 흐름으로 보는 것은 근사다 — 팔 때 실현손익이 원가 변화와 다르면
 * 그만큼 오차가 남는다. 화면이 한 줄로 밝힌다.
 */
export type ComparePoint = { date: string; pct: number };

export function portfolioTWR(points: ValuationPoint[]): ComparePoint[] {
  const out: ComparePoint[] = [];
  let growth = 1;
  for (let i = 0; i < points.length; i++) {
    const p = points[i]!;
    if (i === 0) {
      out.push({ date: p.date, pct: 0 });
      continue;
    }
    const prev = points[i - 1]!;
    if (prev.valueKrw > 0) {
      const flow = p.costKrw - prev.costKrw;
      const r = (p.valueKrw - flow) / prev.valueKrw - 1;
      // 첫 매수일처럼 전날 값이 0인 날은 수익률이 없다 — 곱하지 않는다.
      if (Number.isFinite(r)) growth *= 1 + r;
    }
    out.push({ date: p.date, pct: (growth - 1) * 100 });
  }
  return out;
}

/**
 * 지수 종가를 포트폴리오 날짜에 맞춘다 — 그날 이하의 가장 최근 종가(휴장일은 직전 값).
 * 시작점을 0%로 둔다. 시작일 이전 종가가 없으면 비교하지 않는다(null).
 */
export function alignIndex(dates: string[], rows: DailyCloseRow[]): ComparePoint[] | null {
  const sorted = [...rows]
    .filter((r) => r.close > 0)
    .sort((a, b) => a.tradeDate.localeCompare(b.tradeDate));
  if (sorted.length === 0 || dates.length === 0) return null;
  const at = (d: string): number | null => {
    let v: number | null = null;
    for (const r of sorted) {
      if (r.tradeDate > d) break;
      v = r.close;
    }
    return v;
  };
  const base = at(dates[0]!);
  if (base == null) return null;
  return dates.map((d) => ({ date: d, pct: ((at(d) ?? base) / base - 1) * 100 }));
}
