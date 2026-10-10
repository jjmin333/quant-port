"use client";

import { Sparkles } from "lucide-react";

import { formatDateTimeKST } from "@/lib/format";
import type { FinBrief } from "@/lib/fin-products-api";
import { cn } from "@/lib/utils";

/**
 * 금융상품 분석 화면 맨 위 (2026-10-09 사용자 결정 A — 「비교」가 아니라 정보 + AI 분석).
 *
 * AI 글은 worker-asset의 asset_ai 배치(하루 한 번, 공시·기사 제목이 같으면 다시 부르지 않음)가 쓴 것을 읽기만 한다.
 * 아직 없으면 시장 지표 칸만 선다. 지표는 화면이 이미 받은 공시 목록에서 바로 센다(추가 요청 0).
 */
export function FinMarketCard({
  title,
  brief,
  stats,
}: {
  title: string;
  brief?: FinBrief;
  stats: { label: string; value: string; sub?: string }[];
}) {
  const b = brief?.body;
  return (
    <section aria-label={title} className="rounded-xl border bg-card p-3 sm:p-4">
      {b?.headline ? (
        <div className="mb-3 rounded-lg bg-[linear-gradient(180deg,var(--brand-muted),transparent_80%)] p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="inline-flex items-center gap-1 rounded-md bg-[var(--brand)]/10 px-1.5 py-0.5 text-micro font-bold text-[var(--brand)]">
              <Sparkles className="h-3 w-3" aria-hidden />
              {title}
            </span>
            <span className="text-micro text-[var(--text-muted)]">
              {brief?.updatedAt ? formatDateTimeKST(brief.updatedAt) : ""} · 하루 1회 · 투자 권유 아님
            </span>
          </div>
          <p className="mt-1.5 text-sm font-bold leading-snug text-[var(--text-primary)]">{b.headline}</p>
          {(b.body ?? []).map((para, i) => (
            <p key={i} className="mt-1 text-cell leading-relaxed text-[var(--text-secondary)]">
              {para}
            </p>
          ))}
          {(b.watch ?? []).length > 0 && (
            <p className="mt-1.5 text-cell text-[var(--text-secondary)]">
              <b className="text-[var(--text-primary)]">지켜볼 것</b> · {(b.watch ?? []).join(" · ")}
            </p>
          )}
        </div>
      ) : null}
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {stats.map((s) => (
          <div key={s.label} className="min-w-0 rounded-lg border px-3 py-2">
            <p className="truncate text-micro text-[var(--text-muted)]">{s.label}</p>
            <p className="truncate font-num text-base font-bold tabular-nums text-[var(--text-primary)]">{s.value}</p>
            {s.sub ? <p className="truncate text-micro text-[var(--text-muted)]">{s.sub}</p> : null}
          </div>
        ))}
      </div>
    </section>
  );
}

/** 시장 내 위치 막대 — 「상위 8%」. 왼쪽이 좋은 쪽(점이 왼쪽일수록 좋다). */
export function MarketPosition({ topPct, className }: { topPct: number; className?: string }) {
  const good = topPct <= 20;
  return (
    <span className={cn("inline-flex items-center gap-1.5 text-micro text-[var(--text-muted)]", className)}>
      <span className="relative h-1.5 w-16 rounded-full bg-muted" aria-hidden>
        <span
          className={cn("absolute top-1/2 h-2.5 w-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full", good ? "bg-[var(--brand)]" : "bg-[var(--text-muted)]")}
          style={{ left: `${Math.min(100, Math.max(0, topPct))}%` }}
        />
      </span>
      <span className={cn(good && "font-semibold text-[var(--brand)]")}>시장 상위 {topPct}%</span>
    </span>
  );
}
