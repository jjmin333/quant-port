"use client";

import { PRICE_UNIT_CLASS, PriceDisplay } from "@/components/ui/price-display";
import { FLASH_BG, useValueFlash } from "@/components/ui/value-flash";
import { useExchangeRate } from "@/hooks/use-exchange-rate";
import { formatPercent, formatPriceChange, formatUSD } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

type QuotePriceProps = {
  value: number;
  currency: "KRW" | "USD";
  exchangeRate?: number;
  size?: "sm" | "md" | "lg" | "xl" | "hero";
  className?: string;
  signed?: boolean;
  /** USD 표시 시 원화 환산 병기 (기본 true). 좁은 목록 등에서는 false */
  showKrwEquivalent?: boolean;
  /*
    값이 움직일 때 플래시를 줄지 — **넘기면 준다** (2026-09-04 사용자 요청 4).

    보통 심볼을 넘긴다. 두 가지 일을 한 번에 한다: ① 이 자리가 살아 있는 시세라고
    선언하고 ② 종목이 바뀌는 순간 색 없이 기준을 옮긴다. ②가 없으면 상세 화면에서
    종목을 갈아탈 때 **앞 종목 가격과 비교해** 색이 뜬다 — 7만원에서 25만원으로
    넘어가면 상승 플래시가 뜨는데, 오른 것은 아무것도 없다.

    안 넘기면 색이 없다. 스탬프처럼 **박제된 값**을 그리는 자리가 그렇다 — 거기서
    값이 바뀌는 것은 「시장이 움직였다」가 아니라 「다른 스탬프를 열었다」이다.
  */
  flashKey?: string;
};

export function QuotePrice({
  value,
  currency,
  exchangeRate: exchangeRateProp,
  size = "md",
  className,
  signed = false,
  showKrwEquivalent = true,
  flashKey,
}: QuotePriceProps) {
  /*
    훅은 **조건 아래로 내려가면 안 된다.** 이 함수는 아래에서 `value <= 0`과
    통화별로 일찍 return하므로, 그 뒤에서 부르면 렌더마다 훅 개수가 달라진다.
    `flashKey`가 없을 때 색을 안 주는 것은 훅을 건너뛰어서가 아니라 결과를
    안 쓰기 때문이다.
  */
  const flashDirection = useValueFlash(flashKey ? value : null, flashKey);
  const flashClass = flashKey && flashDirection ? FLASH_BG[flashDirection] : undefined;
  const fetchedRate = useExchangeRate(
    currency === "USD" && showKrwEquivalent && !(exchangeRateProp && exchangeRateProp > 0),
  );
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
        className={cn(flashClass && "rounded px-0.5 transition-colors", flashClass, className)}
        signed={signed}
      />
    );
  }

  const prefix = signed && value > 0 ? "+" : signed && value < 0 ? "-" : "";
  const abs = Math.abs(value);

  const krwPart =
    showKrwEquivalent && exchangeRate > 0 ? (
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
        "inline-flex shrink-0 flex-wrap items-baseline gap-x-1 gap-y-0.5 whitespace-nowrap font-num tabular-nums text-[var(--text-primary)]",
        size === "sm" && "text-sm font-medium",
        size === "md" && "text-base font-semibold",
        size === "lg" && "text-lg font-semibold",
        size === "xl" && "text-base font-semibold xl:text-xl",
        size === "hero" && "text-3xl font-bold tracking-tight lg:text-4xl xl:text-5xl",
        flashClass && "rounded px-0.5 transition-colors",
        flashClass,
        className,
      )}
    >
      {prefix}
      <span className={PRICE_UNIT_CLASS}>$</span>
      {formatUSD(abs)}
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

  const sizeClass =
    size === "lg" ? "text-lg" : size === "sm" ? "text-cell" : "text-sm";

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
      : formatPriceChange(change, "KRW");

  return (
    <span
      className={cn(
        "inline-flex shrink-0 flex-wrap items-center gap-x-2 gap-y-0.5 whitespace-nowrap font-num tabular-nums",
        sizeClass,
        trendTextClass(changeRate),
      )}
    >
      <span>{changeText}</span>
      <span>({formatPercent(changeRate)})</span>
    </span>
  );
}
