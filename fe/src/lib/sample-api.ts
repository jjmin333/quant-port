import { apiRequest } from "@/lib/api-client";
import type { JournalNote } from "@/lib/journal-types";
import type { WatchlistMetrics } from "@/lib/quotes-types";
import type { Stamp, StampSummary } from "@/lib/stamp-types";

/**
 * 예시 데이터는 **서버**가 준다.
 *
 * FE 상수로 두면 비로그인 방문자에게 보여줄 수는 있지만, 정의가 클라이언트마다
 * 갈라지고 무엇보다 실제 응답 모양과 어긋나기 쉽다 — 화면은 예시에서 잘 뜨는데
 * 로그인하면 깨지는 종류의 어긋남이다. 서버가 진짜 엔드포인트와 같은 모양으로
 * 내려주면 화면이 두 경로를 구분할 필요가 없다.
 *
 * 이 경로들은 인증 밖이다. 원래 엔드포인트가 전부 로그인 뒤에 있어서, 공개 경로가
 * 없으면 로그인하지 않은 사람에게는 어차피 아무것도 못 보여준다.
 */

export type SampleWatchlistRow = {
  id: number;
  symbol: string;
  createdAt: string;
  addedPrice?: number;
  name: string;
  market?: string;
  currency: string;
  price: number;
  changeRate: number;
  score?: number;
  grade?: string;
  week52High?: number;
  week52Low?: number;
  /**
   * 전일·전주·전월·52주 — 로그인한 목록이 받는 것과 **같은 블록**을 서버가 그대로 준다.
   * 예전에는 이 필드가 없어서 아래 조립 함수가 `changeRate` 하나로 흉내를 냈고, 계산할
   * 수 없는 전주·전월은 모든 행에서 「—」로 남았다.
   */
  watchlistMetrics?: WatchlistMetrics;
};

export type SampleHoldingRow = {
  id: number;
  symbol: string;
  name?: string;
  quantity: number;
  avgPrice?: number;
  costCurrency?: string;
  source: string;
  updatedAt: string;
  currency: string;
  price: number;
  changeRate: number;
  sector?: string;
  sectorLabel?: string;
};

export type SampleTargetRow = {
  id: number;
  symbol: string;
  targetPrice: number;
  targetPrice2?: number;
  stopPrice?: number;
  basePrice?: number;
  baseDate?: string;
  reachedDate?: string;
  peakPrice?: number;
  troughPrice?: number;
  currency: string;
  note: string;
  createdAt: string;
  updatedAt: string;
  name: string;
  price: number;
  changeRate: number;
};

/** 메모 픽스처 + 그 종목의 현재가. 예시 모드에서는 실시간 시세를 섞지 않는다. */
export type SampleJournalRow = JournalNote & { price?: number };

export async function fetchSampleStamps(): Promise<Stamp[]> {
  return (await apiRequest<Stamp[] | null>("/api/v1/sample/stamps")) ?? [];
}

export async function fetchSampleStampSummary(): Promise<StampSummary> {
  return apiRequest<StampSummary>("/api/v1/sample/stamps/summary");
}

export async function fetchSampleWatchlist(): Promise<SampleWatchlistRow[]> {
  return (await apiRequest<SampleWatchlistRow[] | null>("/api/v1/sample/watchlist")) ?? [];
}

export async function fetchSampleHoldings(): Promise<SampleHoldingRow[]> {
  return (await apiRequest<SampleHoldingRow[] | null>("/api/v1/sample/holdings")) ?? [];
}

export async function fetchSampleJournal(): Promise<SampleJournalRow[]> {
  return (await apiRequest<SampleJournalRow[] | null>("/api/v1/sample/journal/notes")) ?? [];
}

export async function fetchSampleTargets(): Promise<SampleTargetRow[]> {
  return (await apiRequest<SampleTargetRow[] | null>("/api/v1/sample/price-targets")) ?? [];
}

/**
 * 관심 종목 화면은 목록·시세·점수를 **세 요청**으로 나눠 받는다. 셋 다 인증 뒤에
 * 있어서 예시 모드에서는 부를 수 없으므로, 한 픽스처에서 세 모양을 만들어 낸다.
 * 화면 쪽 조립 함수(`buildWatchlistRows`)는 그대로 두는 것이 핵심이다 — 예시 전용
 * 조립 경로를 따로 만들면 실제 화면과 다르게 그려질 수 있다.
 */
export function sampleWatchlistParts(rows: SampleWatchlistRow[]) {
  const items = rows.map((r) => ({
    id: r.id,
    symbol: r.symbol,
    createdAt: r.createdAt,
    addedPrice: r.addedPrice,
  }));

  const quotes = rows.map((r) => {
    /*
     * 전일 종가는 서버 메트릭이 우선이다. 없을 때만 등락률에서 되돌려 만든다 —
     * 행은 현재가·변화폭·등락률 셋을 함께 그려서(`change`가 없으면 현재가 칸이
     * NaN이 된다) 어느 하나만 비워둘 수 없기 때문이다.
     */
    const metrics = r.watchlistMetrics;
    const prevClose =
      metrics?.prevClose && metrics.prevClose > 0
        ? metrics.prevClose
        : r.changeRate === -100
          ? r.price
          : r.price / (1 + r.changeRate / 100);
    return {
      symbol: r.symbol,
      name: r.name,
      currentPrice: r.price,
      changeRate: r.changeRate,
      prevClose,
      change: r.price - prevClose,
      currency: r.currency === "USD" ? ("USD" as const) : ("KRW" as const),
      market: r.market,
      // 서버가 준 블록을 그대로 넘긴다. 여기서 다시 만들면 로그인 화면과 예시 화면이
      // 서로 다른 계산을 쓰게 되고, 그 차이는 조용히 벌어진다.
      watchlistMetrics: metrics ?? {
        addedAt: r.createdAt,
        changeRatePrevClose: r.changeRate,
        prevClose,
        week52High: r.week52High,
        week52Low: r.week52Low,
      },
    };
  });

  const scores = new Map<string, { score: number | null; grade?: string }>(
    rows.map((r) => [r.symbol.toUpperCase(), { score: r.score ?? null, grade: r.grade }]),
  );

  return { items, quotes, scores };
}
