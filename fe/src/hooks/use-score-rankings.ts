"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { fetchUserHoldingsClient } from "@/lib/holdings-import-api";
import {
  ensureWatchlist,
  getWatchlistSymbols,
  subscribeWatchlist,
} from "@/lib/watchlist-store";
import {
  fetchScoreRankingsClient,
  type StockScoreListItem,
} from "@/lib/stock-score-api";
import {
  scoreRankingParamsKey,
  type ScoreRankingParams,
} from "@/lib/score-ranking-config";
import { toUserMessage } from "@/lib/error-message";
import { useQuoteStream } from "@/hooks/use-quote-stream";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { SCREEN_SLOW_REFRESH_MS } from "@/lib/screen-refresh";
import { type ListPriceMap } from "@/lib/quote-stream-state";
import { liveOverlayFor, PRICE_AS_OF_LIVE } from "@/lib/list-price-overlay";

export type EnrichedScoreItem = StockScoreListItem & {
  isWatchlisted: boolean;
  isHeld: boolean;
};

/*
스트림이 준 가격을 행에 덮는다.

**값이 그대로인 행은 같은 객체를 돌려준다** — 매번 새 객체를 만들면 100줄이 2초마다
통째로 다시 그려진다(`ScoreRankingRow`가 memo인 이유이기도 하다).

`priceAsOf`를 `"live"`로 바꾸는 것은 툴팁 문구 때문이다. 그 자리는 「이 값이 언제
것인가」를 말하는데, 배치가 준 종가 날짜를 그대로 두면 지금 흐르는 값 위에 이틀 전
날짜가 붙는다.
*/
function applyStreamPricesToScoreRows(
  rows: EnrichedScoreItem[],
  stream: ListPriceMap,
): EnrichedScoreItem[] {
  let changed = false;
  const next = rows.map((row) => {
    /*
      **판정은 `lib/list-price-overlay`가 한다** — 종목 스크리너도 같은 함수를
      쓴다(2026-09-23 QA P0-01). 예전에는 이 규칙이 이 파일에만 있어서, 장중에
      같은 종목이 랭킹에서는 흐르는 값으로 스크리너에서는 어제 종가로 보였다.
    */
    const live = liveOverlayFor(row.symbol, row.closePrice, row.basePrice, stream);
    if (!live) return row;

    changed = true;
    return {
      ...row,
      closePrice: live.price,
      // 기준가를 모르면 **직전 등락률을 지키고** 0으로 덮지 않는다.
      changeRate: live.changeRate ?? row.changeRate,
      priceAsOf: PRICE_AS_OF_LIVE,
    };
  });
  return changed ? next : rows;
}

/**
 * Score-ranked 국내/해외 leaderboard for the AI 추천 랭킹 화면.
 *
 * # 순위는 배치의 결과, 가격은 지금 값 (2026-09-04 사용자 지적 1)
 *
 * 순위·점수는 하루 한 번 도는 배치가 만들므로 그쪽은 폴링하지 않는다 — 시장·축·기간을
 * 바꿀 때만 다시 받는다. **그런데 가격은 그 규칙을 따르면 안 됐다.** 예전에는 이
 * 목록이 한 번 받은 값을 끝까지 들고 있어서, 옆의 실시간 차트가 2초마다 움직이는
 * 동안 같은 종목의 가격이 이 화면에서만 멈춰 있었다.
 *
 * 그렇다고 이 엔드포인트를 2초마다 부를 수는 없다 — 실측 100 SELECT · 194ms짜리
 * 요청이다(기준가 해석이 종목당 5회). 그래서 **가격만 SSE로 따로 받는다**:
 * 순위는 그대로 두고 행의 가격 칸만 덮는다. 서버 작업은 뷰어 수와 무관하다.
 */
export function useScoreRankings(
  params: ScoreRankingParams,
  limit = 20,
  /**
   * 서버가 미리 받아 둔 첫 목록 (`lib/home-api.server.ts`, `app/(main)/analysis/page.tsx`).
   *
   * 있으면 **첫 렌더부터 목록이 있다** — 그전까지 `/analysis`의 서버 HTML은
   * 「랭킹 로딩 중…」이 전부였고, 크롤러가 받는 문서에 종목명이 한 개도 없었다
   * (2026-09-20 점검 요청 9).
   *
   * **관심·보유 표시는 싣지 않는다.** 그 둘은 사용자마다 다르고 `enrich`가
   * localStorage·API를 읽어 채우는데, 서버는 그 값을 모른다. 서버가 그린 하트와
   * 브라우저가 그릴 하트가 다르면 그게 곧 hydration 불일치라, 씨앗은 언제나
   * 「둘 다 꺼짐」으로 들어오고 마운트 뒤 아래 이펙트들이 켠다.
   */
  seed?: { items: StockScoreListItem[]; tradeDate: string },
) {
  const paramsKey = scoreRankingParamsKey(params);
  const { session } = useAuth();
  const seeded = (seed?.items.length ?? 0) > 0;
  const seededKey = useRef(seeded ? paramsKey : null);
  const [items, setItems] = useState<EnrichedScoreItem[]>(() =>
    (seed?.items ?? []).map((row) => ({ ...row, isWatchlisted: false, isHeld: false })),
  );
  const [tradeDate, setTradeDate] = useState(seed?.tradeDate ?? "");
  // `seededKey.current`가 아니라 `seeded`를 읽는다 — 렌더 중에 ref를 읽으면 안 된다
  // (`react-hooks/refs`). 두 값은 첫 렌더에서 같은 것을 뜻한다.
  const [loading, setLoading] = useState(!seeded);
  const [error, setError] = useState<string | null>(null);
  /** 「다시 시도」가 올리는 값. 이펙트를 한 번 더 돌린다(§BUG-11). */
  const [retryKey, setRetryKey] = useState(0);

  // 관심 집합은 공유 저장소가 들고 있다 — 다른 화면에서 담고 빼도 이 하트가 따라온다.
  const heldRef = useRef<Set<string>>(new Set());

  const enrich = useCallback((rows: StockScoreListItem[]): EnrichedScoreItem[] => {
    const watch = getWatchlistSymbols();
    const held = heldRef.current;
    return rows.map((row) => {
      const sym = row.symbol.toUpperCase();
      return { ...row, isWatchlisted: watch.has(sym), isHeld: held.has(sym) };
    });
  }, []);

  useEffect(() => {
    /*
      **씨앗으로 채워진 첫 회차는 건너뛴다** (2026-09-20 점검 요청 5).

      건너뛰지 않으면 마운트 직후 `setLoading(true)`가 돌아 서버가 그려 준 목록이
      **스켈레톤으로 한 번 지워졌다** 돌아온다 — 선반입으로 없애려던 깜박임을
      선반입이 만드는 꼴이다.

      손잡이를 바꾸면 `paramsKey`가 달라지므로 그때는 정상적으로 받아온다.
      `seededKey`를 null로 비우는 것은 「씨앗은 한 번만 쓴다」는 뜻이다 — 같은
      손잡이로 되돌아왔을 때 서버 시점의 목록을 다시 쓰면 그 사이의 갱신이 사라진다.
    */
    if (seededKey.current === paramsKey) {
      seededKey.current = null;
      return;
    }

    let cancelled = false;
    setLoading(true);
    setError(null);
    /*
      `paramsKey`가 이펙트의 의존성이다 — `params` 객체를 그대로 걸면 부모가 렌더될
      때마다 새 객체라 매번 다시 받아온다. 실제로 바뀐 것은 세 손잡이뿐이다.
    */
    const [market, axis, duration] = paramsKey.split("|");
    fetchScoreRankingsClient({
      market: market as ScoreRankingParams["market"],
      axis,
      duration,
      limit,
    })
      .then(({ items: rows, tradeDate: date }) => {
        if (cancelled) return;
        setItems(enrich(rows));
        setTradeDate(date);
      })
      .catch((e) => {
        if (cancelled) return;
        setError(toUserMessage(e, "종목 랭킹을 불러오지 못했습니다."));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [paramsKey, limit, enrich, retryKey]);

  /*
    **30분마다 순위를 다시 받는다** (2026-09-16 사용자 요청, `lib/screen-refresh`).

    이 순위를 만드는 것은 하루 한 번 도는 점수 배치(00:25)다 — 5분 주기는 스물아홉
    번이 헛걸음인데, 요청 하나가 실측 100 SELECT · 194ms라 그 헛걸음이 싸지도 않다.
    그래도 0이 아닌 이유는 화면을 열어 둔 채 자정을 넘기면 어제 랭킹에 오늘 날짜가
    붙기 때문이다(`tradeDate`가 화면에 적힌다).

    가격은 여기 걸리지 않는다 — 이미 SSE가 아래에서 밀어 준다. 이 회차가 갱신하는
    것은 **순위·점수·기준일**이다.

    갱신은 스피너를 띄우지 않고, 실패하면 보고 있던 순위를 남긴다 — 「다시 시도」가
    있는 오류 화면으로 떨어뜨리면 멀쩡히 보던 목록이 사라진다.
  */
  useVisibleInterval(() => {
    const [market, axis, duration] = paramsKey.split("|");
    fetchScoreRankingsClient({
      market: market as ScoreRankingParams["market"],
      axis,
      duration,
      limit,
    })
      .then(({ items: rows, tradeDate: date }) => {
        setItems(enrich(rows));
        setTradeDate(date);
      })
      .catch(() => {});
  }, SCREEN_SLOW_REFRESH_MS);

  /*
    가격만 푸시로 받는다. 순위·점수는 위 이펙트가 받은 그대로 둔다.

    `assetType`은 서버가 마감을 판정할 달력이다 — 이 화면의 시장 탭이 곧 그 값이고,
    해외 탭은 미국 달력을 쓴다(ETF도 같은 달력이라 `us` 하나로 충분하다).
  */
  const streamSymbols = useMemo(() => items.map((i) => i.symbol), [items]);
  const streamAssetType = params.market === "overseas" ? "us" : "domestic";
  const { prices: streamPrices } = useQuoteStream(streamSymbols, streamAssetType);

  /*
    스트림 가격은 **상태에 저장하지 않고 렌더 중에 얹는다.**

    이펙트에서 `setItems`를 하면 틱마다 렌더가 두 번 돈다(이펙트가 상태를 바꾸고,
    그 상태가 다시 렌더를 부른다) — 2초마다 100줄짜리 목록에서는 그게 그대로
    보인다. 파생값이면 한 번이다. 배치가 준 순위·점수(`items`)와 지금 가격은
    **출처가 다른 값**이라, 하나를 다른 하나에 녹여 저장할 이유도 없다.
  */
  const displayItems = useMemo(
    () => applyStreamPricesToScoreRows(items, streamPrices),
    [items, streamPrices],
  );

  useEffect(
    () => subscribeWatchlist(() => setItems((prev) => enrich(prev))),
    [enrich],
  );

  useEffect(() => {
    if (!session) {
      heldRef.current = new Set();
      setItems((prev) => enrich(prev));
      return;
    }
    let cancelled = false;
    void ensureWatchlist();
    fetchUserHoldingsClient()
      .catch(() => [])
      .then((holdings) => {
        if (cancelled) return;
        heldRef.current = new Set(holdings.map((h) => h.symbol.toUpperCase()));
        setItems((prev) => enrich(prev));
      });
    return () => {
      cancelled = true;
    };
  }, [session, enrich]);

  return {
    items: displayItems,
    tradeDate,
    loading,
    error,
    retry: () => setRetryKey((k) => k + 1),
  };
}
