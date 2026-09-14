"use client";

import { ArrowDown, Sparkles } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import type { WatchlistMarketFilter } from "@/lib/watchlist-calc";
import { cn } from "@/lib/utils";

/**
 * 시안 공용 정렬·필터.
 *
 * 지금 툴바와 두 가지가 다르다 —
 *
 *  1. **「주목도」 정렬이 기본**이다. 신호가 무거운 종목이 위로 온다. 「내 순서」는 그대로
 *     남는다(드래그로 정한 순서는 사용자의 것이라 지우지 않는다).
 *  2. **상승/하락 필터 대신 「신호」 필터**다. 오늘 오른 종목을 고르는 일보다, 말할 거리가
 *     있는 종목만 남기는 쪽이 이 화면에서 훨씬 자주 하는 일이다.
 */

export type WlSort = "attention" | "manual" | "change" | "since" | "score";

const SORT_LABELS: Record<WlSort, string> = {
  attention: "주목도",
  manual: "내 순서",
  change: "등락률",
  since: "담은 뒤",
  score: "점수",
};

export function WlToolbar({
  sort,
  onSortChange,
  market,
  onMarketChange,
  onlySignal,
  onOnlySignalChange,
  counts,
}: {
  sort: WlSort;
  onSortChange: (next: WlSort) => void;
  market: WatchlistMarketFilter;
  onMarketChange: (next: WatchlistMarketFilter) => void;
  onlySignal: boolean;
  onOnlySignalChange: (next: boolean) => void;
  counts: { all: number; domestic: number; us: number; signal: number };
}) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <div className="flex flex-wrap items-center gap-1">
        {(Object.keys(SORT_LABELS) as WlSort[]).map((key) => (
          <Chip
            key={key}
            active={sort === key}
            onClick={() => onSortChange(key)}
            label={SORT_LABELS[key]}
            icon={
              key === "attention" && sort === key ? (
                <Sparkles className="size-2.5" />
              ) : key !== "manual" && sort === key ? (
                <ArrowDown className="size-2.5" />
              ) : null
            }
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
        <Chip
          active={market === "us"}
          onClick={() => onMarketChange("us")}
          label={`미국 ${counts.us}`}
        />
      </div>

      <span className="hidden h-4 w-px bg-[var(--border)] sm:block" aria-hidden />

      <Chip
        active={onlySignal}
        onClick={() => onOnlySignalChange(!onlySignal)}
        label={`신호 ${counts.signal}`}
        icon={<Sparkles className="size-2.5" />}
      />
    </div>
  );
}

function Chip({
  active,
  onClick,
  label,
  icon,
}: {
  active: boolean;
  onClick: () => void;
  label: string;
  icon?: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        buttonVariants({ variant: "outline", size: "xs" }),
        "rounded-full",
        active
          ? "border-[var(--brand)]/35 bg-[var(--brand-muted)] font-semibold text-[var(--brand)]"
          : "border-[var(--border)] bg-card text-[var(--text-muted)] hover:text-[var(--text-primary)]",
      )}
    >
      {icon}
      {label}
    </button>
  );
}
