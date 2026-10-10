"use client";

import { useSyncExternalStore } from "react";
import type { IndexGroupMeta, MarketIndex } from "@/lib/quotes-types";
import { fetchMarketIndicesClient } from "@/lib/market-api";
import { mergeMarketIndices, MARKET_INDEX_PLACEHOLDERS } from "@/lib/market-indices-catalog";
import { isPageVisible } from "@/hooks/use-page-visible";
import { toUserMessage } from "@/lib/error-message";

/** 지수 폴링 주기. 「낡음」 판정의 기준이기도 해서 밖으로 연다(`lib/data-freshness.ts`). */
export const INDEX_POLL_MS = 30_000;
/**
 * 서버의 지수 갱신 회차(`be/internal/service/market_index_store.go` marketIndexPollInterval).
 * 「낡음」은 서버가 확인한 시각(`checkedAt`)을 이 주기로 잰다 — 화면 폴링 주기가 아니다.
 */
export const INDEX_SERVER_LOOP_MS = 60_000;

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
  /** 값이 마지막으로 **바뀐** 시각. 주말·밤에는 몇 분씩 멈춘다. */
  updatedAt: string | null;
  /** 서버가 마지막으로 **확인한** 시각. 「낡은 값」 판정은 이쪽을 본다. */
  checkedAt: string | null;
  groups: Record<string, IndexGroupMeta>;
  /**
   * 한 번이라도 값을 받아 봤는가. 「아직 못 받았다」와 「받았는데 비었다」를 가른다.
   */
  loaded: boolean;
  /**
   * 마지막 시도가 실패했다면 사용자 문구. 성공하면 지워진다.
   *
   * 예전에는 이 칸이 없어서 `catch {}`가 **실패를 삼켰다.** 스냅샷은 초기값
   * 그대로 남고 `updatedAt`은 null이니, 화면은 「갱신 중…」과 스켈레톤을
   * 영원히 그렸다 — 사용자에게는 새로고침 말고 빠져나갈 길이 없었다
   * (2026-09-20 점검 요청 3).
   */
  error: string | null;
};

const EMPTY: Snapshot = {
  indices: MARKET_INDEX_PLACEHOLDERS,
  updatedAt: null,
  checkedAt: null,
  groups: {},
  loaded: false,
  error: null,
};

let snapshot: Snapshot = EMPTY;
/**
 * 서버가 미리 받아 둔 첫 스냅샷. 첫 구독자가 붙기 전에 `seedMarketIndices`가 넣는다.
 *
 * 모듈 전역인 이유는 스냅샷 자체가 그렇기 때문이다 — 소비자가 몇이든 한 벌이다.
 */
let serverSeed: Snapshot | null = null;
const subscribers = new Set<() => void>();
let timer: number | null = null;
let visibilityBound = false;

function publish(next: Snapshot) {
  snapshot = next;
  for (const notify of subscribers) notify();
}

/**
 * SSR이 받아 온 지수로 첫 스냅샷을 채운다 (`lib/home-api.server.ts`).
 *
 * **이미 실제 값이 있으면 덮지 않는다.** 화면을 오가며 스트립이 다시 마운트될
 * 때마다 서버 시점의 값으로 되돌아가면, 방금 폴링이 받아 온 숫자가 과거로
 * 튄다. 씨앗은 **아직 아무것도 없을 때만** 의미가 있다.
 */
export function seedMarketIndices(seed: {
  indices: MarketIndex[];
  updatedAt: string | null;
  checkedAt?: string | null;
  groups: Record<string, IndexGroupMeta>;
}): void {
  if (seed.indices.length === 0) return;
  // 같은 씨앗으로 두 번 부르면(StrictMode의 이중 렌더) 두 번째는 할 일이 없다.
  if (serverSeed && serverSeed.updatedAt === seed.updatedAt) return;

  const next: Snapshot = {
    indices: mergeMarketIndices(seed.indices),
    updatedAt: seed.updatedAt,
    checkedAt: seed.checkedAt ?? null,
    groups: seed.groups,
    loaded: true,
    error: null,
  };
  serverSeed = next;

  /*
    **렌더 중에 불린다.** 그래서 `publish`를 쓰지 않는다 — 구독자에게 알리는 것은
    남의 컴포넌트를 렌더 도중에 setState 하는 일이다.

    알릴 필요도 없다. 구독자가 하나도 없다는 것은 **아직 아무도 이 값을 그린 적이
    없다**는 뜻이고, 곧 이어질 첫 렌더가 `getSnapshot()`으로 이 값을 그대로 읽는다.
    이미 누가 보고 있다면 그쪽이 들고 있는 값이 더 새것이므로 건드리지 않는다.
  */
  if (subscribers.size === 0 && !snapshot.loaded) snapshot = next;
}

async function load() {
  /*
    **첫 적재는 숨어 있어도 한다** (2026-09-20 점검에서 실측).

    예전에는 이 가드가 첫 회차까지 막았다. 백그라운드 탭으로 홈을 열면
    (Cmd/Ctrl+클릭, 세션 복원, 알림에서 열기) `load()`가 즉시 돌아가 버려
    사용자가 그 탭을 **누를 때까지** 스켈레톤과 「갱신 중…」만 서 있었다.
    브라우저 패널이 hidden인 채로 재현했다: 지수 0종. visibility를 켜자마자
    13종이 찼다.

    가드의 본뜻은 「안 보는 탭에 **폴링**을 돌리지 않는다」이지 「안 보는 탭에는
    내용을 만들지 않는다」가 아니다. 그래서 회차 갱신만 막는다 — 첫 적재는
    한 번뿐이고, 그 한 번이 없으면 화면이 아예 성립하지 않는다.
  */
  if (snapshot.loaded && !isPageVisible()) return;
  try {
    const { indices, updatedAt, checkedAt, groups } = await fetchMarketIndicesClient();
    publish({
      indices: mergeMarketIndices(indices),
      // 빈 응답 필드로 직전 값을 지우지 않는다 — 폴링이 한 번 비어도 화면은 유지된다.
      updatedAt: updatedAt || snapshot.updatedAt,
      checkedAt: checkedAt || snapshot.checkedAt,
      groups: Object.keys(groups).length > 0 ? groups : snapshot.groups,
      loaded: true,
      error: null,
    });
  } catch (err) {
    /*
      **값은 지키고 실패는 알린다.** 보고 있던 지수를 지우면 갱신 한 번의 실패가
      값을 통째로 없애는 일이 된다(`lib/screen-refresh` 머리글의 규칙). 대신
      `error`를 올려, 화면이 「갱신 중」이 아니라 「갱신 실패 · 다시 시도」를
      그릴 수 있게 한다.
    */
    publish({ ...snapshot, error: toUserMessage(err, "지수를 불러오지 못했습니다.") });
  }
}

/** 화면의 「다시 시도」. 오류를 지우고 한 번 더 부른다. */
export function retryMarketIndices(): void {
  publish({ ...snapshot, error: null });
  void load();
}

function onVisibilityChange() {
  if (isPageVisible()) void load();
}

function start() {
  if (timer != null) return;
  void load();
  timer = window.setInterval(load, INDEX_POLL_MS);
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

/**
 * 서버 렌더와 **hydration**이 함께 읽는 값.
 *
 * 예전에는 무조건 `EMPTY`였다 — 서버에는 폴링도 스냅샷도 없으니 자리표시자로
 * 그리고 클라이언트에서 채운다는 뜻이었고, 그래서 홈의 첫 HTML에 지수가 한 종도
 * 없었다. 이제는 `page.tsx`가 서버에서 받아 온 것을 `seedMarketIndices`로 넣어
 * 둔다(`lib/home-api.server.ts`).
 *
 * **hydration에서도 같은 값을 돌려줘야 한다.** React는 이 함수를 서버 렌더와
 * hydration 양쪽에서 부르고, 두 결과가 다르면 그게 곧 불일치다. `serverSeed`는
 * 렌더 중에 채워지므로 양쪽에서 같다 — 채우는 쪽이 같은 컴포넌트의 같은 props다.
 */
function getServerSnapshot(): Snapshot {
  return serverSeed ?? EMPTY;
}
