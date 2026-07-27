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

/** Geist Mono ₩ 글리프만 비정상 → sans 기호 + mono 숫자 (USD와 동일 컨테이너) */
function KrwSymbol() {
  return (
    <span
      className="mr-px inline-block align-baseline font-normal leading-none"
      style={{
        fontFamily: "var(--font-geist-sans), system-ui, sans-serif",
        fontSize: "0.88em",
      }}
      aria-hidden
    >
      ₩
    </span>
  );
}

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
 * - USD: Geist Mono, `$` + 숫자
 * - KRW: 동일 mono 컨테이너 + 숫자, ₩만 sans 미세 보정 (A안)
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

  const prefix = signed && value > 0 ? "+" : signed && value < 0 ? "-" : "";
  const abs = Math.abs(value);

  if (currency === "USD") {
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

  const amount = compact ? formatCompactAmount(abs) : formatAmount(abs);

  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-baseline whitespace-nowrap font-mono tabular-nums text-[var(--text-primary)]",
        sizeClass[size],
        className,
      )}
    >
      {prefix}
      <KrwSymbol />
      {amount}
    </span>
  );
}

/** 원화 기호 없이 금액 + 접미사 (예: +3,280,000원) */
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
    ? formatCompactAmount(Math.abs(value))
    : formatAmount(Math.abs(value));

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
      {suffix}
    </span>
  );
}
