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
  type AnalystTrend,
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
 *   · 한 문장 결론 — 「N명 중 M명(xx%)이 매수 이상」 또는 「평균 의견 매수 · 목표가까지 +x%」
 *   · 의견 척도 막대 — 평균 의견이 1~5 위 어디인지
 *   · 목표가 막대 — 현재가에서 목표가까지의 거리
 *   · 분포 — 명수와 비율을 함께
 *
 * 같은 날 이어서(사용자 요청 「추이를 보여 주는 표 + 전문적으로 활용」 — 099 이력 표):
 *   · 「의견 변화」 칩 — 약 90일 전보다 의견·목표가·매수 비중·인원이 어떻게 움직였나(`analystTrend`)
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
  // 컨센서스 없이 리포트만 있는 종목(커버리지가 얇다)도 리포트 목록은 보여 준다.
  const analyst: AnalystConsensus = data.consensus ?? { symbol: "", source: "naver", fetchedAt: "" };
  const trend = analystTrend(data.history);
  const counts = analyst.counts;
  const total = counts?.reduce((a, b) => a + b, 0) ?? 0;
  const buyers = counts ? counts[0] + counts[1] : 0;
  const upside =
    analyst.targetMean && price && price > 0 ? (analyst.targetMean / price - 1) * 100 : undefined;
  const sourceLabel = analyst.source === "naver" ? "네이버 증권 컨센서스" : "Finnhub 추천 추이";
  const targetCurrency = (analyst.targetCurrency as "KRW" | "USD") || currency;

  const verdict =
    total > 0
      ? `애널리스트 ${total}명 중 ${buyers}명(${Math.round((buyers / total) * 100)}%)이 매수 이상 의견입니다.`
      : analyst.rating != null
        ? `평균 의견은 「${ratingLabel(analyst.rating)}」${upside != null ? `, 평균 목표가는 지금보다 ${upside >= 0 ? "+" : ""}${upside.toFixed(1)}% ${upside >= 0 ? "위" : "아래"}입니다.` : "입니다."}`
        : null;

  return (
    <Card className="space-y-3 p-3">
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
          {analyst.asOf ? ` · ${analyst.asOf}` : ""}
        </span>
      </div>

      {verdict && <p className="text-cell leading-relaxed text-[var(--text-secondary)]">{verdict}</p>}
      {/* 2026-10-10 사업화 검토 N5 — 「매수 · +88%」가 이 서비스의 권유처럼 읽히지 않게 누구의 말인지 적는다. */}
      <p className="text-micro text-[var(--text-muted)]">
        증권사 의견을 모은 것이며 Quant Port의 판단이나 투자 권유가 아닙니다.
      </p>

      {analyst.rating != null && <RatingScale rating={analyst.rating} />}

      {analyst.targetMean != null && (
        <div className="space-y-1">
          <p className="flex flex-wrap items-baseline gap-x-2 text-cell">
            <span className="text-micro text-[var(--text-muted)]">평균 목표가</span>
            <b className="tabular-nums text-[var(--text-primary)]">{formatPrice(analyst.targetMean, targetCurrency)}</b>
            {price != null && price > 0 && (
              <span className="text-micro tabular-nums text-[var(--text-muted)]">
                현재가 {formatPrice(price, currency)}
              </span>
            )}
            {upside != null && (
              <span
                className={cn(
                  "ml-auto text-cell font-semibold tabular-nums",
                  upside >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                )}
              >
                {upside >= 0 ? "+" : ""}
                {upside.toFixed(1)}%
              </span>
            )}
          </p>
          {upside != null && <TargetBar upside={upside} />}
        </div>
      )}

      {counts && total > 0 && (
        <div className="space-y-1.5">
          <div className="flex h-2 overflow-hidden rounded-full bg-[var(--surface-elevated)]" aria-hidden>
            {counts.map((n, i) =>
              n > 0 ? <span key={i} className={COUNT_TONE[i]} style={{ width: `${(n / total) * 100}%` }} /> : null,
            )}
          </div>
          <ul className="grid grid-cols-5 gap-1 text-center">
            {counts.map((n, i) => (
              <li key={i} className="rounded-md bg-[var(--surface-elevated)]/60 px-1 py-1">
                <span className="block text-micro text-[var(--text-muted)]">{COUNT_LABELS[i]}</span>
                <b className="block text-cell tabular-nums text-[var(--text-primary)]">{n}</b>
                <span className="block text-micro tabular-nums text-[var(--text-muted)]">
                  {Math.round((n / total) * 100)}%
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {trend && <TrendChips trend={trend} />}
      {data.history.length >= 2 && <HistoryChart history={data.history} currency={currency} />}
      {data.reports.length > 0 && <ReportList reports={data.reports} />}
    </Card>
  );
}

/** 평균 의견(1~5)이 척도 위 어디인지. 막대 색은 매도(파랑) → 매수(빨강)로 이 앱의 등락 색과 같다. */
function RatingScale({ rating }: { rating: number }) {
  const pos = ((Math.min(5, Math.max(1, rating)) - 1) / 4) * 100;
  return (
    <div className="space-y-1">
      <div
        className="relative h-2 rounded-full"
        style={{ background: "linear-gradient(90deg, var(--loss) 0%, var(--muted) 50%, var(--profit) 100%)", opacity: 0.85 }}
        aria-hidden
      >
        <span
          className="absolute top-1/2 h-4 w-1 -translate-x-1/2 -translate-y-1/2 rounded-sm border border-[var(--card)] bg-[var(--text-primary)]"
          style={{ left: `${pos}%` }}
        />
      </div>
      <div className="flex justify-between text-micro text-[var(--text-muted)]">
        {SCALE_TICKS.map((t) => (
          <span key={t}>{t}</span>
        ))}
      </div>
      <p className="text-right text-micro tabular-nums text-[var(--text-muted)]">평균 {rating.toFixed(2)} / 5</p>
    </div>
  );
}

/** 현재가(왼쪽 끝)에서 목표가까지 — ±50%를 막대 끝으로 잘라 그린다. */
function TargetBar({ upside }: { upside: number }) {
  const width = Math.min(100, (Math.abs(upside) / 50) * 100);
  return (
    <div className="h-1.5 overflow-hidden rounded-full bg-[var(--surface-elevated)]" aria-hidden>
      <div
        className={cn("h-full rounded-full", upside >= 0 ? "bg-[var(--profit)]" : "bg-[var(--loss)]")}
        style={{ width: `${Math.max(2, width)}%` }}
      />
    </div>
  );
}

/** 「의견 변화」 — 약 90일 전(이력이 짧으면 가장 오래된 시점) 대비. 움직임이 없는 항목은 칩을 안 단다. */
function TrendChips({ trend }: { trend: AnalystTrend }) {
  const chips: { label: string; value: string; up: boolean }[] = [];
  if (trend.ratingDelta != null && Math.abs(trend.ratingDelta) >= 0.01) {
    chips.push({ label: "의견", value: `${trend.ratingDelta > 0 ? "+" : ""}${trend.ratingDelta.toFixed(2)}`, up: trend.ratingDelta > 0 });
  }
  if (trend.targetPct != null && Math.abs(trend.targetPct) >= 0.1) {
    chips.push({ label: "목표가", value: `${trend.targetPct > 0 ? "+" : ""}${trend.targetPct.toFixed(1)}%`, up: trend.targetPct > 0 });
  }
  if (trend.buyShareDelta != null && Math.abs(trend.buyShareDelta) >= 0.5) {
    chips.push({ label: "매수 비중", value: `${trend.buyShareDelta > 0 ? "+" : ""}${trend.buyShareDelta.toFixed(0)}%p`, up: trend.buyShareDelta > 0 });
  }
  if (trend.analystsDelta) {
    chips.push({ label: "애널리스트", value: `${trend.analystsDelta > 0 ? "+" : ""}${trend.analystsDelta}명`, up: trend.analystsDelta > 0 });
  }
  return (
    <div className="space-y-1">
      <p className="text-micro text-[var(--text-muted)]">{trend.sinceDate} 이후 변화</p>
      {chips.length === 0 ? (
        <p className="text-cell text-[var(--text-secondary)]">의견·목표가에 변화가 없습니다.</p>
      ) : (
        <div className="flex flex-wrap gap-1.5">
          {chips.map((c) => (
            <span
              key={c.label}
              className={cn(
                "rounded-md px-1.5 py-0.5 text-micro font-semibold tabular-nums",
                c.up ? "bg-[var(--profit)]/10 text-[var(--profit)]" : "bg-[var(--loss)]/10 text-[var(--loss)]",
              )}
            >
              {c.label} {c.value}
            </span>
          ))}
        </div>
      )}
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
