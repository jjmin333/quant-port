import { formatAmount, formatCompactAmount, formatUSD } from "@/lib/format";
import { cn } from "@/lib/utils";

export type PriceDisplaySize = "sm" | "md" | "lg" | "xl" | "hero";

const sizeClass: Record<PriceDisplaySize, string> = {
  sm: "text-sm font-medium",
  md: "text-base font-semibold",
  lg: "text-lg font-semibold",
  xl: "text-base font-semibold xl:text-xl",
  hero: "text-3xl font-bold tracking-tight lg:text-4xl xl:text-5xl",
};

export type PriceDisplayProps = {
  value: number;
  currency: "KRW" | "USD";
  size?: PriceDisplaySize;
  className?: string;
  signed?: boolean;
  compact?: boolean;
  /** USD 소수 자릿수 (기본 2) */
  decimals?: number;
};

/**
 * 화면용 가격 표시.
 * - USD: `$` + 숫자
 * - KRW: 숫자 + `원`
 */
export function PriceDisplay({
  value,
  currency,
  size = "md",
  className,
  signed = false,
  compact = false,
  decimals = 2,
}: PriceDisplayProps) {
  if (value <= 0) {
    return (
      <span
        className={cn(
          "font-mono text-[var(--text-muted)]",
          sizeClass[size],
          className,
        )}
      >
        —
      </span>
    );
  }

  if (currency === "KRW") {
    return (
      <PriceAmountPlain
        value={value}
        size={size}
        className={cn("text-[var(--text-primary)]", className)}
        signed={signed}
        compact={compact}
        suffix="원"
      />
    );
  }

  const prefix = signed && value > 0 ? "+" : signed && value < 0 ? "-" : "";
  const abs = Math.abs(value);

  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-baseline whitespace-nowrap font-mono tabular-nums text-[var(--text-primary)]",
        sizeClass[size],
        className,
      )}
    >
      {prefix}${formatUSD(abs, decimals)}
    </span>
  );
}

/** 금액 + 접미사 (예: +3,280,000원) */
export function PriceAmountPlain({
  value,
  size = "md",
  className,
  suffix = "원",
  signed = false,
  compact = false,
}: {
  value: number;
  size?: PriceDisplaySize;
  className?: string;
  suffix?: string;
  signed?: boolean;
  compact?: boolean;
}) {
  const prefix = signed && value > 0 ? "+" : signed && value < 0 ? "-" : "";
  const displayAmount = compact
    ? `${formatCompactAmount(Math.abs(value))}${suffix}`
    : `${formatAmount(Math.abs(value))}${suffix}`;

  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-baseline whitespace-nowrap font-mono tabular-nums",
        sizeClass[size],
        className,
      )}
    >
      {prefix}
      {displayAmount}
    </span>
  );
}
