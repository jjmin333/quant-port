"use client";

import { useEffect, useMemo, useState } from "react";
import { Sparkles } from "lucide-react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import type { LiveQuote } from "@/hooks/use-live-quotes";
import { daysUntil, type UpcomingEvent } from "@/hooks/use-upcoming-events";
import type { CommunitySymbol } from "@/lib/community-types";
import { fetchStockDailyCloses } from "@/lib/daily-close-api";
import type { DailyCloseRow } from "@/lib/daily-close-types";
import { formatPercent, formatPrice, formatRelativeTime } from "@/lib/format";
import { fetchStockNews } from "@/lib/news-api";
import type { NewsArticle } from "@/lib/news-types";
import { SCORE_TONE_BADGE, scoreTone } from "@/lib/stock-score-api";
import { firstSentences } from "@/lib/summary-sentences";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 글에 붙인 종목 한 장 (2026-09-27 사용자 요청 4 — 「종목 모달이 주는 데이터를 글 안에서」).
 *
 * 위에서 아래로: **그때 → 지금**(글 쓴 시점 가격 대비 지금), 30거래일 종가 선(점선 = 글 쓴 날),
 * AI 점수(그때·지금), 저장된 AI 요약 첫 문장, 가까운 실적 일정, 최근 뉴스 두 건.
 * 카드를 누르면 기존 종목 모달이 열려 전부(점수 근거·일별 종가·AI 요약 전문)를 본다.
 *
 * LLM을 부르지 않는다 — AI 요약은 배치가 써 둔 글이다.
 */
export function CommunitySymbolCard({
  item,
  quote,
  event,
  onOpen,
}: {
  item: CommunitySymbol;
  quote?: LiveQuote;
  event?: UpcomingEvent;
  onOpen: () => void;
}) {
  const [bars, setBars] = useState<DailyCloseRow[] | null>(null);
  const [news, setNews] = useState<NewsArticle[] | null>(null);
  const market = item.assetType === "domestic" ? "domestic" : "us";

  useEffect(() => {
    let alive = true;
    fetchStockDailyCloses(item.symbol, 30)
      .then((s) => {
        if (alive) setBars([...s.rows].sort((a, b) => a.tradeDate.localeCompare(b.tradeDate)));
      })
      .catch(() => {
        if (alive) setBars([]);
      });
    fetchStockNews({ symbol: item.symbol, name: item.name, market, limit: 2 })
      .then((rows) => {
        if (alive) setNews(rows);
      })
      .catch(() => {
        if (alive) setNews([]);
      });
    return () => {
      alive = false;
    };
  }, [item.symbol, item.name, market]);

  const currency = (quote?.currency ?? item.snapCurrency ?? (market === "domestic" ? "KRW" : "USD")) as "KRW" | "USD";
  const now = quote && quote.livePrice > 0 ? quote.livePrice : undefined;
  const sinceRate = now != null && item.snapPrice ? ((now - item.snapPrice) / item.snapPrice) * 100 : undefined;
  const ai = firstSentences(item.aiOverview, 2);
  const eventDays = event ? daysUntil(event.date) : null;

  return (
    <button
      type="button"
      onClick={onOpen}
      className="grid w-full min-w-0 grid-cols-[minmax(0,1fr)] gap-2 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 text-left transition-colors hover:border-[var(--brand)]"
      aria-label={`${item.name || item.symbol} 자세히 보기`}
    >
      <div className="flex min-w-0 items-center gap-2">
        <StockAvatar
          symbol={item.symbol}
          name={item.name}
          currency={currency}
          assetType={item.assetType as "domestic" | "us" | "etf" | undefined}
          size="xs"
          className="shrink-0"
        />
        <span className="truncate text-note font-bold text-[var(--text-primary)]">{item.name || item.symbol}</span>
        <span className="shrink-0 text-micro text-[var(--text-muted)]">{item.symbol}</span>
        <span className="ml-auto shrink-0 text-right text-cell">
          {item.snapPrice != null && (
            <span className="text-[var(--text-muted)]">
              작성 시 {formatPrice(item.snapPrice, currency)} →{" "}
            </span>
          )}
          {now != null ? (
            <>
              <b className="font-num text-[var(--text-primary)]">{formatPrice(now, currency)}</b>
              {sinceRate != null && (
                <span className={cn("ml-1 font-num font-semibold", trendTextClass(sinceRate))}>
                  ({formatPercent(sinceRate)})
                </span>
              )}
            </>
          ) : (
            <span className="text-[var(--text-muted)]">시세 불러오는 중</span>
          )}
        </span>
      </div>

      <Sparkline bars={bars} snapAt={item.snapAt} />

      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
        {item.score != null && (
          <span className={cn("rounded-md px-1.5 py-0.5 text-micro font-bold", SCORE_TONE_BADGE[scoreTone(item.score)])}>
            AI 점수 {item.score}
            {item.snapScore != null && item.snapScore !== item.score && (
              <span className="ml-1 font-normal opacity-80">(작성 시 {item.snapScore})</span>
            )}
          </span>
        )}
        {quote && quote.livePrice > 0 && (
          <span className={cn("rounded-md border border-[var(--border)] px-1.5 py-0.5 text-micro font-num", trendTextClass(quote.liveChangeRate))}>
            오늘 {formatPercent(quote.liveChangeRate)}
          </span>
        )}
        {eventDays != null && eventDays <= 30 && (
          <span className="rounded-md bg-amber-500/15 px-1.5 py-0.5 text-micro font-semibold text-amber-700 dark:text-amber-400">
            {event?.label?.trim() || "실적"} {eventDays === 0 ? "오늘" : `D-${eventDays}`}
          </span>
        )}
      </div>

      {ai && (
        <p className="flex min-w-0 gap-1.5 text-cell leading-relaxed text-[var(--text-secondary)]">
          <Sparkles className="mt-0.5 h-3.5 w-3.5 shrink-0 text-[var(--brand)]" />
          <span className="min-w-0">
            {ai}
            {item.aiAt && <span className="ml-1 text-micro text-[var(--text-muted)]">AI 요약 {item.aiAt.slice(5).replace("-", "/")}</span>}
          </span>
        </p>
      )}

      {news && news.length > 0 && (
        <ul className="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-0.5 border-t border-[var(--border)] pt-2">
          {news.map((n) => (
            <li key={n.id} className="flex min-w-0 items-center gap-2 text-cell">
              <span className="block min-w-0 flex-1 truncate text-[var(--text-secondary)]">{n.headlineKo || n.headline}</span>
              <span className="shrink-0 text-micro text-[var(--text-muted)]">{formatRelativeTime(n.publishedAt)}</span>
            </li>
          ))}
        </ul>
      )}
    </button>
  );
}

/** 30거래일 종가 선 — 점선은 글을 쓴 날. 축·눈금 없이 모양만(자세한 차트는 종목 모달이 한다). */
function Sparkline({ bars, snapAt }: { bars: DailyCloseRow[] | null; snapAt: string }) {
  const geom = useMemo(() => {
    if (!bars || bars.length < 2) return null;
    const closes = bars.map((b) => b.close);
    const min = Math.min(...closes);
    const max = Math.max(...closes);
    const span = max - min || 1;
    const W = 300;
    const H = 40;
    const pts = closes.map((c, i) => `${((i / (closes.length - 1)) * W).toFixed(1)},${(H - 2 - ((c - min) / span) * (H - 4)).toFixed(1)}`);
    const snapDate = snapAt.slice(0, 10);
    // 글을 쓴 날 이후 첫 거래일에 점선. 마지막 거래일 뒤(주말·휴장)에 썼으면 오른쪽 끝에.
    const idx = bars.findIndex((b) => b.tradeDate >= snapDate);
    const markX =
      idx >= 0 ? (idx / (bars.length - 1)) * W : snapDate > bars[bars.length - 1].tradeDate ? W - 1 : null;
    const up = closes[closes.length - 1] >= closes[0];
    return { pts: pts.join(" "), markX, up, W, H };
  }, [bars, snapAt]);

  if (bars === null) return <div className="h-10 animate-pulse rounded bg-[var(--surface-elevated)]" />;
  if (!geom) return null;
  return (
    <svg viewBox={`0 0 ${geom.W} ${geom.H}`} preserveAspectRatio="none" className="h-10 w-full" aria-hidden>
      <polyline
        fill="none"
        stroke={geom.up ? "var(--profit)" : "var(--loss)"}
        strokeWidth="1.6"
        vectorEffect="non-scaling-stroke"
        points={geom.pts}
      />
      {geom.markX != null && (
        <line
          x1={geom.markX}
          x2={geom.markX}
          y1="0"
          y2={geom.H}
          stroke="var(--text-muted)"
          strokeDasharray="3 3"
          vectorEffect="non-scaling-stroke"
        />
      )}
    </svg>
  );
}
