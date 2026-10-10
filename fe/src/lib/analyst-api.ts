import { apiRequest } from "@/lib/api-client";
import type { AnalystConsensus } from "@/lib/stock-signals-api";

/**
 * 개요 「애널리스트 의견」 카드의 자료 (2026-10-07) — `GET /stocks/{symbol}/analyst`.
 *
 * 이력(099 `stock_analyst_history`): 해외는 Finnhub 월별 명수(월 1줄), 국내는 네이버
 * 의견·목표가가 **바뀐 날만** 1줄. 리포트는 네이버의 최근 증권사 리포트(제목·증권사·날짜).
 * 배치가 채운 표만 읽는다 — 조회에서 외부를 부르지 않는다.
 */

export type AnalystHistoryPoint = {
  asOf: string;
  source: string;
  rating?: number;
  targetMean?: number;
  targetCurrency?: string;
  /** 강력매수·매수·보유·매도·강력매도 */
  counts?: [number, number, number, number, number];
};

export type AnalystReport = {
  id: number;
  source: string;
  broker: string;
  title: string;
  published?: string;
};

export type AnalystData = {
  consensus: AnalystConsensus | null;
  /** 오래된 것이 먼저 */
  history: AnalystHistoryPoint[];
  /** 최신이 먼저 */
  reports: AnalystReport[];
};

export async function fetchAnalyst(symbol: string): Promise<AnalystData> {
  const d = await apiRequest<Partial<AnalystData> | null>(
    `/api/v1/stocks/${encodeURIComponent(symbol)}/analyst`,
    undefined,
    "애널리스트 자료를 불러오지 못했습니다",
  );
  return { consensus: d?.consensus ?? null, history: d?.history ?? [], reports: d?.reports ?? [] };
}

export function analystReportUrl(r: AnalystReport): string | null {
  return r.source === "naver" ? `https://stock.naver.com/research/company/${r.id}` : null;
}

/** 「의견 변화」 — 최신 시점과 약 90일 전(없으면 가장 오래된) 시점의 차이. */
export type AnalystTrend = {
  sinceDate: string;
  /** 평균 의견(1~5) 차이 */
  ratingDelta?: number;
  /** 평균 목표가 변화율(%) */
  targetPct?: number;
  /** 매수 이상 비중 변화(%p) */
  buyShareDelta?: number;
  /** 애널리스트 수 변화(명) */
  analystsDelta?: number;
};

const DAY_MS = 86_400_000;

function buyShare(c: AnalystHistoryPoint["counts"]): number | undefined {
  if (!c) return undefined;
  const total = c.reduce((a, b) => a + b, 0);
  return total > 0 ? ((c[0] + c[1]) / total) * 100 : undefined;
}

/**
 * 기준 시점은 「최신보다 90일 이상 앞선 마지막 점」이다. 그런 점이 없으면(이력이 짧다) 가장 오래된 점.
 * 점이 하나뿐이면 변화가 없으므로 null — 0으로 적으면 「그대로였다」는 거짓이 된다.
 */
export function analystTrend(history: AnalystHistoryPoint[], windowDays = 90): AnalystTrend | null {
  if (history.length < 2) return null;
  const latest = history[history.length - 1];
  const cutoff = Date.parse(latest.asOf) - windowDays * DAY_MS;
  let ref = history[0];
  for (const p of history.slice(0, -1)) {
    if (Date.parse(p.asOf) <= cutoff) ref = p;
  }
  const out: AnalystTrend = { sinceDate: ref.asOf };
  if (latest.rating != null && ref.rating != null) out.ratingDelta = latest.rating - ref.rating;
  if (latest.targetMean && ref.targetMean) out.targetPct = (latest.targetMean / ref.targetMean - 1) * 100;
  const bNow = buyShare(latest.counts);
  const bRef = buyShare(ref.counts);
  if (bNow != null && bRef != null) out.buyShareDelta = bNow - bRef;
  if (latest.counts && ref.counts) {
    out.analystsDelta = latest.counts.reduce((a, b) => a + b, 0) - ref.counts.reduce((a, b) => a + b, 0);
  }
  return out;
}
