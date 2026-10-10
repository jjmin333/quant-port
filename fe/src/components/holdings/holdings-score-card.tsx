"use client";

import { useMemo } from "react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { Card } from "@/components/ui/card";
import type { HoldingMetrics, HoldingsTotals } from "@/lib/holdings-calc";
import {
  SCORE_TONE_BADGE,
  SCORE_TONE_TEXT,
  scoreTone,
  type StockScoreData,
} from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";

/**
 * 보유 포트폴리오 점수 — 다른 앱에는 없는 것.
 *
 * 증권사 앱은 손익만 보여준다. 이 앱은 종목마다 종합 점수를 갖고 있으니 "내가 들고 있는
 * 것들이 지금 좋은 종목인가"를 답할 수 있다.
 *
 * 평균을 **평가금액 가중**으로 내는 이유: 단순 평균이면 100만원짜리와 10만원짜리가 같은
 * 무게를 갖는다. 내 돈이 어디에 얼마나 놓여 있는지를 말하려면 금액으로 눌러야 한다.
 *
 * ⚠️ 점수는 정보이지 매매 신호가 아니다. "정리 대상" 같은 말을 쓰지 않고, 낮은 점수
 * 목록도 **"점수 낮은 보유 종목"**이라고만 부른다.
 */
export function HoldingsScoreCard({
  metrics,
  totals,
  scores,
  loading,
  onSelect,
}: {
  metrics: HoldingMetrics[];
  totals: HoldingsTotals | null;
  scores: Map<string, StockScoreData>;
  loading: boolean;
  /** 종목을 누르면 보유 목록과 **같은** 퀵룩 모달을 연다. */
  onSelect: (symbol: string) => void;
}) {
  const stats = useMemo(() => {
    const rows = metrics
      .map((m) => ({ m, score: scores.get(m.symbol.toUpperCase()) }))
      .filter((r): r is { m: HoldingMetrics; score: StockScoreData } => r.score?.score != null);
    if (rows.length === 0) return null;

    // 가중치는 평가금액. 값을 못 매긴 종목은 가중 평균에서 빼되, 개수에는 남긴다.
    let weightSum = 0;
    let weighted = 0;
    let highValue = 0;
    let lowValue = 0;
    for (const { m, score } of rows) {
      const w = m.marketValueKRW ?? 0;
      if (w > 0) {
        weightSum += w;
        weighted += w * (score.score ?? 0);
        if ((score.score ?? 0) >= 65) highValue += w;
        if ((score.score ?? 0) < 35) lowValue += w;
      }
    }
    const total = totals?.canTotal ? totals.marketValueKRW : 0;

    const bands = { high: 0, mid: 0, low: 0 };
    for (const { score } of rows) {
      bands[scoreTone(score.score ?? 0)] += 1;
    }

    return {
      average: weightSum > 0 ? weighted / weightSum : null,
      simpleAverage:
        rows.reduce((sum, r) => sum + (r.score.score ?? 0), 0) / rows.length,
      bands,
      scoredCount: rows.length,
      unscoredCount: metrics.length - rows.length,
      highPct: total > 0 ? (highValue / total) * 100 : null,
      lowPct: total > 0 ? (lowValue / total) * 100 : null,
      tradeDate: rows[0].score.tradeDate,
      lowest: [...rows]
        .sort((a, b) => (a.score.score ?? 0) - (b.score.score ?? 0))
        .slice(0, 4),
    };
  }, [metrics, scores, totals]);

  if (loading) {
    return (
      <Card className="p-3">
        <p className="mb-2 text-cell font-semibold">보유 종목 점수</p>
        <div className="h-24 animate-pulse rounded bg-muted" />
      </Card>
    );
  }

  if (!stats) {
    return (
      <Card className="p-3">
        <p className="mb-2 text-cell font-semibold">보유 종목 점수</p>
        <p className="py-8 text-center text-cell text-muted-foreground">
          점수가 산출된 보유 종목이 없습니다
        </p>
      </Card>
    );
  }

  const avg = stats.average ?? stats.simpleAverage;

  return (
      <Card className="p-3">
        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
          <p className="text-cell font-semibold">보유 종목 점수</p>
          <span className="text-micro text-muted-foreground">{stats.tradeDate} 기준</span>
        </div>

        <div className="flex items-baseline gap-2">
          <b className={cn("font-num text-2xl font-bold tabular-nums", SCORE_TONE_TEXT[scoreTone(avg)])}>
            {avg.toFixed(1)}
          </b>
          <span className="text-cell text-muted-foreground">
            {stats.average == null ? "단순 평균" : "평가금액 가중 평균"}
          </span>
        </div>

        {/* 등급 분포 — 종목 수 비율. 폭이 0인 구간은 아예 그리지 않는다. */}
        <div className="mt-2 flex h-5 overflow-hidden rounded-md">
          {(
            [
              ["high", "A", "var(--score-high)"],
              ["mid", "B·C", "var(--score-mid)"],
              ["low", "D", "var(--score-low)"],
            ] as const
          )
            .filter(([band]) => stats.bands[band] > 0)
            .map(([band, label, color]) => (
              <span
                key={band}
                className="grid place-items-center text-micro font-semibold text-white"
                style={{ flex: stats.bands[band], backgroundColor: color }}
              >
                {label} {stats.bands[band]}
              </span>
            ))}
        </div>

        <div className="mt-2 border-t pt-1.5">
          <Row
            label="65점 이상 비중"
            value={stats.highPct == null ? "—" : `${stats.highPct.toFixed(1)}%`}
          />
          <Row
            label="35점 미만 비중"
            value={stats.lowPct == null ? "—" : `${stats.lowPct.toFixed(1)}%`}
          />
          <Row
            label="점수 없음"
            value={stats.unscoredCount === 0 ? "없음" : `${stats.unscoredCount}종목`}
          />
        </div>

        <div className="mt-2 border-t pt-1.5">
          <p className="mb-1 text-cell font-semibold">점수 낮은 보유 종목</p>
          <ul className="min-w-0">
            {stats.lowest.map(({ m, score }) => (
              <li key={m.symbol}>
                <button
                  type="button"
                  onClick={() => onSelect(m.symbol)}
                  className="flex h-[30px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-left text-cell hover:bg-accent"
                >
                  <StockAvatar
                    symbol={m.symbol}
                    name={m.name}
                    currency={m.currency}
                    assetType={m.row.quote?.assetType}
                    size="xs"
                    className="shrink-0"
                  />
                  <span className="min-w-0 flex-1 truncate">{m.name}</span>
                  <span
                    className={cn(
                      "shrink-0 rounded px-1 text-micro font-semibold",
                      SCORE_TONE_BADGE[scoreTone(score.score ?? 0)],
                    )}
                  >
                    {score.score}
                    {score.grade ? ` ${score.grade}` : ""}
                  </span>
                  <span className="w-11 shrink-0 text-right font-num tabular-nums">
                    {m.weight == null ? "—" : `${m.weight.toFixed(1)}%`}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      </Card>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex min-w-0 items-center justify-between gap-2 py-0.5">
      <span className="shrink-0 text-cell text-muted-foreground">{label}</span>
      <b className="min-w-0 truncate font-num text-cell tabular-nums">{value}</b>
    </div>
  );
}
