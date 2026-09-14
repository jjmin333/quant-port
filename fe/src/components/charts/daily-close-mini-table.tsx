"use client";

import { useState } from "react";
import { ChevronDown } from "lucide-react";
import { IndexStanceBadge } from "@/components/market/index-tag-chips";
import type { DailyCloseRow } from "@/lib/daily-close-types";
import { formatPercent } from "@/lib/format";
import { SCORE_TONE_TEXT, scoreTone } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 퀵룩 모달의 **「최근 일별」 표** — 지수와 종목이 함께 쓴다.
 *
 * # 왜 표에 점수를 세우는가
 *
 * 「추세 점수 63」이 좋은 건지는 어제 61, 그제 57을 봐야 알 수 있다. 그 값은 일별 종가
 * 응답에 **이미 실려 온다**(추가 요청 0). 지금까지 화면은 오늘 점수 하나만 보여 주고
 * 비교 대상은 주지 않았다.
 *
 * # 해설이 있는 날만 눌린다
 *
 * 눌러도 아무 일이 없는 줄을 눌리게 두면 어느 줄에 볼 것이 있는지 알 수 없다.
 *
 * # 왜 공용인가 (2026-09-05 사용자 요청 7-2)
 *
 * 지수 모달에만 있던 표를 종목 모달도 갖게 됐다. 다른 것은 **해설이 어느 칸에서
 * 오느냐**뿐이다 — 지수는 `llmNote`(규칙 문장과 따로 온다), 종목은 `aiSummary`.
 * 그래서 그 한 가지만 `noteOf`로 받고 나머지는 한 벌로 둔다.
 */
export type DailyCloseNote = { text: string; stance?: string };

export function DailyCloseMiniTable({
  rows,
  limit = 6,
  formatValue,
  noteOf,
}: {
  /** **최신순**으로 온 그대로 넘긴다 — 표는 오늘이 맨 위다. */
  rows: DailyCloseRow[];
  limit?: number;
  /** 종가 표기. 지수는 포인트, 종목은 통화. */
  formatValue: (value: number) => string;
  /** 그날의 해설. 없으면 그 줄은 버튼이 아니다. */
  noteOf: (row: DailyCloseRow) => DailyCloseNote | null;
}) {
  return (
    <>
      <div className="flex h-[22px] items-center gap-2 border-b px-1 text-micro text-muted-foreground">
        <span className="flex-1">날짜</span>
        <span className="w-[76px] text-right">종가</span>
        <span className="w-[56px] text-right">전일비</span>
        <span className="w-[34px] text-right">점수</span>
      </div>
      <ul className="min-w-0">
        {rows.slice(0, limit).map((row) => (
          <DailyCloseMiniRow
            key={row.tradeDate}
            row={row}
            formatValue={formatValue}
            note={noteOf(row)}
          />
        ))}
      </ul>
    </>
  );
}

function DailyCloseMiniRow({
  row,
  formatValue,
  note,
}: {
  row: DailyCloseRow;
  formatValue: (value: number) => string;
  note: DailyCloseNote | null;
}) {
  const [open, setOpen] = useState(false);
  const text = note?.text.trim() ?? "";

  const cells = (
    <>
      <span className="min-w-0 flex-1 truncate text-left">
        {row.tradeDate.slice(5)}{" "}
        <span className="text-micro text-muted-foreground">({row.weekday})</span>
      </span>
      <span className="w-[76px] text-right font-num tabular-nums">{formatValue(row.close)}</span>
      <span
        className={cn(
          "w-[56px] text-right font-num tabular-nums",
          row.changeRate == null ? "text-muted-foreground" : trendTextClass(row.changeRate),
        )}
      >
        {row.changeRate == null ? "—" : formatPercent(row.changeRate)}
      </span>
      {/* 점수가 없는 날은 "—". 0으로 적으면 그날이 바닥이었다는 없는 사실이 된다. */}
      <span
        className={cn(
          "w-[34px] text-right font-num tabular-nums",
          row.score == null ? "text-muted-foreground" : SCORE_TONE_TEXT[scoreTone(row.score)],
        )}
      >
        {row.score ?? "—"}
      </span>
    </>
  );

  if (!text) {
    return <li className="flex h-[26px] items-center gap-2 px-1 text-cell">{cells}</li>;
  }

  return (
    <li className="text-cell">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex h-[26px] w-full items-center gap-2 rounded px-1 text-left hover:bg-accent"
      >
        {cells}
        <ChevronDown
          className={cn(
            "size-3 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-180",
          )}
          aria-hidden
        />
      </button>
      {open && (
        <div className="px-1 pb-2">
          <IndexStanceBadge stance={note?.stance} />
          <p className="mt-1 whitespace-pre-line rounded-lg bg-muted px-2.5 py-2 text-read">
            {text}
          </p>
        </div>
      )}
    </li>
  );
}
