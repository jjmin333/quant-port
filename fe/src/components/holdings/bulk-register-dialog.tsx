"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { todayTradeDateISO, TradeDateField } from "@/components/holdings/trade-date-field";
import { bulkRegisterTradesClient } from "@/lib/holding-trades-api";
import { formatPrice } from "@/lib/format";
import type { UntrackedHoldingInput } from "@/components/holdings/holdings-valuation-chart";

type DraftItem = UntrackedHoldingInput & { tradeDate: string };

/**
 * 보유 종목 일괄 등록 모달 — 종목별 매수일을 지정할 수 있다.
 */
export function BulkRegisterDialog({
  open,
  onOpenChange,
  candidates,
  skipped,
  busy,
  error,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  candidates: UntrackedHoldingInput[];
  skipped: UntrackedHoldingInput[];
  busy: boolean;
  error: string | null;
  onSubmit: (items: DraftItem[]) => void;
}) {
  const [dateBySymbol, setDateBySymbol] = useState<Record<string, string>>({});

  useEffect(() => {
    if (!open) setDateBySymbol({});
  }, [open]);

  const today = todayTradeDateISO();
  const drafts: DraftItem[] = open
    ? candidates.map((c) => ({
        ...c,
        tradeDate: dateBySymbol[c.symbol] ?? today,
      }))
    : [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[85dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-md">
        <DialogHeader className="border-b px-4 py-3 pr-10 text-left">
          <DialogTitle className="text-base">평가금액 추이 일괄 등록</DialogTitle>
          <DialogDescription className="text-xs leading-relaxed">
            보유 종목별 매수일을 지정해 매매 이력을 등록합니다. 평단·수량은 현재 보유
            정보를 사용하며, 이미 등록된 종목은 날짜만 수정됩니다.
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
          {drafts.length === 0 ? (
            <p className="py-6 text-center text-[11px] text-muted-foreground">
              등록할 종목이 없습니다.
            </p>
          ) : (
            <ul className="space-y-2">
              {drafts.map((d) => (
                <li key={d.symbol} className="rounded-lg border p-2.5">
                  <p className="text-xs font-medium">{d.name || d.symbol}</p>
                  <p className="mt-0.5 text-[10px] text-muted-foreground">
                    {d.quantity}주 ·{" "}
                    {d.avgPrice != null
                      ? formatPrice(d.avgPrice, d.currency)
                      : "—"}
                  </p>
                  <TradeDateField
                    label="매수일"
                    value={d.tradeDate}
                    onChange={(tradeDate) =>
                      setDateBySymbol((prev) => ({ ...prev, [d.symbol]: tradeDate }))
                    }
                    className="mt-2"
                  />
                </li>
              ))}
            </ul>
          )}

          {skipped.length > 0 && (
            <p className="mt-3 text-[10px] text-muted-foreground">
              평단이 없어 제외: {skipped.map((h) => h.name || h.symbol).join(", ")}
            </p>
          )}
          {error && <p className="mt-2 text-[11px] text-[var(--danger)]">{error}</p>}
        </div>

        <div className="flex gap-2 border-t px-4 py-3">
          <Button
            type="button"
            size="sm"
            className="flex-1"
            disabled={busy || drafts.length === 0}
            onClick={() => onSubmit(drafts)}
          >
            {busy ? "등록 중…" : "일괄 등록하기"}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => onOpenChange(false)}
          >
            취소
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

export async function submitBulkRegister(items: DraftItem[]) {
  await bulkRegisterTradesClient(
    items
      .filter((d) => d.avgPrice != null && d.avgPrice > 0 && d.quantity > 0)
      .map((d) => ({
        symbol: d.symbol,
        quantity: d.quantity,
        price: d.avgPrice!,
        currency: d.currency,
        tradeDate: d.tradeDate,
      })),
  );
}
