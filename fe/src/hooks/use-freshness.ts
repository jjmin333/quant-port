"use client";

import { useSyncExternalStore } from "react";
import { freshnessOf, type Freshness } from "@/lib/data-freshness";

/**
 * 「이 값이 늙었는가」를 **hydration 뒤에** 판정한다 (2026-09-20 점검 요청 7).
 *
 * # 왜 훅이 필요한가 — `Date.now()`는 서버와 클라이언트가 다르다
 *
 * 판정에 현재 시각이 들어가므로 렌더 중에 그냥 부르면 서버가 그린 HTML과 브라우저가
 * 처음 그리는 결과가 **달라질 수 있다.** 서버에서 89초였던 값이 브라우저가 hydration
 * 할 때 91초이면 배지가 한쪽에만 있고, React는 그것을 불일치로 잡는다.
 *
 * 그래서 첫 렌더(서버·hydration 양쪽)에서는 시계를 **모른다**고 답한다 — `now`가
 * null이면 `stale`은 언제나 false다. 마운트 뒤에 시계가 생기고, 그때 배지가 나타난다.
 * 기준시각 **표기**는 이 판정을 안 거친다 — 그쪽은 고정된 시간대로 포매팅하므로
 * 양쪽이 같다.
 *
 * # 왜 `useState` + `useEffect`가 아닌가
 *
 * 이펙트 안에서 `setNow(Date.now())`를 하면 마운트 직후 렌더가 한 번 더 돌고, 그건
 * 정확히 lint가 막는 모양이다(`react-hooks/set-state-in-effect`). `use-market-indices`가
 * 같은 이유로 `useSyncExternalStore`를 쓴다 — 시계도 **React 밖에 사는 값**이라
 * 같은 도구가 맞다.
 *
 * # 1분마다 다시 본다
 *
 * 화면을 열어 둔 채로 값이 늙는 경우가 이 배지가 있어야 할 바로 그 경우다. 한 번
 * 계산하고 말면 「열었을 때 신선했으니 계속 신선하다」가 되어, 고치려던 증상을
 * 그대로 둔다. 1분은 가장 짧은 임계값(90초 주기 × 3 = 4.5분)에서도 충분히 촘촘하다.
 *
 * 구독자가 몇이든 **타이머는 하나**다. 카드마다 자기 `setInterval`을 들면 홈에서만
 * 열 개가 돈다.
 */
const TICK_MS = 60_000;

let now: number | null = null;
const subscribers = new Set<() => void>();
let timer: ReturnType<typeof setInterval> | null = null;

function subscribeClock(notify: () => void): () => void {
  subscribers.add(notify);
  if (timer == null) {
    /*
      첫 구독자가 붙는 순간 시계를 연다. 이 대입은 **구독 시점**에 일어나므로
      렌더 중이 아니다 — `useSyncExternalStore`가 구독을 이펙트에서 건다.
    */
    now = Date.now();
    timer = setInterval(() => {
      now = Date.now();
      for (const fn of subscribers) fn();
    }, TICK_MS);
  }
  return () => {
    subscribers.delete(notify);
    if (subscribers.size === 0 && timer != null) {
      clearInterval(timer);
      timer = null;
    }
  };
}

const getClock = () => now;
/** 서버와 hydration은 시계를 모른다 — 그래야 양쪽이 같은 HTML을 낸다. */
const getServerClock = () => null;

export function useFreshness(
  updatedAt: string | null | undefined,
  refreshMs: number,
): Freshness {
  const clock = useSyncExternalStore(subscribeClock, getClock, getServerClock);

  if (clock == null) {
    // 시계를 모르는 동안에도 기준시각 자체는 알려 준다 — 표기는 시계와 무관하다.
    const parsed = updatedAt ? new Date(updatedAt) : null;
    const valid = parsed && !Number.isNaN(parsed.getTime()) ? parsed : null;
    return { at: valid, ageMs: null, stale: false };
  }
  return freshnessOf(updatedAt, refreshMs, clock);
}
