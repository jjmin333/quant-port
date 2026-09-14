/*
탭이 뒤로 가도 **SSE는 붙어 있는다** — 판정만 떼어 놓은 곳 (2026-09-05 사용자 요청 5).

훅(`hooks/use-stream-awake`)은 시계를 걸고 이벤트를 듣는 껍데기이고, 「지금 붙어
있을 때인가」라는 질문의 답은 여기 있다. 나눈 이유는 시험 때문이다 — 이 저장소의
vitest는 node 환경이라 DOM도 React도 없다. 규칙이 훅 안에만 있으면 3시간이라는
경계가 한 번도 확인되지 않은 채 남는다.
*/

/** 숨은 탭의 스트림을 놓기까지 기다리는 시간 — 3시간(2026-09-05 사용자 지정). */
export const STREAM_BACKGROUND_GRACE_MS = 3 * 60 * 60 * 1000;

/**
 * 지금 스트림을 붙여 둘 때인가.
 *
 * @param hiddenSince 탭이 숨은 시각(epoch ms). 보이는 중이면 `null`.
 * @param now 지금(epoch ms).
 * @param graceMs 숨은 채로 버틸 시간.
 *
 * 경계는 **미만**이다 — 정확히 3시간이 된 순간 놓는다. 「이하」로 두면 시계가
 * 정확히 그 값으로 깨어났을 때 한 박자를 더 기다리게 되고, 그 한 박자가 다시
 * 시계를 걸 이유가 된다.
 */
export function shouldKeepStream(
  hiddenSince: number | null,
  now: number,
  graceMs: number = STREAM_BACKGROUND_GRACE_MS,
): boolean {
  if (hiddenSince === null) return true;
  return now - hiddenSince < graceMs;
}
