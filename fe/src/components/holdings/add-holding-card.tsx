"use client";

import { useState } from "react";
import { Layers, X } from "lucide-react";
import { LoginRequiredNote } from "@/components/auth/login-required-note";
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
import { toUserMessage } from "@/lib/error-message";
import { buttonVariants } from "@/components/ui/button";
import { useModalAutoFocus } from "@/hooks/use-modal-autofocus";
import { useLoginGate } from "@/hooks/use-login-gate";

/**
 * 종목 직접 추가.
 */
export function AddHoldingCard({
  onAdded,
  className,
  hideTitle = false,
  lockedStock = null,
  initialPrice,
}: {
  onAdded: () => void;
  exchangeRate?: number;
  className?: string;
  /** 다이얼로그 안에서 쓸 때 — 카드 제목이 다이얼로그 제목과 겹친다. */
  hideTitle?: boolean;
  /**
   * 종목을 **못 바꾸게** 고정한다 (2026-09-03 사용자 결정).
   *
   * 종목 상세에서 「보유종목 등록」을 누르면 어느 종목인지는 이미 정해져 있는데,
   * 그때도 「종목명·코드 검색」이 먼저 떴다 — 방금 보고 있던 종목을 다시 찾아
   * 골라야 했다. 보유 화면의 레일에서 쓸 때는 이 값이 없어 예전 그대로 검색으로 연다.
   */
  lockedStock?: StockInfo | null;
  /**
   * 매수 단가의 기본값. 상세에서 열 때 **버튼을 누른 순간의 현재가**를 넣는다 —
   * 부르는 쪽이 그 값을 얼려서 넘기므로, 창을 띄워 둔 채 시세가 움직여도 적어 둔
   * 값이 발밑에서 바뀌지 않는다.
   */
  initialPrice?: number;
}) {
  /*
    비로그인이면 저장 버튼이 **눌리지 않는다** (2026-09-05 사용자 요청 4).

    종목 헤더의 「보유 등록」은 09-04까지 창을 열기 전에 막았지만, 지금은 로그인
    여부와 무관하게 **같은 창이 열린다**. 이 카드를 여는 길은 그것 하나도 아니다 —
    보유 화면의 사이드 레일과 `AddHoldingDialog`가 각자 연다. 그래서 401이 날 요청을
    막는 자리는 여기 저장 버튼 하나이고, 그 자리가 `disabled`다.

    로그인으로 가는 길은 버튼 아래 한 줄이 연다(`LoginRequiredNote`).
  */
  const gate = useLoginGate("보유 등록");

  /* 폰·태블릿에서는 모달이 열려도 자판을 올리지 않는다 — lib/modal-autofocus.ts */
  const canAutoFocus = useModalAutoFocus();
  const [stock, setStock] = useState<StockInfo | null>(lockedStock);
  const [quantity, setQuantity] = useState("");
  const [price, setPrice] = useState(initialPrice && initialPrice > 0 ? String(initialPrice) : "");
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

  /*
    오류 문구는 **실제로 잘못된 것**을 말해야 한다 (§C-9).

    예전에는 수량이 `-5`든 `abc`든 전부 "수량을 입력해 주세요."였다. 값을 이미 채워
    넣은 사람에게 그 문장은 아무것도 알려주지 않는다 — 무엇을 고쳐야 하는지 모른 채
    같은 값을 다시 넣게 된다.
  */
  const numberFieldError = (raw: string, label: string): string | null => {
    if (raw.trim() === "") return `${label}을(를) 입력해 주세요.`;
    const n = Number(raw);
    if (!Number.isFinite(n)) return `${label}은(는) 숫자로 입력해 주세요.`;
    if (n <= 0) return `${label}은(는) 0보다 커야 합니다.`;
    return null;
  };

  const submit = async () => {
    // 버튼이 이미 막혀 있지만, 다른 길로 이 함수에 닿아도 요청이 나가지 않게 한다.
    if (!gate.authed || !stock) return;
    const qtyError = numberFieldError(quantity, "수량");
    if (qtyError) {
      setError(qtyError);
      return;
    }
    const qty = Number(quantity);
    const priceError = numberFieldError(price, "매수 단가");
    if (priceError) {
      setError(priceError);
      return;
    }
    const prc = Number(price);
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
      // 고정된 종목은 비우지 않는다 — 이 창은 그 종목 하나를 위해 열렸고,
      // 비우면 이어서 한 건 더 담을 때 검색부터 다시 해야 한다.
      if (!lockedStock) setStock(null);
      setQuantity("");
      setPrice("");
      onAdded();
    } catch (err) {
      setError(toUserMessage(err, "저장 실패"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <SectionCard
        title={hideTitle ? "" : "종목 직접 추가"}
        className={className}
        action={
          // 종목이 고정된 창에서는 「일괄 추가」를 두지 않는다 — 그건 여러 종목을
          // 한 번에 담는 동작이라 이 창의 전제(한 종목)와 어긋난다.
          lockedStock ? undefined : (
            <button
              type="button"
              onClick={() => setBulkOpen(true)}
              className={cn(buttonVariants({ variant: "outline", size: "xs" }), "text-muted-foreground hover:bg-accent hover:text-foreground")}
            >
              <Layers className="size-3" />
              일괄 추가
            </button>
          )
        }
      >
        <div className="mb-2">
          {lockedStock ? (
            /* 고정된 종목은 **입력이 아니라 사실**이라 바꾸는 손잡이를 두지 않는다. */
            <div className="flex h-8 min-w-0 items-center gap-2 rounded-md border border-input bg-muted/40 px-2.5">
              <StockAvatar
                symbol={lockedStock.symbol}
                name={lockedStock.name}
                currency={lockedStock.currency}
                size="xs"
                className="shrink-0"
              />
              <span className="min-w-0 flex-1 truncate text-cell">
                {lockedStock.name || lockedStock.symbol}
              </span>
              <span className="shrink-0 text-micro text-muted-foreground">{lockedStock.symbol}</span>
            </div>
          ) : stock ? (
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
                <span className="min-w-0 flex-1 truncate text-cell">{stock.name || stock.symbol}</span>
                <span className="shrink-0 text-micro text-muted-foreground">{stock.symbol}</span>
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
              /* 폰·태블릿에서는 자판을 올리지 않는다 — lib/modal-autofocus.ts */
              autoFocus={canAutoFocus && searchFocusKey > 0}
              inputClassName="h-8 py-1 pl-9 pr-2.5 text-cell md:text-cell"
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
            <span className="mb-0.5 block text-micro text-muted-foreground">수량</span>
            <Input
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
              inputMode="decimal"
              placeholder="0"
              disabled={!stock}
              className="font-num"
            />
          </label>
          <label className="block min-w-0">
            <span className="mb-0.5 block text-micro text-muted-foreground">
              매수 단가 ({currency})
            </span>
            <Input
              value={price}
              onChange={(e) => setPrice(e.target.value)}
              inputMode="decimal"
              placeholder="0"
              disabled={!stock}
              className="font-num"
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

        <Button
          size="sm"
          className="mt-2 w-full"
          /* 로그인 전에는 아예 눌리지 않는다 — 저장 요청이 만들어질 길을 없앤다. */
          disabled={!gate.authed || !stock || busy}
          onClick={submit}
        >
          {gate.label("보유에 추가")}
        </Button>
        <LoginRequiredNote gate={gate} />

        {error && <p className="mt-1.5 text-micro text-[var(--danger)]">{error}</p>}
        {done && !error && (
          <p className={cn("mt-1.5 text-micro text-[var(--success)]")}>{done}</p>
        )}
        <p className="mt-1.5 text-micro leading-relaxed text-muted-foreground">
          매수 기록으로 저장됩니다 — 수량·평단이 이력에서 계산되고, 평가금액 추이에 매수일부터
          반영됩니다. 이미 보유 중인 종목이면 기존 이력에 더해집니다.
        </p>
      </SectionCard>
      <BulkAddHoldingsDialog open={bulkOpen} onOpenChange={setBulkOpen} onSaved={onAdded} />
    </>
  );
}
