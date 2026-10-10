"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { isPageVisible } from "@/hooks/use-page-visible";
import { fetchUserHoldingsClient } from "@/lib/holdings-import-api";
import {
  MARKET_RADAR_DEFAULT_COUNT,
  MARKET_RADAR_PRICE_POLL_MS,
  marketRadarParamsKey,
  marketRadarSearchParams,
  parseMarketRadarParams,
  parseRankingNumber,
} from "@/lib/market-radar-config";
import { useMarketDataSettings } from "@/hooks/use-market-data-settings";
import { useQuoteStream } from "@/hooks/use-quote-stream";
import { changeRateFrom, type ListPriceMap } from "@/lib/quote-stream-state";
import { PRICE_POLL_MS_WHILE_STREAMING } from "@/lib/price-cadence";
import type {
  EnrichedRankingItem,
  MarketRadarParams,
  RankingPriceRow,
  RankingsResponse,
} from "@/lib/market-radar-types";
import type { StockInfo } from "@/lib/quotes-types";
import { fetchRankingPricesClient, fetchRankingsClient } from "@/lib/market-api";
import {
  ensureWatchlist,
  getWatchlistSymbols,
  setWatchlistLocal,
  subscribeWatchlist,
} from "@/lib/watchlist-store";
import { toUserMessage } from "@/lib/error-message";

function computeRankDeltas(
  prev: Map<string, number>,
  items: RankingsResponse["rankings"],
): Map<string, number | null> {
  const out = new Map<string, number | null>();
  for (const item of items) {
    const before = prev.get(item.symbol);
    out.set(item.symbol, before != null ? before - item.rank : null);
  }
  return out;
}

/*
행을 만든다. **이름은 서버가 준 것을 그대로 쓴다.**

예전에는 `stockMap`(두 번째 요청으로 채우는 맵)에서 이름을 찾고, 없으면 심볼을
그렸다. 그 요청이 `requestIdleCallback` 뒤에 나가서 목록이 **종목코드로 먼저
그려졌다가 이름으로 바뀌었고**, 배경 탭에서는 idle 콜백이 아예 안 돌아 코드가
그대로 남았다(실측 2026-09-01: 100행 전부 코드).

이제 `/rankings`가 이름·자산구분·로고를 같이 준다. `stockMap`은 **거래소(market)
표시**에만 남아 있다 — 그 값은 이 화면에서 쓰지 않지만 타입 계약이라 유지한다.
*/
function enrichRows(
  resp: RankingsResponse,
  stockMap: Map<string, StockInfo>,
  watch: Set<string>,
  held: Set<string>,
  rankDeltas: Map<string, number | null>,
): EnrichedRankingItem[] {
  return resp.rankings.map((row) => {
    const sym = row.symbol.toUpperCase();
    const stock = stockMap.get(sym);
    return {
      ...row,
      // 서버 이름이 1순위. 카탈로그에 없는 종목만 심볼로 되돌린다.
      name: row.name || stock?.name || row.symbol,
      market: stock?.market ?? "—",
      assetType: row.assetType ?? stock?.assetType,
      logoUrl: row.logoUrl || stock?.logoUrl,
      rankDelta: rankDeltas.get(row.symbol) ?? null,
      isWatchlisted: watch.has(sym),
      isHeld: held.has(sym),
    };
  });
}

/**
 * 5초마다 받은 가격·등락률을 **있는 행에 덮어쓴다.** 순위와 행 자리는 그대로다.
 *
 * **값이 그대로인 행은 같은 객체를 돌려준다.** 매번 새 객체를 만들면 React가 100행
 * 전부를 다시 그리고, 아래 `priceTick`이 매번 올라가 목록 전체가 5초마다 깜빡인다.
 * 실제로 바뀐 행만 새 객체가 되므로 리렌더도 그 행에서 끝난다.
 */
function applyLivePrices(
  rows: EnrichedRankingItem[],
  prices: Map<string, RankingPriceRow>,
): EnrichedRankingItem[] {
  let changed = false;
  const next = rows.map((row) => {
    const live = prices.get(row.symbol.toUpperCase());
    if (!live || live.lastPrice <= 0) return row;

    const nextPrice = String(live.lastPrice);
    const nextRate = String(live.changeRate);
    // 숫자로 비교한다 — `withStreamPrices`와 같은 이유다. 이 경로는 양쪽 문자열이
    // 다 서버 표기라 지금은 어긋나지 않지만, 두 경로가 같은 행에 쓰는 이상 판정
    // 규칙이 갈라져 있으면 다음에 한쪽만 고쳐진다.
    if (
      Number.parseFloat(row.price.lastPrice) === live.lastPrice &&
      Number.parseFloat(row.price.changeRate ?? "") === live.changeRate
    ) {
      return row;
    }

    const before = Number.parseFloat(row.price.lastPrice);
    changed = true;
    return {
      ...row,
      price: { ...row.price, lastPrice: nextPrice, changeRate: nextRate },
      // 값이 바뀐 횟수. 화면은 이 숫자가 바뀌는 것만 보고 플래시를 준다.
      priceTick: (row.priceTick ?? 0) + 1,
      priceDirection:
        Number.isFinite(before) && before > 0 && live.lastPrice !== before
          ? live.lastPrice > before
            ? ("up" as const)
            : ("down" as const)
          : row.priceDirection,
    };
  });
  return changed ? next : rows;
}

/**
 * 스트림이 준 가격을 행에 얹는다. **상태가 아니라 파생값이다.**
 *
 * `applyLivePrices`(폴링용)와 따로 둔 이유가 둘 있다.
 *
 * ① **등락률을 여기서 낸다.** 스트림은 가격만 보낸다 — 기준가는 세션 안에서 변하지
 * 않는 값이라 행이 이미 들고 있고(`price.basePrice`), 서버가 틱마다 종목별로 그걸
 * 풀면 종목당 4~5 SELECT가 붙는다. 그 비용을 없애려고 푸시로 옮긴 것이라 서버가
 * 그 산수를 계속하면 앞뒤가 안 맞는다. 기준가가 없는 행은 **건너뛴다** — 0.00%는
 * 「보합」으로 읽히는데 실제 뜻은 「기준가를 모른다」이고, 그건 빈칸보다 나쁘다.
 *
 * ② **플래시 신호를 스트림이 세어 둔 값에서 가져온다.** 폴링 경로는 행을 갈아
 * 끼우며 `priceTick`을 하나씩 올렸는데, 파생값은 매 렌더 같은 기준(`rows`)에서
 * 다시 계산되므로 그런 누적을 할 수 없다 — `+1`을 하면 언제나 같은 수가 나와
 * 플래시가 첫 번째 한 번만 뜬다. 그래서 「이 심볼이 몇 번째 갱신인가」를
 * 스트림 맵이 들고 있고(`seq`), 여기서는 그걸 더하기만 한다.
 *
 * 값이 바뀐 행만 새 객체가 되고, 하나도 안 바뀌면 **같은 배열**을 돌려준다.
 */
function withStreamPrices(
  rows: EnrichedRankingItem[],
  stream: ListPriceMap,
): EnrichedRankingItem[] {
  if (stream.size === 0) return rows;

  let changed = false;
  const next = rows.map((row) => {
    const tick = stream.get(row.symbol.toUpperCase());
    if (!tick || !(tick.price > 0)) return row;

    const rate = changeRateFrom(tick.price, parseRankingNumber(row.price.basePrice));
    if (rate === null) return row;

    const nextPrice = String(tick.price);
    const nextRate = String(rate);
    /*
      **같은 값인지는 숫자로 묻는다 — 문자열로 물으면 안 된다** (2026-09-04 사용자 지적 3).

      전에는 가격과 등락률 **문자열이 둘 다** 같아야 행을 그대로 뒀다. 그런데 두
      문자열의 출처가 다르다: 가격은 스트림이 준 숫자를 그대로 찍은 것이고, 행이
      들고 있는 등락률은 **서버가 반올림해 만든 문자열**("0.29")이다. 여기서 다시
      계산한 값은 "0.2898550724637681"이라 **가격이 한 푼도 안 움직였는데도 매번
      달랐다.** 그래서 폴링이 목록을 새로 받을 때마다 전 종목이 한 번씩 깜빡였다.

      숫자로 비교하면 그 차이가 사라진다. 가격이 같으면 등락률은 정의상 같은 값이라
      (기준가는 세션 안에서 안 변한다) 다시 쓸 이유도 없다 — 표기 정밀도만 다른
      같은 값이고, `formatPercent`가 어차피 같은 글자로 그린다.
    */
    if (Number.parseFloat(row.price.lastPrice) === tick.price) {
      return row;
    }

    changed = true;
    const before = tick.prevPrice ?? Number.parseFloat(row.price.lastPrice);
    return {
      ...row,
      price: { ...row.price, lastPrice: nextPrice, changeRate: nextRate },
      priceTick: (row.priceTick ?? 0) + tick.seq,
      priceDirection:
        Number.isFinite(before) && before > 0 && tick.price !== before
          ? tick.price > before
            ? ("up" as const)
            : ("down" as const)
          : row.priceDirection,
    };
  });
  return changed ? next : rows;
}

function patchWatchlistFlag(
  rows: EnrichedRankingItem[],
  symbol: string,
  watchlisted: boolean,
): EnrichedRankingItem[] {
  const sym = symbol.toUpperCase();
  return rows.map((item) =>
    item.symbol.toUpperCase() === sym ? { ...item, isWatchlisted: watchlisted } : item,
  );
}

function scheduleIdle(task: () => void): () => void {
  if (typeof window.requestIdleCallback === "function") {
    const id = window.requestIdleCallback(task, { timeout: 2000 });
    return () => window.cancelIdleCallback(id);
  }
  const id = window.setTimeout(task, 300);
  return () => window.clearTimeout(id);
}

/**
 * @param seed 서버가 미리 받아 둔 첫 목록 (`lib/market-api.server.ts`). 기본 손잡이로
 *   들어온 요청에만 붙는다 — 손잡이가 달린 주소에 기본 목록을 심으면 화면이 한
 *   프레임 동안 **틀린 목록**을 보여 준다.
 */
export function useMarketRankings(
  params: MarketRadarParams,
  seed?: RankingsResponse | null,
) {
  const { session } = useAuth();
  const { rankingsClientPollMs } = useMarketDataSettings();
  const paramsKey = marketRadarParamsKey(params);

  /*
    **선반입값으로 첫 프레임을 그린다** (2026-09-22).

    `enrichRows`에 넘기는 관심·보유 집합이 서버와 브라우저에서 같아야 hydration이
    어긋나지 않는다. 같다 — `watchlist-store`의 집합은 모듈 전역이고 localStorage를
    읽지 않아 새 탭의 첫 렌더에서 **양쪽 다 비어 있고**, 보유는 여기서도 원래
    빈 집합으로 시작해 idle 뒤에 채운다. 종목명·자산구분·로고는 응답이 들고 오고,
    거래소 표시는 `stockMap`이 비어 있어 양쪽 다 「—」다.

    화면 이동으로 들어온 경우(RSC 네비게이션)에는 저장소가 이미 차 있을 수 있는데,
    그때는 hydration 자체가 없으므로 아는 값으로 그리는 쪽이 맞다.
  */
  const [data, setData] = useState<RankingsResponse | null>(seed ?? null);
  const [items, setItems] = useState<EnrichedRankingItem[]>(() =>
    seed ? enrichRows(seed, new Map(), getWatchlistSymbols(), new Set(), new Map()) : [],
  );
  const [loading, setLoading] = useState(!seed);
  const [error, setError] = useState<string | null>(null);
  /*
    **마지막으로 가격을 받아 온 시각.** 목록 머리의 시계가 이 값을 찍는다
    (`lib/market-radar-clock` 참고) — 그 자리가 5분 주기의 순위 스냅샷 시각을
    보여 주는 동안, 정작 화면의 숫자는 1초마다 바뀌고 있었다.

    값이 그대로여도 갱신한다. 「몇 시의 값인가」는 숫자가 바뀌었는지가 아니라
    **언제 서버에 물어봤고 답을 받았는지**이고, 보합인 종목이 많은 순간에
    시계가 멈추면 그게 곧 「멈춘 화면」으로 읽힌다.
  */
  const [pricesAt, setPricesAt] = useState<string | null>(null);

  /*
    이 목록이 속한 시장이 **지금 닫혀 있는가** — 서버가 응답에 실어 준다.
    닫혀 있으면 가격 폴링을 통째로 멈추고, `nextSessionAt`에 다시 켠다.
    화면이 스스로 계산하지 않는 이유는 `lib/market-session` 머리글에 있다.
  */
  const marketClosed = data?.marketSession === "closed";
  const nextSessionAt = data?.nextSessionAt ?? null;

  const prevRanksRef = useRef<Map<string, number>>(new Map());
  const stockCacheRef = useRef<Map<string, StockInfo>>(new Map());
  /*
    관심종목은 **공유 저장소**가 들고 있다(`lib/watchlist-store`).

    예전에는 이 훅이 자기 몫의 집합을 `requestIdleCallback` 뒤에서 받아 왔다. 그래서
    목록은 언제나 「관심 없음」으로 한 번 그려진 뒤 뒤늦게 채워졌고, 다른 화면에
    갔다가 빠르게 돌아오면 그 빈 프레임이 그대로 남아 **담아 둔 종목의 하트가 꺼져
    보였다.** 이제 첫 렌더부터 아는 값으로 그린다.

    보유 종목은 이 화면 말고 쓰는 곳이 없어 그대로 idle에 둔다.
  */
  const heldRef = useRef<Set<string>>(new Set());
  const mountedRef = useRef(true);
  /*
    선반입값도 여기 둔다. 관심 목록이 도착해 `reenrich()`가 돌 때 이 자리가 비어
    있으면 **그냥 돌아가고**, 서버가 그려 준 100행의 하트는 첫 갱신이 올 때까지
    꺼져 있다 — 이 저장소를 만든 이유가 바로 그 빈 프레임이었다.
  */
  const lastRespRef = useRef<RankingsResponse | null>(seed ?? null);
  /** 선반입값으로 이미 그려져 있는가. 아래 초기화 이펙트가 **첫 실행에서만** 본다. */
  const seedPendingRef = useRef(Boolean(seed));

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  /** 저장해 둔 마지막 응답을 지금의 관심·보유 집합으로 다시 그린다. */
  const reenrich = useCallback(() => {
    const resp = lastRespRef.current;
    if (!resp || !mountedRef.current) return;
    const rankDeltas = computeRankDeltas(prevRanksRef.current, resp.rankings);
    setItems(
      enrichRows(resp, stockCacheRef.current, getWatchlistSymbols(), heldRef.current, rankDeltas),
    );
  }, []);

  const loadHeld = useCallback(async () => {
    if (!session) {
      heldRef.current = new Set();
      return;
    }
    const holdings = await fetchUserHoldingsClient().catch(() => []);
    if (!mountedRef.current) return;
    heldRef.current = new Set(holdings.map((h) => h.symbol.toUpperCase()));
    reenrich();
  }, [session, reenrich]);

  /*
    가격·등락률만 다시 받아 **있는 행에 덮어쓴다.**

    순위는 건드리지 않는다 — 스냅샷은 서버가 5분 주기로 갱신하므로 5초마다
    `/rankings`를 다시 부를 이유가 없고, 목록을 통째로 갈아 끼우면 누르려던 행이
    발밑에서 움직인다.

    **장이 닫혀 있으면 아예 부르지 않는다** (2026-09-04 사용자 지적). 예전에는
    그래도 1초마다 물었고, 서버는 빈 배열을 돌려줬다 — 「사실상 공짜」라고 적어
    뒀지만 공짜인 것은 상류 호출뿐이고 요청 자체는 뷰어당 1 req/s로 그대로 났다.
    하루의 대부분이 그 상태이므로, 그 구간이 이 화면이 만드는 요청의 대부분이었다.
    다음 창이 열리는 시각(`nextSessionAt`)에 스스로 깨어난다.
  */
  const refreshPrices = useCallback(async () => {
    if (!isPageVisible() || !mountedRef.current) return;
    try {
      const rows = await fetchRankingPricesClient({
        type: params.type,
        marketCountry: params.market,
        duration: params.duration,
        count: MARKET_RADAR_DEFAULT_COUNT,
      });
      if (!mountedRef.current || rows.length === 0) return;

      const priceMap = new Map(rows.map((row) => [row.symbol.toUpperCase(), row]));
      setItems((prev) => applyLivePrices(prev, priceMap));
      setPricesAt(new Date().toISOString());
    } catch {
      // 시세 덮어쓰기 실패는 조용히 넘긴다 — 순위·이름은 이미 화면에 있고,
      // 여기서 오류를 띄우면 5초마다 배너가 깜빡인다.
    }
  }, [params.type, params.market, params.duration]);

  /*
    관심 상태 반영은 **저장소를 거친다.** 그래야 이 목록만이 아니라 같은 종목을
    보고 있는 상세 헤더의 하트까지 같은 프레임에 바뀐다. 행 객체도 함께 갈아
    끼우는 것은 memo된 행이 구독 알림을 기다리지 않고 즉시 바뀌게 하기 위해서다.
  */
  const syncWatchlistSymbol = useCallback((symbol: string, watchlisted: boolean) => {
    const sym = symbol.trim().toUpperCase();
    if (!sym) return;
    setWatchlistLocal(sym, watchlisted);
    setItems((prev) => patchWatchlistFlag(prev, sym, watchlisted));
  }, []);

  const refresh = useCallback(
    async (initial = false): Promise<RankingsResponse | null> => {
      if (!mountedRef.current) return null;
      /*
        화면이 안 보이면 부르지 않는다 — 배경 탭이 상류 한도를 태울 이유가 없다.

        **여기서 로딩을 풀면 안 된다.** 안 보인다는 것은 「받아 봤더니 없더라」가
        아니라 **「아직 묻지 않았다」**다. 로딩을 풀면 화면이 빈 목록을 근거로
        「표시할 종목이 없습니다」라고 단정하는데, 그건 사실이 아니다.

        대신 상태를 그대로 두고 나간다. 다시 보이는 순간 아래 `visibilitychange`
        훅이 곧바로 부른다 — 그때까지는 「랭킹 로딩 중…」이 맞는 말이다.
      */
      if (!isPageVisible()) return null;
      try {
        if (initial) setError(null);

        const resp = await fetchRankingsClient({
          type: params.type,
          marketCountry: params.market,
          duration: params.duration,
          count: MARKET_RADAR_DEFAULT_COUNT,
        });
        if (!mountedRef.current) return null;

        lastRespRef.current = resp;
        const rankDeltas = computeRankDeltas(prevRanksRef.current, resp.rankings);
        const nextPrev = new Map<string, number>();
        for (const row of resp.rankings) {
          nextPrev.set(row.symbol, row.rank);
        }
        prevRanksRef.current = nextPrev;

        const rows = enrichRows(
          resp,
          stockCacheRef.current,
          getWatchlistSymbols(),
          heldRef.current,
          rankDeltas,
        );

        setData(resp);
        setItems(rows);
        if (initial) setLoading(false);
        // 이름·자산구분·로고는 이 응답이 이미 들고 있다. 예전에 여기서 idle 콜백으로
        // 걸던 두 번째 요청(`/stocks?symbols=…`)이 종목코드 깜빡임의 원인이었다.
        return resp;
      } catch (e) {
        if (!mountedRef.current) return null;
        setError(toUserMessage(e, "랭킹을 불러오지 못했습니다."));
        if (initial) setLoading(false);
        return null;
      }
    },
    [params.type, params.market, params.duration],
  );

  /*
    관심 목록은 **미루지 않는다** — 첫 프레임의 하트가 이 값에 달려 있다. 저장소가
    이미 최신이면 요청 자체가 나가지 않으므로, 화면을 오갈 때 붙던 왕복도 사라진다.
  */
  useEffect(() => {
    if (!session) return;
    void ensureWatchlist().then(reenrich);
  }, [session, reenrich]);

  // 다른 화면(상세 헤더 등)에서 담고 빼도 이 목록이 따라온다.
  useEffect(() => subscribeWatchlist(reenrich), [reenrich]);

  useEffect(() => {
    const cancel = scheduleIdle(() => {
      void loadHeld();
    });
    return cancel;
  }, [loadHeld]);

  /*
    시장·정렬·기간을 바꾸면 **앞 목록의 시각을 통째로 버린다** (2026-09-04 사용자 지적:
    「국내에서 해외로 바꾸면 시간이 조금 느리게 바뀐다」).

    예전에는 `pricesAt`만 지우고 `data`는 새 응답이 올 때까지 그대로 뒀다. 그래서
    전환 직후 시계가 **앞 시장의 스냅샷 시각**을 계속 찍었고, 새 응답이 온 뒤에도
    다음 가격 폴링(1초)이 돌기 전까지 한 박자 더 어긋났다. 지금은 둘 다 지우고,
    새 목록을 받은 **그 자리에서** 가격을 한 번 당겨온다 — 왕복 한 번 안에 끝난다.
  */
  useEffect(() => {
    /*
      **서버가 심어 준 목록은 비우지 않는다** (2026-09-22).

      이 이펙트는 마운트 때도 한 번 돈다. 선반입이 있는 첫 실행에서 위 네 줄을
      그대로 실행하면 서버가 그린 100행이 하이드레이션 직후 **사라졌다가 한 왕복
      뒤에 돌아온다** — 화면에는 선반입을 하기 전과 똑같이 「랭킹 로딩 중…」이
      스쳐 지나가고, 그러면 서버에서 미리 받아 온 값이 하는 일이 없다.

      **첫 실행에서만** 건너뛴다. 이후의 시장·정렬·기간 전환은 전과 똑같이 비운다 —
      그때 남아 있는 목록과 시각은 앞 시장의 것이라 지우는 게 맞다(09-04 사용자 지적:
      「국내에서 해외로 바꾸면 시간이 조금 느리게 바뀐다」).
    */
    const seeded = seedPendingRef.current;
    seedPendingRef.current = false;

    /*
      `prevRanksRef`는 선반입이 있어도 비운다. 서버가 심은 순위를 「이전 순위」로
      두면 첫 갱신에서 60초짜리 창의 순위 변동 화살표가 뜨는데, 새로고침으로 막
      들어온 사람에게 그건 비교할 앞이 없는 값이다. 선반입 전에도 첫 목록의
      `rankDelta`는 전부 null이었고, 그 약속을 그대로 지킨다.
    */
    prevRanksRef.current = new Map();
    if (!seeded) {
      setLoading(true);
      setError(null);
      setData(null);
      setPricesAt(null);
    }
    let cancelled = false;
    void refresh(!seeded).then((resp) => {
      // 마감 시장에는 당겨올 가격이 없다 — 시계는 서버가 준 마감 시각을 찍는다.
      if (cancelled || !resp || resp.marketSession === "closed") return;
      void refreshPrices();
    });
    return () => {
      cancelled = true;
    };
  }, [paramsKey, refresh, refreshPrices]);

  useEffect(() => {
    const id = window.setInterval(() => void refresh(false), rankingsClientPollMs);
    return () => window.clearInterval(id);
  }, [refresh, rankingsClientPollMs]);

  /*
    가격만 따로, 훨씬 자주. 순위 폴링(위)과 주기가 다르다.

    **마운트 직후에는 부르지 않는다.** `/rankings`가 방금 가격을 같이 줬으므로 그
    시점의 값은 이미 최신이고, 여기서 한 번 더 부르면 화면을 열 때마다 같은 값을
    받는 요청이 하나씩 더 나간다.

    **마감 중에는 타이머 자체를 걸지 않는다.** 다음 창이 열릴 때까지 가격이 바뀔 수
    없으므로 물어볼 것이 없다(아래 깨우기 타이머가 그 시각에 다시 켠다).
  */
  /*
    가격을 **푸시로도** 받는다 (2026-09-04 사용자 요청 1).

    아래 폴링은 그대로 두고 이 구독을 얹는다. 스트림이 붙어 있는 동안 폴링 주기가
    30초로 늦춰지고(`PRICE_POLL_MS_WHILE_STREAMING`), 끊기면 저절로 2초로 돌아온다.
    폴링을 0으로 끄지 않는 이유는 `use-quote-stream` 머리글에 있다 — SSE가 조용한
    것과 「가격이 안 변한 것」은 화면에서 구분되지 않는다.

    구독 대상은 **지금 그려져 있는 행들**이다. 목록이 갱신되면 심볼 집합이 바뀌고
    훅이 알아서 다시 연결한다(같은 집합이면 순서가 달라도 연결을 유지한다 —
    `streamSymbolsKey`).
  */
  const streamSymbols = useMemo(() => items.map((i) => i.symbol), [items]);
  const streamAssetType = params.market === "US" ? "us" : "domestic";
  const { prices: streamPrices, streaming } = useQuoteStream(
    streamSymbols,
    streamAssetType,
    !marketClosed,
  );

  /*
    스트림 가격은 **상태에 저장하지 않고 렌더 중에 얹는다.** 이펙트에서 `setItems`를
    하면 틱마다 렌더가 두 번 돌고, 100줄짜리 목록에서 그 두 배는 그대로 보인다.
  */
  const displayItems = useMemo(
    () => withStreamPrices(items, streamPrices),
    [items, streamPrices],
  );

  /*
    시계는 「언제 물어봤나」가 아니라 **「언제 값을 받았나」**를 찍는다. 푸시로 받은
    값도 그 자리에 서야 한다 — 안 그러면 스트림이 도는 동안 숫자는 움직이는데
    시계만 멈춰 있고, 그게 곧 「멈춘 화면」으로 읽힌다.

    스트림 맵이 바뀐 순간을 시각으로 옮기는 일이라 상태 갱신이 맞다(파생값으로
    만들 수 있는 값이 아니다 — 시각은 데이터가 아니라 **사건**이다).
  */
  const streamAt = useMemo(
    () => (streamPrices.size > 0 ? new Date().toISOString() : null),
    [streamPrices],
  );
  const effectivePricesAt = streamAt ?? pricesAt;

  useEffect(() => {
    if (marketClosed) return;
    const every = streaming ? PRICE_POLL_MS_WHILE_STREAMING : MARKET_RADAR_PRICE_POLL_MS;
    const id = window.setInterval(() => void refreshPrices(), every);
    return () => window.clearInterval(id);
  }, [refreshPrices, marketClosed, streaming]);

  /*
    다음 창이 열리는 순간 스스로 깨어난다 (사용자 요구: 「국장 기준 오전 8시와 같이
    장이 일부 열리는 시점부터는 호출을 해서 업데이트」).

    위 60초 순위 폴링도 결국 세션이 바뀐 것을 알아채지만, 그건 최악 1분이 밀린다.
    시각을 알고 있으므로 그 자리에 타이머를 건다 — 브라우저가 잠들었다 깨면 지연이
    지나 있어 즉시 발화하고, 그것도 맞는 동작이다.
  */
  useEffect(() => {
    if (!marketClosed || !nextSessionAt) return;
    const at = new Date(nextSessionAt).getTime();
    if (Number.isNaN(at)) return;
    // 개장 직후 한 박자 뒤에 묻는다 — 종이 울리는 순간에는 서버 달력도 막 넘어간다.
    const delay = Math.min(Math.max(at - Date.now() + 2_000, 1_000), 0x7fffffff);
    const id = window.setTimeout(() => {
      void refresh(false).then((resp) => {
        if (resp && resp.marketSession !== "closed") void refreshPrices();
      });
    }, delay);
    return () => window.clearTimeout(id);
  }, [marketClosed, nextSessionAt, refresh, refreshPrices]);

  /*
    다시 보이는 순간 한 번 당겨온다.

    두 가지를 고친다. ① 숨겨진 채로 로드되면 위 `refresh`가 아무것도 안 받고
    끝나는데, 그러면 다음 폴링(60초)까지 목록이 비어 있다. ② 탭을 오래 두고 온
    사용자가 **낡은 가격을 먼저 보고** 5초 뒤에야 갱신되는 것도 막는다.
  */
  useEffect(() => {
    const onVisible = () => {
      if (!isPageVisible()) return;
      void refresh(items.length === 0).then((resp) => {
        // 순위 응답이 「지금 열려 있다」고 말할 때만 가격을 당긴다. 탭을 밤새 열어
        // 두고 아침에 돌아온 경우가 이 경로인데, 그때 세션은 응답이 알려 준다.
        if (resp && resp.marketSession !== "closed") void refreshPrices();
      });
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, [refresh, refreshPrices, items.length]);

  return {
    data,
    items: displayItems,
    loading,
    error,
    pricesAt: effectivePricesAt,
    refresh,
    syncWatchlistSymbol,
  };
}

export function syncMarketRadarUrl(
  router: { replace: (url: string, opts?: { scroll?: boolean }) => void },
  params: MarketRadarParams,
  symbol?: string | null,
) {
  const q = marketRadarSearchParams(params);
  if (symbol) q.set("symbol", symbol);
  router.replace(`/market?${q.toString()}`, { scroll: false });
}

export function parseMarketRadarFromSearchParams(searchParams: URLSearchParams): MarketRadarParams {
  return parseMarketRadarParams(searchParams);
}
