/*
**지금 보고 있는(또는 마지막으로 본) 종목**을 앱 전체에 하나만 적어 둔다
(2026-09-15 사용자 요청 4).

# 무엇을 위한 자리인가

탭이 숨으면 유예 뒤에 가격 스트림을 전부 놓는다(`lib/stream-awake`). 그것이 요청 3이다.
그런데 증권사 앱은 한 가지를 예외로 둔다 — 사용자가 **한 종목을 열어 놓은 채** 다른
화면으로 가면, 그 종목의 가격과 등락률만은 계속 흐른다.

우리 화면에서 그것이 서는 자리는 **브라우저 탭 제목**이다(사용자가 보낸 사진).
`$141.91 -1.98% | 오라클` — 탭 제목은 그 탭이 **안 보이는 동안에도 탭 스트립에
그대로 보인다.** 그래서 숨은 탭의 구독을 0으로 떨어뜨리지 않고 1로 떨어뜨린다:
보유 서른 종목을 띄워 둔 탭이 숨으면 서른이 아니라 하나가 남는다.

# 두 가지를 함께 센다 — 「지금 열려 있다」와 「마지막으로 봤다」

  · **active** — 종목 상세 화면이나 모달이 떠 있는 동안. 화면이 사라지면 함께 빠진다.
  · **lastViewed** — 한 번 열면 **탭이 닫힐 때까지 남는다**(2026-09-15 사용자 결정:
    「열자마자 뜨고 × 까지 유지」). 다른 메뉴로 옮겨 가도 탭 제목은 그 종목을 계속
    말한다.

둘을 나눈 이유는 **지우는 기준이 다르기 때문**이다. active는 화면의 생애를 따르고,
lastViewed는 탭의 생애를 따른다. 한 칸에 합치면 모달을 닫는 순간 제목이 돌아가
버려 「× 까지 유지」가 깨진다.

# 새로고침을 견딘다 — sessionStorage

「탭이 닫힐 때까지」를 글자 그대로 지키려면 F5를 견뎌야 한다. `sessionStorage`는
**탭 하나에만 살고 탭과 함께 죽는다** — 여기서 원하는 수명과 정확히 같다
(`localStorage`였다면 어제 본 종목이 오늘 새 탭의 제목이 된다).

읽기는 **브라우저에서만** 한다. 서버 렌더에는 이 값이 없으므로, 첫 HTML과 첫
클라이언트 렌더가 어긋나지 않게 이펙트에서 한 번 불러 올린다(`hydrate`).

# 왜 전역 모듈인가 (React 상태가 아니라)

읽는 쪽이 둘이다 — 탭 제목을 쓰는 컴포넌트와, React 밖에 있는 가격 허브
(`lib/list-price-hub`). 값을 컨텍스트에 두면 허브가 그것을 보려고 React에 의존해야
하고, 그러면 시험이 DOM을 필요로 한다(이 저장소의 vitest는 node 환경이다).
*/

export type PinnedSymbol = {
  symbol: string;
  /** `domestic` | `us` — 허브가 시장별로 갈라 열기 때문에 함께 적는다. */
  assetType: string;
};

const pins = new Map<symbol, PinnedSymbol>();
const listeners = new Set<() => void>();

/** 한 번 열면 탭이 닫힐 때까지 남는 종목. 탭 제목이 이것을 말한다. */
let lastViewed: PinnedSymbol | null = null;

const STORAGE_KEY = "quant-port-last-viewed-symbol";

function notify(): void {
  for (const fn of listeners) fn();
}

function normalize(symbol: string): string {
  return symbol.trim().toUpperCase();
}

function persist(): void {
  try {
    if (lastViewed) sessionStorage.setItem(STORAGE_KEY, JSON.stringify(lastViewed));
    else sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // 시크릿 창·저장소 차단. 탭 제목이 새로고침을 못 견딜 뿐 아무것도 안 깨진다.
  }
}

/**
 * 새로고침 전에 보던 종목을 되살린다. 브라우저에서 앱이 뜰 때 한 번 부른다.
 *
 * **이미 값이 있으면 아무것도 안 한다** — 되살리기보다 지금 화면이 언제나 우선이다
 * (링크로 종목을 열면서 들어온 경우, 그 종목이 먼저 등록된다).
 */
export function hydrateLastViewed(): void {
  if (lastViewed || typeof sessionStorage === "undefined") return;
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return;
    const parsed = JSON.parse(raw) as Partial<PinnedSymbol>;
    const symbol = normalize(parsed?.symbol ?? "");
    if (!symbol) return;
    lastViewed = { symbol, assetType: String(parsed?.assetType ?? "") };
    notify();
  } catch {
    // 깨진 값 하나로 화면을 막지 않는다.
  }
}

/**
 * 이 종목을 「보고 있다」고 적는다. 돌려받은 함수를 부르면 **active에서만** 빠진다 —
 * 마지막으로 본 종목은 그대로 남는다(탭 제목은 계속 이 종목을 말한다).
 *
 * 빈 심볼은 등록하지 않는다 — 화면이 종목을 아직 못 정한 상태(로딩 중)에서 빈
 * 문자열로 부르는 일이 흔하고, 그것이 핀 하나로 세어지면 「무엇을 보고 있는가」가
 * 거짓이 된다.
 */
export function pinSymbol(symbol: string, assetType: string): () => void {
  const sym = normalize(symbol);
  if (sym === "") return () => {};
  const token = Symbol(sym);
  const entry: PinnedSymbol = { symbol: sym, assetType };
  pins.set(token, entry);
  if (lastViewed?.symbol !== sym || lastViewed.assetType !== assetType) {
    lastViewed = entry;
    persist();
  }
  notify();
  return () => {
    if (pins.delete(token)) notify();
  };
}

/** 마지막으로 연 종목. 탭 제목이 읽는 값이다. */
export function lastViewedSymbol(): PinnedSymbol | null {
  return lastViewed;
}

/**
 * 이 시장에서 지금 살려 둘 종목들 — **열려 있는 것 + 마지막으로 본 것**.
 *
 * 숨은 탭이 유예를 넘겼을 때 허브가 이 목록으로 구독을 좁힌다.
 */
export function pinnedSymbolsFor(assetType: string): string[] {
  const seen = new Set<string>();
  for (const pin of pins.values()) {
    if (pin.assetType === assetType) seen.add(pin.symbol);
  }
  if (lastViewed && lastViewed.assetType === assetType) seen.add(lastViewed.symbol);
  return [...seen];
}

/** 이 종목이 지금 열려 있는가 — 상세 화면의 체결 스트림이 이 답으로 살아남는다. */
export function isPinned(symbol: string): boolean {
  const sym = normalize(symbol);
  if (sym === "") return false;
  for (const pin of pins.values()) {
    if (pin.symbol === sym) return true;
  }
  return false;
}

/** 핀이 바뀌면 알려 준다. 돌려받은 함수를 부르면 구독이 끝난다. */
export function subscribePinned(fn: () => void): () => void {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

/** 시험용 — 등록된 핀을 전부 지운다. */
export const __testing = {
  reset: () => {
    pins.clear();
    listeners.clear();
    lastViewed = null;
  },
  count: () => pins.size,
};
