"use client";

import { useSyncExternalStore } from "react";
import type { EarningsRangeMode } from "@/lib/earnings-types";

/**
 * 실적 화면의 보기 단위 (월 ↔ 주 ↔ 일).
 *
 * `month`가 기본 — 처음 들어온 사람에게 「이 달 어디가 격전지인가」를 먼저 보여준다.
 * `week`는 실적 시즌에 쓸모 있고(발표가 주 단위로 몰린다), `day`는 하루씩 훑을 때다.
 *
 * `targets-view.ts`·`holdings-view.ts`와 **같은 형태**다: `localStorage` + 모듈
 * 스코프 구독자 집합, `storage` 이벤트는 쓰지 않는다(그건 다른 탭에서만 발생해
 * 같은 화면의 두 구독자를 잇지 못한다).
 *
 * 이펙트에서 `localStorage`를 읽어 `setState`하지 않는 이유는 그게 캐스케이드
 * 렌더이고(react-hooks/set-state-in-effect), 렌더 중에 바로 읽으면 서버가 그린
 * 것과 어긋나 하이드레이션이 깨지기 때문이다. `useSyncExternalStore`는 서버
 * 스냅샷을 따로 받아 둘 다 피한다.
 */

const MODE_KEY = "quant-port-earnings-range-mode";
const listeners = new Set<() => void>();

function subscribe(onChange: () => void) {
  listeners.add(onChange);
  return () => {
    listeners.delete(onChange);
  };
}

function getSnapshot(): EarningsRangeMode {
  const saved = window.localStorage.getItem(MODE_KEY);
  return saved === "week" || saved === "day" ? saved : "month";
}

export function setEarningsRangeMode(next: EarningsRangeMode) {
  window.localStorage.setItem(MODE_KEY, next);
  for (const listener of listeners) listener();
}

/** 저장된 보기 단위. 서버는 항상 기본값 `month`를 그린다. */
export function useEarningsRangeMode(): EarningsRangeMode {
  return useSyncExternalStore(subscribe, getSnapshot, () => "month");
}
