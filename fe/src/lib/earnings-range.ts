import type { EarningsRangeMode } from "@/lib/earnings-types";

/**
 * 실적 화면의 날짜 산술.
 *
 * **`Date`의 로컬 시간대로 계산하지 않는다.** 브라우저가 UTC−5에 있으면
 * `new Date("2026-08-01")`이 7월 31일로 읽혀 격자가 한 칸 밀린다. 날짜는 문자열
 * 로 다루고, 요일·월말만 `Date.UTC`로 구한다.
 */

/** KST 자정 기준 오늘. 사용자 시간대와 무관하게 **시장 날짜**를 쓴다. */
export function todayKST(): string {
  return new Date().toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
}

export function monthStartOf(date: string): string {
  return `${date.slice(0, 7)}-01`;
}

export function currentMonthStart(): string {
  return monthStartOf(todayKST());
}

export function monthEnd(monthStart: string): string {
  const [year, month] = monthStart.split("-").map(Number);
  const last = new Date(Date.UTC(year, month, 0)).getUTCDate();
  return ymd(year, month, last);
}

export function shiftMonth(monthStart: string, delta: number): string {
  const [year, month] = monthStart.split("-").map(Number);
  const d = new Date(Date.UTC(year, month - 1 + delta, 1));
  return ymd(d.getUTCFullYear(), d.getUTCMonth() + 1, 1);
}

export function shiftDay(date: string, delta: number): string {
  const [year, month, day] = date.split("-").map(Number);
  const d = new Date(Date.UTC(year, month - 1, day + delta));
  return ymd(d.getUTCFullYear(), d.getUTCMonth() + 1, d.getUTCDate());
}

/** 그 날짜가 속한 주의 **월요일**. */
export function weekStartOf(date: string): string {
  const [year, month, day] = date.split("-").map(Number);
  const weekday = new Date(Date.UTC(year, month - 1, day)).getUTCDay();
  // 일요일(0)은 그 주의 마지막이 아니라 **직전 주 월요일**에 붙인다 — 한국·미국
  // 증시 주간은 월요일에 시작한다.
  const offset = weekday === 0 ? -6 : 1 - weekday;
  return shiftDay(date, offset);
}

/**
 * 주간 보드에 그리는 날짜들 — **월~금**.
 *
 * 주말을 넣지 않는 이유는 실적 발표가 거래일에만 있어서다(실측: 8월 국내 190건이
 * 전부 평일). 토·일 칸을 두면 격자의 2/7이 영원히 빈다.
 */
export function weekdaysOf(weekStart: string): string[] {
  return [0, 1, 2, 3, 4].map((i) => shiftDay(weekStart, i));
}

export function weekEndOf(weekStart: string): string {
  return shiftDay(weekStart, 4);
}

/** 보기 단위에 맞는 조회 구간. 서버 상한 31일 안에 항상 들어온다. */
export function rangeFor(
  mode: EarningsRangeMode,
  anchor: { month: string; week: string; day: string },
): { from: string; to: string } {
  switch (mode) {
    case "day":
      return { from: anchor.day, to: anchor.day };
    case "week":
      return { from: anchor.week, to: weekEndOf(anchor.week) };
    default:
      return { from: anchor.month, to: monthEnd(anchor.month) };
  }
}

const WEEKDAY = ["일", "월", "화", "수", "목", "금", "토"];

export function weekdayOf(date: string): string {
  const [year, month, day] = date.split("-").map(Number);
  return WEEKDAY[new Date(Date.UTC(year, month - 1, day)).getUTCDay()];
}

/** "8월 6일 (목)" */
export function formatDayLabel(date: string): string {
  const [, month, day] = date.split("-").map(Number);
  return `${month}월 ${day}일 (${weekdayOf(date)})`;
}

/** "8/6" — 좁은 칸용. */
export function formatShortDate(date: string): string {
  const [, month, day] = date.split("-").map(Number);
  return `${month}/${day}`;
}

function ymd(year: number, month: number, day: number): string {
  return `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}
