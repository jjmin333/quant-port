"use client";

import { useMemo } from "react";
import { CalendarOff, ChevronLeft, ChevronRight } from "lucide-react";
import {
  emptyMonthHint,
  emptyMonthMessage,
  isEarningsRangeEmpty,
} from "@/lib/earnings-empty";
import type { EarningsDaySummary, EarningsMarket } from "@/lib/earnings-types";
// 날짜 산술은 `lib/earnings-range`에 한 벌만 둔다 — 주간 보드와 「일」 보기가 같은
// 규칙을 써야 하고, `Date`의 로컬 시간대로 계산하면 격자가 한 칸 밀린다.
import { shiftMonth, todayKST } from "@/lib/earnings-range";
import { cn } from "@/lib/utils";

/**
 * 월 그리드 히트맵 — 「이번 달 어느 날에 몇 건이 몰리나」.
 *
 * 이 칸이 받는 것은 **날짜별 건수뿐**이다(31행). 상세 행으로 세지 않는 이유는
 * 시즌 달의 상세가 2,000행이어서, 격자 하나를 그리려고 그만큼을 내리는 셈이 되기
 * 때문이다(PRINCIPLES 응답 속도 원칙 #4).
 *
 * 확정·예정·추정을 **한 숫자로 합치지 않는다.** 합치면 「9월 3일 42건」이 실제
 * 공시된 일정인지 우리가 작년으로 역산한 추측인지 알 수 없어진다. 칸 안에서
 * 색이 다른 점으로 구분하고, 합계는 툴팁이 말한다.
 */

const WEEKDAYS = ["일", "월", "화", "수", "목", "금", "토"];

export function EarningsMonthGrid({
  month,
  days,
  selected,
  loading,
  market,
  onSelect,
  onMonthChange,
}: {
  /** 이 달의 1일 (YYYY-MM-DD). */
  month: string;
  days: EarningsDaySummary[] | null;
  /** 선택된 날짜 (YYYY-MM-DD) 또는 null = 이 달 전체. */
  selected: string | null;
  loading: boolean;
  /** 빈 달 문구가 「국내」인지 「해외」인지 — `lib/earnings-empty`. */
  market: EarningsMarket;
  onSelect: (date: string | null) => void;
  onMonthChange: (monthStart: string) => void;
}) {
  const byDate = useMemo(
    () => new Map((days ?? []).map((d) => [d.date, d])),
    [days],
  );

  const cells = useMemo(() => buildCells(month), [month]);
  const max = useMemo(
    () => Math.max(1, ...(days ?? []).map((d) => d.confirmed + d.scheduled + d.estimated)),
    [days],
  );

  const [year, mon] = month.split("-").map(Number);
  const todayKey = todayKST();

  /*
    # 한 건도 없는 달은 **격자를 그리지 않는다** (2026-09-05 사용자 지적 8)

    빈 31칸은 「데이터가 없다」가 아니라 「고장 났다」로 읽힌다 — 실제로 국내 9월이
    그랬다. 달 이동 줄은 **그대로 남긴다**: 격자까지 통째로 감추면 다른 달로 갈 길이
    함께 사라져 사용자가 갇힌다.
  */
  const empty = isEarningsRangeEmpty(days);
  const hint = emptyMonthHint(market);

  return (
    <div className="rounded-xl border bg-card p-3">
      <div className="mb-2 flex items-center justify-between">
        <button
          type="button"
          aria-label="이전 달"
          onClick={() => onMonthChange(shiftMonth(month, -1))}
          className="rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <ChevronLeft className="size-4" />
        </button>
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold tabular-nums">
            {year}. {String(mon).padStart(2, "0")}
          </span>
          {/* 날짜를 골라 좁혀 본 뒤 되돌아올 길. 없으면 달 전체를 다시 보려고
              앞뒤로 두 번 눌러야 한다. */}
          {selected && (
            <button
              type="button"
              onClick={() => onSelect(null)}
              className="rounded-full bg-muted px-2 py-0.5 text-micro text-muted-foreground hover:text-foreground"
            >
              이 달 전체 보기
            </button>
          )}
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
        <div className="flex min-h-[176px] flex-col items-center justify-center gap-2 px-4 text-center">
          <CalendarOff className="size-6 text-muted-foreground/60" aria-hidden />
          <p className="text-sm font-semibold text-foreground">
            {emptyMonthMessage(month, market)}
          </p>
          {hint && <p className="max-w-[22rem] text-cell text-muted-foreground">{hint}</p>}
        </div>
      ) : (
        <>
      <div className="mb-1 grid grid-cols-7 gap-1">
        {WEEKDAYS.map((w, i) => (
          <div
            key={w}
            className={cn(
              "text-center text-micro font-medium",
              i === 0 ? "text-[var(--loss)]/70" : "text-muted-foreground",
            )}
          >
            {w}
          </div>
        ))}
      </div>

      <div className="grid grid-cols-7 gap-1">
        {cells.map((date, i) => {
          if (!date) return <div key={`pad-${i}`} className="h-11 rounded-md" />;
          const cell = byDate.get(date);
          const total = cell ? cell.confirmed + cell.scheduled + cell.estimated : 0;
          const isSelected = selected === date;
          const isToday = date === todayKey;
          return (
            <button
              key={date}
              type="button"
              onClick={() => onSelect(isSelected ? null : date)}
              title={cellTitle(date, cell)}
              className={cn(
                "flex h-11 flex-col items-center justify-center gap-0.5 rounded-md border text-micro transition-colors",
                // 건수를 배경 농도로 말한다 — 숫자만으로는 "몰리는 주"가 안 보인다.
                total > 0 ? "border-transparent" : "border-transparent bg-muted/30",
                isSelected && "ring-2 ring-foreground",
                isToday && !isSelected && "ring-1 ring-muted-foreground/50",
                loading && "animate-pulse",
              )}
              style={cellStyle(cell, total, max)}
            >
              <span className={cn("tabular-nums", isToday && "font-bold")}>
                {Number(date.slice(8))}
              </span>
              {total > 0 && (
                <span className="font-num text-micro font-bold tabular-nums">{total}</span>
              )}
              {/*
                발표된 날은 **상회 비율**을 적는다 — 달력을 훑는 사람이 찾는 것은
                「많은 날」이 아니라 「좋았던 날」이다. 아직 발표 전인 날은 비교할
                게 없어 건수만 남는다.
              */}
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
              {total > 0 && (!cell || cell.compared === 0) && (
                <span className="flex items-center gap-[2px]">
                  {cell!.scheduled > 0 && <Dot kind="scheduled" />}
                  {cell!.estimated > 0 && <Dot kind="estimated" />}
                </span>
              )}
            </button>
          );
        })}
      </div>

      <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 border-t pt-2 text-micro text-muted-foreground">
        <span className="flex items-center gap-1">
          <span className="size-[9px] rounded-sm" style={{ backgroundColor: "color-mix(in srgb, var(--profit) 32%, transparent)" }} />
          상회 많음
        </span>
        <span className="flex items-center gap-1">
          <span className="size-[9px] rounded-sm" style={{ backgroundColor: "color-mix(in srgb, var(--loss) 32%, transparent)" }} />
          하회 많음
        </span>
        <span className="flex items-center gap-1">
          <span className="size-[9px] rounded-sm bg-foreground/15" />
          발표 전 (건수만)
        </span>
        <LegendItem kind="scheduled" label="예정" />
        <LegendItem kind="estimated" label="추정" />
      </div>
        </>
      )}
    </div>
  );
}

function Dot({ kind }: { kind: "confirmed" | "scheduled" | "estimated" }) {
  return <span className={cn("size-[4px] rounded-full", DOT_CLASS[kind])} />;
}

const DOT_CLASS = {
  confirmed: "bg-foreground",
  scheduled: "bg-muted-foreground",
  estimated: "bg-[var(--sample)]",
} as const;

function LegendItem({
  kind,
  label,
}: {
  kind: "confirmed" | "scheduled" | "estimated";
  label: string;
}) {
  return (
    <span className="flex items-center gap-1">
      <span className={cn("size-[5px] rounded-full", DOT_CLASS[kind])} />
      {label}
    </span>
  );
}

export function beatRate(cell: EarningsDaySummary): number {
  return cell.compared === 0 ? 0 : (cell.beats / cell.compared) * 100;
}

/**
 * 칸 배경 — **색이 실제 정보를 나른다** (사용자 선택 C2).
 *
 * 예전에는 중립 회색 농도로 건수만 말해서 격자가 밋밋했고, 「많은 날」과
 * 「좋았던 날」이 구별되지 않았다. 이제 발표된 날은 **상회 비율**로 칠한다 —
 * 빨강이 짙을수록 그날 컨센서스를 넘긴 발표가 많았다는 뜻이고, 이 앱의
 * 빨강=상승 규칙과 방향이 같다.
 *
 * 아직 발표 전인 날(예정·추정만 있는 날)은 비교할 게 없다. 그때까지 색을 칠하면
 * 없는 성과를 말하는 셈이라 **중립 농도로 건수만** 표시한다.
 */
function cellStyle(
  cell: EarningsDaySummary | undefined,
  total: number,
  max: number,
): React.CSSProperties | undefined {
  if (total === 0) return undefined;
  if (cell && cell.compared > 0) {
    const rate = beatRate(cell);
    const token = rate >= 50 ? "--profit" : "--loss";
    // 50%에서 멀수록 진하게 — 55%와 95%가 같은 색이면 색을 쓰는 의미가 없다.
    const strength = Math.min(38, 8 + Math.abs(rate - 50) * 0.7);
    return { backgroundColor: `color-mix(in srgb, var(${token}) ${strength}%, transparent)` };
  }
  // 발표 전 — 중립 농도로 건수만.
  const density = Math.round(6 + (total / max) * 20);
  return {
    backgroundColor: `color-mix(in srgb, var(--foreground) ${density}%, transparent)`,
  };
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
 * 달의 격자 칸. 앞쪽은 1일의 요일만큼 비운다.
 *
 * `Date.UTC`로 계산한다 — 브라우저가 UTC−5에 있으면 `new Date("2026-08-01")`이
 * 7월 31일로 읽혀 격자가 한 칸 밀린다.
 */
function buildCells(monthStart: string): (string | null)[] {
  const [year, month] = monthStart.split("-").map(Number);
  const firstWeekday = new Date(Date.UTC(year, month - 1, 1)).getUTCDay();
  const daysInMonth = new Date(Date.UTC(year, month, 0)).getUTCDate();

  const cells: (string | null)[] = Array(firstWeekday).fill(null);
  for (let day = 1; day <= daysInMonth; day++) {
    cells.push(`${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`);
  }
  while (cells.length % 7 !== 0) cells.push(null);
  return cells;
}

