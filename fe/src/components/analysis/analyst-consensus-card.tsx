"use client";

import { useEffect, useState } from "react";
import { ExternalLink, Users } from "lucide-react";
import { Card } from "@/components/ui/card";
import {
  analystReportUrl,
  analystTrend,
  fetchAnalyst,
  type AnalystData,
  type AnalystHistoryPoint,
  type AnalystReport,
} from "@/lib/analyst-api";
import { formatPrice } from "@/lib/format";
import { ratingLabel, type AnalystConsensus } from "@/lib/stock-signals-api";
import { cn } from "@/lib/utils";

const COUNT_LABELS = ["강력매수", "매수", "보유", "매도", "강력매도"] as const;
const COUNT_TONE = [
  "bg-[var(--profit)]",
  "bg-[var(--profit)]/60",
  "bg-[var(--text-muted)]/40",
  "bg-[var(--loss)]/60",
  "bg-[var(--loss)]",
] as const;
/** 의견 척도(1~5)의 눈금 — 왼쪽이 매도, 오른쪽이 매수. */
const SCALE_TICKS = ["강력매도", "매도", "보유", "매수", "강력매수"] as const;

/**
 * 애널리스트 의견 — **개요 탭**, 종합 점수 아래 (2026-10-07 사용자 요청 9 — 그전엔 밸류에이션 탭).
 *
 * 해외: Finnhub 추천 추이(그달의 명수). 목표가는 무료 출처가 없어 싣지 않는다.
 * 국내: 네이버 증권 컨센서스(평균 목표가 · 의견 1~5). 목표가 여력은 **지금 가격**으로 다시 잰다.
 * 투자 권유가 아니라 증권사 의견의 요약이다 — 출처와 기준일을 늘 밝힌다.
 *
 * 10-07에 더 자세히 한 것(전부 이미 받는 값으로 — 새 수집 없음):
 *   · 한 문장 결론 — 해외 「N명 중 M명(xx%)이 매수 이상, 매도 K명」 · 국내 「5단계 중 n단계 · 목표가는 지금의 x배」
 *   · 목표가 · 현재가 · 목표가까지 세 칸, 의견 다섯 단계 중 하나를 칠한 줄(10-13, 그전엔 1~5 척도 막대)
 *   · 분포 — 명수
 *
 * 같은 날 이어서(사용자 요청 「추이를 보여 주는 표 + 전문적으로 활용」 — 099 이력 표):
 *   · 의견 변화 — 약 90일 전보다 목표가·매수 비중·인원이 어떻게 움직였나(`analystTrend`, 10-13부터 칩 대신 결론 문장 안에)
 *   · 추이 차트 — 해외는 월별 의견 분포 막대, 국내는 목표가 계단선
 *   · 최근 증권사 리포트 — 증권사 · 제목 · 날짜, 원문은 네이버로
 *
 * 자료가 없으면(배치가 아직 안 채웠거나 커버리지 밖) **카드를 그리지 않는다**.
 */
export function AnalystConsensusSection({
  symbol,
  price,
  currency,
}: {
  symbol: string;
  price?: number;
  currency: "KRW" | "USD";
}) {
  const [state, setState] = useState<{ symbol: string; data: AnalystData | null } | null>(null);
  useEffect(() => {
    let alive = true;
    fetchAnalyst(symbol)
      .then((d) => alive && setState({ symbol, data: d }))
      .catch(() => alive && setState({ symbol, data: null }));
    return () => {
      alive = false;
    };
  }, [symbol]);
  const data = state?.symbol === symbol ? state.data : null;
  if (!data?.consensus && !data?.reports.length) return null;
  return <AnalystConsensusCard data={data} price={price} currency={currency} />;
}

function AnalystConsensusCard({
  data,
  price,
  currency,
}: {
  data: AnalystData;
  price?: number;
  currency: "KRW" | "USD";
}) {
  const analyst: AnalystConsensus = data.consensus ?? { symbol: "", source: "naver", fetchedAt: "" };
  const trend = analystTrend(data.history);
  const ref = trend ? data.history.find((h) => h.asOf === trend.sinceDate) : undefined;
  const latest = data.history[data.history.length - 1];
  const counts = analyst.counts;
  const total = counts?.reduce((a, b) => a + b, 0) ?? 0;
  const buyers = counts ? counts[0] + counts[1] : 0;
  const sellers = counts ? counts[3] + counts[4] : 0;
  const upside =
    analyst.targetMean && price && price > 0 ? (analyst.targetMean / price - 1) * 100 : undefined;
  const sourceLabel = analyst.source === "naver" ? "네이버 증권" : "Finnhub";
  const targetCurrency = (analyst.targetCurrency as "KRW" | "USD") || currency;
  const step = analyst.rating != null ? Math.min(5, Math.max(1, Math.round(analyst.rating))) : null;
  const [now] = useState(() => Date.now());
  const recentReports = data.reports.filter(
    (r) => r.published && now - Date.parse(r.published) <= 30 * 86_400_000,
  ).length;

  /*
    10-13 요청 3 — 「평균 4.00 / 5」 척도 막대·변화 칩 대신, 몇 명이 · 얼마를 · 최근 어떻게 바뀌었나를
    문장과 숫자로 (시안 `docs/mockups/stock-overview-v2.html` ⑤).
  */
  const ratio = upside != null && analyst.targetMean && price ? analyst.targetMean / price : undefined;
  const targetPhrase =
    upside == null
      ? null
      : ratio != null && ratio >= 2
        ? `평균 목표가는 지금 주가의 ${ratio.toFixed(1)}배입니다`
        : `평균 목표가는 지금 주가보다 ${Math.abs(upside).toFixed(1)}% ${upside >= 0 ? "높습니다" : "낮습니다"}`;

  // 비교 시점의 모습 — 해외는 「N개월 전(M월)에는 a명 중 b명(c%)」, 국내는 「목표가는 날짜 값 → 날짜 값」.
  let change: string | null = null;
  if (trend && ref && latest) {
    if (ref.counts && latest.counts) {
      const refTotal = ref.counts.reduce((a, b) => a + b, 0);
      const refBuy = ref.counts[0] + ref.counts[1];
      if (refTotal > 0) {
        change = `${monthsAgo(ref.asOf, latest.asOf)}(${Number(ref.asOf.slice(5, 7))}월)에는 ${refTotal}명 중 ${refBuy}명(${Math.round((refBuy / refTotal) * 100)}%)`;
      }
    } else if (ref.targetMean && latest.targetMean) {
      change = `목표가는 ${md(ref.asOf)} ${formatPrice(ref.targetMean, targetCurrency)} → ${md(latest.asOf)} ${formatPrice(latest.targetMean, targetCurrency)}`;
    }
  }

  return (
    <Card className="space-y-2.5 p-3">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
        <Users className="h-3.5 w-3.5 shrink-0 text-[var(--brand)]" aria-hidden />
        <h3 className="text-cell font-semibold text-[var(--text-primary)]">애널리스트 의견</h3>
        {analyst.rating != null && (
          <span
            className={cn(
              "rounded-md border border-current px-1.5 py-0.5 text-micro font-semibold leading-none",
              analyst.rating >= 3.5
                ? "text-[var(--profit)]"
                : analyst.rating < 2.5
                  ? "text-[var(--loss)]"
                  : "text-[var(--text-muted)]",
            )}
          >
            {ratingLabel(analyst.rating)}
          </span>
        )}
        <span className="ml-auto text-micro text-[var(--text-muted)]">
          {sourceLabel}
          {analyst.asOf
            ? ` · ${analyst.source === "naver" ? `${md(analyst.asOf)} 기준` : `${Number(analyst.asOf.slice(5, 7))}월 집계`}`
            : ""}
        </span>
      </div>

      {analyst.targetMean != null && (
        <div className="grid grid-cols-3 gap-1.5">
          <Kpi label="평균 목표가" value={formatPrice(analyst.targetMean, targetCurrency)} />
          {price != null && price > 0 && <Kpi label="현재가" value={formatPrice(price, currency)} />}
          {upside != null && (
            <Kpi
              label="목표가까지"
              value={`${upside >= 0 ? "+" : "−"}${Math.abs(upside).toFixed(0)}%`}
              tone={upside >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]"}
            />
          )}
        </div>
      )}

      {counts && total > 0 ? (
        <p className="text-cell leading-relaxed text-[var(--text-secondary)]">
          애널리스트{" "}
          <b className="text-[var(--text-primary)]">
            {total}명 중 {buyers}명({Math.round((buyers / total) * 100)}%)
          </b>
          이 매수 이상, <b className="text-[var(--text-primary)]">매도 {sellers}명</b>입니다.
          {change && trend?.buyShareDelta != null && (
            <>
              {" "}
              {change} — 매수 비중 <Signed v={trend.buyShareDelta} unit="%p" digits={0} />
              {trend.analystsDelta ? (
                <>
                  , 참여 <Signed v={trend.analystsDelta} unit="명" digits={0} />
                </>
              ) : null}
              .
            </>
          )}
        </p>
      ) : (
        analyst.rating != null && (
          <p className="text-cell leading-relaxed text-[var(--text-secondary)]">
            증권사들은 평균적으로{" "}
            <b className="text-[var(--text-primary)]">
              「{ratingLabel(analyst.rating)}」(5단계 중 {step}단계)
            </b>{" "}
            의견이며{targetPhrase ? `, ${targetPhrase}` : "입니다"}.
            {recentReports > 0 && (
              <>
                {" "}
                최근 30일 리포트 <b className="text-[var(--text-primary)]">{recentReports}건</b>
                {change && trend?.targetPct != null ? "," : "."}
              </>
            )}
            {change && trend?.targetPct != null && (
              <>
                {" "}
                {change}(<Signed v={trend.targetPct} unit="%" digits={1} />).
              </>
            )}
          </p>
        )
      )}

      {step != null && <RatingSteps step={step} />}

      {counts && total > 0 && (
        <div className="space-y-1">
          <div className="flex h-2 overflow-hidden rounded-full bg-[var(--surface-elevated)]" aria-hidden>
            {counts.map((n, i) =>
              n > 0 ? <span key={i} className={COUNT_TONE[i]} style={{ width: `${(n / total) * 100}%` }} /> : null,
            )}
          </div>
          <ul className="flex flex-wrap justify-between gap-x-2 text-micro tabular-nums text-[var(--text-muted)]">
            {counts.map((n, i) => (
              <li
                key={i}
                className={cn(i < 2 && n > 0 && "text-[var(--profit)]", i > 2 && n > 0 && "text-[var(--loss)]")}
              >
                {COUNT_LABELS[i]} <b className="font-semibold">{n}</b>
              </li>
            ))}
          </ul>
        </div>
      )}
      {analyst.source !== "naver" && analyst.targetMean == null && (
        <p className="text-micro text-[var(--text-muted)]">
          해외 종목은 무료 목표가 출처가 없어 목표가를 싣지 않습니다.
        </p>
      )}

      {data.history.length >= 2 && <HistoryChart history={data.history} currency={currency} />}
      {data.reports.length > 0 && <ReportList reports={data.reports} />}
      {/* 2026-10-10 사업화 검토 N5 — 「매수 · +88%」가 이 서비스의 권유처럼 읽히지 않게 누구의 말인지 적는다. */}
      <p className="text-micro text-[var(--text-muted)]">
        증권사 의견을 모은 것이며 Quant Port의 판단이나 투자 권유가 아닙니다.
      </p>
    </Card>
  );
}

function md(d: string): string {
  const [, m, day] = d.split("-");
  return m && day ? `${Number(m)}/${Number(day)}` : d;
}

function monthsAgo(from: string, to: string): string {
  const n = Math.max(1, Math.round((Date.parse(to) - Date.parse(from)) / (30.4 * 86_400_000)));
  return `${n}개월 전`;
}

function Signed({ v, unit, digits }: { v: number; unit: string; digits: number }) {
  const r = Number(v.toFixed(digits));
  return (
    <span
      className={cn("font-semibold tabular-nums", r > 0 ? "text-[var(--profit)]" : r < 0 ? "text-[var(--loss)]" : "")}
    >
      {r > 0 ? "+" : r < 0 ? "−" : ""}
      {Math.abs(r).toFixed(digits)}
      {unit}
    </span>
  );
}

function Kpi({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="min-w-0 rounded-md border border-[var(--border)] px-2 py-1">
      <span className="block truncate text-micro text-[var(--text-muted)]">{label}</span>
      <span className={cn("block truncate text-note font-bold tabular-nums", tone ?? "text-[var(--text-primary)]")}>
        {value}
      </span>
    </div>
  );
}

/** 평균 의견(1~5)을 가장 가까운 단계 하나로 칠한다 — 「평균 4.00 / 5」보다 바로 읽힌다(10-13 요청 3). */
function RatingSteps({ step }: { step: number }) {
  const onTone = step >= 4 ? "bg-[var(--profit)]" : step <= 2 ? "bg-[var(--loss)]" : "bg-[var(--text-muted)]";
  return (
    <div className="grid grid-cols-5 gap-0.5 text-center text-micro" aria-label={`5단계 중 ${step}단계`}>
      {SCALE_TICKS.map((t, i) => (
        <span
          key={t}
          className={cn(
            "rounded py-0.5",
            i + 1 === step
              ? cn(onTone, "font-semibold text-white")
              : "bg-[var(--surface-elevated)] text-[var(--text-muted)]",
          )}
        >
          {t}
        </span>
      ))}
    </div>
  );
}

/**
 * 추이 — 명수가 있으면(해외) 시점별 의견 분포 100% 막대, 없으면(국내) 평균 목표가 계단선.
 * 라이브러리를 들이지 않는다 — 점 수십 개짜리 그림이다(타임라인 차트와 같은 판단).
 */
function HistoryChart({ history, currency }: { history: AnalystHistoryPoint[]; currency: "KRW" | "USD" }) {
  const withCounts = history.filter((h) => h.counts && h.counts.some((n) => n > 0));
  if (withCounts.length >= 2) {
    const shown = withCounts.slice(-12);
    return (
      <div className="space-y-1">
        <p className="text-micro text-[var(--text-muted)]">월별 의견 분포</p>
        <div className="flex h-20 items-end gap-1">
          {shown.map((h) => {
            const c = h.counts!;
            const total = c.reduce((a, b) => a + b, 0);
            return (
              <div key={h.asOf} className="flex min-w-0 flex-1 flex-col items-center gap-0.5" title={`${h.asOf} · ${total}명`}>
                <div className="flex h-16 w-full flex-col-reverse overflow-hidden rounded-sm">
                  {c.map((n, i) =>
                    n > 0 ? <span key={i} className={COUNT_TONE[i]} style={{ height: `${(n / total) * 100}%` }} /> : null,
                  )}
                </div>
                <span className="text-[10px] tabular-nums text-[var(--text-muted)]">{h.asOf.slice(2, 7).replace("-", ".")}</span>
              </div>
            );
          })}
        </div>
      </div>
    );
  }
  const pts = history.filter((h) => h.targetMean && h.targetMean > 0);
  if (pts.length < 2) return null;
  const W = 300;
  const H = 64;
  const vals = pts.map((p) => p.targetMean!);
  const lo = Math.min(...vals);
  const hi = Math.max(...vals);
  const span = hi - lo || hi * 0.05 || 1;
  const t0 = Date.parse(pts[0].asOf);
  const t1 = Date.parse(pts[pts.length - 1].asOf);
  const x = (d: string) => ((Date.parse(d) - t0) / (t1 - t0 || 1)) * (W - 4) + 2;
  const y = (v: number) => H - 4 - ((v - lo) / span) * (H - 8);
  // 계단선 — 값은 다음 변경일까지 유지된다(바뀐 날만 저장한다).
  let d = `M${x(pts[0].asOf).toFixed(1)},${y(vals[0]).toFixed(1)}`;
  for (let i = 1; i < pts.length; i++) {
    d += ` H${x(pts[i].asOf).toFixed(1)} V${y(vals[i]).toFixed(1)}`;
  }
  return (
    <div className="space-y-1">
      <p className="flex justify-between text-micro text-[var(--text-muted)]">
        <span>평균 목표가 추이</span>
        <span className="tabular-nums">
          {formatPrice(lo, currency)} ~ {formatPrice(hi, currency)}
        </span>
      </p>
      <svg viewBox={`0 0 ${W} ${H}`} width="100%" height={H} preserveAspectRatio="none" role="img" aria-label="평균 목표가 추이">
        <path d={d} fill="none" stroke="var(--brand)" strokeWidth={1.6} vectorEffect="non-scaling-stroke" />
      </svg>
      <p className="flex justify-between text-micro tabular-nums text-[var(--text-muted)]">
        <span>{pts[0].asOf}</span>
        <span>{pts[pts.length - 1].asOf}</span>
      </p>
    </div>
  );
}

/** 최근 증권사 리포트 — 제목·증권사·날짜만 싣고 원문은 네이버로 보낸다. */
function ReportList({ reports }: { reports: AnalystReport[] }) {
  return (
    <div className="space-y-1">
      <p className="text-micro text-[var(--text-muted)]">최근 증권사 리포트</p>
      <ul className="divide-y divide-[var(--border)]">
        {reports.slice(0, 5).map((r) => {
          const url = analystReportUrl(r);
          const body = (
            <>
              <span className="min-w-0 flex-1 truncate text-cell text-[var(--text-primary)]">{r.title}</span>
              <span className="shrink-0 text-micro text-[var(--text-muted)]">{r.broker}</span>
              <span className="shrink-0 text-micro tabular-nums text-[var(--text-muted)]">{r.published?.slice(5)}</span>
              {url && <ExternalLink className="h-3 w-3 shrink-0 text-[var(--text-muted)]" aria-hidden />}
            </>
          );
          return (
            <li key={`${r.source}-${r.id}`}>
              {url ? (
                <a href={url} target="_blank" rel="noreferrer noopener" className="flex min-w-0 items-center gap-2 py-1 hover:bg-[var(--surface-elevated)]">
                  {body}
                </a>
              ) : (
                <span className="flex min-w-0 items-center gap-2 py-1">{body}</span>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
