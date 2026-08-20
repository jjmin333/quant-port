import { getApiBaseUrl, mergeFetchInit, parseJson } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";

/**
 * 「AI 시장 브리핑」은 **읽기 전용**이다.
 *
 * 생성은 서버의 배치(장중 30분 · 그 외 1시간)와 관리자 수동 실행만 한다 — 화면이
 * 열릴 때 만들면 하루 LLM 비용이 방문자 수에 비례하기 때문이다. 그래서 여기에는
 * "생성" 요청이 없고, 아직 만들어진 게 없으면 `null`이 온다.
 */

export type NewsBriefingMood = "강세" | "중립" | "약세";

export type NewsBriefingIndex = {
  id: string;
  name: string;
  value: number;
  changeRate: number;
};

export type NewsBriefingMention = {
  symbol: string;
  name: string;
  mentions: number;
  changeRate: number;
  /** false면 시세를 붙이지 못한 것 — 0%로 그리면 "보합"으로 읽힌다. */
  hasQuote: boolean;
};

export type NewsBriefingFacts = {
  session: string;
  articleCount: number;
  breakingCount: number;
  indices?: NewsBriefingIndex[];
  topMentioned?: NewsBriefingMention[];
};

export type NewsMarketBriefing = {
  market: "domestic" | "us";
  generatedAt: string;
  /** "llm"이면 AI가 쓴 문장, "rules"면 집계만으로 만든 문장. 배지로 구분해 보여준다. */
  source: "llm" | "rules";
  summary: string;
  mood?: NewsBriefingMood;
  moodReason?: string;
  highlights?: string[];
  watchPoints?: string[];
  facts: NewsBriefingFacts;
};

export async function fetchNewsBriefing(
  market: "domestic" | "us",
): Promise<NewsMarketBriefing | null> {
  const res = await fetch(
    `${getApiBaseUrl()}/api/v1/news/${market}/briefing`,
    mergeFetchInit({ cache: "no-store" }),
  );
  if (!res.ok) {
    throw new Error("브리핑을 불러오지 못했습니다");
  }
  const body = await parseJson<ApiResponse<NewsMarketBriefing | null>>(res);
  return body.data ?? null;
}
