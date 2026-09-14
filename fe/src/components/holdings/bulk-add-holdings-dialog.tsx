"use client";

import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { todayTradeDateISO, TradeDateField } from "@/components/holdings/trade-date-field";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { createHoldingTradeClient } from "@/lib/holding-trades-api";
import type { StockInfo } from "@/lib/quotes-types";
import { toUserMessage } from "@/lib/error-message";

type DraftRow = {
  key: string;
  stock: StockInfo | null;
  quantity: string;
  price: string;
  date: string;
};

/*
 * `crypto.randomUUID()`는 보안 컨텍스트(HTTPS 또는 localhost)에서만 존재한다 — 개발 중
 * 휴대폰에서 사설 IP(http://192.168.x.x:3000)로 접속하면 `crypto.randomUUID`가
 * undefined라 TypeError가 난다. 이 키는 화면 안에서만 쓰는 행 식별자라 굳이 진짜
 * UUID일 필요가 없어 카운터로 대신한다.
 */
let rowKeySeq = 0;
function nextRowKey(): string {
  rowKeySeq += 1;
  return `row-${rowKeySeq}`;
}

function emptyRow(): DraftRow {
  return {
    key: nextRowKey(),
    stock: null,
    quantity: "",
    price: "",
    date: todayTradeDateISO(),
  };
}

/**
 * 여러 종목을 한 번에 매수 기록으로 추가한다.
 */
export function BulkAddHoldingsDialog({
  open,
  onOpenChange,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSaved?: () => void;
}) {
  const [rows, setRows] = useState<DraftRow[]>([emptyRow(), emptyRow()]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const updateRow = (key: string, patch: Partial<DraftRow>) => {
    setRows((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  };

  const submit = async () => {
    const valid = rows.filter((r) => r.stock);
    if (valid.length === 0) {
      setError("추가할 종목을 하나 이상 선택해 주세요.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      for (const row of valid) {
        const qty = Number(row.quantity);
        const price = Number(row.price);
        if (!Number.isFinite(qty) || qty <= 0) {
          throw new Error(`${row.stock!.name || row.stock!.symbol}: 수량을 확인해 주세요.`);
        }
        if (!Number.isFinite(price) || price <= 0) {
          throw new Error(`${row.stock!.name || row.stock!.symbol}: 매수 단가를 확인해 주세요.`);
        }
        const currency = row.stock!.currency === "USD" ? "USD" : "KRW";
        await createHoldingTradeClient({
          symbol: row.stock!.symbol,
          side: "buy",
          quantity: qty,
          price,
          currency,
          tradeDate: row.date,
        });
      }
      onSaved?.();
      onOpenChange(false);
      setRows([emptyRow(), emptyRow()]);
    } catch (err: unknown) {
      setError(toUserMessage(err, "일괄 추가 실패"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[85dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-lg">
        <DialogHeader className="border-b px-4 py-3 pr-10 text-left">
          <DialogTitle className="text-base">종목 일괄 추가</DialogTitle>
          <DialogDescription className="text-cell">
            여러 종목의 매수 기록을 한 번에 등록합니다. 평가금액 추이에 매수일부터 반영됩니다.
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-3">
          {rows.map((row, index) => (
            <div key={row.key} className="rounded-lg border p-2.5">
              <div className="mb-2 flex items-center justify-between gap-2">
                <span className="text-cell font-medium text-muted-foreground">
                  종목 {index + 1}
                </span>
                {rows.length > 1 && (
                  <button
                    type="button"
                    className="text-muted-foreground hover:text-[var(--danger)]"
                    onClick={() => setRows((prev) => prev.filter((r) => r.key !== row.key))}
                    aria-label="행 삭제"
                  >
                    <Trash2 className="size-3.5" />
                  </button>
                )}
              </div>
              {row.stock ? (
                <div className="mb-2 flex h-8 items-center gap-2 rounded-md border px-2">
                  <StockAvatar
                    symbol={row.stock.symbol}
                    name={row.stock.name}
                    currency={row.stock.currency}
                    size="xs"
                  />
                  <span className="min-w-0 flex-1 truncate text-cell">
                    {row.stock.name || row.stock.symbol}
                  </span>
                  <button
                    type="button"
                    className="text-micro text-muted-foreground hover:text-foreground"
                    onClick={() => updateRow(row.key, { stock: null })}
                  >
                    변경
                  </button>
                </div>
              ) : (
                <div className="mb-2">
                  <StockCatalogSearchField
                    placeholder="종목 검색"
                    suggestionsId={`bulk-add-${row.key}`}
                    inputClassName="h-8 py-1 pl-9 pr-2.5 text-cell md:text-cell"
                    onSelect={(picked) => updateRow(row.key, { stock: picked })}
                  />
                </div>
              )}
              <div className="grid grid-cols-2 gap-1.5">
                <Input
                  value={row.quantity}
                  onChange={(e) => updateRow(row.key, { quantity: e.target.value })}
                  inputMode="decimal"
                  placeholder="수량"
                  disabled={!row.stock}
                  className="font-num text-cell"
                />
                <Input
                  value={row.price}
                  onChange={(e) => updateRow(row.key, { price: e.target.value })}
                  inputMode="decimal"
                  placeholder="매수 단가"
                  disabled={!row.stock}
                  className="font-num text-cell"
                />
              </div>
              <TradeDateField
                label="매수일"
                value={row.date}
                onChange={(date) => updateRow(row.key, { date })}
                disabled={!row.stock}
                className="mt-2"
              />
            </div>
          ))}

          <Button
            type="button"
            variant="outline"
            size="sm"
            className="w-full"
            onClick={() => setRows((prev) => [...prev, emptyRow()])}
          >
            <Plus className="mr-1 size-3.5" />
            종목 행 추가
          </Button>

          {error && <p className="text-cell text-[var(--danger)]">{error}</p>}
        </div>

        <div className="flex gap-2 border-t px-4 py-3">
          <Button type="button" size="sm" className="flex-1" disabled={busy} onClick={() => void submit()}>
            {busy ? "저장 중…" : "일괄 추가하기"}
          </Button>
          <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
            취소
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
