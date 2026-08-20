"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  StampPredictionForm,
  type StampPredictionValue,
} from "@/components/stamps/stamp-prediction-form";
import { formatPercent, formatPrice } from "@/lib/format";
import { fetchQuoteDetailClient } from "@/lib/market-api";
import type { StockInfo } from "@/lib/quotes-types";
import { createStampClient } from "@/lib/stamp-api";
import { cn } from "@/lib/utils";

/**
 * 스탬프 화면에서 바로 도장을 찍는 모달.
 *
 * 원래는 종목 화면의 도장 버튼이 유일한 진입점이었다. 그런데 스탬프 목록을 보다가
 * "하나 더 찍어야지"라고 생각한 사람은 종목 화면으로 나갔다가 돌아와야 했고, 아직
 * 한 건도 없는 사람은 어디로 가야 하는지조차 알 수 없었다. 목록이 있는 자리에서
 * 종목을 고르고 바로 찍는다.
 *
 * 시세는 종목을 고른 뒤 한 번만 부른다 — 예상 주가의 기준가이자 52주 미리보기의
 * 축이라 없으면 폼이 반쪽이 된다.
 */
export function StampCreateDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => void;
}) {
  const [stock, setStock] = useState<StockInfo | null>(null);
  const [quote, setQuote] = useState<{
    price: number;
    changeRate?: number;
    currency: "KRW" | "USD";
    week52High?: number;
    week52Low?: number;
  } | null>(null);
  const [loadingQuote, setLoadingQuote] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);

  // 열고 닫을 때의 초기화는 렌더 중에 한다 — 이펙트로 하면 닫힌 프레임이 한 번 더
  // 그려지면서 이전 종목이 남는다 (저장소 공통 패턴).
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (open) {
      setStock(null);
      setQuote(null);
      setError(null);
      setDone(null);
    }
  }

  const pick = async (picked: StockInfo) => {
    setStock(picked);
    setQuote(null);
    setError(null);
    setDone(null);
    setLoadingQuote(true);
    try {
      const detail = await fetchQuoteDetailClient(picked.symbol);
      setQuote({
        price: detail.currentPrice,
        changeRate: detail.changeRate,
        currency: detail.currency === "USD" ? "USD" : "KRW",
        week52High: detail.week52High,
        week52Low: detail.week52Low,
      });
    } catch {
      // 시세를 못 받아도 스탬프 자체는 서버가 자기 시세로 박제하므로 막지 않는다.
      setError("현재가를 불러오지 못했습니다 — 예상 주가는 직접 입력해 주세요.");
    } finally {
      setLoadingQuote(false);
    }
  };

  const submit = async (value: StampPredictionValue) => {
    if (!stock) return;
    setBusy(true);
    setError(null);
    try {
      await createStampClient({ symbol: stock.symbol, ...value });
      setDone(`${stock.name || stock.symbol} 스탬프를 찍었습니다.`);
      setStock(null);
      setQuote(null);
      onCreated();
    } catch (err) {
      const status = (err as { status?: number }).status;
      if (status === 409) setError("이 종목은 방금 찍었습니다 — 5분 뒤에 다시 찍을 수 있습니다.");
      else if (status === 400) setError("예상 주가를 확인해주세요 — 현재가의 10배를 넘을 수 없습니다.");
      else if (status === 401) setError("로그인 후 이용할 수 있습니다.");
      else setError("스탬프 저장에 실패했습니다.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[88vh] overflow-y-auto sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>스탬프 찍기</DialogTitle>
          <DialogDescription>
            종목을 고르고 예상 주가와 검증 기간을 적으면, 지금의 시세·점수·뉴스가 함께
            저장됩니다.
          </DialogDescription>
        </DialogHeader>

        {done && (
          <p className="rounded-lg border border-[var(--brand)] bg-[var(--brand-muted)] px-3 py-2 text-[11px] text-[var(--text-primary)]">
            {done} 다른 종목도 이어서 찍을 수 있습니다.
          </p>
        )}

        {!stock ? (
          <StockCatalogSearchField
            autoFocus
            placeholder="종목명·티커 검색 (예: 삼성전자, NVDA)"
            onSelect={pick}
            suggestionsId="stamp-create-suggestions"
          />
        ) : (
          <>
            <div className="flex items-center gap-2.5 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2.5 py-2">
              <StockAvatar symbol={stock.symbol} name={stock.name} size="sm" />
              <div className="min-w-0 flex-1">
                <p className="truncate text-xs font-semibold text-[var(--text-primary)]">
                  {stock.name || stock.symbol}
                </p>
                <p className="text-[10.5px] tabular-nums text-[var(--text-muted)]">
                  {loadingQuote
                    ? "현재가 불러오는 중…"
                    : quote
                      ? `현재가 ${formatPrice(quote.price, quote.currency)}`
                      : "현재가 —"}
                  {quote?.changeRate !== undefined && (
                    <span
                      className={cn(
                        "ml-1",
                        quote.changeRate > 0
                          ? "text-[var(--profit)]"
                          : quote.changeRate < 0
                            ? "text-[var(--loss)]"
                            : undefined,
                      )}
                    >
                      {formatPercent(quote.changeRate)}
                    </span>
                  )}
                </p>
              </div>
              <button
                type="button"
                onClick={() => {
                  setStock(null);
                  setQuote(null);
                }}
                className="shrink-0 rounded-md border border-[var(--border)] px-2 py-1 text-[10px] text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
              >
                종목 변경
              </button>
            </div>

            {!loadingQuote && (
              <StampPredictionForm
                basePrice={quote?.price}
                currency={quote?.currency ?? "KRW"}
                week52High={quote?.week52High}
                week52Low={quote?.week52Low}
                busy={busy}
                error={error}
                onSubmit={submit}
                onCancel={() => onOpenChange(false)}
              />
            )}
          </>
        )}

        {!stock && error && <p className="text-[11px] text-[var(--loss)]">{error}</p>}
      </DialogContent>
    </Dialog>
  );
}
