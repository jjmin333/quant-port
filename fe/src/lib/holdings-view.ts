"use client";

import { useSyncExternalStore } from "react";

/**
 * 보유종목 목록 보기 전환 (표 ↔ 카드).
 *
 * `table`이 기본 — 한 종목이 한 줄(34px)이라 스무 종목도 한 화면에 들어오고, 평단·손익·
 * 비중까지 같은 눈높이에서 비교된다. `card`는 종목 수가 적거나 모바일에서 한 종목씩
 * 크게 보고 싶을 때.
 *
 * `localStorage` + 모듈 스코프 구독자 집합. `storage` 이벤트는 **다른 탭**에서만 발생해서
 * 같은 화면의 두 구독자(툴바 ↔ 목록)를 잇지 못한다 — `ranking-view.ts`와 같은 이유.
 */

export type HoldingsView = "table" | "card";

const VIEW_KEY = "quant-port-holdings-view";
const listeners = new Set<() => void>();

function subscribe(onChange: () => void) {
  listeners.add(onChange);
  return () => {
    listeners.delete(onChange);
  };
}

function getSnapshot(): HoldingsView {
  return window.localStorage.getItem(VIEW_KEY) === "card" ? "card" : "table";
}

export function setHoldingsView(next: HoldingsView) {
  window.localStorage.setItem(VIEW_KEY, next);
  for (const listener of listeners) listener();
}

/**
 * 저장된 보기.
 *
 * `useState` 초기화에서 localStorage를 읽으면 서버 렌더(값 없음)와 어긋나 하이드레이션이
 * 깨진다. `useSyncExternalStore`는 서버 스냅샷을 따로 줄 수 있어 그걸 피한다 —
 * 서버는 항상 기본값 `table`을 그린다.
 */
export function useHoldingsView(): HoldingsView {
  return useSyncExternalStore(subscribe, getSnapshot, () => "table");
}
