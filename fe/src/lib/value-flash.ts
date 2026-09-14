/*
값이 움직였을 때 화면이 주는 **색 신호의 규칙**. DOM도 타이머도 없다.

# 왜 순수 함수로 떼어 내는가

이 판정이 틀리는 방식은 전부 「가끔 이상하다」로만 보인다 — 처음 뜰 때 한 번
번쩍이거나, 스크롤할 때마다 목록이 깜빡이거나, 종목을 바꿨는데 앞 종목의 가격과
비교해 색이 뜬다. 셋 다 브라우저에서 재현하기 어렵고 눈으로 확인하기도 어렵다.
규칙만 순수 함수로 두면 vitest가 직접 묻는다.

# 세 가지를 구분해야 한다

	첫 값을 받았다        → 색 없음  (비교할 앞 값이 없다)
	같은 값을 다시 받았다 → 색 없음  (아무 일도 안 일어났다)
	값이 움직였다         → 색 있음

가운데 줄이 2026-09-04에 사용자가 지적한 것이다(요청 3). 서버가 값을 안 바꾸고
프레임만 보내도 화면이 그것을 「갱신」으로 세어 색을 칠했다.
*/

/** 값이 움직인 방향. 색은 화면이 정한다(상승 빨강·하락 파랑). */
export type FlashDirection = "up" | "down";

/**
 * 플래시가 화면에 남아 있는 시간(ms).
 *
 * **앱 전체가 이 하나를 쓴다.** 마켓 레이더 행이 700ms를 들고 있었는데, 같은 신호를
 * 다른 화면이 다른 길이로 주면 한 앱이 여러 속도로 말하는 것으로 보인다.
 *
 * 700ms인 이유: 1초 주기(`PRICE_POLL_MS`)보다 짧아야 **다음 갱신 전에 꺼진다.**
 * 안 꺼지면 활발한 종목의 셀이 계속 물들어 있어 색이 「방금 움직였다」가 아니라
 * 「이 종목은 원래 이런 색」으로 읽힌다.
 */
export const VALUE_FLASH_MS = 700;

/**
 * 플래시 상태 — 훅이 들고 있는 값.
 *
 * `seq`가 따로 있는 이유: 같은 방향으로 연달아 움직이면 `direction`은 안 바뀌는데
 * 타이머는 **다시 걸려야** 한다. 방향만 보고 이펙트를 걸면 두 번째 상승에서
 * 타이머가 갱신되지 않아 첫 상승 기준으로 색이 꺼진다.
 */
export type FlashState = {
  /** 마지막으로 비교 기준이 된 값. 아직 아무 값도 못 받았으면 null. */
  seen: number | null;
  direction: FlashDirection | null;
  seq: number;
};

export const IDLE_FLASH: FlashState = { seen: null, direction: null, seq: 0 };

function isNumber(v: number | null | undefined): v is number {
  return typeof v === "number" && Number.isFinite(v);
}

/**
 * 다음 플래시 상태. **값이 안 움직였으면 받은 상태를 그대로 돌려준다** — 참조가
 * 그대로여야 호출부가 「바뀐 것이 없다」를 참조 비교로 알 수 있다.
 *
 * 값이 아직 없으면(`null`·`undefined`·NaN) 기준을 건드리지 않는다. 로딩 중 한
 * 프레임을 빈 값으로 그리는 화면이 흔한데, 그걸 「0으로 떨어졌다」로 읽으면
 * 목록 전체가 파랗게 물든다.
 */
export function nextFlashState(prev: FlashState, value: number | null | undefined): FlashState {
  if (!isNumber(value)) return prev;
  if (prev.seen === null) {
    // 첫 값에는 색이 없다 — 비교할 앞 값이 없으므로 방향이라는 것이 존재하지 않는다.
    return { seen: value, direction: null, seq: prev.seq };
  }
  if (prev.seen === value) return prev;
  return {
    seen: value,
    direction: value > prev.seen ? "up" : "down",
    seq: prev.seq + 1,
  };
}

/**
 * 「이건 다른 것의 값이다」로 되돌린다 — 색 없이 기준만 옮긴다.
 *
 * 한 컴포넌트가 종목을 갈아 끼우며 재사용될 때 쓴다(목록 가상 스크롤, 상세 화면의
 * 종목 전환). 이게 없으면 **앞 종목의 가격과 비교해** 색이 뜬다 — 삼성전자 7만원을
 * 보다가 SK하이닉스 25만원으로 넘어가면 상승 플래시가 뜨는데, 오른 것은 아무것도 없다.
 */
export function resetFlashState(value: number | null | undefined): FlashState {
  return { seen: isNumber(value) ? value : null, direction: null, seq: 0 };
}

/** 타이머가 끝났을 때 — 방향만 지우고 기준값과 `seq`는 남긴다. */
export function clearFlashDirection(prev: FlashState): FlashState {
  if (prev.direction === null) return prev;
  return { ...prev, direction: null };
}
