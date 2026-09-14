"use client";

import { useMemo } from "react";
import { useIndexQuickLook } from "@/components/market/index-quick-look-modal";
import { SectionCard, SectionSkeleton } from "@/components/ui/section-card";
import { useMarketIndices } from "@/hooks/use-market-indices";
import type { MarketIndex } from "@/lib/quotes-types";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 주요 지수 5줄. 여러 화면의 우측 레일이 공유한다.
 *
 * 화면이 무엇을 다루느냐에 따라 먼저 볼 지수가 다르다. 환율은 어느 쪽에서든 남긴다 —
 * 해외 종목은 원화 환산이, 국내 종목은 외국인 수급이 환율에 묶여 있다.
 */
export type IndexPulseFocus = "domestic" | "us";

export const INDEX_PRIORITY: Record<IndexPulseFocus, string[]> = {
  domestic: ["kospi", "kosdaq", "usd-krw", "sp500", "nasdaq"],
  us: ["nasdaq", "sp500", "dow", "usd-krw", "kospi"],
};

export function pickMarketIndices(
  indices: MarketIndex[],
  focus: IndexPulseFocus,
  limit = 5,
): MarketIndex[] {
  const order = INDEX_PRIORITY[focus];
  const byId = new Map(indices.map((i) => [i.id, i]));
  const picked = order.map((id) => byId.get(id)).filter((i) => i !== undefined);
  const rest = indices.filter((i) => !order.includes(i.id));
  return [...picked, ...rest].slice(0, limit);
}

export function IndexPulseCard({
  focus = "domestic",
  title = "지수 흐름",
}: {
  focus?: IndexPulseFocus;
  title?: string;
}) {
  const { indices } = useMarketIndices();

  const rows = useMemo(() => pickMarketIndices(indices, focus), [indices, focus]);

  // 값이 하나도 안 채워졌으면(플레이스홀더 그대로) 0.00%를 늘어놓는 대신 로딩으로 둔다.
  const hasData = rows.some((i) => i.value > 0);

  /*
   * 줄을 누르면 홈 티커 스트립과 **같은 지수 모달**이 열린다. 예전에는 다섯 줄 중
   * 아무것도 눌리지 않아, 코스피가 왜 그렇게 움직였는지 물으려면 「마켓 레이더」로
   * 화면을 떠나야 했다. 그 링크는 카드 제목 옆에 그대로 남아 있다.
   */
  const quickLook = useIndexQuickLook(indices);

  return (
    <>
      <SectionCard title={title} href="/market" hrefLabel="마켓 레이더">
        {!hasData ? (
          <SectionSkeleton rows={5} />
        ) : (
          <ul className="min-w-0">
            {rows.map((idx) => (
              <li key={idx.id} className="border-b border-border/60 last:border-b-0">
                <button
                  type="button"
                  onClick={() => quickLook.open(idx)}
                  className="flex h-[30px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-cell hover:bg-accent"
                >
                  <span className="min-w-0 flex-1 truncate text-left">{idx.name}</span>
                  <span className="shrink-0 font-num text-cell tabular-nums">
                    {idx.value
                      ? idx.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })
                      : "—"}
                  </span>
                  <span
                    className={cn(
                      "w-14 shrink-0 text-right font-num text-cell tabular-nums",
                      trendTextClass(idx.changeRate),
                    )}
                  >
                    {formatPercent(idx.changeRate)}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      {quickLook.modal}
    </>
  );
}
