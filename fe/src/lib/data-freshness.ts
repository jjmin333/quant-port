/*
**이 값이 언제 것인가, 그리고 너무 늙었는가** (2026-09-20 SSR/CSR 점검 요청 7).

# 두 개의 「기준」을 섞지 않는다

화면에 적히는 「기준」에는 **뜻이 다른 두 가지**가 있고, 이 파일이 다루는 것은
하나뿐이다.

  ⓐ **갱신 기준시각** — 우리 파이프라인이 이 값을 마지막으로 받아 온 시각.
     `/quotes/indices`의 `updatedAt`이 그것이다. 이게 늙으면 **고장**이다.
  ⓑ **데이터 기준일** — 값 자체가 속한 거래일. 점수 랭킹의 `tradeDate`가 그것이다.
     일요일에 이 값이 금요일이어도 **정상**이다. 시장이 닫혀 있었으니까.

ⓑ에 벽시계 기준을 들이대면 **주말마다 앱 전체가 「낡음」이라고 자백한다.** 같은
함정을 배치 감시가 이미 한 번 밟았다 — 15:50 국내 종가 잡은 KRX 달력으로 거르는데
기대만 「매일」로 적혀 있어서 토·일마다 거짓 경보가 났다(09-19 하루에 67회,
`docs/archive/sessions.md`). 달력으로 도는 것은 달력으로 판정해야 한다.

그래서 이 파일에는 `tradeDate`를 받는 함수가 없다. 거래일은 **표기만** 하고
(`tradeDateLabel`) 절대 늙었다고 말하지 않는다.

# 임계값이 왜 「주기의 3배」인가

블록마다 뒤에서 값을 만드는 주기가 다르고(5분·30분·90초, `lib/screen-refresh`),
그 주기가 이미 코드에 상수로 있다. 「낡았다」의 뜻은 **「돌았어야 할 회차가 여러 번
지나갔다」**이므로, 기준을 그 주기에서 끌어오면 숫자를 새로 지어내지 않아도 된다.

3배인 이유는 한 번 걸러야 할 것이 셋이라서다 — 요청 한 번의 실패(재시도가 먹는다),
탭이 숨어 있던 구간(`useVisibleInterval`이 건너뛴다), 배치가 몇 분 늦는 경우.
2배면 이 셋 중 어느 것에도 배지가 뜨고, 그러면 배지는 곧 배경이 된다.
*/

/** 「낡았다」로 보기까지 허용하는 갱신 주기의 배수. */
export const STALE_PERIOD_MULTIPLIER = 3;

export type Freshness = {
  /** 파싱된 기준시각. 값이 없거나 형식이 틀리면 null. */
  at: Date | null;
  /** 기준시각으로부터 흐른 밀리초. 모르면 null. */
  ageMs: number | null;
  /** 갱신 주기의 3배를 넘겼는가. 기준시각을 모르면 false — 모름은 고장이 아니다. */
  stale: boolean;
};

export function freshnessOf(
  updatedAt: string | null | undefined,
  refreshMs: number,
  now: number = Date.now(),
): Freshness {
  if (!updatedAt) return { at: null, ageMs: null, stale: false };
  const at = new Date(updatedAt);
  if (Number.isNaN(at.getTime())) return { at: null, ageMs: null, stale: false };

  const ageMs = now - at.getTime();
  /*
    **미래 시각은 늙지 않았다.** 서버와 브라우저 시계가 몇 초 어긋나는 것은 흔하고,
    그때 음수 나이를 그대로 비교하면 조용히 통과한다 — 여기서는 명시적으로 0으로
    깎아 「방금 받은 값」으로 읽는다.
  */
  const age = Math.max(0, ageMs);
  return { at, ageMs: age, stale: age > refreshMs * STALE_PERIOD_MULTIPLIER };
}

/**
 * 카드 헤더에 적는 기준시각 — `오후 11:39 기준`.
 *
 * 시간대를 서울로 **못 박는다.** 이 앱의 「오늘」은 KRX의 오늘이고, 해외에서 열어도
 * 같은 값이 같은 이름으로 불려야 한다 — 지수 스트립이 이미 같은 방식으로 적는다.
 */
export function updatedAtLabel(at: Date | null): string | null {
  if (!at) return null;
  return `${at.toLocaleTimeString("ko-KR", {
    timeZone: "Asia/Seoul",
    hour: "2-digit",
    minute: "2-digit",
  })} 기준`;
}

/**
 * 「낡음」 배지에 붙는 경과 시간 — `12분 지남` · `3시간 지남`.
 *
 * 초 단위는 쓰지 않는다. 이 배지가 뜨는 시점은 이미 주기의 3배가 지난 뒤라
 * 가장 짧은 경우(90초 주기)도 4분이 넘는다.
 */
export function staleAgeLabel(ageMs: number | null): string | null {
  if (ageMs == null) return null;
  const minutes = Math.floor(ageMs / 60_000);
  if (minutes < 60) return `${Math.max(1, minutes)}분 지남`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}시간 지남`;
  return `${Math.floor(hours / 24)}일 지남`;
}

/**
 * 데이터 기준일 — `09-18 종가 기준`.
 *
 * **늙었다고 말하지 않는다.** 거래일은 시장 달력이 정하는 값이라 주말·공휴일에
 * 며칠 전이어도 정상이다(이 파일 머리글 ⓑ).
 */
export function tradeDateLabel(tradeDate: string | null | undefined): string | null {
  const date = (tradeDate ?? "").slice(0, 10);
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) return null;
  return `${date.slice(5)} 종가 기준`;
}
