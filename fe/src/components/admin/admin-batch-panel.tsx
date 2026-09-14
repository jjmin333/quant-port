"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  AlertTriangle,
  BookOpen,
  CheckCircle2,
  ChevronDown,
  Clock,
  Loader2,
  Pencil,
  Play,
  RefreshCw,
  XCircle,
} from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { Markdown } from "@/components/ui/markdown";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import { formatDateTimeKST } from "@/lib/format";
import {
  fetchAdminBatchJobs,
  fetchAdminBatchSchedule,
  runAdminBatchJob,
  saveAdminBatchSchedule,
  batchHealthNeedsAttention,
  BATCH_HEALTH_LABELS,
  type BatchHealthView,
  type BatchJobCategory,
  type BatchJobRun,
  type BatchJobStatus,
  type BatchScheduleDoc,
} from "@/lib/admin";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";
import { buttonVariants } from "@/components/ui/button";

function formatDateTime(value: string | undefined): string {
  if (!value) return "-";
  return formatDateTimeKST(value);
}

/**
 * 실행이 어디서 왔는지. 「재처리」를 따로 두는 이유는, 다운으로 놓친 슬롯을
 * 워치독이 채운 것을 「수동」으로 보이면 누른 적 없는 실행을 보고 원인을 잘못
 * 짚기 때문이다. 모르는 값은 그대로 보여준다 — 조용히 「수동」으로 접지 않는다.
 */
function triggerKindLabel(kind: string): string {
  switch (kind) {
    case "schedule":
      return "스케줄";
    case "manual":
      return "수동";
    case "catchup":
      return "재처리";
    default:
      return kind;
  }
}

function duration(run: BatchJobRun | undefined): string {
  if (!run?.finishedAt) return "";
  const start = new Date(run.startedAt).getTime();
  const end = new Date(run.finishedAt).getTime();
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return "";
  const sec = Math.round((end - start) / 1000);
  if (sec < 60) return `${sec}초`;
  return `${Math.floor(sec / 60)}분 ${sec % 60}초`;
}

function StatusIcon({ status }: { status: BatchJobRun["status"] }) {
  if (status === "running") {
    return <Loader2 className="h-3.5 w-3.5 animate-spin text-[var(--brand)]" />;
  }
  if (status === "error") {
    return <XCircle className="h-3.5 w-3.5 text-[var(--danger)]" />;
  }
  return <CheckCircle2 className="h-3.5 w-3.5 text-[var(--success)]" />;
}

/** Coverage is a small free-form map per job; render it as label: value pairs. */
function CoverageRow({ coverage }: { coverage: Record<string, unknown> }) {
  const LABELS: Record<string, string> = {
    symbols: "대상 종목",
    activeSymbols: "활성 종목",
    latestTradeDate: "최신 거래일",
    latestFilingDate: "최신 접수일",
    tradeDate: "기준일",
    rows: "행",
    scored: "점수 산출",
    indices: "지수",
    considered: "레이더 대상",
    planned: "이번 실행 계획",
    succeeded: "성공",
    failed: "실패",
    skipped: "건너뜀",
    // 적재 배치(등급제 로테이션)의 진행 지표.
    synced: "수집 완료",
    withData: "내역 있음",
    errors: "실패",
    oldestSyncedAt: "가장 오래된 수집",
  };
  const entries = Object.entries(coverage).filter(([, v]) => v != null && v !== "");
  if (entries.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-x-4 gap-y-1 text-cell tabular-nums text-[var(--text-muted)]">
      {entries.map(([key, value]) => (
        <span key={key}>
          {LABELS[key] ?? key}{" "}
          <b className="font-medium text-[var(--text-secondary)]">
            {typeof value === "number"
              ? value.toLocaleString("ko-KR")
              : key === "oldestSyncedAt"
                ? formatDateTime(String(value))
                : String(value)}
          </b>
        </span>
      ))}
    </div>
  );
}

function JobCard({
  job,
  busy,
  onRun,
}: {
  job: BatchJobStatus;
  busy: boolean;
  onRun: (since: string, limit: number) => void;
}) {
  const [since, setSince] = useState("");
  // 공시 스윕만 축이 종목이 아니라 날짜라, 같은 칸을 다른 뜻으로 쓴다.
  // 기본값도 그에 맞춘다 — 1,500일치 공시를 받으려는 사람은 없다.
  const limitMeansDays = job.spec.id === "disclosures_daily";
  const [limit, setLimit] = useState<number>(limitMeansDays ? 1 : 500);
  const rateLimited = job.spec.cost === "rate_limited";

  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3">
      <div className="mb-1.5 flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">{job.spec.label}</h3>
          <span
            className={cn(
              "rounded-full px-1.5 py-0.5 text-micro",
              rateLimited
                ? "bg-[var(--danger)]/10 text-[var(--danger)]"
                : "bg-[var(--success)]/10 text-[var(--success)]",
            )}
          >
            {rateLimited ? "외부 API 호출" : "DB 전용"}
          </span>
          {job.running && (
            <span className="flex items-center gap-1 text-micro text-[var(--brand)]">
              <Loader2 className="h-3 w-3 animate-spin" />
              실행 중
            </span>
          )}
        </div>
        <span className="flex items-center gap-1 text-micro text-[var(--text-muted)]">
          <Clock className="h-3 w-3" />
          {job.spec.schedule}
        </span>
      </div>

      <p className="mb-2 text-cell leading-relaxed text-[var(--text-secondary)]">
        {job.spec.purpose}
      </p>

      {job.spec.note && (
        <p className="mb-2 flex items-start gap-1.5 rounded-md border border-dashed border-[var(--border)] px-2 py-1.5 text-micro leading-relaxed text-[var(--text-muted)]">
          <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0" />
          {job.spec.note}
        </p>
      )}

      {job.coverage && <div className="mb-2">{<CoverageRow coverage={job.coverage} />}</div>}

      <div className="mb-2 flex flex-wrap items-end gap-2">
        {job.spec.acceptsSince && (
          <label className="text-micro text-[var(--text-muted)]">
            <span className="mb-0.5 block">기준일</span>
            <input
              type="date"
              value={since}
              onChange={(e) => setSince(e.target.value)}
              className="rounded border border-[var(--border)] bg-[var(--surface)] px-1.5 py-1 text-cell"
            />
          </label>
        )}
        {job.spec.acceptsLimit && (
          <label className="text-micro text-[var(--text-muted)]">
            <span className="mb-0.5 block">{limitMeansDays ? "거슬러 받을 일수" : "최대 종목"}</span>
            <input
              type="number"
              min={1}
              max={limitMeansDays ? 90 : 20000}
              value={limit}
              onChange={(e) => setLimit(Number(e.target.value))}
              className="w-24 rounded border border-[var(--border)] bg-[var(--surface)] px-1.5 py-1 text-right text-cell"
            />
          </label>
        )}
        <button
          type="button"
          onClick={() => onRun(since, limit)}
          disabled={busy}
          className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--brand)] text-[var(--brand)] disabled:opacity-50")}
        >
          <Play className="h-3.5 w-3.5" />
          실행
        </button>
      </div>

      {job.lastRun ? (
        <div className="border-t border-[var(--border)] pt-2 text-cell">
          <div className="flex flex-wrap items-center gap-2">
            <StatusIcon status={job.lastRun.status} />
            <span className="text-[var(--text-muted)]">
              {formatDateTime(job.lastRun.startedAt)}
              {duration(job.lastRun) && ` · ${duration(job.lastRun)}`}
              {` · ${triggerKindLabel(job.lastRun.triggerKind)}`}
            </span>
          </div>
          {job.lastRun.message && (
            <p
              className={cn(
                "mt-1 leading-relaxed",
                job.lastRun.status === "error"
                  ? "text-[var(--danger)]"
                  : "text-[var(--text-secondary)]",
              )}
            >
              {job.lastRun.message}
            </p>
          )}
        </div>
      ) : (
        <p className="border-t border-[var(--border)] pt-2 text-cell text-[var(--text-muted)]">
          실행 기록 없음
        </p>
      )}
    </div>
  );
}

/**
 * 잡 묶음. 순서가 곧 화면 순서다 — 적재가 먼저인 것은, 조회 경로가 그 표들만
 * 읽으므로 여기가 멈추면 화면이 낡기 때문이다.
 */
const CATEGORIES: { id: BatchJobCategory; label: string; hint: string }[] = [
  {
    id: "ingest",
    label: "외부 소스 적재",
    hint: "외부 API에서 받아 표에 담습니다. 조회 화면은 이 표들만 읽습니다 — 여기가 멈추면 화면이 낡습니다.",
  },
  {
    id: "derive",
    label: "집계·판정",
    hint: "이미 가진 데이터로 계산합니다. 외부 한도와 무관합니다.",
  },
  { id: "llm", label: "AI 생성", hint: "LLM 지출이 발생합니다." },
];

/**
 * 시간표·수동 실행 안내. 리포지터리의 `be/docs/BATCH_SCHEDULE.md`를 그대로
 * 렌더한다 — 화면에 규칙을 따로 적어 두면 배치를 고칠 때 남아서 거짓말이 된다.
 * 저장도 그 파일을 덮어쓴다. 시간표 맨 오른쪽 메모 칸이 운영 중 적는 자리.
 */
function ScheduleDoc() {
  const [open, setOpen] = useState(false);
  const [doc, setDoc] = useState<BatchScheduleDoc | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    if (!open || editing) return;
    setError(null);
    fetchAdminBatchSchedule()
      .then(setDoc)
      .catch((e) => setError(toUserMessage(e, "문서를 불러오지 못했습니다")));
  }, [open, editing]);

  function startEdit() {
    if (!doc) return;
    setDraft(doc.markdown);
    setSaveError(null);
    setEditing(true);
  }

  function cancelEdit() {
    setEditing(false);
    setDraft("");
    setSaveError(null);
  }

  async function save() {
    setSaving(true);
    setSaveError(null);
    try {
      const next = await saveAdminBatchSchedule(draft);
      setDoc(next);
      setEditing(false);
      setDraft("");
    } catch (e) {
      setSaveError(toUserMessage(e, "저장에 실패했습니다"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)]">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2 px-3 py-2.5 text-left"
      >
        <BookOpen className="h-4 w-4 text-[var(--brand)]" />
        <span className="text-sm font-semibold text-[var(--text-primary)]">
          시간표 · 수동 실행 안내
        </span>
        <span className="text-cell text-[var(--text-muted)]">
          어느 잡이 몇 시에 어느 한도를 쓰는지, 오른쪽 메모는 여기서 고친다
        </span>
        <ChevronDown
          className={cn(
            "ml-auto h-4 w-4 shrink-0 text-[var(--text-muted)] transition-transform",
            open && "rotate-180",
          )}
        />
      </button>
      {open && (
        <div className="border-t border-[var(--border)] px-3 py-3">
          {error && <p className="text-cell text-[var(--danger)]">{error}</p>}
          {!error && !doc && (
            <p className="text-cell text-[var(--text-muted)]">불러오는 중…</p>
          )}
          {doc && (
            <>
              <div className="mb-3 flex flex-wrap items-center justify-end gap-2">
                {editing ? (
                  <>
                    <button
                      type="button"
                      onClick={cancelEdit}
                      disabled={saving}
                      className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] disabled:opacity-50")}
                    >
                      취소
                    </button>
                    <button
                      type="button"
                      onClick={() => void save()}
                      disabled={saving || draft.trim() === ""}
                      className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--brand)] text-[var(--brand)] disabled:opacity-50")}
                    >
                      {saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                      저장
                    </button>
                  </>
                ) : (
                  <button
                    type="button"
                    onClick={startEdit}
                    disabled={!doc.writable}
                    title={
                      doc.writable
                        ? "시간표와 메모를 고칩니다. 저장하면 리포 파일이 바뀝니다."
                        : "이 환경에서는 파일을 쓸 수 없습니다. Docker 볼륨·BATCH_SCHEDULE_PATH를 확인하세요."
                    }
                    className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] hover:bg-[var(--surface-elevated)] disabled:cursor-not-allowed disabled:opacity-50")}
                  >
                    <Pencil className="h-3.5 w-3.5" />
                    수정
                  </button>
                )}
              </div>
              {saveError && (
                <p className="mb-3 text-cell text-[var(--danger)]">{saveError}</p>
              )}
              {!doc.writable && !editing && (
                <p className="mb-3 text-cell text-[var(--text-muted)]">
                  읽기 전용입니다. 컨테이너가 `BATCH_SCHEDULE.md`에 쓰지 못합니다.
                </p>
              )}
              {editing ? (
                <MarkdownEditor
                  value={draft}
                  onChange={setDraft}
                  rows={28}
                  disabled={saving}
                  hint="시간표 표의 맨 오른쪽 「메모」칸에 운영 메모를 적습니다. 「## 시간표」제목은 지우지 마세요."
                />
              ) : (
                <Markdown source={doc.markdown} className="text-note leading-[1.75]" />
              )}
              <p className="mt-4 border-t border-[var(--border)] pt-2 text-micro text-[var(--text-muted)]">
                원본: <code>{doc.path}</code>
                {doc.source === "embed" ? " · 이미지에 넣은 사본(디스크 파일 없음)" : " · 디스크"}
                {doc.writable ? " · 저장 가능" : " · 읽기 전용"}
              </p>
            </>
          )}
        </div>
      )}
    </section>
  );
}

function formatLateFor(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) return "";
  const hours = Math.floor(seconds / 3600);
  if (hours >= 24) return ` · ${Math.floor(hours / 24)}일 지남`;
  if (hours >= 1) return ` · ${hours}시간 지남`;
  return ` · ${Math.max(1, Math.floor(seconds / 60))}분 지남`;
}

/**
 * 배치 감시 배너.
 *
 * 화면을 열어야만 아는 상태이지만, 그래도 **열면 반드시 보이는 자리**여야 한다 —
 * 이 기능이 생기기 전에는 「AI 요약이 19일째 안 돌았다」가 목록 한가운데 조용히
 * 앉아 있었고 아무도 못 봤다.
 *
 * 두 가지를 따로 말한다: 잡들이 늦었는지, 그리고 **그것을 판정하는 워치독이 살아
 * 있는지**. 후자가 죽으면 전자의 「정상」은 정상이라는 뜻이 아니다.
 */
function BatchHealthBanner({ health }: { health: BatchHealthView | null }) {
  if (!health) return null;

  const attention = (health.report?.jobs ?? []).filter((j) =>
    batchHealthNeedsAttention(j.health),
  );
  const watchdogDown = health.watchdogStale;

  if (attention.length === 0 && !watchdogDown) {
    return (
      <div className="flex items-center gap-2 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2 text-cell text-[var(--text-secondary)]">
        <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-[var(--success)]" />
        <span>
          예약된 배치가 모두 제 시각에 돌았습니다
          <span className="text-[var(--text-muted)]">
            {" "}
            · {formatDateTime(health.report?.computedAt)} 기준
          </span>
        </span>
      </div>
    );
  }

  return (
    <div className="space-y-1.5 rounded-lg border border-[var(--danger)]/40 bg-[var(--danger)]/5 px-3 py-2.5">
      <div className="flex items-center gap-2 text-cell font-semibold text-[var(--danger)]">
        <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
        {watchdogDown && attention.length === 0 ? (
          <span>배치 감시가 멈췄습니다 — 아래 상태는 최신이 아닐 수 있습니다</span>
        ) : (
          <span>제때 돌지 않은 배치 {attention.length}개</span>
        )}
      </div>
      <ul className="space-y-0.5 text-cell text-[var(--text-secondary)]">
        {attention.map((job) => (
          <li key={job.jobId}>
            <span className="font-medium text-[var(--text-primary)]">{job.label}</span>
            <span className="text-[var(--text-muted)]">
              {" "}
              — {BATCH_HEALTH_LABELS[job.health] ?? job.health}
              {formatLateFor(job.lateBySeconds)} · 마지막 성공{" "}
              {formatDateTime(job.lastSuccessAt)}
            </span>
          </li>
        ))}
      </ul>
      {watchdogDown && attention.length > 0 && (
        <p className="text-micro text-[var(--text-muted)]">
          감시 자체도 멈춰 있습니다 — worker-scores가 살아 있는지 확인하세요.
        </p>
      )}
      {!watchdogDown && (
        <p className="text-micro text-[var(--text-muted)]">
          휴장일은 기대에서 빠집니다 — 여기 뜬 것은 돌았어야 했는데 안 돈 것입니다.
        </p>
      )}
    </div>
  );
}

export function AdminBatchPanel() {
  const router = useRouter();
  const { session, loading: authLoading } = useAuth();
  const isAdmin = isAdminSession(session);

  const [jobs, setJobs] = useState<BatchJobStatus[]>([]);
  const [health, setHealth] = useState<BatchHealthView | null>(null);
  const [history, setHistory] = useState<BatchJobRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const anyRunning = jobs.some((j) => j.running);

  const load = useCallback(async () => {
    try {
      const res = await fetchAdminBatchJobs();
      setJobs(res.jobs ?? []);
      setHistory(res.history ?? []);
      setHealth(res.health ?? null);
      setError(null);
    } catch (e) {
      setError(toUserMessage(e, "배치 목록을 불러오지 못했습니다"));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (authLoading) return;
    if (!isAdmin) {
      router.replace("/dashboard");
      return;
    }
    void load();
  }, [authLoading, isAdmin, router, load]);

  // Poll while something is in flight so the outcome appears without a manual
  // refresh; stop as soon as it settles rather than polling forever.
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  useEffect(() => {
    if (!anyRunning) {
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
      return;
    }
    if (pollRef.current) return;
    pollRef.current = setInterval(() => void load(), 4000);
    return () => {
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
    };
  }, [anyRunning, load]);

  async function handleRun(job: BatchJobStatus, since: string, limit: number) {
    setError(null);
    setNotice(null);
    try {
      await runAdminBatchJob({
        job: job.spec.id,
        since: since || undefined,
        limit: job.spec.acceptsLimit ? limit : undefined,
      });
      setNotice(`${job.spec.label} 실행을 시작했습니다.`);
      await load();
    } catch (e) {
      setError(toUserMessage(e, "실행 실패"));
    }
  }

  if (authLoading || (!isAdmin && !error)) {
    return (
      <>
        <PageHeader title="배치 관리" />
        <PageScrollShell>
          <div className="p-6 text-sm text-[var(--text-muted)]">확인 중…</div>
        </PageScrollShell>
      </>
    );
  }

  return (
    <>
      <PageHeader
        title="배치 관리"
        subtitle="적재·점수 배치의 실행 상태와 커버리지 — 수동 실행 포함"
      />
      <PageScrollShell>
      <div className="space-y-4 p-3 sm:p-4">
        {error && (
          <div className="rounded-lg border border-[var(--danger)]/40 bg-[var(--danger)]/5 px-3 py-2 text-cell text-[var(--danger)]">
            {error}
          </div>
        )}
        {notice && (
          <div className="rounded-lg border border-[var(--brand)]/40 bg-[var(--brand)]/5 px-3 py-2 text-cell text-[var(--brand)]">
            {notice}
          </div>
        )}

        <BatchHealthBanner health={health} />

        <ScheduleDoc />

        <div className="flex items-center justify-between">
          <p className="text-cell text-[var(--text-muted)]">
            같은 외부 소스를 쓰는 잡은 서로를 기다립니다 — 겹쳐 실행해도 실패하지 않고 순서대로
            처리됩니다.
          </p>
          <button
            type="button"
            onClick={() => void load()}
            className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] hover:bg-[var(--surface-elevated)]")}
          >
            <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
            새로고침
          </button>
        </div>

        {loading ? (
          <div className="py-8 text-center text-cell text-[var(--text-muted)]">불러오는 중…</div>
        ) : (
          CATEGORIES.map((category) => {
            const inCategory = jobs.filter((j) => j.spec.category === category.id);
            if (inCategory.length === 0) return null;
            return (
              <section key={category.id} className="space-y-2">
                <div className="flex flex-wrap items-baseline gap-2">
                  <h2 className="text-section font-semibold text-[var(--text-primary)]">
                    {category.label}
                  </h2>
                  <span className="text-micro text-[var(--text-muted)]">{category.hint}</span>
                </div>
                <div className="grid gap-2.5 lg:grid-cols-2">
                  {inCategory.map((job) => (
                    <JobCard
                      key={job.spec.id}
                      job={job}
                      // 잡별로 막는다 — BE의 중복 실행 방지도 잡 단위(HasPending)이고,
                      // 워커도 잡마다 따로 돈다. 전역으로 막으면 40분짜리 경고
                      // 배치가 도는 동안 다른 잡을 한 번도 못 돌린다.
                      busy={job.running}
                      onRun={(since, limit) => void handleRun(job, since, limit)}
                    />
                  ))}
                </div>
              </section>
            );
          })
        )}

        {history.length > 0 && (
          <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3">
            <h2 className="mb-2 text-section font-semibold">최근 실행 기록</h2>
            <ul className="divide-y divide-[var(--border)]">
              {history.map((run) => (
                <li key={run.id} className="flex flex-wrap items-baseline gap-2 py-1.5 text-cell">
                  <StatusIcon status={run.status} />
                  <span className="font-medium text-[var(--text-primary)]">{run.jobId}</span>
                  <span className="text-[var(--text-muted)]">
                    {formatDateTime(run.startedAt)}
                    {duration(run) && ` · ${duration(run)}`}
                  </span>
                  {run.message && (
                    <span
                      className={cn(
                        "min-w-0 flex-1",
                        run.status === "error"
                          ? "text-[var(--danger)]"
                          : "text-[var(--text-secondary)]",
                      )}
                    >
                      {run.message}
                    </span>
                  )}
                </li>
              ))}
            </ul>
          </section>
        )}
      </div>
      </PageScrollShell>
    </>
  );
}
