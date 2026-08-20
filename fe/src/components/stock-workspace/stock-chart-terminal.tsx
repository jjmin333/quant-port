"use client";

import { useRef } from "react";
import type { CandlestickChartHandle } from "@/components/quotes/candlestick-chart";
import { ChartZoomToolbar } from "@/components/quotes/chart-zoom-toolbar";
import { QuoteChartSlot } from "@/components/quotes/quote-chart-slot";
import { PriceDisplay } from "@/components/ui/price-display";
import { useQuotesChartPrefs } from "@/hooks/use-quotes-chart-prefs";
import { useExchangeRate } from "@/hooks/use-exchange-rate";
import { useQuoteCandles } from "@/hooks/use-quote-candles";
import { formatPercent } from "@/lib/format";
import type { CandleInterval, ChartStyle, OrderBook, QuoteDetail, TradeTick } from "@/lib/quotes-types";
import { quoteChangeBaseline } from "@/lib/quotes-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

const intervals: { id: CandleInterval; label: string }[] = [
  { id: "1m", label: "1" },
  { id: "5m", label: "5" },
  { id: "10m", label: "10" },
  { id: "30m", label: "30" },
  { id: "60m", label: "60" },
  { id: "1d", label: "일" },
  { id: "1w", label: "주" },
  { id: "1M", label: "월" },
];

const chartStyles: { id: ChartStyle; label: string; icon: ChartStyleIcon }[] = [
  { id: "bar", label: "바", icon: BarChartIcon },
  { id: "candle", label: "캔들", icon: CandleChartIcon },
  { id: "area", label: "영역", icon: AreaChartIcon },
];

type StockChartTerminalProps = {
  symbol: string;
  detail: QuoteDetail | null;
  orderBook: OrderBook | null;
  trades: TradeTick[];
  loading?: boolean;
  compact?: boolean;
  className?: string;
};

export function StockChartTerminal({
  symbol,
  detail,
  orderBook,
  trades,
  loading = false,
  compact = false,
  className,
}: StockChartTerminalProps) {
  const chartRef = useRef<CandlestickChartHandle>(null);
  const { interval, chartStyle, setInterval, setChartStyle } = useQuotesChartPrefs();
  const exchangeRate = useExchangeRate(true);
  const {
    candles,
    loading: candlesLoading,
    loadMore,
    hasMore,
    loadingMore,
  } = useQuoteCandles(symbol, interval, detail?.currentPrice ?? 0);

  if (loading && !detail) {
    return (
      <div
        className={cn(
          "flex flex-1 items-center justify-center text-sm text-[var(--text-muted)]",
          className,
        )}
      >
        시세 로딩 중…
      </div>
    );
  }

  if (!detail) {
    return (
      <div
        className={cn(
          "flex flex-1 items-center justify-center text-sm text-[var(--text-muted)]",
          className,
        )}
      >
        종목을 선택해 주세요.
      </div>
    );
  }

  const asks = orderBook?.asks ?? [];
  const bids = orderBook?.bids ?? [];
  const currency = detail.currency;

  const maxBookQty = Math.max(
    ...asks.map((a) => a.quantity),
    ...bids.map((b) => b.quantity),
    1,
  );

  function DetailPrice({ value, className: priceClassName }: { value: number; className?: string }) {
    if (value <= 0) return <>—</>;
    return (
      <PriceDisplay
        value={value}
        currency={currency}
        size="sm"
        className={cn("text-xs font-medium", priceClassName)}
      />
    );
  }

  return (
    // Two layouts, switched purely in CSS at `xl`:
    //   < xl  — one scrollable column (chart → 체결 → 호가), each block at its
    //           natural height. `compact` (< 1024) and laptop widths share it.
    //   >= xl — chart/체결 column beside a fixed-width 호가 rail, no page scroll.
    // The stacked layout must scroll: when it did not, the `shrink-0` 호가 rail
    // squeezed the chart column to 0px between 1024–1279px and the chart and
    // 체결 disappeared entirely.
    <div
      className={cn(
        "min-h-0 flex-1 overflow-y-auto overflow-x-hidden",
        "xl:flex xl:flex-row xl:overflow-hidden",
        className,
      )}
    >
      <div className="flex flex-col xl:min-h-0 xl:min-w-0 xl:flex-1 xl:overflow-hidden">
        <section
          className={cn(
            "flex flex-col border-b border-[var(--border)] xl:border-b-0",
            compact ? "min-h-[240px]" : "min-h-[300px] xl:flex-1",
          )}
        >
          <div className="flex shrink-0 items-center justify-between gap-2 border-b border-[var(--border)] px-3 py-2">
            <div className="flex min-w-0 flex-1 items-center gap-2 overflow-x-auto">
              <h3 className="shrink-0 text-xs font-medium text-[var(--text-secondary)]">차트</h3>
              <div className="flex shrink-0 items-center gap-0.5 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] p-0.5">
                {intervals.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setInterval(item.id)}
                    className={cn(
                      "shrink-0 rounded-md px-2.5 py-1 text-[11px] font-medium transition-colors",
                      interval === item.id
                        ? "bg-[var(--chart-toolbar-active-bg)] text-[var(--chart-toolbar-active-text)]"
                        : "text-[var(--text-muted)] hover:bg-[var(--surface)] hover:text-[var(--text-secondary)]",
                    )}
                  >
                    {item.label}
                  </button>
                ))}
                <div className="mx-0.5 h-4 w-px shrink-0 bg-[var(--border)]" aria-hidden />
                {chartStyles.map((item) => {
                  const Icon = item.icon;
                  return (
                    <button
                      key={item.id}
                      type="button"
                      title={item.label}
                      aria-label={`${item.label} 차트`}
                      onClick={() => setChartStyle(item.id)}
                      className={cn(
                        "shrink-0 rounded-md p-1.5 transition-colors",
                        chartStyle === item.id
                          ? "bg-[var(--chart-toolbar-active-bg)] text-[var(--chart-toolbar-active-text)]"
                          : "text-[var(--text-muted)] hover:bg-[var(--surface)] hover:text-[var(--text-secondary)]",
                      )}
                    >
                      <Icon className="h-3.5 w-3.5" />
                    </button>
                  );
                })}
              </div>
            </div>
            <ChartZoomToolbar
              className="shrink-0"
              onZoomIn={() => chartRef.current?.zoomIn()}
              onZoomOut={() => chartRef.current?.zoomOut()}
              onReset={() => chartRef.current?.resetZoom()}
            />
          </div>
          <div className={cn("flex-1 p-1", compact ? "min-h-[220px]" : "min-h-[260px]")}>
            <QuoteChartSlot
              chartRef={chartRef}
              candles={candles}
              loading={candlesLoading}
              currency={detail.currency}
              interval={interval}
              chartStyle={chartStyle}
              height={compact ? 240 : 320}
              hasMore={hasMore}
              loadingMore={loadingMore}
              onLoadMore={loadMore}
            />
          </div>
        </section>

        <section
          className={cn(
            "flex shrink-0 flex-col overflow-hidden",
            compact ? "h-[200px]" : "h-[220px] xl:h-[240px]",
          )}
        >
          <div className="flex shrink-0 items-center justify-between border-b border-[var(--border)] px-3 py-2">
            <h3 className="text-xs font-medium text-[var(--text-secondary)]">체결</h3>
            <span className="text-[10px] text-[var(--text-muted)]">실시간</span>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto">
            <table className="w-full text-[11px]">
              <thead className="sticky top-0 bg-[var(--surface)]">
                <tr className="text-[var(--text-muted)]">
                  <th className="px-3 py-1.5 text-left font-normal">시간</th>
                  <th className="px-3 py-1.5 text-right font-normal">체결가</th>
                  <th className="px-3 py-1.5 text-right font-normal">체결량</th>
                  <th className="px-3 py-1.5 text-right font-normal">등락률</th>
                </tr>
              </thead>
              <tbody>
                {trades.length === 0 ? (
                  <tr>
                    <td colSpan={4} className="px-3 py-6 text-center text-[var(--text-muted)]">
                      체결 내역 없음
                    </td>
                  </tr>
                ) : (
                  trades.map((trade, index) => {
                    const baseline = detail ? quoteChangeBaseline(detail) : 0;
                    const tickRate =
                      baseline > 0 ? ((trade.price - baseline) / baseline) * 100 : 0;
                    return (
                      <tr
                        key={`${trade.id}-${index}`}
                        className="border-t border-[var(--border)]/40 hover:bg-[var(--surface-elevated)]/50"
                      >
                        <td className="px-3 py-1 font-mono text-[var(--text-muted)]">
                          {formatTradeTime(trade.time)}
                        </td>
                        <td
                          className={cn(
                            "px-3 py-1 text-right font-mono font-medium",
                            trade.side === "buy" && "text-[var(--profit)]",
                            trade.side === "sell" && "text-[var(--loss)]",
                            trade.side !== "buy" &&
                              trade.side !== "sell" &&
                              trendTextClass(tickRate),
                          )}
                        >
                          <DetailPrice value={trade.price} />
                        </td>
                        <td className="px-3 py-1 text-right font-mono text-[var(--text-secondary)]">
                          {trade.quantity.toLocaleString("ko-KR")}
                        </td>
                        <td
                          className={cn(
                            "px-3 py-1 text-right font-mono",
                            trendTextClass(tickRate),
                          )}
                        >
                          {baseline > 0 ? formatPercent(tickRate) : "—"}
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <aside
        className={cn(
          "flex w-full shrink-0 flex-col border-t border-[var(--border)]",
          "xl:w-[300px] xl:min-h-0 xl:overflow-hidden xl:border-l xl:border-t-0",
          "min-h-[320px]",
        )}
      >
        <div className="flex shrink-0 items-center justify-between border-b border-[var(--border)] px-3 py-2">
          <h3 className="text-xs font-medium text-[var(--text-secondary)]">호가</h3>
          <span className="text-[10px] text-[var(--text-muted)]">
            {orderBook?.timestamp
              ? new Date(orderBook.timestamp).toLocaleTimeString("ko-KR", { timeZone: "Asia/Seoul" })
              : "—"}
          </span>
        </div>
        <div className="grid shrink-0 grid-cols-3 px-2 py-1 text-[10px] text-[var(--text-muted)]">
          <span>호가</span>
          <span className="text-center">잔량</span>
          <span className="text-right">건수</span>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
          {[...asks].reverse().map((level) => (
            <OrderBookRow
              key={`ask-${level.price}`}
              price={level.price}
              quantity={level.quantity}
              orderCount={level.orderCount}
              currency={detail.currency}
              exchangeRate={exchangeRate}
              side="ask"
              maxQty={maxBookQty}
            />
          ))}
          <div className="my-1 border-y border-[var(--border)] py-2 text-center">
            <PriceDisplay
              value={detail.currentPrice}
              currency={detail.currency}
              size="sm"
              className="text-sm font-semibold"
            />
          </div>
          {bids.map((level) => (
            <OrderBookRow
              key={`bid-${level.price}`}
              price={level.price}
              quantity={level.quantity}
              orderCount={level.orderCount}
              currency={detail.currency}
              exchangeRate={exchangeRate}
              side="bid"
              maxQty={maxBookQty}
            />
          ))}
        </div>
      </aside>
    </div>
  );
}

type ChartStyleIcon = (props: { className?: string }) => React.JSX.Element;

function BarChartIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="none" className={className} aria-hidden>
      <path d="M8 2v12" stroke="currentColor" strokeWidth="1.25" />
      <path d="M5.5 5.5h1.5M5.5 8.5h1.5M9 4.5h1.5M9 10h1.5" stroke="currentColor" strokeWidth="1.25" />
    </svg>
  );
}

function CandleChartIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="none" className={className} aria-hidden>
      <path d="M5 2.5v11M11 4v8" stroke="currentColor" strokeWidth="1.25" />
      <rect x="3.5" y="5.5" width="3" height="4" rx="0.5" stroke="currentColor" strokeWidth="1.25" />
      <rect x="9.5" y="6.5" width="3" height="3" rx="0.5" stroke="currentColor" strokeWidth="1.25" />
    </svg>
  );
}

function AreaChartIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="none" className={className} aria-hidden>
      <path
        d="M2 11.5 5.5 8.5 8.5 10.5 11 7 14 9.5V13H2z"
        fill="currentColor"
        fillOpacity="0.25"
      />
      <path
        d="M2 11.5 5.5 8.5 8.5 10.5 11 7 14 9.5"
        stroke="currentColor"
        strokeWidth="1.25"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function formatTradeTime(raw: string): string {
  if (/^\d{2}:\d{2}(:\d{2})?$/.test(raw)) return raw.slice(0, 8);
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return raw;
  return d.toLocaleTimeString("ko-KR", {
    timeZone: "Asia/Seoul",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function OrderBookRow({
  price,
  quantity,
  orderCount,
  currency,
  exchangeRate,
  side,
  maxQty,
}: {
  price: number;
  quantity: number;
  orderCount: number;
  currency: "KRW" | "USD";
  exchangeRate?: number;
  side: "ask" | "bid";
  maxQty: number;
}) {
  const barWidth = `${(quantity / maxQty) * 100}%`;

  return (
    <div className="relative flex items-center py-0.5 text-[11px]">
      <div
        className={cn(
          "absolute inset-y-0 opacity-25",
          side === "ask" ? "right-0 bg-red-500" : "left-0 bg-blue-500",
        )}
        style={{ width: barWidth }}
      />
      <span
        className={cn(
          "relative z-10 w-[38%] truncate",
          side === "ask" ? "text-right text-[var(--profit)]" : "text-[var(--loss)]",
        )}
      >
        <PriceDisplay
          value={price}
          currency={currency}
          size="sm"
          className="text-[11px] font-medium"
        />
      </span>
      <span className="relative z-10 flex-1 text-center font-mono text-[var(--text-primary)]">
        {quantity.toLocaleString("ko-KR")}
      </span>
      <span className="relative z-10 w-[18%] text-right text-[var(--text-muted)]">
        {orderCount > 0 ? orderCount : "—"}
      </span>
    </div>
  );
}
