"use client";

import { Card } from "@/components/ui/card";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import type { WatchlistRow } from "@/lib/watchlist-calc";
import { watchBucket, type WatchSignal } from "@/lib/watchlist-signals";
import { cn } from "@/lib/utils";

/**
 * 목록 맨 위 한 줄 — **훑기 전에 답이 나오는 자리**.
 *
 * 지금은 이 값들이 우측 레일 「오늘의 관심종목」 카드에 있고, `xl` 미만에서는 목록
 * **아래로** 내려간다. 스무 줄을 다 지나야 "오늘 몇 개가 올랐나"를 알게 되는 셈이라
 * 순서가 뒤집혀 있었다. 요약은 목록보다 먼저 와야 한다.
 *
 * 값은 화면이 이미 들고 있는 것으로만 만든다 — 추가 요청 0.
 */
export function WlSummaryBar({
  rows,
  signals,
  className,
}: {
  rows: WatchlistRow[];
  signals: Map<string, WatchSignal[]>;
  className?: string;
}) {
  const priced = rows.filter((r) => r.quote && r.quote.currentPrice > 0);
  const up = priced.filter((r) => (r.quote?.changeRate ?? 0) > 0).length;
  const down = priced.filter((r) => (r.quote?.changeRate ?? 0) < 0).length;
  const avg =
    priced.length > 0
      ? priced.reduce((sum, r) => sum + (r.quote?.changeRate ?? 0), 0) / priced.length
      : null;

  const sinceRows = rows.filter((r) => r.sinceAddedRate != null);
  const sinceAvg =
    sinceRows.length > 0
      ? sinceRows.reduce((sum, r) => sum + (r.sinceAddedRate ?? 0), 0) / sinceRows.length
      : null;

  const action = rows.filter((r) => watchBucket(signals.get(r.symbol) ?? []) === "action").length;

  return (
    <Card
      className={cn(
        "flex flex-wrap items-center gap-x-4 gap-y-1.5 px-3 py-2 text-cell",
        className,
      )}
    >
      <span
        className={cn(
          "flex items-center gap-1.5 font-semibold",
          action > 0 ? "text-[var(--brand)]" : "text-[var(--text-muted)]",
        )}
      >
        {action > 0 && <span className="size-1.5 rounded-full bg-[var(--brand)]" aria-hidden />}
        지금 볼 것 {action}
        <span className="font-normal text-[var(--text-muted)]">/ {rows.length}종목</span>
      </span>

      <Stat
        label="오늘"
        value={
          <>
            <span className="text-[var(--profit)]">{up}</span>
            <span className="text-[var(--text-muted)]"> · </span>
            <span className="text-[var(--loss)]">{down}</span>
          </>
        }
        hint="상승 · 하락 종목 수"
      />

      {avg != null && (
        <Stat
          label="평균 등락"
          value={<span className={trendTextClass(avg)}>{formatPercent(avg)}</span>}
        />
      )}

      {sinceAvg != null && (
        <Stat
          label="담은 뒤 평균"
          value={<span className={trendTextClass(sinceAvg)}>{formatPercent(sinceAvg)}</span>}
          hint="담은 시점 가격 대비 단순 평균 — 실제 매매 손익이 아닙니다"
        />
      )}
    </Card>
  );
}

function Stat({
  label,
  value,
  hint,
}: {
  label: string;
  value: React.ReactNode;
  hint?: string;
}) {
  return (
    <span className="flex items-baseline gap-1.5" title={hint}>
      <span className="text-micro text-[var(--text-muted)]">{label}</span>
      <span className="font-num font-semibold tabular-nums">{value}</span>
    </span>
  );
}
