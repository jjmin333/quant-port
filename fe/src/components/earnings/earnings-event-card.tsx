"use client";

import type { ReactNode } from "react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  DateStatusBadge,
  DivergenceFlag,
  ReactionValue,
  SessionBadge,
  SurpriseValue,
  eventName,
} from "@/components/earnings/earnings-shared";
import { formatShortDate } from "@/lib/earnings-range";
import type { EarningsEvent, EarningsMetricPair, EarningsSurprise } from "@/lib/earnings-types";
import { formatFinancialAmount } from "@/lib/format";
import type { QuoteSummary } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

/**
 * 실적 발표 한 건을 **카드**로 — 좁은 화면에서 표 대신 선다 (2026-10-09 사용자 요청 3).
 *
 * 표(`EarningsTable` · 종목 실적 탭)는 열이 일곱~열 개라 폰·세로 태블릿에서 가로로 밀렸다.
 * 카드는 같은 값을 **지표 한 줄씩** 세운다: 이름 · 지표(기대 → 실제 · 서프라이즈) · 발표 후 주가.
 * 국내는 영업이익, 해외는 EPS가 헤드라인이고 매출이 그 아래다 — 표의 열 순서와 같다.
 *
 * `title`을 주면 회사 대신 그것을 머리에 쓴다(종목 실적 탭: 회사는 이미 안다, 「2026년 2분기」).
 */
export function EarningsEventCard({
  event,
  quote,
  title,
  onPick,
}: {
  event: EarningsEvent;
  quote?: QuoteSummary;
  title?: ReactNode;
  onPick?: (event: EarningsEvent) => void;
}) {
  const currency = event.currency === "USD" ? "USD" : "KRW";
  const us = event.market === "us";
  const estimated = event.dateStatus === "estimated";
  const money = (v: number) => formatFinancialAmount(v, currency);
  const eps = (v: number) => `${currency === "USD" ? "$" : ""}${v.toFixed(2)}`;
  const Tag = onPick ? "button" : "div";

  return (
    <Tag
      {...(onPick ? { type: "button" as const, onClick: () => onPick(event) } : {})}
      className={cn(
        "block w-full min-w-0 rounded-lg border border-[var(--border)] bg-card p-2.5 text-left",
        onPick && "hover:bg-muted/50",
      )}
    >
      <div className="flex min-w-0 items-center gap-1.5">
        {title ?? (
          <>
            <StockAvatar
              symbol={event.symbol}
              name={eventName(event)}
              currency={currency}
              market={event.market}
              size="xs"
              className="shrink-0"
            />
            <span className="min-w-0 truncate text-cell font-semibold">{eventName(event)}</span>
          </>
        )}
        <span className="shrink-0 font-num text-micro text-muted-foreground">
          {event.fiscalPeriod === "FY" ? "연간" : event.fiscalPeriod}
          {event.basis ? `·${event.basis}` : ""}
        </span>
        <DivergenceFlag event={event} />
        <span className="ml-auto flex shrink-0 items-center gap-1">
          <span className={cn("font-num text-cell tabular-nums", estimated && "text-[var(--sample)]")}>
            {formatShortDate(event.date)}
          </span>
          <SessionBadge session={event.session} />
          {event.dateStatus !== "confirmed" && <DateStatusBadge status={event.dateStatus} />}
        </span>
      </div>

      <dl className="mt-1.5 space-y-1">
        <MetricLine
          label={us ? "EPS" : "영업이익"}
          pair={us ? event.eps : event.opIncome}
          surprise={event.surprise}
          format={us ? eps : money}
        />
        <MetricLine label="매출" pair={event.revenue} surprise={event.revenueSurprise} format={money} />
        {!us && event.netIncome?.actual != null && (
          <MetricLine label="순이익" pair={event.netIncome} format={money} />
        )}
      </dl>

      {(event.marketCap != null || quote || event.dateStatus === "confirmed") && (
        <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 border-t border-[var(--border)]/60 pt-1.5 text-micro text-muted-foreground">
          {event.marketCap != null && (
            <span>
              시총 <span className="font-num text-foreground">{money(event.marketCap)}</span>
            </span>
          )}
          {quote && (
            <span className="font-num tabular-nums">
              현재 <b className="text-foreground">{quote.currentPrice.toLocaleString("ko-KR")}</b>{" "}
              <b className={quote.changeRate >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]"}>
                {quote.changeRate >= 0 ? "+" : ""}
                {quote.changeRate.toFixed(2)}%
              </b>
            </span>
          )}
          {event.dateStatus === "confirmed" && (
            <span className="ml-auto flex items-center gap-1">
              발표 후
              <ReactionValue reaction={event.reaction} status={event.dateStatus} currency={currency} className="text-cell" />
            </span>
          )}
        </div>
      )}
    </Tag>
  );
}

/** 「영업이익  10.1조 → 12.1조  +19.6%」. 둘 다 없으면 줄을 그리지 않는다. */
function MetricLine({
  label,
  pair,
  surprise,
  format,
}: {
  label: string;
  pair?: EarningsMetricPair;
  surprise?: EarningsSurprise;
  format: (v: number) => string;
}) {
  if (pair?.actual == null && pair?.estimate == null) return null;
  return (
    <div className="flex min-w-0 items-baseline gap-2 text-cell">
      <dt className="w-12 shrink-0 text-muted-foreground">{label}</dt>
      <dd className="min-w-0 flex-1 truncate font-num tabular-nums">
        {pair.estimate != null && (
          <span className="text-muted-foreground">
            {pair.actual != null ? "" : "기대 "}
            {format(pair.estimate)}
            {pair.actual != null && " → "}
          </span>
        )}
        {pair.actual != null && <b>{format(pair.actual)}</b>}
      </dd>
      {surprise && <SurpriseValue surprise={surprise} className="shrink-0 text-cell" />}
    </div>
  );
}
