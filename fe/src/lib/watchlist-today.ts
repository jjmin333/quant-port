import type { MyTodayItem } from "@/lib/my-today-api";

/**
 * 관심 종목 화면 맨 위 「오늘 내 관심」 띠 (2026-09-27 사용자 결정 A).
 *
 * 홈의 「내 종목」 카드가 한 줄씩 보여 주는 것(등락 · 점수 변화 · 뉴스 · 실적 임박)을
 * **목록 전체에 대해 한 줄로** 묶는다. 전부 이미 받아 둔 값 위에서 센다 — 요청 0.
 *
 * 칩마다 가장 두드러진 **한 종목**만 이름을 댄다. 여러 개를 나열하면 띠가 표가 된다.
 */

export type WatchlistTodayInput = {
  symbol: string;
  name: string;
  /** 오늘 등락률(%). 시세가 없으면 undefined. */
  changeRate?: number;
  today?: MyTodayItem;
  /** 가장 가까운 예정 일정까지 남은 날(오늘 0). 없으면 null. */
  eventDays: number | null;
};

export type WatchlistTodaySummary = {
  total: number;
  up: number;
  down: number;
  /** 점수 변화 폭이 가장 큰 종목 (|Δ| ≥ 3만). */
  scoreMover?: { name: string; delta: number };
  /** 24시간 기사가 가장 많은 종목 (1건 이상만). */
  newsLeader?: { name: string; count: number };
  /** 가장 가까운 실적·일정 (14일 안만). */
  nextEvent?: { name: string; days: number };
};

/** 점수 칩을 띄울 최소 변화 — 1~2점은 매일 흔들리는 잡음이다. */
export const SCORE_MOVER_MIN = 3;
/** 실적 임박으로 볼 날 수 — 행의 D-n 표시와 같은 기준. */
export const EVENT_SOON_DAYS = 14;

export function summarizeWatchlistToday(rows: WatchlistTodayInput[]): WatchlistTodaySummary {
  const out: WatchlistTodaySummary = { total: rows.length, up: 0, down: 0 };
  for (const r of rows) {
    if (r.changeRate != null && Number.isFinite(r.changeRate)) {
      if (r.changeRate > 0) out.up += 1;
      else if (r.changeRate < 0) out.down += 1;
    }
    const delta = r.today?.scoreDelta;
    if (delta != null && Math.abs(delta) >= SCORE_MOVER_MIN) {
      if (!out.scoreMover || Math.abs(delta) > Math.abs(out.scoreMover.delta)) {
        out.scoreMover = { name: r.name, delta };
      }
    }
    const news = r.today?.news24h ?? 0;
    if (news > 0 && (!out.newsLeader || news > out.newsLeader.count)) {
      out.newsLeader = { name: r.name, count: news };
    }
    if (r.eventDays != null && r.eventDays <= EVENT_SOON_DAYS) {
      if (!out.nextEvent || r.eventDays < out.nextEvent.days) {
        out.nextEvent = { name: r.name, days: r.eventDays };
      }
    }
  }
  return out;
}
