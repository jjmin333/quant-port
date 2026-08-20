"use client";

import { useSyncExternalStore } from "react";

/**
 * 종목 랭킹 보기 밀도 (08-11 사용자 결정).
 *
 * `compact`가 기본 — 종목과 점수만 본다. `wide`는 훑으면서 미리 보고 싶을 때 켠다.
 *
 * 이 상태가 패널 밖으로 나온 이유(08-12): `wide`에서는 **좌측 레일 폭 자체가 넓어진다**.
 * 300px 레일에 순위·아바타·이름·스파크라인·등락·점수를 다 넣으면 이름 몫으로 20px대만
 * 남아 종목명이 한 글자로 잘렸다(사용자 스크린샷). 컬럼 폭은 `PageColumns`가 쥐고
 * 있으므로 페이지와 패널이 같은 값을 봐야 한다.
 *
 * `localStorage` + 모듈 스코프 구독자 집합으로 구현한다. `storage` 이벤트는 **다른 탭**
 * 에서만 발생해서 같은 화면의 두 구독자를 잇지 못한다.
 */

export type RankingView = "compact" | "wide";

const VIEW_KEY = "quant-port-ranking-view";
const listeners = new Set<() => void>();

function subscribe(onChange: () => void) {
  listeners.add(onChange);
  return () => {
    listeners.delete(onChange);
  };
}

function getSnapshot(): RankingView {
  return window.localStorage.getItem(VIEW_KEY) === "wide" ? "wide" : "compact";
}

export function setRankingView(next: RankingView) {
  window.localStorage.setItem(VIEW_KEY, next);
  for (const listener of listeners) listener();
}

/**
 * 저장된 보기 밀도.
 *
 * `useState` 초기화에서 localStorage를 읽으면 서버 렌더(값 없음)와 어긋나 하이드레이션이
 * 깨지고, 이펙트에서 setState로 덮으면 렌더가 한 번 더 돈다. `useSyncExternalStore`는
 * 서버 스냅샷을 따로 줄 수 있어 둘 다 피한다 — 서버는 항상 기본값 `compact`를 그린다.
 */
export function useRankingView(): RankingView {
  return useSyncExternalStore(subscribe, getSnapshot, () => "compact");
}
