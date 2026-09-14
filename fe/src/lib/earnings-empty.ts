import type { EarningsDaySummary, EarningsMarket } from "@/lib/earnings-types";

/**
 * 「이 달에는 아무 일정도 없다」를 **화면이 먼저 말하게** 한다
 * (2026-09-05 사용자 지적 8).
 *
 * # 무엇이 문제였나
 *
 * 국내 실적은 2·5·8·11월에 몰린다(코드 곳곳의 실측 주석이 같은 말을 한다). 그래서
 * 9월에 이 화면을 열면 **한 건도 없는 달**을 보게 되는데, 화면은 그 사실을 어디에서도
 * 말하지 않았다: 위 스코어보드는 「0건」과 「—」 다섯 칸으로 서 있고, 달력은 31칸이
 * 전부 빈 채로 그려지고, 그 아래 목록만 작은 글씨로 「이 기간에 실적 발표가 없습니다」
 * 라고 적었다. 사용자 입장에서는 **데이터가 없는 것인지 화면이 고장 난 것인지**
 * 구분할 방법이 없다.
 *
 * # 왜 함수로 빼는가
 *
 * 판정과 문구를 화면에서 인라인으로 쓰면, 「비었다」의 기준이 스코어보드를 감출 때와
 * 달력에 문구를 낼 때 서로 달라질 수 있다. 둘은 반드시 같은 답이어야 한다.
 */

/** 그날의 발표 수 — 확정·예정·추정을 합친 값. 달력 칸이 「있다/없다」를 재는 단위다. */
export function earningsDayTotal(day: EarningsDaySummary): number {
  return day.confirmed + day.scheduled + day.estimated;
}

/**
 * 이 구간에 발표가 **하나도** 없는가.
 *
 * `null`(아직 안 받았다)은 **비었다고 하지 않는다.** 로딩 중에 「일정이 없습니다」를
 * 띄우면, 잠깐 뒤 달력이 채워지면서 화면이 자기 말을 뒤집는다.
 */
export function isEarningsRangeEmpty(days: EarningsDaySummary[] | null): boolean {
  if (days === null) return false;
  return days.every((d) => earningsDayTotal(d) === 0);
}

/**
 * 빈 달에 띄울 한 줄. 「2026. 09」는 바로 위 달 이동 줄이 이미 적고 있으므로
 * 여기서는 **달만** 말한다 — 같은 화면에서 연도를 두 번 읽게 하지 않는다.
 */
export function emptyMonthMessage(month: string, market: EarningsMarket): string {
  const mon = Number(month.slice(5, 7));
  const where = market === "us" ? "해외" : "국내";
  return `${mon}월에는 ${where} 기업 실적 발표 일정이 없습니다.`;
}

/**
 * 그 아래 작은 줄 — **왜** 비었는지.
 *
 * 국내는 분기 보고서 마감이 밀집한 2·5·8·11월에 발표가 몰린다(실측: 8월 국내 190건이
 * 3~14일에 전부). 그 사실을 적어 두면 빈 달이 고장이 아니라 **달력의 모양**이라는 것이
 * 읽힌다. 해외는 그런 쏠림이 없어 할 말이 없으므로 빈 문자열이다.
 */
export function emptyMonthHint(market: EarningsMarket): string {
  return market === "us"
    ? ""
    : "국내 기업 실적은 2 · 5 · 8 · 11월에 몰립니다. 위 화살표로 달을 옮겨 보세요.";
}
