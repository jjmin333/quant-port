"use client";

import { Pencil, Trash2 } from "lucide-react";
import {
  QuickLookSection,
  StockQuickLookModal,
  type QuickLookStock,
} from "@/components/stocks/stock-quick-look-modal";
import {
  TargetLadder,
  TargetSinceRegistered,
} from "@/components/targets/target-shared";
import { Button } from "@/components/ui/button";
import type { StockScoreData } from "@/lib/stock-score-api";
import type { TargetView } from "@/lib/target-stats";

/**
 * 목표가 퀵룩 — 공용 `StockQuickLookModal`에 「내 목표」 블록만 얹는다.
 *
 * 관심종목·보유종목이 이미 이 껍데기를 쓴다. 같은 종목을 어디서 누르든 헤더·차트·
 * 52주·점수·일정·뉴스가 같은 자리에 있어야 종목을 훑는 사람이 화면을 두 번 배우지
 * 않는다.
 *
 * 차트에는 목표가를 기준선으로 얹는다(`markerPrice`) — 보유 화면이 평단을 얹는
 * 것과 같은 장치라, 지금 가격이 목표선 아래 어디쯤인지가 선 하나로 보인다.
 */
export function TargetQuickLookModal({
  view,
  score,
  open,
  onClose,
  onEdit,
  onRemove,
  readOnly,
}: {
  view: TargetView | null;
  score: StockScoreData | null;
  open: boolean;
  onClose: () => void;
  onEdit: () => void;
  onRemove: () => void;
  /** 예시 모드 — 서버에 없는 행이라 수정·삭제 버튼을 그리지 않는다. */
  readOnly?: boolean;
}) {
  const target = view?.target;
  const quote = view?.quote;

  const stock: QuickLookStock | null =
    view && target
      ? {
          symbol: target.symbol,
          name: quote?.name || target.symbol,
          currency: target.currency,
          assetType: quote?.assetType,
          marketLabel: quote?.market,
          currentPrice: view.current ?? 0,
          change: quote?.change ?? 0,
          changeRate: quote?.changeRate ?? 0,
          hasQuote: view.current !== null,
          week52Low: quote?.watchlistMetrics?.week52Low ?? null,
          week52High: quote?.watchlistMetrics?.week52High ?? null,
          changeRateWeek52Low: quote?.watchlistMetrics?.changeRateWeek52Low ?? null,
          changeRateWeek52High: quote?.watchlistMetrics?.changeRateWeek52High ?? null,
          isHeld: quote?.isHeld,
        }
      : null;

  return (
    <StockQuickLookModal
      key={target?.symbol ?? "none"}
      stock={stock}
      score={score}
      open={open}
      onClose={onClose}
      markerPrice={target?.targetPrice}
      markerLabel="목표가"
      contextSection={
        view && target ? (
          <>
            <QuickLookSection title="내 목표">
              <TargetLadder view={view} />
            </QuickLookSection>
            {target.basePrice && (
              <QuickLookSection title="등록 이후">
                <TargetSinceRegistered view={view} />
              </QuickLookSection>
            )}
            {target.note && (
              <QuickLookSection title="메모">
                <p className="text-cell leading-relaxed text-muted-foreground">{target.note}</p>
              </QuickLookSection>
            )}
          </>
        ) : null
      }
      footerActions={
        view && !readOnly ? (
          <>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="shrink-0"
              onClick={() => {
                onEdit();
                onClose();
              }}
            >
              <Pencil className="size-3.5" />
              목표 수정
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="shrink-0 text-[var(--danger)]"
              onClick={() => {
                onRemove();
                onClose();
              }}
              aria-label="목표가 삭제"
            >
              <Trash2 className="size-3.5" />
            </Button>
          </>
        ) : null
      }
    />
  );
}
