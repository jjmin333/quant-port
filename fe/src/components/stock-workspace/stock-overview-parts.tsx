"use client";

import { useEffect, useState, useSyncExternalStore } from "react";
import { CalendarClock, Star, Zap } from "lucide-react";
import { Card } from "@/components/ui/card";
import { fetchEarningsClient } from "@/lib/earnings-api";
import { earningsDaysFromToday, parseEarningsDateMs } from "@/lib/earnings-date";
import type { EarningsTabData } from "@/lib/earnings-types";
import { fetchStockQA, type StockQA } from "@/lib/stock-qa";
import { useLoginGate } from "@/hooks/use-login-gate";
import {
  ensureWatchlist,
  getWatchlistSymbols,
  isWatchlistLoaded,
  subscribeWatchlist,
  toggleWatchlistSymbol,
} from "@/lib/watchlist-store";
import { cn } from "@/lib/utils";

/*
종목 개요 첫 화면의 세 조각 (2026-10-10 사용자 결정 — 시안 `docs/mockups/business-w4-v1.html` A안).

  · 「왜 움직였나」 한 줄 — 토스 AI 시그널 자리. 규칙 기반(`/qa`의 최근 변화), 원인이 없으면 없다고 말한다.
  · 다음 실적 — D-n과 지난 4번의 서프라이즈 막대.
  · 관심 등록 안내 — 검색으로 들어온 사람을 붙잡는 단 하나의 자리. 약속은 지금 있는 것(사이드바 「종목 찾기」 배지)만.

셋 다 자료가 없으면 아무것도 그리지 않는다.
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

/** 「왜 움직였나」 — 최근 30일 변화 가운데 가장 최근 한 건. */
export function StockWhyLine({ symbol, onMore }: { symbol: string; onMore?: () => void }) {
  const qa = useLoaded<StockQA>(symbol, fetchStockQA);
  const latest = qa?.changes[0];
  if (!qa || !latest) return null;
  const unexplained = latest.kind === "price_move" && latest.explained === false;
  return (
    <div className="flex items-start gap-2 rounded-lg bg-[var(--brand)]/[0.07] px-3 py-2 text-cell">
      <Zap className="mt-0.5 h-3.5 w-3.5 shrink-0 text-[var(--brand)]" aria-hidden />
      <p className="min-w-0 flex-1 leading-snug">
        <b className="mr-1 font-semibold text-[var(--text-primary)]">왜 움직였나</b>
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

const SURPRISE_LABEL: Record<string, string> = { opIncome: "영업이익", eps: "EPS", revenue: "매출" };

/** 다음 실적 D-n + 지난 4번의 서프라이즈. */
export function NextEarningsCard({ symbol, className }: { symbol: string; className?: string }) {
  const tab = useLoaded<EarningsTabData>(symbol, fetchEarningsClient);
  if (!tab) return null;
  const next = tab.calendar.find((c) => c.kind === "upcoming");
  const today = new Date().toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
  const past = tab.announcements
    .filter((a) => a.date <= today && a.surprise?.pct != null)
    .sort((a, b) => b.date.localeCompare(a.date))
    .slice(0, 4)
    .reverse();
  if (!next && past.length === 0) return null;
  const days = next ? earningsDaysFromToday(next.date) : null;
  // D-n은 배지가 말한다 — 본문은 날짜로(「D-19 … D-19」가 겹쳐 나왔다).
  const nextAt = next ? parseEarningsDateMs(next.date) : null;
  const nextDay =
    nextAt != null
      ? new Date(nextAt).toLocaleDateString("en-US", { month: "numeric", day: "numeric", timeZone: "Asia/Seoul" })
      : next?.date;
  const metric = past[past.length - 1]?.surprise?.metric;
  const maxAbs = Math.max(5, ...past.map((a) => Math.abs(a.surprise?.pct ?? 0)));

  return (
    <Card className={cn("space-y-2 px-3 py-2.5", className)}>
      <div className="flex items-center gap-1.5">
        <CalendarClock className="h-3.5 w-3.5 text-[var(--brand)]" aria-hidden />
        <h3 className="text-cell font-semibold text-[var(--text-primary)]">다음 실적</h3>
        {days != null && days >= 0 && (
          <span className="rounded-md bg-[var(--brand)]/10 px-1.5 text-micro font-semibold text-[var(--brand)]">
            {days === 0 ? "오늘" : `D-${days}`}
          </span>
        )}
      </div>
      <p className="text-cell text-[var(--text-secondary)]">
        {next ? `${nextDay} ${next.label}` : "다음 발표일은 아직 알려지지 않았습니다."}
      </p>
      {past.length > 0 && (
        <div>
          {/* 막대 높이 = |서프라이즈| (가장 큰 것 기준), 색 = 넘었나(초록)·못 미쳤나(빨강). 숫자를 막대 아래에 적는다. */}
          <div className="flex items-end gap-2" aria-label="지난 실적 서프라이즈">
            {past.map((a) => {
              const pct = a.surprise?.pct ?? 0;
              const h = Math.max(10, (Math.abs(pct) / maxAbs) * 100);
              return (
                <div key={`${a.fiscalYear}-${a.fiscalPeriod}`} className="flex min-w-0 flex-1 flex-col items-center gap-0.5">
                  <div className="flex h-10 w-full items-end">
                    <div
                      className={cn("w-full rounded-t", pct >= 0 ? "bg-[var(--score-high)]" : "bg-[var(--score-low)]")}
                      style={{ height: `${h}%` }}
                    />
                  </div>
                  <span className={cn("text-micro font-semibold tabular-nums", pct >= 0 ? "text-[var(--score-high)]" : "text-[var(--score-low)]")}>
                    {pct >= 0 ? "+" : ""}
                    {Math.abs(pct) >= 100 ? Math.round(pct) : pct.toFixed(1)}%
                  </span>
                  <span className="truncate text-micro text-[var(--text-muted)]">{a.fiscalPeriod}</span>
                </div>
              );
            })}
          </div>
          <p className="mt-1 text-micro text-[var(--text-muted)]">
            지난 {past.length}번 {SURPRISE_LABEL[metric ?? ""] ?? "실적"} 서프라이즈 · 예상을 넘은 {past.filter((a) => (a.surprise?.pct ?? 0) >= 0).length}번
          </p>
        </div>
      )}
    </Card>
  );
}

const EMPTY = new Set<string>();

/** 관심 등록 안내 — 이미 담았으면 그리지 않는다. */
export function WatchPrompt({ symbol, name }: { symbol: string; name: string }) {
  const watched = useSyncExternalStore(subscribeWatchlist, getWatchlistSymbols, () => EMPTY);
  const gate = useLoginGate("관심 등록");
  const [error, setError] = useState(false);
  useEffect(() => {
    if (gate.authed) ensureWatchlist().catch(() => {});
  }, [gate.authed]);
  // 로그인한 사람은 목록을 받기 전에 그리지 않는다 — 이미 담은 종목에 안내가 떴다 사라진다.
  if (gate.authed && !isWatchlistLoaded()) return null;
  if (watched.has(symbol.trim().toUpperCase())) return null;
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg border border-dashed border-[var(--brand)]/50 px-3 py-2 text-cell">
      <p className="min-w-0 flex-1 text-[var(--text-secondary)]">
        <b className="text-[var(--text-primary)]">{name}</b>을(를) 관심 종목에 담으면 점수가 바뀌거나 실적 발표가 다가올 때
        「종목 찾기 › 오늘의 변화」에 표시합니다.
        {error && <span className="ml-1 text-[var(--danger)]">담지 못했습니다. 다시 눌러 주세요.</span>}
      </p>
      <button
        type="button"
        onClick={() =>
          gate.run(async () => {
            setError(false);
            try {
              await toggleWatchlistSymbol(symbol);
            } catch {
              setError(true);
            }
          })
        }
        className="inline-flex shrink-0 items-center gap-1 rounded-md bg-[var(--brand)] px-3 py-1.5 text-cell font-semibold text-white hover:opacity-90"
      >
        <Star className="h-3.5 w-3.5" aria-hidden />
        {gate.authed ? "관심 등록" : "로그인하고 관심 등록"}
      </button>
    </div>
  );
}
