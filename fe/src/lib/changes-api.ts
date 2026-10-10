import { apiRequest } from "@/lib/api-client";

/**
 * 「변화」 메뉴 — 점수 배치(매일 00:25)가 끝에서 골라 둔 그날의 변화를 읽는다.
 *
 * 서버는 **저장분만** 준다(계산 없음, LLM 없음). 규칙과 문턱은
 * `be/internal/service/change_detect.go` 머리글에 있다.
 */

export type ChangeKind =
  | "price_move"
  | "volume_surge"
  | "score_jump"
  | "news_event"
  | "disclosure"
  | "earnings"
  | "insider"
  | "social";

export type ChangeEvent = {
  detectedFor: string;
  eventDate: string;
  symbol: string;
  name?: string;
  assetType?: "domestic" | "us" | string;
  kind: ChangeKind;
  subtype?: string;
  direction: -1 | 0 | 1;
  magnitude?: number;
  importance: number;
  /** 감지한 날의 시가총액(원 · 달러). 목록 순서가 이것이다. 없으면 맨 뒤. */
  marketCap?: number;
  title: string;
  /** 급등락·거래량에만 있다. false면 같은 날 사건도 기사도 없었다. */
  explained?: boolean;
  detail?: {
    thresholdPct?: number;
    volatilityPct?: number;
    newsCount?: number;
    subtypeLabel?: string;
    [key: string]: unknown;
  };
};

export type ChangesResponse = {
  date: string;
  dates: string[];
  /** count = 이 날·시장·범위에서 그 종류가 있는 종목 수(선택과 무관). */
  kinds: { id: string; label: string; count?: number }[];
  items: ChangeEvent[];
};

export type ChangesQuery = {
  date?: string;
  /** 「전체」는 없다(2026-10-09 — 순위가 시가총액이라 원·달러를 한 줄에 세우지 않는다). */
  market: "domestic" | "us";
  kinds?: ChangeKind[];
  /** 관심·보유 종목만 — 로그인이 필요하다. */
  mine?: boolean;
};

export async function fetchChanges(q: ChangesQuery): Promise<ChangesResponse> {
  const params = new URLSearchParams();
  if (q.date) params.set("date", q.date);
  params.set("market", q.market);
  if (q.kinds && q.kinds.length > 0) params.set("kinds", q.kinds.join(","));
  const path = q.mine ? "/api/v1/user/changes" : "/api/v1/changes";
  const qs = params.toString();
  // apiRequest가 `{ data }` 봉투를 이미 벗긴다.
  const res = await apiRequest<ChangesResponse>(qs ? `${path}?${qs}` : path);
  return { ...res, dates: res.dates ?? [], items: res.items ?? [], kinds: res.kinds ?? [] };
}

export type ChangeGroup = {
  symbol: string;
  name: string;
  assetType?: string;
  importance: number;
  marketCap: number;
  /** 1부터 — 이 목록(날 · 시장 · 고른 필터) 안의 시가총액 순번. */
  rank: number;
  events: ChangeEvent[];
  hasUnexplained: boolean;
};

/**
 * 종목 단위로 묶는다 — 같은 종목의 급등 · 거래량 · 뉴스 사건이 세 줄로 흩어지면 그것이
 * 한 가지 일이라는 것이 안 보인다. 묶음의 순서는 **시가총액**(2026-10-09 요청 3)이고
 * 순위 번호를 단다. 묶음 안의 줄은 중요도 순.
 */
export function groupChanges(items: ChangeEvent[]): ChangeGroup[] {
  const by = new Map<string, ChangeGroup>();
  for (const e of items) {
    let g = by.get(e.symbol);
    if (!g) {
      g = {
        symbol: e.symbol,
        name: e.name || e.symbol,
        assetType: e.assetType,
        importance: 0,
        marketCap: 0,
        rank: 0,
        events: [],
        hasUnexplained: false,
      };
      by.set(e.symbol, g);
    }
    g.events.push(e);
    g.importance = Math.max(g.importance, e.importance);
    g.marketCap = Math.max(g.marketCap, e.marketCap ?? 0);
    if (e.explained === false && e.kind === "price_move") g.hasUnexplained = true;
  }
  const groups = [...by.values()];
  for (const g of groups) g.events.sort((a, b) => b.importance - a.importance);
  groups.sort((a, b) => b.marketCap - a.marketCap || a.symbol.localeCompare(b.symbol));
  groups.forEach((g, i) => (g.rank = i + 1));
  return groups;
}

export type MyChangeSummary = {
  date: string;
  /** 오늘 변화가 있는 내 관심·보유 **종목 수**(이벤트 수가 아니다). */
  symbols: number;
  items: {
    symbol: string;
    name: string;
    title: string;
    direction: -1 | 0 | 1;
    unexplained?: boolean;
  }[];
};

export async function fetchMyChangeSummary(): Promise<MyChangeSummary> {
  const res = await apiRequest<MyChangeSummary>("/api/v1/user/changes/summary");
  return { ...res, items: res.items ?? [] };
}
