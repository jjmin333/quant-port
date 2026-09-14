"use client";

import Link from "next/link";
import { useQuickLook } from "@/components/providers/quick-look-provider";
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
 *
 * **아직 안 온 것과 못 찾은 것은 다르게 그린다.** 예전에는 둘 다 종목코드였고,
 * 그래서 목록을 열 때마다 「000660」이 잠깐 떴다가 「SK하이닉스」로 바뀌었다.
 * 오는 중이면 같은 크기의 빈 자리를 두고(줄이 흔들리지 않게), 정말 못 찾았을
 * 때만 코드를 적는다 — 그때는 코드라도 있는 편이 아무것도 없는 것보다 낫다.
 */
export function NewsSymbolChips({
  symbols,
  quotes,
  quotesLoading = false,
  max = 3,
  className,
  chipClassName,
}: {
  symbols: string[] | undefined;
  quotes: Map<string, QuoteSummary>;
  /** 시세 배치가 아직 돌아오지 않았다 — 이름 자리에 코드를 적지 않는다. */
  quotesLoading?: boolean;
  max?: number;
  className?: string;
  chipClassName?: string;
}) {
  /*
   * **칩은 창을 연다** (2026-09-05 사용자 지적 6).
   *
   * `href`는 그대로 둔다 — 가운데 클릭·「새 탭에서 열기」·크롤러가 쓰는 진짜 주소이고,
   * 껍데기 밖(제공자가 없는 자리)에서는 예전처럼 이동이 답이다. 평범한 왼쪽 클릭만
   * 가로채 창으로 바꾼다. 그래서 `preventDefault`는 **제공자가 있을 때만** 부른다.
   */
  const quickLook = useQuickLook();

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
            href={analysisSymbolHref(symbol)}
            data-meta-chip=""
            // 행 전체가 기사 모달을 여는 버튼이라, 칩 클릭이 위로 새어 나가면
            // 종목 창과 기사 모달이 같이 열린다.
            onClick={(e) => {
              e.stopPropagation();
              if (!quickLook) return;
              // 새 탭·새 창을 노린 클릭은 건드리지 않는다 — 그건 링크의 일이다.
              if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
              e.preventDefault();
              quickLook.openSymbol(symbol, quote ?? null);
            }}
            className={cn(
              "inline-flex max-w-full items-center gap-1 rounded-full border border-[var(--border)] bg-[var(--surface)] px-1.5 py-px text-micro hover:border-[var(--brand)]",
              chipClassName,
            )}
          >
            {quote?.name ? (
              <span className="truncate text-[var(--text-secondary)]">{quote.name}</span>
            ) : quotesLoading ? (
              <span
                aria-hidden
                className="inline-block h-[0.85em] w-10 animate-pulse rounded-sm bg-[var(--surface-elevated)]"
              />
            ) : (
              <span className="truncate text-[var(--text-secondary)]">{symbol}</span>
            )}
            {hasPrice && (
              <span
                className={cn(
                  "shrink-0 font-num font-bold tabular-nums",
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
