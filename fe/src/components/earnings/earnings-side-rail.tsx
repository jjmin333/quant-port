"use client";

import { useEffect, useState } from "react";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  DateStatusBadge,
  ReactionValue,
  SurpriseValue,
  earningsEventKey,
  eventName,
  periodLabel,
} from "@/components/earnings/earnings-shared";
import { formatEarningsDateLabel } from "@/lib/earnings-date";
import { fetchEarningsHighlights } from "@/lib/earnings-api";
import type {
  EarningsConsistency,
  EarningsEvent,
  EarningsHighlights,
  EarningsMarket,
} from "@/lib/earnings-types";
import { formatFinancialAmount } from "@/lib/format";
import { cn } from "@/lib/utils";

/**
 * 실적 화면 우측 레일.
 *
 * # 왜 「상회/하회 TOP」을 뺐나
 *
 * 그 목록은 **서프라이즈 탭이 이미 더 길게** 보여준다 — 레일에서 6줄만 반복하는
 * 셈이라 자리값을 못 했다. 지금 네 카드는 전부 **표에 없는 것**만 담는다: 표는
 * 날짜순이라 「무엇이 큰 발표인가」를 못 말하고, 한 행만 봐서는 「숫자와 주가가
 * 엇갈렸다」·「이 회사가 늘 기대를 넘는다」를 알 수 없다.
 */
export function EarningsSideRail({
  market,
  mySymbols,
  myEvents,
  onPickEvent,
}: {
  market: EarningsMarket;
  mySymbols: string[];
  /** 내 종목의 과거 2주~미래 1달 발표 — 본문이 이미 받아 둔 것. */
  myEvents: EarningsEvent[] | null;
  onPickEvent: (event: EarningsEvent) => void;
}) {
  const highlights = useHighlights(market);

  return (
    <div className="space-y-3">
      <SectionCard title="다가오는 대형 발표">
        <EventList
          events={highlights?.big ?? null}
          empty="예정된 발표가 없습니다"
          onPickEvent={onPickEvent}
          render={(ev) => (
            <>
              <DateStatusBadge status={ev.dateStatus} />
              <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground">
                {formatEarningsDateLabel(ev.date)}
              </span>
              <span className="shrink-0 font-mono text-[10px] tabular-nums">
                {ev.marketCap != null
                  ? formatFinancialAmount(ev.marketCap, ev.currency === "USD" ? "USD" : "KRW")
                  : "—"}
              </span>
            </>
          )}
        />
        <p className="mt-1.5 border-t pt-1.5 text-[10px] leading-relaxed text-muted-foreground">
          시가총액 순입니다. 표는 날짜순이라 <b>무엇이 큰 발표인지</b>는 말하지 못합니다.
          {market === "domestic" && " 국내 예정일은 작년 발표일로 추정한 값입니다."}
        </p>
      </SectionCard>

      <SectionCard title="기대와 주가가 엇갈린 발표">
        {highlights === null ? (
          <SectionSkeleton rows={4} />
        ) : highlights.divergent.length === 0 ? (
          <SectionEmpty text="최근 45일에 엇갈린 발표가 없습니다" />
        ) : (
          <ul className="min-w-0">
            {highlights.divergent.map((ev) => (
              <DivergentRow key={earningsEventKey(ev)} event={ev} onPick={onPickEvent} />
            ))}
          </ul>
        )}
        <p className="mt-1.5 border-t pt-1.5 text-[10px] leading-relaxed text-muted-foreground">
          <b>증권사 예측</b>과 <b>실제 발표</b>가 벌어진 방향과, 그 뒤 <b>주가</b>가 간 방향이
          서로 반대인 경우입니다 — 숫자와 시장이 다르게 읽었다는 뜻입니다.
        </p>
      </SectionCard>

      <SectionCard title="꾸준히 기대를 넘는 기업">
        {highlights === null ? (
          <SectionSkeleton rows={4} />
        ) : highlights.consistent.length === 0 ? (
          <SectionEmpty text="비교 가능한 분기가 4개 이상 쌓인 종목이 아직 없습니다" />
        ) : (
          <ul className="min-w-0">
            {highlights.consistent.map((row) => (
              <ConsistencyRow key={row.symbol} row={row} />
            ))}
          </ul>
        )}
        <p className="mt-1.5 border-t pt-1.5 text-[10px] leading-relaxed text-muted-foreground">
          최근 8분기 기준. 한 번은 우연이지만 <b>8분기 중 7번</b>은 그 회사의 가이던스가
          보수적이라는 뜻입니다.
        </p>
      </SectionCard>

      <SectionCard title="내 종목 최근 발표">
        {mySymbols.length === 0 ? (
          <SectionEmpty text="관심·보유 종목이 없습니다" />
        ) : myEvents === null ? (
          <SectionSkeleton rows={3} />
        ) : myEvents.length === 0 ? (
          <SectionEmpty text="이 기간에 내 종목 일정이 없습니다" />
        ) : (
          <ul className="min-w-0">
            {myEvents.slice(0, 8).map((ev) => (
              <li key={earningsEventKey(ev)}>
                <button
                  type="button"
                  onClick={() => onPickEvent(ev)}
                  className="flex h-[34px] w-full min-w-0 items-center gap-1.5 px-1 text-left text-xs hover:bg-muted/50"
                >
                  <StockAvatar
                    symbol={ev.symbol}
                    name={eventName(ev)}
                    currency={ev.currency === "USD" ? "USD" : "KRW"}
                    market={ev.market}
                    size="xs"
                    className="shrink-0"
                  />
                  <span className="min-w-0 flex-1 truncate">{eventName(ev)}</span>
                  {ev.surprise ? (
                    <SurpriseValue surprise={ev.surprise} className="shrink-0 text-[11px]" />
                  ) : (
                    <DateStatusBadge status={ev.dateStatus} />
                  )}
                  <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground">
                    {formatEarningsDateLabel(ev.date)}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
        <p className="mt-1.5 border-t pt-1.5 text-[10px] leading-relaxed text-muted-foreground">
          관심·보유 종목의 <b>지난 2주 발표와 앞으로 한 달 일정</b>입니다. 지난 것은
          서프라이즈를, 앞으로의 것은 예정·추정을 보여줍니다.
        </p>
      </SectionCard>
    </div>
  );
}

/** 대형 발표처럼 「종목 + 오른쪽 값 몇 개」인 목록의 공통 몸통. */
function EventList({
  events,
  empty,
  onPickEvent,
  render,
}: {
  events: EarningsEvent[] | null;
  empty: string;
  onPickEvent: (event: EarningsEvent) => void;
  render: (event: EarningsEvent) => React.ReactNode;
}) {
  if (events === null) return <SectionSkeleton rows={4} />;
  if (events.length === 0) return <SectionEmpty text={empty} />;
  return (
    <ul className="min-w-0">
      {events.map((ev) => (
        <li key={earningsEventKey(ev)}>
          <button
            type="button"
            onClick={() => onPickEvent(ev)}
            className="flex h-[34px] w-full min-w-0 items-center gap-1.5 px-1 text-left text-xs hover:bg-muted/50"
          >
            <StockAvatar
              symbol={ev.symbol}
              name={eventName(ev)}
              currency={ev.currency === "USD" ? "USD" : "KRW"}
              market={ev.market}
              size="xs"
              className="shrink-0"
            />
            <span className="min-w-0 flex-1 truncate">{eventName(ev)}</span>
            {render(ev)}
          </button>
        </li>
      ))}
    </ul>
  );
}

/**
 * 엇갈린 발표 한 줄 — **증권사 예측 → 실제**를 글자로 밝히고, 그 아래 주가를 둔다.
 *
 * 퍼센트 두 개만 나란히 두면 어느 쪽이 실적이고 어느 쪽이 주가인지 알 수 없다.
 * 라벨을 붙여야 이 카드가 하려는 말(「예측은 이랬는데 실제는 이랬고, 그런데 주가는
 * 반대로 갔다」)이 전달된다.
 */
function DivergentRow({
  event,
  onPick,
}: {
  event: EarningsEvent;
  onPick: (event: EarningsEvent) => void;
}) {
  const currency = event.currency === "USD" ? "USD" : "KRW";
  const s = event.surprise;
  const isEps = s?.metric === "eps";
  const fmt = (v: number) =>
    isEps ? `${currency === "USD" ? "$" : ""}${v.toFixed(2)}` : formatFinancialAmount(v, currency);

  return (
    <li className="border-b last:border-0">
      <button
        type="button"
        onClick={() => onPick(event)}
        className="w-full min-w-0 px-1 py-1.5 text-left hover:bg-muted/50"
      >
        <span className="flex min-w-0 items-center gap-1.5">
          <StockAvatar
            symbol={event.symbol}
            name={eventName(event)}
            currency={currency}
            market={event.market}
            size="xs"
            className="shrink-0"
          />
          <span className="min-w-0 flex-1 truncate text-xs font-medium">{eventName(event)}</span>
          <span className="shrink-0 font-mono text-[9px] text-muted-foreground">
            {periodLabel(event)}
          </span>
        </span>
        {s && (
          <span className="mt-0.5 flex min-w-0 flex-wrap items-baseline gap-x-1 pl-[26px] text-[10px]">
            <span className="text-muted-foreground">증권사 예측</span>
            <span className="font-mono text-muted-foreground">{fmt(s.estimate)}</span>
            <span className="text-muted-foreground">→ 실제</span>
            <span className="font-mono font-semibold">{fmt(s.actual)}</span>
            <SurpriseValue surprise={s} className="text-[10px]" />
          </span>
        )}
        <span className="mt-0.5 flex items-baseline gap-1 pl-[26px] text-[10px]">
          <span className="text-muted-foreground">그 뒤 주가</span>
          <ReactionValue
            reaction={event.reaction}
            status={event.dateStatus}
            className="text-[10px]"
          />
        </span>
      </button>
    </li>
  );
}

function ConsistencyRow({ row }: { row: EarningsConsistency }) {
  const rate = (row.history.beats / row.history.quarters) * 100;
  return (
    <li className="flex h-[32px] min-w-0 items-center gap-1.5 px-1 text-xs">
      <StockAvatar
        symbol={row.symbol}
        name={row.name || row.symbol}
        currency={row.market === "us" ? "USD" : "KRW"}
        market={row.market}
        size="xs"
        className="shrink-0"
      />
      <span className="min-w-0 flex-1 truncate">{row.name || row.symbol}</span>
      <span
        className={cn(
          "shrink-0 font-mono text-[11px] font-bold tabular-nums",
          rate >= 50 ? "text-[var(--profit)]" : "text-[var(--loss)]",
        )}
        title={`최근 ${row.history.quarters}분기 중 ${row.history.beats}번 상회`}
      >
        {row.history.beats}/{row.history.quarters}
      </span>
      {row.history.medianPct != null && (
        <span
          className="shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground"
          title="서프라이즈 중위값 — 평균은 한 분기의 극단에 끌린다"
        >
          중위 {row.history.medianPct >= 0 ? "+" : ""}
          {row.history.medianPct.toFixed(1)}%
        </span>
      )}
    </li>
  );
}

/**
 * 레일 3종을 **한 요청**으로 받는다.
 *
 * 카드마다 부르면 레일 하나에 세 왕복이고, 셋이 서버에서 같은 표를 읽으므로
 * 한 번에 받는 편이 싸다. 실패하면 셋 다 비지만 전부 부가 정보라 본문은 그대로 뜬다.
 */
function useHighlights(market: EarningsMarket): EarningsHighlights | null {
  // 키드 상태 — 이펙트에서 동기로 null을 넣으면 캐스케이드 렌더가 된다.
  const [loaded, setLoaded] = useState<{ key: string; data: EarningsHighlights } | null>(null);

  useEffect(() => {
    let alive = true;
    fetchEarningsHighlights(market)
      .then((data) => {
        if (alive) setLoaded({ key: market, data });
      })
      .catch(() => {
        if (alive) {
          setLoaded({ key: market, data: { big: [], divergent: [], consistent: [] } });
        }
      });
    return () => {
      alive = false;
    };
  }, [market]);

  return loaded?.key === market ? loaded.data : null;
}
