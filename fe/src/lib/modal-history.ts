/**
 * 모달이 점유한 히스토리 항목을 **되돌려도 되는가**를 가르는 규칙.
 *
 * # 왜 규칙이 필요해졌나 (2026-09-06 사용자 지적 2)
 *
 * `hooks/use-modal-history`는 모달이 열리면 히스토리 항목을 하나 쌓고, 뒤로가기가
 * 아닌 길로 닫히면 그 항목을 `history.back()`으로 걷어낸다. 그 전제는 **닫힐 때
 * 우리 항목이 아직 맨 위에 있다**는 것이다.
 *
 * 모달 안에서 **다른 화면으로 떠나면** 그 전제가 깨진다. 실측(비로그인·종목 퀵룩):
 *
 *     push:(same)                      ← 모달이 쌓은 항목
 *     push:/login?next=/news/domestic  ← 관심 버튼 → useLoginGate
 *     back                             ← 모달이 언마운트되며 부른 되돌리기
 *     @/login → @/news/domestic        ← 로그인 화면이 **한 프레임 만에** 튕겨 나갔다
 *
 * 사용자에게는 「로그인 페이지로 갔다가 곧바로 원래 화면으로 되돌아오는」 고장으로
 * 보인다. 되돌리기가 걷어낸 것은 자기 항목이 아니라 **방금 연 로그인 화면**이었다.
 *
 * # 무엇으로 가르는가
 *
 * 우리가 쌓은 항목에는 표식(`quantPortModal`)이 있다. 되돌리기 직전에 `history.state`를
 * 읽어 그 표식이 있으면 맨 위가 아직 우리 것이므로 걷어내고, 없으면 그 위에 남의
 * 항목이 올라온 것이므로 **손대지 않는다.**
 *
 * 주소가 아니라 표식을 보는 이유: 모달은 주소를 바꾸지 않고 항목만 쌓으므로
 * (`pushState(state, "")`), 같은 주소에서 열고 닫는 평범한 경우에 주소 비교는
 * 아무것도 가려내지 못한다.
 *
 * Next의 라우터가 `pushState`를 감싸 자기 트리를 같은 상태 객체에 얹지만, 그건
 * **우리 표식을 지우지 않는다** — 실측한 상태 객체는
 * `{quantPortModal: true, __NA: true, __PRIVATE_NEXTJS_INTERNALS_TREE: {…}}`였다.
 * 반대로 라우터가 스스로 만든 항목에는 표식이 없다. 그래서 이 판정이 성립한다.
 *
 * # 남는 항목은 그대로 둔다
 *
 * 떠난 경우 우리 항목은 히스토리에 남는다(`/news → /news(모달) → /login`). 로그인
 * 화면에서 뒤로가기를 누르면 주소가 같은 그 항목으로 돌아오므로 사용자가 보는 화면은
 * 모달이 없는 `/news`다 — 아무 일도 일어나지 않은 것처럼 보인다. 그 한 칸을 지우자고
 * 떠나는 길을 되돌리는 것보다, 한 칸 남기는 쪽이 사용자에게 싸다.
 */

/** 모달이 쌓은 히스토리 항목의 표식. */
export const MODAL_HISTORY_FLAG = "quantPortModal";

/**
 * 지금 히스토리 맨 위가 **모달이 쌓은 항목인가**. 순수 함수 — `history.state`만 본다.
 *
 * `null`(항목을 쌓은 적 없는 첫 진입)이나 라우터가 만든 상태에는 표식이 없다.
 */
export function isModalHistoryEntry(state: unknown): boolean {
  if (typeof state !== "object" || state === null) return false;
  return (state as Record<string, unknown>)[MODAL_HISTORY_FLAG] === true;
}

/**
 * 모달이 닫혔을 때 `history.back()`으로 우리 항목을 걷어내도 되는가.
 *
 * @param state 되돌리기 **직전에 읽은** `window.history.state`.
 */
export function shouldUnwindModalEntry(state: unknown): boolean {
  if (leavingViaNavigation) {
    // 떠나는 쪽이 미리 말해 두었다 — 상태는 볼 것도 없다. 아래 `leaveModal` 참고.
    leavingViaNavigation = false;
    return false;
  }
  return isModalHistoryEntry(state);
}

/*
  ── 표식만으로는 못 가리는 경우 (2026-09-06 사용자 지적 1) ─────────────────

  위 규칙은 **떠나는 쪽이 히스토리를 먼저 만졌을 때**만 성립한다. `useLoginGate`처럼
  클릭 안에서 곧바로 `router.push`를 부르는 자리는 그렇다.

  Next의 `<Link>`는 다르다. 이동이 트랜지션 안에서 벌어지고, 갈 화면의 페이로드를
  기다린다. 그동안 모달은 이미 언마운트돼 되돌리기가 예약된다. 둘의 순서는 **갈 화면이
  얼마나 빨리 준비되는가**에 달린다 — 실측(퀵룩 바닥 「종목 분석 보기」):

      /watchlist (분석 화면이 준비돼 있던 경우)
        replace:/watchlist(__NA) → push:/analysis?symbol=005930   ← 이동 성공
      /holdings (분석 화면이 아직 준비되지 않은 경우)
        back → replace:/holdings(__NA) → pop@/holdings            ← 이동이 통째로 사라졌다

  뒤쪽에서는 되돌리기가 먼저 나가 **아직 표식이 살아 있고**, 그 popstate가 진행 중이던
  이동을 밀어냈다. 사용자에게는 「버튼을 눌러도 아무 일도 없다」로 보인다. 처음 눌렀을
  때 늘 느린 쪽이라 사실상 항상 고장이었다.

  표식을 언제 읽어도 이 경합은 남는다 — 그래서 **떠나는 쪽이 미리 말한다.** 화면을
  떠나며 닫는 자리는 `leaveModal()`로 닫고, 그 한 번의 판정은 상태를 보지 않고 건너뛴다.

  표식을 남겨 두는 대가는 히스토리 한 칸이고, 그건 위 「남는 항목은 그대로 둔다」와 같다.
*/

/** `leaveModal()`이 세워 두는, **판정 한 번짜리** 표식. */
let leavingViaNavigation = false;

/**
 * 모달 안에서 **화면을 떠나며** 닫는다. 다음 되돌리기 판정 한 번을 건너뛴다.
 *
 * 링크의 `onClick`에서 `onClose` 대신 부른다 — 닫기와 이동이 한 클릭 안에서 함께
 * 일어나는 자리가 그 대상이다.
 */
export function leaveModal(close: () => void): void {
  leavingViaNavigation = true;
  close();
}

/** 테스트용 — 모듈 스코프 표식을 지운다. */
export function resetLeaveModalMark(): void {
  leavingViaNavigation = false;
}
