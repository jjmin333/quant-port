"use client";

import { useEffect, useState } from "react";
import { CalendarClock, Zap } from "lucide-react";
import { Card } from "@/components/ui/card";
import { fetchEarningsClient } from "@/lib/earnings-api";
import { earningsDaysFromToday, parseEarningsDateMs } from "@/lib/earnings-date";
import type { EarningsEvent, EarningsSession, EarningsSurpriseMetric, EarningsTabData } from "@/lib/earnings-types";
import { formatFinancialAmount, formatFinancialTick } from "@/lib/format";
import { fetchStockQA, type StockQA } from "@/lib/stock-qa";
import { cn } from "@/lib/utils";

/*
종목 개요 첫 화면의 두 조각 (2026-10-10 A안 → 10-13 요청 2·3·4, 시안 `docs/mockups/stock-overview-v2.html`).

  · 「최근 핵심 사건」 한 줄 — 규칙 기반(`/qa`의 최근 변화), 원인이 없으면 없다고 말한다.
  · 다음 실적 — 언제(확정/추정과 그 근거) · 얼마를 기대하나 · 지난 4번은 예상 대비 어땠고 주가는 어떻게 반응했나.

관심 등록 안내는 10-13에 개요에서 뺐다(종목 머리줄 ☆가 같은 일을 한다). 둘 다 자료가 없으면 아무것도 그리지 않는다.
*/

function useLoaded<T>(symbol: string, load: (s: string) => Promise<T>): T | null {
  const [state, setState] = useState<{ symbol: string; value: T | null } | null>(null);
  useEffect(() => {
    let alive = true;
    load(symbol)
      .then((value) => alive && setState({ symbol, value }))
      .catch(() => alive && setState({ symbol, value: null }));
    return () => {
      alive = false;
    };
  }, [symbol, load]);
  return state?.symbol === symbol ? state.value : null;
}

function md(d: string): string {
  const [, m, day] = d.split("-");
  return m && day ? `${Number(m)}/${Number(day)}` : d;
}

/** 「최근 핵심 사건」 — 최근 30일 변화 가운데 가장 최근 한 건 (10-13 요청 4, 그전 이름 「왜 움직였나」). */
export function StockWhyLine({ symbol, onMore }: { symbol: string; onMore?: () => void }) {
  const qa = useLoaded<StockQA>(symbol, fetchStockQA);
  const latest = qa?.changes[0];
  if (!qa || !latest) return null;
  const unexplained = latest.kind === "price_move" && latest.explained === false;
  return (
    <div className="flex items-start gap-2 rounded-lg bg-[var(--brand)]/[0.07] px-3 py-2 text-cell">
      <Zap className="mt-0.5 h-3.5 w-3.5 shrink-0 text-[var(--brand)]" aria-hidden />
      <p className="min-w-0 flex-1 leading-snug">
        <b className="mr-1 font-semibold text-[var(--text-primary)]">최근 핵심 사건</b>
        <span className="tabular-nums text-[var(--text-muted)]">{md(latest.date)}</span>{" "}
        <span className="text-[var(--text-secondary)]">{latest.title}</span>
        {unexplained && <span className="text-[var(--text-muted)]"> — 같은 날 뉴스·공시에서 뚜렷한 원인을 찾지 못했습니다</span>}
      </p>
      {onMore && qa.changes.length > 1 && (
        <button type="button" onClick={onMore} className="shrink-0 text-micro text-[var(--brand)] hover:underline">
          최근 변화 {qa.changes.length}건
        </button>
      )}
    </div>
  );
}

const METRIC_LABEL: Record<EarningsSurpriseMetric, string> = { opIncome: "영업이익", eps: "EPS", revenue: "매출" };
const SESSION_LABEL: Record<EarningsSession, string> = { bmo: "장 시작 전", amc: "장 마감 후", dmh: "장중" };
/** 예상 대비 ±이 % 안은 「부합」. */
const INLINE_PCT = 1;

function fmtMetric(v: number, metric: EarningsSurpriseMetric, currency: string): string {
  if (metric === "eps") return currency === "USD" ? `$${v.toFixed(2)}` : `${Math.round(v).toLocaleString("ko-KR")}원`;
  return currency === "KRW" ? formatFinancialTick(v, "KRW") : formatFinancialAmount(v, currency);
}

function periodLabel(e: EarningsEvent): string {
  return `${e.fiscalYear} ${e.fiscalPeriod === "FY" ? "연간" : e.fiscalPeriod}`;
}

function signed(pct: number): string {
  return `${pct > 0 ? "+" : pct < 0 ? "−" : ""}${Math.abs(pct).toFixed(1)}%`;
}

/**
 * 다음 실적 (2026-10-13 요청 3 — 「두루뭉술하지 않게」).
 *
 * 예전 막대 넷은 「무엇을 얼마나 예상했고 실제 얼마였나」·「발표 뒤 주가」·「날짜가 확정인가」를 말하지 못했다.
 * 이제 날짜 줄에 확정/추정과 그 근거를, 표에 예상·실제·예상 대비·주가 반응을 적고 한 문장으로 맺는다.
 */
export function NextEarningsCard({ symbol, className }: { symbol: string; className?: string }) {
  const tab = useLoaded<EarningsTabData>(symbol, fetchEarningsClient);
  if (!tab) return null;
  const next = tab.calendar.find((c) => c.kind === "upcoming");
  const today = new Date().toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
  const past = tab.announcements
    .filter((a) => a.date <= today && a.surprise?.pct != null)
    .sort((a, b) => b.date.localeCompare(a.date))
    .slice(0, 4);
  if (!next && past.length === 0) return null;

  const days = next ? earningsDaysFromToday(next.date) : null;
  const nextAt = next ? parseEarningsDateMs(next.date) : null;
  const nextDay =
    nextAt != null
      ? new Date(nextAt).toLocaleDateString("ko-KR", { month: "long", day: "numeric", weekday: "short", timeZone: "Asia/Seoul" })
      : next?.date;
  // 다가오는 발표의 예상치 — 해외는 Finnhub가 EPS 예상을 같은 행에 싣는다. 국내는 보통 없다.
  const upcoming = next ? tab.announcements.find((a) => a.date === next.date && !a.surprise) : undefined;
  const metric: EarningsSurpriseMetric = past[0]?.surprise?.metric ?? (upcoming?.eps.estimate != null ? "eps" : "opIncome");
  const upcomingEstimate = upcoming?.[metric]?.estimate;
  const lastActual = past[0]?.surprise?.metric === metric ? past[0].surprise.actual : undefined;
  const currency = past[0]?.currency ?? upcoming?.currency ?? "KRW";

  const beats = past.filter((a) => (a.surprise?.pct ?? 0) > INLINE_PCT).length;
  const reactions = past.map((a) => a.reaction).filter((r): r is NonNullable<typeof r> => r != null);
  const avgMove = reactions.length ? reactions.reduce((s, r) => s + Math.abs(r.pct), 0) / reactions.length : null;
  const ups = reactions.filter((r) => r.pct > 0).length;
  const reactionHead = reactions.every((r) => r.basis === "nextDay") ? "다음 날 주가" : "발표 뒤 주가";
  const source = tab.assetType === "domestic" ? "FnGuide" : "Finnhub";

  return (
    <Card className={cn("space-y-2 px-3 py-2.5", className)}>
      <div className="flex flex-wrap items-center gap-1.5">
        <CalendarClock className="h-3.5 w-3.5 text-[var(--brand)]" aria-hidden />
        <h3 className="text-cell font-semibold text-[var(--text-primary)]">다음 실적</h3>
        {days != null && days >= 0 && (
          <span className="rounded-md bg-[var(--brand)]/10 px-1.5 text-micro font-semibold text-[var(--brand)]">
            {days === 0 ? "오늘" : `D-${days}`}
          </span>
        )}
        {next && <span className="text-micro text-[var(--text-muted)]">· {next.label.replace(/\s*실적발표$/, "")}</span>}
      </div>

      {next ? (
        <div className="space-y-0.5 text-cell">
          <p className="flex flex-wrap items-center gap-x-1.5">
            <b className="text-[var(--text-primary)]">
              {nextDay}
              {next.session ? ` ${SESSION_LABEL[next.session]}` : ""}
            </b>
            <span
              className={cn(
                "rounded px-1.5 text-micro font-semibold",
                next.dateStatus === "estimated"
                  ? "bg-[var(--surface-elevated)] text-[var(--text-muted)]"
                  : "bg-[var(--brand)]/10 text-[var(--brand)]",
              )}
            >
              {next.dateStatus === "estimated" ? "추정일" : next.dateStatus === "confirmed" ? "확정" : "예정"}
            </span>
            {next.dateStatus === "estimated" && (
              <span className="text-micro text-[var(--text-muted)]">
                {next.estimateBasis ? `${next.estimateBasis} · ` : ""}회사가 아직 날짜를 공시하지 않았습니다
              </span>
            )}
            {next.session === "amc" && <span className="text-micro text-[var(--text-muted)]">한국 시간 다음 날 아침</span>}
          </p>
          {upcomingEstimate != null ? (
            <p className="text-[var(--text-secondary)]">
              시장 예상 {METRIC_LABEL[metric]} <b className="tabular-nums text-[var(--text-primary)]">{fmtMetric(upcomingEstimate, metric, currency)}</b>
              {lastActual != null && lastActual !== 0 && (
                <span className="text-micro text-[var(--text-muted)]">
                  {" "}
                  — 지난 분기 실제 {fmtMetric(lastActual, metric, currency)}보다 {signed((upcomingEstimate / lastActual - 1) * 100)}
                </span>
              )}
            </p>
          ) : (
            <p className="text-micro text-[var(--text-muted)]">이번 분기 예상치(컨센서스)는 아직 없습니다.</p>
          )}
        </div>
      ) : (
        <p className="text-cell text-[var(--text-secondary)]">다음 발표일은 아직 알려지지 않았습니다.</p>
      )}

      {past.length > 0 && (
        <div>
          <table className="w-full text-micro tabular-nums">
            <thead>
              <tr className="text-[var(--text-muted)]">
                <th className="py-1 text-left font-normal">지난 발표</th>
                <th className="py-1 text-right font-normal">예상 {METRIC_LABEL[metric]}</th>
                <th className="py-1 text-right font-normal">실제</th>
                <th className="py-1 text-right font-normal">예상 대비</th>
                <th className="py-1 text-right font-normal">{reactionHead}</th>
              </tr>
            </thead>
            <tbody>
              {past.map((a) => {
                const s = a.surprise!;
                const pct = s.pct ?? 0;
                const verdict = pct > INLINE_PCT ? "상회" : pct < -INLINE_PCT ? "하회" : "부합";
                const r = a.reaction?.pct;
                return (
                  <tr key={`${a.fiscalYear}-${a.fiscalPeriod}-${a.date}`} className="border-t border-[var(--border)]">
                    <td className="py-1 text-[var(--text-primary)]">
                      {periodLabel(a)} <span className="text-[var(--text-muted)]">{md(a.date)}</span>
                    </td>
                    <td className="py-1 text-right text-[var(--text-secondary)]">{fmtMetric(s.estimate, s.metric, a.currency)}</td>
                    <td className="py-1 text-right text-[var(--text-primary)]">{fmtMetric(s.actual, s.metric, a.currency)}</td>
                    <td
                      className={cn(
                        "py-1 text-right font-semibold",
                        verdict === "상회" ? "text-[var(--score-high)]" : verdict === "하회" ? "text-[var(--score-low)]" : "text-[var(--text-muted)]",
                      )}
                    >
                      {signed(pct)} {verdict}
                    </td>
                    <td
                      className={cn(
                        "py-1 text-right",
                        r == null ? "text-[var(--text-muted)]" : r > 0 ? "text-[var(--profit)]" : r < 0 ? "text-[var(--loss)]" : "",
                      )}
                    >
                      {r == null ? "—" : signed(r)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          <p className="mt-1.5 text-cell text-[var(--text-secondary)]">
            최근 {past.length}번 중{" "}
            <b className="text-[var(--text-primary)]">
              {beats === 0 ? "한 번도 예상을 넘지 못했고" : `${beats === past.length ? "모두" : `${beats}번`} 예상을 넘었고`}
            </b>
            {avgMove != null ? (
              <>
                , 발표 뒤 주가는 평균 <b className="text-[var(--text-primary)]">±{avgMove.toFixed(1)}%</b> 움직였습니다
                <span className="text-micro text-[var(--text-muted)]">
                  {" "}
                  (오름 {ups} · 내림 {reactions.length - ups})
                </span>
                .
              </>
            ) : (
              "."
            )}
          </p>
          <p className="text-micro text-[var(--text-muted)]">
            예상 = 증권사 컨센서스({source}) · ±{INLINE_PCT}% 안은 「부합」
          </p>
        </div>
      )}
    </Card>
  );
}
