"use client";

import { TrendingUp } from "lucide-react";
import { SectionCard, SectionEmpty, SectionKeyValue } from "@/components/ui/section-card";
import { formatPercent } from "@/lib/format";
import type { QuoteSummary } from "@/lib/quotes-types";
import { trendTextClass } from "@/lib/trend-colors";
import { sinceAddedStats, type WatchlistRow } from "@/lib/watchlist-calc";

/**
 * 담은 뒤 성과.
 *
 * 관심종목은 "샀으면 어땠을까"의 기록이다. 그런데 지금까지 화면 어디에도 그 답이 없었다
 * — 담은 가격은 BE가 저장하고 있었지만 아무도 읽지 않았다.
 *
 * ⚠️ 이건 **수익률이 아니라 관찰 기록**이다. 돈이 들어간 적이 없으니 "수익"이라 부르지
 * 않고, 단순 평균을 쓴다(가중할 금액 자체가 없다).
 */
export function WatchlistSinceAddedCard({
  rows,
  onPick,
}: {
  rows: WatchlistRow[];
  /** 최고·최저로 뽑힌 종목을 누르면 퀵룩이 열린다 — 이름만 적어두면 확인하러 갈 곳이 없다. */
  onPick: (symbol: string, seed?: QuoteSummary | null) => void;
}) {
  const stats = sinceAddedStats(rows);

  return (
    <SectionCard
      title={
        <span className="flex items-center gap-1.5">
          <TrendingUp className="size-3.5" />
          담은 뒤 성과
        </span>
      }
    >
      {!stats ? (
        <SectionEmpty text="담은 가격이 기록된 종목이 아직 없습니다" />
      ) : (
        <>
          <SectionKeyValue
            label="평균"
            value={formatPercent(stats.average)}
            valueClassName={trendTextClass(stats.average)}
          />
          <SectionKeyValue
            label="플러스 / 마이너스"
            value={
              <>
                <span className="text-[var(--profit)]">{stats.winners}</span>
                {" / "}
                <span className="text-[var(--loss)]">{stats.losers}</span>
              </>
            }
          />
          <SectionKeyValue
            label="최고"
            value={
              <button
                type="button"
                onClick={() => onPick(stats.best.symbol, stats.best.quote)}
                className="rounded px-0.5 hover:bg-accent"
              >
                <span className="text-foreground">
                  {stats.best.quote?.name || stats.best.symbol}
                </span>{" "}
                <span className={trendTextClass(stats.best.sinceAddedRate ?? 0)}>
                  {formatPercent(stats.best.sinceAddedRate ?? 0)}
                </span>
              </button>
            }
          />
          <SectionKeyValue
            label="최저"
            value={
              <button
                type="button"
                onClick={() => onPick(stats.worst.symbol, stats.worst.quote)}
                className="rounded px-0.5 hover:bg-accent"
              >
                <span className="text-foreground">
                  {stats.worst.quote?.name || stats.worst.symbol}
                </span>{" "}
                <span className={trendTextClass(stats.worst.sinceAddedRate ?? 0)}>
                  {formatPercent(stats.worst.sinceAddedRate ?? 0)}
                </span>
              </button>
            }
          />
          <SectionKeyValue
            label="평균 관심 기간"
            value={stats.avgHoldDays == null ? "—" : `${Math.round(stats.avgHoldDays)}일`}
          />
          <p className="mt-1 text-micro leading-relaxed text-muted-foreground">
            {stats.count}종목 기준 · 담은 시점 가격 대비 단순 평균입니다. 실제 매매 손익이
            아닙니다.
          </p>
        </>
      )}
    </SectionCard>
  );
}
