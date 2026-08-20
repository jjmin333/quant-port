"use client";

import { useEffect, useState } from "react";
import { Card } from "@/components/ui/card";
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
        <h2 className="text-xs font-semibold text-foreground">
          컨센서스 대비 실적
          <span className="ml-1.5 font-normal text-muted-foreground">최근 90일</span>
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
                <span className="w-5 shrink-0 text-right font-mono text-[10px] tabular-nums text-muted-foreground">
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
                    <span className="truncate text-xs font-medium">{eventName(event)}</span>
                    <span className="shrink-0 font-mono text-[10px] text-muted-foreground">
                      {periodLabel(event)}
                    </span>
                  </div>
                  <div className="mt-0.5 flex min-w-0 flex-wrap items-center gap-x-2">
                    {event.surprise && (
                      <EstimateActualPair surprise={event.surprise} currency={event.currency} />
                    )}
                    <span className="shrink-0 text-[10px] text-muted-foreground">
                      {/* 어느 지표 기준인지 밝힌다 — 국내는 영업이익, 해외는 EPS라
                          한 목록에 두 기준이 섞인다. */}
                      {event.surprise ? `${SURPRISE_METRIC_LABEL[event.surprise.metric]} 기준` : ""}
                    </span>
                    <span className="shrink-0 font-mono text-[10px] text-muted-foreground">
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
                    className="text-[11px]"
                  />
                </div>
              </button>
            </li>
          ))}
        </ol>
      )}

      <div className="mt-2 space-y-1 border-t pt-2 text-[10px] leading-relaxed text-muted-foreground">
        <p>
          국내는 <b>영업이익</b>, 해외는 <b>EPS</b> 기준입니다 — 각 시장이 「어닝
          서프라이즈」로 부르는 지표가 다릅니다. 두 종류는 이 줄 세우기에서 빠집니다: 적자를
          예상했던 종목(증감률을 낼 기준선이 없음)과 <b>컨센서스가 너무 작은 종목</b>
          (영업이익 50억원 · EPS 0.1달러 미만 — 기준선이 0에 가까우면 몇 배가 나와도 회사
          규모의 이야기입니다). 둘 다 캘린더 본문에서는 그대로 보입니다.
        </p>
        <p>
          ⚠️ 해외에서 <b>수백 %대 상회·하회</b>는 대개 자산 매각·세금 환입 같은{" "}
          <b>일회성 항목</b> 때문입니다. 발표치는 회계 기준(GAAP) EPS인데 컨센서스는 그런
          항목을 뺀 조정 EPS라, 한 번의 특별이익이 그대로 배수로 나타납니다. 영업의
          개선·악화를 보려면 함께 적힌 <b>기대 → 발표</b> 금액과 종목 상세의 분기 추이를
          같이 보십시오.
        </p>
      </div>
    </Card>
  );
}
