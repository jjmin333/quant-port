"use client";

import { useCallback, useSyncExternalStore } from "react";
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

async function loadFromServer() {
  if (loading) return;
  loading = true;
  try {
    const rows = await fetchPriceTargetsClient();
    cachedTargets = rows.map(toClientTarget);
    loaded = true;
    notify();
  } finally {
    loading = false;
  }
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

function subscribe(onChange: () => void) {
  listeners.add(onChange);
  if (!loaded && !loading) {
    void migrateLegacyIfNeeded().then(() => loadFromServer());
  }
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
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
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
