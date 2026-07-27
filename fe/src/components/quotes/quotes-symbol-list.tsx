"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { Star } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { QuotePrice } from "@/components/quotes/quote-price";
import { formatVolume, formatPercent } from "@/lib/format";
import type { QuoteFilter, QuoteSummary } from "@/lib/quotes-types";
import { useQuotesList } from "@/hooks/use-quotes-list";
import { cn } from "@/lib/utils";
import { trendTextClass } from "@/lib/trend-colors";

const filters: { id: QuoteFilter; label: string }[] = [
  { id: "all", label: "전체" },
  { id: "domestic", label: "국내" },
  { id: "us", label: "미국" },
  { id: "watchlist", label: "관심" },
  { id: "held", label: "보유" },
];

type QuotesSymbolListProps = {
  selectedSymbol: string;
  onSelect: (symbol: string) => void;
  /** From global header search on /quotes */
  searchQuery?: string;
};

export function QuotesSymbolList({
  selectedSymbol,
  onSelect,
  searchQuery = "",
}: QuotesSymbolListProps) {
  const [filter, setFilter] = useState<QuoteFilter>("all");
  const { items, loading } = useQuotesList(filter, searchQuery);

  const displayItems = useMemo(() => items, [items]);

  return (
    <div className="flex h-full min-w-[220px] shrink-0 flex-col border-r border-[var(--border)] bg-[var(--surface)] lg:min-w-[260px] lg:max-w-[280px]">
      <div className="shrink-0 space-y-3 border-b border-[var(--border)] p-3">
        <div className="flex flex-wrap gap-1">
          {filters.map((f) => (
            <button
              key={f.id}
              type="button"
              onClick={() => setFilter(f.id)}
              className={cn(
                "rounded-md px-2 py-0.5 text-[11px] font-medium transition-colors",
                filter === f.id
                  ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                  : "text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]",
              )}
            >
              {f.label}
            </button>
          ))}
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {loading && displayItems.length === 0 ? (
          <p className="p-4 text-center text-xs text-[var(--text-muted)]">로딩 중…</p>
        ) : (
          displayItems.map((item) => (
            <SymbolRow
              key={item.symbol}
              item={item}
              active={item.symbol === selectedSymbol}
              onSelect={onSelect}
            />
          ))
        )}
        {!loading && displayItems.length === 0 && (
          <p className="p-4 text-center text-xs text-[var(--text-muted)]">
            검색 결과가 없습니다.
          </p>
        )}
      </div>
    </div>
  );
}

function SymbolRow({
  item,
  active,
  onSelect,
}: {
  item: QuoteSummary;
  active: boolean;
  onSelect: (symbol: string) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onSelect(item.symbol)}
      className={cn(
        "flex w-full items-center justify-between gap-2 border-b border-[var(--border)] px-3 py-2.5 text-left transition-colors",
        active ? "bg-[var(--accent-muted)]" : "hover:bg-[var(--surface-elevated)]",
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span className="truncate text-sm font-medium text-[var(--text-primary)]">
            {item.name}
          </span>
          {item.isWatchlisted && (
            <Star className="h-3 w-3 shrink-0 fill-amber-400 text-amber-400" />
          )}
        </div>
        <div className="mt-0.5 flex items-center gap-1.5">
          <span className="text-[10px] text-[var(--text-muted)]">{item.symbol}</span>
          <Badge variant="default" className="px-1 py-0 text-[9px]">
            {item.market}
          </Badge>
        </div>
      </div>
      <div className="shrink-0 text-right">
        <QuotePrice value={item.currentPrice} currency={item.currency} size="sm" />
        <p
          className={cn(
            "mt-0.5 font-mono text-[11px]",
            item.currentPrice > 0 ? trendTextClass(item.changeRate) : "text-[var(--text-muted)]",
          )}
        >
          {item.currentPrice > 0 ? formatPercent(item.changeRate) : "—"}
        </p>
        <p className="text-[10px] text-[var(--text-muted)]">{formatVolume(item.volume)}</p>
      </div>
    </button>
  );
}

export function QuotesSymbolListFooter({ symbol }: { symbol: string }) {
  return (
    <div className="shrink-0 border-t border-[var(--border)] p-2">
      <Link
        href={`/stocks/${symbol}`}
        className="block rounded-lg bg-[var(--surface-elevated)] px-3 py-2 text-center text-xs text-[var(--accent)] hover:bg-[var(--accent-muted)]"
      >
        종목 상세 보기 →
      </Link>
    </div>
  );
}
