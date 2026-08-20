"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Loader2, RefreshCw } from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import { formatDateTimeKST } from "@/lib/format";
import {
  API_QUOTA_CATEGORY_LABELS,
  fetchAdminApiQuotas,
  type ApiQuotaCategory,
  type ApiQuotaUsage,
} from "@/lib/admin";
import { cn } from "@/lib/utils";

function formatDateTime(value: string | null | undefined): string {
  if (!value) return "-";
  return formatDateTimeKST(value);
}

function formatInterval(ms: number | undefined): string | null {
  if (!ms) return null;
  if (ms < 1000) return `${ms}ms 간격`;
  const seconds = ms / 1000;
  if (seconds < 60) return `${seconds}초 간격`;
  const minutes = seconds / 60;
  if (minutes < 60) return `${Number.isInteger(minutes) ? minutes : minutes.toFixed(1)}분 간격`;
  const hours = minutes / 60;
  return `${Number.isInteger(hours) ? hours : hours.toFixed(1)}시간 간격`;
}

function isBlocked(q: ApiQuotaUsage): boolean {
  if (!q.blockedUntil) return false;
  return new Date(q.blockedUntil).getTime() > Date.now();
}

function usageRatio(count: number, limit: number | undefined): number | null {
  if (!limit || limit <= 0) return null;
  return Math.min(1, count / limit);
}

function barColor(ratio: number | null, blocked: boolean): string {
  if (blocked) return "bg-red-500";
  if (ratio == null) return "bg-[var(--brand)]/40";
  if (ratio >= 1) return "bg-red-500";
  if (ratio >= 0.8) return "bg-amber-500";
  return "bg-[var(--brand)]";
}

function UsageRow({
  label,
  count,
  limit,
  blocked,
}: {
  label: string;
  count: number;
  limit: number | undefined;
  blocked: boolean;
}) {
  const ratio = usageRatio(count, limit);
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between text-[11px] text-[var(--text-muted)]">
        <span>{label}</span>
        <span className="tabular-nums text-[var(--text-secondary)]">
          {count.toLocaleString()}
          {limit ? ` / ${limit.toLocaleString()}` : ""}
        </span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
        <div
          className={cn("h-full rounded-full transition-all", barColor(ratio, blocked))}
          style={{ width: `${ratio == null ? (count > 0 ? 100 : 0) : ratio * 100}%` }}
        />
      </div>
    </div>
  );
}

const CATEGORY_ORDER: ApiQuotaCategory[] = ["market", "corp", "news", "translation", ""];

export function AdminApiPanel() {
  const router = useRouter();
  const { session, loading } = useAuth();

  const [quotas, setQuotas] = useState<ApiQuotaUsage[]>([]);
  const [category, setCategory] = useState<ApiQuotaCategory | "all">("all");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loadedAt, setLoadedAt] = useState<Date | null>(null);

  const load = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const data = await fetchAdminApiQuotas();
      setQuotas(data);
      setLoadedAt(new Date());
    } catch (e) {
      setError(e instanceof Error ? e.message : "불러오기 실패");
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!isAdminSession(session)) {
      router.replace("/");
      return;
    }
    void load();
  }, [loading, session, router, load]);

  const filtered = useMemo(
    () => (category === "all" ? quotas : quotas.filter((q) => q.category === category)),
    [quotas, category],
  );

  const grouped = useMemo(() => {
    const map = new Map<ApiQuotaCategory, ApiQuotaUsage[]>();
    for (const q of filtered) {
      const list = map.get(q.category) ?? [];
      list.push(q);
      map.set(q.category, list);
    }
    return CATEGORY_ORDER.filter((c) => map.has(c)).map((c) => [c, map.get(c)!] as const);
  }, [filtered]);

  const summary = useMemo(() => {
    const blockedCount = quotas.filter(isBlocked).length;
    const todayCalls = quotas.reduce((sum, q) => sum + q.dayCount, 0);
    const categories = new Set(quotas.map((q) => q.category)).size;
    return { total: quotas.length, blockedCount, todayCalls, categories };
  }, [quotas]);

  if (loading || !isAdminSession(session)) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-[var(--text-muted)]">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        확인 중…
      </div>
    );
  }

  return (
    <>
      <PageHeader title="API 관리" subtitle="외부 API 호출 용도 · 호출량 · 할당량 (관리자 전용)" />
      <PageScrollShell bare>
        <div className="page-shell-wide min-w-0 space-y-4 py-4">
          <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
            <Link
              href="/admin"
              className="rounded-md border border-[var(--border)] px-2.5 py-1.5 text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
            >
              ← 관리자 홈
            </Link>
            <div className="flex items-center gap-2">
              {loadedAt && (
                <span className="text-[10px] text-[var(--text-muted)]">
                  마지막 새로고침 {formatDateTime(loadedAt.toISOString())}
                </span>
              )}
              <button
                type="button"
                onClick={() => void load()}
                disabled={busy}
                className="inline-flex items-center gap-1 rounded-md border border-[var(--border)] px-2.5 py-1.5 text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] disabled:opacity-50"
              >
                <RefreshCw className={cn("h-3.5 w-3.5", busy && "animate-spin")} />
                새로고침
              </button>
            </div>
          </div>

          {error && (
            <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">
              {error}
            </div>
          )}

          <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard label="연동 API" value={summary.total} sub={`${summary.categories}개 카테고리`} />
            <StatCard
              label="차단 중"
              value={summary.blockedCount}
              sub={summary.blockedCount > 0 ? "쿼터 초과 · 대기 중" : "정상"}
              tone={summary.blockedCount > 0 ? "danger" : undefined}
            />
            <StatCard label="오늘 호출량 합계" value={summary.todayCalls.toLocaleString()} sub="전체 provider" />
            <StatCard
              label="분류"
              value={CATEGORY_ORDER.filter((c) => c && quotas.some((q) => q.category === c)).length}
              sub="시장 · 공시 · 뉴스 · 번역"
            />
          </section>

          <div className="flex flex-wrap gap-1.5 text-xs">
            <button
              type="button"
              onClick={() => setCategory("all")}
              className={cn(
                "rounded-full px-3 py-1 font-medium",
                category === "all"
                  ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                  : "border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
              )}
            >
              전체
            </button>
            {CATEGORY_ORDER.filter((c) => c).map((c) => (
              <button
                key={c}
                type="button"
                onClick={() => setCategory(c)}
                className={cn(
                  "rounded-full px-3 py-1 font-medium",
                  category === c
                    ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                    : "border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
                )}
              >
                {API_QUOTA_CATEGORY_LABELS[c] ?? c}
              </button>
            ))}
          </div>

          {quotas.length === 0 && !busy ? (
            <p className="rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-6 text-center text-sm text-[var(--text-muted)]">
              데이터가 없습니다.
            </p>
          ) : (
            <div className="space-y-5">
              {grouped.map(([cat, items]) => (
                <section key={cat || "uncategorized"}>
                  <h2 className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
                    {API_QUOTA_CATEGORY_LABELS[cat] ?? "기타"}
                  </h2>
                  <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                    {items.map((q) => {
                      const blocked = isBlocked(q);
                      const interval = formatInterval(q.limits.minIntervalMs);
                      return (
                        <div
                          key={q.provider}
                          className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3.5"
                        >
                          <div className="flex flex-wrap items-start justify-between gap-2">
                            <div>
                              <p className="text-sm font-semibold text-[var(--text-primary)]">
                                {q.label || q.provider}
                              </p>
                              <p className="font-mono text-[10px] text-[var(--text-muted)]">{q.provider}</p>
                            </div>
                            <span
                              className={cn(
                                "shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium",
                                blocked
                                  ? "bg-red-500/15 text-red-400"
                                  : "bg-emerald-500/15 text-emerald-400",
                              )}
                            >
                              {blocked ? "차단됨" : "정상"}
                            </span>
                          </div>
                          {q.purpose && (
                            <p className="mt-2 text-xs leading-relaxed text-[var(--text-secondary)]">
                              {q.purpose}
                            </p>
                          )}
                          {blocked && (
                            <p className="mt-2 rounded border border-red-500/30 bg-red-500/10 px-2 py-1 text-[10px] text-red-300">
                              {formatDateTime(q.blockedUntil)} 까지 호출 차단
                            </p>
                          )}
                          <div className="mt-3 space-y-2">
                            <UsageRow label="분당" count={q.minuteCount} limit={q.limits.perMinute} blocked={blocked} />
                            {q.limits.tokensPerMinute != null && (
                              <UsageRow
                                label="분당 토큰"
                                count={q.minuteTokens ?? 0}
                                limit={q.limits.tokensPerMinute}
                                blocked={blocked}
                              />
                            )}
                            <UsageRow label="일간" count={q.dayCount} limit={q.limits.dailyQuota} blocked={blocked} />
                            <UsageRow label="월간" count={q.monthCount} limit={q.limits.monthlyQuota} blocked={blocked} />
                          </div>
                          <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-[10px] text-[var(--text-muted)]">
                            {interval && <span>{interval}</span>}
                            <span>마지막 호출: {formatDateTime(q.lastCallAt)}</span>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </section>
              ))}
            </div>
          )}
        </div>
      </PageScrollShell>
    </>
  );
}

function StatCard({
  label,
  value,
  sub,
  tone,
}: {
  label: string;
  value: string | number;
  sub?: string;
  tone?: "danger";
}) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
      <p className="text-xs text-[var(--text-muted)]">{label}</p>
      <p
        className={cn(
          "mt-1 text-2xl font-semibold",
          tone === "danger" ? "text-red-400" : "text-[var(--text-primary)]",
        )}
      >
        {value}
      </p>
      {sub && <p className="mt-1 text-xs text-[var(--text-secondary)]">{sub}</p>}
    </div>
  );
}
