"use client";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { StampDetail, StanceBadge } from "@/components/stamps/stamp-detail";
import { StampSeal } from "@/components/stamps/stamp-seal";
import { formatDateTimeKST } from "@/lib/format";
import {
  HORIZON_LABELS,
  VERDICT_LABELS,
  type Stamp,
  type StampUpdateInput,
} from "@/lib/stamp-types";

/**
 * 스탬프 상세 — 행 아래로 펼치던 아코디언을 모달로 옮겼다.
 *
 * 상세가 길다(예측 · 당시/현재 · 5축 · 차트 · 수익률 표 · 뉴스 · 시장 · 메모). 행
 * 아래에 펼치면 그 아래 행들이 화면 밖으로 밀려 목록을 잃는다. 무엇보다 관심종목·
 * 보유종목·뉴스는 이미 전부 모달이라, 스탬프만 다른 조작감을 갖고 있었다.
 */
export function StampDetailModal({
  stamp,
  readOnly = false,
  onOpenChange,
  onUpdate,
  onOpen,
  onDelete,
}: {
  /** null이면 닫힌다 — 열림 상태와 대상이 한 값으로 묶여 어긋날 수 없다. */
  stamp: Stamp | null;
  /** 예시 데이터를 보는 중 — 서버에 없는 행이라 편집·삭제를 아예 감춘다. */
  readOnly?: boolean;
  onOpenChange: (open: boolean) => void;
  onUpdate: (input: StampUpdateInput) => Promise<void>;
  onOpen: () => void;
  onDelete: () => void;
}) {
  return (
    <Dialog open={stamp !== null} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[88vh] overflow-y-auto sm:max-w-[640px]">
        {stamp && (
          <>
            <DialogHeader>
              <div className="flex items-start gap-2.5">
                <StockAvatar symbol={stamp.symbol} name={stamp.symbolName} size="sm" />
                <div className="min-w-0 flex-1">
                  <DialogTitle className="flex flex-wrap items-center gap-1.5 text-sm">
                    {stamp.symbolName || stamp.symbol}
                    <span className="font-mono text-[11px] font-normal text-[var(--text-muted)]">
                      {stamp.symbol}
                    </span>
                    <StanceBadge stance={stamp.stance} />
                    <Badge
                      variant={
                        stamp.verdict === "hit"
                          ? "profit"
                          : stamp.verdict === "miss"
                            ? "loss"
                            : "default"
                      }
                      className="px-1.5 py-0 text-[10px]"
                    >
                      {VERDICT_LABELS[stamp.verdict]}
                    </Badge>
                  </DialogTitle>
                  <DialogDescription className="text-[11px]">
                    {formatDateTimeKST(stamp.stampedAt)} 스탬프 ·{" "}
                    {HORIZON_LABELS[stamp.horizonKind]} · 개봉 {formatDateTimeKST(stamp.dueAt)}
                  </DialogDescription>
                </div>
                <StampSeal stamp={stamp} size="sm" className="mt-0.5" />
              </div>
            </DialogHeader>

            <StampDetail
              stamp={stamp}
              readOnly={readOnly}
              onUpdate={onUpdate}
              onOpen={onOpen}
              onDelete={onDelete}
            />
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
