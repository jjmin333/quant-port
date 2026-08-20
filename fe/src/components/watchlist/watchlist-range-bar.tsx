"use client";

import { formatCompactAmount, formatPrice, formatUSD } from "@/lib/format";
import type { QuoteCurrency } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

/**
 * 52주 레인지 안에서 현재가가 어디쯤인지.
 *
 * "52주 저가 (+41.2%) / 52주 고가 (-8.6%)" 두 칸이던 것을 막대로 바꿨다. 두 숫자를
 * 머리에서 다시 합쳐야 "지금 위쪽인가 아래쪽인가"를 알 수 있었고, 종목을 열둘 훑으면
 * 그 계산을 열두 번 한다. 위치는 눈으로 보는 값이라 막대가 맞다.
 *
 * 08-14 사용자 결정: **저·고가 값과 「상단 %」를 위아래에 둔 세 줄 형태로 되돌린다.**
 * 한 줄로 눌러 막대와 %만 남겼더니 정보는 맞아도 보기 싫었고, 행 높이는 더 써도 된다는
 * 판단이다. 트랙 그라데이션(저가 쪽 파랑 → 고가 쪽 빨강)도 함께 되돌린다.
 */

export function WatchlistRangeBar({
  pct,
  low,
  high,
  currency,
  className,
  size = "md",
}: {
  /** 0~100. 없으면 트랙만 회색으로 둔다. */
  pct: number | null;
  low?: number;
  high?: number;
  currency: QuoteCurrency;
  className?: string;
  /** `sm` — 목록 행(좁고 낮다). `md` — 모달. */
  size?: "sm" | "md";
}) {
  const sm = size === "sm";

  return (
    <div className={cn("flex min-w-0 flex-col justify-center", className)}>
      {/* 저·고가. 좁으면 접고 막대와 %만 남긴다 — 30px짜리 숫자는 읽히지 않는다. */}
      <div
        className={cn(
          "items-baseline justify-between gap-1",
          sm ? "hidden @2xl:flex" : "flex",
        )}
      >
        <span className="truncate text-[10px] leading-none tabular-nums text-[var(--text-muted)]">
          {priceText(low, currency, sm)}
        </span>
        <span className="shrink-0 text-[9px] leading-none text-[var(--text-muted)]">52주</span>
        <span className="truncate text-[10px] leading-none tabular-nums text-[var(--text-muted)]">
          {priceText(high, currency, sm)}
        </span>
      </div>

      <div className={sm ? "my-1" : "my-1.5"}>
        <span
          className={cn(
            "relative block rounded-full bg-[var(--surface-elevated)]",
            sm ? "h-1.5" : "h-2",
          )}
          style={
            pct == null
              ? undefined
              : {
                  backgroundImage:
                    "linear-gradient(90deg, color-mix(in oklab, var(--loss) 26%, transparent), var(--surface-elevated) 48%, color-mix(in oklab, var(--profit) 26%, transparent))",
                }
          }
        >
          {pct != null && (
            <>
              <span
                className="absolute inset-y-0 left-0 rounded-full bg-[var(--brand)]/35"
                style={{ width: `${pct}%` }}
              />
              <span
                className={cn(
                  "absolute top-1/2 w-[2px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-[var(--text-primary)]",
                  sm ? "h-3" : "h-4",
                )}
                style={{ left: `${pct}%` }}
              />
            </>
          )}
        </span>
      </div>

      <p
        className={cn(
          "truncate text-center leading-none text-[var(--text-muted)]",
          sm ? "text-[10px]" : "text-[11px]",
        )}
      >
        {pct == null ? (
          "—"
        ) : (
          <>
            상단{" "}
            <b className="font-semibold tabular-nums text-[var(--text-secondary)]">
              {Math.round(pct)}%
            </b>
          </>
        )}
      </p>
    </div>
  );
}

/**
 * 좁은 칸에서는 축약(9.3만), 넓은 칸에서는 전체 금액.
 *
 * 목록 행의 저·고가 칸은 60px 남짓이라 `1,988,000원`이 들어가지 않는다. 잘린 숫자보다는
 * 축약이 낫다 — 정확한 값은 퀵룩 모달이 전체 금액으로 보여준다.
 */
function priceText(value: number | undefined, currency: QuoteCurrency, compact: boolean): string {
  if (value == null || value <= 0 || Number.isNaN(value)) return "—";
  if (!compact) return formatPrice(value, currency);
  if (currency === "USD") return `$${formatUSD(value)}`;
  return formatCompactAmount(value);
}
