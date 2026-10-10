"use client";

import { useEffect, useMemo, useState } from "react";
import { ChevronDown, ChevronRight, ExternalLink, Loader2, Route } from "lucide-react";

import { MiniToggle } from "@/components/ui/mini-toggle";
import { useStockTimeline, useTimelineDayNews } from "@/hooks/use-stock-timeline";
import { fetchStockDailyCloses } from "@/lib/daily-close-api";
import type { DailyCloseRow } from "@/lib/daily-close-types";
import { formatPrice } from "@/lib/format";
import { SCORE_TONE_TEXT, scoreTone } from "@/lib/stock-score-api";
import {
  TIMELINE_RANGES,
  formatRate,
  formatShares,
  formatSignedShares,
  reasonTone,
  type StockTimeline,
  type TimelineDay,
  type TimelineLane,
} from "@/lib/stock-timeline-api";
import { cn } from "@/lib/utils";

/**
 * 「주가가 왜 움직였나」 — 종목 타임라인 (2026-09-18 사용자 결정 B).
 *
 * # 왜 AI 분석 탭인가
 *
 * 그 탭은 이미 **「왜」를 다루는 자리**다 — 점수 근거 패널이 거기 있다. 「이 점수가 왜
 * 이런가」 다음에 「주가는 왜 움직였나」가 온다. 새 탭을 만들지 않은 이유는 탭이 이미
 * 아홉이고, 그 수가 폰 폭의 탭바를 넘친다는 사실이 코드에 이미 적혀 있기 때문이다
 * (`stock-workspace-tab-bar.tsx`).
 *
 * # 이 패널이 지키는 것 셋
 *
 *   · **「수집 전」과 「아무 일 없었다」를 가른다.** 레인 칩이 주황이면 그 창의 일부만
 *     덮는다는 뜻이고, 그 앞 구간에 사건이 없는 것이 아니라 기록이 없는 것이다.
 *   · **문턱을 밝힌다.** 「큰 등락」은 종목마다 다르다 — ±N%가 어디서 왔는지 적는다.
 *   · **설명이 없으면 없다고 한다.** 뉴스 1건뿐인 −10% 날을 그대로 보여 준다.
 *
 * # 「일별 시세 · 종합 점수」를 흡수했다 (2026-10-07 사용자 요청 7)
 *
 * 둘은 같은 날짜 축 위의 두 목록이었다 — 이쪽은 「사건이 있는 날」, 저쪽은 「모든 날의
 * 종가·점수·그날의 AI 요약」. 이제 한 목록이다: 줄마다 종가와 점수가 붙고, 펼치면 그날의
 * AI 요약이 사건 근거(실적·내부자·공시·뉴스) 위에 선다. 기본은 「사건 있는 날」이고
 * 「모든 날」로 바꾸면 예전 일별 목록이 된다. 점수·요약은 `/daily-closes`(최대 120일)에서 온다.
 */

/** 「모든 날」에서 한 번에 펴는 줄 수 — 예전 일별 목록의 한 쪽과 같다. */
const ALL_DAYS_PAGE = 30;

/** 점수·그날의 AI 요약 — 날짜로 찾는다. 실패해도 타임라인은 그대로 선다(값만 빈다). */
function useDailyCloseIndex(symbol: string) {
  const [state, setState] = useState<{ symbol: string; byDate: Map<string, DailyCloseRow> } | null>(null);
  useEffect(() => {
    let alive = true;
    fetchStockDailyCloses(symbol, 120)
      .then((s) => alive && setState({ symbol, byDate: new Map((s.rows ?? []).map((r) => [r.tradeDate, r])) }))
      .catch(() => alive && setState({ symbol, byDate: new Map() }));
    return () => {
      alive = false;
    };
  }, [symbol]);
  return state?.symbol === symbol ? state.byDate : null;
}

export function StockTimelinePanel({
  symbol,
  compact = false,
  className,
}: {
  symbol: string;
  compact?: boolean;
  className?: string;
}) {
  const [days, setDays] = useState(90);
  const { data, loading, error } = useStockTimeline(symbol, days);
  const closesByDate = useDailyCloseIndex(symbol);

  return (
    <div
      className={cn(
        "overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--card)]",
        className,
      )}
    >
      <div
        className={cn(
          "flex flex-wrap items-baseline gap-x-2 gap-y-1 pt-2.5 pb-1.5",
          compact ? "px-2.5" : "px-3",
        )}
      >
        <Route className="h-3.5 w-3.5 shrink-0 self-center text-[var(--brand)]" aria-hidden />
        <span className="text-cell font-semibold text-[var(--text-primary)]">
          주가가 왜 움직였나 · 일별 시세
        </span>
        {data && (
          <span className="text-micro text-[var(--text-muted)]">
            {data.fromDate} ~ {data.toDate} · 표시 {data.days.length}일
          </span>
        )}
        <div className="ml-auto flex shrink-0 gap-0.5 self-center rounded-lg bg-muted p-0.5">
          {TIMELINE_RANGES.map((r) => (
            <button
              key={r.id}
              type="button"
              onClick={() => setDays(r.id)}
              className={cn(
                "rounded-md px-2 py-0.5 text-cell font-medium transition-colors",
                days === r.id
                  ? "bg-card text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>

      <div className={cn("pb-3", compact ? "px-2.5" : "px-3")}>
        {loading ? (
          <p className="flex items-center gap-1.5 text-cell text-[var(--text-muted)]">
            <Loader2 className="h-3 w-3 animate-spin" />
            타임라인을 불러오는 중입니다…
          </p>
        ) : error ? (
          /* 「불러오지 못했다」와 「아직 없다」는 다른 상태다 (요구 8). */
          <p className="text-cell text-[var(--text-muted)]">{error}</p>
        ) : !data || data.closes.length === 0 ? (
          <p className="text-cell text-[var(--text-muted)]">
            이 종목은 아직 일별 기록이 없습니다.
          </p>
        ) : (
          <TimelineBody symbol={symbol} data={data} closesByDate={closesByDate} />
        )}
      </div>
    </div>
  );
}

type ListMode = "events" | "all";

const LIST_MODES: { id: ListMode; label: string }[] = [
  { id: "events", label: "사건 있는 날" },
  { id: "all", label: "모든 날" },
];

function TimelineBody({
  symbol,
  data,
  closesByDate,
}: {
  symbol: string;
  data: StockTimeline;
  closesByDate: Map<string, DailyCloseRow> | null;
}) {
  const [openDate, setOpenDate] = useState<string | null>(null);
  const [mode, setMode] = useState<ListMode>("events");
  const [shown, setShown] = useState(ALL_DAYS_PAGE);
  const partial = data.lanes.filter((l) => l.availableFrom && !l.coversWindow);
  const currency = data.currency === "USD" ? "USD" : "KRW";

  /* 「모든 날」은 종가 선의 날짜 전부(최신순)이고, 사건이 있는 날은 그 날의 기록을 그대로 쓴다. */
  const allDays = useMemo(() => {
    const byDate = new Map(data.days.map((d) => [d.date, d]));
    return [...data.closes]
      .reverse()
      .map((p): TimelineDay => byDate.get(p.date) ?? { date: p.date, close: p.close, changeRate: p.changeRate, significant: false });
  }, [data]);
  const list = mode === "events" ? data.days : allDays.slice(0, shown);
  const latest = allDays[0];
  const latestUnscored = closesByDate != null && latest != null && closesByDate.get(latest.date)?.score == null;

  return (
    <div className="space-y-2">
      <TimelineChart data={data} />
      <ChartLegend />

      <ul className="flex flex-wrap gap-1">
        {data.lanes.map((lane) => (
          <LaneChip key={lane.id} lane={lane} />
        ))}
      </ul>

      <div className="flex flex-wrap items-center gap-2">
        <p className="min-w-0 flex-1 text-micro text-[var(--text-muted)]">
          ±{data.moveThreshold}% 이상을 큰 등락으로 봤습니다 (이 기간 변동성 {data.volatility}%).
        </p>
        <MiniToggle options={LIST_MODES} value={mode} onChange={setMode} />
      </div>

      {list.length === 0 ? (
        <p className="py-4 text-center text-cell text-[var(--text-muted)]">
          이 기간에는 크게 움직이거나 사건이 있었던 날이 없습니다. 「모든 날」에서 일별 시세를 볼 수 있습니다.
        </p>
      ) : (
        <ul>
          <li className="flex items-center gap-2 border-b border-[var(--border)] pb-1 text-micro text-[var(--text-muted)]">
            <span className="w-11 shrink-0">날짜</span>
            <span className="w-14 shrink-0 text-right">등락</span>
            <span className="min-w-0 flex-1">왜</span>
            <span className="shrink-0 text-right">종가</span>
            <span className="w-8 shrink-0 text-right">점수</span>
            <span className="w-3 shrink-0" />
          </li>
          {list.map((day) => (
            <DayRow
              key={day.date}
              symbol={symbol}
              day={day}
              close={closesByDate?.get(day.date)}
              currency={currency}
              open={openDate === day.date}
              onToggle={() => setOpenDate((cur) => (cur === day.date ? null : day.date))}
            />
          ))}
        </ul>
      )}
      {mode === "all" && shown < allDays.length && (
        <button
          type="button"
          onClick={() => setShown((n) => n + ALL_DAYS_PAGE)}
          className="w-full rounded-md border border-[var(--border)] py-1 text-cell text-[var(--text-muted)] hover:bg-[var(--surface-elevated)]"
        >
          {Math.min(ALL_DAYS_PAGE, allDays.length - shown)}일 더 보기
        </button>
      )}

      {/* 「마감했는데 왜 점수가 비어 있나」 — 가장 최근 줄이 비었을 때만 답한다(2026-10-09 요청 9: 늘 깔려 있던 문장). */}
      {latestUnscored && (
        <p className="text-micro text-[var(--brand)]">오늘 점수는 새벽 00:25(KST)에 채워집니다.</p>
      )}

      {/*
        경고는 목록 **아래**에 선다. 위에 두면 읽는 사람이 경고를 지나 목록으로 가는데,
        아래에 두면 빈 구간을 본 직후에 만난다.
      */}
      {partial.length > 0 && (
        <div className="flex gap-2 rounded-r-lg border-l-[3px] border-[var(--score-mid)] bg-[var(--score-mid)]/6 px-3 py-2">
          <p className="text-micro leading-relaxed text-[var(--text-secondary)]">
            <b className="text-[var(--score-mid)]">
              {partial.map((l) => l.label).join(" · ")}
            </b>
            은 이 기간의 일부만 기록돼 있습니다. 그 앞 구간은 사건이 없었던 것이 아니라{" "}
            <b>아직 수집하기 전</b>입니다.
          </p>
        </div>
      )}
    </div>
  );
}

/** 레인 칩 — 주황은 「이 창의 일부만」, 점선은 「자료 없음」. */
function LaneChip({ lane }: { lane: TimelineLane }) {
  if (!lane.availableFrom) {
    return (
      <li className="rounded-full border border-dashed border-[var(--border)] px-2 py-0.5 text-micro text-[var(--text-muted)]">
        {lane.label} · 자료 없음
      </li>
    );
  }
  return (
    <li
      className={cn(
        "rounded-full border px-2 py-0.5 text-micro",
        lane.coversWindow
          ? "border-[var(--border)] text-[var(--text-muted)]"
          : "border-[var(--score-mid)] text-[var(--score-mid)]",
      )}
      title={lane.coversWindow ? undefined : `${lane.availableFrom}부터 기록돼 있습니다`}
    >
      {lane.label}{" "}
      <b className="font-semibold text-[var(--text-primary)]">
        {lane.count.toLocaleString("ko-KR")}
      </b>
      {!lane.coversWindow && ` · ${lane.availableFrom}부터`}
    </li>
  );
}

function DayRow({
  symbol,
  day,
  close,
  currency,
  open,
  onToggle,
}: {
  symbol: string;
  day: TimelineDay;
  /** 그날의 종가·점수·AI 요약(`/daily-closes`). 120일 밖이거나 못 받았으면 없다. */
  close?: DailyCloseRow;
  currency: "KRW" | "USD";
  open: boolean;
  onToggle: () => void;
}) {
  // 점수가 없으면 「-」다 — 0으로 적으면 아주 나쁜 점수로 읽힌다.
  const score = close?.score ?? day.score?.value ?? null;
  const price = close?.close ?? day.close;
  const rate = day.changeRate;
  const tone =
    rate == null || Math.abs(rate) < 0.005
      ? "text-[var(--text-muted)]"
      : rate > 0
        ? "text-[var(--profit)]"
        : "text-[var(--loss)]";

  return (
    <li className="border-b border-[var(--border)] last:border-b-0">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-start gap-2 py-1.5 text-left transition-colors hover:bg-[var(--surface-elevated)]"
      >
        <span className="w-11 shrink-0 pt-0.5 text-micro tabular-nums text-[var(--text-muted)]">
          {day.date.slice(5)}
        </span>
        <span className={cn("w-14 shrink-0 pt-0.5 text-right text-micro font-semibold tabular-nums", tone)}>
          {formatRate(rate)}
        </span>
        <span className="min-w-0 flex-1">
          <span className="flex flex-wrap gap-1">
            {(day.reasons ?? []).map((r) => (
              <ReasonChip key={r} reason={r} />
            ))}
          </span>
          <WhyLine day={day} />
        </span>
        <span className="shrink-0 pt-0.5 text-right text-micro tabular-nums text-[var(--text-secondary)]">
          {price ? formatPrice(price, currency) : "—"}
        </span>
        <span
          className={cn(
            "w-8 shrink-0 pt-0.5 text-right text-cell font-semibold tabular-nums",
            score == null ? "text-[var(--text-muted)]" : SCORE_TONE_TEXT[scoreTone(score)],
          )}
        >
          {score ?? "-"}
        </span>
        {open ? (
          <ChevronDown className="mt-0.5 h-3 w-3 shrink-0 text-[var(--text-muted)]" aria-hidden />
        ) : (
          <ChevronRight className="mt-0.5 h-3 w-3 shrink-0 text-[var(--text-muted)]" aria-hidden />
        )}
      </button>
      {open && <DayDetail symbol={symbol} day={day} aiSummary={close?.aiSummary} />}
    </li>
  );
}

function ReasonChip({ reason }: { reason: string }) {
  const tone = reasonTone(reason);
  return (
    <span
      className={cn(
        "rounded px-1.5 py-0.5 text-micro",
        tone === "up" && "bg-[var(--profit)]/12 font-semibold text-[var(--profit)]",
        tone === "down" && "bg-[var(--loss)]/12 font-semibold text-[var(--loss)]",
        tone === "earnings" && "bg-[var(--score-high)]/12 font-semibold text-[var(--score-high)]",
        tone === "score" && "bg-[var(--brand)]/10 text-[var(--brand)]",
        tone === "plain" && "bg-[var(--surface-elevated)] text-[var(--text-muted)]",
      )}
    >
      {reason}
    </span>
  );
}

/** 「왜」 한 줄 — 점수 귀인 + 그날의 숫자들. 넘치면 잘린다(펼치면 다 보인다). */
function WhyLine({ day }: { day: TimelineDay }) {
  const parts: string[] = [];
  if (day.score?.driver) parts.push(day.score.driver);
  if (day.news?.count) parts.push(`뉴스 ${day.news.count.toLocaleString("ko-KR")}건`);
  if (day.insider && day.insider.netShares !== 0) {
    parts.push(`내부자 순${formatSignedShares(day.insider.netShares)}`);
  }
  if (day.disclosures?.length) parts.push(day.disclosures[0].title);
  if (parts.length === 0) return null;
  return (
    <span className="mt-0.5 block truncate text-micro text-[var(--text-muted)]">
      {parts.join(" · ")}
    </span>
  );
}

function DayDetail({ symbol, day, aiSummary }: { symbol: string; day: TimelineDay; aiSummary?: string }) {
  const { items, loading, error } = useTimelineDayNews(symbol, day.date);

  return (
    <div className="mb-2 ml-11 space-y-2 border-l border-[var(--border)] pl-3">
      {aiSummary?.trim() && (
        <p className="rounded-lg bg-[var(--surface-elevated)]/60 px-2.5 py-1.5 text-cell leading-relaxed text-[var(--text-secondary)]">
          {aiSummary}
        </p>
      )}
      {day.earnings && <EarningsDetail day={day} />}

      {day.insider && (
        <p className="text-micro text-[var(--text-secondary)]">
          <b>내부자 거래 {day.insider.count}건</b> — 매수 {formatShares(day.insider.buyShares)} ·
          매도 {formatShares(day.insider.sellShares)} · 순{formatSignedShares(day.insider.netShares)}
        </p>
      )}

      {day.disclosures?.length ? (
        <ul className="space-y-0.5">
          {day.disclosures.map((d) => (
            <li key={d.receiptNo || d.title} className="text-micro text-[var(--text-secondary)]">
              공시 · {d.title}
              {d.filer && <span className="text-[var(--text-muted)]"> ({d.filer})</span>}
            </li>
          ))}
        </ul>
      ) : null}

      <div>
        <p className="text-micro font-semibold text-[var(--text-primary)]">
          그날의 뉴스
          {day.news?.count ? (
            <span className="ml-1 font-normal text-[var(--text-muted)]">
              전체 {day.news.count.toLocaleString("ko-KR")}건 중
            </span>
          ) : null}
        </p>
        {loading ? (
          <p className="mt-0.5 flex items-center gap-1.5 text-micro text-[var(--text-muted)]">
            <Loader2 className="h-3 w-3 animate-spin" />
            불러오는 중입니다…
          </p>
        ) : error ? (
          <p className="mt-0.5 text-micro text-[var(--text-muted)]">{error}</p>
        ) : items.length === 0 ? (
          /*
            **여기가 이 기능의 정직한 자리다.** −10%인데 기사가 없는 날이 실제로 있다
            (005930 2026-07-13, 뉴스 1건). 그걸 빈칸으로 두면 화면이 설명한 척을 한다.
          */
          <p className="mt-0.5 text-micro text-[var(--text-muted)]">
            이 날 수집된 기사가 없습니다 — 우리 데이터로는 설명되지 않는 움직임입니다.
          </p>
        ) : (
          <ul className="mt-0.5 space-y-0.5">
            {items.map((n) => (
              <li key={n.id}>
                <a
                  href={n.url}
                  target="_blank"
                  rel="noreferrer noopener"
                  className="flex items-start gap-1 text-micro text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
                >
                  <span className="shrink-0 tabular-nums text-[var(--text-muted)]">
                    {n.publishedAt.slice(11)}
                  </span>
                  <span className="min-w-0 flex-1">{n.headline}</span>
                  <ExternalLink className="mt-0.5 h-2.5 w-2.5 shrink-0 text-[var(--text-muted)]" aria-hidden />
                </a>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

function EarningsDetail({ day }: { day: TimelineDay }) {
  const e = day.earnings;
  if (!e) return null;
  return (
    <p className="text-micro text-[var(--text-secondary)]">
      <b>
        실적 {e.period}
        {e.session && ` · ${e.session}`}
      </b>
      {e.status === "actual" ? " · 발표치" : " · 예정"}
      {/* 추정치가 없으면 서프라이즈도 없다 — 0%로 채우면 「예상대로였다」가 된다. */}
      {typeof e.surprise === "number" && (
        <span
          className={cn(
            "ml-1 font-semibold",
            e.surprise > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
          )}
        >
          EPS 서프라이즈 {formatRate(e.surprise)}
        </span>
      )}
    </p>
  );
}

function ChartLegend() {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-micro text-[var(--text-muted)]">
      <LegendDot className="bg-[var(--profit)]" label="급등" />
      <LegendDot className="bg-[var(--loss)]" label="급락" />
      <LegendDot className="bg-[var(--score-high)]" label="실적 발표" />
      <LegendDot className="bg-[var(--text-muted)] opacity-50" label="공시 · 내부자 · 점수 변화" />
    </div>
  );
}

function LegendDot({ className, label }: { className: string; label: string }) {
  return (
    <span className="flex items-center gap-1">
      <i className={cn("inline-block h-1.5 w-1.5 rounded-full", className)} aria-hidden />
      {label}
    </span>
  );
}

/**
 * 주가 선 + 사건 점.
 *
 * 라이브러리를 들이지 않는다 — 점 수십 개짜리 꺾은선에 차트 엔진을 붙이면 번들만 커진다
 * (백테스트 곡선과 같은 판단).
 */
function TimelineChart({ data }: { data: StockTimeline }) {
  const W = 900;
  const H = 150;

  const geom = useMemo(() => {
    const values = data.closes.map((p) => p.close);
    if (values.length < 2) return null;
    const lo = Math.min(...values) * 0.98;
    const hi = Math.max(...values) * 1.02;
    const span = hi - lo || 1;
    const x = (i: number) => (i / (values.length - 1)) * (W - 8) + 4;
    const y = (v: number) => H - 6 - ((v - lo) / span) * (H - 12);

    const index = new Map(data.closes.map((p, i) => [p.date, i]));
    const marks = data.days
      .map((day) => {
        const i = index.get(day.date);
        if (i == null) return null;
        const reasons = day.reasons ?? [];
        const big = reasons.includes("급등") || reasons.includes("급락");
        const earnings = reasons.includes("실적 발표");
        return {
          date: day.date,
          cx: x(i),
          cy: y(values[i]),
          r: earnings ? 4 : big ? 3.4 : 2.2,
          fill: earnings
            ? "var(--score-high)"
            : big
              ? (day.changeRate ?? 0) > 0
                ? "var(--profit)"
                : "var(--loss)"
              : "var(--text-muted)",
          opacity: big || earnings ? 1 : 0.45,
          title: `${day.date} ${reasons.join(" · ")}`,
        };
      })
      .filter((m): m is NonNullable<typeof m> => m !== null);

    return {
      points: values.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" "),
      marks,
    };
  }, [data]);

  if (!geom) return null;

  return (
    <svg
      viewBox={`0 0 ${W} ${H}`}
      width="100%"
      height={H}
      preserveAspectRatio="none"
      className="block"
      role="img"
      aria-label="주가와 사건"
    >
      <polyline
        points={geom.points}
        fill="none"
        stroke="var(--text-muted)"
        strokeWidth={1.6}
        vectorEffect="non-scaling-stroke"
      />
      {geom.marks.map((m) => (
        <circle key={m.date} cx={m.cx} cy={m.cy} r={m.r} fill={m.fill} opacity={m.opacity}>
          <title>{m.title}</title>
        </circle>
      ))}
    </svg>
  );
}
