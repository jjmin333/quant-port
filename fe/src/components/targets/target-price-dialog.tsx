"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { LoginRequiredNote } from "@/components/auth/login-required-note";
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
import { useLoginGate } from "@/hooks/use-login-gate";
import { createPriceTargetClient } from "@/lib/price-targets-api";
import { formatPrice } from "@/lib/format";
import { toUserMessage } from "@/lib/error-message";
import type { StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

/**
 * 종목 헤더의 「목표가 설정」이 여는 창.
 *
 * # 왜 모달인가 (2026-09-03 사용자 결정)
 *
 * 예전에는 `/targets`로 **화면을 떠났다.** 목표가를 하나 적자고 보던 차트를 잃고,
 * 돌아오려면 뒤로가기를 눌러야 했다. 바로 옆 「스탬프 생성」은 이미 그 자리에서
 * 모달로 끝나는데 혼자만 달랐다. 목록·수정·삭제는 여전히 `/targets`가 한다 —
 * 이 창은 **지금 이 종목에 하나 새로 다는 것**만 한다.
 *
 * # 키보드는 저절로 뜨지 않는다
 *
 * 모바일에서 입력칸에 `autoFocus`를 걸면 창이 뜨자마자 키보드가 화면 절반을 덮어,
 * 정작 무엇을 적는 창인지(종목·현재가) 보이지 않는다. 사용자가 칸을 눌렀을 때
 * 올라오는 편이 맞다.
 *
 * # 로그인 확인은 **이 안에서** 한다 (2026-09-05 사용자 요청 4)
 *
 * 09-04에는 이 창을 여는 버튼(`StockSymbolActionBar`)이 먼저 막아서 비로그인에게는
 * 창이 아예 열리지 않았다. 지금은 **로그인한 사람과 같은 창이 같은 모양으로** 열리고,
 * 막는 자리는 저장 버튼 하나다 — `disabled`라 요청이 만들어질 길이 없다.
 *
 * 창을 열어 둔 채 토큰이 만료되는 경우는 이 검사로 잡히지 않는다(그때 `session`은
 * 아직 있다). 그 길은 서버가 401로 돌려주고 화면은 그 문구를 그대로 보여 준다.
 *
 * 규칙 자체(문구·돌아올 자리)는 `useLoginGate` 한 곳에 있다.
 */
export function TargetPriceDialog({
  open,
  onOpenChange,
  stock,
  currentPrice,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  stock: StockInfo;
  /** 버튼을 누른 순간의 현재가. 목표가와의 거리 계산에만 쓴다. */
  currentPrice?: number;
  onSaved?: () => void;
}) {
  const router = useRouter();
  /*
    문구는 「로그인하고 저장」이 아니라 **「로그인하고 목표가 설정」**이다
    (2026-09-05 사용자 요청 4). 「저장」은 무엇을 저장하는지 안 적혀 있어서, 스탬프·보유
    등록의 같은 버튼과 **글자가 똑같아진다** — 세 창을 오가면 어느 것이 무엇이었는지
    사라진다.

    이제 셋 다 **비로그인에게도 창이 열리고**, 막는 자리는 저장 버튼 하나다(`disabled`).
    로그인으로 가는 길은 버튼 아래 한 줄이 연다(`LoginRequiredNote`).
  */
  const gate = useLoginGate("목표가 설정");
  const [targetPrice, setTargetPrice] = useState("");
  const [stopPrice, setStopPrice] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  const currency = stock.currency === "USD" ? "USD" : "KRW";
  const target = Number(targetPrice);
  const valid = Number.isFinite(target) && target > 0;
  const gap =
    valid && currentPrice && currentPrice > 0
      ? ((target - currentPrice) / currentPrice) * 100
      : null;

  const close = (next: boolean) => {
    if (!next) {
      setTargetPrice("");
      setStopPrice("");
      setNote("");
      setError(null);
      setDone(false);
    }
    onOpenChange(next);
  };

  const submit = async () => {
    // 버튼이 이미 막혀 있지만, 다른 길로 이 함수에 닿아도 요청이 나가지 않게 한다.
    if (!gate.authed) return;
    if (!valid) {
      setError("목표가는 0보다 큰 숫자여야 합니다.");
      return;
    }
    const stop = Number(stopPrice);
    setBusy(true);
    setError(null);
    try {
      await createPriceTargetClient({
        symbol: stock.symbol,
        targetPrice: target,
        stopPrice: stopPrice.trim() !== "" && Number.isFinite(stop) && stop > 0 ? stop : undefined,
        currency,
        note: note.trim() || undefined,
      });
      setDone(true);
      onSaved?.();
    } catch (err) {
      setError(toUserMessage(err, "목표가 저장에 실패했습니다."));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={close}>
      {/*
        **키보드가 저절로 올라오지 않게 한다** (2026-09-03 사용자 결정). 그 판정은
        이제 이 창이 아니라 `DialogContent`가 모든 모달에 대해 한다 — 폰·태블릿은
        올리지 않고, 데스크톱은 예전처럼 목표가 칸으로 포커스가 간다
        (`lib/modal-autofocus.ts`).
      */}
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle className="flex min-w-0 items-center gap-2 text-sm">
            <StockAvatar
              symbol={stock.symbol}
              name={stock.name}
              currency={stock.currency}
              assetType={stock.assetType}
              size="sm"
              className="shrink-0"
            />
            <span className="min-w-0 flex-1 truncate">{stock.name || stock.symbol} 목표가</span>
          </DialogTitle>
          <DialogDescription className="text-cell">
            {currentPrice && currentPrice > 0
              ? `현재가 ${formatPrice(currentPrice, currency)} · 등록가로 함께 기록됩니다`
              : "지금 주가가 등록가로 함께 기록됩니다"}
          </DialogDescription>
        </DialogHeader>

        {done ? (
          <div className="space-y-3 py-4 text-center">
            <p className="text-sm text-[var(--text-primary)]">목표가를 저장했습니다.</p>
            <div className="flex justify-center gap-2">
              <Button variant="outline" size="sm" onClick={() => close(false)}>
                닫기
              </Button>
              <Button size="sm" onClick={() => router.push("/predictions")}>
                목표가 예측으로
              </Button>
            </div>
          </div>
        ) : (
          <div className="space-y-2">
            <label className="block">
              <span className="mb-0.5 block text-micro text-muted-foreground">
                목표가 ({currency})
              </span>
              {/*
                `autoFocus`를 걸지 않는다 — 모바일에서 창이 뜨자마자 키보드가 올라와
                종목·현재가가 가려진다(위 머리말 참고).
              */}
              <Input
                value={targetPrice}
                onChange={(e) => setTargetPrice(e.target.value)}
                inputMode="decimal"
                placeholder="0"
                className="font-num"
              />
            </label>
            {gap !== null && (
              <p
                className={cn(
                  "text-micro tabular-nums",
                  gap >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                )}
              >
                현재가 대비 {gap > 0 ? "+" : ""}
                {gap.toFixed(1)}%
              </p>
            )}
            <label className="block">
              <span className="mb-0.5 block text-micro text-muted-foreground">
                손절가 ({currency}) · 선택
              </span>
              <Input
                value={stopPrice}
                onChange={(e) => setStopPrice(e.target.value)}
                inputMode="decimal"
                placeholder="0"
                className="font-num"
              />
            </label>
            <Input
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="메모 (선택)"
              className="text-cell"
            />

            <Button
              className="mt-1 w-full"
              /* 로그인 전에는 아예 눌리지 않는다 — 저장 요청이 만들어질 길을 없앤다. */
              disabled={!gate.authed || !valid || busy}
              onClick={submit}
            >
              {gate.label("목표가 저장")}
            </Button>
            <LoginRequiredNote gate={gate} />
            {error && <p className="text-micro text-[var(--danger)]">{error}</p>}
            <p className="text-micro leading-relaxed text-muted-foreground">
              2차 목표와 수정·삭제는 목표가 관리 화면에서 할 수 있습니다.
            </p>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
