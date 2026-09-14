"use client";

import { formatPrice } from "@/lib/format";
import { STAMP_NEUTRAL_BAND_PCT, type Stamp } from "@/lib/stamp-types";
import { cn } from "@/lib/utils";

/**
 * 예상한 폭 중 얼마나 왔는가.
 *
 * 등락률만 적으면 "+9.6%"가 잘한 것인지 알 수 없다 — 목표가 +10.5%였다면 거의 다
 * 온 것이고 +30%였다면 3분의 1도 못 온 것이다. 게이지는 그 비율을 재는 자다.
 *
 * 관망 스탬프에는 그릴 것이 없다(목표까지의 거리가 0에 가깝다). 그때는 게이지 대신
 * 밴드 안에 있는지만 말한다.
 */

function toneOf(progress: number): string {
  if (progress >= 100) return "var(--profit)";
  if (progress < 0) return "var(--loss)";
  return "var(--brand)";
}

export function StampTargetGauge({
  stamp,
  /** 목표가를 이미 옆에 적어둔 자리(목록 행)에서는 끈다 — 같은 숫자를 두 번 읽히지 않게. */
  showTargetPrice = false,
  className,
}: {
  stamp: Stamp;
  showTargetPrice?: boolean;
  className?: string;
}) {
  const currency = stamp.currency === "USD" ? "USD" : "KRW";
  const targetLabel = showTargetPrice
    ? `목표 ${formatPrice(stamp.targetPrice, currency)}`
    : "목표";

  if (stamp.stance === "neutral" || stamp.targetProgress === undefined) {
    const drift = stamp.current?.changeRate;
    return (
      <div className={cn("text-micro text-[var(--text-muted)]", className)}>
        {showTargetPrice && <div>{targetLabel}</div>}
        <div>
          {drift === undefined
            ? `관망 · ±${STAMP_NEUTRAL_BAND_PCT}% 밴드로 판정`
            : `관망 · 현재 ${drift > 0 ? "+" : ""}${drift.toFixed(1)}%`}
        </div>
        <div className="mt-0.5">
          {drift !== undefined && Math.abs(drift) <= STAMP_NEUTRAL_BAND_PCT
            ? `밴드 안 (±${STAMP_NEUTRAL_BAND_PCT}%)`
            : `밴드 밖 (±${STAMP_NEUTRAL_BAND_PCT}%)`}
        </div>
      </div>
    );
  }

  // 장중에 목표를 찍은 스탬프는 그 뒤 되돌아왔어도 **도달한 것**이다. 현재가로
  // 진행률을 재면 "도달"이라고 적어 놓고 "목표의 59%"라고 말하는 모순이 된다.
  const progress = stamp.targetHit ? 100 : stamp.targetProgress;
  const tone = toneOf(progress);
  // 초과 달성도 폭이 보여야 하므로 트랙은 120%까지 그린다.
  const width = Math.max(0, Math.min(100, (Math.max(progress, 0) / 120) * 100));
  const markerAt = (100 / 120) * 100;

  return (
    <div className={cn("min-w-0", className)}>
      <div className="flex justify-between text-micro text-[var(--text-muted)]">
        <span>{stamp.targetHit ? "도달" : "현재"}</span>
        <span>{targetLabel}</span>
      </div>
      <div className="relative my-1 h-[5px] rounded-full bg-[var(--surface-elevated)]">
        <span
          className="absolute inset-y-0 left-0 rounded-full"
          style={{ width: `${width}%`, background: tone }}
        />
        {/* 목표 지점 눈금 — 채움이 여기 닿으면 도달이다. */}
        <span
          className="absolute -top-[3px] h-[11px] w-[2px] bg-[var(--text-secondary)] opacity-60"
          style={{ left: `${markerAt}%` }}
        />
      </div>
      <div className="text-micro" style={{ color: tone }}>
        {stamp.targetHit
          ? "목표가 도달"
          : progress < 0
            ? `반대로 ${Math.abs(progress).toFixed(0)}%`
            : `목표의 ${Math.round(progress)}% 도달`}
      </div>
    </div>
  );
}
