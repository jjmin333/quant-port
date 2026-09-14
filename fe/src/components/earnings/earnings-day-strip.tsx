"use client";

import { useEffect, useMemo, useRef } from "react";
import { CalendarOff, CalendarRange, ChevronLeft, ChevronRight } from "lucide-react";
import { beatRate } from "@/components/earnings/earnings-month-grid";
import { emptyMonthHint, emptyMonthMessage, isEarningsRangeEmpty } from "@/lib/earnings-empty";
import type { EarningsDaySummary, EarningsMarket } from "@/lib/earnings-types";
import { shiftMonth, todayKST } from "@/lib/earnings-range";
import { cn } from "@/lib/utils";

/**
 * 달력을 **가로로 눕힌 것** (2026-09-06 사용자 선택).
 *
 * # 왜
 *
 * 375×812 실측에서 상단이 스코어보드 228px + 월 격자 345px = **573px**이었다.
 * 뷰포트의 70%를 쓰고 나서야 첫 회사가 나온다. 사용자는 스코어보드와 아래 상세
 * 표를 「유용한 데이터」라 유지하기로 했으므로, 줄일 수 있는 것은 격자뿐이다.
 *
 * 이 띠는 격자와 **같은 것을 말한다** — 건수, 상회 비율, 예정·추정 점, 오늘 표시,
 * 배경 농도. 격자가 6줄로 하던 것을 한 줄로 할 뿐이다. 정보를 버리지 않았으므로
 * 「달이 어떻게 생겼나」는 그대로 읽힌다: 짙은 칸이 몰린 구간이 성수기다.
 *
 * 달 전체를 한눈에 보고 싶을 때는 「달력」 버튼이 예전 격자를 그대로 편다 — 시즌
 * 판단은 여전히 격자가 낫다.
 *
 * # 오늘로 스크롤한다
 *
 * 30칸을 가로로 늘어놓으면 좁은 화면에는 8~9칸만 보인다. 그 창이 1일에 멈춰 있으면
 * 「오늘 누가 발표하나」를 보려고 매번 손으로 밀어야 한다. 고른 날짜가 바뀔 때마다
 * 그 칸을 창 가운데로 옮긴다.
 */

const WEEKDAYS = ["일", "월", "화", "수", "목", "금", "토"];

export function EarningsDayStrip({
  month,
  days,
  selected,
  loading,
  market,
  onSelect,
  onMonthChange,
  onExpand,
}: {
  month: string;
  days: EarningsDaySummary[] | null;
  selected: string | null;
  loading: boolean;
  market: EarningsMarket;
  onSelect: (date: string | null) => void;
  onMonthChange: (monthStart: string) => void;
  /** 「달력」 — 예전 월 격자를 편다. */
  onExpand: () => void;
}) {
  const byDate = useMemo(() => new Map((days ?? []).map((d) => [d.date, d])), [days]);
  const cells = useMemo(() => buildDays(month), [month]);
  const max = useMemo(
    () => Math.max(1, ...(days ?? []).map((d) => d.confirmed + d.scheduled + d.estimated)),
    [days],
  );

  const [year, mon] = month.split("-").map(Number);
  const todayKey = todayKST();
  const empty = isEarningsRangeEmpty(days);

  const scrollerRef = useRef<HTMLDivElement | null>(null);
  const activeRef = useRef<HTMLButtonElement | null>(null);
  /*
    고른 칸을 창 가운데로. `scrollIntoView({block:"nearest"})`는 **세로도 건드려**
    페이지 전체가 튀므로, 가로 스크롤 위치를 직접 계산한다.
  */
  useEffect(() => {
    const box = scrollerRef.current;
    const cell = activeRef.current;
    if (!box || !cell) return;
    const target = cell.offsetLeft - box.clientWidth / 2 + cell.clientWidth / 2;
    box.scrollTo({ left: Math.max(0, target), behavior: "smooth" });
  }, [selected, month]);

  return (
    <div className="rounded-xl border bg-card p-3">
      <div className="mb-2 flex items-center justify-between gap-2">
        <button
          type="button"
          aria-label="이전 달"
          onClick={() => onMonthChange(shiftMonth(month, -1))}
          className="rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <ChevronLeft className="size-4" />
        </button>
        <div className="flex min-w-0 items-center gap-2">
          <span className="text-sm font-semibold tabular-nums">
            {year}. {String(mon).padStart(2, "0")}
          </span>
          {selected && (
            <button
              type="button"
              onClick={() => onSelect(null)}
              className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-micro text-muted-foreground hover:text-foreground"
            >
              이 달 전체 보기
            </button>
          )}
          <button
            type="button"
            onClick={onExpand}
            className="flex shrink-0 items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-micro text-muted-foreground hover:text-foreground"
          >
            <CalendarRange className="size-3" aria-hidden />
            달력
          </button>
        </div>
        <button
          type="button"
          aria-label="다음 달"
          onClick={() => onMonthChange(shiftMonth(month, 1))}
          className="rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <ChevronRight className="size-4" />
        </button>
      </div>

      {empty ? (
        <div className="flex min-h-[88px] flex-col items-center justify-center gap-1.5 px-4 text-center">
          <CalendarOff className="size-5 text-muted-foreground/60" aria-hidden />
          <p className="text-sm font-semibold text-foreground">
            {emptyMonthMessage(month, market)}
          </p>
          {emptyMonthHint(market) && (
            <p className="max-w-[22rem] text-cell text-muted-foreground">
              {emptyMonthHint(market)}
            </p>
          )}
        </div>
      ) : (
        <>
          <div
            ref={scrollerRef}
            className="-mx-1 flex snap-x gap-1 overflow-x-auto px-1 pb-1 [scrollbar-width:thin]"
          >
            {cells.map((date) => {
              const cell = byDate.get(date);
              const total = cell ? cell.confirmed + cell.scheduled + cell.estimated : 0;
              const isSelected = selected === date;
              const isToday = date === todayKey;
              const dow = new Date(`${date}T00:00:00Z`).getUTCDay();
              return (
                <button
                  key={date}
                  ref={isSelected || (!selected && isToday) ? activeRef : undefined}
                  type="button"
                  onClick={() => onSelect(isSelected ? null : date)}
                  title={cellTitle(date, cell)}
                  style={cellStyle(cell, total, max)}
                  className={cn(
                    "flex h-[52px] w-[42px] shrink-0 snap-start flex-col items-center justify-center gap-px rounded-md border text-micro transition-colors",
                    total > 0 ? "border-transparent" : "border-transparent bg-muted/30",
                    isSelected && "ring-2 ring-foreground",
                    isToday && !isSelected && "ring-1 ring-muted-foreground/50",
                    loading && "animate-pulse",
                  )}
                >
                  <span
                    className={cn(
                      "leading-none",
                      dow === 0 ? "text-[var(--loss)]/70" : "text-muted-foreground",
                    )}
                  >
                    {WEEKDAYS[dow]}
                  </span>
                  <span className={cn("tabular-nums leading-none", isToday && "font-bold")}>
                    {Number(date.slice(8))}
                  </span>
                  {total > 0 && (
                    <span className="font-num text-micro font-bold leading-none tabular-nums">
                      {total}
                    </span>
                  )}
                  {cell && cell.compared > 0 && (
                    <span
                      className={cn(
                        "text-micro font-semibold leading-none",
                        beatRate(cell) >= 50 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                      )}
                    >
                      {Math.round(beatRate(cell))}%
                    </span>
                  )}
                  {total > 0 && cell && cell.compared === 0 && (
                    <span className="flex items-center gap-[2px]">
                      {cell.scheduled > 0 && <Dot kind="scheduled" />}
                      {cell.estimated > 0 && <Dot kind="estimated" />}
                    </span>
                  )}
                </button>
              );
            })}
          </div>

          {/* 범례는 격자와 같은 것을 같은 말로. 여기서만 줄이면 두 보기가 다른 화면이 된다. */}
          <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 border-t pt-2 text-micro text-muted-foreground">
            <span className="flex items-center gap-1">
              <span
                className="size-[9px] rounded-sm"
                style={{ backgroundColor: "color-mix(in srgb, var(--profit) 32%, transparent)" }}
              />
              상회 많음
            </span>
            <span className="flex items-center gap-1">
              <span
                className="size-[9px] rounded-sm"
                style={{ backgroundColor: "color-mix(in srgb, var(--loss) 32%, transparent)" }}
              />
              하회 많음
            </span>
            <span className="flex items-center gap-1">
              <span className="size-[9px] rounded-sm bg-foreground/15" />
              발표 전 (건수만)
            </span>
            <span className="flex items-center gap-1">
              <Dot kind="scheduled" />
              예정
            </span>
            <span className="flex items-center gap-1">
              <Dot kind="estimated" />
              추정
            </span>
          </div>
        </>
      )}
    </div>
  );
}

function Dot({ kind }: { kind: "scheduled" | "estimated" }) {
  return (
    <span
      className={cn(
        "size-[4px] rounded-full",
        kind === "scheduled" ? "bg-muted-foreground" : "bg-[var(--sample)]",
      )}
    />
  );
}

/** 월 격자와 **같은 규칙**으로 칠한다 — 두 보기의 색이 다르면 같은 달이 다르게 읽힌다. */
function cellStyle(
  cell: EarningsDaySummary | undefined,
  total: number,
  max: number,
): React.CSSProperties | undefined {
  if (total === 0) return undefined;
  if (cell && cell.compared > 0) {
    const rate = beatRate(cell);
    const token = rate >= 50 ? "--profit" : "--loss";
    const strength = Math.min(38, 8 + Math.abs(rate - 50) * 0.7);
    return { backgroundColor: `color-mix(in srgb, var(${token}) ${strength}%, transparent)` };
  }
  const density = Math.round(6 + (total / max) * 20);
  return { backgroundColor: `color-mix(in srgb, var(--foreground) ${density}%, transparent)` };
}

function cellTitle(date: string, cell?: EarningsDaySummary): string {
  if (!cell) return `${date} · 일정 없음`;
  const parts: string[] = [];
  if (cell.confirmed) parts.push(`발표됨 ${cell.confirmed}`);
  if (cell.scheduled) parts.push(`예정 ${cell.scheduled}`);
  if (cell.estimated) parts.push(`추정 ${cell.estimated}`);
  if (cell.compared > 0) {
    parts.push(`컨센서스 상회 ${cell.beats}/${cell.compared} (${Math.round(beatRate(cell))}%)`);
  }
  return `${date} · ${parts.join(" · ")}`;
}

/**
 * 그 달의 날짜들. 격자와 달리 **앞뒤 빈 칸이 없다** — 가로 줄에서 빈 칸은 자리만
 * 먹고 아무 말도 하지 않는다. 요일은 칸마다 자기 글자로 말한다.
 */
function buildDays(monthStart: string): string[] {
  const [year, month] = monthStart.split("-").map(Number);
  const daysInMonth = new Date(Date.UTC(year, month, 0)).getUTCDate();
  const out: string[] = [];
  for (let day = 1; day <= daysInMonth; day++) {
    out.push(`${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`);
  }
  return out;
}
