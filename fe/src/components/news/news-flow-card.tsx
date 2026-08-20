"use client";

import type { NewsFlowBucket } from "@/lib/news-derive";
import { cn } from "@/lib/utils";

/**
 * A2 — 「뉴스 흐름」 24시간 막대.
 *
 * 168건이 한 줄로 흐르면 "언제 무슨 일이 터졌는지"가 사라진다. 시간대별 기사량을 세면
 * 이슈가 발생한 시각이 봉우리로 보이고, 막대를 눌러 그 시각의 기사만 남길 수 있다.
 *
 * 추가 요청은 없다 — 이미 받은 기사의 발행 시각만 센다.
 */
export function NewsFlowCard({
  buckets,
  activeHour,
  onSelectHour,
}: {
  buckets: NewsFlowBucket[];
  activeHour: number | null;
  onSelectHour: (startMs: number | null) => void;
}) {
  const total = buckets.reduce((sum, b) => sum + b.count, 0);
  const breaking = buckets.reduce((sum, b) => sum + b.breaking, 0);
  const max = Math.max(...buckets.map((b) => b.count), 1);
  if (total === 0) return null;

  // 마지막 칸은 지금 채워지는 중이라 낮게 나오는 것이 정상이다.
  const lastIndex = buckets.length - 1;

  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3">
      <div className="mb-2 flex items-center justify-between gap-2">
        <span className="text-xs font-semibold text-[var(--text-primary)]">
          뉴스 흐름 · 최근 24시간
        </span>
        <span className="text-[11px] text-[var(--text-muted)]">
          {total.toLocaleString("ko-KR")}건
          {breaking > 0 && ` · 속보 ${breaking}건`}
          {activeHour !== null && (
            <button
              type="button"
              onClick={() => onSelectHour(null)}
              className="ml-2 rounded-full bg-[var(--brand-muted)] px-2 py-0.5 font-medium text-[var(--brand)]"
            >
              시간 필터 해제
            </button>
          )}
        </span>
      </div>

      <div className="flex h-[54px] items-end gap-[3px]">
        {buckets.map((bucket, i) => {
          const ms = bucket.start.getTime();
          const active = activeHour === ms;
          const label = `${bucket.start.getHours()}시 · ${bucket.count}건${
            bucket.breaking > 0 ? ` (속보 ${bucket.breaking})` : ""
          }`;
          return (
            <button
              key={ms}
              type="button"
              title={label}
              aria-label={label}
              aria-pressed={active}
              onClick={() => onSelectHour(active ? null : ms)}
              className={cn(
                "min-h-[2px] flex-1 rounded-t-sm transition-colors",
                active
                  ? "bg-[var(--brand)] ring-1 ring-[var(--brand)]"
                  : bucket.breaking > 0
                    ? "bg-[var(--loss)]/70 hover:bg-[var(--loss)]"
                    : i === lastIndex
                      ? "bg-[var(--brand)]/70 hover:bg-[var(--brand)]"
                      : "bg-[var(--brand)]/35 hover:bg-[var(--brand)]/60",
              )}
              style={{ height: `${Math.max(3, (bucket.count / max) * 100)}%` }}
            />
          );
        })}
      </div>

      <div className="mt-1 flex justify-between text-[10px] text-[var(--text-muted)]">
        <span>{buckets[0].start.getHours()}시</span>
        <span>{buckets[Math.floor(buckets.length / 2)].start.getHours()}시</span>
        <span>지금 {buckets[lastIndex].start.getHours()}시</span>
      </div>
    </div>
  );
}
