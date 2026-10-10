import { getApiBaseUrl } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import { fetchWithDeadline } from "@/lib/fetch-timeout";
import type { EarningsEvent } from "@/lib/earnings-types";
import type { StockScoreData } from "@/lib/stock-score-api";

/*
홈 비로그인 블록의 재료 (2026-10-10 사용자 결정 — 시안 `docs/mockups/home-guest-v2.html` A안).

전부 **저장분 조회**이고 서버에서 한 번에 받는다 — 처음 온 사람과 크롤러가 같은 HTML을 본다.
점수는 하루 한 번(00:25) 바뀌므로 10분 접는다. 하나가 실패해도 그 칸만 빠진다.
*/

const REVALIDATE = 600;

/** 대표 종목 — 고정(사용자 결정). 국내 둘·미국 넷, 처음 온 사람이 이름을 아는 종목. */
export const GUEST_FEATURED = ["005930", "000660", "NVDA", "AAPL", "TSLA", "MSFT"] as const;

async function get<T>(path: string): Promise<T | null> {
  try {
    const res = await fetchWithDeadline(`${getApiBaseUrl()}${path}`, { next: { revalidate: REVALIDATE } });
    if (!res.ok) return null;
    const body = (await res.json()) as ApiResponse<T>;
    return body.data ?? null;
  } catch {
    return null;
  }
}

type ScoreChangeRow = {
  symbol: string;
  score: number;
  fromScore?: number;
  delta?: number;
  available: boolean;
  topDriver?: { kind: string; label?: string; from?: number; to?: number };
};

export type GuestMover = {
  symbol: string;
  name: string;
  from: number;
  to: number;
  delta: number;
  driver?: { label: string; from: number; to: number };
};

export type GuestEarning = { symbol: string; name: string; date: string; period: string; score?: number };

export type GuestBacktest = {
  fromDate: string;
  toDate: string;
  totalReturn: number;
  benchmarks: { label: string; alpha: number }[];
};

export type GuestShowcase = {
  featured: StockScoreData[];
  movers: { domestic: GuestMover[]; us: GuestMover[]; date?: string };
  earnings: GuestEarning[];
  backtest: GuestBacktest | null;
};

type ChangesPayload = { date: string; items: { symbol: string; name?: string; kind: string }[] };

function scoreList(data: unknown): StockScoreData[] {
  if (Array.isArray(data)) return data as StockScoreData[];
  return data ? (Object.values(data as Record<string, StockScoreData>)) : [];
}

async function movers(market: "domestic" | "us"): Promise<{ date?: string; rows: GuestMover[] }> {
  const changes = await get<ChangesPayload>(`/api/v1/changes?market=${market}&kinds=score_jump`);
  const top = (changes?.items ?? []).slice(0, 5);
  if (top.length === 0) return { date: changes?.date, rows: [] };
  const names = new Map(top.map((c) => [c.symbol, c.name ?? c.symbol]));
  const rows = await get<ScoreChangeRow[]>(
    `/api/v1/stocks/scores/changes?symbols=${encodeURIComponent(top.map((c) => c.symbol).join(","))}`,
  );
  const out: GuestMover[] = [];
  for (const r of rows ?? []) {
    if (!r.available || r.fromScore == null || r.delta == null) continue;
    const d = r.topDriver;
    out.push({
      symbol: r.symbol,
      name: names.get(r.symbol) ?? r.symbol,
      from: r.fromScore,
      to: r.score,
      delta: r.delta,
      driver:
        d?.kind === "axis" && d.label && d.from != null && d.to != null
          ? { label: d.label, from: d.from, to: d.to }
          : undefined,
    });
  }
  out.sort((a, b) => Math.abs(b.delta) - Math.abs(a.delta));
  return { date: changes?.date, rows: out };
}

function addDays(ymd: string, days: number): string {
  const d = new Date(`${ymd}T00:00:00+09:00`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
}

/*
앞으로 2주 실적 — 시가총액 순. **시장별로 따로 줄 세운다**: 응답의 시가총액이 국내는 원, 미국은
달러라 한 줄로 정렬하면 서울반도체(6천억 원)가 램 리서치(3,987억 달러) 위에 선다(10-10 실측).
*/
async function earnings(): Promise<GuestEarning[]> {
  const from = new Date().toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
  const events = (await get<EarningsEvent[]>(`/api/v1/earnings/calendar?from=${from}&to=${addDays(from, 14)}`)) ?? [];
  const pick = (market: string, n: number) =>
    events
      .filter((e) => e.market === market)
      .sort((a, b) => (b.marketCap ?? 0) - (a.marketCap ?? 0))
      .slice(0, n);
  const chosen = [...pick("us", 4), ...pick("domestic", 2)].sort((a, b) => a.date.localeCompare(b.date));
  if (chosen.length === 0) return [];
  const scores = scoreList(
    await get<unknown>(`/api/v1/stocks/scores/batch?symbols=${encodeURIComponent(chosen.map((e) => e.symbol).join(","))}`),
  );
  const byScore = new Map(scores.map((s) => [s.symbol, s.score ?? undefined]));
  return chosen.map((e) => ({
    symbol: e.symbol,
    name: e.name ?? e.symbol,
    date: e.date,
    period: `${e.fiscalPeriod}`,
    score: byScore.get(e.symbol) ?? undefined,
  }));
}

type BacktestPayload = {
  fromDate: string;
  toDate: string;
  portfolio: { totalReturn: number };
  benchmarks: { label: string; alpha: number; primary?: boolean }[];
};

/** 「이 랭킹의 성적」과 같은 기본값(국내 · 종합 · 상위 20 · 매주). 숫자는 그대로 공개(사용자 결정). */
async function backtest(): Promise<GuestBacktest | null> {
  const b = await get<BacktestPayload>(`/api/v1/stocks/scores/backtest?market=domestic&axis=composite&topN=20&rebalance=weekly`);
  if (!b?.portfolio) return null;
  return {
    fromDate: b.fromDate,
    toDate: b.toDate,
    totalReturn: b.portfolio.totalReturn,
    benchmarks: b.benchmarks.map((x) => ({ label: x.label, alpha: x.alpha })),
  };
}

export async function fetchGuestShowcase(): Promise<GuestShowcase> {
  const [featuredRaw, dom, us, earn, bt] = await Promise.all([
    get<unknown>(`/api/v1/stocks/scores/batch?symbols=${GUEST_FEATURED.join(",")}`),
    movers("domestic"),
    movers("us"),
    earnings(),
    backtest(),
  ]);
  const featured = scoreList(featuredRaw);
  const order = new Map<string, number>(GUEST_FEATURED.map((s, i) => [s, i]));
  featured.sort((a, b) => (order.get(a.symbol) ?? 99) - (order.get(b.symbol) ?? 99));
  return {
    featured: featured.filter((s) => s.score != null),
    movers: { domestic: dom.rows, us: us.rows, date: dom.date ?? us.date },
    earnings: earn,
    backtest: bt,
  };
}
