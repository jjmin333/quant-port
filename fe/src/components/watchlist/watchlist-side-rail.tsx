"use client";

import { useMemo } from "react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { useSymbolQuickLook } from "@/components/stocks/symbol-quick-look-modal";
import { IndexPulseCard } from "@/components/market/index-pulse-card";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { WatchlistNewsCard } from "@/components/watchlist/watchlist-news-card";
import { WatchlistScheduleCard } from "@/components/watchlist/watchlist-schedule-card";
import { WatchlistSinceAddedCard } from "@/components/watchlist/watchlist-since-added-card";
import {
  SectionCard,
  SectionEmpty,
  SectionKeyValue,
} from "@/components/ui/section-card";
import type { UpcomingEvent } from "@/hooks/use-upcoming-events";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import type { QuoteSummary, StockInfo } from "@/lib/quotes-types";
import type { WatchlistRow } from "@/lib/watchlist-calc";
import { cn } from "@/lib/utils";

/**
 * 관심종목 화면의 우측 레일.
 *
 * "종목 추가" 검색 카드가 목록 맨 위를 차지하고 있었다. 추가는 가끔 하는 일인데 매번
 * 목록을 한 칸 아래로 밀어냈다 — 자주 쓰는 것이 위로 와야 한다. 레일로 옮기고, 그 자리에
 * 목록을 훑기 전에 알고 싶은 것(오늘 몇 개가 오르고 내렸는지, 어느 종목이 튀는지)을 둔다.
 *
 * 08-14에 세 장을 더했다 — **본문이 답하지 않는 것**만. 목록은 "지금 얼마"만 말하므로
 * 앞으로의 일정, 무슨 일이 있었는지(뉴스), 담아둔 뒤 어떻게 됐는지를 옆에 세운다.
 * 오늘 집계와 담은 뒤 성과는 화면에 이미 있는 값으로 계산해 **추가 요청이 0**이고,
 * 일정은 목록 화면이 한 번 부른 결과를 그대로 받는다.
 */
export function WatchlistSideRail({
  rows,
  quotes,
  events,
  adding,
  onAddStock,
}: {
  /** 화면에 보이는 순서 그대로 — 뉴스가 위쪽 종목부터 조회한다. */
  rows: WatchlistRow[];
  quotes: QuoteSummary[];
  events: UpcomingEvent[] | null;
  adding: boolean;
  onAddStock: (stock: StockInfo) => void;
}) {
  const stats = useMemo(() => {
    const priced = quotes.filter((q) => q.currentPrice > 0);
    if (priced.length === 0) return null;
    const up = priced.filter((q) => q.changeRate > 0).length;
    const down = priced.filter((q) => q.changeRate < 0).length;
    const avg = priced.reduce((sum, q) => sum + q.changeRate, 0) / priced.length;
    /*
     * 강세는 오른 종목에서만, 약세는 내린 종목에서만 뽑는다. 정렬 결과의 위 3개 / 아래
     * 3개를 그냥 자르면 종목이 다섯 개 이하일 때 같은 종목이 양쪽에 동시에 나오고,
     * 보합 종목이 "약세"로 불린다.
     */
    const sorted = [...priced].sort((a, b) => b.changeRate - a.changeRate);
    return {
      count: priced.length,
      up,
      down,
      flat: priced.length - up - down,
      avg,
      leaders: sorted.filter((q) => q.changeRate > 0).slice(0, 3),
      laggards: sorted
        .filter((q) => q.changeRate < 0)
        .slice(-3)
        .reverse(),
    };
  }, [quotes]);

  const newsTargets = useMemo(
    () =>
      rows
        .filter((r) => r.quote)
        .map((r) => ({
          symbol: r.symbol,
          name: r.quote?.name || r.symbol,
          market: (r.quote?.currency === "USD" ? "us" : "domestic") as "domestic" | "us",
        })),
    [rows],
  );

  /*
   * 레일 안에서 종목을 누르면 **모달로 연다**. 예전에는 `/analysis`로 떠났는데, 옆 칸을
   * 곁눈질하러 온 사람을 화면 밖으로 내보내는 셈이라 셋을 견주려면 뒤로가기를 세 번
   * 눌러야 했다. 「종목 분석 보기」는 모달 바닥에 그대로 있다.
   *
   * 목록이 이미 시세를 들고 있으므로 `seed`로 넘겨 **추가 요청 없이** 연다.
   */
  const quickLook = useSymbolQuickLook();

  return (
    <>
      <SectionCard title="종목 추가">
        <p className="mb-1.5 text-[11px] text-muted-foreground">
          종목명 또는 티커로 검색한 뒤 목록에서 선택하세요.
        </p>
        <StockCatalogSearchField
          disabled={adding}
          suggestionsId="watchlist-stock-suggestions"
          onSelect={onAddStock}
        />
      </SectionCard>

      <SectionCard title="오늘의 관심종목">
        {!stats ? (
          <SectionEmpty text="시세를 불러오면 요약이 표시됩니다" />
        ) : (
          <>
            <SectionKeyValue label="종목 수" value={`${stats.count}개`} />
            <SectionKeyValue
              label="상승 / 보합 / 하락"
              value={
                <>
                  <span className="text-[var(--profit)]">{stats.up}</span>
                  {" / "}
                  {stats.flat}
                  {" / "}
                  <span className="text-[var(--loss)]">{stats.down}</span>
                </>
              }
            />
            <SectionKeyValue
              label="평균 등락률"
              value={formatPercent(stats.avg)}
              valueClassName={trendTextClass(stats.avg)}
            />
          </>
        )}
      </SectionCard>

      {stats && (stats.leaders.length > 0 || stats.laggards.length > 0) && (
        <SectionCard title="오늘 많이 움직인 종목">
          {stats.leaders.length > 0 && (
            <MoverList label="강세" rows={stats.leaders} onPick={quickLook.open} />
          )}
          {stats.laggards.length > 0 && (
            <div className={cn(stats.leaders.length > 0 && "mt-2 border-t pt-2")}>
              <MoverList label="약세" rows={stats.laggards} onPick={quickLook.open} />
            </div>
          )}
        </SectionCard>
      )}

      {rows.length > 0 && <WatchlistScheduleCard events={events} onPick={quickLook.open} />}
      {rows.length > 0 && <WatchlistSinceAddedCard rows={rows} onPick={quickLook.open} />}
      {newsTargets.length > 0 && <WatchlistNewsCard symbols={newsTargets} />}

      <IndexPulseCard />

      {quickLook.modal}
    </>
  );
}

function MoverList({
  label,
  rows,
  onPick,
}: {
  label: string;
  rows: QuoteSummary[];
  onPick: (symbol: string, seed?: QuoteSummary | null) => void;
}) {
  return (
    <>
      <p className="mb-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </p>
      <ul className="min-w-0">
        {rows.map((q) => (
          <li key={q.symbol}>
            <button
              type="button"
              onClick={() => onPick(q.symbol, q)}
              className="flex h-[30px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-xs hover:bg-accent"
            >
              <StockAvatar
                symbol={q.symbol}
                name={q.name}
                currency={q.currency}
                assetType={q.assetType}
                size="xs"
                className="shrink-0"
              />
              <span className="min-w-0 flex-1 truncate text-left">{q.name || q.symbol}</span>
              <span
                className={cn(
                  "shrink-0 font-mono text-xs tabular-nums",
                  trendTextClass(q.changeRate),
                )}
              >
                {formatPercent(q.changeRate)}
              </span>
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}
