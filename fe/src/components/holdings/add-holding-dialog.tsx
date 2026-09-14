"use client";

import { AddHoldingCard } from "@/components/holdings/add-holding-card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { StockInfo } from "@/lib/quotes-types";

/**
 * 「보유에 추가」를 **사이드 레일 밖에서도** 열 수 있게 하는 껍데기.
 *
 * 왜 필요한가: 추가 폼(`AddHoldingCard`)은 보유 화면의 사이드 레일 안에만 있었고,
 * 그 레일은 보유가 0건이면 렌더되지 않았다(`holdings-page-client.tsx`의 `isEmpty`
 * 분기가 안내 카드로 갈아끼운다). 그래서 **가입 직후 사용자는 첫 보유 종목을 추가할
 * 방법이 아예 없었다** — 빈 화면의 CTA는 `/analysis`로 보냈고 거기에도, 종목 상세에도
 * 보유 추가 버튼이 없었다(전 화면 버튼·링크 텍스트 확인: 「보유」「매수」「잔고」 0건).
 *
 * 같은 폼을 두 벌 만들지 않는다 — 레일과 이 다이얼로그가 같은 컴포넌트를 쓴다.
 */
export function AddHoldingDialog({
  open,
  onOpenChange,
  onAdded,
  stock = null,
  price,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAdded: () => void;
  /**
   * 종목 상세에서 열 때 그 종목. 넘기면 검색 없이 **고정**된다
   * (`AddHoldingCard`의 `lockedStock`).
   */
  stock?: StockInfo | null;
  /** 버튼을 누른 순간의 현재가. 매수 단가 기본값이 된다. */
  price?: number;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/*
        키보드 자동 노출은 이제 `DialogContent`가 한 곳에서 가른다 — 폰·태블릿은
        올리지 않고 데스크톱은 그대로다(`lib/modal-autofocus.ts`). 이 창만 따로
        막던 분기는 그래서 지웠다.
      */}
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>{stock ? `${stock.name || stock.symbol} 보유 등록` : "보유 종목 추가"}</DialogTitle>
          <DialogDescription>
            {stock
              ? "수량·매수 단가를 확인하고 저장하면 손익과 비중이 계산됩니다."
              : "종목을 고르고 수량·평단을 적으면 손익과 비중이 계산됩니다."}
          </DialogDescription>
        </DialogHeader>
        {/*
          카드 제목은 다이얼로그 제목과 겹치므로 지운다. 같은 말을 두 번 하는 화면은
          어느 쪽이 진짜인지 헷갈리게 만든다.

          `key`로 여닫을 때마다 폼을 새로 만든다 — 매수 단가 기본값은 마운트 시점의
          `price`에서 오므로, 다시 열었을 때 그때의 시세가 들어가야 한다.
        */}
        <AddHoldingCard
          key={open ? `${stock?.symbol ?? ""}:${price ?? 0}` : "closed"}
          onAdded={onAdded}
          hideTitle
          lockedStock={stock}
          initialPrice={price}
        />
      </DialogContent>
    </Dialog>
  );
}
