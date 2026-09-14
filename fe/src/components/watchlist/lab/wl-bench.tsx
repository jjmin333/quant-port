"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight, Trash2 } from "lucide-react";
import { PriceHistoryChart } from "@/components/charts/price-history-chart";
import { SymbolNewsCard } from "@/components/stocks/symbol-news-card";
import { Card } from "@/components/ui/card";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { WatchlistRangeBar } from "@/components/watchlist/watchlist-range-bar";
import {
  chipSignals,
  EMPTY_RECORD,
  PriceBlock,
  ScoreBadge,
  SignalChip,
  SinceAdded,
  SymbolLabel,
  TargetLine,
  type SymbolRecord,
  type WlListProps,
} from "@/components/watchlist/lab/wl-shared";
import { useSparklineCandles } from "@/hooks/use-sparkline-candles";
import { daysUntil, type UpcomingEvent } from "@/hooks/use-upcoming-events";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { formatPercent, formatPrice } from "@/lib/format";
import type { StockScoreData } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import type { WatchlistRow } from "@/lib/watchlist-calc";
import { attentionScore, type WatchSignal } from "@/lib/watchlist-signals";
import { cn } from "@/lib/utils";

/**
 * 시안 D — **워크벤치(2단)**.
 *
 * 관심종목을 보는 일은 하나를 파고드는 게 아니라 **여럿을 견주는 일**이다. 지금 화면은
 * 그때마다 모달을 열고 닫는데, 셋을 비교하려면 열고-읽고-닫기를 세 번 한다. 왼쪽에 목록,
 * 오른쪽에 지금 고른 종목을 세우면 그 왕복이 사라진다 — 화면을 떠나지도, 덮지도 않는다.
 *
 * 종목 랭킹(`market-radar`)이 이미 이 구조로 서 있어 새 개념이 아니다.
 *
 * `xl`(1280px) 미만에서는 오른쪽 기둥을 세울 폭이 없다. 그 판정과 대체 화면은 몸통이
 * 맡는다(`watchlist-lab-client`) — 좁은 화면의 D는 시안 A와 같은 화면(목록 + 우측 레일)이다.
 */
export function WlBench({
  rows,
  records,
  signals,
  // `onOpen`(퀵룩 열기)은 여기서 쓰지 않는다 — 2단에서는 고르는 순간 오른쪽에 펼쳐지므로
  // 모달을 겹쳐 띄울 이유가 없다. 좁은 화면의 D는 몸통이 시안 A 화면으로 대체한다.
  onRemove,
  scores,
  eventsBySymbol,
  summary,
  toolbar,
}: WlListProps & {
  scores: Map<string, StockScoreData>;
  eventsBySymbol: Map<string, UpcomingEvent[]>;
  summary: React.ReactNode;
  toolbar: React.ReactNode;
}) {
  const [picked, setPicked] = useState<string | null>(null);

  const selected = useMemo(() => {
    if (rows.length === 0) return null;
    return rows.find((r) => r.symbol === picked) ?? rows[0];
  }, [rows, picked]);

  return (
    <div className="grid min-w-0 items-start gap-3 xl:grid-cols-[22rem_minmax(0,1fr)]">
      <aside className="flex min-w-0 flex-col gap-2 xl:sticky xl:top-3 xl:max-h-[calc(100dvh-5.5rem)] xl:overflow-y-auto xl:overscroll-contain">
        {summary}
        {toolbar}
        <Card className="min-w-0 overflow-hidden p-0">
          <ul className="min-w-0">
            {rows.map((row) => (
              <li key={row.symbol}>
                <PickRow
                  row={row}
                  active={selected?.symbol === row.symbol}
                  attention={attentionScore(signals.get(row.symbol) ?? [])}
                  topSignal={signals.get(row.symbol)?.[0]}
                  onPick={() => setPicked(row.symbol)}
                />
              </li>
            ))}
          </ul>
        </Card>
      </aside>

      <div className="min-w-0">
        {selected ? (
          <Detail
            row={selected}
            record={records.get(selected.symbol) ?? EMPTY_RECORD}
            signals={signals.get(selected.symbol) ?? []}
            score={scores.get(selected.symbol.toUpperCase()) ?? null}
            events={eventsBySymbol.get(selected.symbol) ?? []}
            onRemove={() => onRemove(selected.symbol)}
          />
        ) : (
          <Card className="p-8 text-center text-cell text-[var(--text-muted)]">
            왼쪽에서 종목을 고르세요.
          </Card>
        )}
      </div>
    </div>
  );
}

/** 왼쪽 목록 한 줄 — 고르는 것이 전부라 값은 셋(이름·등락·가장 무거운 신호)만 둔다. */
function PickRow({
  row,
  active,
  attention,
  topSignal,
  onPick,
}: {
  row: WatchlistRow;
  active: boolean;
  attention: number;
  topSignal?: WatchSignal;
  onPick: () => void;
}) {
  const q = row.quote;
  return (
    <button
      type="button"
      onClick={onPick}
      aria-current={active ? "true" : undefined}
      className={cn(
        "flex w-full min-w-0 items-center gap-2 border-b border-[var(--border)]/70 px-2.5 py-2 text-left transition-colors last:border-b-0",
        active ? "bg-[var(--brand-muted)]" : "hover:bg-[var(--surface-elevated)]",
      )}
    >
      <span
        className={cn(
          "size-1.5 shrink-0 rounded-full",
          attention >= 26
            ? "bg-[var(--brand)]"
            : attention >= 10
              ? "bg-[var(--text-muted)]/50"
              : "bg-transparent",
        )}
        aria-hidden
      />
      <StockAvatar
        symbol={row.symbol}
        name={q?.name}
        currency={q?.currency}
        assetType={q?.assetType}
        size="xs"
        className="shrink-0"
      />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-note font-semibold text-[var(--text-primary)]">
          {q?.name || row.symbol}
        </span>
        {topSignal && (
          <span className="block truncate text-micro text-[var(--text-muted)]">
            {topSignal.label}
          </span>
        )}
      </span>
      <span className="shrink-0 text-right">
        <span className="block font-num text-cell font-semibold tabular-nums text-[var(--text-primary)]">
          {q && q.currentPrice > 0 ? formatPrice(q.currentPrice, q.currency) : "—"}
        </span>
        <span
          className={cn(
            "block font-num text-micro tabular-nums",
            trendTextClass(q?.changeRate ?? 0),
          )}
        >
          {q && q.currentPrice > 0 ? formatPercent(q.changeRate) : ""}
        </span>
      </span>
    </button>
  );
}

/** 오른쪽 패널 — 목록이 답하지 않는 것을 모아 둔다. */
function Detail({
  row,
  record,
  signals,
  score,
  events,
  onRemove,
}: {
  row: WatchlistRow;
  record: SymbolRecord;
  signals: WatchSignal[];
  score: StockScoreData | null;
  events: UpcomingEvent[];
  onRemove: () => void;
}) {
  const q = row.quote;
  const metrics = q?.watchlistMetrics;
  const { points, times } = useSparklineCandles(row.symbol, true);

  return (
    <Card className="@container flex min-w-0 flex-col gap-3 p-4">
      <div className="flex min-w-0 items-start justify-between gap-3">
        <SymbolLabel
          row={row}
          record={record}
          size="md"
          className="min-w-0 flex-1"
          sub={<SinceAdded row={row} />}
        />
        <div className="flex shrink-0 items-center gap-2">
          <ScoreBadge score={row.score} grade={row.scoreGrade} />
          <PriceBlock quote={q} size="md" />
        </div>
      </div>

      {/* 패널은 폭이 넉넉하니 신호를 자르지 않고 전부 편다. */}
      {chipSignals(signals).length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {chipSignals(signals).map((s) => (
            <SignalChip key={s.id} signal={s} />
          ))}
        </div>
      )}

      {points.length >= 2 && (
        <PriceHistoryChart
          points={points}
          times={times}
          currency={q?.currency ?? "KRW"}
          markerPrice={row.addedPrice}
          markerLabel="담은 가격"
          title="최근 흐름"
          height={140}
        />
      )}

      <WatchlistRangeBar
        size="md"
        pct={row.rangePct}
        low={metrics?.week52Low}
        high={metrics?.week52High}
        currency={q?.currency ?? "KRW"}
      />

      <div className="grid grid-cols-2 gap-2 rounded-lg bg-[var(--surface-elevated)]/60 p-2.5 text-cell sm:grid-cols-4">
        <Stat label="담은 날" value={row.addedAt ? row.addedAt.slice(0, 10) : "—"} />
        <Stat
          label="담은 가격"
          value={row.addedPrice != null ? formatPrice(row.addedPrice, q?.currency ?? "KRW") : "—"}
        />
        <Stat
          label="담은 뒤"
          value={row.sinceAddedRate != null ? formatPercent(row.sinceAddedRate) : "—"}
          className={row.sinceAddedRate != null ? trendTextClass(row.sinceAddedRate) : undefined}
        />
        <Stat label="보유" value={q?.isHeld ? "보유 중" : "미보유"} />
      </div>

      <div className="flex min-w-0 flex-wrap items-baseline justify-between gap-2 text-cell">
        {record.target && q ? (
          <TargetLine target={record.target} price={q.currentPrice} />
        ) : (
          <span className="text-[var(--text-muted)]">적어 둔 목표가 없음</span>
        )}
        {score?.tradeDate && (
          <span className="text-micro text-[var(--text-muted)]">점수 기준일 {score.tradeDate}</span>
        )}
      </div>

      {events.length > 0 && (
        <div className="border-t border-[var(--border)]/60 pt-2">
          <p className="mb-1 text-cell font-semibold text-[var(--text-secondary)]">다가오는 일정</p>
          <ul className="min-w-0 space-y-0.5">
            {events.slice(0, 4).map((ev, i) => {
              const d = daysUntil(ev.date);
              return (
                <li key={`${ev.date}-${i}`} className="flex items-center gap-2 text-cell">
                  <span className="w-14 shrink-0 font-num tabular-nums text-[var(--text-muted)]">
                    {ev.date.slice(5)}
                  </span>
                  <span className="min-w-0 flex-1 truncate">{ev.label || "실적 발표"}</span>
                  {d != null && (
                    <span className="shrink-0 font-num text-micro text-[var(--brand)]">D-{d}</span>
                  )}
                </li>
              );
            })}
          </ul>
        </div>
      )}

      {q && (
        <SymbolNewsCard
          symbols={[
            {
              symbol: row.symbol,
              name: q.name || row.symbol,
              market: q.currency === "USD" ? "us" : "domestic",
            },
          ]}
          title="이 종목 뉴스"
          subjectLabel={q.name || row.symbol}
        />
      )}

      <div className="flex items-center justify-between gap-2 border-t border-[var(--border)]/60 pt-2">
        <Link
          href={analysisSymbolHref(row.symbol)}
          className="inline-flex items-center gap-1 text-cell font-medium text-[var(--brand)] hover:underline"
        >
          종목 분석 보기
          <ArrowRight className="size-3" />
        </Link>
        <button
          type="button"
          onClick={onRemove}
          className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-cell text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--danger)]"
        >
          <Trash2 className="size-3.5" />
          관심종목에서 빼기
        </button>
      </div>
    </Card>
  );
}

function Stat({
  label,
  value,
  className,
}: {
  label: string;
  value: React.ReactNode;
  className?: string;
}) {
  return (
    <span className="block min-w-0">
      <span className="block text-micro text-[var(--text-muted)]">{label}</span>
      <span className={cn("block truncate font-num font-semibold tabular-nums", className)}>
        {value}
      </span>
    </span>
  );
}
