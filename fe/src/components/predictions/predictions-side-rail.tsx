"use client";

import { useMemo } from "react";

import { StockAvatar } from "@/components/stocks/stock-avatar";
import { buttonVariants } from "@/components/ui/button";
import { PriceDisplay } from "@/components/ui/price-display";
import {
  SectionCard,
  SectionEmpty,
  SectionKeyValue,
} from "@/components/ui/section-card";
import { formatDateKST, formatPercent, formatPrice } from "@/lib/format";
import {
  SCORE_SHIFT_ALERT,
  type Prediction,
  type PredictionSummary,
} from "@/lib/prediction-types";
import type { UserHolding } from "@/lib/user-holdings-types";
import { cn } from "@/lib/utils";
import {
  PredictionDday,
  PredictionStatusBadge,
  horizonLabel,
} from "@/components/predictions/prediction-bits";

/**
 * 우측 레일 — 목록을 읽는 동안 곁에 있으면 되는 것들.
 *
 * 「곧 결판납니다」가 이 화면에 매일 들어올 이유다. 목표 임박·손절 통과·만기 임박·
 * 점수 급변은 서로 다른 네 가지 기준이지만 사용자에게는 **「오늘 봐야 할 것」 하나**라
 * 한 목록에 섞는다. 탭으로 나누면 각 탭이 0건인 날 화면이 텅 빈 것처럼 보인다.
 */

type Props = {
  summary: PredictionSummary;
  all: Prediction[];
  urgent: Prediction[];
  holdings: UserHolding[];
  symbols: { symbol: string; name: string; count: number }[];
  symbolFilter: string | null;
  onSymbolFilter: (symbol: string | null) => void;
  onSelect: (p: Prediction) => void;
  onCreateFor: (symbol: string) => void;
};

/** 왜 급한지 한 마디. 배지만 있으면 「임박」이 목표 쪽인지 손절 쪽인지 모른다. */
function urgencyReason(p: Prediction): string {
  switch (p.status) {
    case "due":
      return "만기가 지났습니다 — 열면 채점됩니다";
    case "stop_hit":
      return p.stopPrice !== undefined ? "손절선을 통과했습니다" : "손절 구간입니다";
    case "reached":
      return p.reachedDate ? `${formatDateKST(p.reachedDate)} 목표 도달` : "목표에 닿았습니다";
    case "near":
      if (p.toStopPct !== undefined && Math.abs(p.toStopPct) <= 5) {
        return `손절까지 ${Math.abs(p.toStopPct).toFixed(1)}%`;
      }
      return p.toTargetPct === undefined
        ? "목표에 가까워졌습니다"
        : `목표까지 ${Math.abs(p.toTargetPct).toFixed(1)}%`;
    default:
      if (p.daysLeft !== undefined && p.daysLeft >= 0) {
        return p.daysLeft === 0 ? "오늘 정산됩니다" : `${p.daysLeft}일 뒤 정산됩니다`;
      }
      return horizonLabel(p);
  }
}

export function PredictionsSideRail({
  summary,
  all,
  urgent,
  holdings,
  symbols,
  symbolFilter,
  onSymbolFilter,
  onSelect,
  onCreateFor,
}: Props) {
  const upcoming = useMemo(
    () =>
      all
        .filter((p) => p.dueAt && p.status !== "due" && (p.daysLeft ?? 0) >= 0)
        .sort((a, b) => (a.dueAt as string).localeCompare(b.dueAt as string))
        .slice(0, 6),
    [all],
  );

  const shifted = useMemo(
    () =>
      all
        .filter(
          (p) =>
            p.scoreShift !== undefined &&
            Math.abs(p.scoreShift) >= SCORE_SHIFT_ALERT &&
            (p.status === "live" || p.status === "near"),
        )
        .sort((a, b) => Math.abs(b.scoreShift as number) - Math.abs(a.scoreShift as number))
        .slice(0, 4),
    [all],
  );

  const bySymbol = useMemo(() => new Set(all.map((p) => p.symbol)), [all]);

  const heldWith = useMemo(
    () =>
      holdings
        .map((h) => ({ holding: h, prediction: all.find((p) => p.symbol === h.symbol) }))
        .filter(
          (x): x is { holding: UserHolding; prediction: Prediction } =>
            x.prediction !== undefined,
        )
        .slice(0, 5),
    [holdings, all],
  );

  const heldWithout = useMemo(
    () => holdings.filter((h) => !bySymbol.has(h.symbol)).slice(0, 6),
    [holdings, bySymbol],
  );

  return (
    <>
      <SectionCard
        title="곧 결판납니다"
        action={
          urgent.length > 0 ? (
            <span className="text-micro text-muted-foreground">{urgent.length}건</span>
          ) : undefined
        }
      >
        {urgent.length === 0 ? (
          <SectionEmpty text="당장 볼 것은 없습니다" />
        ) : (
          <ul className="min-w-0 space-y-1.5">
            {urgent.map((p) => (
              <li key={p.key}>
                <button
                  type="button"
                  onClick={() => onSelect(p)}
                  className="flex w-full min-w-0 items-start gap-2 rounded-md px-1 py-1 text-left hover:bg-accent"
                >
                  <PredictionStatusBadge status={p.status} className="mt-0.5 w-[52px] text-center" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-cell font-medium">{p.symbolName}</span>
                    <span className="block truncate text-micro text-muted-foreground">
                      {urgencyReason(p)}
                    </span>
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
        <p className="mt-2 px-1 text-micro leading-relaxed text-muted-foreground">
          목표·손절 5% 이내, 만기 3일 이내, 이미 닿았거나 지난 것을 한 목록에 모읍니다.
        </p>
      </SectionCard>

      {shifted.length > 0 && (
        <SectionCard
          title="전제가 흔들렸습니다"
          action={<span className="text-micro text-muted-foreground">등록 후 점수 변화</span>}
        >
          <ul className="min-w-0 space-y-1">
            {shifted.map((p) => (
              <li key={p.key}>
                <button
                  type="button"
                  onClick={() => onSelect(p)}
                  className="flex w-full min-w-0 items-center gap-2 rounded-md px-1 py-1 text-left hover:bg-accent"
                >
                  <StockAvatar symbol={p.symbol} name={p.symbolName} size="xs" />
                  <span className="min-w-0 flex-1 truncate text-cell">{p.symbolName}</span>
                  <span
                    className={cn(
                      "shrink-0 font-num text-cell font-semibold",
                      (p.scoreShift as number) > 0
                        ? "text-[var(--profit)]"
                        : "text-[var(--loss)]",
                    )}
                  >
                    {p.scoreThen} → {p.scoreNow}
                  </span>
                </button>
              </li>
            ))}
          </ul>
          <p className="mt-2 px-1 text-micro leading-relaxed text-muted-foreground">
            가격이 아니라 <b className="text-foreground">회사 쪽 숫자</b>가 먼저 움직인 예측입니다.
            다시 볼 자리이지 매매 신호가 아닙니다.
          </p>
        </SectionCard>
      )}

      <SectionCard title="다가오는 만기">
        {upcoming.length === 0 ? (
          <SectionEmpty text="기한이 있는 예측이 없습니다" />
        ) : (
          <ul className="min-w-0 space-y-1">
            {upcoming.map((p) => (
              <li key={p.key}>
                <button
                  type="button"
                  onClick={() => onSelect(p)}
                  className="flex w-full min-w-0 items-center gap-2 rounded-md px-1 py-1 text-left hover:bg-accent"
                >
                  <PredictionDday prediction={p} />
                  <span className="min-w-0 flex-1 truncate text-cell">{p.symbolName}</span>
                  <span className="shrink-0 font-num text-micro text-muted-foreground">
                    {p.dueAt ? formatDateKST(p.dueAt) : ""}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      {heldWith.length > 0 && (
        <SectionCard
          title="보유 중인 예측"
          action={<span className="text-micro text-muted-foreground">평단 대비</span>}
        >
          <ul className="min-w-0 space-y-1">
            {heldWith.map(({ holding, prediction }) => {
              const avg = holding.avgPrice ?? null;
              const upside =
                avg && avg > 0 ? ((prediction.targetPrice - avg) / avg) * 100 : null;
              return (
                <li key={prediction.key}>
                  <button
                    type="button"
                    onClick={() => onSelect(prediction)}
                    className="w-full min-w-0 rounded-md px-1 py-1 text-left hover:bg-accent"
                  >
                    <span className="block truncate text-cell">{prediction.symbolName}</span>
                    <span className="mt-0.5 block truncate font-num text-cell text-muted-foreground">
                      평단 {avg ? formatPrice(avg, prediction.currency) : "—"} → 목표{" "}
                      <PriceDisplay
                        value={prediction.targetPrice}
                        currency={prediction.currency}
                        size="inherit"
                      />
                      {upside !== null && (
                        <span
                          className={cn(
                            "ml-1 font-semibold",
                            upside > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                          )}
                        >
                          {formatPercent(upside)}
                        </span>
                      )}
                    </span>
                    {!prediction.upside && (
                      <span className="mt-0.5 block text-micro text-amber-600 dark:text-amber-400">
                        보유 중인데 하방 예측입니다 — 계좌와 예측이 반대입니다
                      </span>
                    )}
                  </button>
                </li>
              );
            })}
          </ul>
        </SectionCard>
      )}

      <SectionCard
        title="예측 없는 보유 종목"
        action={
          heldWithout.length > 0 ? (
            <span className="text-micro text-muted-foreground">{heldWithout.length}종목</span>
          ) : undefined
        }
      >
        {holdings.length === 0 ? (
          <SectionEmpty text="보유 종목이 없습니다" />
        ) : heldWithout.length === 0 ? (
          <SectionEmpty text="보유 종목 전부 예측이 있습니다" />
        ) : (
          <ul className="min-w-0">
            {heldWithout.map((h) => (
              <li key={h.symbol}>
                <div className="flex h-[30px] min-w-0 items-center gap-2 rounded-md px-1 text-cell hover:bg-accent">
                  <StockAvatar symbol={h.symbol} name={h.name} size="xs" className="shrink-0" />
                  <span className="min-w-0 flex-1 truncate">{h.name || h.symbol}</span>
                  <button
                    type="button"
                    onClick={() => onCreateFor(h.symbol)}
                    className={cn(
                      buttonVariants({ variant: "outline", size: "xs" }),
                      "text-micro text-muted-foreground hover:border-[var(--brand)] hover:text-foreground",
                    )}
                  >
                    예측 세우기
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <SectionCard
        title="종목별"
        action={
          symbolFilter ? (
            <button
              type="button"
              onClick={() => onSymbolFilter(null)}
              className="text-micro text-[var(--brand)]"
            >
              전체 보기
            </button>
          ) : undefined
        }
      >
        {symbols.length === 0 ? (
          <SectionEmpty text="등록된 예측이 없습니다" />
        ) : (
          <ul className="min-w-0">
            {symbols.slice(0, 10).map((s) => (
              <li key={s.symbol}>
                <button
                  type="button"
                  onClick={() => onSymbolFilter(symbolFilter === s.symbol ? null : s.symbol)}
                  aria-pressed={symbolFilter === s.symbol}
                  className={cn(
                    "flex h-[30px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-left text-cell hover:bg-accent",
                    symbolFilter === s.symbol && "bg-[var(--brand-muted)] text-[var(--brand)]",
                  )}
                >
                  <StockAvatar symbol={s.symbol} name={s.name} size="xs" className="shrink-0" />
                  <span className="min-w-0 flex-1 truncate">{s.name}</span>
                  <span className="shrink-0 font-num text-micro text-muted-foreground">
                    {s.count}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <SectionCard title="현황">
        <SectionKeyValue label="예측" value={`${summary.total}건`} />
        <SectionKeyValue label="기한 / 무기한" value={`${summary.dated} / ${summary.undated}`} />
        <SectionKeyValue label="개봉 대기" value={`${summary.dueCount}건`} />
        <SectionKeyValue label="목표 임박" value={`${summary.nearCount}건`} />
        <SectionKeyValue label="손절 통과" value={`${summary.stopHits}건`} />
      </SectionCard>
    </>
  );
}
