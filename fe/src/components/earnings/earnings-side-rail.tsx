"use client";

import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { HelpHint } from "@/components/ui/help-hint";
import { MajorEarningsCard } from "@/components/earnings/major-earnings-card";
import { useEarningsHighlights } from "@/hooks/use-earnings-highlights";
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
import type {
  EarningsConsistency,
  EarningsEvent,
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
  /* 카드 셋이 한 응답을 나눠 쓴다 — 훅은 `hooks/use-earnings-highlights`로 나갔다. */
  const highlights = useEarningsHighlights(market);

  return (
    <div className="space-y-3">
      {/*
        홈의 「주요 실적 일정」과 **같은 부품**이다 (2026-09-05 사용자 요청 7).
        레일에서는 서버가 준 시가총액 순 그대로 두고, 오른쪽에 확정 배지·날짜·시가총액을
        적는다 — 표가 이미 날짜순이라 여기서 답할 것은 「무엇이 큰 발표인가」다.
      */}
      <MajorEarningsCard
        variant="rail"
        events={highlights?.big ?? null}
        onPick={onPickEvent}
      />

      <SectionCard
        title={
          <span className="flex items-center gap-1">
            기대와 주가가 엇갈린 발표
            <HelpHint>증권사 예측 대비 발표가 벌어진 방향과 그 뒤 주가가 간 방향이 서로 반대인 발표입니다.</HelpHint>
          </span>
        }
      >
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
      </SectionCard>

      <SectionCard
        title={
          <span className="flex items-center gap-1">
            꾸준히 기대를 넘는 기업
            <span className="font-normal text-muted-foreground">최근 4분기</span>
          </span>
        }
      >
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
      </SectionCard>

      <SectionCard
        title={
          <span className="flex items-center gap-1">
            내 종목 최근 발표
            <span className="font-normal text-muted-foreground">지난 2주 · 앞으로 한 달</span>
          </span>
        }
      >
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
                  className="flex h-[34px] w-full min-w-0 items-center gap-1.5 px-1 text-left text-cell hover:bg-muted/50"
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
                    <SurpriseValue surprise={ev.surprise} className="shrink-0 text-cell" />
                  ) : (
                    <DateStatusBadge status={ev.dateStatus} />
                  )}
                  <span className="shrink-0 font-num text-micro tabular-nums text-muted-foreground">
                    {formatEarningsDateLabel(ev.date)}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>
    </div>
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
    /*
      아래 두 줄에는 툴팁 트리거가 들어 있고 그것은 포커스를 받는다
      (`earnings-shared.tsx`의 `HintTooltip`). 예전에는 이 줄 전체가 하나의
      `<button>`이라 **버튼 안에 버튼**이 됐다(axe `nested-interactive`).
      그래서 **종목 줄만** 버튼으로 두고 수치 줄은 형제로 뺀다. 호버 강조는
      바깥 껍데기가 들고 있어 한 덩어리로 보인다.
    */
    <li className="border-b last:border-0">
      <div className="px-1 py-1.5 hover:bg-muted/50">
      <button
        type="button"
        onClick={() => onPick(event)}
        className="w-full min-w-0 cursor-pointer bg-transparent p-0 text-left focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-[var(--brand)]"
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
          <span className="min-w-0 flex-1 truncate text-cell font-medium">{eventName(event)}</span>
          <span className="shrink-0 font-num text-micro text-muted-foreground">
            {periodLabel(event)}
          </span>
        </span>
      </button>
        {s && (
          <span className="mt-0.5 flex min-w-0 flex-wrap items-baseline gap-x-1 pl-[26px] text-micro">
            <span className="text-muted-foreground">증권사 예측</span>
            <span className="font-num text-muted-foreground">{fmt(s.estimate)}</span>
            <span className="text-muted-foreground">→ 실제</span>
            <span className="font-num font-semibold">{fmt(s.actual)}</span>
            <SurpriseValue surprise={s} className="text-micro" />
          </span>
        )}
        <span className="mt-0.5 flex items-baseline gap-1 pl-[26px] text-micro">
          <span className="text-muted-foreground">그 뒤 주가</span>
          <ReactionValue
            reaction={event.reaction}
            status={event.dateStatus}
            currency={currency}
            className="text-micro"
          />
        </span>
      </div>
    </li>
  );
}

function ConsistencyRow({ row }: { row: EarningsConsistency }) {
  const rate = (row.history.beats / row.history.quarters) * 100;
  return (
    <li className="flex h-[32px] min-w-0 items-center gap-1.5 px-1 text-cell">
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
          "shrink-0 font-num text-cell font-bold tabular-nums",
          rate >= 50 ? "text-[var(--profit)]" : "text-[var(--loss)]",
        )}
        title={`최근 ${row.history.quarters}분기 중 ${row.history.beats}번 상회`}
      >
        {row.history.beats}/{row.history.quarters}
      </span>
      {row.history.medianPct != null && (
        <span
          className="shrink-0 font-num text-micro tabular-nums text-muted-foreground"
          title="서프라이즈 중위값 — 평균은 한 분기의 극단에 끌린다"
        >
          중위 {row.history.medianPct >= 0 ? "+" : ""}
          {row.history.medianPct.toFixed(1)}%
        </span>
      )}
    </li>
  );
}

