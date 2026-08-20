"use client";

import { useState } from "react";
import { Layers, X } from "lucide-react";
import { BulkAddHoldingsDialog } from "@/components/holdings/bulk-add-holdings-dialog";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SectionCard } from "@/components/ui/section-card";
import { createHoldingTradeClient } from "@/lib/holding-trades-api";
import type { StockInfo } from "@/lib/quotes-types";
import { todayTradeDateISO, TradeDateField } from "@/components/holdings/trade-date-field";
import { cn } from "@/lib/utils";

/**
 * 종목 직접 추가.
 */
export function AddHoldingCard({
  onAdded,
  className,
}: {
  onAdded: () => void;
  exchangeRate?: number;
  className?: string;
}) {
  const [stock, setStock] = useState<StockInfo | null>(null);
  const [quantity, setQuantity] = useState("");
  const [price, setPrice] = useState("");
  const [date, setDate] = useState(todayTradeDateISO());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);
  const [searchFocusKey, setSearchFocusKey] = useState(0);
  const [bulkOpen, setBulkOpen] = useState(false);

  const currency = stock?.currency === "USD" ? "USD" : "KRW";

  const clearStock = () => {
    setStock(null);
    setError(null);
  };

  const reopenSearch = () => {
    clearStock();
    setSearchFocusKey((k) => k + 1);
  };

  const submit = async () => {
    if (!stock) return;
    const qty = Number(quantity);
    if (!Number.isFinite(qty) || qty <= 0) {
      setError("수량을 입력해 주세요.");
      return;
    }
    const prc = Number(price);
    if (!Number.isFinite(prc) || prc <= 0) {
      setError("매수 단가를 입력해 주세요.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await createHoldingTradeClient({
        symbol: stock.symbol,
        side: "buy",
        quantity: qty,
        price: prc,
        currency,
        tradeDate: date,
      });
      setDone(`${stock.name || stock.symbol} ${qty}주를 추가했습니다.`);
      setStock(null);
      setQuantity("");
      setPrice("");
      onAdded();
    } catch (err) {
      setError(err instanceof Error ? err.message : "저장 실패");
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <SectionCard
        title="종목 직접 추가"
        className={className}
        action={
          <button
            type="button"
            onClick={() => setBulkOpen(true)}
            className="flex shrink-0 items-center gap-1 rounded-md border px-2 py-1 text-[11px] text-muted-foreground hover:bg-accent hover:text-foreground"
          >
            <Layers className="size-3" />
            일괄 추가
          </button>
        }
      >
        <div className="mb-2">
          {stock ? (
            <div className="flex h-8 min-w-0 items-center rounded-md border border-input bg-transparent shadow-xs dark:bg-input/30">
              <button
                type="button"
                onClick={reopenSearch}
                className="flex min-w-0 flex-1 items-center gap-2 px-2.5 text-left outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
                aria-label="종목 다시 검색"
              >
                <StockAvatar
                  symbol={stock.symbol}
                  name={stock.name}
                  currency={stock.currency}
                  size="xs"
                  className="shrink-0"
                />
                <span className="min-w-0 flex-1 truncate text-xs">{stock.name || stock.symbol}</span>
                <span className="shrink-0 text-[10px] text-muted-foreground">{stock.symbol}</span>
              </button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="mr-0.5 size-7 shrink-0 text-muted-foreground"
                onClick={clearStock}
                aria-label="종목 선택 취소"
              >
                <X className="size-3.5" />
              </Button>
            </div>
          ) : (
            <StockCatalogSearchField
              key={searchFocusKey}
              placeholder="종목명·코드 검색"
              suggestionsId="holdings-add-suggestions"
              autoFocus={searchFocusKey > 0}
              inputClassName="h-8 py-1 pl-9 pr-2.5 text-xs md:text-xs"
              onSelect={(picked) => {
                setStock(picked);
                setDone(null);
                setError(null);
              }}
            />
          )}
        </div>

        <div className="grid grid-cols-2 gap-1.5">
          <label className="block min-w-0">
            <span className="mb-0.5 block text-[10px] text-muted-foreground">수량</span>
            <Input
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
              inputMode="decimal"
              placeholder="0"
              disabled={!stock}
              className="font-mono"
            />
          </label>
          <label className="block min-w-0">
            <span className="mb-0.5 block text-[10px] text-muted-foreground">
              매수 단가 ({currency})
            </span>
            <Input
              value={price}
              onChange={(e) => setPrice(e.target.value)}
              inputMode="decimal"
              placeholder="0"
              disabled={!stock}
              className="font-mono"
            />
          </label>
        </div>
        <TradeDateField
          label="매수일"
          value={date}
          onChange={setDate}
          disabled={!stock}
          className="mt-2"
        />

        <Button size="sm" className="mt-2 w-full" disabled={!stock || busy} onClick={submit}>
          보유에 추가
        </Button>

        {error && <p className="mt-1.5 text-[10px] text-[var(--danger)]">{error}</p>}
        {done && !error && (
          <p className={cn("mt-1.5 text-[10px] text-[var(--success)]")}>{done}</p>
        )}
        <p className="mt-1.5 text-[10px] leading-relaxed text-muted-foreground">
          매수 기록으로 저장됩니다 — 수량·평단이 이력에서 계산되고, 평가금액 추이에 매수일부터
          반영됩니다. 이미 보유 중인 종목이면 기존 이력에 더해집니다.
        </p>
      </SectionCard>
      <BulkAddHoldingsDialog open={bulkOpen} onOpenChange={setBulkOpen} onSaved={onAdded} />
    </>
  );
}
