import { apiRequest } from "@/lib/api-client";

/**
 * 종목 화면의 애널리스트·레딧 (2026-09-28 사용자 결정 — 요청 6·7). 배치가 채운 표만 읽는다.
 *
 * 애널리스트: 해외 = Finnhub 추천 추이(명수, 목표가 없음) · 국내 = 네이버 증권 평균 목표가·의견.
 * 의견 `rating`은 둘 다 1(강력매도)~5(강력매수).
 */
export type AnalystConsensus = {
  symbol: string;
  source: "finnhub" | "naver" | string;
  asOf?: string;
  rating?: number;
  targetMean?: number;
  targetCurrency?: string;
  fetchedAt: string;
  /** 강력매수·매수·보유·매도·강력매도 명수(해외만). */
  counts?: [number, number, number, number, number];
};

export type SocialDay = {
  date: string;
  mentions: number;
  mentionsPrev: number;
  upvotes: number;
  rank: number;
  rankPrev?: number;
};

export type SocialSubreddit = { source: string; mentions: number; mentionsPrev: number; upvotes: number };

/** ApeWisdom 종목 페이지의 24시간 요약 (2026-09-29). */
export type RedditInsight = {
  date: string;
  users?: number;
  /** 긍정 비율 0..100 */
  sentimentPct?: number;
  wsbPct?: number;
  keywords?: { word: string; n: number }[];
};

export type RedditPost = {
  id: string;
  subreddit: string;
  title: string;
  titleKo?: string;
  snippet?: string;
  snippetKo?: string;
  url: string;
  publishedAt: string;
};

export type CommunityBody = {
  summary: string;
  tags: string[];
  bull?: string[];
  bear?: string[];
  stance?: string;
};

/** 커뮤니티 점수(-100..100, 규칙) + AI 요약(배치, 상위 40종목만). */
export type RedditCommunity = {
  score: number;
  stance: "낙관" | "관망" | "비관" | string;
  date: string;
  source?: "llm" | "rule";
  body?: CommunityBody;
  /** 글이 점수보다 이전 날 것일 때 그 날짜. */
  bodyDate?: string;
};

export type RedditData = {
  insight: RedditInsight | null;
  posts: RedditPost[];
  community: RedditCommunity | null;
};

export type StockSignals = {
  analyst: AnalystConsensus | null;
  social: SocialDay[];
  subreddits: SocialSubreddit[];
  reddit: RedditData | null;
};

export async function fetchStockSignals(symbol: string): Promise<StockSignals> {
  const data = await apiRequest<Partial<StockSignals> | null>(
    `/api/v1/stocks/${encodeURIComponent(symbol)}/signals`,
    undefined,
    "애널리스트·레딧 자료를 불러오지 못했습니다",
  );
  return {
    analyst: data?.analyst ?? null,
    social: data?.social ?? [],
    subreddits: data?.subreddits ?? [],
    reddit: data?.reddit ? { ...data.reddit, posts: data.reddit.posts ?? [] } : null,
  };
}

/** 의견 평균을 말로. 네이버·Finnhub 공통 척도. */
export function ratingLabel(r: number): string {
  if (r >= 4.5) return "강력 매수";
  if (r >= 3.5) return "매수";
  if (r >= 2.5) return "보유";
  if (r >= 1.5) return "매도";
  return "강력 매도";
}

/* ------------------------------------------------------------------------- */
/* 미국 개미 분위기 (2026-10-01 요청 3) — Reddit 시장 단위 집계                 */
/* ------------------------------------------------------------------------- */

export type MarketMood = {
  date: string;
  /** -100..100 규칙 점수(지수 ETF 감성 60% + 언급 상위 종목 40%). 표본이 작으면 없음. */
  score?: number;
  stance?: string;
  sentimentPct?: number;
  users: number;
  mentions: number;
  mentionsPrev: number;
  bulls: number;
  bears: number;
  etfs: { symbol: string; mentions: number; users?: number; sentimentPct?: number }[];
  surges: { symbol: string; name: string; mentions: number; mentionsPrev: number }[];
  subreddits: SocialSubreddit[];
  history: { date: string; mentions: number; score?: number }[];
  posts: RedditPost[];
  summary?: CommunityBody;
  summaryDate?: string;
  summaryAi?: boolean;
};

export async function fetchMarketMood(): Promise<MarketMood | null> {
  return apiRequest<MarketMood | null>("/api/v1/social/market-mood", undefined, "레딧 분위기를 불러오지 못했습니다");
}
