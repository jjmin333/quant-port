"use client";

import { ChevronRight, Loader2, Sparkles } from "lucide-react";
import { stockSummaryHeadline } from "@/hooks/use-stock-summary";
import type { StockSummaryData } from "@/lib/stock-summary-types";
import { cn } from "@/lib/utils";

type StockAISummaryLineProps = {
  data: StockSummaryData | null;
  loading: boolean;
  /** Opens the full analysis (AI 분석 tab). */
  onOpenAnalysis?: () => void;
  compact?: boolean;
  className?: string;
};

/**
 * AI 핵심 요약 card shown atop the 개요 (Overview) tab: title row, up to 4
 * clamped lines of the headline sentence, keyword tags, then a full-width
 * "전체 분석 보기" action tying into the AI 분석 tab.
 */
export function StockAISummaryLine({
  data,
  loading,
  onOpenAnalysis,
  compact = false,
  className,
}: StockAISummaryLineProps) {
  const headline = stockSummaryHeadline(data);
  const keywords = data?.keywords ?? [];

  // Nothing to say and nothing pending — render nothing rather than an empty
  // card that would just add noise to the tab.
  if (!loading && !headline) return null;

  return (
    <div
      className={cn(
        "overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)]/60",
        className,
      )}
    >
      <div className={cn("flex items-center gap-1.5", compact ? "px-2.5 pt-2 pb-1" : "px-3 pt-2.5 pb-1")}>
        <Sparkles className="h-3.5 w-3.5 shrink-0 text-[var(--brand)]" aria-hidden />
        <span className={cn("font-semibold text-[var(--text-primary)]", compact ? "text-[11px]" : "text-[12px]")}>
          AI 핵심 요약
        </span>
        {data?.source === "rule" && (
          <span className="ml-auto shrink-0 rounded px-1 py-0.5 text-[9px] text-[var(--text-muted)]">
            규칙 기반
          </span>
        )}
      </div>

      <div className={cn(compact ? "px-2.5 pb-2" : "px-3 pb-2.5")}>
        {loading ? (
          <span className="flex items-center gap-1.5 text-[11px] text-[var(--text-muted)]">
            <Loader2 className="h-3 w-3 animate-spin" />
            AI 요약 분석 중…
          </span>
        ) : (
          <>
            <p
              className={cn(
                "line-clamp-4 whitespace-normal leading-relaxed text-[var(--text-secondary)]",
                compact ? "text-[11px]" : "text-[12px]",
              )}
              title={headline}
            >
              {headline}
            </p>
            {keywords.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-1.5">
                {keywords.map((keyword) => (
                  <span
                    key={keyword}
                    className="rounded-full border border-[var(--border)] px-2 py-0.5 text-[10px] text-[var(--text-secondary)]"
                  >
                    {keyword}
                  </span>
                ))}
              </div>
            )}
          </>
        )}
      </div>

      {!loading && onOpenAnalysis && (
        <button
          type="button"
          onClick={onOpenAnalysis}
          className="flex w-full items-center justify-between border-t border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-[11px] font-medium text-[var(--brand)] hover:bg-[var(--surface-elevated)]"
        >
          전체 분석 보기
          <ChevronRight className="h-3.5 w-3.5" aria-hidden />
        </button>
      )}
    </div>
  );
}
