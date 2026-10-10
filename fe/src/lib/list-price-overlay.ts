import { changeRateFrom, type ListPriceMap } from "@/lib/quote-stream-state";

/**
 * 목록 행에 스트림 가격을 얹는 **판정** — 적용은 호출자가 한다.
 *
 * # 왜 함수 하나로 모았나 (2026-09-23 QA P0-01)
 *
 * AI 추천 랭킹은 이 일을 `use-score-rankings`에서 하고 있었고, 종목 스크리너는
 * **아예 하지 않았다.** 그래서 장이 닫혀 있는 동안에는 두 화면이 같은 숫자를
 * 보여 주고(실측: 리튬포어스 073570이 양쪽 다 2,095원 · +1.70%), 열리는 순간
 * 갈렸다 — 한 화면은 흐르는 값, 다른 화면은 어제 종가.
 *
 * 서버도 같은 모양으로 고쳤다(`be/internal/service/list_price_overlay.go`).
 * 첫 응답을 서버가 맞춰 주더라도, 그 뒤로 **한쪽만 SSE로 흐르면 화면을 오래 열어
 * 둔 사람에게는 다시 갈린다** — 그래서 양쪽이 필요하다.
 *
 * # 판정과 적용을 나눈 이유
 *
 * 두 행 타입의 필드 이름이 다르다(`closePrice` ↔ `close`). 이름을 맞추려면 응답
 * 계약을 바꿔야 하고 그건 화면 여럿이 함께 따라오는 일이라, **나뉘는 자리를
 * 「이름」에 두고 「규칙」은 하나로** 둔다. 서버 쪽 분리와 같은 모양이다.
 */

export type LivePriceOverlay = {
  /** 이 행이 표시할 가격. */
  price: number;
  /**
   * 이 행이 표시할 등락률. **기준가를 모르면 null**이고, 그때 호출자는 직전
   * 등락률을 지킨다 — 0으로 덮으면 「보합」으로 읽히는데 실제 뜻은 「모른다」다.
   */
  changeRate: number | null;
};

/**
 * 스트림이 이 행에 대해 할 말이 있나.
 *
 * `null`이면 **행을 손대지 않는다.** 목록에서 그 종목만 조용히 종가로 남는 편이,
 * 절반은 실시간이고 절반은 빈칸인 표보다 낫다(서버 쪽 주석과 같은 규칙).
 *
 * @param symbol     행의 종목 코드. 대소문자는 여기서 맞춘다.
 * @param shownPrice 행이 지금 그리고 있는 가격. 틱이 같은 값이면 `null`을 돌려
 *   같은 객체를 유지하게 한다 — 100줄짜리 목록이 2초마다 통째로 다시 그려지는
 *   것을 막는 자리다.
 * @param basePrice  서버가 고른 기준가(`basePrice`). 없으면 등락률은 null이다.
 */
export function liveOverlayFor(
  symbol: string,
  shownPrice: number | null | undefined,
  basePrice: number | null | undefined,
  stream: ListPriceMap,
): LivePriceOverlay | null {
  const tick = stream.get(symbol.toUpperCase());
  if (!tick || !(tick.price > 0)) return null;
  if (shownPrice === tick.price) return null;
  return { price: tick.price, changeRate: changeRateFrom(tick.price, basePrice) };
}

/**
 * 「이 가격이 언제 것인가」를 말하는 칸의 값.
 *
 * 배치가 준 종가 날짜를 그대로 두면 **지금 흐르는 값 위에 이틀 전 날짜가 붙는다.**
 * 서버가 실시간으로 덮었을 때 쓰는 값과 같은 문자열이어야 한다
 * (`model.ScorePriceAsOfLive`).
 */
export const PRICE_AS_OF_LIVE = "live";
