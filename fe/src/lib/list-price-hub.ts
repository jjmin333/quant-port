import {
  applyListPriceTicks,
  reconnectDelayMs,
  streamSymbolsKey,
  type ListPriceMap,
  type ListPriceTick,
} from "@/lib/quote-stream-state";
import { shouldKeepStream } from "@/lib/stream-awake";

/*
목록 가격 SSE를 **시장당 연결 하나**로 모은다 (2026-09-05 사용자 지적 6).

# 무엇을 재서 알았나

비로그인 홈 화면 하나를 열었을 때 열린 연결 넷(실측 2026-09-05, 네트워크 로그):

	symbols=000660,005380,005930                                    &assetType=domestic
	symbols=000950,004960,0091W0,019175,044780,047080,053950,073570 &assetType=domestic
	symbols=000660,005930,009150,069500,102110,122630,360750        &assetType=domestic
	symbols=ADBE,CPRT,FDX,KR,ORCL,SUNB                              &assetType=us

카드마다 `useLiveQuotes`를 달았기 때문이다(2026-09-04 요청 8: 「종목이 있는 카드는
전부 가격을 말한다」). 값을 모으는 훅은 하나인데 **연결은 카드 수만큼** 생겼다.

연결 하나는 서버에서 goroutine 하나와 nginx upstream 자리 하나다. 게다가 카드의
심볼 목록은 데이터가 도착하면서 바뀌므로(빈 배열 → 실제 목록), 그때마다 **끊고 다시
붙는다** — 사용자가 「SSE 연결이 자주 끊겼다가 다시 붙는다」고 적은 것이 이것이다.

# 무엇을 하나

시장(assetType)마다 허브 하나가 **구독자들의 심볼을 합쳐** 연결 하나를 연다. 위 넷은
국내 하나 · 해외 하나, 모두 둘이 된다.

합집합이 바뀌면 다시 열어야 하는데(서버는 열 때 심볼을 받는다), **바로 열지 않고
`RECONNECT_DEBOUNCE_MS`를 기다린다.** 카드들의 목록은 한 프레임에 다 채워지지 않아서,
기다리지 않으면 카드 수만큼 연결을 열었다 닫는다 — 줄이려던 것을 그 자리에서 되돌린다.

# 가격 맵은 시장 단위로 공유한다

구독자는 자기 심볼만 꺼내 쓰므로(`prices.get(symbol)`), 남의 종목이 맵에 함께 있어도
화면에는 영향이 없다. 대신 맵을 나눠 들면 같은 종목이 카드마다 다른 값이 될 수 있다 —
09-04에 고친 「한 값은 한 값이다」가 다시 갈라지는 자리다.

# 탭이 뒤로 가도 끊지 않는다 (사용자 요청 5)

판정은 `lib/stream-awake`가 한다. 여기서는 `visibilitychange`를 **앱 전체에 하나만**
듣는다 — 훅마다 듣던 것을 허브로 올리면서 얻은 덤이다.
*/

/** 합집합이 바뀐 뒤 실제로 다시 열기까지 기다리는 시간. */
export const RECONNECT_DEBOUNCE_MS = 60;

/** 개장 시각을 모를 때의 되묻기 주기 — `use-quote-detail`의 값과 같다. */
export const CLOSED_RECHECK_MS = 5 * 60_000;

export type HubState = {
  /** 이 시장에서 지금까지 받은 가격. 구독자는 자기 심볼만 꺼내 쓴다. */
  prices: ListPriceMap;
  /** 연결이 살아 있고 값이 흐르는가. */
  connected: boolean;
};

export type Subscriber = {
  symbols: readonly string[];
  notify: () => void;
};

/** 이 파일이 브라우저를 모르게 하는 이음매 — 시험은 가짜를 꽂는다. */
export type StreamPort = {
  open: (symbols: string[], assetType: string, handlers: StreamHandlers) => StreamHandle;
  now: () => number;
  setTimer: (fn: () => void, ms: number) => number;
  clearTimer: (id: number) => void;
};

export type StreamHandlers = {
  onSnapshot: (ticks: ListPriceTick[]) => void;
  onPrice: (tick: ListPriceTick) => void;
  onSession: (nextSessionAt: string | undefined) => void;
  onError: () => void;
};

export type StreamHandle = {
  close: () => void;
  /** 연결이 이미 죽었는가 — 다시 보일 때 되살릴지 정하는 데 쓴다. */
  isClosed: () => boolean;
};

const EMPTY_PRICES: ListPriceMap = new Map();

type Market = {
  assetType: string;
  subs: Set<Subscriber>;
  handle: StreamHandle | null;
  /** 지금 연결이 열려 있는 심볼 키. 합집합과 다르면 다시 열어야 한다. */
  openKey: string;
  state: HubState;
  nextSessionAt: string | undefined;
  /**
   * 이 시각까지는 **열지 않는다**(epoch ms). 마감 통보를 받으면 여기에 다음 개장
   * 시각이 들어간다.
   *
   * 없으면 어떻게 되는지 실측했다(2026-09-05, 토요일·장 마감): 카드가 뜨고 목록이
   * 채워질 때마다 `reconcile`이 「연결이 없네」를 보고 다시 열고, 서버는 그때마다
   * `session` 하나를 주고 끊는다. 홈 한 화면에 20초 동안 여섯 번이었다. 답이 「월요일
   * 08:00에 열립니다」로 정해져 있는데 여섯 번 물은 셈이다.
   */
  closedUntil: number | null;
  reconnectTimer: number | null;
  sessionTimer: number | null;
};

const markets = new Map<string, Market>();
let port: StreamPort | null = null;
/** 숨은 시각. 보이는 중이면 null. */
let hiddenSince: number | null = null;
let graceMs: number | undefined;
let awakeTimer: number | null = null;

function market(assetType: string): Market {
  let m = markets.get(assetType);
  if (!m) {
    m = {
      assetType,
      subs: new Set(),
      handle: null,
      openKey: "",
      state: { prices: EMPTY_PRICES, connected: false },
      nextSessionAt: undefined,
      closedUntil: null,
      reconnectTimer: null,
      sessionTimer: null,
    };
    markets.set(assetType, m);
  }
  return m;
}

function publish(m: Market, next: Partial<HubState>): void {
  m.state = { ...m.state, ...next };
  for (const sub of m.subs) sub.notify();
}

/** 이 시장의 구독자들이 원하는 심볼 전부. */
export function unionOf(subs: Iterable<Subscriber>): string[] {
  const seen = new Set<string>();
  for (const sub of subs) {
    for (const raw of sub.symbols) {
      const sym = raw.trim().toUpperCase();
      if (sym) seen.add(sym);
    }
  }
  return [...seen];
}

function awake(): boolean {
  return shouldKeepStream(hiddenSince, port ? port.now() : Date.now(), graceMs);
}

function closeStream(m: Market): void {
  m.handle?.close();
  m.handle = null;
  m.openKey = "";
  if (m.state.connected) publish(m, { connected: false });
}

function clearTimers(m: Market): void {
  if (!port) return;
  if (m.reconnectTimer !== null) {
    port.clearTimer(m.reconnectTimer);
    m.reconnectTimer = null;
  }
  if (m.sessionTimer !== null) {
    port.clearTimer(m.sessionTimer);
    m.sessionTimer = null;
  }
}

/** 지금 상태에 맞게 연결을 열거나 닫는다. 같은 심볼로 이미 열려 있으면 아무것도 안 한다. */
function reconcile(m: Market): void {
  if (!port) return;
  const symbols = unionOf(m.subs);
  const key = streamSymbolsKey(symbols);

  if (key === "" || !awake()) {
    closeStream(m);
    /*
      구독자가 하나도 없으면 가격도 버린다. 남겨 두면 다음에 같은 시장을 여는 화면이
      **몇 분 전 값**을 첫 프레임에 그린다 — 09-04에 고친 「시장 전환 때 앞 시장 값이
      새 목록에 얹힌다」와 같은 종류다.
    */
    if (m.subs.size === 0 && m.state.prices.size > 0) {
      publish(m, { prices: EMPTY_PRICES });
    }
    return;
  }

  /*
    마감 중이면 열지 않는다. 값이 바뀔 수 없는 동안 카드가 하나 뜰 때마다 연결을
    맺는 것이 실측에서 가장 잦은 낭비였다(위 `closedUntil` 참조). 개장 시각에는
    `sessionTimer`가 이 자리를 다시 부른다.
  */
  if (m.closedUntil !== null && port.now() < m.closedUntil) {
    closeStream(m);
    return;
  }

  if (m.handle && m.openKey === key && !m.handle.isClosed()) return;

  closeStream(m);
  m.openKey = key;
  m.handle = port.open(symbols, m.assetType, {
    onSnapshot: (ticks) => publish(m, { prices: applyListPriceTicks(m.state.prices, ticks), connected: true }),
    onPrice: (tick) => publish(m, { prices: applyListPriceTicks(m.state.prices, [tick]), connected: true }),
    onSession: (nextSessionAt) => {
      m.nextSessionAt = nextSessionAt;
      /*
        「모른다」를 무한정으로 굳히지 않는다 — 시각을 못 받으면 5분 뒤에는 다시
        열어 본다. 그 5분은 `scheduleSessionWake`가 쓰는 값과 같아야 한다.
      */
      m.closedUntil =
        port!.now() + (reconnectDelayMs(nextSessionAt, port!.now()) ?? CLOSED_RECHECK_MS);
      closeStream(m);
      scheduleSessionWake(m);
    },
    /*
      onerror는 「끊겼다」와 「재연결 중」을 구분해 주지 않는다. 여기서는 폴링을
      되살리도록 `connected`만 내린다 — 재연결은 EventSource가 알아서 한다.
    */
    onError: () => publish(m, { connected: false }),
  });
}

/** 합집합이 바뀌었다 — 곧바로가 아니라 한 박자 뒤에 다시 연다(머리글 참조). */
function scheduleReconcile(m: Market): void {
  if (!port || m.reconnectTimer !== null) return;
  m.reconnectTimer = port.setTimer(() => {
    m.reconnectTimer = null;
    reconcile(m);
  }, RECONNECT_DEBOUNCE_MS);
}

/**
 * 마감 통보를 받았다 — 다음 개장 시각에 스스로 깨어난다.
 *
 * 시각을 못 받으면 5분마다 되묻는다. 「모른다」를 「닫혔다」로 굳히면 화면이 개장을
 * 영영 못 알아챈다.
 */
function scheduleSessionWake(m: Market): void {
  if (!port) return;
  if (m.sessionTimer !== null) port.clearTimer(m.sessionTimer);
  m.sessionTimer = port.setTimer(() => {
    m.sessionTimer = null;
    m.closedUntil = null;
    reconcile(m);
  }, reconnectDelayMs(m.nextSessionAt, port.now()) ?? CLOSED_RECHECK_MS);
}


/** 허브가 쓸 이음매를 꽂는다. 브라우저에서는 앱이 뜨면서 한 번 부른다. */
export function installStreamPort(next: StreamPort, backgroundGraceMs?: number): void {
  port = next;
  graceMs = backgroundGraceMs;
}

/**
 * 탭이 보이거나 숨었다.
 *
 * 숨는 것만으로는 끊지 않고, 유예가 지나면 그때 놓는다(`lib/stream-awake`).
 * 다시 보일 때는 **죽어 있는 연결만** 되살린다 — 살아 있는 것을 다시 열면
 * 스냅샷을 한 벌 더 받는 셈이라, 끊지 않으려고 한 일을 스스로 되돌린다.
 */
export function setVisible(visible: boolean): void {
  if (!port) return;
  if (visible) {
    hiddenSince = null;
    if (awakeTimer !== null) {
      port.clearTimer(awakeTimer);
      awakeTimer = null;
    }
    for (const m of markets.values()) {
      if (!m.handle || m.handle.isClosed()) reconcile(m);
    }
    return;
  }

  if (hiddenSince === null) hiddenSince = port.now();
  if (awakeTimer !== null) return;
  awakeTimer = port.setTimer(() => {
    awakeTimer = null;
    if (awake()) return;
    for (const m of markets.values()) {
      clearTimers(m);
      closeStream(m);
    }
  }, graceMs ?? 3 * 60 * 60 * 1000);
}

/** 이 시장의 지금 상태. `useSyncExternalStore`의 `getSnapshot`이 부른다. */
export function getHubState(assetType: string): HubState {
  return markets.get(assetType)?.state ?? EMPTY_STATE;
}

export const EMPTY_STATE: HubState = { prices: EMPTY_PRICES, connected: false };

/** 심볼 목록을 등록한다. 돌려받은 함수를 부르면 구독이 끝난다. */
export function subscribeListPrices(
  assetType: string,
  symbols: readonly string[],
  notify: () => void,
): () => void {
  const m = market(assetType);
  const sub: Subscriber = { symbols, notify };
  m.subs.add(sub);
  scheduleReconcile(m);
  return () => {
    m.subs.delete(sub);
    if (m.subs.size === 0) clearTimers(m);
    scheduleReconcile(m);
  };
}

/** 시험용 — 모든 시장을 초기 상태로 되돌린다. */
export const __testing = {
  reset: () => {
    for (const m of markets.values()) {
      clearTimers(m);
      m.handle?.close();
    }
    markets.clear();
    port = null;
    hiddenSince = null;
    graceMs = undefined;
    awakeTimer = null;
  },
  openKey: (assetType: string) => markets.get(assetType)?.openKey ?? "",
  subCount: (assetType: string) => markets.get(assetType)?.subs.size ?? 0,
};
