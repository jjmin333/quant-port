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
import { Input } from "@/components/ui/input";
import { formatKRW } from "@/lib/format";
import {
  fetchUserCashBalancesClient,
  saveUserCashBalancesClient,
} from "@/lib/user-cash-balances-api";

/**
 * 현금(원·달러) 입력 다이얼로그.
 *
 * 레일 전용 카드를 없애고 종목 직접 추가 카드 우측 상단에서 연다.
 * 증권사 연동 예수금과 별도로 사용자가 직접 입력한 잔고만 총 자산에 합산한다.
 */
export function CashBalanceDialog({
  open,
  onOpenChange,
  exchangeRate,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  exchangeRate: number;
  onSaved?: () => void;
}) {
  const [krw, setKrw] = useState("");
  const [usd, setUsd] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    let alive = true;
    setLoading(true);
    setError(null);
    fetchUserCashBalancesClient()
      .then((b) => {
        if (!alive) return;
        setKrw(b.krw > 0 ? String(b.krw) : "");
        setUsd(b.usd > 0 ? String(b.usd) : "");
        setLoading(false);
      })
      .catch(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [open]);

  const parsedKrw = Number(krw.replace(/,/g, ""));
  const parsedUsd = Number(usd.replace(/,/g, ""));
  const usdKRW =
    Number.isFinite(parsedUsd) && parsedUsd > 0 && exchangeRate > 0
      ? parsedUsd * exchangeRate
      : null;

  const submit = async () => {
    const nextKrw = Number.isFinite(parsedKrw) && parsedKrw >= 0 ? parsedKrw : 0;
    const nextUsd = Number.isFinite(parsedUsd) && parsedUsd >= 0 ? parsedUsd : 0;
    setBusy(true);
    setError(null);
    try {
      await saveUserCashBalancesClient({ krw: nextKrw, usd: nextUsd });
      onSaved?.();
      onOpenChange(false);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "저장 실패");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="gap-4 sm:max-w-sm">
        <DialogHeader>
          <DialogTitle className="text-base">현금 추가</DialogTitle>
          <DialogDescription className="text-xs">
            예수금·달러 잔고를 입력하면 총 자산에 합산됩니다. 증권사 연동 예수금과는
            별도입니다.
          </DialogDescription>
        </DialogHeader>

        {loading ? (
          <div className="space-y-2 py-2">
            <div className="h-8 animate-pulse rounded bg-muted" />
            <div className="h-8 animate-pulse rounded bg-muted" />
          </div>
        ) : (
          <div className="space-y-3">
            <label className="block space-y-1">
              <span className="text-[11px] text-muted-foreground">원화 (KRW)</span>
              <Input
                type="number"
                min={0}
                step={1}
                inputMode="numeric"
                placeholder="0"
                value={krw}
                onChange={(e) => setKrw(e.target.value)}
                className="font-mono tabular-nums"
              />
            </label>
            <label className="block space-y-1">
              <span className="text-[11px] text-muted-foreground">달러 (USD)</span>
              <Input
                type="number"
                min={0}
                step={0.01}
                inputMode="decimal"
                placeholder="0"
                value={usd}
                onChange={(e) => setUsd(e.target.value)}
                className="font-mono tabular-nums"
              />
            </label>
            {usdKRW != null && (
              <p className="text-[10px] text-muted-foreground">
                달러 현금 ≈ {formatKRW(Math.round(usdKRW))} (환율 {formatKRW(exchangeRate)})
              </p>
            )}
            {error && <p className="text-[11px] text-[var(--danger)]">{error}</p>}
            <div className="flex gap-2 pt-1">
              <Button type="button" size="sm" className="flex-1" disabled={busy} onClick={() => void submit()}>
                {busy ? "저장 중…" : "저장"}
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
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
