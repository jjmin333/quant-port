"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight, ChevronDown } from "lucide-react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  DateStatusBadge,
  DivergenceFlag,
  ReactionValue,
  SURPRISE_METRIC_LABEL,
  SessionBadge,
  SurpriseValue,
  eventName,
  periodLabel,
} from "@/components/earnings/earnings-shared";
import { EarningsQuarterBars } from "@/components/earnings/earnings-quarter-bars";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { fetchEarningsClient } from "@/lib/earnings-api";
import type { EarningsEvent, EarningsTabData } from "@/lib/earnings-types";
import { formatShortDate } from "@/lib/earnings-range";
import { formatFinancialAmount } from "@/lib/format";
import { cn } from "@/lib/utils";

/**
 * 실적 전용 상세 모달 (W2).
 *
 * # 왜 시세 모달이 아닌가
 *
 * 지금까지 실적 캘린더에서 행을 누르면 `SymbolQuickLookModal`(현재가·차트)이
 * 열렸다. 실적 화면에서 종목을 누른 사람이 보고 싶은 것은 시세가 아니라 **이 회사의
 * 실적 이력**이다 — 매번 기대를 넘기는 회사인지, 지난 분기에 무엇을 했는지,
 * 다음은 언제인지.
 *
 * # 무엇을 보여주나
 *
 * 목록이 이미 들고 있는 값(`event`)으로 **즉시** 헤더와 반응을 그리고, 이력(분기
 * 추이·적중 이력)은 열 때 한 번 부른다. 목록에 300줄이 있어도 요청은 연 것 하나뿐이다.
 */
export function EarningsDetailModal({
  event,
  onClose,
}: {
  /** null이면 닫힌 상태. */
  event: EarningsEvent | null;
  onClose: () => void;
}) {
  const symbol = event?.symbol ?? null;
  const [loaded, setLoaded] = useState<{ symbol: string; data: EarningsTabData | null } | null>(
    null,
  );

  useEffect(() => {
    if (!symbol) return;
    let alive = true;
    fetchEarningsClient(symbol)
      .then((data) => {
        if (alive) setLoaded({ symbol, data });
      })
      .catch(() => {
        // 이력을 못 받아도 모달은 뜬다 — 목록이 준 값만으로도 헤더는 완전하다.
        if (alive) setLoaded({ symbol, data: null });
      });
    return () => {
      alive = false;
    };
  }, [symbol]);

  const detail = loaded?.symbol === symbol ? loaded.data : null;
  const loading = symbol != null && loaded?.symbol !== symbol;

  return (
    <Dialog open={event != null} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="max-h-[88vh] overflow-y-auto sm:max-w-[640px]">
        {event && (
          <>
            <DialogHeader className="px-4 pr-10">
              <DialogTitle className="flex min-w-0 items-center gap-2 text-left">
                <StockAvatar
                  symbol={event.symbol}
                  name={eventName(event)}
                  currency={event.currency === "USD" ? "USD" : "KRW"}
                  market={event.market}
                  size="sm"
                  className="shrink-0"
                />
                <span className="min-w-0 truncate">{eventName(event)}</span>
                <span className="shrink-0 font-mono text-[11px] font-normal text-muted-foreground">
                  {event.symbol}
                </span>
              </DialogTitle>
              <DialogDescription className="text-left text-[11px]">
                {periodLabel(event)} · {event.date.replace(/-/g, ".")} 발표
                {event.revisedAt && (
                  <span title={`${event.revisedAt}에 정정 공시 — 수치는 정정본입니다`}>
                    {" "}
                    (정정 {formatShortDate(event.revisedAt)})
                  </span>
                )}
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-3 px-4 pb-4">
              <HeadlineRow event={event} />
              <MetricTiles event={event} />

              {loading ? (
                <div className="h-[120px] animate-pulse rounded-lg bg-muted" />
              ) : (
                <>
                  <EarningsQuarterBars
                    announcements={detail?.announcements ?? [event]}
                    calendar={detail?.calendar ?? []}
                    currency={event.currency}
                  />
                  <HistoryStrip detail={detail} />
                </>
              )}

              <AlternatesBlock event={event} />

              <Link
                href={`/stocks/${encodeURIComponent(event.symbol)}`}
                className="flex items-center justify-center gap-1 rounded-lg border py-2 text-xs font-medium hover:bg-muted"
              >
                종목 상세로 이동
                <ArrowRight className="size-3.5" />
              </Link>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

/** 서프라이즈와 주가 반응을 나란히 — 이 모달의 요지. */
function HeadlineRow({ event }: { event: EarningsEvent }) {
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg bg-muted/50 p-3">
      <div className="min-w-0">
        <p className="text-[10px] text-muted-foreground">
          {event.surprise ? `${SURPRISE_METRIC_LABEL[event.surprise.metric]} 서프라이즈` : "발표 상태"}
        </p>
        {event.surprise ? (
          <SurpriseValue surprise={event.surprise} className="text-lg" />
        ) : (
          <div className="mt-0.5 flex items-center gap-1">
            <DateStatusBadge status={event.dateStatus} />
            <SessionBadge session={event.session} />
          </div>
        )}
      </div>
      <div className="min-w-0">
        <p className="text-[10px] text-muted-foreground">
          {event.reaction?.basis === "sameDay" ? "발표 당일 주가" : "다음 거래일 주가"}
        </p>
        <ReactionValue
          reaction={event.reaction}
          status={event.dateStatus}
          className="text-lg"
        />
      </div>
      <DivergenceFlag event={event} />
      {event.estimateBasis && (
        <p className="text-[10px] text-[var(--sample)]">{event.estimateBasis}</p>
      )}
    </div>
  );
}

/** 매출 · 영업이익 · 순이익 — 발표치와 (있으면) QoQ. */
function MetricTiles({ event }: { event: EarningsEvent }) {
  const tiles = [
    { label: "매출", pair: event.revenue },
    { label: "영업이익", pair: event.opIncome },
    { label: "순이익", pair: event.netIncome },
  ].filter((t) => t.pair.actual != null || t.pair.estimate != null);

  if (tiles.length === 0) {
    // 미국 종목은 매출/EPS만 오고 영업이익·순이익이 없다.
    if (event.eps.actual == null && event.eps.estimate == null) return null;
    return (
      <div className="grid grid-cols-2 gap-2">
        <Tile label="EPS 기대" value={epsText(event.eps.estimate, event.currency)} />
        <Tile label="EPS 발표" value={epsText(event.eps.actual, event.currency)} />
      </div>
    );
  }

  return (
    <div className="grid grid-cols-3 gap-2">
      {tiles.map((t) => (
        <Tile
          key={t.label}
          label={t.label}
          value={
            t.pair.actual != null
              ? formatFinancialAmount(t.pair.actual, event.currency === "USD" ? "USD" : "KRW")
              : "—"
          }
          sub={
            t.pair.actual == null && t.pair.estimate != null
              ? `기대 ${formatFinancialAmount(t.pair.estimate, event.currency === "USD" ? "USD" : "KRW")}`
              : undefined
          }
          qoq={t.pair.qoq}
        />
      ))}
    </div>
  );
}

function Tile({
  label,
  value,
  sub,
  qoq,
}: {
  label: string;
  value: string;
  sub?: string;
  qoq?: number;
}) {
  return (
    <div className="min-w-0 rounded-lg border p-2">
      <p className="text-[10px] text-muted-foreground">{label}</p>
      <p className="truncate font-mono text-xs font-semibold tabular-nums">{value}</p>
      {qoq != null && (
        <p
          className={cn(
            "font-mono text-[10px] tabular-nums",
            qoq >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
          )}
          // YoY로 착각하면 성장률을 완전히 잘못 읽는다. 실측으로 QoQ임을 확인했다.
          title="전분기 대비(QoQ) 증감률"
        >
          QoQ {qoq >= 0 ? "+" : ""}
          {qoq.toFixed(1)}%
        </p>
      )}
      {sub && <p className="truncate text-[10px] text-muted-foreground">{sub}</p>}
    </div>
  );
}

/** 적중 이력 · 평균/중위 서프라이즈 · 다음 발표. */
function HistoryStrip({ detail }: { detail: EarningsTabData | null }) {
  const next = useMemo(
    () => (detail?.calendar ?? []).find((c) => c.kind === "upcoming"),
    [detail],
  );
  const hit = detail?.hitHistory;
  if (!hit && !next) return null;

  return (
    <div className="flex flex-wrap items-start gap-x-6 gap-y-2 rounded-lg bg-muted/50 p-3">
      {hit && (
        <>
          <div>
            <p className="text-[10px] text-muted-foreground">컨센서스 적중 이력</p>
            <p className="text-xs font-bold">
              <span className="text-[var(--profit)]">{hit.beats}</span>
              <span className="font-normal text-muted-foreground"> / {hit.quarters}분기 상회</span>
            </p>
          </div>
          {hit.medianPct != null && (
            <div>
              <p className="text-[10px] text-muted-foreground">서프라이즈 중위</p>
              <p
                className={cn(
                  "font-mono text-xs font-bold tabular-nums",
                  hit.medianPct >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                )}
              >
                {hit.medianPct >= 0 ? "+" : ""}
                {hit.medianPct.toFixed(1)}%
              </p>
              {hit.avgPct != null && (
                <p className="text-[10px] text-muted-foreground">
                  평균 {hit.avgPct >= 0 ? "+" : ""}
                  {hit.avgPct.toFixed(1)}%
                </p>
              )}
            </div>
          )}
        </>
      )}
      {next && (
        <div>
          <p className="text-[10px] text-muted-foreground">다음 발표</p>
          <p className="flex items-center gap-1 text-xs font-bold">
            {next.dateStatus && <DateStatusBadge status={next.dateStatus} />}
            <span className="font-mono tabular-nums">{formatShortDate(next.date)}</span>
          </p>
          {next.estimateBasis && (
            <p className="text-[10px] text-[var(--sample)]">{next.estimateBasis}</p>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * 접어 둔 개별(또는 다른 기준) 발표.
 *
 * 서버가 연결을 대표로 세우고 나머지를 `alternates`로 넘긴다. 지우지 않는 이유는
 * 지주회사처럼 개별이 의미 있는 경우가 있어서고, 기본으로 펼치지 않는 이유는
 * 한 회사가 상회와 하회를 동시에 말하는 것처럼 보이기 때문이다.
 */
function AlternatesBlock({ event }: { event: EarningsEvent }) {
  const [open, setOpen] = useState(false);
  const alternates = event.alternates ?? [];
  if (alternates.length === 0) return null;

  return (
    <div className="rounded-lg border">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-1.5 px-3 py-2 text-left text-[11px] hover:bg-muted/50"
      >
        <ChevronDown className={cn("size-3.5 transition-transform", open && "rotate-180")} />
        <span className="text-muted-foreground">
          {alternates.map((a) => a.basis || "다른 기준").join(" · ")} 기준 발표도 있습니다
        </span>
      </button>
      {open && (
        <div className="border-t px-3 py-2">
          {alternates.map((alt) => (
            <div
              key={`${alt.basis}-${alt.fiscalPeriod}`}
              className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 py-1 text-[11px]"
            >
              <span className="rounded bg-muted px-1 py-0.5 text-[10px]">{alt.basis || "기타"}</span>
              {alt.opIncome.estimate != null && alt.opIncome.actual != null && (
                <span className="font-mono text-muted-foreground">
                  기대 {formatFinancialAmount(alt.opIncome.estimate, "KRW")} →{" "}
                  {formatFinancialAmount(alt.opIncome.actual, "KRW")}
                </span>
              )}
              <SurpriseValue surprise={alt.surprise} className="text-[11px]" />
            </div>
          ))}
          <p className="mt-1 border-t pt-1 text-[10px] leading-relaxed text-muted-foreground">
            국내 실적 보도의 기준은 <b>연결</b>이라 그쪽을 대표로 보여줍니다. 지주회사처럼
            개별이 의미 있는 경우가 있어 지우지 않고 접어 둡니다.
          </p>
        </div>
      )}
    </div>
  );
}

function epsText(value: number | undefined, currency: string): string {
  if (value == null) return "—";
  return `${currency === "USD" ? "$" : ""}${value.toFixed(2)}`;
}
