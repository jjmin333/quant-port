/**
 * 포트폴리오 위험 — 변동성 · 최대낙폭 · 지수 베타 · 함께 움직이는 종목 쌍 (2026-09-24 사용자 결정).
 *
 * # 무엇을 재는가: 「지금 비중으로 지난 반년을 들고 있었다면」
 *
 * 매매 이력으로 만든 실제 곡선(평가금액 추이)은 추가 매수·매도가 섞여 변동성을 재기에 거칠고,
 * 이력이 없는 사용자(스크린샷 가져오기)에게는 아예 없다. 그래서 **현재 비중을 고정하고** 각
 * 종목의 일별 수익률을 섞는다. 「내 계좌가 실제로 이렇게 움직였다」가 아니라 「지금 들고 있는
 * 조합의 성질」이다 — 카드가 그렇게 적는다.
 *
 * # 날짜 맞추기
 *
 * 국내·미국 종목이 섞이면 휴장일이 다르다. 모든 종목의 날짜를 합친 달력 위에서, 그날 종가가
 * 없는 종목은 **움직이지 않은 것**(수익률 0)으로 본다. 환율은 반영하지 않는다(현지 통화 수익률).
 */

export type Close = { d: string; c: number };

export type RiskHolding = { symbol: string; name: string; weight: number };

export type RiskPair = { a: string; b: string; aName: string; bName: string; corr: number; weight: number };

export type PortfolioRisk = {
  days: number;
  /** 연율 변동성 %. */
  volatilityPct: number;
  /** 최대낙폭 % (음수). */
  maxDrawdownPct: number;
  /** 지수 대비 베타. 지수 자료가 모자라면 null. */
  beta: number | null;
  /** 상관 0.6 이상인 쌍, 상관이 큰 순서로 셋까지. */
  pairs: RiskPair[];
  /** 종가가 모자라 빠진 종목. */
  skipped: string[];
};

export const RISK_MIN_DAYS = 40;
export const RISK_PAIR_THRESHOLD = 0.6;

/** 종가 → 날짜별 수익률(그 종목의 직전 종가 대비). */
function returnsByDate(rows: Close[]): Map<string, number> {
  const out = new Map<string, number>();
  const sorted = [...rows].filter((r) => r.c > 0).sort((a, b) => a.d.localeCompare(b.d));
  for (let i = 1; i < sorted.length; i++) {
    out.set(sorted[i]!.d, sorted[i]!.c / sorted[i - 1]!.c - 1);
  }
  return out;
}

function mean(v: number[]): number {
  return v.reduce((s, x) => s + x, 0) / v.length;
}

function sd(v: number[]): number {
  const m = mean(v);
  return Math.sqrt(v.reduce((s, x) => s + (x - m) ** 2, 0) / Math.max(1, v.length - 1));
}

function corr(x: number[], y: number[]): number | null {
  const mx = mean(x);
  const my = mean(y);
  let cxy = 0;
  let cxx = 0;
  let cyy = 0;
  for (let i = 0; i < x.length; i++) {
    cxy += (x[i]! - mx) * (y[i]! - my);
    cxx += (x[i]! - mx) ** 2;
    cyy += (y[i]! - my) ** 2;
  }
  if (cxx <= 0 || cyy <= 0) return null;
  return cxy / Math.sqrt(cxx * cyy);
}

export function computePortfolioRisk(
  holdings: RiskHolding[],
  closes: Record<string, Close[]>,
  index: Close[] | null,
): PortfolioRisk | null {
  const series = new Map<string, Map<string, number>>();
  const skipped: string[] = [];
  for (const h of holdings) {
    const r = returnsByDate(closes[h.symbol] ?? []);
    if (r.size < RISK_MIN_DAYS) skipped.push(h.symbol);
    else series.set(h.symbol, r);
  }
  const used = holdings.filter((h) => series.has(h.symbol));
  const totalW = used.reduce((s, h) => s + h.weight, 0);
  if (used.length === 0 || totalW <= 0) return null;

  // 모든 종목 날짜의 합집합 — 그중 모든 종목이 시작한 뒤의 날만 쓴다(상장 전을 0으로 채우지 않는다).
  const start = used
    .map((h) => [...series.get(h.symbol)!.keys()].sort()[0]!)
    .sort()
    .at(-1)!;
  const dates = [
    ...new Set(used.flatMap((h) => [...series.get(h.symbol)!.keys()])),
  ]
    .filter((d) => d >= start)
    .sort();
  if (dates.length < RISK_MIN_DAYS) return null;

  const port = dates.map((d) =>
    used.reduce((s, h) => s + (h.weight / totalW) * (series.get(h.symbol)!.get(d) ?? 0), 0),
  );

  let peak = 1;
  let level = 1;
  let mdd = 0;
  for (const r of port) {
    level *= 1 + r;
    peak = Math.max(peak, level);
    mdd = Math.min(mdd, level / peak - 1);
  }

  let beta: number | null = null;
  if (index && index.length > 0) {
    const idx = returnsByDate(index);
    const xs: number[] = [];
    const ys: number[] = [];
    dates.forEach((d, i) => {
      const v = idx.get(d);
      if (v != null) {
        xs.push(v);
        ys.push(port[i]!);
      }
    });
    if (xs.length >= RISK_MIN_DAYS) {
      const mx = mean(xs);
      const my = mean(ys);
      let cov = 0;
      let vx = 0;
      for (let i = 0; i < xs.length; i++) {
        cov += (xs[i]! - mx) * (ys[i]! - my);
        vx += (xs[i]! - mx) ** 2;
      }
      if (vx > 0) beta = cov / vx;
    }
  }

  const pairs: RiskPair[] = [];
  for (let i = 0; i < used.length; i++) {
    for (let j = i + 1; j < used.length; j++) {
      const a = used[i]!;
      const b = used[j]!;
      const sa = series.get(a.symbol)!;
      const sb = series.get(b.symbol)!;
      const common = dates.filter((d) => sa.has(d) && sb.has(d));
      if (common.length < RISK_MIN_DAYS) continue;
      const c = corr(
        common.map((d) => sa.get(d)!),
        common.map((d) => sb.get(d)!),
      );
      if (c != null && c >= RISK_PAIR_THRESHOLD) {
        pairs.push({
          a: a.symbol,
          b: b.symbol,
          aName: a.name,
          bName: b.name,
          corr: c,
          weight: ((a.weight + b.weight) / totalW) * 100,
        });
      }
    }
  }
  pairs.sort((x, y) => y.corr - x.corr);

  return {
    days: dates.length,
    volatilityPct: sd(port) * Math.sqrt(250) * 100,
    maxDrawdownPct: mdd * 100,
    beta,
    pairs: pairs.slice(0, 3),
    skipped,
  };
}

/* ------------------------------------------------------------------------- *
 * 바꿔 보기 (what-if) — 2026-09-24 사용자 결정: 위험 카드 안.
 *
 * 저장하지 않는 계산기다. 「N주 더 사거나 팔면」 비중이 어떻게 바뀌는지만 보여 주고, 사라고도
 * 팔라고도 말하지 않는다. 가격은 저장된 마지막 종가(원화 환산)다 — 지금 호가가 아니다.
 * ------------------------------------------------------------------------- */

export type WhatIfHolding = RiskHolding & { sector?: string };

/** 한 종목의 원화 평가액을 deltaKRW만큼 바꾼 새 보유. 0 이하가 되면 뺀다(공매도는 없다). */
export function applyWhatIf(
  holdings: WhatIfHolding[],
  change: { symbol: string; name: string; deltaKRW: number; sector?: string },
): WhatIfHolding[] {
  const out = holdings.map((h) => ({ ...h }));
  const hit = out.find((h) => h.symbol === change.symbol);
  if (hit) {
    hit.weight = Math.max(0, hit.weight + change.deltaKRW);
  } else if (change.deltaKRW > 0) {
    out.push({ symbol: change.symbol, name: change.name, weight: change.deltaKRW, sector: change.sector });
  }
  return out.filter((h) => h.weight > 0);
}

/** 섹터별 비중(%), 큰 순서. 섹터를 모르면 「미분류」. */
export function sectorShares(holdings: WhatIfHolding[]): { sector: string; pct: number }[] {
  const total = holdings.reduce((s, h) => s + h.weight, 0);
  if (total <= 0) return [];
  const by = new Map<string, number>();
  for (const h of holdings) {
    const k = h.sector || "미분류";
    by.set(k, (by.get(k) ?? 0) + h.weight);
  }
  return [...by.entries()]
    .map(([sector, v]) => ({ sector, pct: (v / total) * 100 }))
    .sort((a, b) => b.pct - a.pct);
}
