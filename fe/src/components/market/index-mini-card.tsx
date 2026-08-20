"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { PriceHistoryChart } from "@/components/charts/price-history-chart";
import { formatPercent } from "@/lib/format";
import type { MarketIndex } from "@/lib/quotes-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 지수 미니 카드 — `StockMiniCard`와 **같은 껍데기**를 쓴다.
 *
 * 기사 하나에 종목과 지수가 같이 걸리는 일은 흔하다("아마존 상승, 애플 하락, 서울
 * 증시 사상 최고"). 그런데 지금까지 모달은 종목만 카드로 냈고 지수는 아무 데도 없어,
 * 정작 기사 제목이 말하는 코스피를 확인하려면 모달을 닫고 마켓 화면으로 가야 했다.
 *
 * 종목 카드와 나란히 놓이므로 배치·글자 크기·차트를 그대로 맞춘다. 다른 것은 둘뿐 —
 * 「지수」 배지가 붙고(종목이 아님을 한눈에), 점수·액션 자리가 없다(지수는 담을 수도
 * 스탬프를 찍을 수도 없다).
 *
 * 스파크라인은 지수 응답이 이미 들고 오는 값이다(20포인트). 없으면 차트를 그리지
 * 않는다 — 빈 상자를 두느니 값과 등락률만 내는 편이 정직하다.
 */
export function IndexMiniCard({
  index,
  href = "/market",
  onNavigate,
  className,
}: {
  index: MarketIndex;
  href?: string;
  onNavigate?: () => void;
  className?: string;
}) {
  const spark = (index.sparkline ?? []).filter((v) => Number.isFinite(v) && v > 0);

  return (
    <div
      className={cn(
        "min-w-0 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-2.5",
        className,
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <Link
          href={href}
          onClick={onNavigate}
          className="group flex min-w-0 items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]"
        >
          <span className="truncate">{index.name}</span>
          <ArrowRight className="size-3 shrink-0 text-[var(--text-muted)] group-hover:text-[var(--brand)]" />
        </Link>
        <span className="shrink-0 rounded-md bg-[var(--surface-elevated)] px-1.5 py-0.5 text-[10px] font-semibold text-[var(--text-muted)]">
          지수
        </span>
      </div>

      <div className="mt-1.5 flex items-baseline gap-1.5">
        <span className="font-mono text-lg font-bold tabular-nums text-[var(--text-primary)]">
          {index.value > 0
            ? index.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })
            : "—"}
        </span>
        <span
          className={cn(
            "font-mono text-xs font-semibold tabular-nums",
            trendTextClass(index.changeRate),
          )}
        >
          {formatPercent(index.changeRate)}
        </span>
      </div>

      {spark.length >= 2 && (
        <PriceHistoryChart
          points={spark}
          currency={index.currency}
          valueFormat="plain"
          title="최근 흐름"
          height={80}
          className="mt-1.5"
        />
      )}
    </div>
  );
}
