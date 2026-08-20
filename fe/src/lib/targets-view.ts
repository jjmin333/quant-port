"use client";

import { useSyncExternalStore } from "react";

/**
 * 목표가 목록 보기 전환 (진행축 행 ↔ 카드 그리드).
 *
 * `row`가 기본 — 한 종목이 한 줄이라 열 개 스무 개를 위아래로 훑을 때 가장 빠르고,
 * 등록가·현재가·목표가·손절가가 같은 축 위에 나란히 놓인다. `card`는 종목 수가 적어
 * 목표선까지의 **궤적**을 크게 보고 싶을 때.
 *
 * 보유종목 화면의 `holdings-view.ts`와 같은 형태. `localStorage` + 모듈 스코프 구독자
 * 집합이고 `storage` 이벤트는 쓰지 않는다 — 그건 다른 탭에서만 발생해서 같은 화면의
 * 두 구독자(툴바 ↔ 목록)를 잇지 못한다.
 */

export type TargetsView = "row" | "card";

const VIEW_KEY = "quant-port-targets-view";
const listeners = new Set<() => void>();

function subscribe(onChange: () => void) {
  listeners.add(onChange);
  return () => {
    listeners.delete(onChange);
  };
}

function getSnapshot(): TargetsView {
  return window.localStorage.getItem(VIEW_KEY) === "card" ? "card" : "row";
}

export function setTargetsView(next: TargetsView) {
  window.localStorage.setItem(VIEW_KEY, next);
  for (const listener of listeners) listener();
}

/**
 * 저장된 보기. 서버 스냅샷이 따로 있어야 하이드레이션이 어긋나지 않는다 —
 * 서버는 항상 기본값 `row`를 그린다.
 */
export function useTargetsView(): TargetsView {
  return useSyncExternalStore(subscribe, getSnapshot, () => "row");
}
