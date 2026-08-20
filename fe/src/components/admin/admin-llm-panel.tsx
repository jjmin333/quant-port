"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { CalendarClock, ChevronDown, Loader2, Play, RefreshCw, Save, Sparkles } from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import { formatDateTimeKST } from "@/lib/format";
import {
  fetchAdminApiQuotas,
  fetchAdminLLMBatchStatus,
  fetchAdminLLMBatchTargets,
  fetchAdminLLMRefreshStatus,
  fetchAdminLLMRequestLog,
  fetchAdminLLMSettings,
  fetchAdminStockSummaryStatus,
  triggerAdminLLMRefresh,
  updateAdminLLMSettings,
  LLM_BATCH_EXCLUDE_LABELS,
  LLM_BATCH_STOP_LABELS,
  LLM_MANUAL_REFRESH_MAX_SYMBOLS,
  type ApiQuotaUsage,
  type LLMBatchMarketStatus,
  type LLMBatchSelection,
  type LLMBatchStatus,
  type LLMManualRefreshJob,
  type LLMRefreshSymbolState,
  type LLMRequestLogEntry,
  type LLMSettings,
  type LLMTargetMode,
  type StockSummaryStatus,
} from "@/lib/admin";
import { cn } from "@/lib/utils";

function formatDateTime(value: string | null | undefined): string {
  if (!value) return "-";
  return formatDateTimeKST(value);
}

const MARKET_LABELS: Record<string, string> = { KR: "국내", US: "미국" };

function marketLabel(market: string): string {
  return MARKET_LABELS[market] ?? market;
}

/** -1 from the API means "provider ceiling unset", not "nothing left". */
function formatAllowance(value: number | undefined): string {
  if (value == null) return "-";
  if (value < 0) return "제한 없음";
  return value.toLocaleString();
}

const REFRESH_STATUS_LABELS: Record<LLMRefreshSymbolState["status"], string> = {
  pending: "대기",
  running: "생성 중",
  done: "완료",
  error: "실패",
};

function RefreshStateRow({ state }: { state: LLMRefreshSymbolState }) {
  return (
    <div className="flex items-center justify-between gap-2 rounded-lg border border-[var(--border)] px-2.5 py-2 text-xs">
      <span className="flex items-center gap-2">
        {state.status === "running" && <Loader2 className="h-3.5 w-3.5 animate-spin text-[var(--brand)]" />}
        <span className="font-mono font-medium text-[var(--text-primary)]">{state.symbol}</span>
      </span>
      <span className="flex items-center gap-2 text-right text-[10px] text-[var(--text-muted)]">
        {state.status === "done" && (
          <>
            <span>{state.source === "llm" ? "AI 생성" : "규칙 기반 폴백"}</span>
            <span>{formatDateTime(state.generatedAt)}</span>
          </>
        )}
        {state.status === "error" && <span className="text-red-400">{state.error}</span>}
        <span
          className={cn(
            "shrink-0 rounded-full px-2 py-0.5 font-medium",
            state.status === "done" && "bg-emerald-500/15 text-emerald-400",
            state.status === "error" && "bg-red-500/15 text-red-400",
            state.status === "running" && "bg-[var(--brand-muted)] text-[var(--brand)]",
            state.status === "pending" && "bg-[var(--surface-elevated)] text-[var(--text-muted)]",
          )}
        >
          {REFRESH_STATUS_LABELS[state.status]}
        </span>
      </span>
    </div>
  );
}

function usageRatio(count: number, limit: number | undefined): number | null {
  if (!limit || limit <= 0) return null;
  return Math.min(1, count / limit);
}

function barColor(ratio: number | null): string {
  if (ratio == null) return "bg-[var(--brand)]/40";
  if (ratio >= 1) return "bg-red-500";
  if (ratio >= 0.8) return "bg-amber-500";
  return "bg-[var(--brand)]";
}

function QuotaBar({ label, count, limit }: { label: string; count: number; limit: number | undefined }) {
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
          className={cn("h-full rounded-full transition-all", barColor(ratio))}
          style={{ width: `${ratio == null ? (count > 0 ? 100 : 0) : ratio * 100}%` }}
        />
      </div>
    </div>
  );
}

/** One market's schedule, last run, and exclusion tally. */
function BatchMarketCard({
  status,
  targets,
}: {
  status: LLMBatchMarketStatus;
  targets: LLMBatchSelection | null;
}) {
  const run = status.lastRun;
  const excluded = targets?.excluded ?? run?.excluded ?? {};
  const excludedEntries = Object.entries(excluded).filter(([, count]) => count > 0);

  return (
    <div className="rounded-lg border border-[var(--border)] p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-xs font-semibold text-[var(--text-primary)]">
          {marketLabel(status.market)} 배치
        </h3>
        {run?.running && (
          <span className="inline-flex items-center gap-1 rounded-full bg-[var(--brand-muted)] px-2 py-0.5 text-[10px] text-[var(--brand)]">
            <Loader2 className="h-3 w-3 animate-spin" />
            실행 중
          </span>
        )}
      </div>

      <dl className="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-[11px]">
        <dt className="text-[var(--text-muted)]">다음 실행</dt>
        <dd className="text-right tabular-nums text-[var(--text-secondary)]">
          {formatDateTime(status.nextRunAt)}
        </dd>
        <dt className="text-[var(--text-muted)]">개장 전 마감</dt>
        <dd className="text-right tabular-nums text-[var(--text-secondary)]">
          {formatDateTime(status.nextDeadlineAt)}
        </dd>
        {targets && (
          <>
            <dt className="text-[var(--text-muted)]">현재 대상</dt>
            <dd className="text-right tabular-nums text-[var(--text-secondary)]">
              {(targets.targets?.length ?? 0).toLocaleString()}종목
              <span className="text-[var(--text-muted)]"> / 순위 {targets.considered}</span>
            </dd>
          </>
        )}
      </dl>

      {excludedEntries.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          {excludedEntries.map(([reason, count]) => (
            <span
              key={reason}
              className="rounded-full bg-[var(--surface-elevated)] px-2 py-0.5 text-[10px] text-[var(--text-muted)]"
            >
              {LLM_BATCH_EXCLUDE_LABELS[reason] ?? reason} {count}
            </span>
          ))}
        </div>
      )}

      {run ? (
        <div className="mt-2.5 border-t border-[var(--border)] pt-2 text-[11px]">
          <p className="text-[var(--text-muted)]">
            최근 실행 {run.tradeDate} · {formatDateTime(run.startedAt)}
          </p>
          <p className="mt-1 text-[var(--text-secondary)]">
            <span className="text-emerald-400">{run.succeeded} 성공</span>
            {run.failed > 0 && <span className="text-red-400"> · {run.failed} 실패</span>}
            <span className="text-[var(--text-muted)]">
              {" "}
              · {run.tokensUsed.toLocaleString()} 토큰
            </span>
          </p>
          {run.stopReason && run.stopReason !== "completed" && (
            <p className="mt-1 text-amber-400">
              중단: {LLM_BATCH_STOP_LABELS[run.stopReason] ?? run.stopReason}
              {run.carryover && run.carryover.length > 0 && ` · 다음 실행으로 ${run.carryover.length}종목 이월`}
            </p>
          )}
        </div>
      ) : (
        <p className="mt-2.5 border-t border-[var(--border)] pt-2 text-[11px] text-[var(--text-muted)]">
          아직 실행 기록이 없습니다.
        </p>
      )}
    </div>
  );
}

function RequestLogRow({ entry }: { entry: LLMRequestLogEntry }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="rounded-lg border border-[var(--border)]">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center justify-between gap-2 px-2.5 py-2 text-left text-xs hover:bg-[var(--surface-elevated)]"
      >
        <span className="flex items-center gap-2">
          <ChevronDown className={cn("h-3.5 w-3.5 shrink-0 transition-transform", open && "rotate-180")} />
          <span className="font-medium text-[var(--text-primary)]">{entry.name || entry.symbol}</span>
          <span className="font-mono text-[10px] text-[var(--text-muted)]">{entry.symbol}</span>
          <span className="text-[10px] text-[var(--text-muted)]">{formatDateTime(entry.timestamp)}</span>
          {entry.manual && (
            <span className="rounded-full bg-[var(--brand-muted)] px-1.5 py-0.5 text-[10px] text-[var(--brand)]">
              수동
            </span>
          )}
        </span>
        <span
          className={cn(
            "shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium",
            entry.success ? "bg-emerald-500/15 text-emerald-400" : "bg-red-500/15 text-red-400",
          )}
        >
          {entry.success ? "성공" : "실패"}
        </span>
      </button>
      {open && (
        <div className="border-t border-[var(--border)] px-2.5 py-2">
          {!entry.success && entry.error && (
            <p className="mb-2 text-[11px] text-red-400">{entry.error}</p>
          )}
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all rounded-md bg-[var(--background)] p-2 text-[10px] text-[var(--text-secondary)]">
            {(() => {
              try {
                return JSON.stringify(JSON.parse(entry.factsJson), null, 2);
              } catch {
                return entry.factsJson;
              }
            })()}
          </pre>
        </div>
      )}
    </div>
  );
}

export function AdminLLMPanel() {
  const router = useRouter();
  const { session, loading } = useAuth();

  const [status, setStatus] = useState<StockSummaryStatus | null>(null);
  const [quota, setQuota] = useState<ApiQuotaUsage | null>(null);
  const [requestLog, setRequestLog] = useState<LLMRequestLogEntry[]>([]);
  const [logTotal, setLogTotal] = useState(0);
  const [logPage, setLogPage] = useState(1);
  const [logStatusFilter, setLogStatusFilter] = useState<"all" | "failed">("all");
  const [logBusy, setLogBusy] = useState(false);
  const logPageSize = 30;
  const [nameFilter, setNameFilter] = useState("all");
  const [symbolsDraft, setSymbolsDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saveMessage, setSaveMessage] = useState<string | null>(null);
  const [loadedAt, setLoadedAt] = useState<Date | null>(null);

  const [manualDraft, setManualDraft] = useState("");
  const [manualJob, setManualJob] = useState<LLMManualRefreshJob | null>(null);
  const [manualError, setManualError] = useState<string | null>(null);
  const [triggering, setTriggering] = useState(false);

  const [settings, setSettings] = useState<LLMSettings | null>(null);
  const [batch, setBatch] = useState<LLMBatchStatus | null>(null);
  const [targets, setTargets] = useState<Record<string, LLMBatchSelection | null>>({});

  const load = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const [settingsData, statusData, quotas, job, batchData, krTargets, usTargets] =
        await Promise.all([
          fetchAdminLLMSettings(),
          fetchAdminStockSummaryStatus(),
          fetchAdminApiQuotas(),
          fetchAdminLLMRefreshStatus(),
          fetchAdminLLMBatchStatus(),
          fetchAdminLLMBatchTargets("KR"),
          fetchAdminLLMBatchTargets("US"),
        ]);
      setStatus(statusData);
      setSettings(settingsData);
      setSymbolsDraft((settingsData.pilotSymbols ?? []).join(", "));
      setQuota(quotas.find((q) => q.provider === "openai") ?? null);
      setManualJob(job && job.symbols?.length ? job : null);
      setBatch(batchData);
      setTargets({ KR: krTargets, US: usTargets });
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

  const loadLog = useCallback(async () => {
    if (loading || !isAdminSession(session)) return;
    setLogBusy(true);
    try {
      const logData = await fetchAdminLLMRequestLog({
        status: logStatusFilter,
        page: logPage,
        pageSize: logPageSize,
      });
      setRequestLog(logData.entries);
      setLogTotal(logData.total);
    } catch (e) {
      setError(e instanceof Error ? e.message : "요청 기록을 불러오지 못했습니다");
    } finally {
      setLogBusy(false);
    }
  }, [loading, session, logStatusFilter, logPage]);

  useEffect(() => {
    void loadLog();
  }, [loadLog]);

  // Filter changes should reset back to page 1, not stay on a page that may
  // no longer exist under the new filter.
  useEffect(() => {
    setLogPage(1);
  }, [logStatusFilter]);

  // Poll while a manual run is in flight; refresh the surrounding panels
  // (cache status, quota, request log) once it finishes.
  const manualRunning = manualJob?.running ?? false;

  useEffect(() => {
    if (!manualRunning) return;
    let cancelled = false;
    const timer = setInterval(() => {
      void (async () => {
        try {
          const job = await fetchAdminLLMRefreshStatus();
          if (cancelled || !job) return;
          setManualJob(job);
          if (!job.running) {
            void load();
            void loadLog();
          }
        } catch {
          // Transient poll failure: keep the last known job state and retry.
        }
      })();
    }, 2000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [manualRunning, load, loadLog]);

  const requestNames = useMemo(() => {
    const names = new Set(requestLog.map((e) => e.name || e.symbol));
    return Array.from(names).sort();
  }, [requestLog]);

  const filteredRequestLog = useMemo(() => {
    if (nameFilter === "all") return requestLog;
    return requestLog.filter((e) => (e.name || e.symbol) === nameFilter);
  }, [requestLog, nameFilter]);

  async function handleSave(overrides?: { targetMode?: LLMTargetMode; dailyCap?: number }) {
    setSaving(true);
    setError(null);
    setSaveMessage(null);
    try {
      const symbols = symbolsDraft
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
      const updated = await updateAdminLLMSettings({ pilotSymbols: symbols, ...overrides });
      setSettings(updated);
      setSymbolsDraft((updated.pilotSymbols ?? []).join(", "));
      setSaveMessage("저장했습니다. 즉시 적용됩니다(재기동 불필요).");
      const [statusData, batchData] = await Promise.all([
        fetchAdminStockSummaryStatus(),
        fetchAdminLLMBatchStatus(),
      ]);
      setStatus(statusData);
      setBatch(batchData);
    } catch (e) {
      setError(e instanceof Error ? e.message : "저장 실패");
    } finally {
      setSaving(false);
    }
  }

  async function handleManualRefresh() {
    setTriggering(true);
    setManualError(null);
    try {
      const symbols = manualDraft
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
      const job = await triggerAdminLLMRefresh(symbols);
      setManualJob(job);
    } catch (e) {
      setManualError(e instanceof Error ? e.message : "요청 실패");
    } finally {
      setTriggering(false);
    }
  }

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
      <PageHeader
        title="LLM 모델 관리"
        subtitle="AI 요약(밸류에이션·기술적분석·내부자·뉴스) 자동 배치 · 고정 종목 · 수동 즉시 생성 · 한도 · 요청 기록 (관리자 전용)"
      />
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

          <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="flex items-center gap-1.5 text-sm font-semibold text-[var(--text-primary)]">
                <Sparkles className="h-4 w-4 text-[var(--brand)]" />
                LLM Provider
              </h2>
              {status && (
                <span
                  className={cn(
                    "rounded-full px-2 py-0.5 text-[10px] font-medium",
                    status.enabled
                      ? "bg-emerald-500/15 text-emerald-400"
                      : "bg-[var(--surface-elevated)] text-[var(--text-muted)]",
                  )}
                >
                  {status.enabled ? `활성 · ${status.provider || "provider 미상"}` : "비활성 — 규칙 기반 폴백만 사용"}
                </span>
              )}
            </div>
            {quota ? (
              <div className="mt-3 space-y-2">
                <QuotaBar label="분당 요청 (RPM)" count={quota.minuteCount} limit={quota.limits.perMinute} />
                <QuotaBar label="분당 토큰 (TPM)" count={quota.minuteTokens ?? 0} limit={quota.limits.tokensPerMinute} />
                <QuotaBar label="일간 요청 (RPD)" count={quota.dayCount} limit={quota.limits.dailyQuota} />
                <QuotaBar label="일간 토큰 (TPD)" count={quota.dayTokens ?? 0} limit={quota.limits.tokensPerDay} />
                <QuotaBar label="월간 요청" count={quota.monthCount} limit={quota.limits.monthlyQuota} />
                <p className="text-[10px] text-[var(--text-muted)]">
                  마지막 호출: {formatDateTime(quota.lastCallAt)} · 상세 provider 목록은{" "}
                  <Link href="/admin/api" className="text-[var(--brand)] hover:underline">
                    API 관리
                  </Link>
                  에서 확인하세요.
                </p>
              </div>
            ) : (
              <p className="mt-2 text-xs text-[var(--text-muted)]">API 한도 정보를 불러오지 못했습니다.</p>
            )}
          </section>

          <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="flex items-center gap-1.5 text-sm font-semibold text-[var(--text-primary)]">
                <CalendarClock className="h-4 w-4 text-[var(--brand)]" />
                자동 배치 (마켓 레이더 기준)
              </h2>
              {batch && (
                <span className="rounded-full bg-[var(--surface-elevated)] px-2 py-0.5 text-[10px] text-[var(--text-muted)]">
                  {batch.targetMode === "radar" ? "레이더 상위 종목" : "고정 종목만"} · 상한 {batch.dailyCap}
                </span>
              )}
            </div>
            <p className="mt-1 text-xs leading-relaxed text-[var(--text-muted)]">
              마켓 레이더 <strong className="text-[var(--text-secondary)]">1일 기준 거래대금·거래량 상위 100</strong>{" "}
              종목에서 ETF·ETN·ETP를 제외한 개별 종목을 시장별로 하루 한 번 분석합니다. 각 시장의{" "}
              <strong className="text-[var(--text-secondary)]">정규장 개장 전</strong>에 끝나도록 역산해 시작하며,
              휴장일에는 실행하지 않습니다. 남은 일일 요청·토큰 한도를 먼저 확인해 대상 수를 자르고, 못 돌린
              종목은 다음 실행으로 이월합니다.
            </p>
            {batch && (
              <div className="mt-3 grid gap-2 text-[11px] sm:grid-cols-3">
                <div className="rounded-lg border border-[var(--border)] px-2.5 py-2">
                  <p className="text-[var(--text-muted)]">오늘 남은 요청</p>
                  <p className="tabular-nums text-[var(--text-primary)]">
                    {formatAllowance(batch.callsRemaining)}
                  </p>
                </div>
                <div className="rounded-lg border border-[var(--border)] px-2.5 py-2">
                  <p className="text-[var(--text-muted)]">오늘 남은 토큰</p>
                  <p className="tabular-nums text-[var(--text-primary)]">
                    {formatAllowance(batch.tokensRemaining)}
                  </p>
                </div>
                <div className="rounded-lg border border-[var(--border)] px-2.5 py-2">
                  <p className="text-[var(--text-muted)]">호출당 평균 토큰</p>
                  <p className="tabular-nums text-[var(--text-primary)]">
                    {batch.avgTokensPerCall.toLocaleString()}
                  </p>
                </div>
              </div>
            )}
            {batch && batch.markets.length > 0 && (
              <div className="mt-2 grid gap-2 sm:grid-cols-2">
                {batch.markets.map((m) => (
                  <BatchMarketCard key={m.market} status={m} targets={targets[m.market] ?? null} />
                ))}
              </div>
            )}
            {settings && (
              <div className="mt-3 flex flex-wrap items-end gap-2">
                <label className="flex flex-col gap-1 text-[11px] text-[var(--text-muted)]">
                  대상 선정
                  <select
                    value={settings.targetMode}
                    onChange={(e) => void handleSave({ targetMode: e.target.value as LLMTargetMode })}
                    disabled={saving}
                    className="rounded-md border border-[var(--border)] bg-[var(--background)] px-2 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-[var(--brand)] disabled:opacity-50"
                  >
                    <option value="radar">레이더 상위 + 고정 종목</option>
                    <option value="manual">고정 종목만</option>
                  </select>
                </label>
                <label className="flex flex-col gap-1 text-[11px] text-[var(--text-muted)]">
                  1회 최대 종목 수
                  <input
                    type="number"
                    min={1}
                    max={1000}
                    defaultValue={settings.dailyCap}
                    onBlur={(e) => {
                      const next = Number(e.target.value);
                      if (Number.isFinite(next) && next !== settings.dailyCap) {
                        void handleSave({ dailyCap: next });
                      }
                    }}
                    disabled={saving}
                    className="w-28 rounded-md border border-[var(--border)] bg-[var(--background)] px-2 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-[var(--brand)] disabled:opacity-50"
                  />
                </label>
                <p className="text-[10px] text-[var(--text-muted)]">
                  예비 한도: 요청 {settings.quotaReserve} · 토큰 {settings.tokenReserve.toLocaleString()} (번역·브리핑·수동
                  생성용으로 남겨둡니다)
                </p>
              </div>
            )}
          </section>

          <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
            <h2 className="text-sm font-semibold text-[var(--text-primary)]">고정 종목</h2>
            <p className="mt-1 text-xs leading-relaxed text-[var(--text-muted)]">
              레이더 순위에 없어도{" "}
              <strong className="text-[var(--text-secondary)]">항상 분석 대상에 포함</strong>되는 종목입니다. 배치
              목록의 맨 앞에 놓이므로 한도가 모자라 잘릴 때도 살아남습니다. 심볼을 쉼표로 구분해 입력하세요
              (예: 005930, 000660). 최대 50개, 자동으로 대문자 정규화·중복 제거됩니다.
            </p>
            <div className="mt-3 flex flex-col gap-2">
              <input
                type="text"
                value={symbolsDraft}
                onChange={(e) => setSymbolsDraft(e.target.value)}
                placeholder="005930, 000660"
                className="rounded-md border border-[var(--border)] bg-[var(--background)] px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
              />
              <button
                type="button"
                onClick={() => void handleSave()}
                disabled={saving}
                className="mt-1 inline-flex w-fit items-center justify-center gap-1.5 rounded-md bg-[var(--brand)] px-3 py-2 text-sm font-medium text-[var(--brand-foreground)] disabled:opacity-50"
              >
                {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                저장
              </button>
            </div>
            {saveMessage && <p className="mt-2 text-xs text-emerald-400">{saveMessage}</p>}
          </section>

          <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
            <h2 className="flex items-center gap-1.5 text-sm font-semibold text-[var(--text-primary)]">
              <Play className="h-4 w-4 text-[var(--brand)]" />
              수동 즉시 생성
            </h2>
            <p className="mt-1 text-xs leading-relaxed text-[var(--text-muted)]">
              자동 배치와 무관하게 지금 바로 AI 요약을 다시 만듭니다.{" "}
              <strong className="text-[var(--text-secondary)]">배치 대상이 아니어도 됩니다</strong> — 여기 입력한
              종목은 레이더 순위·고정 목록과 상관없이 LLM을 호출합니다. 한 번에 최대{" "}
              {LLM_MANUAL_REFRESH_MAX_SYMBOLS}개, API 한도 보호를 위해 종목당 6초 간격으로 순차 실행됩니다(위
              RPM/TPM/TPD 한도는 그대로 적용).
            </p>
            <div className="mt-3 flex flex-col gap-2 sm:flex-row">
              <input
                type="text"
                value={manualDraft}
                onChange={(e) => setManualDraft(e.target.value)}
                placeholder="005930, 000660, AAPL"
                className="flex-1 rounded-md border border-[var(--border)] bg-[var(--background)] px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
              />
              <button
                type="button"
                onClick={() => void handleManualRefresh()}
                disabled={triggering || manualRunning || manualDraft.trim() === ""}
                className="inline-flex w-fit shrink-0 items-center justify-center gap-1.5 rounded-md bg-[var(--brand)] px-3 py-2 text-sm font-medium text-[var(--brand-foreground)] disabled:opacity-50"
              >
                {triggering || manualRunning ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Play className="h-4 w-4" />
                )}
                {manualRunning ? "생성 중…" : "즉시 생성"}
              </button>
            </div>
            {manualError && <p className="mt-2 text-xs text-red-400">{manualError}</p>}
            {manualJob && (
              <div className="mt-3 space-y-1.5">
                <p className="text-[10px] text-[var(--text-muted)]">
                  시작 {formatDateTime(manualJob.startedAt)}
                  {manualJob.finishedAt && ` · 완료 ${formatDateTime(manualJob.finishedAt)}`}
                </p>
                {manualJob.symbols.map((state) => (
                  <RefreshStateRow key={state.symbol} state={state} />
                ))}
              </div>
            )}
          </section>

          <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
            <h2 className="text-sm font-semibold text-[var(--text-primary)]">고정 종목 캐시 상태</h2>
            {!status || status.symbols.length === 0 ? (
              <p className="mt-2 text-xs text-[var(--text-muted)]">고정 종목이 설정되지 않았습니다.</p>
            ) : (
              <div className="mt-3 grid gap-2 sm:grid-cols-2">
                {status.symbols.map((s) => (
                  <div
                    key={s.symbol}
                    className="flex items-center justify-between rounded-lg border border-[var(--border)] px-2.5 py-2 text-xs"
                  >
                    <span className="font-mono font-medium text-[var(--text-primary)]">{s.symbol}</span>
                    <span className="text-right text-[10px] text-[var(--text-muted)]">
                      {s.cached ? (
                        <>
                          {s.source === "llm" ? "AI 생성" : "규칙 기반"} · {formatDateTime(s.generatedAt)}
                        </>
                      ) : (
                        "캐시 없음"
                      )}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="text-sm font-semibold text-[var(--text-primary)]">요청 INPUT 기록</h2>
              <div className="flex items-center gap-2">
                <select
                  value={logStatusFilter}
                  onChange={(e) => setLogStatusFilter(e.target.value as "all" | "failed")}
                  className="rounded-md border border-[var(--border)] bg-[var(--background)] px-2 py-1 text-xs text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
                >
                  <option value="all">전체 상태</option>
                  <option value="failed">실패만</option>
                </select>
                {requestNames.length > 1 && (
                  <select
                    value={nameFilter}
                    onChange={(e) => setNameFilter(e.target.value)}
                    className="rounded-md border border-[var(--border)] bg-[var(--background)] px-2 py-1 text-xs text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
                  >
                    <option value="all">전체 종목</option>
                    {requestNames.map((name) => (
                      <option key={name} value={name}>
                        {name}
                      </option>
                    ))}
                  </select>
                )}
              </div>
            </div>
            <p className="mt-1 text-xs leading-relaxed text-[var(--text-muted)]">
              LLM에 실제로 전송된 입력(factsJSON)과 성공/실패 여부를 기록합니다. 전체{" "}
              {logTotal.toLocaleString()}건 중 {requestLog.length}건 표시 중입니다. 항목을 눌러 펼치면 전체
              입력을 확인할 수 있습니다.
            </p>
            {logBusy ? (
              <p className="mt-2 text-xs text-[var(--text-muted)]">불러오는 중...</p>
            ) : filteredRequestLog.length === 0 ? (
              <p className="mt-2 text-xs text-[var(--text-muted)]">아직 기록된 요청이 없습니다.</p>
            ) : (
              <div className="mt-3 space-y-1.5">
                {filteredRequestLog.map((entry, i) => (
                  <RequestLogRow key={`${entry.symbol}-${entry.timestamp}-${i}`} entry={entry} />
                ))}
              </div>
            )}
            {logTotal > logPageSize && (
              <div className="mt-3 flex items-center justify-between gap-2 border-t border-[var(--border)] pt-2.5">
                <button
                  type="button"
                  disabled={logPage <= 1 || logBusy}
                  onClick={() => setLogPage((p) => Math.max(1, p - 1))}
                  className="rounded-md border border-[var(--border)] px-2.5 py-1 text-xs text-[var(--text-primary)] disabled:opacity-40"
                >
                  이전
                </button>
                <span className="text-[11px] text-[var(--text-muted)]">
                  {logPage} / {Math.max(1, Math.ceil(logTotal / logPageSize))} 페이지
                </span>
                <button
                  type="button"
                  disabled={logPage >= Math.ceil(logTotal / logPageSize) || logBusy}
                  onClick={() => setLogPage((p) => p + 1)}
                  className="rounded-md border border-[var(--border)] px-2.5 py-1 text-xs text-[var(--text-primary)] disabled:opacity-40"
                >
                  다음
                </button>
              </div>
            )}
          </section>
        </div>
      </PageScrollShell>
    </>
  );
}
