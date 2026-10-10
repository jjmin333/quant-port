"use client";

import { memo } from "react";
import Link from "next/link";
import { isPlainLeftClick } from "@/lib/plain-click";
import { Heart, NotepadText } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { PriceDisplay } from "@/components/ui/price-display";
import { ValueFlash } from "@/components/ui/value-flash";
import { parseItemChangeRate } from "@/components/market-radar/market-radar-sparkline";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { parseRankingNumber } from "@/lib/market-radar-config";
import type { EnrichedRankingItem } from "@/lib/market-radar-types";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

type MarketRadarRowProps = {
  item: EnrichedRankingItem;
  active: boolean;
  compact?: boolean;
  onSelect: (symbol: string) => void;
  onToggleWatchlist: (item: EnrichedRankingItem) => void;
  onOpenMemo: (item: EnrichedRankingItem) => void;
  watchlistBusy?: boolean;
};

/*
랭킹 한 줄.

# 왜 button이 아니라 div인가

행 전체가 「종목 선택」이고 그 안에 관심·메모 **버튼이 들어간다.** 버튼 안의 버튼은
무효 HTML이라 브라우저가 마크업을 재구성하고, 그러면 안쪽 버튼이 클릭을 못 받는다.
그래서 행은 `role="button"`인 div이고, 안쪽 아이콘만 진짜 `<button>`이다.

키보드로도 같아야 하므로 `tabIndex`와 Enter·Space 처리를 직접 단다 — div는 button이
공짜로 주던 것을 하나도 안 준다.

# 아이콘 클릭이 행 선택을 겸하지 않게

아이콘의 `onClick`은 `stopPropagation`한다. 안 하면 관심 등록 한 번에 종목까지
바뀌어, 누른 적 없는 차트가 열린다.
*/
function MarketRadarRowImpl({
  item,
  active,
  compact,
  onSelect,
  onToggleWatchlist,
  onOpenMemo,
  watchlistBusy,
}: MarketRadarRowProps) {
  const price = parseRankingNumber(item.price.lastPrice);
  const changeRate = parseItemChangeRate(
    item.price.changeRate,
    item.price.lastPrice,
    item.price.basePrice,
  );

  const priceNode =
    price > 0 ? <PriceDisplay value={price} currency={item.currency} size="inherit" /> : "—";

  /*
    # 행 껍데기는 버튼이 아니다 (2026-09-06)

    예전에는 이 `div`가 `role="button"` + `tabIndex`였고 **그 안에 관심·메모 버튼이
    들어 있었다.** 버튼 안의 버튼은 ARIA가 허용하지 않는다 — 크로미움은 너그럽게
    처리해 줘서 탭 순서는 멀쩡했지만(실측: 행 → 관심 → 메모), 버튼을 잎 노드로
    다루는 스크린리더의 탐색 모드에서는 안쪽 버튼에 닿지 못할 수 있다.

    그래서 **주 동작만 진짜 `<button>`으로 내리고** 아이콘 버튼을 그 형제로 뒀다.
    보이는 모습과 탭 순서는 그대로다 — 바뀌는 것은 보조기술이 이 줄을 「버튼 안에
    버튼이 있는 것」이 아니라 **버튼 셋이 나란한 줄**로 읽는다는 점이다.

    바깥 `div`가 여백·테두리·호버를 계속 들고 있어야 행 전체가 한 덩어리로 보인다.
  */
  return (
    <div
      className={cn(
        "flex w-full items-center border-b border-[var(--border)]/70 transition-colors",
        compact ? "gap-2.5 px-3 py-2" : "gap-2 px-2 py-1.5",
        active ? "bg-[var(--brand-muted)]" : "hover:bg-[var(--surface-elevated)]",
      )}
    >
      {/*
        주 동작은 종목 페이지로 가는 **진짜 링크**다(2026-10-09 — 검색 색인 개선 B). 크롤러는
        이 href로 종목 페이지를 찾고, Ctrl/가운데 클릭은 새 탭으로 연다. 일반 클릭은 지금처럼
        옆 패널을 연다.
      */}
      <Link
        href={`/stocks/${encodeURIComponent(item.symbol)}`}
        prefetch={false}
        onClick={(e) => {
          if (!isPlainLeftClick(e)) return;
          e.preventDefault();
          onSelect(item.symbol);
        }}
        aria-current={active ? "true" : undefined}
        aria-label={`${item.name} ${item.rank}위`}
        className={cn(
          "flex min-w-0 flex-1 cursor-pointer items-center bg-transparent p-0 text-left",
          "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-[var(--brand)]",
          compact ? "gap-2.5" : "gap-2",
        )}
      >
        <div
          className={cn(
            "flex shrink-0 items-baseline justify-end gap-0.5 tabular-nums",
            compact ? "w-8" : "w-7",
          )}
        >
        <span
          className={cn(
            "font-num font-semibold text-[var(--text-primary)]",
            compact ? "text-note" : "text-cell",
          )}
        >
          {item.rank}
        </span>
        {item.rankDelta != null && item.rankDelta !== 0 && (
          <span
            className={cn(
              "text-micro font-medium leading-none",
              item.rankDelta > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
            )}
          >
            {item.rankDelta > 0 ? `↑${item.rankDelta}` : `↓${Math.abs(item.rankDelta)}`}
          </span>
        )}
      </div>

      <StockAvatar
        symbol={item.symbol}
        name={item.name}
        currency={item.currency}
        assetType={item.assetType}
        logoUrl={item.logoUrl}
        size="xs"
        className={cn("shrink-0", compact ? "h-7 w-7 text-micro" : "h-6 w-6 text-micro")}
      />

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1">
          <span
            className={cn(
              "truncate font-medium leading-tight text-[var(--text-primary)]",
              compact ? "text-note" : "text-cell",
            )}
          >
            {item.name}
          </span>
          {item.isHeld && (
            <Badge variant="info" className="shrink-0 px-1 py-0 text-micro leading-none">
              보유
            </Badge>
          )}
        </div>
        {/*
          이름 아래는 **가격과 등락률**이다. 예전에는 여기에 종목코드와 거래대금이
          있었는데, 그 둘은 목록을 훑는 사람이 찾는 값이 아니다.
        */}
        <div
          className={cn(
            "mt-0.5 flex min-w-0 items-baseline gap-1.5 font-num tabular-nums leading-tight",
            compact ? "text-cell" : "text-micro",
          )}
        >
          {/*
            플래시는 공용 부품이 정한다(`ui/value-flash`). 예전에는 이 행이 자기
            타이머와 `priceTick` 카운터로 직접 세었는데, 그 방식은 **행 객체가
            바뀌었는가**를 보는 것이라 등락률 문자열만 달라져도 색이 떴다
            (2026-09-04 사용자 지적 3). 지금은 **그리는 숫자 자체**를 넘긴다 —
            화면에 보이는 값이 안 바뀌면 색도 없다.

            `resetKey`로 심볼을 넘기는 이유: 이 행은 순위가 바뀌면 다른 종목을
            그린다. 안 넘기면 앞 종목 가격과 비교해 엉뚱한 색이 뜬다.
          */}
          <ValueFlash
            value={price > 0 ? price : null}
            resetKey={item.symbol}
            className="truncate text-[var(--text-primary)]"
          >
            {priceNode}
          </ValueFlash>
          <span className={cn("shrink-0", trendTextClass(changeRate))}>
            {price > 0 ? formatPercent(changeRate) : "—"}
          </span>
        </div>
      </div>

      </Link>

      {/* 주 동작의 **형제**다 — 버튼 안에 들어가면 다시 중첩이 된다. */}
      <div className="flex shrink-0 items-center gap-0.5">
        <RowIconButton
          compact={compact}
          active={item.isWatchlisted}
          busy={watchlistBusy}
          label={item.isWatchlisted ? `${item.name} 관심 해제` : `${item.name} 관심 등록`}
          onClick={() => onToggleWatchlist(item)}
        >
          <Heart
            className={cn("size-full", item.isWatchlisted && "fill-current")}
            aria-hidden="true"
          />
        </RowIconButton>
        <RowIconButton
          compact={compact}
          label={`${item.name} 메모 작성`}
          onClick={() => onOpenMemo(item)}
        >
          <NotepadText className="size-full" aria-hidden="true" />
        </RowIconButton>
      </div>
    </div>
  );
}

export function RowIconButton({
  children,
  label,
  onClick,
  compact,
  active,
  busy,
}: {
  children: React.ReactNode;
  label: string;
  onClick: () => void;
  compact?: boolean;
  active?: boolean;
  busy?: boolean;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      aria-pressed={active}
      aria-busy={busy}
      onClick={(e) => {
        // 행 전체가 종목 선택이다 — 여기서 막지 않으면 관심 등록 한 번에
        // 누른 적 없는 종목의 차트가 열린다.
        e.stopPropagation();
        onClick();
      }}
      onKeyDown={(e) => e.stopPropagation()}
      className={cn(
        "grid place-items-center rounded-lg transition-colors",
        "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-[var(--brand)]",
        compact ? "size-8 p-[7px]" : "size-[26px] p-[5.5px]",
        busy && "pointer-events-none opacity-60",
        active
          ? "text-rose-500 hover:text-rose-600"
          : "text-[var(--text-muted)] hover:bg-[color-mix(in_oklab,var(--foreground)_8%,transparent)] hover:text-[var(--text-primary)]",
      )}
    >
      {children}
    </button>
  );
}

/*
`memo`가 여기서 값을 한다. 5초마다 가격이 들어오는데 실제로 바뀌는 행은 몇 개뿐이고,
`applyLivePrices`가 안 바뀐 행에는 **같은 객체**를 돌려준다. 그래서 이 비교가
100행 중 대부분을 리렌더에서 걷어낸다.
*/
export const MarketRadarRow = memo(MarketRadarRowImpl);
