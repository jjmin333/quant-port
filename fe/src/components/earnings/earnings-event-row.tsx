"use client";

import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  DateStatusBadge,
  DivergenceFlag,
  EstimateActualPair,
  ReactionValue,
  SessionBadge,
  SurpriseValue,
  eventName,
  periodLabel,
} from "@/components/earnings/earnings-shared";
import { formatEarningsDateLabel } from "@/lib/earnings-date";
import type { EarningsEvent } from "@/lib/earnings-types";
import { cn } from "@/lib/utils";

/**
 * 캘린더 본문 한 줄.
 *
 * 한 줄이 답하는 것은 넷이다 — **언제**(날짜 + 확정/예정/추정) · **누가**(종목) ·
 * **얼마나 빗나갔나**(기대 → 발표 → 서프라이즈) · **그래서 주가는**(반응).
 *
 * 마지막 열이 이 화면의 요지다. 서프라이즈와 반응이 **엇갈린** 줄(상회했는데
 * 하락)이 가장 값어치 있는 정보이고, 두 열을 나란히 두지 않으면 알 수 없다.
 *
 * 추정 행에는 근거를 같은 줄에 적는다(`estimateBasis`). 「9월 2일 현대차 3Q」만
 * 적어 두면 회사가 그날을 공시한 것처럼 보이는데, 국내에는 그런 공시가 없다.
 */
export function EarningsEventRow({
  event,
  onPickEvent,
}: {
  event: EarningsEvent;
  onPickEvent: (event: EarningsEvent) => void;
}) {
  const name = eventName(event);
  const estimated = event.dateStatus === "estimated";

  return (
    <li className="border-b last:border-0">
      <button
        type="button"
        onClick={() => onPickEvent(event)}
        className="flex w-full min-w-0 items-start gap-2 px-1 py-2 text-left hover:bg-muted/50"
      >
        <div className="flex w-[52px] shrink-0 flex-col items-end gap-0.5 pt-0.5">
          <span
            className={cn(
              "font-num text-cell font-semibold tabular-nums",
              estimated ? "text-[var(--sample)]" : "text-foreground",
            )}
          >
            {formatEarningsDateLabel(event.date)}
          </span>
          <span className="font-num text-micro tabular-nums text-muted-foreground">
            {event.date.slice(5).replace("-", "/")}
          </span>
        </div>

        <StockAvatar
          symbol={event.symbol}
          name={name}
          currency={event.currency === "USD" ? "USD" : "KRW"}
          market={event.market}
          size="sm"
          className="mt-0.5 shrink-0"
        />

        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5">
            <span className="truncate text-cell font-medium text-foreground">{name}</span>
            <span className="shrink-0 font-num text-micro text-muted-foreground">
              {periodLabel(event)}
            </span>
            <DateStatusBadge status={event.dateStatus} />
            <SessionBadge session={event.session} />
            <DivergenceFlag event={event} />
          </div>

          <div className="mt-0.5 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5">
            {event.surprise ? (
              <EstimateActualPair surprise={event.surprise} currency={event.currency} />
            ) : estimated ? (
              <span className="text-micro text-[var(--sample)]">{event.estimateBasis}</span>
            ) : (
              <span className="text-micro text-muted-foreground">
                {/* 예정 행은 아직 비교할 실적이 없다. 컨센서스만 있으면 그것을
                    적어 "무엇이 기대되는가"를 알려 준다. */}
                {expectationText(event)}
              </span>
            )}
            {event.revisedAt && (
              <span
                className="shrink-0 text-micro text-muted-foreground"
                title={`${event.revisedAt}에 정정 공시됨 — 수치는 정정본, 날짜는 원 발표일입니다`}
              >
                정정 {event.revisedAt.slice(5).replace("-", "/")}
              </span>
            )}
            {/* 접어 둔 개별 기준이 있다는 사실은 목록에서도 알려 준다 — 모달을
                열어야만 알 수 있으면 아무도 모른다. */}
            {event.alternates && event.alternates.length > 0 && (
              <span
                className="shrink-0 text-micro text-muted-foreground"
                title="같은 분기의 다른 기준(개별) 발표도 있습니다 — 눌러서 확인"
              >
                +{event.alternates.map((a) => a.basis || "기타").join("·")}
              </span>
            )}
          </div>
        </div>

        <div className="flex shrink-0 flex-col items-end gap-0.5 pt-0.5">
          <SurpriseValue surprise={event.surprise} className="text-cell" />
          <ReactionValue
            reaction={event.reaction}
            status={event.dateStatus}
            currency={event.currency}
            className="text-cell"
          />
        </div>
      </button>
    </li>
  );
}

/** 예정 행의 한 줄 — 컨센서스가 있으면 무엇이 기대되는지, 없으면 조용히 비운다. */
function expectationText(event: EarningsEvent): string {
  const eps = event.eps?.estimate;
  if (eps != null) {
    return `EPS 컨센서스 ${event.currency === "USD" ? "$" : ""}${eps.toFixed(2)}`;
  }
  const op = event.opIncome?.estimate;
  if (op != null) {
    return `영업이익 컨센서스 ${(op / 1e8).toLocaleString("ko-KR", { maximumFractionDigits: 0 })}억원`;
  }
  return "";
}
