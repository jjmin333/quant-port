/*
실시간 차트 목록 머리에 찍히는 **시각**이 무엇을 가리키는가.

# 예전에 5분마다 멈춰 있던 이유

이 자리는 `refreshedAt ?? rankedAt`, 즉 **순위 스냅샷을 서버가 갱신한 시각**을 찍고
있었다. 순위 스냅샷은 5분 주기라 초록 점 옆의 시계가 5분에 한 번씩 튀었다. 그런데
같은 목록의 **가격은 1초마다** 갱신되고 있었으므로(`MARKET_RADAR_PRICE_POLL_MS`),
숫자는 계속 움직이는데 「방금 갱신됨」이라는 표시는 몇 분 전에 멈춰 있는 화면이었다.

# 마감 중에는 「장이 닫힌 시각」이다 (2026-09-04 사용자 지적)

그 다음 판이 마감 세션이었다. 장이 닫혀 있으면 가격을 받아 온 것이 없으므로 시계가
**스냅샷 시각**으로 되돌아갔는데, 그 값은 서버 폴러가 마지막으로 돈 시각이라 새벽
0시 40분에 열어도 0시 40분 언저리를 찍었다. 「지금 만들어진 값」으로 읽힌다.

지금은 서버가 응답에 **그 시장이 닫힌 시각**(`sessionClosedAt`)을 실어 준다. 국내는
시간외까지 끝나는 20:00이고, 그것이 이 목록의 숫자가 실제로 멈춘 시각이다. 하루가
넘어간 뒤에 보는 값이라 **날짜도 함께** 찍는다 — 시각만 있으면 「오후 08:00:00」이
오늘 저녁인지 어제 저녁인지 알 수 없다.

# 지금 규칙

	장중        → 마지막으로 가격을 받아 온 시각        (basis "prices", 초록 점)
	마감        → 그 시장이 닫힌 시각 + 날짜            (basis "closed")
	둘 다 없음  → 순위 스냅샷 시각                      (basis "snapshot")

마지막 갈래는 달력을 못 구했을 때의 자리다 — 그때 「모른다」를 감추기보다 예전처럼
스냅샷 나이를 말하는 편이 낫다.
*/

import { formatSessionStamp } from "@/lib/market-session";

export type RankingClockBasis = "prices" | "closed" | "snapshot";

export type RankingClockInput = {
  /** 마지막으로 `/rankings/prices`가 값을 준 시각(ISO). 장이 닫혀 있으면 null. */
  pricesAt: string | null;
  /**
   * 그 시장이 닫힌 시각(ISO). 서버가 거래소 달력에서 계산해 `/rankings` 응답에
   * 실어 준다 — 열려 있는 동안에는 오지 않는다.
   */
  sessionClosedAt?: string | null;
  /** 서버 폴러가 순위 스냅샷을 갱신한 시각. */
  refreshedAt?: string | null;
  /** 스냅샷이 매겨진 시각(상류 기준). `refreshedAt`이 없을 때의 대체재. */
  rankedAt?: string | null;
};

export type RankingClock = {
  at: string;
  basis: RankingClockBasis;
} | null;

export function rankingClock({
  pricesAt,
  sessionClosedAt,
  refreshedAt,
  rankedAt,
}: RankingClockInput): RankingClock {
  /*
    **마감이 가장 세다.** 시장을 바꾼 직후에는 앞 시장에서 받아 둔 `pricesAt`이
    아직 남아 있을 수 있는데, 닫힌 목록 위에 그 시각을 찍으면 「1초 전 값」이라고
    거짓말을 한다.
  */
  const closed = sessionClosedAt?.trim();
  if (closed) return { at: closed, basis: "closed" };

  const live = pricesAt?.trim();
  if (live) return { at: live, basis: "prices" };

  const snapshot = refreshedAt?.trim() || rankedAt?.trim();
  if (snapshot) return { at: snapshot, basis: "snapshot" };

  return null;
}

/**
 * 초까지 찍는다 — 1초마다 갱신되는 값이라 분 단위로는 멈춰 보인다.
 *
 * 마감 시각에는 **날짜를 앞에 붙인다**(`09-03 오후 08:00:00`). 자정을 넘겨서 보는
 * 값이라 시각만으로는 어느 날 장인지 알 수 없다.
 */
export function formatRankingClock(at: string, basis: RankingClockBasis = "prices"): string {
  // 종목 상세 헤더의 각주와 **같은 함수**로 찍는다 — 같은 순간을 두 화면이 다른
  // 모양으로 적으면 어느 쪽이 진짜인지 헷갈린다(`lib/market-session`).
  return formatSessionStamp(at, basis === "closed");
}

/** 시계 옆 툴팁이 「이 시각이 무엇의 시각인가」를 말한다. */
export function rankingClockHint(basis: RankingClockBasis, refreshHint: string): string {
  if (basis === "prices") return "실시간 시세 갱신";
  if (basis === "closed") return "장 마감 — 다음 장이 열릴 때까지 값이 바뀌지 않습니다";
  return `${refreshHint} 순위 갱신`;
}
