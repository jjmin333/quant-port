"use client";

import { useCallback, useEffect, useState } from "react";
import { PageHeader } from "@/components/layout/header";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { PriceDisplay } from "@/components/ui/price-display";
import { useExchangeRate } from "@/hooks/use-exchange-rate";

const STORAGE_KEY = "quant-port-targets";

function inferTargetCurrency(symbol: string): "KRW" | "USD" {
  const trimmed = symbol.trim();
  if (/^\d{6}$/.test(trimmed)) return "KRW";
  return "USD";
}

type PriceTarget = {
  id: string;
  symbol: string;
  targetPrice: number;
  currency: "KRW" | "USD";
  note: string;
  createdAt: string;
};

function loadTargets(): PriceTarget[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as PriceTarget[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function saveTargets(targets: PriceTarget[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(targets));
}

export function TargetsClient() {
  const [targets, setTargets] = useState<PriceTarget[]>([]);
  const [symbol, setSymbol] = useState("");
  const [targetPrice, setTargetPrice] = useState("");
  const [note, setNote] = useState("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [mounted, setMounted] = useState(false);
  const exchangeRate = useExchangeRate(true);

  useEffect(() => {
    setTargets(loadTargets());
    setMounted(true);
  }, []);

  const resetForm = () => {
    setSymbol("");
    setTargetPrice("");
    setNote("");
    setEditingId(null);
  };

  const saveTarget = useCallback(() => {
    const trimmedSymbol = symbol.trim().toUpperCase();
    const price = Number(targetPrice);
    if (!trimmedSymbol || Number.isNaN(price) || price <= 0) return;
    const currency = inferTargetCurrency(trimmedSymbol);

    if (editingId) {
      const next = targets.map((t) =>
        t.id === editingId
          ? { ...t, symbol: trimmedSymbol, targetPrice: price, currency, note: note.trim() }
          : t,
      );
      setTargets(next);
      saveTargets(next);
    } else {
      const entry: PriceTarget = {
        id: crypto.randomUUID(),
        symbol: trimmedSymbol,
        targetPrice: price,
        currency,
        note: note.trim(),
        createdAt: new Date().toISOString(),
      };
      const next = [entry, ...targets];
      setTargets(next);
      saveTargets(next);
    }
    resetForm();
  }, [symbol, targetPrice, note, editingId, targets]);

  const startEdit = (target: PriceTarget) => {
    setEditingId(target.id);
    setSymbol(target.symbol);
    setTargetPrice(String(target.targetPrice));
    setNote(target.note);
  };

  const removeTarget = useCallback(
    (id: string) => {
      const next = targets.filter((t) => t.id !== id);
      setTargets(next);
      saveTargets(next);
      if (editingId === id) resetForm();
    },
    [targets, editingId],
  );

  return (
    <>
      <PageHeader title="목표가 관리" subtitle="종목별 목표가 설정" />
      <main className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto min-w-[var(--dashboard-content-min)] max-w-3xl space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>{editingId ? "목표가 수정" : "목표가 추가"}</CardTitle>
              {editingId && (
                <button
                  type="button"
                  onClick={resetForm}
                  className="text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)]"
                >
                  취소
                </button>
              )}
            </CardHeader>
            <div className="grid gap-3 sm:grid-cols-2">
              <input
                type="text"
                value={symbol}
                onChange={(e) => setSymbol(e.target.value)}
                placeholder="종목코드 (예: 005930)"
                className="h-9 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--accent)] focus:outline-none"
              />
              <div className="flex gap-2">
                <input
                  type="number"
                  value={targetPrice}
                  onChange={(e) => setTargetPrice(e.target.value)}
                  placeholder="목표가"
                  min="0"
                  step="any"
                  className="h-9 min-w-0 flex-1 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--accent)] focus:outline-none"
                />
              </div>
              <input
                type="text"
                value={note}
                onChange={(e) => setNote(e.target.value)}
                placeholder="메모 (선택)"
                className="h-9 sm:col-span-2 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--accent)] focus:outline-none"
              />
              <button
                type="button"
                onClick={saveTarget}
                className="sm:col-span-2 rounded-lg bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white hover:opacity-90"
              >
                {editingId ? "수정" : "추가"}
              </button>
            </div>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>목표가 목록</CardTitle>
              {mounted && <Badge variant="info">{targets.length}건</Badge>}
            </CardHeader>

            {!mounted ? (
              <div className="h-24 animate-pulse rounded-lg bg-[var(--surface-elevated)]" />
            ) : targets.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">등록된 목표가가 없습니다.</p>
            ) : (
              <ul className="space-y-2">
                {targets.map((target) => (
                  <li
                    key={target.id}
                    className="flex items-center justify-between gap-3 rounded-lg bg-[var(--surface-elevated)] px-4 py-3"
                  >
                    <div className="min-w-0">
                      <p className="font-medium text-[var(--text-primary)]">{target.symbol}</p>
                      <p className="text-sm text-[var(--accent)]">
                        <PriceDisplay
                          value={target.targetPrice}
                          currency={target.currency}
                          size="sm"
                          className="text-[var(--accent)]"
                        />
                      </p>
                      {target.note && (
                        <p className="mt-0.5 truncate text-xs text-[var(--text-muted)]">
                          {target.note}
                        </p>
                      )}
                    </div>
                    <div className="flex shrink-0 gap-2">
                      <button
                        type="button"
                        onClick={() => startEdit(target)}
                        className="text-xs text-[var(--text-muted)] hover:text-[var(--accent)]"
                      >
                        수정
                      </button>
                      <button
                        type="button"
                        onClick={() => removeTarget(target.id)}
                        className="text-xs text-[var(--text-muted)] hover:text-rose-400"
                      >
                        삭제
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>
      </main>
    </>
  );
}
