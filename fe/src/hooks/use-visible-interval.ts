"use client";

import { useEffect, useRef } from "react";
import { isPageVisible } from "@/hooks/use-page-visible";
import { isRefreshDue } from "@/lib/refresh-due";

/*
**안 보이는 탭은 주기 요청을 만들지 않는다** (2026-09-15 사용자 요청 3).

# 무엇이 잘못돼 있었나

가격 폴링 훅들은 이미 `isPageVisible()` 가드를 갖고 있었는데(`use-quote-detail`,
`use-market-rankings`), **목록 화면 셋은 빠져 있었다** — 보유·관심·관심 실험실이
각자 `window.setInterval(refreshQuotes, …)`을 걸고 숨은 채로도 계속 돌렸다.
펀더멘털 폴링(종목 상세)도 같았다.

창을 다섯 띄운 사용자에게 이것은 **다섯 배**다. 스트림과 달리 폴링은 왕복이 뷰어
수에 정비례하고, 그 끝에는 Toss 호출이 있다.

# 왜 「가드」가 아니라 이 훅인가

가드를 `refreshQuotes` 안에 넣으면 첫 적재까지 막힌다 — 숨은 채로 마운트된 화면
(백그라운드에서 연 링크)이 영영 비어 있게 된다. 부르는 자리마다 `if`를 적으면
이번처럼 **하나를 빠뜨린다.**

그래서 「주기적으로 부르는 일」 자체를 한 이름으로 만든다. 첫 적재는 화면이 자기
이펙트로 하던 대로 하고, 이 훅은 **되풀이만** 맡는다.

# 돌아오면 기다리지 않는다 — 판정은 **시각**으로 (2026-09-16 사용자 지적)

처음에는 「숨어 있는 동안 회차가 **울렸는가**」를 표식으로 들고 있다가 다시 보일 때
메웠다. 그 전제가 틀렸다 — 크롬은 오래 숨은 탭의 타이머를 분당 한 번으로 내리고
조건이 맞으면 탭을 **얼린다**(콜백이 하나도 안 돈다). 창이 완전히 가려져도 `hidden`
이고, 노트북을 덮으면 타이머는 멈춘 채 시각만 흐른다. 그래서 몇 시간이 지나도 표식이
false인 채로 남아 **돌아와도 아무 일이 없었다** — 「탭을 계속 띄워놓고 있었는데 제대로
안 바뀌던데」가 이것이다.

지금은 **마지막으로 부른 시각** 하나만 센다. 회차가 울리든, 보이게 되든, 창이 포커스를
받든, bfcache에서 되살아나든 — 질문은 언제나 「마지막으로 부른 지 한 주기가 지났는가」
다(`lib/refresh-due`). 이유가 넷이면 부르는 조건도 넷이어야 할 것 같지만, 그 넷은
**깨어날 기회**일 뿐이고 판정은 하나다.

같은 함정을 `lib/stream-awake`가 먼저 밟았다 — 「울렸으니 지났다」는 믿을 수 없다.
*/

/**
 * `enabled`인 동안 `ms`마다 `fn`을 부른다 — **탭이 보이는 동안만**.
 *
 * 숨은 동안에는 부르지 않고, 다시 보이는 순간 **마지막 호출로부터 한 주기가 지났으면**
 * 즉시 한 번 부른다(안 지났으면 값이 아직 젊다는 뜻이라 부르지 않는다).
 * 첫 호출은 하지 않는다(화면의 첫 적재는 화면이 한다).
 */
export function useVisibleInterval(fn: () => void, ms: number, enabled = true): void {
  /*
    콜백을 ref로 든다. 호출부가 매 렌더 새 함수를 넘겨도(대개 그렇다) 타이머를
    다시 걸지 않으려는 것이다 — 다시 걸면 주기가 영영 채워지지 않는다.
  */
  const fnRef = useRef(fn);
  useEffect(() => {
    fnRef.current = fn;
  }, [fn]);

  /**
   * 마지막으로 부른 시각. 마운트 직후에는 **화면의 첫 적재**가 그 자리라, 값을 넣는
   * 것은 아래 이펙트다 — 렌더 중에 `Date.now()`를 읽으면 순수하지 않다
   * (`react-hooks/purity`). 이펙트가 반드시 먼저 돌므로 0이 쓰이는 일은 없다.
   */
  const lastRunRef = useRef(0);

  useEffect(() => {
    if (!enabled || ms <= 0) return;
    lastRunRef.current = Date.now();

    /**
     * 깨어날 기회마다 **같은 질문**을 한다 — 회차가 울렸을 때, 다시 보일 때, 창이
     * 포커스를 받을 때(가려졌다 드러난 창), bfcache에서 되살아날 때.
     */
    const runIfDue = () => {
      if (!isPageVisible()) return;
      const now = Date.now();
      if (!isRefreshDue(lastRunRef.current, now, ms)) return;
      lastRunRef.current = now;
      fnRef.current();
    };

    const id = window.setInterval(runIfDue, ms);
    document.addEventListener("visibilitychange", runIfDue);
    window.addEventListener("focus", runIfDue);
    window.addEventListener("pageshow", runIfDue);

    return () => {
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", runIfDue);
      window.removeEventListener("focus", runIfDue);
      window.removeEventListener("pageshow", runIfDue);
    };
  }, [ms, enabled]);
}
