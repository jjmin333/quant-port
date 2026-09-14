"use client";

import { useSyncExternalStore } from "react";
import type { IndexGroupMeta, MarketIndex } from "@/lib/quotes-types";
import { fetchMarketIndicesClient } from "@/lib/market-api";
import { mergeMarketIndices, MARKET_INDEX_PLACEHOLDERS } from "@/lib/market-indices-catalog";
import { isPageVisible } from "@/hooks/use-page-visible";

const POLL_MS = 30_000;

/**
 * 지수 스냅샷은 **화면에 몇 군데서 쓰이든 한 벌만 받는다**.
 *
 * 예전에는 훅이 자기 `setInterval`을 들었다. 홈에 소비자가 하나(지수 스트립)일 때는
 * 티가 안 났지만, 브리핑 카드가 같은 값을 쓰기 시작하면 30초마다 같은 요청이 두 번
 * 나가고 **두 카드가 서로 다른 응답을 들고 있게 된다** — 폴링 시점이 어긋나면 스트립의
 * 코스피와 브리핑의 코스피가 한 틱씩 다르다. 지금 고치려는 증상이 정확히 그것이라,
 * 소비자를 늘리면서 폴링을 늘리지 않는 게 요점이다.
 *
 * 그래서 모듈 하나에 스냅샷과 구독자 목록을 두고, **구독자가 처음 생길 때 폴링을
 * 시작하고 마지막이 떠날 때 멈춘다**. 늦게 붙은 구독자는 마지막 스냅샷을 즉시 받으므로
 * 카드가 빈 채로 한 프레임 뜨는 일도 없다.
 */
type Snapshot = {
  indices: MarketIndex[];
  updatedAt: string | null;
  groups: Record<string, IndexGroupMeta>;
};

const EMPTY: Snapshot = {
  indices: MARKET_INDEX_PLACEHOLDERS,
  updatedAt: null,
  groups: {},
};

let snapshot: Snapshot = EMPTY;
const subscribers = new Set<() => void>();
let timer: number | null = null;
let visibilityBound = false;

function publish(next: Snapshot) {
  snapshot = next;
  for (const notify of subscribers) notify();
}

async function load() {
  if (!isPageVisible()) return;
  try {
    const { indices, updatedAt, groups } = await fetchMarketIndicesClient();
    publish({
      indices: mergeMarketIndices(indices),
      // 빈 응답 필드로 직전 값을 지우지 않는다 — 폴링이 한 번 비어도 화면은 유지된다.
      updatedAt: updatedAt || snapshot.updatedAt,
      groups: Object.keys(groups).length > 0 ? groups : snapshot.groups,
    });
  } catch {
    // keep last snapshot
  }
}

function onVisibilityChange() {
  if (isPageVisible()) void load();
}

function start() {
  if (timer != null) return;
  void load();
  timer = window.setInterval(load, POLL_MS);
  if (!visibilityBound) {
    document.addEventListener("visibilitychange", onVisibilityChange);
    visibilityBound = true;
  }
}

function stop() {
  if (timer == null) return;
  window.clearInterval(timer);
  timer = null;
  if (visibilityBound) {
    document.removeEventListener("visibilitychange", onVisibilityChange);
    visibilityBound = false;
  }
}

/**
 * `useSyncExternalStore`를 쓰는 이유.
 *
 * 이 훅이 읽는 값은 React 밖(모듈 스코프)에 산다. `useState` + `useEffect`로 흉내
 * 내면 **마운트 직후 한 번 더 렌더가 돈다** — 구독을 걸기 전에 도착해 있던 스냅샷을
 * 이펙트 안에서 setState로 맞춰 줘야 하기 때문이고, 그건 정확히 lint가 막는 모양이다
 * (`react-hooks/set-state-in-effect`). 여기서는 첫 렌더가 곧장 최신 스냅샷을 읽는다.
 *
 * `getSnapshot`이 돌려주는 참조는 `publish`가 새 객체를 넣을 때만 바뀐다 — 매번 새
 * 객체를 만들면 React가 무한히 다시 읽는다.
 */
export function useMarketIndices(): Snapshot {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

function subscribe(notify: () => void): () => void {
  const entry = () => notify();
  subscribers.add(entry);
  start();
  return () => {
    subscribers.delete(entry);
    if (subscribers.size === 0) stop();
  };
}

function getSnapshot(): Snapshot {
  return snapshot;
}

// 서버 렌더에는 폴링도 스냅샷도 없다. 자리표시자로 그리고 클라이언트에서 채운다.
function getServerSnapshot(): Snapshot {
  return EMPTY;
}
