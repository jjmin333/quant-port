"use client";

import { useEffect, useState } from "react";

import { fetchScreenPresetsClient, formatCount, type ScreenPreset } from "@/lib/screener-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/** 어제(앞 점수일) 대비 종목 수 — 같으면 「어제와 같음」, 없으면 그리지 않는다(2026-10-10 요청 4). */
function PresetDelta({ count, prev }: { count: number; prev?: number }) {
  if (prev == null || count < 0) return null;
  const d = count - prev;
  if (d === 0) return <span className="ml-1.5 font-normal text-[var(--text-muted)]">어제와 같음</span>;
  return (
    <span className={cn("ml-1.5 font-semibold", trendTextClass(d))} title={`어제 ${formatCount(prev)}종목`}>
      {d > 0 ? "▲" : "▼"}
      {formatCount(Math.abs(d))}
    </span>
  );
}

/**
 * 목적 카드 (2026-10-09 결정 A) — 「무엇부터 눌러야 할지」의 답. 누르면 그 조건으로 갈아 끼우고 바로 찾는다.
 * 카드마다 지금 걸리는 종목 수를 적는다(서버가 같은 점수일로 셈). 0~1종목인 카드는 흐리게 — 시즌 밖의
 * 「실적 발표 임박」처럼 눌러 봐야 빈 결과인 카드가 다른 카드와 같은 무게로 서지 않게.
 */
export function ScreenerPresets({
  market,
  activeId,
  onPick,
}: {
  market: string;
  activeId: string | null;
  onPick: (preset: ScreenPreset) => void;
}) {
  const [presets, setPresets] = useState<ScreenPreset[] | null>(null);
  useEffect(() => {
    let alive = true;
    fetchScreenPresetsClient(market)
      .then((p) => alive && setPresets(p))
      .catch(() => alive && setPresets([]));
    return () => {
      alive = false;
    };
  }, [market]);

  if (presets === null) {
    return <div className="h-[86px] animate-pulse rounded-xl bg-[var(--surface-elevated)]" aria-hidden />;
  }
  // 지금 0종목인 카드는 세우지 않는다 — 해외의 「외국인·기관 순매수」처럼 그 시장엔 값이 없는 카드도 여기서 빠진다.
  const shown = presets.filter((p) => p.count !== 0);
  if (shown.length === 0) return null;

  return (
    <div className="-mx-1 flex min-w-0 gap-2 overflow-x-auto px-1 pb-1" role="group" aria-label="자주 쓰는 조건">
      {shown.map((p) => {
        const thin = p.count >= 0 && p.count <= 1;
        return (
          <button
            key={p.id}
            type="button"
            onClick={() => onPick(p)}
            aria-pressed={activeId === p.id}
            className={cn(
              "flex w-[11.5rem] shrink-0 flex-col items-start rounded-xl border bg-[var(--card)] px-3 py-2.5 text-left transition-colors",
              activeId === p.id
                ? "border-[var(--brand)] shadow-[0_0_0_2px_var(--brand-muted)]"
                : "border-[var(--border)] hover:border-[var(--brand)]/60",
              thin && activeId !== p.id && "opacity-55",
            )}
          >
            <span className="text-sm font-bold text-[var(--text-primary)]">
              <span aria-hidden className="mr-1">
                {p.icon}
              </span>
              {p.label}
            </span>
            <span className="mt-0.5 line-clamp-1 text-micro text-[var(--text-muted)]">{p.desc}</span>
            <span className="mt-1.5 text-cell font-bold tabular-nums text-[var(--brand)]">
              {p.count < 0 ? "—" : `${formatCount(p.count)}종목`}
              {thin ? <span className="ml-1 font-normal text-[var(--text-muted)]">지금은 적음</span> : null}
              <PresetDelta count={p.count} prev={p.prevCount} />
            </span>
            {p.newNames && p.newNames.length > 0 && (
              <span className="mt-0.5 line-clamp-1 w-full text-micro text-[var(--text-secondary)]" title={`새로 걸림: ${p.newNames.join(", ")}`}>
                <b className="font-semibold text-[var(--brand)]">새로</b> {p.newNames.join(" · ")}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
