"use client";

import { useEffect, useState } from "react";
import { Scale } from "lucide-react";
import {
  fetchAdminWeightSuggestion,
  type ScoreWeights,
  type WeightSuggestion,
} from "@/lib/admin";
import { toUserMessage } from "@/lib/error-message";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const AXIS_LABELS: Record<keyof ScoreWeights, string> = {
  technical: "추세",
  quality: "수익성",
  growth: "성장",
  valuation: "가치",
  sentiment: "심리·수급",
};

/**
 * 성적표 기반 가중치 제안 (2026-09-24 사용자 결정 — 「도구만 지금」).
 *
 * 제안만 한다. 「편집기에 채우기」는 위 가중치 편집기의 값을 바꿀 뿐이고, 저장·재계산은
 * 사람이 한다. 가중치는 전역 하나라 시장 탭은 근거를 보는 창이다 — 두 시장의 제안이
 * 다르면 어느 쪽을 따를지도 사람이 정한다.
 */
export function AdminWeightSuggestion({ onApply }: { onApply: (w: ScoreWeights) => void }) {
  const [market, setMarket] = useState<"domestic" | "us">("domestic");
  // 결과를 시장과 함께 둔다 — 탭을 바꾼 순간 이전 시장의 표가 새 탭 이름 아래 보이지 않게.
  const [result, setResult] = useState<{
    market: "domestic" | "us";
    data?: WeightSuggestion;
    error?: string;
  } | null>(null);

  useEffect(() => {
    let alive = true;
    fetchAdminWeightSuggestion(market)
      .then((d) => alive && setResult({ market, data: d }))
      .catch(
        (e) => alive && setResult({ market, error: toUserMessage(e, "제안을 불러오지 못했습니다") }),
      );
    return () => {
      alive = false;
    };
  }, [market]);

  const current = result?.market === market ? result : null;
  const data = current?.data ?? null;
  const error = current?.error ?? null;

  function apply() {
    if (!data?.ready) return;
    const next = {} as ScoreWeights;
    for (const a of data.axes) next[a.axis] = a.suggested;
    onApply(next);
  }

  return (
    <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 sm:p-4">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 className="flex items-center gap-2 text-section font-semibold">
          <Scale className="h-4 w-4 text-[var(--brand)]" />
          성적표 기반 가중치 제안
        </h2>
        <div className="flex items-center gap-1">
          {(["domestic", "us"] as const).map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMarket(m)}
              aria-pressed={market === m}
              className={cn(
                "rounded-md px-2 py-0.5 text-cell",
                market === m
                  ? "bg-[var(--brand)]/10 font-semibold text-[var(--brand)]"
                  : "text-[var(--text-muted)] hover:text-foreground",
              )}
            >
              {m === "domestic" ? "국내" : "미국"}
            </button>
          ))}
        </div>
      </div>
      <p className="mb-2 text-micro text-[var(--text-muted)]">
        축별 5일 앞 수익률 IC가 유의하게 양(+)인 축 쪽으로, 현재 값과 반씩 섞어 제안합니다. 표본 90일
        전에는 제안하지 않습니다. 저장하지 않습니다.
      </p>
      {error && <p className="text-cell text-[var(--danger)]">{error}</p>}
      {!data && !error && <p className="text-cell text-[var(--text-muted)]">불러오는 중…</p>}
      {data && (
        <>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[420px] text-cell">
              <thead className="text-micro text-[var(--text-muted)]">
                <tr>
                  <th className="py-1 text-left font-medium">축</th>
                  <th className="py-1 text-right font-medium">표본</th>
                  <th className="py-1 text-right font-medium">평균 IC</th>
                  <th className="py-1 text-right font-medium">t</th>
                  <th className="py-1 text-right font-medium">현재</th>
                  <th className="py-1 text-right font-medium">제안</th>
                </tr>
              </thead>
              <tbody>
                {data.axes.map((a) => {
                  const diff = a.suggested - a.current;
                  return (
                    <tr key={a.axis} className="border-t border-[var(--border)]">
                      <td className="py-1">{AXIS_LABELS[a.axis] ?? a.axis}</td>
                      <td className="py-1 text-right tabular-nums">{a.days}일</td>
                      <td className="py-1 text-right tabular-nums">
                        {a.meanIC == null ? "-" : a.meanIC.toFixed(3)}
                      </td>
                      <td
                        className={cn(
                          "py-1 text-right tabular-nums",
                          a.significant && "font-semibold text-foreground",
                        )}
                      >
                        {a.tStat == null ? "-" : a.tStat.toFixed(2)}
                      </td>
                      <td className="py-1 text-right tabular-nums">{a.current}</td>
                      <td className="py-1 text-right tabular-nums">
                        {data.ready ? (
                          <>
                            {a.suggested}
                            {diff !== 0 && (
                              <span
                                className={cn(
                                  "ml-1 text-micro",
                                  diff > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                                )}
                              >
                                {diff > 0 ? `+${diff}` : diff}
                              </span>
                            )}
                          </>
                        ) : (
                          "-"
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
            <p className="text-micro text-[var(--text-muted)]">
              {data.ready ? "현재 전역 가중치 대비 제안입니다(합 100)." : data.reason}
            </p>
            <button
              type="button"
              onClick={apply}
              disabled={!data.ready}
              className={cn(
                buttonVariants({ variant: "outline", size: "sm" }),
                "border-[var(--border)] disabled:opacity-50",
              )}
            >
              편집기에 채우기
            </button>
          </div>
        </>
      )}
    </section>
  );
}
