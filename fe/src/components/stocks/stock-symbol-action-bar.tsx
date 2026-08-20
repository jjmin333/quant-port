"use client";

import Link from "next/link";
import { NotebookPen, Star, Target } from "lucide-react";
import { PublishCallButton } from "@/components/research/publish-call-button";
import { StampButton } from "@/components/stamps/stamp-button";
import { cn } from "@/lib/utils";

const linkPillClass =
  "inline-flex shrink-0 items-center gap-1 rounded-lg border border-[var(--border)] px-2 py-1 text-[11px] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]";

type StockSymbolActionBarProps = {
  symbol: string;
  watchlisted: boolean;
  watchlistBusy?: boolean;
  onToggleWatchlist: () => void;
  /** Chart interval on screen — frozen into a stamp when one is taken. */
  chartInterval?: string;
  price?: number;
  currency?: "KRW" | "USD";
  week52High?: number;
  week52Low?: number;
  stampSize?: "sm" | "md";
  className?: string;
};

/**
 * 종목 헤더 우측 액션 — 관심·스탬프·리포트·메모장·목표가.
 *
 * 실시간 차트·AI 추천 랭킹 등 같은 종목을 여러 화면에서 볼 때 버튼 구성이
 * 달라지면 "여기서는 메모가 없네?" 같은 혼란이 생긴다. 한 컴포넌트로 묶는다.
 */
export function StockSymbolActionBar({
  symbol,
  watchlisted,
  watchlistBusy = false,
  onToggleWatchlist,
  chartInterval,
  price,
  currency,
  week52High,
  week52Low,
  stampSize = "sm",
  className,
}: StockSymbolActionBarProps) {
  return (
    <div className={cn("flex shrink-0 flex-wrap items-center gap-1.5", className)}>
      <button
        type="button"
        onClick={onToggleWatchlist}
        aria-busy={watchlistBusy}
        aria-pressed={watchlisted}
        className={cn(
          "inline-flex shrink-0 items-center gap-1 rounded-lg border border-[var(--border)] px-2 py-1 text-[11px] hover:bg-[var(--surface-elevated)]",
          watchlistBusy && "pointer-events-none",
          watchlisted ? "text-amber-500" : "text-[var(--text-secondary)]",
        )}
      >
        <Star
          className={cn(
            "h-3.5 w-3.5 transition-colors",
            watchlisted && "fill-amber-400 text-amber-400",
            watchlistBusy && "opacity-80",
          )}
        />
        {watchlisted ? "관심 해제" : "관심 추가"}
      </button>
      <StampButton
        symbol={symbol}
        interval={chartInterval}
        price={price}
        currency={currency}
        week52High={week52High}
        week52Low={week52Low}
        size={stampSize}
      />
      <PublishCallButton symbol={symbol} />
      <Link
        href="/journal"
        title="메모장에서 이 종목에 대한 기록 남기기"
        className={linkPillClass}
      >
        <NotebookPen className="h-3.5 w-3.5" />
        메모장
      </Link>
      <Link
        href="/targets"
        title="목표가 관리에서 이 종목의 목표가 설정"
        className={linkPillClass}
      >
        <Target className="h-3.5 w-3.5" />
        목표가 관리
      </Link>
    </div>
  );
}
