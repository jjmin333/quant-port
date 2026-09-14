"use client";

import { useMemo } from "react";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { NewsSampleRailNote } from "@/components/news/news-login-gate";
import { PersonalScheduleCard } from "@/components/earnings/personal-schedule-card";
import { useUpcomingEvents } from "@/hooks/use-upcoming-events";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { isDomesticAsset } from "@/hooks/use-quote-stream";
import { QuoteCell } from "@/components/ui/quote-cell";
import { useMarketIndices } from "@/hooks/use-market-indices";
import { formatPercent } from "@/lib/format";
import { isArticleBreakingNews } from "@/lib/news-breaking";
import { newsDisplayHeadline } from "@/lib/news-display";
import type { NewsArticle } from "@/lib/news-types";
import type { QuoteSummary } from "@/lib/quotes-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/* ── E1: 다가오는 일정 ────────────────────────────────────────────────────── */

/**
 * 레일의 다른 카드는 전부 **지금·과거**를 본다(지수·키워드·많이 본 뉴스·리포트).
 * 앞으로를 보는 칸이 하나도 없어서, 실적 발표를 뉴스로 알게 되는 일이 생겼다.
 *
 * 일정은 내 종목(비로그인이면 인기 종목) 기준이다. 매크로 지표 일정(FOMC·CPI)은
 * 아직 소스가 없어 **넣지 않았다** — 없는 일정을 지어내느니 실적만 정확히 내는 편이 낫다.
 */
export function NewsUpcomingEventsCard({
  market,
  symbols,
  quotes,
  personal,
}: {
  /*
    이 카드가 서 있는 뉴스 탭 (2026-09-04 사용자 요청 12).

    **국내 뉴스에는 국내 종목만 나온다.** 심볼은 사용자의 보유·관심에서 오는데
    그 목록에는 두 시장이 섞여 있어서, 국내 뉴스를 읽는 화면의 레일에 애플·엔비디아
    일정이 올라왔다. 화면이 「국내」라고 적힌 채 해외 종목을 말하면 그건 다른 화면의
    내용이다.
  */
  market: "domestic" | "us";
  symbols: string[];
  quotes: Map<string, QuoteSummary>;
  personal: boolean;
}) {
  /*
    시장이 맞는 종목만 남긴다. 자산 구분은 시세가 알고 있고, 시세가 아직 없는
    종목은 심볼 모양으로 가른다(국내는 여섯 자리 숫자) — 그래야 시세가 늦게
    도착하는 동안 해외 종목이 국내 목록에 한 프레임 얹히지 않는다.
  */
  const mySymbols = useMemo(
    () =>
      symbols.filter(
        (s) => isDomesticAsset(quotes.get(s.toUpperCase())?.assetType, s) === (market === "domestic"),
      ),
    [symbols, quotes, market],
  );

  const targets = useMemo(
    () =>
      mySymbols.slice(0, 8).map((symbol) => ({
        symbol,
        name: quotes.get(symbol)?.name || symbol,
      })),
    [mySymbols, quotes],
  );
  const { events } = useUpcomingEvents(targets, 8);

  return (
    <PersonalScheduleCard
      events={events}
      /* 뉴스 화면에는 종목 시세가 없다 — 이 레일에서만 오른쪽에 시세를 적는다(2026-09-04 요청 8). */
      showQuotes
      market={market}
      href="/calendar"
      hrefLabel="캘린더"
      footer={
        !personal && events !== null && events.length > 0 ? <NewsSampleRailNote /> : undefined
      }
    />
  );
}

/* ── E2: 내 종목 뉴스 ─────────────────────────────────────────────────────── */

/**
 * 내 보유·관심 종목이 언급된 기사만.
 *
 * 비로그인일 때는 흐림 처리 대신 **인기 종목 예시**를 실제로 보여주고 그렇게 적는다.
 * 흐리게 덮으면 없는 데이터를 있는 것처럼 보이게 만든다.
 */
export function NewsMyStocksCard({
  articles,
  symbols,
  quotes,
  personal,
  onArticleClick,
}: {
  articles: NewsArticle[];
  symbols: string[];
  quotes: Map<string, QuoteSummary>;
  personal: boolean;
  onArticleClick?: (article: NewsArticle) => void;
}) {
  const mine = useMemo(() => {
    const wanted = new Set(symbols.map((s) => s.toUpperCase()));
    if (wanted.size === 0) return [];
    return articles
      .filter((article) =>
        (article.symbols ?? []).some((s) => wanted.has(s.trim().toUpperCase())),
      )
      .slice(0, 6);
  }, [articles, symbols]);

  /*
    행마다 짝지어진 종목의 **지금** 시세 (2026-09-04 사용자 요청 8·9).

    전에는 화면이 처음 받은 `quotes` 맵을 그대로 읽어 등락률만 적었다. 그 맵은 뉴스
    화면이 열릴 때 한 번 채워지고 다시 안 바뀌므로, 기사를 읽는 몇 분 동안 숫자가
    굳어 있었다 — 그리고 가격 자체는 아예 없었다.
  */
  const rowSymbols = useMemo(
    () =>
      mine
        .map((article) =>
          (article.symbols ?? []).find((s) =>
            symbols.some((mineSym) => mineSym.toUpperCase() === s.trim().toUpperCase()),
          ),
        )
        .filter((s): s is string => Boolean(s))
        .map((s) => s.trim()),
    [mine, symbols],
  );
  const live = useLiveQuotes(rowSymbols);

  return (
    <SectionCard
      title="내 종목 뉴스"
      action={
        <span className="shrink-0 text-cell text-muted-foreground">
          {personal ? `${mine.length}건` : "예시"}
        </span>
      }
    >
      {mine.length === 0 ? (
        <SectionEmpty
          text={personal ? "내 종목이 언급된 기사가 없습니다" : "관련 기사가 없습니다"}
        />
      ) : (
        <ul className="@container min-w-0">
          {mine.map((article) => {
            const first = (article.symbols ?? []).find((s) =>
              symbols.some((mineSym) => mineSym.toUpperCase() === s.trim().toUpperCase()),
            );
            const sym = first?.trim().toUpperCase();
            /*
              스트림이 준 값을 먼저 쓰고, 아직 안 붙었으면 화면이 들고 있던 값으로.
              둘을 합친 유니온으로 쓰지 않고 **따로 읽는 이유**는 좁히기가 안 되기
              때문이다 — 두 타입 다 `symbol`·`currency`를 갖고 있어서 `in` 검사로는
              구별되지 않는다.
            */
            const streamed = sym ? live.get(sym) : undefined;
            const quote = sym ? (streamed ?? quotes.get(sym)) : undefined;
            const price = streamed?.livePrice ?? quote?.currentPrice ?? 0;
            const rate = streamed?.liveChangeRate ?? quote?.changeRate ?? 0;
            const hasPrice = price > 0;
            return (
              <li key={article.id}>
                <button
                  type="button"
                  onClick={() => onArticleClick?.(article)}
                  className="flex w-full min-w-0 items-center gap-2 rounded-md px-1 py-1.5 text-left hover:bg-accent"
                >
                  {isArticleBreakingNews(article) && (
                    <span className="shrink-0 rounded bg-[var(--loss)]/15 px-1 text-micro font-bold text-[var(--loss)]">
                      속보
                    </span>
                  )}
                  <span className="min-w-0 flex-1 truncate text-cell">
                    {newsDisplayHeadline(article)}
                  </span>
                  {hasPrice && quote && (
                    <QuoteCell
                      price={price}
                      changeRate={rate}
                      currency={quote.currency}
                      symbol={quote.symbol}
                    />
                  )}
                </button>
              </li>
            );
          })}
        </ul>
      )}
      {!personal && <NewsSampleRailNote />}
    </SectionCard>
  );
}

/* ── E4: 매크로 지표 (해외 전용) ──────────────────────────────────────────── */

/**
 * 해외 화면에만 둔다 — 국내 화면의 「지수 흐름」과 성격이 겹치기 때문이다.
 *
 * 목록은 **실제로 수집되는 지표만** 담았다. 목업의 달러인덱스는 파이프라인에 없고,
 * 미 10년은 FRED 일별로 들어온다.
 */
const MACRO_INDEX_IDS = ["vix", "us10y", "sox", "nq100", "gold", "btc"];

export function NewsMacroCard() {
  const { indices } = useMarketIndices();

  const rows = useMemo(() => {
    const byId = new Map(indices.map((i) => [i.id, i]));
    return MACRO_INDEX_IDS.map((id) => byId.get(id)).filter((i) => i !== undefined);
  }, [indices]);

  const hasData = rows.some((i) => i.value > 0);

  return (
    <SectionCard title="매크로 지표" href="/market" hrefLabel="마켓 레이더">
      {!hasData ? (
        <SectionSkeleton rows={5} />
      ) : (
        <ul className="min-w-0">
          {rows.map((idx) => (
            <li
              key={idx.id}
              className="flex h-[30px] min-w-0 items-center gap-2 border-b border-border/60 px-1 text-cell last:border-b-0"
            >
              <span className="min-w-0 flex-1 truncate">{idx.name}</span>
              <span className="shrink-0 font-num text-cell tabular-nums">
                {idx.value
                  ? idx.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })
                  : "—"}
              </span>
              <span
                className={cn(
                  "w-14 shrink-0 text-right font-num text-cell tabular-nums",
                  trendTextClass(idx.changeRate),
                )}
              >
                {formatPercent(idx.changeRate)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
