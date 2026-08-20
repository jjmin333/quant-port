"use client";

import { useMemo, useState } from "react";
import { Plus, ArrowRight } from "lucide-react";
import Link from "next/link";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SectionCard, SectionEmpty } from "@/components/ui/section-card";
import { formatPercent, formatPrice } from "@/lib/format";
import type { HoldingMetrics } from "@/lib/holdings-calc";
import {
  inferTargetCurrency,
  savePriceTarget,
  usePriceTargets,
  type PriceTarget,
} from "@/lib/price-targets";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 보유 종목에 걸어둔 내 목표가까지 남은 거리 + 빠른 목표가 추가.
 */
export function HoldingsTargetsCard({
  metrics,
  onPick,
}: {
  metrics: HoldingMetrics[];
  /** 행을 누르면 그 종목 퀵룩이 열린다. 「목표가 관리」로 떠나는 링크는 카드 제목 옆에 남아 있다. */
  onPick: (symbol: string) => void;
}) {
  const targets = usePriceTargets();
  const [addOpen, setAddOpen] = useState(false);
  const [pickSymbol, setPickSymbol] = useState("");
  const [targetPrice, setTargetPrice] = useState("");
  const [error, setError] = useState<string | null>(null);

  const rows = useMemoRows(targets, metrics);

  const heldWithoutTarget = metrics.filter(
    (m) => !targets.some((t) => t.symbol.toUpperCase() === m.symbol.toUpperCase()),
  );

  const resetForm = () => {
    setPickSymbol("");
    setTargetPrice("");
    setError(null);
  };

  const saveTarget = async () => {
    const m = metrics.find((row) => row.symbol === pickSymbol);
    if (!m) {
      setError("종목을 선택해 주세요.");
      return;
    }
    const price = Number(targetPrice);
    if (!Number.isFinite(price) || price <= 0) {
      setError("목표가를 입력해 주세요.");
      return;
    }
    const currency = inferTargetCurrency(m.symbol);
    const existing = targets.find((t) => t.symbol.toUpperCase() === m.symbol.toUpperCase());
    try {
      await savePriceTarget({
        id: existing?.id,
        symbol: m.symbol.toUpperCase(),
        targetPrice: price,
        currency,
        note: existing?.note ?? "",
      });
      setAddOpen(false);
      resetForm();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "저장 실패");
    }
  };

  return (
    <>
      <SectionCard
        title="내 목표가까지"
        action={
          <div className="flex shrink-0 items-center gap-2">
            {metrics.length > 0 && (
              <button
                type="button"
                onClick={() => {
                  resetForm();
                  setAddOpen(true);
                }}
                className="flex items-center gap-0.5 text-[11px] text-muted-foreground hover:text-foreground"
              >
                <Plus className="size-3" />
                추가
              </button>
            )}
            <Link
              href="/targets"
              className="flex items-center gap-0.5 text-[11px] text-muted-foreground hover:text-foreground"
            >
              목표가 관리
              <ArrowRight className="size-3" />
            </Link>
          </div>
        }
      >
        {rows.length === 0 ? (
          <SectionEmpty text="보유 종목에 등록된 목표가가 없습니다" />
        ) : (
          <ul className="min-w-0">
            {rows.map(({ target, m, gap }) => (
              <li key={target.id}>
                <button
                  type="button"
                  onClick={() => onPick(m.symbol)}
                  className="flex h-[30px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-[11px] hover:bg-accent"
                >
                  <span className="min-w-0 flex-1 truncate text-left">{m.name}</span>
                  <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground">
                    {formatPrice(target.targetPrice, target.currency)}
                  </span>
                  {gap == null ? (
                    <span className="w-14 shrink-0 text-right text-muted-foreground">—</span>
                  ) : gap <= 0 ? (
                    <span className="w-14 shrink-0 rounded bg-[var(--brand-muted)] text-center text-[10px] font-semibold text-[var(--brand)]">
                      도달
                    </span>
                  ) : (
                    <span
                      className={cn(
                        "w-14 shrink-0 text-right font-mono tabular-nums",
                        trendTextClass(gap),
                      )}
                    >
                      {formatPercent(gap)}
                    </span>
                  )}
                </button>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent className="gap-4 sm:max-w-sm">
          <DialogHeader>
            <DialogTitle className="text-base">목표가 추가</DialogTitle>
            <DialogDescription className="text-xs">
              보유 종목 중에서 선택합니다.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <label className="block space-y-1">
              <span className="text-[11px] text-muted-foreground">종목</span>
              <select
                value={pickSymbol}
                onChange={(e) => setPickSymbol(e.target.value)}
                className="h-8 w-full rounded-lg border bg-background px-2.5 text-xs focus:border-[var(--brand)] focus:outline-none"
              >
                <option value="">선택…</option>
                {(heldWithoutTarget.length > 0 ? heldWithoutTarget : metrics).map((m) => (
                  <option key={m.symbol} value={m.symbol}>
                    {m.name} ({m.symbol})
                  </option>
                ))}
              </select>
            </label>
            <label className="block space-y-1">
              <span className="text-[11px] text-muted-foreground">목표가</span>
              <Input
                type="number"
                min={0}
                step="any"
                placeholder="0"
                value={targetPrice}
                onChange={(e) => setTargetPrice(e.target.value)}
                className="font-mono tabular-nums"
              />
            </label>
            {error && <p className="text-[11px] text-[var(--danger)]">{error}</p>}
            <div className="flex gap-2">
              <Button type="button" size="sm" className="flex-1" onClick={saveTarget}>
                저장
              </Button>
              <Button type="button" size="sm" variant="outline" onClick={() => setAddOpen(false)}>
                취소
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

function useMemoRows(targets: PriceTarget[], metrics: HoldingMetrics[]) {
  return useMemo(() => {
    const bySymbol = new Map(metrics.map((m) => [m.symbol.toUpperCase(), m]));
    return targets
      .map((t) => ({ target: t, m: bySymbol.get(t.symbol.toUpperCase()) }))
      .filter((r): r is { target: PriceTarget; m: HoldingMetrics } => Boolean(r.m))
      .map(({ target, m }) => {
        const gap =
          m.hasQuote && target.targetPrice > 0
            ? ((target.targetPrice - m.currentPrice) / m.currentPrice) * 100
            : null;
        return { target, m, gap };
      })
      .sort((a, b) => {
        if (a.gap == null) return 1;
        if (b.gap == null) return -1;
        return a.gap - b.gap;
      })
      .slice(0, 5);
  }, [targets, metrics]);
}
