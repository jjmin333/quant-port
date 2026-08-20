"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Clock3, Eye, Flag, History, Loader2 } from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { Card } from "@/components/ui/card";
import { Markdown } from "@/components/ui/markdown";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  ContrarianBadge,
  DirectionBadge,
  EvidenceCard,
  HorizonProgress,
  StatusBadge,
  toneClass,
} from "@/components/research/research-shared";
import { ResearchThumbnailPicker } from "@/components/research/research-thumbnail-picker";
import {
  CommentThread,
  RatingPanel,
  ReactionBar,
} from "@/components/research/research-social";
import {
  fetchResearchCallClient,
  fetchResearchRevisionsClient,
  recordResearchViewClient,
  reportResearchClient,
  retractResearchClient,
  updateResearchClient,
} from "@/lib/research-api";
import {
  BENCHMARK_LABELS,
  STATUS_LABELS,
  dataGapLabel,
  VISIBILITY_LABELS,
  type ResearchCall,
  type ResearchRevision,
  type ResearchVisibility,
} from "@/lib/research-types";
import { formatDateTimeKST, formatPercent, formatPrice } from "@/lib/format";
import { readingMinutes } from "@/lib/markdown";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { cn } from "@/lib/utils";

/**
 * 투자 리포트 상세 — 한 컬럼 중앙 정렬 읽기 화면.
 *
 * 이전 2단(본문 + 고정 사이드) 배치를 버렸다. 리포트는 본문이 주인공인 글이고,
 * 사이드에 붙은 수치 패널은 좁은 화면에서 어차피 본문 아래로 떨어져 순서가
 * 뒤집혔다. 지금은 모든 폭에서 같은 순서로 읽힌다: 제목 → 무엇을 주장했나(요약
 * 카드) → 왜 그렇게 보나(본문) → 서버가 잰 근거 → 반응.
 */
export function ResearchDetailClient({ id }: { id: number }) {
  const [call, setCall] = useState<ResearchCall | null>(null);
  const [revisions, setRevisions] = useState<ResearchRevision[]>([]);
  const [showRevisions, setShowRevisions] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setCall(await fetchResearchCallClient(id));
    } catch {
      setError("리포트를 불러오지 못했습니다.");
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  // Recorded once per mount, separately from the fetch — a counter that moved
  // with every re-render would be a render counter. The server de-duplicates
  // the same viewer for 30 minutes on top of this.
  useEffect(() => {
    recordResearchViewClient(id).catch(() => {});
  }, [id]);

  async function openRevisions() {
    if (!showRevisions && revisions.length === 0) {
      setRevisions(await fetchResearchRevisionsClient(id).catch(() => []));
    }
    setShowRevisions((v) => !v);
  }

  async function handleRetract() {
    if (!call) return;
    if (!window.confirm("철회해도 통계와 철회율에는 그대로 남습니다. 진행할까요?")) return;
    setBusy(true);
    try {
      setCall(await retractResearchClient(call.id));
    } finally {
      setBusy(false);
    }
  }

  async function handleReport() {
    if (!call) return;
    const reason = window.prompt(
      "신고 사유를 입력하세요 (spam / manipulation / misinformation / abuse / other)",
      "other",
    );
    if (!reason) return;
    setBusy(true);
    try {
      await reportResearchClient(call.id, reason);
      window.alert("신고가 접수되었습니다.");
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return (
      <>
        <PageHeader title="투자 리포트" />
        <PageScrollShell contentClassName="space-y-4">
          <Card className="p-6 text-center text-sm text-[var(--text-muted)]">불러오는 중…</Card>
        </PageScrollShell>
      </>
    );
  }

  if (error || !call) {
    return (
      <>
        <PageHeader title="투자 리포트" />
        <PageScrollShell contentClassName="space-y-4">
          <Card className="space-y-3 p-6 text-center">
            <p className="text-sm text-[var(--loss)]">{error ?? "리포트를 찾을 수 없습니다."}</p>
            <Link href="/reports" className="text-xs text-[var(--brand)]">
              리포트 목록으로
            </Link>
          </Card>
        </PageScrollShell>
      </>
    );
  }

  const currency = call.currency ?? "KRW";
  const alpha = call.verdictAlpha ?? call.horizon?.alpha;
  const ret = call.verdictReturn ?? call.horizon?.return ?? call.current?.changeRate;
  const gaps = call.context.dataGaps ?? [];

  return (
    <>
      <PageHeader width="reading" title="투자 리포트" subtitle={call.symbolName || call.symbol} />
      <PageScrollShell width="reading" contentClassName="space-y-5">
        <Link
          href="/reports"
          className="inline-flex items-center gap-1 text-xs text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          리포트 목록
        </Link>

        {/* ---- 글머리 ---- */}
        <header className="space-y-3">
          <div className="flex flex-wrap items-center gap-1.5">
            <DirectionBadge direction={call.direction} />
            <StatusBadge status={call.status} />
            {call.contrarian && <ContrarianBadge />}
            {call.visibility !== "public" && (
              <span className="text-[10px] text-[var(--text-muted)]">
                {VISIBILITY_LABELS[call.visibility]}
              </span>
            )}
          </div>

          {/* PageHeader가 이미 페이지의 h1이므로 글 제목은 h2로 둔다 — 크기는
              본문 최상위 제목답게 유지하되 문서 개요가 h1 두 개로 갈라지지 않게. */}
          <h2 className="text-2xl font-bold leading-tight tracking-tight text-[var(--text-primary)] sm:text-[28px]">
            {call.title}
          </h2>

          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-[var(--text-muted)]">
            {call.authorName && (
              <Link
                href={`/reports/authors/${call.userId}`}
                className="font-medium text-[var(--text-secondary)] hover:text-[var(--brand)]"
              >
                {call.authorName}
              </Link>
            )}
            <span>{formatDateTimeKST(call.publishedAt)}</span>
            <span className="inline-flex items-center gap-1">
              <Clock3 className="h-3 w-3" />
              {readingMinutes(call.bodyMd)}분
            </span>
            <span className="inline-flex items-center gap-1">
              <Eye className="h-3 w-3" />
              {call.viewCount.toLocaleString()}
            </span>
            {call.revisions > 1 && (
              <button
                type="button"
                onClick={openRevisions}
                className="inline-flex items-center gap-1 text-[var(--brand)]"
              >
                <History className="h-3 w-3" />
                수정 {call.revisions - 1}회
              </button>
            )}
          </div>

          {showRevisions && (
            <div className="space-y-1.5 rounded-lg bg-[var(--surface-elevated)] p-3">
              <p className="text-[10px] text-[var(--text-muted)]">
                본문 수정 이력 — 지울 수 없습니다
              </p>
              {revisions.map((rev) => (
                <div key={rev.id} className="text-[11px] text-[var(--text-secondary)]">
                  <span className="tabular-nums text-[var(--text-muted)]">rev {rev.rev}</span> ·{" "}
                  {formatDateTimeKST(rev.createdAt)} · {rev.title}
                </div>
              ))}
            </div>
          )}
        </header>

        {/* ---- 무엇을 주장했나 ---- */}
        <Card className="space-y-3 p-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <Link
              href={analysisSymbolHref(call.symbol)}
              className="inline-flex items-center gap-2 text-sm font-medium text-[var(--text-primary)] hover:text-[var(--brand)]"
            >
              <StockAvatar symbol={call.symbol} name={call.symbolName} size="xs" />
              {call.symbolName || call.symbol}
            </Link>
            <div className="flex gap-5">
              <div className="text-right">
                <p className="text-[10px] text-[var(--text-muted)]">
                  알파 ({BENCHMARK_LABELS[call.benchmarkId ?? ""] ?? call.benchmarkId ?? "—"} 대비)
                </p>
                <p className={cn("text-lg font-semibold tabular-nums", toneClass(alpha))}>
                  {alpha === undefined ? "—" : `${formatPercent(alpha)}p`}
                </p>
              </div>
              <div className="text-right">
                <p className="text-[10px] text-[var(--text-muted)]">수익률</p>
                <p className={cn("text-lg font-semibold tabular-nums", toneClass(ret))}>
                  {ret === undefined ? "—" : formatPercent(ret)}
                </p>
              </div>
            </div>
          </div>

          <HorizonProgress
            progress={call.progress}
            publishedAt={call.publishedAt}
            dueAt={call.dueAt}
            status={call.status}
          />

          <dl className="grid grid-cols-2 gap-x-4 gap-y-1.5 text-[11px] sm:grid-cols-3">
            <Row label="기준가" value={formatPrice(call.basePrice, currency)} />
            <Row
              label="현재가"
              value={call.current ? formatPrice(call.current.price, currency) : "—"}
            />
            <Row
              label="목표가"
              value={call.targetPrice ? formatPrice(call.targetPrice, currency) : "—"}
            />
            <Row
              label="손절가"
              value={call.stopPrice ? formatPrice(call.stopPrice, currency) : "—"}
            />
            <Row label="투자 기간" value={`${call.horizonDays}일`} />
            <Row label="확신도" value={`${call.conviction}%`} />
          </dl>

          <p className="text-[10px] leading-relaxed text-[var(--text-muted)]">
            {call.status === "open"
              ? "판정은 종가로만 합니다 — 장중 고가 터치는 목표 달성으로 보지 않습니다."
              : `${STATUS_LABELS[call.status]}${call.verdictDate ? ` · ${call.verdictDate} 종가 기준` : ""}`}
          </p>
        </Card>

        {/* ---- 본문 ---- */}
        {editing && call.canEdit ? (
          <EditPanel
            call={call}
            busy={busy}
            onCancel={() => setEditing(false)}
            onSaved={(updated) => {
              setCall(updated);
              setRevisions([]);
              setEditing(false);
            }}
          />
        ) : (
          <Card className="p-5 sm:p-7">
            {call.bodyMd.trim() === "" ? (
              <p className="text-sm text-[var(--text-muted)]">본문이 없습니다.</p>
            ) : (
              <Markdown source={call.bodyMd} />
            )}
          </Card>
        )}

        {/* ---- 서버가 잰 근거 ---- */}
        <section className="space-y-3">
          <h2 className="text-sm font-semibold text-[var(--text-primary)]">발행 시점 근거</h2>
          <div className="grid gap-3 sm:grid-cols-2">
            <EvidenceCard
              evidence={call.evidence}
              score={call.scoreAtPublish}
              grade={call.gradeAtPublish}
              currency={currency}
            />

            <div className="space-y-3">
              {call.expectedValue && (
                <Card className="space-y-1.5 p-3">
                  <p className="text-[11px] font-medium text-[var(--text-muted)]">
                    발행 시점 기대값
                  </p>
                  <div className="grid grid-cols-2 gap-2 text-[11px]">
                    <div>
                      <p className="text-[10px] text-[var(--text-muted)]">확신도 기준</p>
                      <p
                        className={cn(
                          "font-semibold tabular-nums",
                          toneClass(call.expectedValue.statedEV),
                        )}
                      >
                        {call.expectedValue.statedEV === undefined
                          ? "—"
                          : formatPercent(call.expectedValue.statedEV)}
                      </p>
                    </div>
                    <div>
                      <p className="text-[10px] text-[var(--text-muted)]">실적 기준</p>
                      <p
                        className={cn(
                          "font-semibold tabular-nums",
                          toneClass(call.expectedValue.historicalEV),
                        )}
                      >
                        {call.expectedValue.historicalEV === undefined
                          ? "—"
                          : formatPercent(call.expectedValue.historicalEV)}
                      </p>
                    </div>
                  </div>
                  {call.expectedValue.riskReward !== undefined && (
                    <p className="text-[10px] text-[var(--text-muted)]">
                      손익비 {call.expectedValue.riskReward.toFixed(2)}:1
                    </p>
                  )}
                </Card>
              )}

              {call.returns.length > 0 && (
                <Card className="space-y-2 p-3">
                  <p className="text-[11px] font-medium text-[var(--text-muted)]">구간별 성과</p>
                  <div className="space-y-1">
                    {call.returns.map((r) => (
                      <div key={r.horizon} className="flex items-center justify-between text-[11px]">
                        <span className="text-[var(--text-muted)]">{r.horizon}</span>
                        <span className="flex gap-3">
                          <span className={cn("tabular-nums", toneClass(r.return))}>
                            {r.return === undefined ? "—" : formatPercent(r.return)}
                          </span>
                          <span className={cn("w-16 text-right tabular-nums", toneClass(r.alpha))}>
                            {r.alpha === undefined ? "—" : `${formatPercent(r.alpha)}p`}
                          </span>
                        </span>
                      </div>
                    ))}
                  </div>
                  <p className="text-[9px] text-[var(--text-muted)]">
                    아직 도래하지 않은 구간은 0%가 아니라 —로 표시합니다.
                  </p>
                </Card>
              )}
            </div>
          </div>

          {call.context.news.length > 0 && (
            <Card className="space-y-2 p-4">
              <p className="text-[11px] font-medium text-[var(--text-muted)]">발행 시점 뉴스</p>
              <ul className="space-y-1.5">
                {call.context.news.map((n, i) => (
                  <li key={n.id ?? `${i}`} className="text-xs text-[var(--text-secondary)]">
                    {n.url ? (
                      <a
                        href={n.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="hover:text-[var(--brand)]"
                      >
                        {n.headline}
                      </a>
                    ) : (
                      n.headline
                    )}
                    <span className="ml-1.5 text-[10px] text-[var(--text-muted)]">
                      {n.publisherName}
                    </span>
                  </li>
                ))}
              </ul>
            </Card>
          )}

          {gaps.length > 0 && (
            <Card className="p-3">
              <p className="text-[11px] text-[var(--text-muted)]">
                발행 시점에 확보하지 못한 데이터: {gaps.map(dataGapLabel).join(", ")}
              </p>
            </Card>
          )}
        </section>

        {/* ---- 반응 ---- */}
        <div className="flex flex-wrap items-center justify-between gap-2">
          <ReactionBar call={call} />
          <div className="flex flex-wrap items-center gap-2">
            {call.canEdit && call.status !== "retracted" && (
              <>
                <button
                  type="button"
                  onClick={() => setEditing(true)}
                  className="rounded-lg border border-[var(--border)] px-3 py-1.5 text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
                >
                  본문 수정
                </button>
                <button
                  type="button"
                  onClick={handleRetract}
                  disabled={busy}
                  className="rounded-lg border border-[var(--border)] px-3 py-1.5 text-xs text-[var(--loss)] disabled:opacity-40"
                >
                  철회
                </button>
              </>
            )}
            {!call.canEdit && (
              <button
                type="button"
                onClick={handleReport}
                disabled={busy}
                className="inline-flex items-center gap-1 rounded-lg border border-[var(--border)] px-3 py-1.5 text-xs text-[var(--text-muted)] disabled:opacity-40"
              >
                <Flag className="h-3 w-3" />
                신고
              </button>
            )}
          </div>
        </div>

        <RatingPanel call={call} onRated={load} />

        <CommentThread callId={call.id} />

        <p className="px-1 pb-2 text-[10px] leading-relaxed text-[var(--text-muted)]">
          이 리포트는 작성자 개인의 의견이며 투자 권유가 아닙니다. 서비스는 발행 시점 데이터를
          박제하고 결과를 채점할 뿐입니다.
        </p>
      </PageScrollShell>
    </>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-2">
      <dt className="text-[var(--text-muted)]">{label}</dt>
      <dd className="tabular-nums text-[var(--text-secondary)]">{value}</dd>
    </div>
  );
}

/**
 * The only editable surface. Report values are absent here by design — the API
 * would reject them anyway, but the UI should not suggest they are negotiable.
 */
function EditPanel({
  call,
  busy,
  onCancel,
  onSaved,
}: {
  call: ResearchCall;
  busy: boolean;
  onCancel: () => void;
  onSaved: (updated: ResearchCall) => void;
}) {
  const [title, setTitle] = useState(call.title);
  const [body, setBody] = useState(call.bodyMd);
  const [thumbnailUrl, setThumbnailUrl] = useState(call.thumbnailUrl ?? "");
  const [visibility, setVisibility] = useState<ResearchVisibility>(call.visibility);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    setSaving(true);
    setError(null);
    try {
      onSaved(
        await updateResearchClient(call.id, { title, bodyMd: body, thumbnailUrl, visibility }),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "저장에 실패했습니다.");
      setSaving(false);
    }
  }

  return (
    <Card className="space-y-3 p-4 sm:p-5">
      <input
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        maxLength={200}
        className="w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3.5 py-2.5 text-base font-semibold text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
      />
      <MarkdownEditor value={body} onChange={setBody} rows={18} disabled={saving} />

      {/* 표지는 논거가 아니라 표현이다 — 바꿔도 수정 이력(bodyRev)이 오르지 않는다. */}
      <div className="space-y-1.5 border-t border-[var(--border)] pt-3">
        <p className="text-[11px] font-medium text-[var(--text-muted)]">표지 이미지 (선택)</p>
        <ResearchThumbnailPicker
          value={thumbnailUrl}
          onChange={setThumbnailUrl}
          bodyMd={body}
          symbol={call.symbol}
          symbolName={call.symbolName}
          currency={call.currency}
        />
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5">
          {(["public", "unlisted", "private"] as ResearchVisibility[]).map((v) => (
            <button
              key={v}
              type="button"
              onClick={() => setVisibility(v)}
              className={cn(
                "rounded-md px-2.5 py-1 text-xs font-medium transition-colors",
                visibility === v
                  ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
                  : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
              )}
            >
              {VISIBILITY_LABELS[v]}
            </button>
          ))}
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={onCancel}
            className="rounded-lg border border-[var(--border)] px-3 py-1.5 text-xs text-[var(--text-muted)]"
          >
            취소
          </button>
          <button
            type="button"
            onClick={save}
            disabled={saving || busy}
            className="inline-flex items-center gap-1.5 rounded-lg bg-[var(--brand)] px-3 py-1.5 text-xs font-medium text-white disabled:opacity-40"
          >
            {saving && <Loader2 className="h-3 w-3 animate-spin" />}
            저장
          </button>
        </div>
      </div>
      <p className="text-[10px] text-[var(--text-muted)]">
        본문이 바뀌면 수정 이력이 공개 기록으로 남습니다. 투자 의견 값(방향·가격·기간·확신도)은
        수정할 수 없습니다.
      </p>
      {error && <p className="text-xs text-[var(--loss)]">{error}</p>}
    </Card>
  );
}
