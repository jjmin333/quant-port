import { formatAmount, formatCompactAmount, formatUSD } from "@/lib/format";
import { cn } from "@/lib/utils";

export type PriceDisplaySize = "inherit" | "sm" | "md" | "lg" | "xl" | "hero";

const sizeClass: Record<PriceDisplaySize, string> = {
  /*
   * 크기를 정하지 않는다 — 표 셀처럼 **줄이 이미 크기를 갖고 있는 자리**용이다.
   * 이게 없으면 목록 행에서 이 컴포넌트를 쓸 수 없어(가장 작은 sm이 14px) 호출부가
   * `formatPrice()` 문자열로 도망갔고, 그래서 단위 축소가 닿지 않는 자리가 생겼다.
   */
  inherit: "",
  sm: "text-sm font-medium",
  md: "text-base font-semibold",
  lg: "text-lg font-semibold",
  xl: "text-base font-semibold xl:text-xl",
  hero: "text-3xl font-bold tracking-tight lg:text-4xl xl:text-5xl",
};

/**
 * 통화 단위(원 · 달러 기호)는 값보다 한 단 작고 흐리다.
 *
 * 값과 단위가 같은 크기면 여덟 줄짜리 목록에서 「원」이 여덟 번 값과 같은 무게로
 * 반복된다. 단위는 어느 줄에서나 같으므로 정보가 아니다 — 읽히는 것은 숫자다.
 * 크기만 줄이고 색은 `--text-muted`로 낮춘다(0.85배는 실측으로 고른 값이다).
 */
export const PRICE_UNIT_CLASS = "text-[0.85em] font-normal text-[var(--text-muted)]";

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
          "font-num text-[var(--text-muted)]",
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
        "inline-flex shrink-0 items-baseline whitespace-nowrap font-num tabular-nums text-[var(--text-primary)]",
        sizeClass[size],
        className,
      )}
    >
      {prefix}
      <span className={PRICE_UNIT_CLASS}>$</span>
      {formatUSD(abs, decimals)}
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
    ? formatCompactAmount(Math.abs(value))
    : formatAmount(Math.abs(value));

  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-baseline whitespace-nowrap font-num tabular-nums",
        sizeClass[size],
        className,
      )}
    >
      {prefix}
      {displayAmount}
      <span className={PRICE_UNIT_CLASS}>{suffix}</span>
    </span>
  );
}
