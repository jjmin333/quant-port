import { getApiBaseUrl } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import { fetchWithDeadline } from "@/lib/fetch-timeout";
import type { StockSeoSummary } from "@/lib/stock-seo";
import type { StockQA } from "@/lib/stock-qa";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import type { EarningsTabData } from "@/lib/earnings-types";
import type { StockSummaryData } from "@/lib/stock-summary-types";
import type { StockAxes } from "@/lib/stock-style";

/**
 * 종목 페이지를 서버에서 그릴 재료 (2026-09-24 사용자 결정).
 *
 * **한 시간 접는다.** 크롤러가 종목 주소 천 개를 차례로 열어도 종목당 BE 호출은 시간당 한
 * 번이다(OCI 무료 인스턴스 · DB 1 ECPU). 점수는 하루 한 번 바뀌므로 한 시간 늦어도 틀리지
 * 않는다. 실패는 null — 화면은 원래대로 클라이언트가 그리고 색인만 빠진다.
 */
const SEO_REVALIDATE = 3600;

export async function fetchStockSeoSummary(symbol: string): Promise<StockSeoSummary | null> {
  try {
    const res = await fetchWithDeadline(
      `${getApiBaseUrl()}/api/v1/seo/stocks/${encodeURIComponent(symbol)}`,
      { next: { revalidate: SEO_REVALIDATE } },
    );
    if (!res.ok) return null;
    const body = (await res.json()) as ApiResponse<StockSeoSummary | null>;
    return body.data ?? null;
  } catch {
    return null;
  }
}

/** 색인 대상 종목 한 줄 — BE `/seo/stocks`의 `stocks`. */
export type IndexableStock = {
  symbol: string;
  name: string;
  assetType: string;
  market: string;
  sectorLabel?: string;
  grade?: string;
};

/**
 * 색인 대상 종목(시장별 최근 20거래일 거래대금 상위 500씩, 2026-10-09). 사이트맵·`/stocks` 허브·
 * 종목 페이지의 같은 섹터 링크가 함께 읽는다. 실패하면 빈 목록.
 */
export async function fetchIndexableStocks(): Promise<IndexableStock[]> {
  try {
    // 사람이 기다리는 화면만 읽는 게 아니다 — BE 목록이 차가울 때(약 4초)도 받도록 넉넉히.
    const res = await fetchWithDeadline(
      `${getApiBaseUrl()}/api/v1/seo/stocks`,
      { next: { revalidate: SEO_REVALIDATE } },
      10_000,
    );
    if (!res.ok) return [];
    const body = (await res.json()) as ApiResponse<{ stocks?: IndexableStock[] }>;
    return body.data?.stocks ?? [];
  } catch {
    return [];
  }
}

async function seoGet<T>(path: string): Promise<T | null> {
  try {
    const res = await fetchWithDeadline(`${getApiBaseUrl()}${path}`, { next: { revalidate: SEO_REVALIDATE } });
    if (!res.ok) return null;
    const body = (await res.json()) as ApiResponse<T>;
    return body.data ?? null;
  } catch {
    return null;
  }
}

/**
 * 종목 페이지의 접힌 요약 바 재료(2026-10-09) — 최근 변화(`/qa`)와 재무 지표(`/fundamentals`).
 * 요약 한 줄과 같은 1시간 캐시, 실패는 null(그 칸만 빠진다).
 */
export async function fetchStockSeoDetails(symbol: string): Promise<{
  qa: StockQA | null;
  fundamentals: FundamentalsData | null;
  earnings: EarningsTabData | null;
  overview: string | null;
}> {
  const s = encodeURIComponent(symbol);
  // 차례로 부른다 — 데이터 캐시가 빈 냉시동에 revalidate fetch 둘을 동시에 부르면 첫 요청이
  // 깨지는 일이 홈에서 있었다(09-22). 둘 다 캐시되면 몇 ms라 순차의 비용은 없다.
  const qa = await seoGet<StockQA>(`/api/v1/stocks/${s}/qa`);
  const fundamentals = await seoGet<FundamentalsData>(`/api/v1/stocks/${s}/fundamentals`);
  // 「삼성전자 실적 발표일」류 검색 — 다음 발표일·지난 서프라이즈를 서버 HTML에(2026-10-10, 성장 보고서 기회 2위).
  const earnings = await seoGet<EarningsTabData>(`/api/v1/stocks/${s}/earnings`);
  // 저장된 AI 요약 문단(배치가 쓴 것, 조회 경로 LLM 아님) — 종목마다 다른 글이라 색인 본문으로 가장 값지다(2026-10-10).
  const summary = await seoGet<StockSummaryData>(`/api/v1/stocks/${s}/summary`);
  return { qa, fundamentals, earnings, overview: summary?.overview?.trim() || null };
}

/** 색인 대상 종목 한 줄 + 점수 — 업종·스타일 목록 페이지의 재료. */
export type IndexableScored = IndexableStock & { score: number; grade?: string; axes: StockAxes; peerPct?: number };

/**
 * 색인 대상 1,000종목의 점수·축(2026-10-10). `/stocks/scores/batch`가 한 번에 200개라 다섯 번 부른다.
 * 응답이 무거워(inputs 포함 종목당 약 2KB) 묶음마다 따로 1시간 캐시한다 — Next 데이터 캐시는 2MB를 넘는
 * 응답을 저장하지 않는다. 점수가 없는 종목은 뺀다.
 */
export async function fetchIndexableScored(): Promise<IndexableScored[]> {
  const stocks = await fetchIndexableStocks();
  const out: IndexableScored[] = [];
  for (let i = 0; i < stocks.length; i += 200) {
    const chunk = stocks.slice(i, i + 200);
    const views = await seoGet<{ symbol: string; score: number; grade?: string; peerPct?: number; breakdown?: StockAxes }[]>(
      `/api/v1/stocks/scores/batch?symbols=${chunk.map((c) => encodeURIComponent(c.symbol)).join(",")}`,
    );
    const bySymbol = new Map((views ?? []).map((v) => [v.symbol, v]));
    for (const c of chunk) {
      const v = bySymbol.get(c.symbol);
      if (v && typeof v.score === "number") {
        out.push({ ...c, score: v.score, grade: v.grade ?? c.grade, axes: v.breakdown ?? {}, peerPct: v.peerPct });
      }
    }
  }
  return out;
}
