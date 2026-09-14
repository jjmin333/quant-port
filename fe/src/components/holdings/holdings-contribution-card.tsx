"use client";

import { useMemo } from "react";
import { Card } from "@/components/ui/card";
import { formatKRW, formatPercent, formatPriceChange } from "@/lib/format";
import type { HoldingMetrics, HoldingsTotals } from "@/lib/holdings-calc";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 오늘 손익 기여도 — "오늘 +30만원"이 어디서 왔나.
 *
 * 등락률 순위와 답이 다르다는 게 이 위젯의 존재 이유다. 5% 오른 종목이라도 비중이
 * 1%면 지갑은 거의 안 움직이고, 1% 오른 대장주 하나가 그날 손익의 절반일 수 있다.
 * 그래서 여기서는 **금액**으로 줄을 세운다.
 *
 * 막대는 가운데(0원)를 기준으로 양쪽으로 뻗는다. 폭은 그날 **가장 크게 기여한 종목의
 * 절댓값**에 맞춘다 — 합계에 맞추면 서로 상쇄된 날에 모든 막대가 화면 밖으로 나간다.
 */
export function HoldingsContributionCard({
  metrics,
  totals,
  onSelect,
}: {
  metrics: HoldingMetrics[];
  totals: HoldingsTotals | null;
  onSelect?: (symbol: string) => void;
}) {
  const rows = useMemo(() => {
    const priced = metrics.filter((m) => m.dayPnlKRW != null && m.dayPnlKRW !== 0);
    return priced
      .sort((a, b) => (b.dayPnlKRW ?? 0) - (a.dayPnlKRW ?? 0))
      .slice(0, 8);
  }, [metrics]);

  const scale = useMemo(
    () => Math.max(1, ...rows.map((m) => Math.abs(m.dayPnlKRW ?? 0))),
    [rows],
  );

  return (
    <Card className="p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <p className="text-cell font-semibold">오늘 손익 기여</p>
        {totals?.canTotal && (
          <span className={cn("font-num text-cell tabular-nums", trendTextClass(totals.dayPnlKRW))}>
            합계 {formatPriceChange(Math.round(totals.dayPnlKRW), "KRW")}
          </span>
        )}
      </div>

      {rows.length === 0 ? (
        <p className="py-8 text-center text-cell text-muted-foreground">
          오늘 값이 움직인 보유 종목이 없습니다
        </p>
      ) : (
        <ul className="min-w-0">
          {rows.map((m) => {
            const pnl = m.dayPnlKRW ?? 0;
            const width = (Math.abs(pnl) / scale) * 50;
            const up = pnl > 0;
            return (
              <li key={m.symbol}>
                <button
                  type="button"
                  onClick={() => onSelect?.(m.symbol)}
                  className="grid w-full grid-cols-[minmax(0,1fr)_84px_84px] items-center gap-2 rounded-md px-1 py-1 text-left text-cell hover:bg-accent"
                >
                  <span className="min-w-0 truncate">{m.name}</span>
                  <span className="relative h-3.5">
                    <span
                      className="absolute inset-y-[-2px] left-1/2 w-px bg-border"
                      aria-hidden
                    />
                    <span
                      className="absolute top-0.5 h-2.5 rounded-sm"
                      style={{
                        left: up ? "50%" : `${50 - width}%`,
                        width: `${width}%`,
                        backgroundColor: up ? "var(--profit)" : "var(--loss)",
                      }}
                      aria-hidden
                    />
                  </span>
                  <span className={cn("text-right font-num tabular-nums", trendTextClass(pnl))}>
                    {formatPriceChange(Math.round(pnl), "KRW")}
                    <span className="block text-micro">{formatPercent(m.changeRate)}</span>
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}

      {totals && !totals.canTotal && (
        <p className="mt-2 border-t pt-1.5 text-micro text-muted-foreground">
          환율을 못 받아 달러 종목의 기여는 제외했습니다.
        </p>
      )}
      {totals?.canTotal && rows.length > 0 && (
        <p className="mt-2 border-t pt-1.5 text-micro text-muted-foreground">
          등락률이 아니라 <b className="text-foreground">금액</b> 기준입니다 — 비중이 큰 종목은
          조금만 움직여도 위로 올라옵니다. 총 평가금액 {formatKRW(totals.marketValueKRW)}.
        </p>
      )}
    </Card>
  );
}
