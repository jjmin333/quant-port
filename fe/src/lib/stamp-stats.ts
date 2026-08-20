import { STANCE_LABELS, type Stamp, type StampStance } from "@/lib/stamp-types";

/**
 * 성적표에 들어가는 집계.
 *
 * 전부 이미 받아둔 스탬프 배열에서 계산한다 — `GET /stamps` 한 번에 판정·알파·
 * 확신도·목표가 오차가 전부 실려 오므로 화면이 쪼갤 축마다 서버를 다시 부를 이유가
 * 없다. 개인 스탬프는 많아야 수백 건이라 매 렌더 계산해도 체감되지 않는다.
 */

/** 판정이 끝난 스탬프만 성적에 넣는다 — 검증 중인 것을 섞으면 초기 적중률이 0에 눌린다. */
export function isJudged(s: Stamp): boolean {
  return s.verdict === "hit" || s.verdict === "miss";
}

export type Bucket = {
  key: string;
  label: string;
  judged: number;
  hits: number;
  /** 판정 건이 없으면 undefined — 0%로 적으면 "다 틀렸다"로 읽힌다. */
  hitRate?: number;
  avgAlpha?: number;
};

function bucketOf(key: string, label: string, items: Stamp[]): Bucket {
  const judged = items.filter(isJudged);
  const hits = judged.filter((s) => s.verdict === "hit").length;
  const alphas = items
    .map((s) => s.horizon?.alpha)
    .filter((v): v is number => v !== undefined);
  return {
    key,
    label,
    judged: judged.length,
    hits,
    hitRate: judged.length > 0 ? (hits / judged.length) * 100 : undefined,
    avgAlpha: alphas.length > 0 ? alphas.reduce((a, b) => a + b, 0) / alphas.length : undefined,
  };
}

export function byStance(stamps: Stamp[]): Bucket[] {
  const order: StampStance[] = ["bullish", "neutral", "bearish"];
  return order.map((stance) =>
    bucketOf(stance, STANCE_LABELS[stance], stamps.filter((s) => s.stance === stance)),
  );
}

export function byHorizon(stamps: Stamp[]): Bucket[] {
  const seen = new Map<string, Stamp[]>();
  for (const s of stamps) {
    const list = seen.get(s.horizonKind) ?? [];
    list.push(s);
    seen.set(s.horizonKind, list);
  }
  // 라벨은 화면 쪽 HORIZON_LABELS와 맞춰야 하므로 호출부에서 붙인다.
  return Array.from(seen.entries()).map(([kind, items]) => bucketOf(kind, kind, items));
}

export function byConviction(stamps: Stamp[]): Bucket[] {
  return [1, 2, 3, 4, 5].map((v) =>
    bucketOf(String(v), `확신 ${v}`, stamps.filter((s) => s.conviction === v)),
  );
}

export type SymbolRecord = {
  symbol: string;
  name: string;
  count: number;
  judged: number;
  hits: number;
  hitRate?: number;
  avgAlpha?: number;
};

/** 종목별 성적, 스탬프가 많은 순. */
export function bySymbol(stamps: Stamp[]): SymbolRecord[] {
  const map = new Map<string, Stamp[]>();
  for (const s of stamps) {
    const list = map.get(s.symbol) ?? [];
    list.push(s);
    map.set(s.symbol, list);
  }
  return Array.from(map.entries())
    .map(([symbol, items]) => {
      const b = bucketOf(symbol, symbol, items);
      return {
        symbol,
        name: items[0]?.symbolName || symbol,
        count: items.length,
        judged: b.judged,
        hits: b.hits,
        hitRate: b.hitRate,
        avgAlpha: b.avgAlpha,
      };
    })
    .sort((a, b) => b.count - a.count || a.symbol.localeCompare(b.symbol));
}

/**
 * 스탬프를 찍은 순서대로 알파를 누적한다. 각 점은 "여기까지 찍은 스탬프들이 벤치마크
 * 대비 합계 몇 %p 앞섰나"이다 — 수익률이 아니라 판단의 누적 성적이다.
 */
export function cumulativeAlpha(stamps: Stamp[]): number[] {
  const ordered = stamps
    .filter((s) => s.horizon?.alpha !== undefined)
    .slice()
    .sort((a, b) => (a.stampedAt < b.stampedAt ? -1 : 1));
  let sum = 0;
  return ordered.map((s) => {
    sum += s.horizon?.alpha ?? 0;
    return sum;
  });
}

export type HorizonAverage = {
  horizon: string;
  mine?: number;
  benchmark?: number;
};

/** 구간별로 내 평균 수익률과 벤치마크 평균을 나란히 낸다. */
export function horizonAverages(stamps: Stamp[]): HorizonAverage[] {
  const labels = ["1D", "1W", "1M", "3M"];
  return labels.map((horizon) => {
    const rows = stamps
      .map((s) => s.returns.find((r) => r.horizon === horizon))
      .filter((r): r is NonNullable<typeof r> => r !== undefined);
    const mine = rows.map((r) => r.return).filter((v): v is number => v !== undefined);
    const bench = rows
      .map((r) => r.benchmarkReturn)
      .filter((v): v is number => v !== undefined);
    return {
      horizon,
      mine: mine.length > 0 ? mine.reduce((a, b) => a + b, 0) / mine.length : undefined,
      benchmark: bench.length > 0 ? bench.reduce((a, b) => a + b, 0) / bench.length : undefined,
    };
  });
}

/** 성적이 가장 좋았던/나빴던 스탬프 — 성적표가 이름을 댈 수 있게. */
export function bestAndWorst(stamps: Stamp[]): { best?: Stamp; worst?: Stamp } {
  const scored = stamps.filter((s) => s.horizon?.alpha !== undefined);
  if (scored.length === 0) return {};
  let best = scored[0];
  let worst = scored[0];
  for (const s of scored) {
    if ((s.horizon?.alpha ?? 0) > (best.horizon?.alpha ?? 0)) best = s;
    if ((s.horizon?.alpha ?? 0) < (worst.horizon?.alpha ?? 0)) worst = s;
  }
  return { best, worst };
}
