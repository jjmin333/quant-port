"use client";

import { GripVertical, Trash2 } from "lucide-react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  WL_CELL_H,
  WL_COL,
  WL_ROW_INNER,
  WL_ROW_OUTER,
  WatchlistCell,
  WatchlistScoreCell,
} from "@/components/watchlist/watchlist-columns";
import { WatchlistRangeBar } from "@/components/watchlist/watchlist-range-bar";
import { Card } from "@/components/ui/card";
import { daysUntil, type UpcomingEvent } from "@/hooks/use-upcoming-events";
import { formatPercent, formatPrice, formatPriceChange } from "@/lib/format";
import { ValueFlash } from "@/components/ui/value-flash";
import { SCORE_TONE_BADGE, scoreTone } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import type { WatchlistRow as Row } from "@/lib/watchlist-calc";
import type { MyTodayItem } from "@/lib/my-today-api";
import { firstSentences } from "@/lib/summary-sentences";
import { cn } from "@/lib/utils";

/**
 * 관심종목 한 줄.
 *
 * 08-14 A안: **라벨을 행에서 걷어냈다.** 예전에는 「전일·전주·전월·담은 뒤」가 종목 수만큼
 * 인쇄돼 숫자보다 라벨이 많았다. 이제 라벨은 목록 맨 위 헤더 한 줄이 맡고, 행에는 값만
 * 남는다.
 *
 * 칸은 전부 비율로 늘어나 **카드 폭을 다 쓴다** — 고정 폭으로 잡았더니 넓은 화면에서
 * 남는 자리가 현재가 오른쪽에 몰려 카드의 40%가 비었다. 폭·접힘 규칙은 `WL_COL` 한
 * 곳에 있고 헤더가 같은 상수를 본다.
 *
 * 값이 계단처럼 어긋나지 않는 것은 `WatchlistCell`이 값 줄 19px·보조 줄 13px로 높이를
 * **고정**하고, 모든 칸이 같은 높이(`WL_CELL_H`) 안에서 가운데 서기 때문이다.
 */
export function WatchlistRow({
  row,
  pending,
  draggable,
  isDragging,
  isDropTarget,
  events,
  today,
  onOpen,
  onRemove,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDrop,
}: {
  row: Row;
  pending: boolean;
  /** 「내 순서」 모드일 때만 참 — 정렬된 화면에서 드래그를 받으면 내 순서가 덮어써진다. */
  draggable: boolean;
  isDragging: boolean;
  isDropTarget: boolean;
  events: UpcomingEvent[];
  /** 오늘의 신호(`/user/today`) — 뉴스 수·변화 한 줄·AI 한 줄. 예시 모드에는 없다. */
  today?: MyTodayItem;
  onOpen: () => void;
  onRemove: () => void;
  onDragStart: () => void;
  onDragEnd: () => void;
  onDragOver: (e: React.DragEvent) => void;
  onDrop: (e: React.DragEvent) => void;
}) {
  const q = row.quote;
  const metrics = q?.watchlistMetrics;
  const hasPrice = Boolean(q && q.currentPrice > 0);

  // 가장 가까운 예정 일정. 며칠 안 남은 것만 알린다 — 두 달 뒤 실적일은 지금 이 화면에서
  // 할 일이 없다.
  const nextEvent = events[0];
  const nextEventDays = nextEvent ? daysUntil(nextEvent.date) : null;
  const eventSoon = nextEventDays != null && nextEventDays <= 14;

  return (
    <Card
      className={cn(
        "flex flex-col gap-1 px-3 py-2.5 transition-colors hover:border-[var(--brand)] hover:bg-[var(--surface-elevated)]/40",
        isDragging && "opacity-50",
        isDropTarget && "border-[var(--brand)]/50",
      )}
      onDragOver={onDragOver}
      onDrop={onDrop}
    >
      <div className={cn(WL_ROW_OUTER, "min-w-0")}>
      <span className={WL_COL.grip}>
        {draggable && (
          <button
            type="button"
            draggable
            onDragStart={onDragStart}
            onDragEnd={onDragEnd}
            className="cursor-grab rounded p-1 text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-secondary)] active:cursor-grabbing"
            aria-label={`${row.symbol} 순서 변경`}
          >
            <GripVertical className="h-4 w-4" />
          </button>
        )}
      </span>

      <button
        type="button"
        onClick={onOpen}
        className={cn(WL_ROW_INNER, "rounded-lg text-left")}
        aria-label={`${q?.name ?? row.symbol} 상세`}
      >
        {/* 종목 */}
        <span className={cn(WL_COL.name, WL_CELL_H, "flex items-center gap-2.5")}>
          <StockAvatar
            symbol={row.symbol}
            name={q?.name}
            currency={q?.currency}
            assetType={q?.assetType}
            size="xs"
            className="@2xl:h-8 @2xl:w-8"
          />
          <span className="min-w-0 flex-1">
            <span className="flex min-w-0 items-center gap-1">
              <span className="truncate text-note font-semibold leading-[19px] text-[var(--text-primary)]">
                {q?.name || row.symbol}
              </span>
              {eventSoon && (
                <span
                  className="h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--brand)]"
                  title={`D-${nextEventDays} · ${nextEvent.label.trim()}`}
                />
              )}
            </span>
            <span className="block truncate text-micro leading-[13px] text-[var(--text-muted)]">
              {row.symbol}
            </span>
          </span>
        </span>

        {/* 종합 점수 */}
        <WatchlistScoreCell className={WL_COL.score}>
          {row.score != null ? (
            <span
              className={cn(
                "grid h-[19px] w-full place-items-center rounded-md text-cell font-bold tabular-nums",
                SCORE_TONE_BADGE[scoreTone(row.score)],
              )}
              title={`종합 점수 ${row.score}${row.scoreGrade ? ` (${row.scoreGrade})` : ""}`}
            >
              {Math.round(row.score)}
            </span>
          ) : (
            <span className="text-cell text-[var(--text-muted)]" title="산출된 점수 없음">
              —
            </span>
          )}
        </WatchlistScoreCell>

        {/* 52주 위치 */}
        <WatchlistRangeBar
          size="sm"
          pct={row.rangePct}
          low={metrics?.week52Low}
          high={metrics?.week52High}
          currency={q?.currency ?? "KRW"}
          className={cn(WL_COL.range, WL_CELL_H)}
        />

        {/* 기간 등락 */}
        <PeriodCell className={WL_COL.prevDay} rate={metrics?.changeRatePrevClose} pending={pending} />
        <PeriodCell className={WL_COL.prevWeek} rate={metrics?.changeRatePrevWeek} pending={pending} />
        <PeriodCell className={WL_COL.prevMonth} rate={metrics?.changeRatePrevMonth} pending={pending} />

        {/* 담은 뒤 */}
        <WatchlistCell
          className={WL_COL.since}
          value={row.sinceAddedRate != null ? formatPercent(row.sinceAddedRate) : "—"}
          valueClassName={
            row.sinceAddedRate != null
              ? trendTextClass(row.sinceAddedRate)
              : "text-[var(--text-muted)]"
          }
          sub={row.addedAt ? row.addedAt.slice(5, 10) : "기록 없음"}
        />

        {/* 현재가 — 실시간 차트 목록과 같은 플래시를 준다(2026-09-04 사용자 요청 4).
            `WatchlistCell`이 값 칸을 `truncate`된 고정 높이 span으로 그리므로,
            감싸는 자리는 그 안쪽이다 — 바깥에 두면 셀의 두 줄 레이아웃이 깨진다. */}
        <WatchlistCell
          className={WL_COL.price}
          value={
            pending ? (
              "…"
            ) : hasPrice && q ? (
              <ValueFlash value={q.currentPrice} resetKey={row.symbol}>
                {formatPrice(q.currentPrice, q.currency)}
              </ValueFlash>
            ) : (
              "—"
            )
          }
          valueClassName={cn("text-section font-bold", !hasPrice && "text-[var(--text-muted)]")}
          sub={
            pending || !hasPrice || !q ? null : (
              <span className={trendTextClass(q.changeRate)}>
                {/* 좁은 화면에는 「전일」 칸이 없으므로 여기서 등락률을 말한다. */}
                <span className="@2xl:hidden">{formatPercent(q.changeRate)}</span>
                <span className="hidden @2xl:inline">{formatPriceChange(q.change, q.currency)}</span>
              </span>
            )
          }
        />
      </button>

      <span className={cn(WL_COL.remove, "flex justify-end")}>
        <button
          type="button"
          onClick={onRemove}
          className="rounded-lg p-1.5 text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-rose-400"
          aria-label={`${q?.name ?? row.symbol} 관심종목에서 삭제`}
        >
          <Trash2 className="h-4 w-4" />
        </button>
      </span>
      </div>
      <TodayLines
        today={today}
        eventDays={eventSoon ? nextEventDays : null}
        onOpen={onOpen}
      />
    </Card>
  );
}

/**
 * 행 아래 두 줄 — **오늘 무슨 일이 있었나** (2026-09-27 사용자 결정 A).
 *
 * 1줄: 실적 D-n · 24시간 뉴스 수 · 오늘의 변화 한 줄(홈 「내 종목」 카드와 같은 재료).
 * 2줄: AI 한 줄 — 저장된 AI 요약 글의 첫 문장. LLM을 부르지 않는다.
 *
 * 둘 다 **값이 있을 때만** 선다. 신호가 없는 종목까지 빈 줄을 그리면 목록이 길어지기만 한다.
 * 한 칸짜리 grid(minmax(0,1fr))로 폭을 가둔다 — Safari에서 버튼 안 flex 자식이 말줄임 없이
 * 카드를 뚫고 나가던 홈 카드의 결함(같은 날 제보 2)을 여기서는 처음부터 피한다.
 */
function TodayLines({
  today,
  eventDays,
  onOpen,
}: {
  today?: MyTodayItem;
  eventDays: number | null;
  onOpen: () => void;
}) {
  const ai = firstSentences(today?.aiOverview, 1);
  const hasSignals = eventDays != null || (today?.news24h ?? 0) > 0 || Boolean(today?.change);
  if (!hasSignals && !ai) return null;
  const change = today?.change;
  return (
    <button
      type="button"
      onClick={onOpen}
      tabIndex={-1}
      className="grid w-full min-w-0 grid-cols-[minmax(0,1fr)] gap-0.5 overflow-hidden text-left text-micro text-[var(--text-muted)] @3xl:pl-9"
    >
      {hasSignals && (
        <span className="flex min-w-0 items-center gap-1.5 overflow-hidden">
          {eventDays != null && (
            <span className="shrink-0 rounded bg-amber-500/15 px-1 text-amber-700 dark:text-amber-400">
              실적 {eventDays === 0 ? "오늘" : `D-${eventDays}`}
            </span>
          )}
          {(today?.news24h ?? 0) > 0 && <span className="shrink-0">뉴스 {today?.news24h}</span>}
          {change && (
            <span
              className={cn(
                "block min-w-0 flex-1 truncate",
                change.direction > 0
                  ? "text-[var(--profit)]"
                  : change.direction < 0
                    ? "text-[var(--loss)]"
                    : undefined,
              )}
              title={change.title}
            >
              {change.title}
            </span>
          )}
        </span>
      )}
      {ai && (
        <span className="flex min-w-0 items-center gap-1.5 overflow-hidden">
          <span className="shrink-0 rounded bg-[var(--brand-muted)] px-1 font-semibold text-[var(--brand)]">AI</span>
          <span
            className="block min-w-0 flex-1 truncate text-[var(--text-secondary)]"
            title={today?.aiAt ? `${ai} (AI 요약 ${today.aiAt.slice(5).replace("-", "/")})` : ai}
          >
            {ai}
          </span>
        </span>
      )}
    </button>
  );
}

function PeriodCell({
  className,
  rate,
  pending,
}: {
  className: string;
  rate: number | null | undefined;
  pending: boolean;
}) {
  const has = rate != null && !Number.isNaN(rate);
  return (
    <WatchlistCell
      className={className}
      value={pending ? "…" : has ? formatPercent(rate) : "—"}
      valueClassName={has && !pending ? trendTextClass(rate) : "text-[var(--text-muted)]"}
    />
  );
}
