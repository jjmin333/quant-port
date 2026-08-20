"use client";

import dynamic from "next/dynamic";
import { Star } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { parseItemChangeRate } from "@/components/market-radar/market-radar-sparkline";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  MARKET_RADAR_SHOW_SPARKLINES,
  parseRankingNumber,
  rankingMetricLabel,
} from "@/lib/market-radar-config";
import type { EnrichedRankingItem, MarketRadarParams, RankingType } from "@/lib/market-radar-types";
import { formatPercent, formatTradingValue, formatVolume } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

const MarketRadarSparkline = dynamic(
  () =>
    import("@/components/market-radar/market-radar-sparkline").then((m) => ({
      default: m.MarketRadarSparkline,
    })),
  { ssr: false },
);

type MarketRadarListProps = {
  items: EnrichedRankingItem[];
  selectedSymbol: string;
  rankingType: RankingType;
  radarParams: MarketRadarParams;
  loading: boolean;
  compact?: boolean;
  onSelect: (symbol: string) => void;
};

export function MarketRadarList({
  items,
  selectedSymbol,
  rankingType,
  radarParams,
  loading,
  compact = false,
  onSelect,
}: MarketRadarListProps) {
  const metric = rankingMetricLabel(rankingType);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col bg-[var(--surface)]">
      <div
        className={cn(
          "min-h-0 flex-1 overflow-y-auto overscroll-contain",
          // Touch viewports get a little runway past the last row so the final
          // item clears the home indicator; desktop needs no filler.
          compact ? "pb-[env(safe-area-inset-bottom,0px)]" : "pb-1",
        )}
      >
        {loading && items.length === 0 ? (
          <p className="p-4 text-center text-xs text-[var(--text-muted)]">랭킹 로딩 중…</p>
        ) : items.length === 0 ? (
          <p className="p-4 text-center text-xs text-[var(--text-muted)]">
            {radarParams.duration === "realtime"
              ? "표시할 종목이 없습니다."
              : "해당 기간 랭킹 데이터를 수집 중이거나 표시할 종목이 없습니다."}
          </p>
        ) : (
          items.map((item) => (
            <RankingRow
              key={`${item.rank}-${item.symbol}`}
              item={item}
              metric={metric}
              active={item.symbol === selectedSymbol}
              compact={compact}
              onSelect={onSelect}
            />
          ))
        )}
      </div>
    </div>
  );
}

function RankingRow({
  item,
  metric,
  active,
  compact,
  onSelect,
}: {
  item: EnrichedRankingItem;
  metric: "amount" | "volume";
  active: boolean;
  compact?: boolean;
  onSelect: (symbol: string) => void;
}) {
  const price = parseRankingNumber(item.price.lastPrice);
  const changeRate = parseItemChangeRate(
    item.price.changeRate,
    item.price.lastPrice,
    item.price.basePrice,
  );
  const currency = item.currency;

  const metricLabel =
    metric === "volume"
      ? formatVolume(parseRankingNumber(item.tradingVolume))
      : formatTradingValue(parseRankingNumber(item.tradingAmount), currency, 0);

  const priceText =
    price > 0
      ? currency === "USD"
        ? `$${price.toLocaleString("en-US", { maximumFractionDigits: 2 })}`
        : `${Math.round(price).toLocaleString("ko-KR")}원`
      : "—";

  return (
    <button
      type="button"
      onClick={() => onSelect(item.symbol)}
      aria-current={active ? "true" : undefined}
      className={cn(
        "flex w-full items-center border-b border-[var(--border)]/70 text-left transition-colors",
        // 44px+ tap target and roomier type on touch; dense rows on desktop
        // where a pointer and more vertical room are available.
        compact ? "gap-2.5 px-3 py-2.5" : "gap-1.5 px-2 py-1",
        active ? "bg-[var(--brand-muted)]" : "hover:bg-[var(--surface-elevated)]",
      )}
    >
      <div
        className={cn(
          "flex shrink-0 items-baseline justify-end gap-0.5 tabular-nums",
          compact ? "w-8" : "w-7",
        )}
      >
        <span
          className={cn(
            "font-mono font-semibold text-[var(--text-primary)]",
            compact ? "text-[13px]" : "text-[11px]",
          )}
        >
          {item.rank}
        </span>
        {item.rankDelta != null && item.rankDelta !== 0 && (
          <span
            className={cn(
              "font-medium",
              compact ? "text-[9px]" : "text-[8px]",
              item.rankDelta > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
            )}
          >
            {item.rankDelta > 0 ? `↑${item.rankDelta}` : `↓${Math.abs(item.rankDelta)}`}
          </span>
        )}
      </div>

      <StockAvatar
        symbol={item.symbol}
        name={item.name}
        currency={currency}
        assetType={item.assetType}
        size="xs"
        className={cn("shrink-0", compact ? "h-7 w-7 text-[10px]" : "h-5 w-5 text-[8px]")}
      />

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1">
          <span
            className={cn(
              "truncate font-medium leading-tight text-[var(--text-primary)]",
              compact ? "text-[13px]" : "text-[11px]",
            )}
          >
            {item.name}
          </span>
          {item.isHeld && (
            <Badge
              variant="info"
              className={cn(
                "shrink-0 px-1 py-0 leading-none",
                compact ? "text-[9px]" : "text-[7px]",
              )}
            >
              보유
            </Badge>
          )}
          {item.isWatchlisted && (
            <Star
              className={cn(
                "shrink-0 fill-amber-400 text-amber-400",
                compact ? "h-3.5 w-3.5" : "h-3 w-3",
              )}
              aria-label="관심 종목"
            />
          )}
        </div>
        <p
          className={cn(
            "truncate leading-tight text-[var(--text-muted)]",
            compact ? "text-[11px]" : "text-[9px]",
          )}
        >
          {/* The ranking metric is the reason the row is in this list at all, so
              it stays visible on touch too — it used to be desktop-only. */}
          {item.symbol}
          {compact && metricLabel && ` · ${metricLabel}`}
        </p>
      </div>

      {!compact && MARKET_RADAR_SHOW_SPARKLINES && (
        <MarketRadarSparkline symbol={item.symbol} changeRate={changeRate} className="hidden sm:flex" />
      )}

      <div className="shrink-0 text-right leading-tight">
        <p
          className={cn(
            "font-mono tabular-nums text-[var(--text-primary)]",
            compact ? "text-[13px]" : "text-[11px]",
          )}
        >
          {priceText}
        </p>
        <p
          className={cn(
            "font-mono tabular-nums",
            compact ? "text-[12px]" : "text-[10px]",
            trendTextClass(changeRate),
          )}
        >
          {price > 0 ? formatPercent(changeRate) : "—"}
        </p>
        {!compact && metricLabel && (
          <p className="text-[9px] tabular-nums text-[var(--text-muted)]">{metricLabel}</p>
        )}
      </div>
    </button>
  );
}
