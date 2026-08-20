"use client";

import { useMemo } from "react";
import { X } from "lucide-react";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { NewsSampleRailNote } from "@/components/news/news-login-gate";
import { useUpcomingEvents } from "@/hooks/use-upcoming-events";
import { useMarketIndices } from "@/hooks/use-market-indices";
import { publisherBreakdown } from "@/lib/news-derive";
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
  symbols,
  quotes,
  personal,
}: {
  symbols: string[];
  quotes: Map<string, QuoteSummary>;
  personal: boolean;
}) {
  const targets = useMemo(
    () =>
      symbols.slice(0, 8).map((symbol) => ({
        symbol,
        name: quotes.get(symbol)?.name || symbol,
      })),
    [symbols, quotes],
  );
  const { events } = useUpcomingEvents(targets, 8);

  return (
    <SectionCard title="다가오는 일정" href="/calendar" hrefLabel="캘린더">
      {events === null ? (
        <SectionSkeleton rows={4} />
      ) : events.length === 0 ? (
        <SectionEmpty text="예정된 실적·공시가 없습니다" />
      ) : (
        <ul className="min-w-0">
          {events.slice(0, 6).map((event, i) => (
            <li
              key={`${event.symbol}-${event.date}-${i}`}
              className="flex items-start gap-2 border-b border-border/60 py-1.5 text-[11px] last:border-b-0"
            >
              <span className="w-12 shrink-0 font-mono tabular-nums text-muted-foreground">
                {formatEventDate(event.date)}
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate">{event.name}</span>
                <span className="block truncate text-[10px] text-muted-foreground">
                  {event.label || "실적 발표"}
                </span>
              </span>
            </li>
          ))}
        </ul>
      )}
      {!personal && events !== null && events.length > 0 && <NewsSampleRailNote />}
    </SectionCard>
  );
}

function formatEventDate(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return iso.slice(5);
  const days = Math.round((at.getTime() - Date.now()) / 86_400_000);
  if (days <= 0) return "오늘";
  if (days <= 30) return `D-${days}`;
  return `${at.getMonth() + 1}/${at.getDate()}`;
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

  return (
    <SectionCard
      title="내 종목 뉴스"
      action={
        <span className="shrink-0 text-[11px] text-muted-foreground">
          {personal ? `${mine.length}건` : "예시"}
        </span>
      }
    >
      {mine.length === 0 ? (
        <SectionEmpty
          text={personal ? "내 종목이 언급된 기사가 없습니다" : "관련 기사가 없습니다"}
        />
      ) : (
        <ul className="min-w-0">
          {mine.map((article) => {
            const first = (article.symbols ?? []).find((s) =>
              symbols.some((mineSym) => mineSym.toUpperCase() === s.trim().toUpperCase()),
            );
            const quote = first ? quotes.get(first.trim().toUpperCase()) : undefined;
            const hasPrice = Boolean(quote && quote.currentPrice > 0);
            return (
              <li key={article.id}>
                <button
                  type="button"
                  onClick={() => onArticleClick?.(article)}
                  className="flex w-full min-w-0 items-center gap-2 rounded-md px-1 py-1.5 text-left hover:bg-accent"
                >
                  {isArticleBreakingNews(article) && (
                    <span className="shrink-0 rounded bg-[var(--loss)]/15 px-1 text-[9px] font-bold text-[var(--loss)]">
                      속보
                    </span>
                  )}
                  <span className="min-w-0 flex-1 truncate text-xs">
                    {newsDisplayHeadline(article)}
                  </span>
                  {hasPrice && (
                    <span
                      className={cn(
                        "shrink-0 font-mono text-[11px] tabular-nums",
                        trendTextClass(quote!.changeRate),
                      )}
                    >
                      {formatPercent(quote!.changeRate)}
                    </span>
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

/* ── E3: 출처 분포 ────────────────────────────────────────────────────────── */

/** 막대를 누르면 그 매체 기사만 남는다 — 드롭다운에 묻혀 있던 기능을 꺼내 놓는다. */
export function NewsSourceBreakdownCard({
  articles,
  activePublisherId,
  onSelect,
}: {
  articles: NewsArticle[];
  activePublisherId: string | null;
  onSelect: (publisherId: string | null) => void;
}) {
  const rows = useMemo(() => publisherBreakdown(articles), [articles]);
  const max = rows[0]?.count ?? 1;

  return (
    <SectionCard
      title="출처 분포"
      action={
        <span className="shrink-0 text-[11px] text-muted-foreground">
          {articles.length.toLocaleString("ko-KR")}건
        </span>
      }
    >
      {rows.length === 0 ? (
        <SectionEmpty text="집계할 기사가 없습니다" />
      ) : (
        <ul className="min-w-0 space-y-0.5">
          {rows.map((row) => {
            const active = activePublisherId === row.id;
            return (
              <li key={row.id}>
                <button
                  type="button"
                  onClick={() => onSelect(active ? null : row.id)}
                  aria-pressed={active}
                  className="grid w-full grid-cols-[4rem_minmax(0,1fr)_1.75rem] items-center gap-1.5 rounded-md px-1 py-1 text-left text-[11px] hover:bg-accent"
                >
                  <span
                    className={cn(
                      "truncate",
                      active ? "font-semibold text-[var(--brand)]" : "text-muted-foreground",
                    )}
                  >
                    {row.name}
                  </span>
                  <span className="h-[5px] overflow-hidden rounded-full bg-muted">
                    <i
                      className={cn(
                        "block h-full rounded-full",
                        active ? "bg-[var(--brand)]" : "bg-[var(--brand)]/55",
                      )}
                      style={{ width: `${Math.max(6, (row.count / max) * 100)}%` }}
                    />
                  </span>
                  <span className="text-right font-mono tabular-nums text-muted-foreground">
                    {row.count}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </SectionCard>
  );
}

/* ── E4: 매크로 지표 (해외 전용) ──────────────────────────────────────────── */

/**
 * 해외 화면에만 둔다 — 국내 화면의 「지수 흐름」과 성격이 겹치기 때문이다.
 *
 * 목록은 **실제로 수집되는 지표만** 담았다. 목업에는 미 10년물·달러인덱스·WTI가
 * 있었지만 지금 파이프라인에 그 셋은 들어오지 않는다. 없는 줄을 만들어 두는 대신
 * 있는 것(VIX·반도체·선물·금·비트코인)으로 채운다.
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
              className="flex h-[30px] min-w-0 items-center gap-2 border-b border-border/60 px-1 text-xs last:border-b-0"
            >
              <span className="min-w-0 flex-1 truncate">{idx.name}</span>
              <span className="shrink-0 font-mono text-xs tabular-nums">
                {idx.value
                  ? idx.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })
                  : "—"}
              </span>
              <span
                className={cn(
                  "w-14 shrink-0 text-right font-mono text-xs tabular-nums",
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

/** 출처 필터를 걸었을 때 목록 제목 옆에 뜨는 해제 칩 — 어디서 걸렸는지 알 수 있게. */
export function NewsPublisherFilterChip({
  name,
  onClear,
}: {
  name: string;
  onClear: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClear}
      aria-label={`출처 ${name} 필터 해제`}
      className="inline-flex items-center gap-1 rounded-full bg-[var(--brand-muted)] px-2 py-0.5 text-[11px] font-medium text-[var(--brand)]"
    >
      {name}
      <X className="size-3" />
    </button>
  );
}
