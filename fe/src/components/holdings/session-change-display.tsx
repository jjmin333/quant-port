"use client";

import { formatPercent, formatPriceChange } from "@/lib/format";
import type { HoldingDisplay } from "@/lib/holding-display";
import { extendedSessionLabel, shouldShowExtendedChange } from "@/lib/holding-display";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

export function SessionChangeCell({
  display,
  variant,
  size = "sm",
}: {
  display: HoldingDisplay;
  variant: "regular" | "extended";
  size?: "sm" | "lg";
}) {
  const textSize = size === "lg" ? "text-lg font-semibold" : "font-medium text-sm";
  const subSize = size === "lg" ? "text-xs" : "text-[10px]";

  if (display.prevClose <= 0) {
    return <span className="text-[var(--text-muted)]">—</span>;
  }

  if (variant === "extended" && !shouldShowExtendedChange(display)) {
    return <span className="text-[var(--text-muted)]">—</span>;
  }

  const rate = variant === "regular" ? display.regularChangeRate : display.extendedChangeRate;
  const change = variant === "regular" ? display.regularChange : display.extendedChange;

  return (
    <div className="min-w-[5.5rem]">
      <p className={cn("font-mono tabular-nums", textSize, trendTextClass(rate))}>
        {formatPercent(rate)}
      </p>
      {change !== 0 && (
        <p className={cn(subSize, "mt-0.5 text-[var(--text-muted)] tabular-nums")}>
          {display.currency === "USD"
            ? formatPriceChange(change, "USD", display.exchangeRate)
            : `${change >= 0 ? "+" : ""}₩${Math.round(change).toLocaleString("ko-KR")}`}
        </p>
      )}
    </div>
  );
}

/** @deprecated Use SessionChangeCell with separate columns instead. */
export function SessionChangeDisplay({
  display,
  size = "sm",
}: {
  display: HoldingDisplay;
  size?: "sm" | "lg";
}) {
  return (
    <div className="space-y-2">
      <SessionChangeCell display={display} variant="regular" size={size} />
      {shouldShowExtendedChange(display) && (
        <div>
          <p
            className={cn(
              size === "lg" ? "text-xs" : "text-[10px]",
              "text-[var(--text-muted)]",
            )}
          >
            {extendedSessionLabel(display.marketSession, display.currency)}
          </p>
          <SessionChangeCell display={display} variant="extended" size={size} />
        </div>
      )}
    </div>
  );
}
