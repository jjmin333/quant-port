"use client";

import Link from "next/link";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import type { QuoteSummary } from "@/lib/quotes-types";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * C1 — 기사에 걸린 종목과 그 등락률.
 *
 * "이 뉴스에 시장이 어떻게 반응했나"를 기사에서 바로 읽게 한다. 시세는 목록 전체가
 * **하나의 배치 요청을 공유**하므로(useSymbolQuotes) 행마다 요청이 늘지 않는다.
 *
 * 시세를 못 붙인 종목은 등락률 없이 이름만 낸다 — 0%를 적으면 보합으로 읽힌다.
 */
export function NewsSymbolChips({
  symbols,
  quotes,
  max = 3,
  className,
  chipClassName,
}: {
  symbols: string[] | undefined;
  quotes: Map<string, QuoteSummary>;
  max?: number;
  className?: string;
  chipClassName?: string;
}) {
  const shown = (symbols ?? [])
    .map((s) => s.trim().toUpperCase())
    .filter(Boolean)
    .slice(0, max);
  if (shown.length === 0) return null;

  return (
    <div className={cn("flex min-w-0 flex-wrap items-center gap-1", className)}>
      {shown.map((symbol) => {
        const quote = quotes.get(symbol);
        const hasPrice = Boolean(quote && quote.currentPrice > 0);
        return (
          <Link
            key={symbol}
            href={analysisSymbolHref(symbol, "ai")}
            data-meta-chip=""
            // 행 전체가 기사 모달을 여는 버튼이라, 칩 클릭이 위로 새어 나가면
            // 종목으로 가면서 모달도 같이 열린다.
            onClick={(e) => e.stopPropagation()}
            className={cn(
              "inline-flex max-w-full items-center gap-1 rounded-full border border-[var(--border)] bg-[var(--surface)] px-1.5 py-px text-[10px] hover:border-[var(--brand)]",
              chipClassName,
            )}
          >
            <span className="truncate text-[var(--text-secondary)]">
              {quote?.name || symbol}
            </span>
            {hasPrice && (
              <span
                className={cn(
                  "shrink-0 font-mono font-bold tabular-nums",
                  trendTextClass(quote!.changeRate),
                )}
              >
                {formatPercent(quote!.changeRate)}
              </span>
            )}
          </Link>
        );
      })}
    </div>
  );
}
