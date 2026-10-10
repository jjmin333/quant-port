"use client";

import { useEffect, useState } from "react";
import { Card } from "@/components/ui/card";
import { HelpHint } from "@/components/ui/help-hint";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  DivergenceFlag,
  EstimateActualPair,
  ReactionValue,
  SURPRISE_METRIC_LABEL,
  SurpriseValue,
  earningsEventKey,
  eventName,
  periodLabel,
} from "@/components/earnings/earnings-shared";
import { fetchEarningsSurprises } from "@/lib/earnings-api";
import type { EarningsEvent, EarningsMarketFilter } from "@/lib/earnings-types";

/**
 * 서프라이즈 보기 — 「최근 발표에서 누가 기대를 넘었나」를 크기 순으로.
 *
 * 캘린더가 답하지 못하는 질문이다. 캘린더는 시간축이라 8월 3일에 몰린 서른 건 중
 * 어느 것이 놀라운 결과였는지 알려면 서른 줄을 읽어야 한다.
 *
 * **퍼센트가 없는 발표는 여기 오지 않는다.** 기준치가 0 이하면(적자 예상) 서버가
 * 퍼센트 대신 「흑자전환」 같은 라벨을 주는데, 크기를 비교할 수 없는 것을 크기 순
 * 목록에 넣으면 정렬 기준 없는 행들이 목록의 한쪽 끝을 차지한다. 그 발표들은
 * 캘린더 본문에서 라벨로 보인다.
 */

const ORDERS: { id: "best" | "worst"; label: string }[] = [
  { id: "best", label: "상회 순" },
  { id: "worst", label: "하회 순" },
];

export function EarningsSurpriseView({
  market,
  onPickEvent,
}: {
  market: EarningsMarketFilter;
  onPickEvent: (event: EarningsEvent) => void;
}) {
  const [order, setOrder] = useState<"best" | "worst">("best");
  // 정렬·시장을 바꿀 때 이펙트에서 동기로 null을 넣으면 캐스케이드 렌더가 된다.
  // 키를 함께 들고 있다가 안 맞으면 "아직 안 옴"으로 읽는다(이 앱의 기존 패턴).
  const key = `${market}|${order}`;
  const [loaded, setLoaded] = useState<{ key: string; events: EarningsEvent[] } | null>(null);
  const events = loaded?.key === key ? loaded.events : null;

  useEffect(() => {
    let alive = true;
    fetchEarningsSurprises(market, order, 50)
      .then((rows) => {
        if (alive) setLoaded({ key, events: rows ?? [] });
      })
      .catch(() => {
        if (alive) setLoaded({ key, events: [] });
      });
    return () => {
      alive = false;
    };
  }, [key, market, order]);

  return (
    <Card className="p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-section font-semibold text-foreground">
          컨센서스 대비 실적
          <span className="ml-1.5 font-normal text-muted-foreground">최근 90일</span>
          {/* 늘 깔려 있던 두 문단을 필요할 때만 여는 풀이로 (2026-10-09 요청 9). */}
          <HelpHint className="ml-1.5">
            <p>국내는 <b>영업이익</b>, 해외는 <b>EPS</b> 기준입니다. 적자 예상 종목과 컨센서스가 너무 작은 종목(영업이익 50억원 · EPS 0.1달러 미만)은 이 줄 세우기에서 빠집니다.</p>
            <p className="mt-1">해외의 수백 %대 상회·하회는 대개 자산 매각·세금 환입 같은 일회성 항목 때문입니다(발표는 GAAP, 컨센서스는 조정 EPS).</p>
          </HelpHint>
        </h2>
        <MiniToggle options={ORDERS} value={order} onChange={setOrder} />
      </div>

      {events === null ? (
        <SectionSkeleton rows={10} />
      ) : events.length === 0 ? (
        <SectionEmpty text="비교 가능한 발표가 없습니다" />
      ) : (
        <ol className="min-w-0">
          {events.map((event, i) => (
            <li key={earningsEventKey(event)} className="border-b last:border-0">
              <button
                type="button"
                onClick={() => onPickEvent(event)}
                className="flex w-full min-w-0 items-center gap-2 px-1 py-2 text-left hover:bg-muted/50"
              >
                <span className="w-5 shrink-0 text-right font-num text-micro tabular-nums text-muted-foreground">
                  {i + 1}
                </span>
                <StockAvatar
                  symbol={event.symbol}
                  name={eventName(event)}
                  currency={event.currency === "USD" ? "USD" : "KRW"}
                  market={event.market}
                  size="sm"
                  className="shrink-0"
                />
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 items-center gap-1.5">
                    <span className="truncate text-cell font-medium">{eventName(event)}</span>
                    <span className="shrink-0 font-num text-micro text-muted-foreground">
                      {periodLabel(event)}
                    </span>
                  </div>
                  <div className="mt-0.5 flex min-w-0 flex-wrap items-center gap-x-2">
                    {event.surprise && (
                      <EstimateActualPair surprise={event.surprise} currency={event.currency} />
                    )}
                    <span className="shrink-0 text-micro text-muted-foreground">
                      {/* 어느 지표 기준인지 밝힌다 — 국내는 영업이익, 해외는 EPS라
                          한 목록에 두 기준이 섞인다. */}
                      {event.surprise ? `${SURPRISE_METRIC_LABEL[event.surprise.metric] ?? "실적"} 기준` : ""}
                    </span>
                    <span className="shrink-0 font-num text-micro text-muted-foreground">
                      {event.date.slice(5).replace("-", "/")} 발표
                    </span>
                    <DivergenceFlag event={event} />
                  </div>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-0.5">
                  <SurpriseValue surprise={event.surprise} className="text-sm" />
                  {/* 서프라이즈 순 목록에서 반응 열이 특히 값어치 있다 — 상단의
                      「가장 크게 상회한 발표」가 주가로는 어땠는지가 바로 보인다. */}
                  <ReactionValue
                    reaction={event.reaction}
                    status={event.dateStatus}
                    currency={event.currency}
                    className="text-cell"
                  />
                </div>
              </button>
            </li>
          ))}
        </ol>
      )}
    </Card>
  );
}
