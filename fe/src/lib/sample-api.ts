import { apiRequest } from "@/lib/api-client";
import type { JournalNote } from "@/lib/journal-types";
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
    // 전일 종가와 변화폭은 등락률에서 되돌려 만든다. 행은 셋을 함께 그리므로
    // (`change`가 없으면 현재가 칸이 NaN이 된다) 어느 하나만 채우면 안 된다.
    const prevClose = r.changeRate === -100 ? r.price : r.price / (1 + r.changeRate / 100);
    return {
      symbol: r.symbol,
      name: r.name,
      currentPrice: r.price,
      changeRate: r.changeRate,
      prevClose,
      change: r.price - prevClose,
      currency: r.currency === "USD" ? ("USD" as const) : ("KRW" as const),
      market: r.market,
      watchlistMetrics: {
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
