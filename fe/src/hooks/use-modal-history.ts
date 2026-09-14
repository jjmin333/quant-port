"use client";

import { useEffect, useRef } from "react";

import { MODAL_HISTORY_FLAG, shouldUnwindModalEntry } from "@/lib/modal-history";

/**
 * 모달이 열려 있는 동안 **히스토리 항목 하나를 점유한다** — 뒤로가기가 앱을 떠나는
 * 대신 모달을 닫게 하려고 (§C-3).
 *
 * 왜 필요한가: 모달은 화면을 덮는데 주소는 그대로다. 사용자 입장에서는 「새 화면」이
 * 열린 것이므로 닫는 동작으로 뒤로가기를 누른다. 그런데 히스토리에 아무것도 쌓이지
 * 않았으니 그 뒤로가기는 **직전 페이지로 나가 버린다.** 실측: `/news/domestic`에서
 * 기사 모달을 열고 뒤로가기 → 모달이 닫히는 게 아니라 사이트 밖(`about:blank`)으로
 * 나갔다. 모바일에서는 물리 뒤로가기가 기본 반사라 더 자주 밟힌다.
 *
 * 주소를 바꾸지 않는 이유: 퀵룩 모달은 목록 위에 겹쳐 보는 것이지 그 자체가 페이지가
 * 아니다. 주소를 바꾸면 새로고침·공유했을 때 무엇을 그려야 하는지가 애매해지고,
 * 지금 화면(카드가 `<a href>`이기도 하다)에서는 전문 페이지가 이미 그 역할을 한다.
 */

/*
  ── 왜 모듈 스코프에 스택이 필요한가 ────────────────────────────────────────

  `popstate`는 **창 하나에 오는 사건**인데, 열려 있는 모달마다 각자 리스너를 달면
  그 하나의 사건이 전부에게 배달된다. 모달이 겹쳐 있으면 그게 곧바로 버그가 된다:

      「한 눈에 보기」 목록(A) 열림 → 항목 A push
      목록에서 지수 하나 누름(B)   → 항목 B push
      B를 닫기 버튼으로 닫음        → B가 back() → popstate 1회
      → A의 리스너도 그 popstate를 받아 **A까지 닫힌다**

  실측(08-31): 지수 목록에서 지수를 열었다 닫으면 목록이 함께 사라져, 다음 지수를
  보려면 목록을 매번 다시 열어야 했다.

  그래서 리스너는 **창에 하나만** 두고, 열린 순서를 스택으로 들고 있다가 맨 위 하나만
  닫는다. 그리고 우리가 부른 `back()`이 만든 popstate는 사용자의 뒤로가기가 아니므로
  세어 두었다가 삼킨다 — 그러지 않으면 닫는 동작이 아래 모달을 연쇄로 닫는다.
*/
type StackEntry = { close: () => void };

const stack: StackEntry[] = [];
/** 우리가 부른 `back()`이 만들어 낼 popstate 수. 사용자 뒤로가기와 구분하는 값. */
let selfPops = 0;
let listening = false;

function onPopState() {
  if (selfPops > 0) {
    // 우리가 스스로 걷어낸 항목이다 — 닫을 것이 이미 닫혔다.
    selfPops -= 1;
    return;
  }
  stack.pop()?.close();
}

function syncListener() {
  const needed = stack.length > 0 || selfPops > 0;
  if (needed && !listening) {
    window.addEventListener("popstate", onPopState);
    listening = true;
  } else if (!needed && listening) {
    window.removeEventListener("popstate", onPopState);
    listening = false;
  }
}

export function useModalHistory(open: boolean, onClose: () => void): void {
  const pushedRef = useRef(false);
  /*
    최신 `onClose` 를 이펙트 재실행 없이 참조한다 — 호출부가 인라인 화살표 함수를
    넘기면 매 렌더 새 참조가 되는데, 그때마다 이펙트가 다시 돌면 히스토리 항목이
    쌓였다 지워졌다 한다.

    **렌더 중이 아니라 이펙트에서** 담는다. 렌더 중 `ref.current` 를 건드리면 React가
    기대하는 순수성이 깨지고(동시성 모드에서 렌더는 버려질 수 있다) 린트도 막는다.
  */
  const closeRef = useRef(onClose);
  useEffect(() => {
    closeRef.current = onClose;
  }, [onClose]);

  /*
    되돌리기를 **예약**해 두는 자리.

    `history.back()`은 동기 호출이 아니다 — 브라우저가 `popstate`를 다음 태스크에
    올린다. 그 비동기성이 개발 모드에서 모달을 열자마자 닫아 버렸다:

        push        ← 이펙트 1회차
        back        ← StrictMode가 곧바로 정리(cleanup)를 돌린다
        push        ← 이펙트 2회차(같은 인스턴스에 다시 구독)
        pop         ← 1회차가 부른 back의 popstate가 **이제야** 도착
        onClose()   ← 열자마자 자기가 닫는다

    실측(08-31): 홈의 AI 추천 랭킹·주요 실적·많이 언급된 종목에서 종목을 눌러도 퀵룩이
    뜨지 않았다. React가 이펙트를 마운트→정리→마운트로 한 번 더 돌리는 것은 정상이고,
    **거기서 살아남지 못한 쪽은 히스토리를 만지는 이 이펙트**다. 운영 빌드는 이펙트가
    한 번만 돌아 증상이 없었고, 그래서 개발에서만 나는 것처럼 보였다.

    그래서 정리 단계에서 곧바로 되돌리지 않고 **다음 태스크로 미룬다.** 이펙트가 곧이어
    다시 돌면 그 예약을 취소하고 이미 쌓아 둔 항목을 그대로 쓴다 — push/back 왕복 자체가
    사라진다. 진짜로 닫힌 경우에는 예약이 그대로 실행돼 항목이 걷힌다.
  */
  const backTimerRef = useRef<number | null>(null);
  const entryRef = useRef<StackEntry | null>(null);

  useEffect(() => {
    if (!open) return;
    if (typeof window === "undefined") return;

    if (backTimerRef.current != null) {
      // 정리가 예약해 둔 되돌리기를 취소한다 — 우리가 쌓은 항목이 아직 살아 있다.
      window.clearTimeout(backTimerRef.current);
      backTimerRef.current = null;
      pushedRef.current = true;
    } else {
      window.history.pushState({ [MODAL_HISTORY_FLAG]: true }, "");
      pushedRef.current = true;
    }

    const entry: StackEntry = { close: () => closeRef.current() };
    entryRef.current = entry;
    stack.push(entry);
    syncListener();

    return () => {
      const at = stack.indexOf(entry);
      if (at >= 0) stack.splice(at, 1);
      entryRef.current = null;
      syncListener();

      if (!pushedRef.current) return;
      // 뒤로가기가 아닌 경로로 닫혔다 — 우리가 남긴 항목을 치운다. 다만 곧바로가 아니라
      // 다음 태스크에서. 이펙트가 다시 돌면 위에서 이 예약을 취소한다.
      backTimerRef.current = window.setTimeout(() => {
        backTimerRef.current = null;
        pushedRef.current = false;
        /*
          **떠나서 닫힌 경우에는 되돌리지 않는다** (2026-09-06 사용자 지적 2).

          모달 안에서 다른 주소로 갔다면 우리 항목 위에 그 화면의 항목이 올라와 있다.
          그때 부르는 `back()`은 우리 것이 아니라 **방금 연 화면**을 걷어낸다 —
          비로그인으로 종목 창의 관심을 누르면 로그인 화면이 한 프레임 만에 튕겨
          나갔다. 판정 규칙과 실측 기록은 `lib/modal-history`에 있다.
        */
        if (!shouldUnwindModalEntry(window.history.state)) return;
        /*
          이 back()이 만들 popstate는 사용자의 뒤로가기가 아니다. 세어 두지 않으면
          아래에 깔린 모달이 그 사건을 받아 **연쇄로 닫힌다** — 위 스택 주석 참고.
        */
        selfPops += 1;
        syncListener();
        window.history.back();
      }, 0);
    };
  }, [open]);
}
