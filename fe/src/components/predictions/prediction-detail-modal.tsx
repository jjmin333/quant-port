"use client";

import Link from "next/link";
import { ExternalLink, Trash2 } from "lucide-react";

import { StockAvatar } from "@/components/stocks/stock-avatar";
import { StampDetail } from "@/components/stamps/stamp-detail";
import {
  TargetLadder,
  TargetSinceRegistered,
} from "@/components/targets/target-shared";
import { buttonVariants } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { PriceDisplay } from "@/components/ui/price-display";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { formatDateTimeKST, formatPercent } from "@/lib/format";
import { leaveModal } from "@/lib/modal-history";
import { SCORE_SHIFT_ALERT, type Prediction } from "@/lib/prediction-types";
import type { QuoteSummary } from "@/lib/quotes-types";
import type { StampUpdateInput } from "@/lib/stamp-types";
import { buildTargetView } from "@/lib/target-stats";
import { cn } from "@/lib/utils";
import {
  PredictionAxis,
  PredictionSeal,
  PredictionStatusBadge,
  horizonLabel,
  toneForValue,
} from "@/components/predictions/prediction-bits";

/**
 * 예측 상세.
 *
 * 기한 있는 예측은 스탬프 상세를 그대로 쓴다 — 5축 대조·그때의 뉴스·경로 차트가
 * 이미 다 만들어져 있고, 통합한다고 그걸 다시 그릴 이유가 없다.
 *
 * 기한 없는 예측은 **스냅샷이 없다**. 목표가 테이블에는 등록 당시 점수·뉴스가
 * 저장되지 않기 때문이다. 그 자리를 지어내는 대신 사다리와 등록 이후 요약을 두고,
 * 「없다」고 한 줄 적는다.
 */
export function PredictionDetailModal({
  prediction,
  quote,
  readOnly = false,
  onOpenChange,
  onUpdate,
  onOpen,
  onDelete,
}: {
  /** null이면 닫힌다 — 열림 상태와 대상이 한 값으로 묶여 어긋날 수 없다. */
  prediction: Prediction | null;
  quote?: QuoteSummary;
  readOnly?: boolean;
  onOpenChange: (open: boolean) => void;
  onUpdate: (input: StampUpdateInput) => Promise<void>;
  onOpen: () => void;
  onDelete: () => void;
}) {
  return (
    <Dialog open={prediction !== null} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[88vh] overflow-y-auto sm:max-w-[640px]">
        {prediction && (
          <>
            <DialogHeader>
              <div className="flex items-start gap-2.5">
                <StockAvatar
                  symbol={prediction.symbol}
                  name={prediction.symbolName}
                  size="sm"
                />
                <div className="min-w-0 flex-1">
                  <DialogTitle className="flex flex-wrap items-center gap-1.5 text-sm">
                    {prediction.symbolName}
                    <span className="font-num text-cell font-normal text-[var(--text-muted)]">
                      {prediction.symbol}
                    </span>
                    <PredictionStatusBadge status={prediction.status} />
                  </DialogTitle>
                  <DialogDescription className="text-cell">
                    {formatDateTimeKST(prediction.createdAt)} 등록 ·{" "}
                    {horizonLabel(prediction)}
                    {prediction.dueAt && <> · {formatDateTimeKST(prediction.dueAt)} 만기</>}
                  </DialogDescription>
                </div>
                <PredictionSeal prediction={prediction} size="sm" className="mt-0.5" />
              </div>
            </DialogHeader>

            {/* E3 — 등록 뒤 종합점수가 크게 움직였다. 「팔아라」가 아니라 「다시 보라」다. */}
            {prediction.scoreShift !== undefined &&
              Math.abs(prediction.scoreShift) >= SCORE_SHIFT_ALERT &&
              (prediction.status === "live" || prediction.status === "near") && (
                <p className="rounded-lg border-l-[3px] border-amber-500 bg-amber-500/10 px-3 py-2 text-cell leading-relaxed">
                  등록 뒤 종합점수가{" "}
                  <b className="tabular-nums">
                    {prediction.scoreThen} → {prediction.scoreNow}
                  </b>
                  으로 {prediction.scoreShift > 0 ? "올랐습니다" : "떨어졌습니다"}. 예측을 세울
                  때의 전제가 그대로인지 한 번 볼 자리입니다.
                </p>
              )}

            {prediction.source === "stamp" && prediction.stamp ? (
              <StampDetail
                stamp={prediction.stamp}
                readOnly={readOnly}
                onUpdate={onUpdate}
                onOpen={onOpen}
                onDelete={onDelete}
              />
            ) : (
              <UndatedDetail
                prediction={prediction}
                quote={quote}
                readOnly={readOnly}
                onDelete={onDelete}
                onClose={() => onOpenChange(false)}
              />
            )}
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

/** 기한 없는 예측(기존 목표가)의 상세. */
function UndatedDetail({
  prediction,
  quote,
  readOnly,
  onDelete,
  onClose,
}: {
  prediction: Prediction;
  quote?: QuoteSummary;
  readOnly: boolean;
  onDelete: () => void;
  onClose: () => void;
}) {
  const view = prediction.target ? buildTargetView(prediction.target, quote) : null;

  return (
    <div className="space-y-3">
      <section className="rounded-xl border border-[var(--border)] p-3">
        <h3 className="mb-1 text-cell font-semibold">예측</h3>
        <p className="text-note leading-relaxed tabular-nums">
          {prediction.basePrice !== undefined && (
            <>
              <PriceDisplay
                value={prediction.basePrice}
                currency={prediction.currency}
                size="inherit"
              />
              <span className="mx-1 text-[var(--text-muted)]">에서</span>
            </>
          )}
          <b className={prediction.upside ? "text-[var(--profit)]" : "text-[var(--loss)]"}>
            <PriceDisplay
              value={prediction.targetPrice}
              currency={prediction.currency}
              size="inherit"
            />
          </b>
          <span className="text-[var(--text-muted)]">
            {prediction.expectedReturn === undefined
              ? " 예상"
              : ` 예상 (${formatPercent(prediction.expectedReturn)})`}
          </span>
          {prediction.currentPrice !== undefined && (
            <>
              <span className="mx-1 text-[var(--text-muted)]">·</span>
              현재{" "}
              <b>
                <PriceDisplay
                  value={prediction.currentPrice}
                  currency={prediction.currency}
                  size="inherit"
                />
              </b>
              <span className={cn("ml-1", toneForValue(prediction.sinceBasePct))}>
                {prediction.sinceBasePct === undefined
                  ? ""
                  : formatPercent(prediction.sinceBasePct)}
              </span>
            </>
          )}
        </p>
        <PredictionAxis prediction={prediction} className="mt-1" />
      </section>

      {view && (
        <section className="rounded-xl border border-[var(--border)] p-3">
          <h3 className="mb-2 text-cell font-semibold">가격 사다리</h3>
          <TargetLadder view={view} />
        </section>
      )}

      {view && prediction.basePrice !== undefined && (
        <section className="rounded-xl border border-[var(--border)] p-3">
          <h3 className="mb-2 text-cell font-semibold">등록 이후</h3>
          <TargetSinceRegistered view={view} />
        </section>
      )}

      {prediction.note && (
        <section className="rounded-xl border border-[var(--border)] p-3">
          <h3 className="mb-1 text-cell font-semibold">메모</h3>
          <p className="text-cell leading-relaxed text-[var(--text-secondary)]">
            {prediction.note}
          </p>
        </section>
      )}

      <p className="rounded-lg bg-[var(--surface-elevated)] px-3 py-2 text-micro leading-relaxed text-[var(--text-muted)]">
        <b className="text-[var(--text-secondary)]">기한이 없는 예측이라 채점하지 않습니다.</b>{" "}
        목표·손절에 닿는지만 계속 추적합니다. 등록 당시의 점수·뉴스 스냅샷도 없습니다 — 기한을
        정해 새로 등록하면 그때부터 함께 저장됩니다.
      </p>

      <div className="flex flex-wrap gap-1.5">
        <Link
          href={analysisSymbolHref(prediction.symbol)}
          onClick={() => leaveModal(onClose)}
          className={cn(
            buttonVariants({ variant: "outline", size: "sm" }),
            "gap-1 text-cell",
          )}
        >
          종목 분석 보기
          <ExternalLink className="size-3" />
        </Link>
        {!readOnly && (
          /*
           * 확인은 여기서 하지 않는다 — `onDelete`가 앱 공통 확인 대화상자를 연다
           * (`useConfirm`). 여기에 2단 인라인 확인을 두면 기한 있는 예측(스탬프
           * 상세)과 **같은 화면 안에서 삭제 흐름이 둘로 갈리고**, 목표가 쪽만
           * 확인을 두 번 받게 된다.
           */
          <button
            type="button"
            onClick={onDelete}
            className={cn(
              buttonVariants({ variant: "outline", size: "sm" }),
              "ml-auto gap-1 text-cell text-[var(--text-muted)] hover:text-rose-400",
            )}
          >
            <Trash2 className="size-3" />
            삭제
          </button>
        )}
      </div>
    </div>
  );
}
