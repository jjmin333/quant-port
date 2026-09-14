/*
목록 가격 스트림의 **순수 규칙**. DOM도 EventSource도 없다.

훅에서 떼어 낸 이유는 여기가 틀리기 쉬운 자리이기 때문이다 — 심볼 목록이 매 렌더
새 배열이라는 것, 늦게 도착한 값이 새 값을 덮는다는 것, 마감 시각이 이미 지났다는 것.
셋 다 브라우저에서는 「가끔 이상하다」로만 보이고 재현이 어렵다. 순수 함수로 두면
vitest가 직접 묻는다.
*/

/** 스트림이 전하는 한 줄. 서버의 `model.ListPriceEvent`와 같은 모양이다. */
export type ListPriceTick = {
  symbol: string;
  price: number;
  currency?: string;
  /**
   * 이 **가격이 정해진** 시각.
   *
   * 맵에 남는 값은 「마지막으로 프레임을 받은 시각」이 아니다 — 가격이 같은 틱은
   * 통째로 버려지므로(`applyListPriceTick`) 여기 남는 것은 값이 마지막으로 **움직인**
   * 시각이다. 늦게 도착한 값을 거르는 비교도 그 뜻으로 읽으면 맞다: 값이 바뀐
   * 시점보다 오래된 프레임만 버린다.
   */
  updatedAt: string;
};

/**
 * 맵에 들어가는 값 — 서버가 준 것에 **이 화면이 세어 둔 둘**을 더한 것.
 *
 * 이 둘이 필요한 이유는 목록이 가격 변화에 **플래시**를 주기 때문이다. 예전에는
 * 그 신호를 폴링이 만들었다(행 객체를 갈아 끼우며 `priceTick`을 올렸다). 가격을
 * 푸시로 옮기면서 그 자리가 사라졌는데, 화면은 **파생값**으로 그려야 해서
 * (렌더 중 계산 — 이펙트에서 setState를 하면 렌더가 한 번 더 돈다) 「몇 번째
 * 갱신인가」를 값 자체가 들고 있어야 한다.
 */
export type ListPriceEntry = ListPriceTick & {
  /**
   * 이 심볼의 **가격이 바뀐** 횟수. 화면은 이 숫자가 바뀌는 것만 보고 플래시를 준다.
   *
   * 「받은 횟수」가 아니다 — 값이 그대로인 틱은 세지 않는다(`applyListPriceTick`).
   */
  seq: number;
  /** 직전 가격. 플래시의 방향(상승·하락)을 정한다. 첫 값이면 null. */
  prevPrice: number | null;
};

export type ListPriceMap = Map<string, ListPriceEntry>;

/**
 * 심볼 목록을 이펙트 의존성으로 쓸 수 있는 **안정된 문자열**로 만든다.
 *
 * 목록 화면은 렌더마다 `items.map(i => i.symbol)`로 새 배열을 만든다. 그 배열을
 * 그대로 의존성에 걸면 참조가 매번 달라 **렌더마다 SSE를 끊고 다시 연결한다** —
 * 연결 하나로 아끼려던 것을 렌더 수만큼 되돌려 놓는 셈이다.
 *
 * 정렬해서 합치므로 순위가 뒤바뀌어도(같은 종목 집합이면) 키가 같다. 가격 구독에
 * 순서는 의미가 없고, 순위는 목록 갱신 때마다 흔들리기 때문이다.
 */
export function streamSymbolsKey(symbols: readonly string[]): string {
  const seen = new Set<string>();
  for (const s of symbols) {
    const sym = s?.trim().toUpperCase();
    if (sym) seen.add(sym);
  }
  return [...seen].sort().join(",");
}

/** `streamSymbolsKey`가 만든 키를 다시 목록으로. 훅이 URL을 만들 때 쓴다. */
export function symbolsFromKey(key: string): string[] {
  return key ? key.split(",") : [];
}

function toMillis(iso: string | undefined): number {
  if (!iso) return 0;
  const t = new Date(iso).getTime();
  return Number.isNaN(t) ? 0 : t;
}

/**
 * 틱 하나를 반영한 새 맵. **오래된 값은 새 값을 덮지 않는다.**
 *
 * SSE는 순서를 지키지만 이 화면에는 값의 출처가 둘이다 — 스트림과, 붙는 순간의
 * 스냅샷(그리고 스트림이 끊긴 동안의 폴링). 그 둘이 엇갈리면 방금 받은 가격 위에
 * 1초 전 스냅샷이 덮여 **숫자가 뒤로 튄다.** `updatedAt`으로 막는다.
 *
 * 값이 실제로 안 바뀌면 **같은 맵을 그대로 돌려준다** — 참조가 그대로여야 구독한
 * 컴포넌트가 다시 그리지 않는다. 2초마다 100줄짜리 목록을 통째로 다시 그리는 것과
 * 안 그리는 것의 차이다.
 */
export function applyListPriceTick(prev: ListPriceMap, tick: ListPriceTick): ListPriceMap {
  const symbol = tick.symbol?.trim().toUpperCase();
  if (!symbol || !(tick.price > 0)) return prev;

  const current = prev.get(symbol);
  if (current) {
    if (toMillis(current.updatedAt) > toMillis(tick.updatedAt)) return prev;
    /*
      **가격이 같으면 아무 일도 안 일어난 것이다** (2026-09-04 사용자 지적 3).

      예전에는 `updatedAt`까지 같아야 그대로 뒀다. 그런데 서버가 붙이는 시각은
      「캐시에 쓴 시각」이라 **값이 안 변해도 틱마다 달라진다.** 그래서 조용한
      종목도 초마다 새 엔트리가 되고, `seq`가 올라가고, 화면이 그것을 갱신으로
      세어 셀을 파랗고 빨갛게 칠했다 — 가격은 그대로인데.

      발행 쪽에도 같은 가드를 넣었지만(`publishListPrice`) 여기서도 막는다.
      값의 출처가 스트림 하나가 아니기 때문이다 — 붙는 순간의 스냅샷과 폴링이
      같은 맵에 들어오고, 그 둘은 「방금 바뀌었는가」를 모른 채 지금 값을 준다.
      **플래시의 진실은 이 맵에만 있다.**
    */
    if (current.price === tick.price) return prev;
  }

  const next = new Map(prev);
  next.set(symbol, {
    ...tick,
    symbol,
    seq: (current?.seq ?? 0) + 1,
    prevPrice: current?.price ?? null,
  });
  return next;
}

/** 여러 틱을 한 번에 — 스냅샷 프레임이 배열로 온다. 맵 복사는 한 번만 한다. */
export function applyListPriceTicks(
  prev: ListPriceMap,
  ticks: readonly ListPriceTick[],
): ListPriceMap {
  let next = prev;
  for (const tick of ticks) {
    next = applyListPriceTick(next, tick);
  }
  return next;
}

/**
 * 다음 개장까지 기다릴 밀리초. 시각이 없거나 이미 지났으면 null.
 *
 * 2초를 더하는 이유는 `use-quote-detail`과 같다 — 종이 울리는 그 순간에는 서버
 * 달력도 막 넘어가는 중이라, 정확히 그 시각에 물으면 아직 「닫힘」을 받는다.
 *
 * 상한은 `setTimeout`이 32비트를 넘기면 **즉시 발화**하기 때문에 필요하다(주말을
 * 넘기는 대기가 그 범위에 든다). 넘치면 최대치까지만 자고 그때 다시 잰다.
 */
export function reconnectDelayMs(nextSessionAt: string | undefined, now = Date.now()): number | null {
  if (!nextSessionAt) return null;
  const at = new Date(nextSessionAt).getTime();
  if (Number.isNaN(at)) return null;
  return Math.min(Math.max(at - now + 2_000, 1_000), 0x7fffffff);
}

/**
 * 스트림이 준 가격으로 등락률을 낸다.
 *
 * **기준가는 서버가 안 보낸다.** 세션 안에서 변하지 않는 값이라 목록이 처음 뜰 때
 * 받아 둔 것을 그대로 쓰면 되고, 서버가 종목마다 그걸 풀면 종목당 4~5 SELECT가
 * 붙는다(실측: `/stocks/scores/top` 20줄이 100 SELECT였다). 푸시로 옮긴 이유가
 * 사라지는 비용이라, 이 산수만 화면으로 넘겼다.
 *
 * 기준가가 없거나 0이면 null이다 — 0으로 나눈 값이나 「0.00%」를 내보내지 않는다.
 * 0.00%는 「보합」으로 읽히는데 실제 뜻은 「기준가를 모른다」이고, 그건 빈칸보다 나쁘다.
 */
export function changeRateFrom(price: number, prevClose: number | null | undefined): number | null {
  if (!(price > 0) || !prevClose || prevClose <= 0) return null;
  return ((price - prevClose) / prevClose) * 100;
}
