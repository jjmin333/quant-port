"use client";

import { useEffect, type RefObject } from "react";
import { X } from "lucide-react";
import {
  CandlestickChart,
  type CandlestickChartHandle,
} from "@/components/quotes/candlestick-chart";
import { ChartZoomToolbar } from "@/components/quotes/chart-zoom-toolbar";
import { useQuotesChartPrefs } from "@/hooks/use-quotes-chart-prefs";
import type { Candle, CandleInterval, ChartStyle } from "@/lib/quotes-types";
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

type ChartFullscreenModalProps = {
  open: boolean;
  onClose: () => void;
  title: string;
  symbol: string;
  currency: "KRW" | "USD";
  candles: Candle[];
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
  source?: string | null;
  chartRef: RefObject<CandlestickChartHandle | null>;
};

export function ChartFullscreenModal({
  open,
  onClose,
  title,
  symbol,
  currency,
  candles,
  hasMore,
  loadingMore,
  onLoadMore,
  source,
  chartRef,
}: ChartFullscreenModalProps) {
  const { interval, chartStyle, setInterval, setChartStyle } = useQuotesChartPrefs();

  useEffect(() => {
    if (!open) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKeyDown);

    return () => {
      document.body.style.overflow = prev;
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex flex-col">
      <button
        type="button"
        aria-label="닫기"
        className="absolute inset-0 bg-black/50"
        onClick={onClose}
      />

      <div
        role="dialog"
        aria-modal="true"
        aria-label={`${title} 차트`}
        className="relative mx-auto flex h-full w-full max-w-[1400px] flex-col bg-[var(--surface)] shadow-2xl sm:my-4 sm:h-[calc(100%-2rem)] sm:rounded-xl sm:border sm:border-[var(--border)]"
      >
        <header className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-[var(--border)] px-4 py-3">
          <div className="min-w-0">
            <h2 className="truncate text-base font-semibold text-[var(--text-primary)]">{title}</h2>
            <p className="text-xs text-[var(--text-muted)]">{symbol}</p>
          </div>
          <div className="flex items-center gap-2">
            <ChartZoomToolbar
              onZoomIn={() => chartRef.current?.zoomIn()}
              onZoomOut={() => chartRef.current?.zoomOut()}
              onReset={() => chartRef.current?.resetZoom()}
            />
            <button
              type="button"
              aria-label="전체 화면 닫기"
              onClick={onClose}
              className="flex h-8 w-8 items-center justify-center rounded-md border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </header>

        <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-[var(--border)] px-4 py-2">
          <div className="flex items-center gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5">
            {intervals.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => setInterval(item.id)}
                className={cn(
                  "rounded-md px-2.5 py-1 text-[11px] font-medium transition-colors",
                  interval === item.id
                    ? "bg-[var(--chart-toolbar-active-bg)] text-[var(--chart-toolbar-active-text)]"
                    : "text-[var(--text-muted)] hover:bg-[var(--surface)] hover:text-[var(--text-secondary)]",
                )}
              >
                {item.label}
              </button>
            ))}
          </div>
          <div className="flex items-center gap-1">
            {(["bar", "candle", "area"] as ChartStyle[]).map((style) => (
              <button
                key={style}
                type="button"
                onClick={() => setChartStyle(style)}
                className={cn(
                  "rounded-md px-2.5 py-1 text-[11px] font-medium transition-colors",
                  chartStyle === style
                    ? "bg-[var(--chart-toolbar-active-bg)] text-[var(--chart-toolbar-active-text)]"
                    : "text-[var(--text-muted)] hover:bg-[var(--surface)] hover:text-[var(--text-secondary)]",
                )}
              >
                {style === "candle" ? "캔들" : style === "bar" ? "바" : "영역"}
              </button>
            ))}
          </div>
        </div>

        <div className="min-h-0 flex-1 p-2">
          {candles.length > 0 ? (
            <CandlestickChart
              ref={chartRef}
              candles={candles}
              currency={currency}
              interval={interval}
              chartStyle={chartStyle}
              height={560}
              hasMore={hasMore}
              loadingMore={loadingMore}
              onLoadMore={onLoadMore}
            />
          ) : (
            <div className="flex h-full items-center justify-center text-sm text-[var(--text-muted)]">
              차트 데이터 없음
            </div>
          )}
        </div>

        <p className="shrink-0 border-t border-[var(--border)] px-4 py-2 text-[10px] text-[var(--text-muted)]">
          마우스 드래그로 이동 · 툴바/핀치로 확대/축소 · 왼쪽 끝까지 스크롤하면 과거 데이터를 불러옵니다.
        </p>
      </div>
    </div>
  );
}
