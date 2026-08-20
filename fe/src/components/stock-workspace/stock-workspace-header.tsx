"use client";

import { ChevronLeft } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { QuoteChange, QuotePrice } from "@/components/quotes/quote-price";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { StockSymbolActionBar } from "@/components/stocks/stock-symbol-action-bar";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import { quoteChangeBaseline, quoteChangeBasisLabel } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

type StockWorkspaceHeaderProps = {
  stock: StockInfo;
  quote: QuoteDetail | null;
  exchangeRate: number;
  loading?: boolean;
  watchlisted: boolean;
  watchlistBusy: boolean;
  onToggleWatchlist: () => void;
  onBack?: () => void;
  compact?: boolean;
  /** Show live price, pulse badge (default true). */
  showLiveQuote?: boolean;
  /** Chart interval on screen — frozen into a stamp when one is taken. */
  chartInterval?: string;
};

export function StockWorkspaceHeader({
  stock,
  quote,
  exchangeRate,
  loading,
  watchlisted,
  watchlistBusy,
  onToggleWatchlist,
  onBack,
  compact = false,
  showLiveQuote = true,
  chartInterval,
}: StockWorkspaceHeaderProps) {
  const updatedLabel =
    quote?.updatedAt &&
    new Date(quote.updatedAt).toLocaleTimeString("ko-KR", {
      timeZone: "Asia/Seoul",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });

  return (
    <header
      className={cn(
        "shrink-0 border-b border-[var(--border)] bg-[var(--surface)]",
        compact ? "px-3 py-2" : "px-3 py-2.5 sm:px-4",
      )}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0 flex flex-1 items-start gap-2">
          {onBack && (
            <button
              type="button"
              onClick={onBack}
              className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-[var(--border)] text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
              aria-label="목록으로"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
          )}
          <StockAvatar
            symbol={stock.symbol}
            name={stock.name}
            currency={stock.currency}
            assetType={stock.assetType}
            size={compact ? "sm" : "md"}
          />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-1.5">
              <h1
                className={cn(
                  "truncate font-semibold text-[var(--text-primary)]",
                  compact ? "text-sm" : "text-base sm:text-lg",
                )}
              >
                {loading ? "…" : stock.name}
              </h1>
              <span className="font-mono text-[10px] text-[var(--text-muted)]">{stock.symbol}</span>
              <Badge variant="default" className="px-1 py-0 text-[9px]">
                {stock.market}
              </Badge>
              {quote?.isHeld && (
                <Badge variant="info" className="px-1 py-0 text-[9px]">
                  보유
                </Badge>
              )}
              {showLiveQuote && quote && quote.currentPrice > 0 && (
                <span className="inline-flex items-center gap-1 text-[9px] text-[var(--profit)]">
                  <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[var(--profit)]" />
                  실시간
                </span>
              )}
            </div>
            {showLiveQuote && updatedLabel && quote && (
              <p className="mt-0.5 text-[10px] text-[var(--text-muted)]">
                {updatedLabel} 기준 · {quoteChangeBasisLabel(quote)}
              </p>
            )}
          </div>
        </div>
        <StockSymbolActionBar
          symbol={stock.symbol}
          watchlisted={watchlisted}
          watchlistBusy={watchlistBusy}
          onToggleWatchlist={onToggleWatchlist}
          chartInterval={chartInterval}
          price={quote?.currentPrice}
          currency={quote?.currency}
          week52High={quote?.week52High}
          week52Low={quote?.week52Low}
        />
      </div>

      {showLiveQuote && quote && quote.currentPrice > 0 && (
        <div className={cn("mt-2 flex flex-wrap items-end gap-x-3 gap-y-1", onBack && "pl-9")}>
          <QuotePrice
            value={quote.currentPrice}
            currency={quote.currency}
            exchangeRate={exchangeRate}
            size="xl"
            className="!text-xl sm:!text-2xl"
          />
          <QuoteChange
            change={quote.change}
            changeRate={quote.changeRate}
            currency={quote.currency}
            exchangeRate={exchangeRate}
            size="sm"
            hasBaseline={quoteChangeBaseline(quote) > 0}
          />
        </div>
      )}
    </header>
  );
}
