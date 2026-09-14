"use client";

import { addWatchlistClient, fetchWatchlistClient, removeWatchlistClient } from "@/lib/market-api";

/*
관심종목 심볼 집합을 **앱에 하나만** 둔다.

# 고친 것

랭킹 목록에서 하트를 누르고 다른 화면에 갔다가 돌아오면, 실제로는 담겨 있는데
하트가 비어 있었다. 화면마다 관심 목록을 따로 받아 오면서 **받기 전의 빈 집합으로
먼저 그렸기** 때문이다. 특히 마켓 레이더는 그 요청을 `requestIdleCallback`(타임아웃
2초)으로 미뤄 두고 있어서, 목록은 언제나 「관심 없음」 상태로 한 번 그려진 뒤
뒤늦게 채워졌다 — 빠르게 돌아올수록 그 빈 프레임이 눈에 남는다.

집합을 모듈에 두면 두 번째 마운트는 **네트워크를 기다리지 않는다.** 이미 아는
답으로 첫 프레임을 그리고, 갱신은 뒤에서 조용히 한다.

# 규칙

  · `ensureLoaded()`는 같은 시점의 중복 요청을 하나로 접는다.
  · 토글은 **먼저 반영하고 나중에 보낸다**(낙관적). 실패하면 되돌린다. 어느 화면에서
    누르든 구독 중인 모든 화면이 같은 프레임에 바뀐다.
  · 로그아웃하면 `resetWatchlist()`로 통째로 버린다 — 캐시 키에 사용자가 없다.
*/

let symbols = new Set<string>();
let loaded = false;
let loadedAt = 0;
let inflight: Promise<Set<string>> | null = null;

const listeners = new Set<() => void>();

/** 이보다 오래된 집합은 다시 받아 온다(값은 그대로 쓰면서 뒤에서 갱신). */
const STALE_AFTER_MS = 60_000;

function normalize(symbol: string): string {
  return symbol.trim().toUpperCase();
}

function emit() {
  for (const listener of listeners) listener();
}

export function subscribeWatchlist(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/**
 * 지금 아는 집합. **요청하지 않는다** — 첫 렌더가 이 값으로 그려지고, 로딩은
 * `isWatchlistLoaded()`가 따로 말한다.
 */
export function getWatchlistSymbols(): Set<string> {
  return symbols;
}

export function isWatchlistLoaded(): boolean {
  return loaded;
}

export function isWatchlisted(symbol: string): boolean {
  return symbols.has(normalize(symbol));
}

/** 로그인·로그아웃 등 "누구인지"가 바뀌었을 때. */
export function resetWatchlist(): void {
  symbols = new Set();
  loaded = false;
  loadedAt = 0;
  inflight = null;
  emit();
}

function applySet(next: Set<string>) {
  symbols = next;
  loaded = true;
  loadedAt = Date.now();
  emit();
}

/**
 * 필요하면 받아 온다. 이미 최신이면 아무 일도 하지 않는다.
 *
 * `force`는 쓰기 직후처럼 서버가 정답을 갖고 있는 것이 확실할 때만.
 */
export function ensureWatchlist(force = false): Promise<Set<string>> {
  if (!force && loaded && Date.now() - loadedAt < STALE_AFTER_MS) {
    return Promise.resolve(symbols);
  }
  if (inflight) return inflight;

  inflight = fetchWatchlistClient()
    .then((list) => {
      applySet(new Set(list.map((item) => normalize(item.symbol))));
      return symbols;
    })
    .catch(() => {
      // 실패는 「관심이 없다」가 아니다. 아는 값을 그대로 둔다 — 여기서 비우면
      // 네트워크가 한 번 튈 때마다 하트가 전부 꺼진다.
      if (!loaded) applySet(new Set());
      return symbols;
    })
    .finally(() => {
      inflight = null;
    });

  return inflight;
}

/** 서버 왕복 없이 집합만 고친다(낙관적 갱신·되돌리기). */
export function setWatchlistLocal(symbol: string, watchlisted: boolean): void {
  const sym = normalize(symbol);
  if (!sym) return;
  if (symbols.has(sym) === watchlisted) return;
  const next = new Set(symbols);
  if (watchlisted) next.add(sym);
  else next.delete(sym);
  symbols = next;
  loaded = true;
  emit();
}

/**
 * 담기/빼기. 화면에 먼저 반영하고 서버로 보낸다 — 5초 폴링이 도는 화면에서
 * 왕복을 기다리면 그 사이 갱신이 눌린 상태를 덮어써 「눌렀는데 안 눌린」다.
 *
 * 실패하면 원래대로 되돌리고 예외를 다시 던진다(부르는 쪽이 알아야 한다).
 */
export async function toggleWatchlistSymbol(symbol: string): Promise<boolean> {
  const sym = normalize(symbol);
  if (!sym) return false;
  const next = !symbols.has(sym);
  setWatchlistLocal(sym, next);
  try {
    if (next) await addWatchlistClient(sym);
    else await removeWatchlistClient(sym);
    loadedAt = Date.now();
    return next;
  } catch (e) {
    setWatchlistLocal(sym, !next);
    throw e;
  }
}
