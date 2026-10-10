import { apiRequest } from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";

/**
 * 종목 타임라인 — `be/internal/model/stock_timeline.go`의 거울.
 *
 * # 화면이 반드시 지켜야 할 한 가지
 *
 * **레인마다 기록 시작일이 다르다.** 실측(005930): 주가는 2024-12-16부터인데 공시는
 * 2026-08-20부터다. 1년을 그리면 공시 레인이 열 달 비는데, 그것은 「아무 일 없었다」가
 * 아니라 **「아직 수집하기 전」**이다. `coversWindow`가 false인 레인은 그 사실을
 * 화면이 말해야 한다 (요구 8).
 *
 * # 뉴스는 건수만 온다
 *
 * 삼성전자는 하루 평균 180건이다. 90일 창에 제목까지 실으면 응답이 메가바이트가 된다.
 * 사람이 하루를 펼치면 `fetchTimelineDayNews`가 그때 답한다.
 */

export type TimelineLaneID =
  | "close"
  | "score"
  | "earnings"
  | "news"
  | "insider"
  | "disclosure";

export type TimelineLane = {
  id: TimelineLaneID;
  label: string;
  /** 이 종목 **전체 기록**의 첫 날. 비어 있으면 자료가 한 건도 없다. */
  availableFrom?: string;
  /** 요청한 창 안의 건수. */
  count: number;
  /** 기록이 창보다 먼저 시작했는가. false면 그 앞은 「수집 전」이다. */
  coversWindow: boolean;
};

export type TimelinePoint = {
  date: string;
  close: number;
  changeRate?: number;
};

export type TimelineScore = {
  value: number | null;
  delta?: number;
  /** Phase 1 귀인이 만든 한 문장 — 「추세 19.2 → 26.9」. */
  driver?: string;
  driverKind?: string;
};

export type TimelineEarnings = {
  period: string;
  session?: string;
  /** "actual"(발표치) | "estimate"(예정·추정) */
  status: string;
  revenueActual?: number;
  revenueEstimate?: number;
  opIncomeActual?: number;
  epsActual?: number;
  epsEstimate?: number;
  /** 추정치가 없으면 **없다** — 0%로 채우면 「예상대로였다」가 된다. */
  surprise?: number;
  currency?: string;
};

export type TimelineInsider = {
  count: number;
  buyShares: number;
  sellShares: number;
  netShares: number;
};

export type TimelineDisclosure = {
  title: string;
  receiptNo?: string;
  filer?: string;
};

export type TimelineDay = {
  date: string;
  close?: number;
  changeRate?: number;
  significant: boolean;
  /** 이 날이 왜 목록에 있는지 — 「급등」·「실적 발표」·「공시 2건」… */
  reasons?: string[];
  score?: TimelineScore;
  earnings?: TimelineEarnings;
  news?: { count: number };
  insider?: TimelineInsider;
  disclosures?: TimelineDisclosure[];
};

export type StockTimeline = {
  symbol: string;
  name?: string;
  currency?: string;
  fromDate: string;
  toDate: string;
  closes: TimelinePoint[];
  days: TimelineDay[];
  lanes: TimelineLane[];
  /** 「큰 등락」의 문턱(%). 종목마다 다르다 — 화면이 그 값을 밝힌다. */
  moveThreshold: number;
  /** 문턱의 근거 — 이 창의 일간 등락률 표준편차(%). */
  volatility: number;
};

export type TimelineNewsItem = {
  id: string;
  headline: string;
  url: string;
  publisher?: string;
  publishedAt: string;
  viewCount: number;
};

export type TimelineDayNews = {
  symbol: string;
  date: string;
  items: TimelineNewsItem[];
};

/** 기간 손잡이 — 길수록 「수집 전」 구간이 넓어진다. */
export const TIMELINE_RANGES: { id: number; label: string }[] = [
  { id: 90, label: "90일" },
  { id: 180, label: "180일" },
  { id: 365, label: "1년" },
];

/**
 * 한 종목의 타임라인. 실측 90일 0.42초 · 16.7KB.
 *
 * 하루 한 번 도는 배치의 산물이라 같은 방문 안에서 다시 물어도 값이 같다 — 탭을
 * 오가며 다시 마운트되는 자리라 캐시가 필요하다.
 */
export function fetchStockTimelineClient(
  symbol: string,
  days: number,
): Promise<StockTimeline | null> {
  const path = `/api/v1/stocks/${encodeURIComponent(symbol)}/timeline?days=${days}`;
  return cachedGet(
    path,
    () => apiRequest<StockTimeline | null>(path, undefined, "타임라인 조회 실패"),
    60_000,
  );
}

/**
 * 하루를 펼쳤을 때의 기사 목록.
 *
 * 많이 본 순, 같으면 **이른 시각 순**이다 — 이 DB의 조회수는 사실상 전부 0이라
 * 「많이 본 순」만으로는 그날 마지막 몇 분만 나온다(서버 쪽 주석에 실측이 있다).
 */
export function fetchTimelineDayNewsClient(
  symbol: string,
  date: string,
  limit = 12,
): Promise<TimelineDayNews | null> {
  const path = `/api/v1/stocks/${encodeURIComponent(symbol)}/timeline/news?date=${date}&limit=${limit}`;
  return cachedGet(
    path,
    () => apiRequest<TimelineDayNews | null>(path, undefined, "뉴스 조회 실패"),
    5 * 60_000,
  );
}

/* ------------------------------------------------------------------------- */
/* 표시 규칙                                                                  */
/* ------------------------------------------------------------------------- */

/**
 * 등락률 — **0에는 부호를 붙이지 않는다.**
 *
 * `(v > 0 ? '+' : '−')`로 쓰면 0.00이 「−0.00%」가 된다(목업 실측). 작은 실수지만
 * 금융 화면에서 음수 부호는 뜻이 있다.
 */
export function formatRate(value: number | undefined): string {
  if (typeof value !== "number") return "—";
  if (Math.abs(value) < 0.005) return "0.00%";
  return `${value > 0 ? "+" : "−"}${Math.abs(value).toFixed(2)}%`;
}

/**
 * 주식 수 — 1,164주.
 *
 * **매수·매도는 크기이지 방향이 아니다.** 그래서 부호를 붙이지 않는다(실측에서
 * 「매도 +381주」가 나왔다 — 판 것을 더한 것처럼 읽힌다). 방향이 있는 것은 순증감뿐이고,
 * 그때만 `formatSignedShares`를 쓴다.
 */
export function formatShares(value: number): string {
  return `${Math.abs(Math.round(value)).toLocaleString("ko-KR")}주`;
}

/** 순증감 — 방향이 뜻을 갖는 유일한 자리. */
export function formatSignedShares(value: number): string {
  const abs = Math.abs(Math.round(value));
  if (abs === 0) return "0주";
  return `${value < 0 ? "−" : "+"}${abs.toLocaleString("ko-KR")}주`;
}

export type TimelineReasonTone = "up" | "down" | "earnings" | "score" | "plain";

/**
 * 사건 칩의 색.
 *
 * 급등·급락만 상승/하락 색을 쓴다. 나머지를 그 색으로 칠하면 「공시가 있었다」가
 * 「올랐다」로 읽힌다 — 이 앱에서 빨강·파랑은 방향이다.
 */
export function reasonTone(reason: string): TimelineReasonTone {
  if (reason === "급등") return "up";
  if (reason === "급락") return "down";
  if (reason === "실적 발표") return "earnings";
  if (reason.startsWith("AI 점수")) return "score";
  return "plain";
}
