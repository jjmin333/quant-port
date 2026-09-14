"use client";

import { PriceDisplay } from "@/components/ui/price-display";
import { useShellMode } from "@/components/providers/shell-mode-provider";
import { formatPercent } from "@/lib/format";
import type { MarketIndex } from "@/lib/quotes-types";
import { useMarketIndices } from "@/hooks/use-market-indices";
import { cn } from "@/lib/utils";
import { trendTextClass } from "@/lib/trend-colors";

/**
 * 실시간 차트·호가 화면 상단의 지수 띠.
 *
 * 예전에는 같은 파일에 홈용 `MarketOverview`(그룹별 카드 그리드)가 함께 있었다. 홈이
 * **전 종목을 한 줄에 흘리는 스트립**(`IndexTickerStrip`)으로 바뀌면서 같은 지수를
 * 한 화면에서 두 번 보여주게 됐고, 아래쪽 「전체 지수」 접힘 섹션과 함께 걷어냈다.
 * 지수를 눌러 들여다보는 일은 이제 어디서든 `IndexQuickLookModal` 하나가 맡는다.
 */
function IndexValueDisplay({ index, compact = false }: { index: MarketIndex; compact?: boolean }) {
  if (index.value <= 0) {
    return <span className="text-[var(--text-muted)]">—</span>;
  }

  const sizeClass = compact
    ? "text-sm font-semibold tracking-tight"
    : "text-xl font-semibold tracking-tight";

  if (index.currency === "USD") {
    return (
      <PriceDisplay
        value={index.value}
        currency="USD"
        size="lg"
        className={sizeClass}
      />
    );
  }

  if (index.id === "btc") {
    return (
      <PriceDisplay
        value={index.value}
        currency="KRW"
        size="lg"
        className={sizeClass}
      />
    );
  }

  return (
    <span className={cn("font-sans tabular-nums text-[var(--text-primary)]", sizeClass)}>
      {index.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })}
    </span>
  );
}

export function MarketOverviewCompact() {
  const { isCompact } = useShellMode();
  const { indices } = useMarketIndices();

  return (
    <div className="flex shrink-0 overflow-x-auto border-b border-[var(--border)] bg-[var(--surface)]">
      <div className={cn("flex min-w-max items-center", isCompact ? "py-1.5" : "py-2")}>
        {indices.map((index) => {
          const hasValue = index.value > 0;
          return (
            <div
              key={index.id}
              className={cn(
                "flex shrink-0 items-center whitespace-nowrap border-r border-[var(--border)] last:border-r-0",
                isCompact ? "gap-2 px-3" : "gap-3 px-4",
              )}
            >
              <span
                className={cn(
                  "font-medium text-[var(--text-muted)]",
                  isCompact ? "text-micro" : "text-cell",
                )}
              >
                {index.name}
              </span>
              <span
                className={cn(
                  "font-semibold text-[var(--text-primary)]",
                  isCompact ? "text-cell" : "text-sm",
                )}
              >
                <IndexValueDisplay index={index} compact={isCompact} />
              </span>
              <span
                className={cn(
                  "font-sans tabular-nums",
                  isCompact ? "text-micro" : "text-cell",
                  hasValue ? trendTextClass(index.changeRate) : "text-[var(--text-muted)]",
                )}
              >
                {hasValue ? formatPercent(index.changeRate) : "—"}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
