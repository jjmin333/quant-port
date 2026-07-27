"use client";

import { PriceDisplay } from "@/components/ui/price-display";
import { useExchangeRate } from "@/hooks/use-exchange-rate";
import { formatPriceChange, formatUSD } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

type QuotePriceProps = {
  value: number;
  currency: "KRW" | "USD";
  exchangeRate?: number;
  size?: "sm" | "md" | "lg" | "xl" | "hero";
  className?: string;
  signed?: boolean;
};

export function QuotePrice({
  value,
  currency,
  exchangeRate: exchangeRateProp,
  size = "md",
  className,
  signed = false,
}: QuotePriceProps) {
  const fetchedRate = useExchangeRate(currency === "USD" && !(exchangeRateProp && exchangeRateProp > 0));
  const exchangeRate = exchangeRateProp && exchangeRateProp > 0 ? exchangeRateProp : fetchedRate;

  if (value <= 0) {
    return (
      <span className={cn("font-sans text-[var(--text-muted)]", className)}>—</span>
    );
  }

  if (currency === "KRW") {
    return (
      <PriceDisplay
        value={value}
        currency="KRW"
        size={size}
        className={className}
        signed={signed}
      />
    );
  }

  const prefix = signed && value > 0 ? "+" : signed && value < 0 ? "-" : "";
  const abs = Math.abs(value);

  const krwPart =
    exchangeRate > 0 ? (
      <span className="text-[0.72em] font-normal text-[var(--text-secondary)]">
        (
        <PriceDisplay
          value={abs * exchangeRate}
          currency="KRW"
          size="sm"
          className="text-[length:inherit] font-normal text-[var(--text-secondary)]"
        />
        )
      </span>
    ) : null;

  return (
    <span
      className={cn(
        "inline-flex shrink-0 flex-wrap items-baseline gap-x-1 gap-y-0.5 whitespace-nowrap font-mono tabular-nums text-[var(--text-primary)]",
        size === "sm" && "text-sm font-medium",
        size === "md" && "text-base font-semibold",
        size === "lg" && "text-lg font-semibold",
        size === "xl" && "text-base font-semibold xl:text-xl",
        size === "hero" && "text-3xl font-bold tracking-tight lg:text-4xl xl:text-5xl",
        className,
      )}
    >
      {prefix}${formatUSD(abs)}
      {krwPart}
    </span>
  );
}

type QuoteChangeProps = {
  change: number;
  changeRate: number;
  currency: "KRW" | "USD";
  exchangeRate?: number;
  size?: "sm" | "md" | "lg";
  hasBaseline?: boolean;
};

export function QuoteChange({
  change,
  changeRate,
  currency,
  exchangeRate: exchangeRateProp,
  size = "md",
  hasBaseline = true,
}: QuoteChangeProps) {
  const fetchedRate = useExchangeRate(currency === "USD" && !(exchangeRateProp && exchangeRateProp > 0));
  const exchangeRate = exchangeRateProp && exchangeRateProp > 0 ? exchangeRateProp : fetchedRate;

  const isUp = changeRate >= 0;
  const sizeClass =
    size === "lg" ? "text-lg" : size === "sm" ? "text-xs" : "text-sm";

  if (!hasBaseline) {
    return (
      <span className={cn("inline-flex shrink-0 font-sans text-[var(--text-muted)]", sizeClass)}>
        —
      </span>
    );
  }

  const changeText =
    currency === "USD"
      ? formatPriceChange(change, "USD", exchangeRate)
      : `${isUp ? "+" : ""}${Math.abs(change).toLocaleString("ko-KR")}`;

  return (
    <span
      className={cn(
        "inline-flex shrink-0 flex-wrap items-center gap-x-2 gap-y-0.5 whitespace-nowrap font-mono tabular-nums",
        sizeClass,
        trendTextClass(changeRate),
      )}
    >
      <span>{changeText}</span>
      <span>
        ({isUp ? "+" : ""}
        {changeRate.toFixed(2)}%)
      </span>
    </span>
  );
}
