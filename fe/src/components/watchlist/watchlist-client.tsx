"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Star, Trash2 } from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { Card } from "@/components/ui/card";
import { QuotePrice } from "@/components/quotes/quote-price";
import {
  addWatchlistClient,
  fetchQuotesClient,
  fetchWatchlistClient,
  removeWatchlistClient,
} from "@/lib/market-api";
import type { QuoteSummary } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";
import { trendTextClass } from "@/lib/trend-colors";

const POLL_MS = 5_000;

export function WatchlistClient() {
  const { activeQuery, isSearchActive, mode } = useHeaderSearch();
  const stockSearchActive = isSearchActive && mode === "stock";
  const [symbols, setSymbols] = useState<string[]>([]);
  const [quotes, setQuotes] = useState<QuoteSummary[]>([]);
  const [newSymbol, setNewSymbol] = useState("");
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [list, prices] = await Promise.all([
        fetchWatchlistClient(),
        fetchQuotesClient("watchlist"),
      ]);
      setSymbols((list ?? []).map((i) => i.symbol));
      setQuotes(prices ?? []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "로드 실패");
    }
  }, []);

  useEffect(() => {
    void load();
    const id = window.setInterval(load, POLL_MS);
    return () => window.clearInterval(id);
  }, [load]);

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!newSymbol.trim()) return;
    try {
      await addWatchlistClient(newSymbol.trim());
      setNewSymbol("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "추가 실패");
    }
  }

  async function handleRemove(symbol: string) {
    try {
      await removeWatchlistClient(symbol);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "삭제 실패");
    }
  }

  const quoteBySymbol = Object.fromEntries(quotes.map((q) => [q.symbol, q]));

  const visibleSymbols = useMemo(() => {
    if (!stockSearchActive) return symbols;
    const q = activeQuery.toLowerCase();
    return symbols.filter((symbol) => {
      if (symbol.toLowerCase().includes(q)) return true;
      const name = quoteBySymbol[symbol]?.name?.toLowerCase() ?? "";
      return name.includes(q);
    });
  }, [symbols, activeQuery, stockSearchActive, quoteBySymbol]);

  return (
    <>
      <PageHeader title="관심종목" subtitle="워치리스트 · 실시간 시세" />
      <main className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto max-w-4xl space-y-4">
          {error && (
            <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-2 text-sm text-amber-200">
              {error}
            </div>
          )}

          <form onSubmit={handleAdd} className="flex gap-2">
            <input
              value={newSymbol}
              onChange={(e) => setNewSymbol(e.target.value)}
              placeholder="종목코드 추가 (예: NVDA)"
              className="flex-1 rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm"
            />
            <button
              type="submit"
              className="rounded-lg bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white"
            >
              추가
            </button>
          </form>

          {visibleSymbols.length === 0 ? (
            <Card className="p-8 text-center text-sm text-[var(--text-muted)]">
              {stockSearchActive
                ? "검색 결과가 없습니다."
                : "관심종목이 없습니다. 종목코드를 추가하세요."}
            </Card>
          ) : (
            visibleSymbols.map((symbol) => {
              const q = quoteBySymbol[symbol];
              return (
                <Card
                  key={symbol}
                  className="flex items-center justify-between gap-4 p-4 transition-colors hover:border-[var(--accent)]"
                >
                  <Link href={`/stocks/${symbol}`} className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <Star className="h-4 w-4 fill-amber-400 text-amber-400" />
                      <div>
                        <p className="font-medium text-[var(--text-primary)]">
                          {q?.name ?? symbol}
                        </p>
                        <p className="text-xs text-[var(--text-muted)]">{symbol}</p>
                      </div>
                    </div>
                  </Link>
                  {q ? (
                    <div className="text-right">
                      <QuotePrice value={q.currentPrice} currency={q.currency} size="md" />
                      <p className={cn("font-mono text-sm", trendTextClass(q.changeRate))}>
                        {q.changeRate >= 0 ? "+" : ""}
                        {q.changeRate.toFixed(2)}%
                      </p>
                    </div>
                  ) : (
                    <span className="text-xs text-[var(--text-muted)]">시세 없음</span>
                  )}
                  <button
                    type="button"
                    onClick={() => handleRemove(symbol)}
                    className="rounded-lg p-2 text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-rose-400"
                    aria-label="삭제"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </Card>
              );
            })
          )}
        </div>
      </main>
    </>
  );
}
