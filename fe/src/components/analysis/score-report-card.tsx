"use client";

import { HelpHint } from "@/components/ui/help-hint";
import { useEffect, useState } from "react";
import { SCORE_RANKING_AXIS_OPTIONS, type ScoreRankingAxis } from "@/lib/score-ranking-config";
import {
  fetchScoreReportCard,
  reportVerdict,
  type ScoreReportAxis,
  type ScoreReportCard,
} from "@/lib/score-report-card-api";
import { cn } from "@/lib/utils";

/**
 * 「축별 예측력」 — 백테스트 패널 아래의 성적표 (2026-09-23 사용자 결정 — 지금 공개).
 *
 * 백테스트는 「이 랭킹을 샀으면 얼마였나」이고, 이것은 「점수가 수익의 순서를 맞혔나」다.
 * 둘은 다르다 — 상위 20만 보는 백테스트는 운 좋은 몇 종목에 흔들리고, IC는 전 종목의 순서를 본다.
 *
 * **표본이 짧으면 짧다고 먼저 말한다.** 판정 단어가 숫자보다 앞에 선다.
 */

const VERDICT: Record<ReturnType<typeof reportVerdict>, { label: string; className: string }> = {
  short: { label: "표본 부족", className: "bg-muted text-muted-foreground" },
  noise: { label: "의미 없음", className: "bg-muted text-muted-foreground" },
  positive: {
    label: "맞힘",
    className: "bg-[var(--profit)]/10 text-[var(--profit)]",
  },
  negative: {
    label: "거꾸로",
    className: "bg-[var(--loss)]/10 text-[var(--loss)]",
  },
};

function axisName(axis: string): string {
  return SCORE_RANKING_AXIS_OPTIONS.find((o) => o.id === axis)?.label ?? axis;
}

function fmt(v: number | undefined, digits = 3): string {
  return v == null ? "—" : v.toFixed(digits);
}

export function ScoreReportCardSection({
  market,
  axis,
}: {
  market: "domestic" | "overseas";
  axis: ScoreRankingAxis;
}) {
  const [data, setData] = useState<ScoreReportCard | null>(null);
  const [error, setError] = useState<string | null>(null);
  const apiMarket = market === "overseas" ? "us" : "domestic";

  useEffect(() => {
    let alive = true;
    fetchScoreReportCard(apiMarket)
      .then((res) => {
        if (!alive) return;
        setData(res);
        setError(null);
      })
      .catch((e: unknown) => {
        if (alive) setError(e instanceof Error ? e.message : "성적표를 불러오지 못했습니다");
      });
    return () => {
      alive = false;
    };
  }, [apiMarket]);

  if (error) return <p className="mt-4 text-cell text-muted-foreground">{error}</p>;
  if (!data) return null;

  const selected = data.axes.find((a) => a.axis === axis) ?? data.axes[0];
  const maxDays = Math.max(0, ...data.axes.map((a) => a.days));

  return (
    <section className="mt-5 border-t border-[var(--border)] pt-4">
      <h3 className="flex items-center gap-1.5 text-cell font-semibold text-[var(--text-primary)]">
        축별 예측력 (성적표)
        <HelpHint>
          점수와 그 뒤 {data.horizon}거래일 수익률의 순위상관(IC) · 상위 {data.topN}종목의 전체 대비 초과수익입니다.
          IC가 0이면 점수가 순서를 못 맞힌 것입니다.
        </HelpHint>
      </h3>

      {maxDays === 0 ? (
        <p className="mt-3 rounded-lg border border-dashed border-[var(--border)] px-3 py-4 text-center text-cell text-muted-foreground">
          아직 잰 날이 없습니다. 점수일로부터 {data.horizon}거래일이 지나야 첫 결과가 나옵니다.
        </p>
      ) : (
        <>
          <p className="mt-2 rounded-md bg-amber-500/10 px-2 py-1.5 text-micro text-amber-800 dark:text-amber-300">
            표본 {maxDays}일 — {data.minDays}일 미만이거나 |t| &lt; 2이면 통계적으로 의미가
            없습니다. 날짜별 {data.horizon}일 창이 서로 겹치므로 t는 유효 표본(일수 ÷ {data.horizon}
            )으로 보정한 값입니다.
          </p>
          <table className="mt-2 w-full table-fixed text-cell">
            <thead>
              <tr className="text-left text-micro text-muted-foreground">
                <th className="w-[26%] py-1 font-normal">축</th>
                <th className="w-[20%] py-1 text-right font-normal">평균 IC</th>
                <th className="w-[16%] py-1 text-right font-normal">t</th>
                <th className="w-[20%] py-1 text-right font-normal">상위 초과</th>
                <th className="w-[18%] py-1 text-right font-normal">판정</th>
              </tr>
            </thead>
            <tbody>
              {data.axes.map((a) => {
                const v = VERDICT[reportVerdict(a, data.minDays)];
                return (
                  <tr
                    key={a.axis}
                    className={cn(
                      "border-t border-[var(--border)]/60",
                      a.axis === selected?.axis && "font-semibold",
                    )}
                  >
                    <td className="truncate py-1">{axisName(a.axis)}</td>
                    <td className="py-1 text-right font-num tabular-nums">{fmt(a.meanIC)}</td>
                    <td className="py-1 text-right font-num tabular-nums">{fmt(a.tStat, 1)}</td>
                    <td className="py-1 text-right font-num tabular-nums">
                      {a.meanTopExcess == null
                        ? "—"
                        : `${a.meanTopExcess >= 0 ? "+" : ""}${a.meanTopExcess.toFixed(2)}%p`}
                    </td>
                    <td className="py-1 text-right">
                      <span className={cn("rounded px-1 text-micro", v.className)}>{v.label}</span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          {selected && selected.series.length > 0 && (
            <IcBars axis={selected} methodDates={data.methodChanges.map((m) => m.date)} />
          )}
        </>
      )}

      {data.methodChanges.length > 0 && (
        <ul className="mt-2 space-y-0.5 text-micro text-muted-foreground">
          {data.methodChanges.map((m) => (
            <li key={m.date}>
              <span className="font-medium">{m.date} 산출 방식 변경</span> — {m.note}. 이 날 앞뒤의
              IC는 같은 잣대가 아닙니다.
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/** 날짜별 IC 막대. 가운데가 0, 위가 +. 산출 방식이 바뀐 날에 점선을 긋는다. */
function IcBars({ axis, methodDates }: { axis: ScoreReportAxis; methodDates: string[] }) {
  const s = axis.series;
  const w = 100;
  const h = 36;
  const barW = w / s.length;
  const scale = Math.max(0.05, ...s.map((p) => Math.abs(p.ic)));
  return (
    <figure className="mt-3">
      <svg
        viewBox={`0 0 ${w} ${h}`}
        preserveAspectRatio="none"
        className="h-16 w-full"
        role="img"
        aria-label={`${axisName(axis.axis)} 날짜별 IC`}
      >
        <line x1="0" x2={w} y1={h / 2} y2={h / 2} stroke="var(--border)" strokeWidth="0.3" />
        {s.map((p, i) => {
          const bh = (Math.abs(p.ic) / scale) * (h / 2 - 1);
          const y = p.ic >= 0 ? h / 2 - bh : h / 2;
          return (
            <rect
              key={p.date}
              x={i * barW + barW * 0.15}
              y={y}
              width={barW * 0.7}
              height={Math.max(bh, 0.3)}
              fill={p.ic >= 0 ? "var(--profit)" : "var(--loss)"}
              opacity={0.75}
            >
              <title>{`${p.date} IC ${p.ic.toFixed(3)}`}</title>
            </rect>
          );
        })}
        {methodDates.map((d) => {
          const i = s.findIndex((p) => p.date >= d);
          if (i < 0) return null;
          return (
            <line
              key={d}
              x1={i * barW}
              x2={i * barW}
              y1="0"
              y2={h}
              stroke="var(--text-muted)"
              strokeWidth="0.4"
              strokeDasharray="1.5 1.5"
            />
          );
        })}
      </svg>
      <figcaption className="mt-0.5 flex justify-between text-micro text-muted-foreground">
        <span>{s[0]?.date}</span>
        <span>{axisName(axis.axis)} · 날짜별 IC</span>
        <span>{s[s.length - 1]?.date}</span>
      </figcaption>
    </figure>
  );
}
