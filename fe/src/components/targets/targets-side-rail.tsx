"use client";

import { useMemo } from "react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { SectionCard, SectionEmpty, SectionKeyValue } from "@/components/ui/section-card";
import { formatPrice } from "@/lib/format";
import type { UserHolding } from "@/lib/user-holdings-types";
import type { TargetView } from "@/lib/target-stats";
import { cn } from "@/lib/utils";
import { PriceDisplay } from "@/components/ui/price-display";
import { buttonVariants } from "@/components/ui/button";

/**
 * 목표가 화면의 우측 레일.
 *
 * 「목표가 없는 보유 종목」이 이 레일의 핵심이다 — 이 화면을 실제로 채우게 만드는
 * 유일한 진입점이라서다. 나머지 카드는 이미 등록한 것을 정리해 보여줄 뿐이고, 등록
 * 자체를 유도하는 자리는 여기 하나뿐이었다.
 *
 * 집계는 전부 **이미 받아둔 배열에서** 센다 — 레일을 그리자고 요청을 늘리지 않는다.
 */
export function TargetsSideRail({
  views,
  holdings,
  onOpenTarget,
  onAddTarget,
}: {
  views: TargetView[];
  holdings: UserHolding[];
  onOpenTarget: (id: string) => void;
  onAddTarget: (symbol: string, name: string) => void;
}) {
  const near = useMemo(
    () =>
      views
        .filter((v) => v.status === "near" || v.status === "stop_near")
        .sort((a, b) => Math.abs(a.toTargetPct ?? 999) - Math.abs(b.toTargetPct ?? 999))
        .slice(0, 5),
    [views],
  );

  const reached = useMemo(() => views.filter((v) => v.status === "reached"), [views]);

  /** 보유 중이면서 목표가도 있는 종목 — 평단과 목표를 나란히 놓는다. */
  const heldWithTarget = useMemo(() => {
    const bySymbol = new Map(holdings.map((h) => [h.symbol.toUpperCase(), h]));
    return views
      .map((v) => ({ view: v, holding: bySymbol.get(v.target.symbol.toUpperCase()) }))
      .filter((r): r is { view: TargetView; holding: UserHolding } => Boolean(r.holding))
      .slice(0, 5);
  }, [views, holdings]);

  /** 보유 중인데 목표가가 없는 종목. */
  const heldWithoutTarget = useMemo(() => {
    const targeted = new Set(views.map((v) => v.target.symbol.toUpperCase()));
    return holdings.filter((h) => !targeted.has(h.symbol.toUpperCase())).slice(0, 6);
  }, [views, holdings]);

  return (
    <>
      {/*
        제목이 「도달 임박」이면 안 된다 — 이 카드는 손절 근접까지 함께 세는데,
        상단 타일의 「도달 임박」은 목표 쪽만 센다. 같은 화면에 같은 이름의 숫자가
        둘(1과 2) 있으면 어느 쪽이 틀린 건지 사용자가 알 방법이 없다.
      */}
      <SectionCard
        title="곧 결판납니다"
        action={
          near.length > 0 ? (
            <span className="rounded-md bg-amber-500/15 px-1.5 py-0.5 text-micro font-semibold text-amber-500">
              {near.length}
            </span>
          ) : undefined
        }
      >
        {near.length === 0 ? (
          <SectionEmpty text="가까워진 목표가 없습니다" />
        ) : (
          <ul className="min-w-0">
            {near.map((v) => (
              <li key={v.target.id}>
                <button
                  type="button"
                  onClick={() => onOpenTarget(v.target.id)}
                  className="flex h-[30px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-cell hover:bg-accent"
                >
                  <StockAvatar
                    symbol={v.target.symbol}
                    name={v.quote?.name}
                    size="xs"
                    className="shrink-0"
                  />
                  <span className="min-w-0 flex-1 truncate text-left">
                    {v.quote?.name || v.target.symbol}
                  </span>
                  <span
                    className={cn(
                      "shrink-0 font-num text-cell font-semibold tabular-nums",
                      v.status === "stop_near" ? "text-[var(--loss)]" : "text-amber-500",
                    )}
                  >
                    {v.status === "stop_near"
                      ? `손절 ${v.toStopPct?.toFixed(1)}%`
                      : `${(v.toTargetPct ?? 0) > 0 ? "+" : ""}${v.toTargetPct?.toFixed(1)}%`}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      {reached.length > 0 && (
        <SectionCard title="도달한 목표" action={<span className="text-micro text-muted-foreground">다음 목표를 정할 때</span>}>
          <ul className="min-w-0">
            {reached.slice(0, 5).map((v) => (
              <li key={v.target.id}>
                <button
                  type="button"
                  onClick={() => onOpenTarget(v.target.id)}
                  className="flex w-full min-w-0 items-center gap-2 rounded-md px-1 py-1.5 text-left hover:bg-accent"
                >
                  <StockAvatar
                    symbol={v.target.symbol}
                    name={v.quote?.name}
                    size="xs"
                    className="shrink-0"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-cell">{v.quote?.name || v.target.symbol}</span>
                    <span className="block truncate text-cell text-muted-foreground">
                      목표 <PriceDisplay value={v.target.targetPrice} currency={v.target.currency} size="inherit" />
                      {v.target.reachedDate && ` · ${v.target.reachedDate} 도달`}
                    </span>
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </SectionCard>
      )}

      {heldWithTarget.length > 0 && (
        <SectionCard title="보유 중인 목표가" action={<span className="text-micro text-muted-foreground">평단 대비</span>}>
          <ul className="min-w-0 space-y-1">
            {heldWithTarget.map(({ view, holding }) => {
              const avg = holding.avgPrice ?? null;
              const upside =
                avg && avg > 0 ? ((view.target.targetPrice - avg) / avg) * 100 : null;
              return (
                <li key={view.target.id}>
                  <button
                    type="button"
                    onClick={() => onOpenTarget(view.target.id)}
                    className="w-full min-w-0 rounded-md px-1 py-1 text-left hover:bg-accent"
                  >
                    <span className="block truncate text-cell">
                      {view.quote?.name || view.target.symbol}
                    </span>
                    <span className="mt-0.5 block truncate font-num text-cell text-muted-foreground">
                      평단 {avg ? formatPrice(avg, view.target.currency) : "—"} → 목표{" "}
                      <PriceDisplay value={view.target.targetPrice} currency={view.target.currency} size="inherit" />
                      {upside !== null && (
                        <span
                          className={cn(
                            "ml-1 font-semibold",
                            upside > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                          )}
                        >
                          {upside > 0 ? "+" : ""}
                          {upside.toFixed(1)}%
                        </span>
                      )}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        </SectionCard>
      )}

      <SectionCard
        title="목표가 없는 보유 종목"
        action={
          heldWithoutTarget.length > 0 ? (
            <span className="text-micro text-muted-foreground">{heldWithoutTarget.length}종목</span>
          ) : undefined
        }
      >
        {holdings.length === 0 ? (
          <SectionEmpty text="보유 종목이 없습니다" />
        ) : heldWithoutTarget.length === 0 ? (
          <SectionEmpty text="보유 종목 전부 목표가가 있습니다" />
        ) : (
          <ul className="min-w-0">
            {heldWithoutTarget.map((h) => (
              <li key={h.symbol}>
                <div className="flex h-[30px] min-w-0 items-center gap-2 rounded-md px-1 text-cell hover:bg-accent">
                  <StockAvatar symbol={h.symbol} name={h.name} size="xs" className="shrink-0" />
                  <span className="min-w-0 flex-1 truncate">{h.name || h.symbol}</span>
                  <button
                    type="button"
                    onClick={() => onAddTarget(h.symbol, h.name || h.symbol)}
                    className={cn(buttonVariants({ variant: "outline", size: "xs" }), "text-micro text-muted-foreground hover:border-[var(--brand)] hover:text-foreground")}
                  >
                    목표 설정
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <SectionCard title="목표 도달 현황">
        {views.length === 0 ? (
          <SectionEmpty text="등록된 목표가가 없습니다" />
        ) : (
          <>
            <SectionKeyValue label="등록" value={`${views.length}건`} />
            <SectionKeyValue label="목표 도달" value={`${reached.length}건`} />
            <SectionKeyValue
              label="손절 설정"
              value={`${views.filter((v) => v.target.stopPrice).length}/${views.length}`}
            />
            <SectionKeyValue
              label="시세 확인됨"
              value={`${views.filter((v) => v.current !== null).length}/${views.length}`}
            />
          </>
        )}
      </SectionCard>
    </>
  );
}
