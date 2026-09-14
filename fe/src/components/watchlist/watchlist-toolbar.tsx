"use client";

import { ArrowDown } from "lucide-react";
import {
  WATCHLIST_SORT_LABELS,
  type WatchlistMarketFilter,
  type WatchlistSort,
  type WatchlistTrendFilter,
} from "@/lib/watchlist-calc";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

/**
 * 관심종목 정렬·필터.
 *
 * 지금까지 순서를 바꾸는 방법은 드래그뿐이었다. "오늘 제일 많이 빠진 게 뭐지"를 알려면
 * 눈으로 열두 줄을 훑어야 했고, 그 답을 얻자고 내가 정해둔 순서를 부술 수는 없다.
 *
 * 그래서 **정렬은 보기 방식일 뿐 저장하지 않는다**. 「내 순서」가 유일하게 드래그가
 * 열리는 모드이고, 다른 정렬에서는 핸들을 숨긴다 — 정렬된 화면에서 드래그를 받으면
 * 화면에 보이는 순서가 그대로 저장돼 내 순서가 조용히 덮어써진다.
 */
export function WatchlistToolbar({
  sort,
  onSortChange,
  market,
  onMarketChange,
  trend,
  onTrendChange,
  counts,
}: {
  sort: WatchlistSort;
  onSortChange: (next: WatchlistSort) => void;
  market: WatchlistMarketFilter;
  onMarketChange: (next: WatchlistMarketFilter) => void;
  trend: WatchlistTrendFilter;
  onTrendChange: (next: WatchlistTrendFilter) => void;
  counts: { all: number; domestic: number; us: number; up: number; down: number };
}) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <div className="flex flex-wrap items-center gap-1">
        {(Object.keys(WATCHLIST_SORT_LABELS) as WatchlistSort[]).map((key) => (
          <Chip
            key={key}
            active={sort === key}
            onClick={() => onSortChange(key)}
            label={WATCHLIST_SORT_LABELS[key]}
            // 내림차순 정렬임을 표시한다. 「내 순서」는 방향이라는 게 없다.
            icon={key !== "manual" && sort === key ? <ArrowDown className="size-2.5" /> : null}
          />
        ))}
      </div>

      <span className="hidden h-4 w-px bg-[var(--border)] sm:block" aria-hidden />

      <div className="flex flex-wrap items-center gap-1">
        <Chip
          active={market === "all"}
          onClick={() => onMarketChange("all")}
          label={`전체 ${counts.all}`}
        />
        <Chip
          active={market === "domestic"}
          onClick={() => onMarketChange("domestic")}
          label={`국내 ${counts.domestic}`}
        />
        <Chip active={market === "us"} onClick={() => onMarketChange("us")} label={`미국 ${counts.us}`} />
      </div>

      <span className="hidden h-4 w-px bg-[var(--border)] sm:block" aria-hidden />

      <div className="flex flex-wrap items-center gap-1">
        <Chip
          active={trend === "up"}
          onClick={() => onTrendChange(trend === "up" ? "all" : "up")}
          label={`상승 ${counts.up}`}
          tone="up"
        />
        <Chip
          active={trend === "down"}
          onClick={() => onTrendChange(trend === "down" ? "all" : "down")}
          label={`하락 ${counts.down}`}
          tone="down"
        />
      </div>
    </div>
  );
}

function Chip({
  active,
  onClick,
  label,
  icon,
  tone,
}: {
  active: boolean;
  onClick: () => void;
  label: string;
  icon?: React.ReactNode;
  tone?: "up" | "down";
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(buttonVariants({ variant: "outline", size: "xs" }), "rounded-full",
        active
          ? "border-[var(--brand)]/35 bg-[var(--brand-muted)] font-semibold text-[var(--brand)]"
          : "border-[var(--border)] bg-card text-[var(--text-muted)] hover:text-[var(--text-primary)]",
        // 상승·하락 칩은 꺼져 있을 때도 방향 색을 옅게 남긴다 — 어느 쪽 버튼인지
        // 글자를 읽지 않고 알 수 있게.
        !active && tone === "up" && "text-[var(--profit)]/75 hover:text-[var(--profit)]",
        !active && tone === "down" && "text-[var(--loss)]/75 hover:text-[var(--loss)]",
      )}
    >
      {icon}
      {label}
    </button>
  );
}
