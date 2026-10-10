import { apiRequest } from "@/lib/api-client";

/**
 * 홈 「내 종목」 카드 (2026-09-26 사용자 결정 — 재방문 개선 C).
 *
 * 서버는 관심·보유 합집합과 **저장된** 신호만 준다: 점수 두 날의 차이, 최근 24시간
 * 기사 수, 그날 변화 감지의 대표 한 줄. 가격은 `useLiveQuotes`, 실적 일정은
 * `useUpcomingEvents`가 따로 채운다 — 같은 값을 두 경로로 받지 않기 위해서다.
 * 규칙은 `be/internal/repository/my_today.go` 머리글.
 */

export type MyTodayItem = {
  symbol: string;
  name: string;
  assetType: string;
  held: boolean;
  watched: boolean;
  score?: number;
  /** 전 점수일 대비. 어제 점수가 없던 종목은 비어 있다(0이 아니다). */
  scoreDelta?: number;
  news24h: number;
  change?: { title: string; direction: number; unexplained?: boolean };
  /** 저장된 AI 요약 글 앞부분(최대 300자) — 화면은 첫 문장만 쓴다. LLM 호출 없음. */
  aiOverview?: string;
  /** 그 글이 쓰인 날(YYYY-MM-DD). */
  aiAt?: string;
};

export type MyToday = {
  scoreDate: string;
  changeDate: string;
  items: MyTodayItem[];
};

export async function fetchMyToday(): Promise<MyToday> {
  const res = await apiRequest<MyToday>("/api/v1/user/today");
  return { ...res, items: res.items ?? [] };
}
