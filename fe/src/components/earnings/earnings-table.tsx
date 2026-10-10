"use client";

import { useMemo, useState } from "react";
import { ArrowDown, ArrowUp, ChevronsUpDown } from "lucide-react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { EarningsEventCard } from "@/components/earnings/earnings-event-card";
import {
  DateStatusBadge,
  DivergenceFlag,
  EarningsHint,
  ReactionValue,
  REACTION_COLUMN_HINT,
  SurpriseValue,
  earningsEventKey,
  eventName,
} from "@/components/earnings/earnings-shared";
import type { EarningsEvent, EarningsMarket } from "@/lib/earnings-types";
import { formatShortDate } from "@/lib/earnings-range";
import { formatFinancialAmount } from "@/lib/format";
import type { QuoteSummary } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

/**
 * 실적 표 — **열이 곧 정렬 기준**이다.
 *
 * 예전에는 한 행이 2줄짜리 카드였다. 그러면 세로로 훑는 것밖에 못 하고,
 * 「영업이익 큰 순」·「서프라이즈 순」·「시가총액 순」으로 줄을 세울 수 없다.
 * 실적 화면에서 사람이 실제로 하는 일이 그 줄 세우기다.
 *
 * # 지표는 시장이 정한다
 *
 * 국내는 **영업이익**, 해외는 **EPS + 발표 시각**. 메뉴를 국내·해외로 가른 덕에
 * 한 표에 두 기준이 섞이지 않는다.
 *
 * # 「예측/실적」을 한 칸에 둔다
 *
 * 두 열로 쪼개면 눈이 좌우로 왕복해야 한다. 붙여 두면 「1조 519억 / **1조 3,442억**」이
 * 한 번에 읽힌다 — 실적은 bold. 국내는 「예측 / 실적」, 해외는 investing.com 관행대로
 * 「실적 / 예측」. 매출도 같은 짝이다 — 실적만 두면 컨센서스를 보러 옆 칸을 왕복한다.
 *
 * # 기본 정렬
 *
 * **시가총액 내림차순**으로 시작한다 — 큰 회사부터 본다.
 *
 * # 좁으면 카드 (2026-10-09 사용자 요청 3)
 *
 * 열이 열 개라 폰·세로 태블릿에서 가로로 밀렸다. 표가 놓인 **칸의 폭**(컨테이너 쿼리)이
 * 46rem 미만이면 같은 줄을 `EarningsEventCard`로 세우고, 머리글 정렬은 선택 상자가 맡는다.
 */

type SortKey =
  | "name"
  | "date"
  | "metricActual"
  | "surprise"
  | "revenue"
  | "revenueSurprise"
  | "marketCap"
  | "price"
  | "session"
  | "reaction";

type SortDir = "asc" | "desc";

type Column = {
  key: SortKey;
  label: string;
  sub?: string;
  align: "left" | "right";
  /** 해외에만 있는 열은 국내 표에서 아예 그리지 않는다(빈 열을 두지 않는다). */
  markets?: EarningsMarket[];
};

function defaultSort(): { key: SortKey; dir: SortDir } {
  return { key: "marketCap", dir: "desc" };
}

function columnsFor(market: EarningsMarket): Column[] {
  const metric = market === "us" ? "EPS" : "영업이익";
  const pairSub = market === "domestic" ? "예측 / 실적" : "실적 / 예측";
  const cols: Column[] = [
    { key: "name", label: "회사", align: "left" },
    { key: "date", label: "발표일", align: "right" },
    // 시간(장전/장후)은 **해외에만** 있다. 국내 소스에는 발표 시각이 아예 없어
    // 열을 두면 그 시장에서 영원히 빈 칸이 된다.
    ...(market === "us"
      ? [{ key: "session" as SortKey, label: "시간", align: "right" as const }]
      : []),
    { key: "marketCap", label: "시가총액", align: "right" },
    { key: "metricActual", label: metric, sub: pairSub, align: "right" },
    { key: "surprise", label: `${metric} 서프라이즈`, align: "right" },
    {
      key: "revenue",
      label: "매출",
      sub: pairSub,
      align: "right",
    },
    { key: "revenueSurprise", label: "매출 서프라이즈", align: "right" },
    { key: "price", label: "현재가", align: "right" },
    { key: "reaction", label: "발표 후 주가", align: "right" },
  ];
  return cols;
}

export function EarningsTable({
  events,
  market,
  quotes,
  onPickEvent,
}: {
  events: EarningsEvent[];
  market: EarningsMarket;
  /** 심볼 → 시세. 표에 보이는 행만큼 배치로 한 번 받아 온다. */
  quotes: Map<string, QuoteSummary>;
  onPickEvent: (event: EarningsEvent) => void;
}) {
  const [sort, setSort] = useState<{ key: SortKey; dir: SortDir }>(defaultSort);

  const columns = useMemo(() => columnsFor(market), [market]);

  const toggle = (key: SortKey) => {
    setSort((cur) =>
      cur.key === key
        ? { key, dir: cur.dir === "asc" ? "desc" : "asc" }
        : // 숫자 열은 **큰 것부터** 보고 싶다. 이름·날짜만 오름차순으로 시작한다.
          { key, dir: key === "name" || key === "date" ? "asc" : "desc" },
    );
  };

  const rows = useMemo(() => {
    const copy = [...events];
    copy.sort((a, b) => {
      const av = sortValue(a, sort.key, market, quotes);
      const bv = sortValue(b, sort.key, market, quotes);
      // 값이 없는 행은 방향과 무관하게 **항상 아래**로. 위로 올리면 정렬을
      // 눌렀는데 빈 칸부터 보이게 된다.
      if (av == null && bv == null) return 0;
      if (av == null) return 1;
      if (bv == null) return -1;
      if (typeof av === "string" || typeof bv === "string") {
        const cmp = String(av).localeCompare(String(bv), "ko");
        return sort.dir === "asc" ? cmp : -cmp;
      }
      return sort.dir === "asc" ? av - bv : bv - av;
    });
    return copy;
  }, [events, sort, market, quotes]);

  const sortLabel = (col: Column) => (col.key === "metricActual" || col.key === "revenue" ? `${col.label} 실적` : col.label);

  return (
    <div className="@container">
      <div className="space-y-2 @[46rem]:hidden">
        <div className="flex items-center justify-end gap-1.5 text-cell">
          <label htmlFor="earnings-sort" className="text-muted-foreground">
            정렬
          </label>
          <select
            id="earnings-sort"
            value={sort.key}
            onChange={(e) => toggle(e.target.value as SortKey)}
            className="h-8 rounded-md border border-[var(--border)] bg-card px-2 text-cell"
          >
            {columns.map((col) => (
              <option key={col.key} value={col.key}>
                {sortLabel(col)}
              </option>
            ))}
          </select>
          <button
            type="button"
            onClick={() => setSort((cur) => ({ ...cur, dir: cur.dir === "asc" ? "desc" : "asc" }))}
            aria-label={sort.dir === "asc" ? "오름차순 — 눌러서 내림차순" : "내림차순 — 눌러서 오름차순"}
            className="grid size-8 place-items-center rounded-md border border-[var(--border)] bg-card"
          >
            {sort.dir === "asc" ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />}
          </button>
        </div>
        {rows.map((event) => (
          <EarningsEventCard
            key={earningsEventKey(event)}
            event={event}
            quote={quotes.get(event.symbol)}
            onPick={onPickEvent}
          />
        ))}
      </div>
    <div className="hidden overflow-x-auto @[46rem]:block">
      <table className="w-full min-w-[900px] border-collapse text-cell">
        <thead>
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                onClick={() => toggle(col.key)}
                className={cn(
                  "cursor-pointer select-none whitespace-nowrap border-b px-2 py-1.5 text-cell font-medium",
                  col.align === "left" ? "text-left" : "text-right",
                  sort.key === col.key
                    ? "font-bold text-foreground"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                <span className="inline-flex items-center gap-0.5">
                  {col.key === "reaction" ? (
                    <EarningsHint content={REACTION_COLUMN_HINT}>{col.label}</EarningsHint>
                  ) : (
                    col.label
                  )}
                  {col.sub && (
                    <span className="font-normal text-muted-foreground">{col.sub}</span>
                  )}
                  <SortIcon active={sort.key === col.key} dir={sort.dir} />
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((event) => (
            <Row
              key={earningsEventKey(event)}
              event={event}
              market={market}
              quote={quotes.get(event.symbol)}
              onPick={onPickEvent}
            />
          ))}
        </tbody>
      </table>
    </div>
    </div>
  );
}

/**
 * 발표 시각. 미공개(빈 값)를 「장전」·「장후」 중 하나로 찍지 않는다 — 미국 실적은
 * 장 밖에서 나오고, 장전이냐 장후냐에 따라 주가에 반영되는 날이 하루 다르다.
 */
const SESSION_LABEL: Record<string, string> = {
  bmo: "장전",
  amc: "장후",
  dmh: "장중",
};

/** 정렬용 순서 — 하루의 흐름대로 장전 → 장중 → 장후, 미공개는 맨 뒤. */
const SESSION_ORDER: Record<string, number> = { bmo: 0, dmh: 1, amc: 2 };

function SortIcon({ active, dir }: { active: boolean; dir: SortDir }) {
  if (!active) return <ChevronsUpDown className="size-3 opacity-35" />;
  return dir === "asc" ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
}

function Row({
  event,
  market,
  quote,
  onPick,
}: {
  event: EarningsEvent;
  market: EarningsMarket;
  quote?: QuoteSummary;
  onPick: (event: EarningsEvent) => void;
}) {
  const currency = event.currency === "USD" ? "USD" : "KRW";
  const metric = (market === "us" ? event.eps : event.opIncome) ?? {};
  const estimated = event.dateStatus === "estimated";

  return (
    <tr
      onClick={() => onPick(event)}
      className="cursor-pointer border-b border-border/55 hover:bg-muted/50"
    >
      <td className="max-w-[260px] px-2 py-1.5">
        <span className="flex min-w-0 items-center gap-1.5">
          <StockAvatar
            symbol={event.symbol}
            name={eventName(event)}
            currency={currency}
            market={event.market}
            size="xs"
            className="shrink-0"
          />
          <span className="min-w-0 truncate font-medium">{eventName(event)}</span>
          {/* 연결/개별은 **작은 글자**로. 배지로 만들면 다시 칩이 된다. */}
          <span className="shrink-0 font-num text-micro text-muted-foreground">
            {event.fiscalPeriod === "FY" ? "연간" : event.fiscalPeriod}
            {event.basis ? `·${event.basis}` : ""}
          </span>
          <DivergenceFlag event={event} />
        </span>
      </td>

      <td className="whitespace-nowrap px-2 py-1.5 text-right">
        <span className={cn("font-num", estimated && "text-[var(--sample)]")}>
          {formatShortDate(event.date)}
        </span>
        {event.dateStatus !== "confirmed" && (
          <span className="ml-1 align-middle">
            <DateStatusBadge status={event.dateStatus} />
          </span>
        )}
      </td>

      {market === "us" && (
        <td className="whitespace-nowrap px-2 py-1.5 text-right text-muted-foreground">
          {SESSION_LABEL[event.session ?? ""] ?? "—"}
        </td>
      )}

      <td className="whitespace-nowrap px-2 py-1.5 text-right font-num text-muted-foreground">
        {event.marketCap != null ? formatFinancialAmount(event.marketCap, currency) : "—"}
      </td>

      <td className="whitespace-nowrap px-2 py-1.5 text-right font-num">
        <MetricPairCell
          pair={metric}
          market={market}
          format={(v) => metricText(v, market, currency)}
        />
      </td>

      <td className="whitespace-nowrap px-2 py-1.5 text-right">
        <SurpriseValue surprise={event.surprise} className="text-cell" />
      </td>

      <td className="whitespace-nowrap px-2 py-1.5 text-right font-num">
        <MetricPairCell
          pair={event.revenue}
          market={market}
          format={(v) => formatFinancialAmount(v, currency)}
        />
      </td>

      <td className="whitespace-nowrap px-2 py-1.5 text-right">
        <SurpriseValue surprise={event.revenueSurprise} className="text-cell" />
      </td>

      <td className="whitespace-nowrap px-2 py-1.5 text-right font-num text-cell tabular-nums">
        {quote ? (
          <>
            <span className="font-semibold">{quote.currentPrice.toLocaleString("ko-KR")}</span>{" "}
            <span
              className={cn(
                "font-semibold",
                quote.changeRate >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
              )}
            >
              {quote.changeRate >= 0 ? "+" : ""}
              {quote.changeRate.toFixed(2)}%
            </span>
          </>
        ) : (
          <span className="text-muted-foreground">—</span>
        )}
      </td>

      <td
        className="whitespace-nowrap px-2 py-1.5 text-right"
        onClick={(e) => e.stopPropagation()}
        onPointerDown={(e) => e.stopPropagation()}
      >
        <ReactionValue
          reaction={event.reaction}
          status={event.dateStatus}
          currency={currency}
          className="text-cell"
        />
      </td>
    </tr>
  );
}

/** EPS는 통화 축약(조·억)이 말이 안 되므로 소수점으로 적는다. */
function metricText(value: number, market: EarningsMarket, currency: "KRW" | "USD"): string {
  if (market === "us") return `${currency === "USD" ? "$" : ""}${value.toFixed(2)}`;
  return formatFinancialAmount(value, currency);
}

/** 국내: 예측 / **실적**. 해외: **실적** / 예측. */
function MetricPairCell({
  pair,
  market,
  format,
}: {
  pair: { estimate?: number | null; actual?: number | null };
  market: EarningsMarket;
  format: (value: number) => string;
}) {
  const estimateFirst = market === "domestic";

  if (pair.actual == null && pair.estimate == null) {
    return <span className="text-muted-foreground">—</span>;
  }

  const actualText = pair.actual != null ? format(pair.actual) : "—";
  const estimateText = pair.estimate != null ? format(pair.estimate) : "—";

  if (estimateFirst) {
    return (
      <>
        <span className="text-muted-foreground">{estimateText}</span>
        {" / "}
        <b>{actualText}</b>
      </>
    );
  }
  return (
    <>
      <b>{actualText}</b>
      <span className="text-muted-foreground">
        {" / "}
        {estimateText}
      </span>
    </>
  );
}

function sortValue(
  event: EarningsEvent,
  key: SortKey,
  market: EarningsMarket,
  quotes: Map<string, QuoteSummary>,
): number | string | null {
  switch (key) {
    case "name":
      return eventName(event);
    case "date":
      return event.date;
    case "metricActual": {
      const metric = market === "us" ? event.eps : event.opIncome;
      return metric.actual ?? metric.estimate ?? null;
    }
    case "surprise":
      return event.surprise?.pct ?? null;
    case "revenue":
      return event.revenue.actual ?? event.revenue.estimate ?? null;
    case "revenueSurprise":
      return event.revenueSurprise?.pct ?? null;
    case "marketCap":
      return event.marketCap ?? null;
    case "price":
      return quotes.get(event.symbol)?.changeRate ?? null;
    case "session":
      return event.session ? (SESSION_ORDER[event.session] ?? null) : null;
    case "reaction":
      return event.reaction?.pct ?? null;
  }
}
