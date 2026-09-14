import type { FinancialHistoryPeriod } from "@/lib/financial-history-types";

/**
 * 분기 축 라벨.
 *
 * `full`은 데이터 키와 툴팁이 쓴다 — **반드시 고유**해야 한다. recharts의 카테고리
 * 축은 d3 band scale이라 같은 값이 두 번 오면 한 칸으로 합쳐진다("3월"만 쓰면
 * 24·25·26년 1분기가 서로를 덮어쓴다).
 *
 * `month`/`year`는 눈금에만 쓰는 두 줄 표기다. 예전에는 12분기 전부 "26년 3월"
 * 여섯 글자를 한 줄에 그렸는데, 카드 폭에 다 안 들어가자 recharts가 겹치는 눈금을
 * 스스로 버렸다 — 끝은 살리고 그 앞을 지우는 방식이라 **마지막 직전 라벨(26년 3월)이
 * 통째로 사라진 것처럼 보였다.** 연도를 아랫줄로 내리고 바뀔 때만 적으면 눈금 하나가
 * "12월" 폭으로 줄어 모바일에서도 12개가 다 들어간다.
 */
export type PeriodTick = {
  full: string;
  month: string;
  /** 연도가 바뀌는 분기에만 있다. */
  year?: string;
};

function periodYearMonth(p: FinancialHistoryPeriod): { year: number; month: number } | null {
  if (!p.periodEnd) return null;
  const d = new Date(p.periodEnd);
  if (Number.isNaN(d.getTime())) return null;
  return { year: d.getFullYear(), month: d.getMonth() + 1 };
}

export function buildPeriodTicks(periods: FinancialHistoryPeriod[]): PeriodTick[] {
  let prevYear: number | null = null;
  return periods.map((p) => {
    const ym = periodYearMonth(p);
    if (!ym) {
      // 결산일을 모르는 행 — 회계연도/분기로 떨어뜨린다. 다음 행이 연도를 다시
      // 적도록 prevYear를 비운다.
      prevYear = null;
      const full = `${p.fiscalYear}년 ${p.periodIndex}분기`;
      return { full, month: `${p.periodIndex}분기`, year: `${String(p.fiscalYear).slice(2)}년` };
    }
    const short = `${String(ym.year).slice(2)}년`;
    const tick: PeriodTick = {
      full: `${short} ${ym.month}월`,
      month: `${ym.month}월`,
      year: ym.year === prevYear ? undefined : short,
    };
    prevYear = ym.year;
    return tick;
  });
}

/** 축 라벨(`full`) → 두 줄 표기. 커스텀 tick이 payload.value로 되찾는다. */
export function periodTickIndex(ticks: PeriodTick[]): Record<string, PeriodTick> {
  const out: Record<string, PeriodTick> = {};
  for (const t of ticks) out[t.full] = t;
  return out;
}
