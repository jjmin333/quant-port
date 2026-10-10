"use client";

import { useCallback, useEffect, useState } from "react";
import { Trash2 } from "lucide-react";
import { HoldingThesisPicker } from "@/components/holdings/holding-thesis-picker";
import {
  QuickLookKV,
  QuickLookSection,
  StockQuickLookModal,
  type QuickLookStock,
} from "@/components/stocks/stock-quick-look-modal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  createHoldingTradeClient,
  deleteHoldingTradeClient,
  fetchHoldingTradesClient,
} from "@/lib/holding-trades-api";
import type { HoldingTrade, TradeSideKind } from "@/lib/holding-trades-types";
import { todayTradeDateISO, TradeDateField } from "@/components/holdings/trade-date-field";
import { formatKRW, formatPercent, formatPrice, formatPriceChange } from "@/lib/format";
import type { HoldingMetrics } from "@/lib/holdings-calc";
import type { StockScoreData } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";
import { PriceDisplay } from "@/components/ui/price-display";

/**
 * 보유 종목 상세 — 공용 `StockQuickLookModal`에 보유 맥락만 얹는다.
 *
 * 종목을 누르면 종목 분석으로 나가버리던 게 문제였고, 그다음엔 관심종목과 전혀 다른
 * 모양의 모달이 떠서 같은 종목인데 화면마다 아는 것이 달랐다. 지금은 뼈대가 같고
 * 「내 보유」·매매 이력만 이 화면이 더 안다.
 *
 * 수량·평단을 고치는 방법이 두 갈래인 이유:
 *  - **매매 이력이 있는 종목**은 수량·평단이 이력에서 파생된다. 여기서 숫자를 직접
 *    덮어쓰게 두면 다음 매수 한 건에 되돌아가 버린다. 그래서 이력이 있으면 직접 수정
 *    대신 매수/매도를 추가하게 한다 — 그게 실제로 일어난 일이기도 하다.
 *  - **이력이 없는 종목**(스크린샷 임포트만 한 종목)은 직접 입력이 유일한 방법이라
 *    그대로 열어둔다.
 */

type Props = {
  metrics: HoldingMetrics | null;
  score?: StockScoreData | null;
  open: boolean;
  onClose: () => void;
  /** 수량·평단 직접 수정(이력 없는 종목). 페이지가 전체 목록을 PUT 한다. */
  onSaveHolding: (symbol: string, quantity: number, avgPrice: number | null) => Promise<void>;
  onDeleteHolding: (symbol: string) => Promise<void>;
  /** 이력이 바뀌면 페이지가 목록·차트를 다시 읽는다. */
  onTradesChanged: () => void;
};

export function HoldingDetailModal({
  metrics,
  score,
  open,
  onClose,
  onSaveHolding,
  onDeleteHolding,
  onTradesChanged,
}: Props) {
  const m = metrics;
  const quote = m?.row.quote;
  const w52 = quote?.watchlistMetrics;

  const [trades, setTrades] = useState<{ key: string; rows: HoldingTrade[] } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const symbol = m?.symbol ?? "";

  const loadTrades = useCallback(() => {
    if (!symbol) return;
    fetchHoldingTradesClient(symbol)
      .then((rows) => setTrades({ key: symbol, rows }))
      .catch(() => setTrades({ key: symbol, rows: [] }));
  }, [symbol]);

  useEffect(() => {
    if (!open) return;
    loadTrades();
  }, [open, loadTrades]);

  // 닫혔다 다시 열었을 때 지난번 삭제 확인·오류가 남아 있으면 안 된다.
  // 이펙트 대신 렌더 중 조정 — 닫힌 프레임을 한 번 더 그리지 않는다.
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (!open) {
      if (confirmDelete) setConfirmDelete(false);
      if (error) setError(null);
    }
  }

  const tradeRows = trades?.key === symbol ? trades.rows : null;
  const hasHistory = (tradeRows?.length ?? 0) > 0;

  const stock: QuickLookStock | null = m
    ? {
        symbol: m.symbol,
        name: m.name,
        currency: m.currency,
        assetType: quote?.assetType,
        marketLabel: quote?.market || m.row.brokerHint || undefined,
        currentPrice: m.currentPrice,
        change: quote?.change ?? 0,
        changeRate: m.changeRate,
        hasQuote: m.hasQuote,
        week52Low: w52?.week52Low ?? null,
        week52High: w52?.week52High ?? null,
        changeRateWeek52Low: w52?.changeRateWeek52Low ?? null,
        changeRateWeek52High: w52?.changeRateWeek52High ?? null,
        rangePct: null,
        isHeld: true,
      }
    : null;

  return (
    <StockQuickLookModal
      key={symbol || "none"}
      stock={stock}
      score={score}
      open={open}
      onClose={onClose}
      markerPrice={m?.avgPrice}
      markerLabel="평단"
      loadingLabel="보유 종목을 불러오는 중…"
      contextSection={m ? <HoldingContext metrics={m} /> : null}
      extraSections={
        m ? (
          <>
            {error && (
              <p className="mb-3 rounded-md bg-[var(--danger)]/10 px-2 py-1.5 text-cell text-[var(--danger)]">
                {error}
              </p>
            )}

            {/* 매수 이유 — 보유 이유 점검이 이 축을 추적한다 (2026-09-24). */}
            <HoldingThesisPicker
              key={m.symbol}
              symbol={m.symbol}
              initial={m.row.thesisAxes ?? []}
              onSaved={() => onTradesChanged()}
            />

            <TradeSection
              symbol={m.symbol}
              currency={m.currency}
              holdingQuantity={m.quantity}
              holdingAvgPrice={m.avgPrice}
              trades={tradeRows}
              busy={busy}
              setBusy={setBusy}
              setError={setError}
              onChanged={() => {
                loadTrades();
                onTradesChanged();
              }}
            />

            {!hasHistory && tradeRows != null && (
              <ManualEdit
                metrics={m}
                busy={busy}
                onSave={async (quantity, avgPrice) => {
                  setBusy(true);
                  setError(null);
                  try {
                    await onSaveHolding(m.symbol, quantity, avgPrice);
                  } catch (err) {
                    setError(toUserMessage(err, "저장 실패"));
                  } finally {
                    setBusy(false);
                  }
                }}
              />
            )}
          </>
        ) : null
      }
      footerActions={
        m ? (
          confirmDelete ? (
            <>
              <span className="self-center text-micro text-muted-foreground">
                매매 이력까지 삭제
              </span>
              <Button size="sm" variant="outline" onClick={() => setConfirmDelete(false)}>
                취소
              </Button>
              <Button
                size="sm"
                variant="destructive"
                disabled={busy}
                onClick={async () => {
                  setBusy(true);
                  try {
                    await onDeleteHolding(m.symbol);
                    onClose();
                  } catch (err) {
                    setError(toUserMessage(err, "삭제 실패"));
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                삭제
              </Button>
            </>
          ) : (
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="shrink-0 text-[var(--danger)]"
              onClick={() => setConfirmDelete(true)}
              aria-label="보유 삭제"
            >
              <Trash2 className="size-3.5" />
            </Button>
          )
        ) : null
      }
    />
  );
}

/**
 * 「내 보유」 — 종목 자체가 아니라 **내 돈**에 대한 사실만 모은다.
 *
 * 평가손익을 맨 위에 크게 두는 이유: 보유 화면에서 종목을 누르는 이유가 대개 그것
 * 하나다. 평단이 없으면 손익을 0으로 그리지 않고 `—`로 두고 이유를 적는다.
 */
function HoldingContext({ metrics }: { metrics: HoldingMetrics }) {
  const m = metrics;
  const tone = m.unrealizedPnl == null ? "" : trendTextClass(m.unrealizedPnl);

  return (
    <QuickLookSection title="내 보유">
      <div className="rounded-lg border p-2.5">
        <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
          <span className="text-cell text-muted-foreground">평가손익</span>
          {m.unrealizedPnl == null || m.returnRate == null ? (
            <span className="font-num text-lg font-bold text-muted-foreground">—</span>
          ) : (
            <span className="flex items-baseline gap-2">
              <b className={cn("font-num text-lg font-bold tabular-nums", tone)}>
                {formatPriceChange(
                  m.currency === "USD" ? m.unrealizedPnl : Math.round(m.unrealizedPnl),
                  m.currency,
                )}
              </b>
              <b className={cn("font-num text-cell tabular-nums", tone)}>
                {formatPercent(m.returnRate)}
              </b>
            </span>
          )}
        </div>
        {m.unrealizedPnl == null && (
          <p className="mt-0.5 text-micro text-muted-foreground">
            평단가를 입력하면 손익이 계산됩니다
          </p>
        )}

        <div className="mt-2 border-t pt-1.5">
          <QuickLookKV
            label="수량 · 평단"
            value={`${m.quantity}주 · ${m.avgPrice == null ? "—" : formatPrice(m.avgPrice, m.currency)}`}
          />
          <QuickLookKV
            label="매입금액 → 평가금액"
            value={
              m.costBasis == null || m.marketValue == null
                ? "—"
                : `${formatPrice(m.costBasis, m.currency)} → ${formatPrice(m.marketValue, m.currency)}`
            }
          />
          <QuickLookKV
            label="내 포트폴리오 비중"
            value={m.weight == null ? "—" : `${m.weight.toFixed(1)}%`}
          />
          {m.currency === "USD" && m.marketValueKRW != null && (
            <QuickLookKV label="원화 환산 평가금액" value={formatKRW(Math.round(m.marketValueKRW))} />
          )}
        </div>
      </div>
    </QuickLookSection>
  );
}

function TradeSection({
  symbol,
  currency,
  holdingQuantity,
  holdingAvgPrice,
  trades,
  busy,
  setBusy,
  setError,
  onChanged,
}: {
  symbol: string;
  currency: "KRW" | "USD";
  holdingQuantity: number;
  holdingAvgPrice: number | null;
  trades: HoldingTrade[] | null;
  busy: boolean;
  setBusy: (v: boolean) => void;
  setError: (v: string | null) => void;
  onChanged: () => void;
}) {
  const [adding, setAdding] = useState(false);
  const [side, setSide] = useState<TradeSideKind>("buy");
  const [quantity, setQuantity] = useState("");
  const [price, setPrice] = useState("");
  const [date, setDate] = useState(todayTradeDateISO);

  const isFirstTrade = trades != null && trades.length === 0;

  const openAddForm = () => {
    setAdding((open) => {
      if (open) return false;
      setError(null);
      setDate(todayTradeDateISO());
      if (trades != null && trades.length === 0) {
        // 스크린샷 임포트 등 이력 없이 보유만 있는 종목 → 첫 기록은 현재 수량·평단으로 채운다.
        setSide("buy");
        setQuantity(holdingQuantity > 0 ? String(holdingQuantity) : "");
        setPrice(holdingAvgPrice != null && holdingAvgPrice > 0 ? String(holdingAvgPrice) : "");
      } else {
        setQuantity("");
        setPrice("");
      }
      return true;
    });
  };

  const submit = async () => {
    const qty = Number(quantity);
    const prc = Number(price);
    if (!Number.isFinite(qty) || qty <= 0) {
      setError("수량을 입력해 주세요.");
      return;
    }
    if (!Number.isFinite(prc) || prc <= 0) {
      setError("단가를 입력해 주세요.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await createHoldingTradeClient({
        symbol,
        side,
        quantity: qty,
        price: prc,
        currency,
        tradeDate: date,
      });
      setQuantity("");
      setPrice("");
      setAdding(false);
      onChanged();
    } catch (err) {
      setError(toUserMessage(err, "저장 실패"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <QuickLookSection
      title={
        <>
          매매 이력
          {trades != null && trades.length > 0 && (
            <span className="ml-1 font-normal">{trades.length}건</span>
          )}
        </>
      }
      action={
        <Button size="xs" variant="outline" disabled={trades == null} onClick={openAddForm}>
          {adding ? "닫기" : "+ 기록 추가"}
        </Button>
      }
    >
      {adding && (
        <div className="mb-2 rounded-lg border p-2.5">
          <div className="mb-2 flex items-center gap-0.5 rounded-md bg-muted p-0.5">
            {(
              [
                ["buy", "매수"],
                ["sell", "매도"],
              ] as const
            ).map(([value, label]) => (
              <button
                key={value}
                type="button"
                onClick={() => setSide(value)}
                aria-pressed={side === value}
                className={cn(
                  "flex-1 rounded px-2 py-0.5 text-cell transition-colors",
                  side === value
                    ? "bg-card font-semibold text-foreground shadow-xs"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {label}
              </button>
            ))}
          </div>
          <div className="grid grid-cols-2 gap-2">
            <Field label="수량">
              <Input
                value={quantity}
                onChange={(e) => setQuantity(e.target.value)}
                inputMode="decimal"
                placeholder="0"
                className="h-9 font-num text-sm"
              />
            </Field>
            <Field label={`단가 (${currency})`}>
              <Input
                value={price}
                onChange={(e) => setPrice(e.target.value)}
                inputMode="decimal"
                placeholder="0"
                className="h-9 font-num text-sm"
              />
            </Field>
          </div>
          <TradeDateField value={date} onChange={setDate} className="mt-2" />
          {isFirstTrade && holdingQuantity > 0 && (
            <p className="mt-1.5 text-micro text-muted-foreground">
              현재 보유 {holdingQuantity}주
              {holdingAvgPrice != null && holdingAvgPrice > 0
                ? ` · 평단 ${formatPrice(holdingAvgPrice, currency)}`
                : ""}
              를 첫 매수 기록으로 불러왔습니다. 체결일만 맞추면 됩니다.
            </p>
          )}
          <Button size="sm" className="mt-2 w-full" disabled={busy} onClick={submit}>
            {side === "buy" ? "매수 기록" : "매도 기록"} 저장
          </Button>
          <p className="mt-1 text-micro text-muted-foreground">
            저장하면 수량·평단이 이력에서 다시 계산되고, 평가금액 추이에 그날부터 반영됩니다.
          </p>
        </div>
      )}

      {trades == null ? (
        <div className="h-7 animate-pulse rounded bg-muted" />
      ) : trades.length === 0 ? (
        <p className="py-3 text-center text-micro leading-relaxed text-muted-foreground">
          기록된 매매가 없습니다.
          <br />
          매수 기록을 남기면 평가금액 추이 곡선에 이 종목이 포함됩니다.
        </p>
      ) : (
        <ul className="min-w-0">
          {trades.map((t) => (
            <li
              key={t.id}
              className="flex min-w-0 items-center gap-2 border-b border-border/50 py-1 text-cell last:border-b-0"
            >
              <span className="w-[62px] shrink-0 text-muted-foreground">{t.tradeDate}</span>
              <span
                className={cn(
                  "w-8 shrink-0 rounded px-1 text-center text-micro font-semibold",
                  t.side === "sell"
                    ? "bg-[var(--loss)]/10 text-[var(--loss)]"
                    : "bg-[var(--profit)]/10 text-[var(--profit)]",
                )}
              >
                {t.side === "sell" ? "매도" : "매수"}
              </span>
              <span className="min-w-0 flex-1 truncate text-right font-num tabular-nums">
                {t.quantity} × <PriceDisplay value={t.price} currency={t.currency === "USD" ? "USD" : "KRW"} size="inherit" />
              </span>
              <button
                type="button"
                aria-label="이 기록 삭제"
                disabled={busy}
                onClick={async () => {
                  setBusy(true);
                  setError(null);
                  try {
                    await deleteHoldingTradeClient(t.id, symbol);
                    onChanged();
                  } catch (err) {
                    setError(toUserMessage(err, "삭제 실패"));
                  } finally {
                    setBusy(false);
                  }
                }}
                className="shrink-0 rounded p-0.5 text-muted-foreground hover:bg-accent hover:text-[var(--danger)]"
              >
                <Trash2 className="size-3" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </QuickLookSection>
  );
}

function ManualEdit({
  metrics,
  busy,
  onSave,
}: {
  metrics: HoldingMetrics;
  busy: boolean;
  onSave: (quantity: number, avgPrice: number | null) => Promise<void>;
}) {
  const [quantity, setQuantity] = useState(String(metrics.quantity));
  const [avgPrice, setAvgPrice] = useState(metrics.avgPrice == null ? "" : String(metrics.avgPrice));

  const dirty =
    quantity !== String(metrics.quantity) ||
    avgPrice !== (metrics.avgPrice == null ? "" : String(metrics.avgPrice));

  return (
    <QuickLookSection title="직접 수정">
      <div className="grid grid-cols-2 gap-1.5">
        <Field label="수량">
          <Input
            value={quantity}
            onChange={(e) => setQuantity(e.target.value)}
            inputMode="decimal"
            className="font-num"
          />
        </Field>
        <Field label={`평단가 (${metrics.currency})`}>
          <Input
            value={avgPrice}
            onChange={(e) => setAvgPrice(e.target.value)}
            inputMode="decimal"
            placeholder="모르면 비워둡니다"
            className="font-num"
          />
        </Field>
      </div>
      <Button
        size="sm"
        className="mt-1.5 w-full"
        disabled={busy || !dirty}
        onClick={() => {
          const qty = Number(quantity);
          if (!Number.isFinite(qty) || qty <= 0) return;
          const avg = avgPrice.trim() === "" ? null : Number(avgPrice);
          void onSave(qty, avg != null && Number.isFinite(avg) && avg > 0 ? avg : null);
        }}
      >
        저장
      </Button>
      <p className="mt-1 text-micro text-muted-foreground">
        평단가를 비우면 이 종목은 손익 계산에서 빠집니다(0원으로 계산하지 않습니다).
      </p>
    </QuickLookSection>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block min-w-0">
      <span className="mb-0.5 block text-micro text-muted-foreground">{label}</span>
      {children}
    </label>
  );
}
