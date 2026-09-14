"use client";

import { PriceDisplay } from "@/components/ui/price-display";
import { ValueFlash } from "@/components/ui/value-flash";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/*
카드 한 줄의 **가격 · 등락률 칸**. 종목이 들어간 카드는 전부 이것을 쓴다.

# 왜 부품인가 (2026-09-04 사용자 요청 8)

「거래대금 상위처럼」이 사용자가 요청 6·7에서 두 번 말한 기준이다. 그 카드의 규칙이
코드에 적혀 있지 않고 한 파일의 JSX에만 있었기 때문에, 다른 카드로 옮기려면 사람이
눈으로 베껴야 했다 — 그러면 폭·색·자릿수가 미묘하게 갈리고, 그 차이는 리뷰에서
안 보인다.

규칙은 셋이다:

	가격은 등락률 **왼쪽**에      (사용자 요청 6)
	등락률은 오른쪽 고정폭        (여러 줄의 % 기호가 세로로 선다)
	0.00%는 색이 없다             (「보합」으로 읽히는데 뜻은 「모른다」이다)

# 좁으면 가격을 생략한다 (사용자 요청 6)

「화면이 너무 좁아져서 가격을 표기할 수 없는 경우에만 생략」. CSS로 한다 —
`@container`의 폭 질의라 카드가 실제로 좁아진 그 순간에 사라진다. JS로 재면
리사이즈마다 리렌더가 돌고, 첫 프레임에는 폭을 모른다.

`hidden @[15rem]:inline`은 「컨테이너가 15rem보다 넓을 때만 보인다」는 뜻이다.
15rem(240px)은 종목명이 최소 폭을 유지한 채 가격이 들어갈 수 있는 경계다. 카드
쪽에 `@container`가 없으면 이 질의는 **참이 되지 않아 가격이 안 보이므로**,
쓰는 카드의 목록에 `@container`를 함께 단다.
*/

export type QuoteCellProps = {
  price: number;
  changeRate: number;
  currency: "KRW" | "USD";
  /** 플래시 기준을 옮길 열쇠. 보통 심볼 — 없으면 색이 뜨지 않는다. */
  symbol?: string;
  className?: string;
};

export function QuoteCell({ price, changeRate, currency, symbol, className }: QuoteCellProps) {
  const has = price > 0;
  return (
    <span className={cn("flex shrink-0 items-baseline gap-1.5", className)}>
      <ValueFlash
        value={has ? price : null}
        resetKey={symbol}
        className="hidden font-num text-cell tabular-nums text-[var(--text-primary)] @[15rem]:inline"
      >
        {has ? (
          <PriceDisplay value={price} currency={currency} size="inherit" />
        ) : (
          <span className="text-muted-foreground">—</span>
        )}
      </ValueFlash>
      <span
        className={cn(
          "w-14 shrink-0 text-right font-num text-cell tabular-nums",
          has && changeRate !== 0 ? trendTextClass(changeRate) : "text-muted-foreground",
        )}
      >
        {has ? formatPercent(changeRate) : "—"}
      </span>
    </span>
  );
}
