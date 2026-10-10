"use client";

import { PortfolioBenchmarkCompare } from "@/components/holdings/portfolio-benchmark-compare";
import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  BulkRegisterDialog,
  submitBulkRegister,
} from "@/components/holdings/bulk-register-dialog";
import { fetchValuationSeriesClient } from "@/lib/holding-trades-api";
import type { ValuationSeries } from "@/lib/holding-trades-types";
import { formatKRW, formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";

export type UntrackedHoldingInput = {
  symbol: string;
  name: string;
  quantity: number;
  avgPrice: number | null;
  currency: "KRW" | "USD";
};

/**
 * 평가금액 추이 — **실제 매매 이력** 기준.
 *
 * 이 화면의 원래 한계는 `user_holdings`가 현재 수량만 가진다는 것이었다. 그래서 곡선을
 * 그리려면 "지금 수량을 과거 전체에 곱한다"는 합성밖에 못 했고, 두 달 전에 산 종목이
 * 석 달 전부터 있던 것처럼 보였다. 이제 매매 이력에서 **날짜별 실제 보유수량**을 복원해
 * 그날 종가로 값을 매긴다(계산은 BE `/user/holdings/valuation`).
 *
 * 대신 새 한계가 생겼다 — 이력을 넣지 않은 종목은 과거 어느 날에도 존재할 수 없다.
 * 그건 숨기지 않고 카드 하단에 이름을 적는다. 일부 종목이 빠진 곡선을 "내 포트폴리오"
 * 라고 부르면 안 된다.
 *
 * 매입금액 선을 함께 그리는 이유: 평가금액만 있으면 오르내림이 시장 때문인지 내가
 * 더 샀기 때문인지 구분되지 않는다. 두 선의 벌어진 간격이 곧 평가손익이다.
 */

const RANGES: Array<{ label: string; days: number }> = [
  { label: "1개월", days: 30 },
  { label: "3개월", days: 90 },
  { label: "1년", days: 365 },
];

export function HoldingsValuationChart({
  reloadKey,
  untrackedHoldings = [],
  onTradesChanged,
}: {
  reloadKey?: number;
  /** 매매 이력 없이 보유만 된 종목 — 일괄 등록 대상 */
  untrackedHoldings?: UntrackedHoldingInput[];
  onTradesChanged?: () => void;
}) {
  const [days, setDays] = useState(90);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkError, setBulkError] = useState<string | null>(null);
  const [bulkModalOpen, setBulkModalOpen] = useState(false);
  // 어떤 요청(기간 + 이력 버전)에 대한 결과인지 함께 들고 있는다. 별도 loading 플래그를
  // 이펙트 안에서 동기로 켜면 렌더가 한 번 더 돌고, 그 프레임 동안 **이전 기간의 곡선이
  // 새 탭 밑에 남는다**.
  const [loaded, setLoaded] = useState<{
    key: string;
    series: ValuationSeries | null;
    error: string | null;
  } | null>(null);
  const [hover, setHover] = useState<number | null>(null);

  const key = `${days}:${reloadKey ?? 0}`;
  const current = loaded?.key === key ? loaded : null;
  const loading = current === null;
  const series = current?.series ?? null;
  const error = current?.error ?? null;

  useEffect(() => {
    let cancelled = false;
    fetchValuationSeriesClient(days)
      .then((next) => {
        if (!cancelled) setLoaded({ key, series: next, error: null });
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setLoaded({
            key,
            series: null,
            error: toUserMessage(err, "불러오기 실패"),
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [days, key]);

  const chart = useMemo(() => {
    const points = series?.points ?? [];
    if (points.length < 2) return null;

    const values = points.map((p) => p.valueKrw);
    const costs = points.map((p) => p.costKrw);
    const all = [...values, ...costs].filter((v) => v > 0);
    const max = Math.max(...all);
    const min = Math.min(...all);
    const span = max - min || max || 1;
    // 위아래 6% 여백 — 선이 카드 테두리에 붙으면 값이 잘린 것처럼 보인다.
    const pad = span * 0.06;
    const lo = min - pad;
    const hi = max + pad;

    const x = (i: number) => (i / (points.length - 1)) * 600;
    const y = (v: number) => 120 - ((v - lo) / (hi - lo)) * 120;

    const line = (arr: number[]) =>
      arr.map((v, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");

    const first = values[0];
    const last = values[values.length - 1];
    const rate = first > 0 ? ((last - first) / first) * 100 : 0;

    return {
      points,
      valuePath: line(values),
      // 매입금액이 0인 구간(전부 팔았던 날)은 선을 끊지 않고 그대로 0으로 둔다 —
      // 실제로 그날 원가가 0이었던 게 맞다.
      costPath: line(costs),
      areaPath: `${line(values)} L600,120 L0,120 Z`,
      x,
      y,
      first,
      last,
      rate,
    };
  }, [series]);

  const hoveredPoint = chart && hover != null ? chart.points[hover] : null;

  const untrackedSet = useMemo(() => {
    const fromSeries = series?.untrackedSymbols ?? [];
    if (fromSeries.length > 0) {
      return new Set(fromSeries.map((s) => s.toUpperCase()));
    }
    // 이력 API 응답 전·거래 0건일 때는 전달받은 보유 목록 전체를 후보로 본다.
    if (!loading && !series?.points?.length) {
      return new Set(untrackedHoldings.map((h) => h.symbol.toUpperCase()));
    }
    return new Set<string>();
  }, [series, loading, untrackedHoldings]);

  const bulkCandidates = useMemo(
    () =>
      untrackedHoldings.filter(
        (h) =>
          untrackedSet.has(h.symbol.toUpperCase()) &&
          h.quantity > 0 &&
          h.avgPrice != null &&
          h.avgPrice > 0,
      ),
    [untrackedHoldings, untrackedSet],
  );
  const bulkSkipped = untrackedHoldings.filter(
    (h) =>
      untrackedSet.has(h.symbol.toUpperCase()) &&
      !bulkCandidates.some((c) => c.symbol === h.symbol),
  );

  const bulkRegister = async (items: Array<UntrackedHoldingInput & { tradeDate: string }>) => {
    if (items.length === 0) return;
    setBulkBusy(true);
    setBulkError(null);
    try {
      await submitBulkRegister(items);
      setBulkModalOpen(false);
      onTradesChanged?.();
    } catch (err: unknown) {
      setBulkError(toUserMessage(err, "일괄 등록 실패"));
    } finally {
      setBulkBusy(false);
    }
  };

  const showBulkPrompt =
    !loading &&
    !error &&
    bulkCandidates.length > 0 &&
    (!chart || (series?.untrackedSymbols?.length ?? 0) > 0);

  return (
    <>
    <Card className="p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <p className="text-cell font-semibold">평가금액 추이</p>
        <div className="flex items-center gap-0.5 rounded-md bg-muted p-0.5">
          {RANGES.map((r) => (
            <button
              key={r.days}
              type="button"
              onClick={() => setDays(r.days)}
              aria-pressed={days === r.days}
              className={cn(
                "rounded px-2 py-0.5 text-cell transition-colors",
                days === r.days
                  ? "bg-card font-semibold text-foreground shadow-xs"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="h-[120px] animate-pulse rounded bg-muted" />
      ) : error ? (
        <p className="py-10 text-center text-cell text-muted-foreground">{error}</p>
      ) : !chart ? (
        <div className="py-6 text-center">
          <p className="text-cell leading-relaxed text-muted-foreground">
            매매 이력이 없어 추이를 그릴 수 없습니다.
            <br />
            종목을 눌러 매수 기록을 추가하면 그날부터의 실제 평가금액이 쌓입니다.
          </p>
          {showBulkPrompt && (
            <BulkRegisterBlock
              candidates={bulkCandidates}
              skipped={bulkSkipped}
              busy={bulkBusy}
              error={bulkError}
              onOpenModal={() => setBulkModalOpen(true)}
            />
          )}
        </div>
      ) : (
        <>
          <div className="relative">
            <svg
              viewBox="0 0 600 120"
              preserveAspectRatio="none"
              className="block h-[120px] w-full"
              onMouseLeave={() => setHover(null)}
              onMouseMove={(e) => {
                const rect = e.currentTarget.getBoundingClientRect();
                const ratio = (e.clientX - rect.left) / rect.width;
                const idx = Math.round(ratio * (chart.points.length - 1));
                setHover(Math.max(0, Math.min(chart.points.length - 1, idx)));
              }}
            >
              <defs>
                <linearGradient id="holdings-val-fill" x1="0" y1="0" x2="0" y2="1">
                  <stop
                    offset="0%"
                    stopColor={chart.rate >= 0 ? "var(--profit)" : "var(--loss)"}
                    stopOpacity="0.26"
                  />
                  <stop
                    offset="100%"
                    stopColor={chart.rate >= 0 ? "var(--profit)" : "var(--loss)"}
                    stopOpacity="0"
                  />
                </linearGradient>
              </defs>
              <path d={chart.areaPath} fill="url(#holdings-val-fill)" />
              <path
                d={chart.costPath}
                fill="none"
                stroke="var(--muted-foreground)"
                strokeWidth="1"
                strokeDasharray="3 3"
                vectorEffect="non-scaling-stroke"
              />
              <path
                d={chart.valuePath}
                fill="none"
                stroke={chart.rate >= 0 ? "var(--profit)" : "var(--loss)"}
                strokeWidth="1.6"
                vectorEffect="non-scaling-stroke"
              />
              {hover != null && (
                <line
                  x1={chart.x(hover)}
                  y1="0"
                  x2={chart.x(hover)}
                  y2="120"
                  stroke="var(--muted-foreground)"
                  strokeWidth="1"
                  vectorEffect="non-scaling-stroke"
                />
              )}
            </svg>
          </div>

          <div className="mt-1.5 flex flex-wrap items-center justify-between gap-x-4 gap-y-1 text-micro text-muted-foreground">
            {hoveredPoint ? (
              <>
                <span>{hoveredPoint.date}</span>
                <span className="font-num tabular-nums text-foreground">
                  평가 {formatKRW(Math.round(hoveredPoint.valueKrw))}
                </span>
                <span className="font-num tabular-nums">
                  매입 {formatKRW(Math.round(hoveredPoint.costKrw))}
                </span>
              </>
            ) : (
              <>
                <span>
                  {chart.points[0].date} · {formatKRW(Math.round(chart.first))}
                </span>
                <span className={cn("font-num tabular-nums", trendTextClass(chart.rate))}>
                  구간 {formatPercent(chart.rate)}
                </span>
                <span>
                  {chart.points[chart.points.length - 1].date} ·{" "}
                  {formatKRW(Math.round(chart.last))}
                </span>
              </>
            )}
          </div>

          <p className="mt-1.5 border-t pt-1.5 text-micro leading-relaxed text-muted-foreground">
            실선 = 평가금액, 점선 = 매입금액
            {series?.untrackedSymbols && series.untrackedSymbols.length > 0 && (
              <>
                {" "}
                매매 이력이 없는 {series.untrackedSymbols.length}종목(
                {series.untrackedSymbols.slice(0, 3).join(", ")}
                {series.untrackedSymbols.length > 3 ? " 외" : ""})은 이 곡선에 포함되지
                않았습니다.
              </>
            )}
            {series?.missingSymbols && series.missingSymbols.length > 0 && (
              <> 일별 종가가 없는 {series.missingSymbols.join(", ")}도 빠졌습니다.</>
            )}
          </p>

          {/* 같은 기간 지수와 비교 — 접힌 한 줄로 시작한다 (2026-09-24 사용자 결정). */}
          {chart && <PortfolioBenchmarkCompare points={chart.points} />}

          {showBulkPrompt && (
            <BulkRegisterBlock
              candidates={bulkCandidates}
              skipped={bulkSkipped}
              busy={bulkBusy}
              error={bulkError}
              onOpenModal={() => setBulkModalOpen(true)}
            />
          )}
        </>
      )}
    </Card>

    <BulkRegisterDialog
      open={bulkModalOpen}
      onOpenChange={setBulkModalOpen}
      candidates={bulkCandidates}
      skipped={bulkSkipped}
      busy={bulkBusy}
      error={bulkError}
      onSubmit={(items) => void bulkRegister(items)}
    />
    </>
  );
}

function BulkRegisterBlock({
  candidates,
  skipped,
  busy,
  error,
  onOpenModal,
}: {
  candidates: UntrackedHoldingInput[];
  skipped: UntrackedHoldingInput[];
  busy: boolean;
  error: string | null;
  onOpenModal: () => void;
}) {
  return (
    <div className="mx-auto mt-4 max-w-sm rounded-lg border border-dashed bg-muted/30 p-3 text-left">
      <p className="text-cell font-medium text-foreground">
        보유 {candidates.length}종목을 평가금액 추이에 등록
      </p>
      <p className="mt-1 text-micro leading-relaxed text-muted-foreground">
        현재 수량·평단으로 매수 기록을 만듭니다. 종목별 매수일을 지정할 수 있습니다.
      </p>
      <p className="mt-1 truncate text-micro text-muted-foreground">
        {candidates
          .slice(0, 4)
          .map((h) => h.name || h.symbol)
          .join(", ")}
        {candidates.length > 4 ? ` 외 ${candidates.length - 4}종목` : ""}
      </p>
      {skipped.length > 0 && (
        <p className="mt-1 text-micro text-muted-foreground">
          평단이 없어 제외: {skipped.map((h) => h.name || h.symbol).join(", ")}
        </p>
      )}
      {error && <p className="mt-1 text-micro text-[var(--danger)]">{error}</p>}
      <Button
        type="button"
        size="sm"
        className="mt-2 w-full"
        disabled={busy || candidates.length === 0}
        onClick={onOpenModal}
      >
        {busy ? "등록 중…" : "일괄 등록하기"}
      </Button>
    </div>
  );
}
