import { formatPercent, formatPrice } from "@/lib/format";
import type { QuoteCurrency } from "@/lib/quotes-types";

/*
브라우저 **탭 제목**이 말할 한 줄 (2026-09-15 사용자 요청 4).

	$141.91 -1.98% | 오라클
	1,703,000원 +0.35% | SK하이닉스

# 왜 탭 제목인가

사용자가 보낸 사진이 증권사 앱의 떠 있는 시세 줄이 아니라 **브라우저 탭**이었다.
그게 요청 3·4의 앞뒤를 맞춘다 — 탭 제목은 그 탭이 **안 보이는 동안에도** 탭
스트립에 그대로 보인다. 그래서 숨은 탭이 「보던 종목 하나」만은 계속 받아야 한다
(`lib/pinned-symbols`).

# 값은 왜 가격이 먼저인가

탭이 여럿이면 제목은 앞에서부터 잘린다. 잘린 뒤에도 남아야 하는 것은 **숫자**다 —
어느 종목인지는 그 탭을 연 사람이 이미 알고 있고, 모르는 것은 「지금 얼마인가」다.
사진의 순서가 그렇고, 이유도 그것이라고 본다.

# 등락률은 스트림에 없다

목록 가격 스트림이 실어 주는 것은 **가격뿐**이다(`lib/quote-stream-state`의
`ListPriceTick`). 등락률은 기준가(전일 종가)가 있어야 나오므로, 호출부가 스냅샷
한 벌에서 기준가를 받아 두고 새 가격이 올 때마다 여기서 다시 센다 —
`applyStreamPricesToQuotes`가 목록에서 하는 것과 같은 계산이다.
*/

export type QuoteTabTitleInput = {
  name: string;
  price: number;
  /** 전일 종가. 없거나 0이면 등락률을 적지 않는다. */
  prevClose: number;
  currency: QuoteCurrency;
};

/**
 * 탭 제목 한 줄. 그릴 것이 없으면 **빈 문자열**을 돌려준다 — 호출부는 그때 원래
 * 제목을 그대로 둔다.
 *
 * 가격이 0 이하면 아직 못 받은 값이다. 그 자리에 「—」를 적으면 탭 스트립에 고장난
 * 탭이 하나 서 있는 것처럼 보이므로, 값이 올 때까지는 아예 손대지 않는다.
 */
export function formatQuoteTabTitle(input: QuoteTabTitleInput): string {
  const name = input.name.trim();
  if (name === "" || !(input.price > 0)) return "";

  const price = formatPrice(input.price, input.currency);
  const rate = quoteChangeRate(input.price, input.prevClose);
  /*
    기준가를 모르면 등락률을 **빼고** 가격만 적는다. 0.00%로 적으면 「오늘 안
    움직였다」는 거짓말이 되고, 그것은 이 줄이 하는 유일한 일을 뒤집는다.
  */
  if (rate === null) return `${price} | ${name}`;
  return `${price} ${formatPercent(rate)} | ${name}`;
}

/**
 * 새 가격과 전일 종가로 등락률을 다시 센다. 기준가가 없으면 `null`.
 *
 * 스냅샷이 준 `changeRate`를 그대로 쓰면 안 된다 — 그 값은 스냅샷을 받은 순간의
 * 것이고, 그 뒤에 스트림으로 온 가격과 짝이 맞지 않는다. 탭 제목에서 그 어긋남은
 * 「가격은 움직이는데 등락률만 멈춰 있다」로 보인다.
 */
export function quoteChangeRate(price: number, prevClose: number): number | null {
  if (!(prevClose > 0) || !(price > 0)) return null;
  return ((price - prevClose) / prevClose) * 100;
}
