import { fetchWithDeadline } from "@/lib/fetch-timeout";
import { mergeMarketIndices } from "@/lib/market-indices-catalog";
import type { IndexGroupMeta, MarketIndex } from "@/lib/quotes-types";
import type { StockScoreListItem, ScoreRankingMarket } from "@/lib/stock-score-api";

/**
 * 홈·분석 화면의 **첫 화면을 서버에서 미리 받아 온다**
 * (2026-09-20 SSR/CSR 점검 요청 1·5·9).
 *
 * # 무엇이 잘못돼 있었나
 *
 * `/news/domestic`은 이미 이렇게 하고 있었다(`news-api.server.ts`). 홈과 `/analysis`는
 * 아니었고, 그래서 두 화면의 서버 HTML에는 **데이터가 한 글자도 없었다.** 실측
 * (2026-09-20, Googlebot UA):
 *
 *   `/`         → 「오늘의 시장 **갱신 중…** / AI 추천 랭킹 TOP / 거래대금 상위 …」
 *   `/analysis` → 「**랭킹 로딩 중…** / 시세 로딩 중… / 점수 불러오는 중…」
 *
 * 제목과 구획 이름만 있고 **종목명도 지수도 숫자도 없다.** 크롤러에게 이 두 문서는
 * 목차이지 내용이 아니다. 같은 이유로, JS가 늦거나 막힌 환경에서 사용자가 처음
 * 보는 것도 스켈레톤이다.
 *
 * # 왜 쿠키를 안 보내는가
 *
 * 여기서 받는 것은 **공개 데이터**다. 실측으로 확인했다 — 지수·점수 랭킹 모두
 * 비로그인 `curl`에 200을 준다. 쿠키를 붙이면 ⓐ Next의 데이터 캐시가 한 사용자의
 * 쿠키로 받은 응답을 모두에게 돌려줄 위험이 생기고, ⓑ 응답이 사용자마다 달라질
 * 이유가 없는데 캐시가 사용자 수만큼 갈라진다. `serverApiRequest`(쿠키 전달)를
 * 안 쓰고 직접 부르는 이유가 이것이다.
 *
 * 관심·보유 표시처럼 **사용자마다 다른 것은 선반입하지 않는다** — 그건 마운트
 * 뒤에 클라이언트가 얹는다(`use-score-rankings`의 `enrich`). 서버가 그것까지
 * 그리면 hydration이 어긋난다.
 *
 * # 실패는 화면을 막지 않는다
 *
 * 전부 `catch`로 빈 값을 돌려준다. 선반입은 **먼저 보여 주는 일**이지 화면이
 * 서 있을 조건이 아니다 — 클라이언트가 어차피 마운트 직후 다시 받는다. 상한도
 * 3초로 짧다(`SSR_TIMEOUT_MS`): 여기서 기다리는 시간은 TTFB에 그대로 더해진다.
 */

/** 서버 안에서는 nginx를 거치지 않고 BE로 직접 간다. */
function apiBase(): string {
  return process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
}

export type ServerIndicesSeed = {
  indices: MarketIndex[];
  updatedAt: string | null;
  checkedAt?: string | null;
  groups: Record<string, IndexGroupMeta>;
};

export const EMPTY_INDICES_SEED: ServerIndicesSeed = {
  indices: [],
  updatedAt: null,
  groups: {},
};

/**
 * 지수 스냅샷 선반입. 재검증 30초 — 클라이언트 폴링 주기(`POLL_MS`)와 같다.
 *
 * 더 길게 잡으면 **첫 화면이 폴링보다 낡은 값으로 시작**하고, 사용자는 화면이
 * 뜨자마자 숫자가 한 번 바뀌는 것을 본다. 더 짧게 잡을 이유는 없다 — 뒤에서
 * 값을 만드는 주기가 그것이다.
 */
export async function fetchIndicesServer(): Promise<ServerIndicesSeed> {
  try {
    const res = await fetchWithDeadline(`${apiBase()}/api/v1/quotes/indices`, {
      next: { revalidate: 30 },
    });
    if (!res.ok) return EMPTY_INDICES_SEED;
    const body = (await res.json()) as {
      data: MarketIndex[] | null;
      updatedAt?: string;
      checkedAt?: string;
      groups?: Record<string, IndexGroupMeta>;
    };
    return {
      // 자리표시자 병합은 클라이언트와 **같은 함수**를 쓴다 — 다르면 그 차이가
      // 곧 hydration 불일치다.
      indices: mergeMarketIndices(body.data ?? []),
      updatedAt: body.updatedAt ?? null,
      checkedAt: body.checkedAt ?? null,
      groups: body.groups ?? {},
    };
  } catch {
    return EMPTY_INDICES_SEED;
  }
}

export type ServerScoreSeed = {
  items: StockScoreListItem[];
  tradeDate: string;
};

export const EMPTY_SCORE_SEED: ServerScoreSeed = { items: [], tradeDate: "" };

/**
 * AI 추천 랭킹 선반입.
 *
 * 재검증 60초. 순위·점수를 만드는 것은 하루 한 번 도는 배치지만 **등락률은
 * 장중에 서버가 실시간 시세로 덮으므로**(`stock-score-api.ts` 주석) 하루를
 * 통째로 캐시할 수는 없다. 클라이언트 캐시가 30초라 그보다 길게 두어, 마운트
 * 직후의 클라이언트 요청이 선반입값을 **덮어쓰는 쪽**이 되게 한다.
 */
export async function fetchScoreTopServer(
  market: ScoreRankingMarket,
  limit: number,
): Promise<ServerScoreSeed> {
  /*
    **숫자가 아닌 limit은 여기서 막는다** (2026-09-20 실측).

    타입은 `number`인데 런타임에 `undefined`가 들어온 적이 있다 — 상수를
    `"use client"` 모듈에서 가져왔더니 서버 그래프에서 클라이언트 참조로 바뀌었다
    (`lib/dashboard-rows.ts` 머리글). 그때 `String(undefined)`가 쿼리에 실려 나갔고
    BE는 그 값을 무시하고 기본 20건을 줬다 — **컴파일도 lint도 통과하고 요청만
    조용히 틀렸다.** 홈 카드가 일곱 줄이 아니라 스무 줄로 섰다.

    타입이 못 잡는 자리라 런타임이 잡는다. 던지는 이유는 이 값이 틀리면 화면의
    줄 수가 틀리기 때문이다 — 아래 `catch`가 받아 빈 씨앗으로 떨어지므로
    페이지는 그대로 서고, 로그에는 남는다.
  */
  if (!Number.isInteger(limit) || limit <= 0) {
    throw new Error(`fetchScoreTopServer: limit이 양의 정수가 아니다 (${limit})`);
  }

  try {
    const params = new URLSearchParams({ market, limit: String(limit) });
    const res = await fetchWithDeadline(
      `${apiBase()}/api/v1/stocks/scores/top?${params}`,
      { next: { revalidate: 60 } },
    );
    if (!res.ok) return EMPTY_SCORE_SEED;
    const body = (await res.json()) as { data: StockScoreListItem[] | null };
    const items = body.data ?? [];
    return { items, tradeDate: items[0]?.tradeDate ?? "" };
  } catch {
    return EMPTY_SCORE_SEED;
  }
}
