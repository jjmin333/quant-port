"use client";

import { useState } from "react";
import { Trash2 } from "lucide-react";
import { TradeDateField, todayTradeDateISO } from "@/components/holdings/trade-date-field";
import {
  QuickLookSection,
  QuickLookStat,
  StockQuickLookModal,
  type QuickLookStock,
} from "@/components/stocks/stock-quick-look-modal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { formatPercent, formatPrice } from "@/lib/format";
import { createHoldingTradeClient } from "@/lib/holding-trades-api";
import type { StockScoreData } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import type { WatchlistRow } from "@/lib/watchlist-calc";

/**
 * 관심종목 퀵룩 — 공용 `StockQuickLookModal`에 관심종목 맥락만 얹는다.
 *
 * 행을 누르면 예전에는 `/stocks/{symbol}`로 **페이지를 떠났다**. 목록을 훑다가 하나가
 * 궁금해서 눌렀을 뿐인데 돌아오면 스크롤 위치도, 방금 보던 비교 대상도 사라진다.
 * 관심종목을 보는 일은 하나를 파고드는 게 아니라 여럿을 견주는 일이라 그 이동이 비쌌다.
 */
export function WatchlistQuickLookModal({
  row,
  score,
  open,
  onClose,
  onRemove,
  onHoldingAdded,
}: {
  row: WatchlistRow | null;
  score: StockScoreData | null;
  open: boolean;
  onClose: () => void;
  onRemove: (symbol: string) => void;
  onHoldingAdded: () => void;
}) {
  const q = row?.quote;
  const metrics = q?.watchlistMetrics;

  const stock: QuickLookStock | null =
    row && q
      ? {
          symbol: row.symbol,
          name: q.name || row.symbol,
          currency: q.currency,
          assetType: q.assetType,
          marketLabel: q.market,
          currentPrice: q.currentPrice,
          change: q.change,
          changeRate: q.changeRate,
          hasQuote: q.currentPrice > 0,
          week52Low: metrics?.week52Low ?? null,
          week52High: metrics?.week52High ?? null,
          changeRateWeek52Low: metrics?.changeRateWeek52Low ?? null,
          changeRateWeek52High: metrics?.changeRateWeek52High ?? null,
          rangePct: row.rangePct,
          isHeld: q.isHeld,
        }
      : null;

  return (
    <StockQuickLookModal
      key={row?.symbol ?? "none"}
      stock={stock}
      score={score}
      open={open}
      onClose={onClose}
      contextSection={
        row && q ? (
          <QuickLookSection title="담은 뒤">
            <div className="grid grid-cols-4 gap-2 text-center">
              <QuickLookStat label="담은 날" value={row.addedAt ? row.addedAt.slice(0, 10) : "—"} />
              <QuickLookStat
                label="담은 가격"
                value={row.addedPrice != null ? formatPrice(row.addedPrice, q.currency) : "—"}
              />
              <QuickLookStat
                label="수익률"
                value={row.sinceAddedRate != null ? formatPercent(row.sinceAddedRate) : "—"}
                className={
                  row.sinceAddedRate != null ? trendTextClass(row.sinceAddedRate) : undefined
                }
              />
              <QuickLookStat label="보유 여부" value={q.isHeld ? "보유 중" : "미보유"} />
            </div>
            {row.addedPrice == null && (
              <p className="mt-1 text-[10px] leading-relaxed text-muted-foreground">
                담은 가격이 기록되기 전에 추가된 종목입니다 — 이후 추가하는 종목부터 채워집니다.
              </p>
            )}
          </QuickLookSection>
        ) : null
      }
      extraSections={row && q ? <AddToHoldings row={row} onAdded={onHoldingAdded} /> : null}
      footerActions={
        row ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="shrink-0 text-[var(--danger)]"
            onClick={() => {
              onRemove(row.symbol);
              onClose();
            }}
            aria-label="관심종목에서 삭제"
          >
            <Trash2 className="size-3.5" />
          </Button>
        ) : null
      }
    />
  );
}

/**
 * 관심에서 보유로.
 *
 * 관심종목을 보다 실제로 샀을 때 지금까지는 화면을 옮겨 처음부터 다시 종목을 검색해야
 * 했다. 종목은 이미 정해져 있으니 남은 건 수량·단가·날짜뿐이다.
 *
 * `AddHoldingCard`와 같은 이유로 **매수 기록으로** 저장한다 — 수량만 덮어쓰면 평가금액
 * 추이 곡선에 이 종목이 언제부터 있었는지가 남지 않는다.
 */
function AddToHoldings({ row, onAdded }: { row: WatchlistRow; onAdded: () => void }) {
  const q = row.quote!;
  const [openForm, setOpenForm] = useState(false);
  const [quantity, setQuantity] = useState("");
  const [price, setPrice] = useState(() => (q.currentPrice > 0 ? String(q.currentPrice) : ""));
  const [date, setDate] = useState(todayTradeDateISO);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);

  const submit = async () => {
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
        symbol: row.symbol,
        side: "buy",
        quantity: qty,
        price: prc,
        currency: q.currency,
        tradeDate: date,
      });
      setDone(`${q.name || row.symbol} ${qty}주를 보유에 추가했습니다.`);
      setQuantity("");
      onAdded();
    } catch (err) {
      setError(err instanceof Error ? err.message : "저장 실패");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mt-4 border-t pt-3">
      {!openForm ? (
        <Button
          type="button"
          size="sm"
          variant="outline"
          className="w-full"
          onClick={() => setOpenForm(true)}
        >
          {q.isHeld ? "매수 기록 추가" : "보유에 추가"}
        </Button>
      ) : (
        <>
          <p className="mb-1.5 text-[11px] font-semibold">
            {q.isHeld ? "매수 기록 추가" : "보유에 추가"}
          </p>
          <div className="grid grid-cols-2 gap-1.5">
            <label className="block min-w-0">
              <span className="mb-0.5 block text-[10px] text-muted-foreground">수량</span>
              <Input
                value={quantity}
                onChange={(e) => setQuantity(e.target.value)}
                inputMode="decimal"
                placeholder="0"
                className="font-mono"
              />
            </label>
            <label className="block min-w-0">
              <span className="mb-0.5 block text-[10px] text-muted-foreground">
                매수 단가 ({q.currency})
              </span>
              <Input
                value={price}
                onChange={(e) => setPrice(e.target.value)}
                inputMode="decimal"
                placeholder="0"
                className="font-mono"
              />
            </label>
          </div>
          <TradeDateField label="매수일" value={date} onChange={setDate} className="mt-2" />
          <Button size="sm" className="mt-2 w-full" disabled={busy} onClick={submit}>
            저장
          </Button>
          {error && <p className="mt-1.5 text-[10px] text-[var(--danger)]">{error}</p>}
          {done && !error && <p className="mt-1.5 text-[10px] text-[var(--success)]">{done}</p>}
          <p className="mt-1.5 text-[10px] leading-relaxed text-muted-foreground">
            매수 기록으로 저장됩니다 — 수량·평단이 이력에서 계산되고, 평가금액 추이에 매수일부터
            반영됩니다.
          </p>
        </>
      )}
    </div>
  );
}
