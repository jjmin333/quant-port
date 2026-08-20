"use client";

import { useMemo } from "react";
import { Flame } from "lucide-react";
import {
  pickMarketIndices,
  type IndexPulseFocus,
} from "@/components/market/index-pulse-card";
import { useMarketIndices } from "@/hooks/use-market-indices";
import type { SymbolMention } from "@/lib/news-derive";
import {
  countArticlesByDashboardIndex,
  type DashboardIndexId,
} from "@/lib/news-dashboard-indices";
import type { NewsArticle } from "@/lib/news-types";
import type { QuoteSummary } from "@/lib/quotes-types";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

const pulseCardClass =
  "min-w-0 rounded-xl border border-[var(--border)] bg-[var(--surface)] px-2 py-1.5 text-left transition-colors hover:border-[var(--text-muted)]";

/**
 * A1 — 중앙 상단 지수·이슈 종목 스트립.
 *
 * 지수·종목 카드 모두 등락률 + 기사 N건을 보여 주고, 클릭하면 해당 키워드로 목록을
 * 좁힌다.
 */
export function NewsIssueStrip({
  market,
  articles,
  mentions,
  quotes,
  activeSymbol,
  activeIndexId,
  onSelect,
  onSelectIndex,
}: {
  market: IndexPulseFocus;
  articles: NewsArticle[];
  mentions: SymbolMention[];
  quotes: Map<string, QuoteSummary>;
  activeSymbol: string | null;
  activeIndexId: DashboardIndexId | null;
  onSelect: (symbol: string) => void;
  onSelectIndex: (indexId: DashboardIndexId) => void;
}) {
  const { indices } = useMarketIndices();
  const indexRows = useMemo(() => pickMarketIndices(indices, market), [indices, market]);
  const hasIndexData = indexRows.some((i) => i.value > 0);

  const indexCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const idx of indexRows) {
      counts.set(idx.id, countArticlesByDashboardIndex(articles, idx.id as DashboardIndexId));
    }
    return counts;
  }, [articles, indexRows]);

  if (!hasIndexData && mentions.length === 0) return null;

  const maxMention = mentions[0]?.count || 1;
  const maxIndexMention = Math.max(...indexRows.map((idx) => indexCounts.get(idx.id) ?? 0), 1);

  return (
    <div className="grid grid-cols-3 gap-1.5 @2xl:grid-cols-6">
      {hasIndexData &&
        indexRows.map((idx) => {
          const count = indexCounts.get(idx.id) ?? 0;
          const active = activeIndexId === idx.id;

          return (
            <button
              key={idx.id}
              type="button"
              onClick={() => onSelectIndex(idx.id as DashboardIndexId)}
              aria-pressed={active}
              className={cn(
                pulseCardClass,
                active && "border-[var(--brand)] bg-[var(--brand-muted)]",
              )}
            >
              <div className="truncate text-[11px] font-semibold text-[var(--text-primary)]">
                {idx.name}
              </div>
              <div
                className={cn(
                  "font-mono text-[13px] font-bold tabular-nums",
                  trendTextClass(idx.changeRate),
                )}
              >
                {formatPercent(idx.changeRate)}
              </div>
              <div className="text-[10px] text-[var(--text-muted)]">{count}건</div>
              <div className="mt-1 h-[3px] overflow-hidden rounded-full bg-[var(--surface-elevated)]">
                <i
                  className="block h-full rounded-full bg-[var(--brand)]"
                  style={{
                    width: `${Math.max(8, (count / maxIndexMention) * 100)}%`,
                  }}
                />
              </div>
            </button>
          );
        })}

      {mentions.map((mention) => {
        const quote = quotes.get(mention.symbol);
        const active = activeSymbol === mention.symbol;
        const hasPrice = Boolean(quote && quote.currentPrice > 0);

        return (
          <button
            key={mention.symbol}
            type="button"
            onClick={() => onSelect(mention.symbol)}
            aria-pressed={active}
            className={cn(
              pulseCardClass,
              active && "border-[var(--brand)] bg-[var(--brand-muted)]",
            )}
          >
            <div className="truncate text-[11px] font-semibold text-[var(--text-primary)]">
              {quote?.name || mention.symbol}
            </div>
            <div
              className={cn(
                "font-mono text-[13px] font-bold tabular-nums",
                hasPrice ? trendTextClass(quote!.changeRate) : "text-[var(--text-muted)]",
              )}
            >
              {hasPrice ? formatPercent(quote!.changeRate) : "—"}
            </div>
            <div className="flex items-center gap-1 text-[10px] text-[var(--text-muted)]">
              <span>{mention.count}건</span>
              {mention.hasBreaking && <Flame className="size-3 text-[var(--loss)]" />}
            </div>
            <div className="mt-1 h-[3px] overflow-hidden rounded-full bg-[var(--surface-elevated)]">
              <i
                className="block h-full rounded-full bg-[var(--brand)]"
                style={{ width: `${Math.max(8, (mention.count / maxMention) * 100)}%` }}
              />
            </div>
          </button>
        );
      })}
    </div>
  );
}
