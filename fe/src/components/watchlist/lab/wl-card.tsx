"use client";

import { Trash2 } from "lucide-react";
import { Card } from "@/components/ui/card";
import { WatchlistRangeBar } from "@/components/watchlist/watchlist-range-bar";
import {
  chipSignals,
  PriceBlock,
  ScoreBadge,
  SignalChipRow,
  SinceAdded,
  SymbolLabel,
  TargetLine,
  WlSpark,
  type SymbolRecord,
} from "@/components/watchlist/lab/wl-shared";
import { holdDays, type WatchSignal } from "@/lib/watchlist-signals";
import type { WatchlistRow } from "@/lib/watchlist-calc";
import { cn } from "@/lib/utils";

/**
 * 시안 B가 쓰는 종목 카드 한 장.
 *
 * 목록 행은 **비교**에 강하고(같은 값이 세로선 위에 선다) 카드는 **한 종목을 이해하는 데**
 * 강하다. 관심종목은 보통 열댓 개라 표가 필요할 만큼 길지 않고, 오히려 종목마다 "지금
 * 어떤 상태인가"를 한눈에 보는 쪽이 맞을 수 있다 — 그 가정을 재는 시안이다.
 *
 * 카드 한 장에 지금 목록이 가진 것이 **다 들어간다**(점수·52주 위치·담은 뒤·현재가)
 * 그 위에 행에서는 자리가 없던 것을 더한다 — 최근 흐름, 내가 적어 둔 목표가, 신호.
 * 모바일에서 한 장, 태블릿에서 두 장, PC에서 세 장이 한 줄에 선다.
 */
export function WlCard({
  row,
  record,
  signals,
  onOpen,
  onRemove,
}: {
  row: WatchlistRow;
  record: SymbolRecord;
  signals: WatchSignal[];
  onOpen: () => void;
  onRemove: () => void;
}) {
  const q = row.quote;
  const metrics = q?.watchlistMetrics;
  const held = row.addedAt ? holdDays(row.addedAt) : null;

  return (
    <Card className="@container group relative flex min-w-0 flex-col gap-2 p-3 transition-colors hover:border-[var(--brand)]">
      {/*
        삭제는 카드를 여는 버튼 **밖**에 둔다. 버튼 안의 버튼은 무효 HTML이라 브라우저가
        마크업을 재구성하고, 그러면 안쪽 버튼이 클릭을 못 받는다(마켓 레이더 행에서 같은
        문제를 겪었다). 그래서 절대 위치로 겹쳐 올린다.
      */}
      <button
        type="button"
        onClick={onRemove}
        aria-label={`${q?.name ?? row.symbol} 관심종목에서 삭제`}
        className="absolute right-1.5 top-1.5 z-10 rounded-lg p-1.5 text-[var(--text-muted)] opacity-0 transition-opacity hover:bg-[var(--surface-elevated)] hover:text-[var(--danger)] focus-visible:opacity-100 group-hover:opacity-100"
      >
        <Trash2 className="h-3.5 w-3.5" />
      </button>

      <button
        type="button"
        onClick={onOpen}
        aria-label={`${q?.name ?? row.symbol} 상세`}
        className="flex min-w-0 flex-col gap-2 text-left"
      >
        <div className="flex min-w-0 items-start gap-2">
          <SymbolLabel
            row={row}
            record={record}
            size="md"
            className="min-w-0 flex-1"
            sub={
              held != null ? (
                <span className="truncate">담은 지 {held}일</span>
              ) : undefined
            }
          />
          <ScoreBadge score={row.score} grade={row.scoreGrade} className="mr-6 shrink-0" />
        </div>

        <div className="flex min-w-0 items-end justify-between gap-2">
          <PriceBlock quote={q} size="md" align="left" className="min-w-0" />
          <WlSpark
            symbol={row.symbol}
            changeRate={q?.changeRate ?? 0}
            className="h-8 w-[6.5rem] shrink-0"
          />
        </div>

        <WatchlistRangeBar
          size="md"
          pct={row.rangePct}
          low={metrics?.week52Low}
          high={metrics?.week52High}
          currency={q?.currency ?? "KRW"}
        />

        <div
          className={cn(
            "flex min-w-0 items-baseline justify-between gap-2 border-t border-[var(--border)]/60 pt-1.5",
          )}
        >
          <SinceAdded row={row} />
          {record.target && q ? (
            <TargetLine target={record.target} price={q.currentPrice} className="text-right" />
          ) : (
            <span className="text-cell text-[var(--text-muted)]">목표가 없음</span>
          )}
        </div>

        <SignalChipRow signals={chipSignals(signals)} max={4} />
      </button>
    </Card>
  );
}
