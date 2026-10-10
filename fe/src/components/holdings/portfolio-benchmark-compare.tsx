"use client";

import { useEffect, useMemo, useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { fetchIndexDailyCloses } from "@/lib/daily-close-api";
import type { DailyCloseRow } from "@/lib/daily-close-types";
import type { ValuationPoint } from "@/lib/holding-trades-types";
import { alignIndex, portfolioTWR } from "@/lib/portfolio-benchmark";
import { fetchBacktestBenchmarksClient } from "@/lib/score-backtest-api";
import { cn } from "@/lib/utils";

/**
 * 평가금액 추이 카드 안의 「지수와 비교」 (2026-09-24 사용자 결정).
 *
 * 접힌 한 줄로 시작한다 — 카드의 기본 얼굴은 평가금액이고, 비교는 궁금할 때 여는 것이다.
 * 펼치면 같은 기간의 **내 포트폴리오 수익률(TWR — 추가 매수를 뺀 값)**과 고른 지수를 0%에서
 * 함께 그린다. 지수 목록은 AI 랭킹 백테스트와 같은 화이트리스트다(화면이 이름을 하드코딩하지
 * 않는다). 색은 내 포트폴리오에만 주고 지수는 점선이다 — 빨강·파랑은 이 앱에서 등락의 색이다.
 */
export function PortfolioBenchmarkCompare({ points }: { points: ValuationPoint[] }) {
  const [open, setOpen] = useState(false);
  const [options, setOptions] = useState<{ id: string; label: string }[]>([]);
  const [benchmark, setBenchmark] = useState("kospi");
  const [rows, setRows] = useState<{ id: string; rows: DailyCloseRow[] } | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open || options.length > 0) return;
    fetchBacktestBenchmarksClient()
      .then((c) => {
        if (!c) return;
        setOptions([...(c.catalog.domestic ?? []), ...(c.catalog.overseas ?? [])]);
      })
      .catch(() => {});
  }, [open, options.length]);

  useEffect(() => {
    if (!open || points.length < 2) return;
    let alive = true;
    // 기간보다 넉넉히 받는다 — 시작일 이하의 종가가 있어야 0%를 잡는다.
    fetchIndexDailyCloses(benchmark, Math.min(400, points.length + 15))
      .then((s) => {
        if (!alive) return;
        setRows({ id: benchmark, rows: s.rows });
        setError(null);
      })
      .catch((e: unknown) => {
        if (alive) setError(e instanceof Error ? e.message : "지수를 불러오지 못했습니다");
      });
    return () => {
      alive = false;
    };
  }, [open, benchmark, points.length]);

  const view = useMemo(() => {
    if (!rows || rows.id !== benchmark || points.length < 2) return null;
    const mine = portfolioTWR(points);
    const idx = alignIndex(
      points.map((p) => p.date),
      rows.rows,
    );
    if (!idx) return null;
    const all = [...mine.map((p) => p.pct), ...idx.map((p) => p.pct), 0];
    const hi = Math.max(...all);
    const lo = Math.min(...all);
    const span = hi - lo || 1;
    const x = (i: number) => (i / (points.length - 1)) * 600;
    const y = (v: number) => 100 - ((v - lo) / span) * 100;
    const path = (s: { pct: number }[]) =>
      s.map((p, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(p.pct).toFixed(1)}`).join(" ");
    const lastMine = mine[mine.length - 1]!.pct;
    const lastIdx = idx[idx.length - 1]!.pct;
    return { minePath: path(mine), idxPath: path(idx), zeroY: y(0), lastMine, lastIdx };
  }, [rows, benchmark, points]);

  const label = options.find((o) => o.id === benchmark)?.label ?? benchmark.toUpperCase();

  return (
    <div className="mt-2 border-t border-[var(--border)] pt-1.5">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex w-full items-center gap-1 text-left text-cell text-muted-foreground hover:text-foreground"
      >
        {open ? (
          <ChevronDown className="h-3.5 w-3.5" aria-hidden />
        ) : (
          <ChevronRight className="h-3.5 w-3.5" aria-hidden />
        )}
        지수와 비교
        {view && (
          <span className="ml-auto font-num tabular-nums">
            내 포트폴리오 {fmtPct(view.lastMine)} · {label} {fmtPct(view.lastIdx)}
          </span>
        )}
      </button>

      {open && (
        <div className="mt-2">
          <div className="-mx-1 flex gap-1 overflow-x-auto px-1 pb-1">
            {options.map((o) => (
              <button
                key={o.id}
                type="button"
                onClick={() => setBenchmark(o.id)}
                aria-pressed={benchmark === o.id}
                className={cn(
                  "shrink-0 rounded-full border px-2 py-0.5 text-micro",
                  benchmark === o.id
                    ? "border-foreground bg-foreground text-background"
                    : "border-[var(--border)] text-muted-foreground hover:text-foreground",
                )}
              >
                {o.label}
              </button>
            ))}
          </div>
          {error ? (
            <p className="py-3 text-center text-cell text-muted-foreground">{error}</p>
          ) : !view ? (
            <p className="py-3 text-center text-cell text-muted-foreground">
              {rows && rows.id === benchmark
                ? "이 기간의 시작일에 지수 종가가 없어 비교하지 못했습니다."
                : "지수를 불러오는 중…"}
            </p>
          ) : (
            <>
              <svg
                viewBox="0 0 600 100"
                preserveAspectRatio="none"
                className="h-24 w-full"
                role="img"
                aria-label={`내 포트폴리오와 ${label} 비교`}
              >
                <line
                  x1="0"
                  x2="600"
                  y1={view.zeroY}
                  y2={view.zeroY}
                  stroke="var(--border)"
                  strokeWidth="1"
                />
                <path
                  d={view.idxPath}
                  fill="none"
                  stroke="var(--text-muted)"
                  strokeWidth="1.5"
                  strokeDasharray="5 4"
                  vectorEffect="non-scaling-stroke"
                />
                <path
                  d={view.minePath}
                  fill="none"
                  stroke="var(--brand)"
                  strokeWidth="2"
                  vectorEffect="non-scaling-stroke"
                />
              </svg>
              <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-micro text-muted-foreground">
                <span className="flex items-center gap-1">
                  <span className="inline-block h-0.5 w-3 bg-[var(--brand)]" aria-hidden />내
                  포트폴리오 {fmtPct(view.lastMine)}
                </span>
                <span className="flex items-center gap-1">
                  <span
                    className="inline-block w-3 border-t border-dashed border-[var(--text-muted)]"
                    aria-hidden
                  />
                  {label} {fmtPct(view.lastIdx)}
                </span>
                <span className="ml-auto font-num tabular-nums">
                  차이 {fmtPct(view.lastMine - view.lastIdx)}p
                </span>
              </div>
              <p className="mt-1 text-micro text-muted-foreground">내 포트폴리오: 시간가중 수익률(추가 매수 제외)</p>
            </>
          )}
        </div>
      )}
    </div>
  );
}

function fmtPct(v: number): string {
  return `${v >= 0 ? "+" : ""}${v.toFixed(2)}%`;
}
