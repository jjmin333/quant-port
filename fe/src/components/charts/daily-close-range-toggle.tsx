"use client";

import type { BarStep } from "@/lib/daily-close-window";
import { cn } from "@/lib/utils";

/**
 * 일별 종가 차트의 기간 토글(20일 · 60일 · 전체).
 *
 * 어느 단계를 낼지는 `lib/daily-close-window`가 정한다 — 여기서는 그리기만 한다.
 * 단계가 하나뿐이면 **아무것도 그리지 않는다**: 누를 곳이 하나인 토글은 자리만 차지하고,
 * 「고를 수 있다」는 거짓말을 한다.
 */
export function DailyCloseRangeToggle({
  steps,
  active,
  onChange,
}: {
  steps: BarStep[];
  active: number;
  onChange: (step: number) => void;
}) {
  if (steps.length < 2) return null;
  return (
    <div className="flex shrink-0 gap-0.5 rounded-lg bg-muted p-0.5">
      {steps.map(({ step, label }) => (
        <button
          key={step}
          type="button"
          onClick={() => onChange(step)}
          aria-pressed={active === step}
          className={cn(
            "rounded-md px-2 py-0.5 text-cell font-medium transition-colors",
            active === step
              ? "bg-card text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          {label}
        </button>
      ))}
    </div>
  );
}
