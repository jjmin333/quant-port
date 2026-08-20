"use client";

import { useMemo } from "react";
import { IndexMiniCard } from "@/components/market/index-mini-card";
import { StockMiniCard } from "@/components/stocks/stock-mini-card";
import { useMarketIndices } from "@/hooks/use-market-indices";
import { useSymbolQuotes } from "@/hooks/use-news-symbols";
import { useStockScores } from "@/hooks/use-stock-scores";
import { useUpcomingEvents } from "@/hooks/use-upcoming-events";
import { articleDashboardIndexIds } from "@/lib/news-dashboard-indices";
import type { NewsArticle } from "@/lib/news-types";
import type { MarketIndex, QuoteSummary } from "@/lib/quotes-types";

/** 한 기사에 카드로 세울 종목 수 상한. 이보다 많으면 읽는 게 아니라 스크롤이 된다. */
const MAX_SYMBOL_CARDS = 6;

/**
 * D1 — 기사 모달의 「관련 종목·지수」.
 *
 * 기사를 읽는 동안 "그래서 지금 얼마인데?"에 답한다. 예전에는 모달을 닫고 다른
 * 화면으로 가야 알 수 있었다.
 *
 * 두 가지를 고쳤다 —
 *
 *  1. **탭이 아니라 카드를 전부 편다.** 종목이 여럿이면 위에 탭을 두고 한 번에 하나만
 *     보여줬는데, "아마존이 오르고 애플이 내렸다"는 기사에서 둘을 비교하려면 탭을
 *     번갈아 눌러야 했다. 기사가 두 종목 이야기를 한다면 두 카드가 같이 보여야 한다.
 *  2. **지수도 카드로 낸다.** 같은 기사가 "서울 증시 사상 최고"를 말하는데 코스피는
 *     모달 어디에도 없었다. 판정은 목록 필터와 **같은 규칙**을 쓴다.
 *
 * 지수를 먼저 놓는다 — 개별 종목보다 시장 전체가 먼저 읽히는 게 기사의 순서다.
 * 카드 모양은 `StockMiniCard`·`IndexMiniCard`가 앱 전체와 공유한다.
 */
export function NewsRelatedEntityPanel({
  article,
  symbols,
  quotes,
  onNavigate,
}: {
  article: NewsArticle;
  symbols: string[];
  quotes: Map<string, QuoteSummary>;
  /** 카드에서 다른 화면으로 떠날 때 모달을 닫는다. */
  onNavigate?: () => void;
}) {
  const usable = useMemo(
    () =>
      [
        ...new Set(symbols.map((s) => s.trim().toUpperCase()).filter(Boolean)),
      ].slice(0, MAX_SYMBOL_CARDS),
    [symbols],
  );

  /*
   * 화면 전체가 공유하는 시세 배치(`useSymbolQuotes`)는 상한이 60종목이다. 목록에
   * 걸린 종목이 그보다 많으면 상한 밖의 종목은 시세가 없는데, 하필 그 종목이 지금
   * 연 기사의 카드일 수 있다 — 실제로 아마존 카드가 "시세를 불러오지 못했습니다"에
   * 통화까지 원화로 떨어졌다(통화는 시세에서 온다).
   *
   * 모달은 한 번에 하나만 열리므로, **빠진 것만** 여기서 한 번 더 부른다. 다 채워져
   * 있으면 목록이 비어 요청이 나가지 않는다.
   */
  const missing = useMemo(() => usable.filter((s) => !quotes.has(s)), [usable, quotes]);
  const filledQuotes = useSymbolQuotes(missing);
  const quoteFor = (symbol: string) => quotes.get(symbol) ?? filledQuotes.get(symbol);

  const { indices } = useMarketIndices();
  const indexRows = useMemo(() => {
    const wanted = articleDashboardIndexIds(article);
    const byId = new Map(indices.map((i) => [i.id, i]));
    // 값이 아직 안 채워진 지수(플레이스홀더)는 0.00%만 적힌 카드가 되므로 뺀다.
    return wanted
      .map((id) => byId.get(id))
      .filter((i): i is MarketIndex => i !== undefined && i.value > 0);
  }, [article, indices]);

  // 카드가 여러 장이어도 점수 요청은 한 번이다.
  const scores = useStockScores(usable);

  // 일정은 배치 엔드포인트가 없어 종목당 1요청이라, 카드 수 상한이 곧 요청 상한이다.
  const eventTargets = useMemo(
    () =>
      usable.map((symbol) => ({
        symbol,
        name: quotes.get(symbol)?.name || filledQuotes.get(symbol)?.name || symbol,
      })),
    [usable, quotes, filledQuotes],
  );
  const { bySymbol: eventsBySymbol } = useUpcomingEvents(eventTargets, MAX_SYMBOL_CARDS);

  if (usable.length === 0 && indexRows.length === 0) return null;

  return (
    <div className="flex min-w-0 flex-col gap-2">
      {indexRows.map((index) => (
        <IndexMiniCard key={index.id} index={index} onNavigate={onNavigate} />
      ))}
      {usable.map((symbol) => (
        <StockMiniCard
          key={symbol}
          symbol={symbol}
          quote={quoteFor(symbol)}
          score={scores.get(symbol) ?? null}
          events={eventsBySymbol.get(symbol)}
          articleId={article.id}
          onNavigate={onNavigate}
        />
      ))}
    </div>
  );
}
