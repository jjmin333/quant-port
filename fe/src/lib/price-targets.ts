"use client";

import { useCallback, useEffect, useSyncExternalStore } from "react";
import { isSampleModeActive } from "@/components/common/sample-mode";
import { PortfolioFetchError } from "@/lib/api-client";
import { toUserMessage } from "@/lib/error-message";
import {
  createPriceTargetClient,
  deletePriceTargetClient,
  fetchPriceTargetsClient,
  updatePriceTargetClient,
  type PriceTargetRecord,
} from "@/lib/price-targets-api";

const LEGACY_STORAGE_KEY = "quant-port-targets";

export type PriceTarget = {
  id: string;
  symbol: string;
  targetPrice: number;
  /** 2차 목표. 한 단계만 잡았으면 없다. */
  targetPrice2?: number;
  /** 논지가 틀렸다고 인정하는 자리. */
  stopPrice?: number;
  /** 등록 시점 주가. 서버가 잰 값이라 목표를 고쳐도 그대로다. */
  basePrice?: number;
  baseDate?: string;
  /** 일봉 고/저가 목표에 닿은 첫날 — 서버가 읽을 때 계산한다. */
  reachedDate?: string;
  peakPrice?: number;
  troughPrice?: number;
  currency: "KRW" | "USD";
  note: string;
  createdAt: string;
};

export function inferTargetCurrency(symbol: string): "KRW" | "USD" {
  const trimmed = symbol.trim();
  if (/^\d{6}$/.test(trimmed)) return "KRW";
  return "USD";
}

function toClientTarget(row: PriceTargetRecord): PriceTarget {
  return {
    id: String(row.id),
    symbol: row.symbol,
    targetPrice: row.targetPrice,
    targetPrice2: row.targetPrice2,
    stopPrice: row.stopPrice,
    basePrice: row.basePrice,
    baseDate: row.baseDate,
    reachedDate: row.reachedDate,
    peakPrice: row.peakPrice,
    troughPrice: row.troughPrice,
    currency: row.currency,
    note: row.note,
    createdAt: row.createdAt,
  };
}

const listeners = new Set<() => void>();
let cachedTargets: PriceTarget[] = [];
let loading = false;
let loaded = false;
let migrateStarted = false;

function notify() {
  for (const listener of listeners) listener();
}

/*
마지막 적재가 **왜** 실패했는지.

예전에는 실패를 통째로 삼켰다. 그래서 「목표가 관리」 화면은 서버가 500을 주든
네트워크가 끊기든 **목표가 0건과 구분할 수 없었고**, 0건 분기가 첫 사용자용 온보딩
튜토리얼을 그렸다. 실측: 목표가가 저장돼 있는 계정에서 그 API만 막았더니 화면이
"1 목표가를 적는다…"를 띄웠고 실패를 알리는 문구는 한 글자도 없었다. 사용자에게
그 화면은 「내 기록이 지워졌다」로 읽힌다.

401은 여전히 조용히 넘긴다 — 비로그인 방문자가 예시 모드로 「내 투자」를 구경하는
정상 경로이고, 그건 오류가 아니라 로그인하지 않았다는 사실일 뿐이다.
*/
let lastError: string | null = null;

async function loadFromServer() {
  if (loading) return;
  loading = true;
  try {
    const rows = await fetchPriceTargetsClient();
    cachedTargets = rows.map(toClientTarget);
    loaded = true;
    lastError = null;
    notify();
  } catch (err) {
    const status = err instanceof PortfolioFetchError ? err.status : 0;
    lastError =
      status === 401 || status === 403
        ? null
        : toUserMessage(err, "목표가를 불러오지 못했습니다.");
    // `loaded`를 세우지 않아 로그인 뒤 다음 구독 때 다시 시도한다.
    notify();
  } finally {
    loading = false;
  }
}

/** 적재 실패 사유. 화면이 「없음」과 「못 불러옴」을 가르는 근거. */
export function priceTargetsError(): string | null {
  return lastError;
}

/** 「다시 시도」가 부르는 통로. 실패 상태를 지우고 한 번 더 받는다. */
export async function retryPriceTargets(): Promise<void> {
  lastError = null;
  attemptedForSample = null;
  notify();
  await loadFromServer();
}

function loadLegacyTargets(): PriceTarget[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = localStorage.getItem(LEGACY_STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as PriceTarget[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

async function migrateLegacyIfNeeded() {
  if (migrateStarted || typeof window === "undefined") return;
  migrateStarted = true;
  const legacy = loadLegacyTargets();
  if (legacy.length === 0) return;
  try {
    const existing = await fetchPriceTargetsClient();
    if (existing.length === 0) {
      for (const t of legacy) {
        await createPriceTargetClient({
          symbol: t.symbol,
          targetPrice: t.targetPrice,
          currency: t.currency,
          note: t.note,
        });
      }
    }
    localStorage.removeItem(LEGACY_STORAGE_KEY);
    await loadFromServer();
  } catch {
    migrateStarted = false;
  }
}

/*
소비처가 셋이라(보유 화면 목표가 카드 · 메모 모달 · 「목표가 관리」) 적재 조건을
여기 한 곳에 모은다. 렌더 지점마다 막으면 반드시 새는 곳이 생긴다 — 실제로
보유 화면만 막았더니 `/journal`과 `/targets`가 그대로 401을 냈다.

**예시 모드에서는 인증 경로를 부르지 않는다.** 이 스토어는 구독하는 순간 조건 없이
`/user/price-targets`를 읽었는데, 비로그인 예시 화면에서는 그게 401이고
`session-fetch`가 401마다 `/auth/refresh`를 한 번 더 붙여 요청이 두 배가 됐다.

예시 데이터를 이 스토어에 **담지는 않는다.** 「목표가 관리」 화면이 이미
`/sample/price-targets` 픽스처를 따로 읽어 갈라 쓰고 있어서(targets-client.tsx),
여기에까지 예시를 담으면 모드를 끈 뒤에도 예시가 남는 **같은 종류의 버그**가 생긴다.
*/
/**
 * 어떤 조건으로 마지막 적재를 시도했는지. `null`이면 아직 시도 전.
 *
 * 이게 없으면 안 되는 이유 두 가지가 정확히 반대 방향이다.
 *
 *  - **재시도가 있어야 한다.** `subscribe`는 처음 구독할 때 한 번만 돈다. 「목표가
 *    관리」 화면은 예시를 켜고 끄는 동안 계속 구독 상태라(리마운트되지 않는다,
 *    실측), 예시 중 건너뛴 적재를 다시 시도하지 않으면 예시를 끈 뒤 내 목표가가
 *    영영 비어 있다.
 *  - **무한 재시도면 안 된다.** `loadFromServer`는 실패해도 `loaded`를 세우지
 *    않는다(로그인 뒤 다시 받으려는 의도). 그래서 렌더마다 부르면 비로그인
 *    사용자에게 401이 렌더 수만큼 쏟아진다.
 *
 * 시도 조건(예시 모드 여부)이 그대로면 다시 시도하지 않는 것으로 둘을 만족시킨다 —
 * 조건이 바뀌면(예시 끄기) 정확히 한 번 더 시도한다.
 */
let attemptedForSample: boolean | null = null;

function ensureLoaded() {
  if (loaded || loading) return;
  const sample = isSampleModeActive();
  if (attemptedForSample === sample) return;
  attemptedForSample = sample;
  if (sample) return;
  void migrateLegacyIfNeeded().then(() => loadFromServer());
}

function subscribe(onChange: () => void) {
  listeners.add(onChange);
  // 새 구독은 다시 시도할 자격을 준다 — 로그인 후 화면을 다시 열면 받아 오던
  // 기존 동작을 그대로 유지한다.
  attemptedForSample = null;
  ensureLoaded();
  return () => listeners.delete(onChange);
}

function getSnapshot(): PriceTarget[] {
  return cachedTargets;
}

/** SSR/hydration — 매 호출마다 새 배열을 만들면 무한 루프 */
const serverSnapshot: PriceTarget[] = [];

function getServerSnapshot(): PriceTarget[] {
  return serverSnapshot;
}

export function usePriceTargets(): PriceTarget[] {
  const targets = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  // 예시 모드라 건너뛴 적재를 모드가 꺼진 뒤에 다시 시도한다. `ensureLoaded`가
  // 조건별로 한 번만 시도하므로 매 렌더 호출해도 요청은 늘지 않는다.
  useEffect(() => {
    ensureLoaded();
  });
  return targets;
}

/**
 * 목표가와 **적재 상태**를 함께 준다.
 *
 * `usePriceTargets` 는 배열만 돌려주므로 화면이 「0건」과 「못 불러옴」을 구분할 수
 * 없었다. 그 구분이 필요한 화면은 이쪽을 쓴다 — 실패를 빈 상태로 그리면 사용자는
 * 기록이 지워졌다고 읽는다.
 */
export function usePriceTargetsState(): {
  targets: PriceTarget[];
  error: string | null;
  retry: () => Promise<void>;
} {
  const targets = usePriceTargets();
  const error = useSyncExternalStore(subscribe, priceTargetsError, () => null);
  return { targets, error, retry: retryPriceTargets };
}

export async function refreshPriceTargets(): Promise<void> {
  await loadFromServer();
}

export async function savePriceTarget(input: {
  symbol: string;
  targetPrice: number;
  targetPrice2?: number;
  stopPrice?: number;
  currency: "KRW" | "USD";
  note?: string;
  id?: string;
}): Promise<void> {
  if (input.id) {
    await updatePriceTargetClient(Number(input.id), {
      targetPrice: input.targetPrice,
      targetPrice2: input.targetPrice2,
      stopPrice: input.stopPrice,
      currency: input.currency,
      note: input.note ?? "",
    });
  } else {
    await createPriceTargetClient({
      symbol: input.symbol,
      targetPrice: input.targetPrice,
      targetPrice2: input.targetPrice2,
      stopPrice: input.stopPrice,
      currency: input.currency,
      note: input.note,
    });
  }
  await loadFromServer();
}

export async function removePriceTarget(id: string): Promise<void> {
  await deletePriceTargetClient(Number(id));
  await loadFromServer();
}

/** @deprecated API 기반 — 호환용 래퍼 */
export function loadPriceTargets(): PriceTarget[] {
  return cachedTargets;
}

/** @deprecated API 기반 — 호환용 래퍼 */
export function savePriceTargets(targets: PriceTarget[]) {
  void (async () => {
    for (const t of targets) {
      await savePriceTarget(t);
    }
  })();
}

export function usePriceTargetActions() {
  const refresh = useCallback(() => refreshPriceTargets(), []);
  return { savePriceTarget, removePriceTarget, refresh };
}
