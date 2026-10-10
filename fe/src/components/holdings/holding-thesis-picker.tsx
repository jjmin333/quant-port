"use client";

import { useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { apiRequest } from "@/lib/api-client";
import { SCORE_RANKING_AXIS_OPTIONS } from "@/lib/score-ranking-config";
import { cn } from "@/lib/utils";

/**
 * 「살 때 본 것」 — 보유 종목의 매수 이유 축 (2026-09-24 사용자 결정).
 *
 * 고르면 보유 이유 점검이 종합 점수 대신 **그 축들의 평균 변화**로 판단한다. 셋까지 —
 * 다섯을 다 고르면 그건 이유가 아니라 종합 점수다. 누를 때마다 바로 저장한다(따로 저장
 * 단추를 두면 모달을 닫으며 잃는다).
 */
const AXES = SCORE_RANKING_AXIS_OPTIONS.filter((o) => o.id !== "composite");
const MAX = 3;

export function HoldingThesisPicker({
  symbol,
  initial,
  onSaved,
}: {
  symbol: string;
  initial: string[];
  onSaved?: (axes: string[]) => void;
}) {
  const { session } = useAuth();
  const [axes, setAxes] = useState<string[]>(initial);
  const [note, setNote] = useState<string | null>(null);

  async function toggle(id: string) {
    if (!session) {
      setNote("로그인하면 매수 이유가 저장됩니다");
      return;
    }
    const next = axes.includes(id) ? axes.filter((a) => a !== id) : [...axes, id];
    if (next.length > MAX) {
      setNote(`셋까지 고를 수 있습니다`);
      return;
    }
    const prev = axes;
    setAxes(next);
    setNote(null);
    try {
      await apiRequest(`/api/v1/user/holdings/${encodeURIComponent(symbol)}/thesis`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ axes: next }),
      });
      onSaved?.(next);
    } catch (e) {
      setAxes(prev);
      setNote(e instanceof Error ? e.message : "저장하지 못했습니다");
    }
  }

  return (
    <section className="mb-3">
      <p className="mb-1 text-cell font-semibold">살 때 본 것</p>
      <div className="flex flex-wrap gap-1.5">
        {AXES.map((a) => {
          const on = axes.includes(a.id);
          return (
            <button
              key={a.id}
              type="button"
              onClick={() => void toggle(a.id)}
              aria-pressed={on}
              className={cn(
                "rounded-full border px-2.5 py-0.5 text-cell transition-colors",
                on
                  ? "border-[var(--brand)] bg-[var(--brand)]/10 font-semibold text-[var(--brand)]"
                  : "border-[var(--border)] text-muted-foreground hover:text-foreground",
              )}
            >
              {a.label}
            </button>
          );
        })}
      </div>
      <p className="mt-1 text-micro text-muted-foreground">
        {note ??
          (axes.length > 0
            ? "보유 이유 점검이 종합 점수 대신 고른 축의 변화로 판단합니다."
            : "고르면 보유 이유 점검이 그 축을 추적합니다(셋까지).")}
      </p>
    </section>
  );
}
