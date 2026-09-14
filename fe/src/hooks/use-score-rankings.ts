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
import { changeRateFrom, type ListPriceMap } from "@/lib/quote-stream-state";

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
    const tick = stream.get(row.symbol.toUpperCase());
    if (!tick || !(tick.price > 0)) return row;
    if (row.closePrice === tick.price) return row;

    const rate = changeRateFrom(tick.price, row.basePrice);
    changed = true;
    return {
      ...row,
      closePrice: tick.price,
      // 기준가를 모르면 **직전 등락률을 지키고** 0으로 덮지 않는다.
      changeRate: rate ?? row.changeRate,
      priceAsOf: "live",
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
export function useScoreRankings(params: ScoreRankingParams, limit = 20) {
  const paramsKey = scoreRankingParamsKey(params);
  const { session } = useAuth();
  const [items, setItems] = useState<EnrichedScoreItem[]>([]);
  const [tradeDate, setTradeDate] = useState("");
  const [loading, setLoading] = useState(true);
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
