"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Loader2, RefreshCw, Save, Trash2 } from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import {
  buildCacheScanPattern,
  deleteAdminCacheEntry,
  fetchAdminCacheEntries,
  fetchAdminCacheEntry,
  fetchAdminCacheNamespaces,
  updateAdminCacheEntry,
  type CacheEntryDetail,
  type CacheEntrySummary,
  type CacheNamespace,
} from "@/lib/admin";
import { cn } from "@/lib/utils";
import { useConfirm } from "@/components/providers/confirm-provider";
import { toUserMessage } from "@/lib/error-message";
import { buttonVariants } from "@/components/ui/button";

function formatTTL(seconds: number): string {
  if (seconds === -1) return "만료 없음";
  if (seconds === -2) return "—";
  if (seconds < 60) return `${seconds}초`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}분`;
  if (seconds < 86400) return `${(seconds / 3600).toFixed(1)}시간`;
  return `${(seconds / 86400).toFixed(1)}일`;
}

function formatSize(bytes: number): string {
  if (bytes <= 0) return "—";
  if (bytes < 1024) return `${bytes} B`;
  return `${(bytes / 1024).toFixed(1)} KB`;
}

export function AdminCachePanel() {
  const confirm = useConfirm();
  const router = useRouter();
  const { session, loading } = useAuth();

  const [namespaces, setNamespaces] = useState<CacheNamespace[]>([]);
  const [namespaceId, setNamespaceId] = useState("quotes");
  const [searchInput, setSearchInput] = useState("");
  const [searchTerm, setSearchTerm] = useState("");
  const [entries, setEntries] = useState<CacheEntrySummary[]>([]);
  const [cursor, setCursor] = useState(0);
  const [nextCursor, setNextCursor] = useState(0);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [detail, setDetail] = useState<CacheEntryDetail | null>(null);
  const [valueDraft, setValueDraft] = useState("");
  const [ttlDraft, setTtlDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const namespacePattern =
    namespaces.find((n) => n.id === namespaceId)?.pattern ?? "quotes:*";

  const activePattern = buildCacheScanPattern(namespacePattern, searchTerm);

  const loadNamespaces = useCallback(async () => {
    const list = await fetchAdminCacheNamespaces();
    setNamespaces(list);
  }, []);

  const loadEntries = useCallback(async (reset: boolean, startCursor = 0) => {
      setBusy(true);
      setError(null);
      try {
        const result = await fetchAdminCacheEntries({
          pattern: activePattern,
          cursor: reset ? 0 : startCursor,
          limit: 50,
        });
        const rows = result.entries ?? [];
        setEntries((prev) => (reset ? rows : [...prev, ...rows]));
        setCursor(result.cursor);
        setNextCursor(result.cursor);
      } catch (e) {
        setError(toUserMessage(e, "목록 불러오기 실패"));
      } finally {
        setBusy(false);
      }
    },
    [activePattern],
  );

  const loadDetail = useCallback(async (key: string) => {
    setBusy(true);
    setError(null);
    try {
      const row = await fetchAdminCacheEntry(key);
      setDetail(row);
      setValueDraft(row.value);
      setTtlDraft(row.ttlSeconds >= 0 ? String(row.ttlSeconds) : "");
      setSelectedKey(key);
    } catch (e) {
      setError(toUserMessage(e, "항목 불러오기 실패"));
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => setSearchTerm(searchInput), 300);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    if (loading) return;
    if (!isAdminSession(session)) {
      router.replace("/");
      return;
    }
    void loadNamespaces();
  }, [loading, session, router, loadNamespaces]);

  useEffect(() => {
    if (!isAdminSession(session)) return;
    setEntries([]);
    setCursor(0);
    setNextCursor(0);
    setSelectedKey(null);
    setDetail(null);
    void loadEntries(true);
  }, [activePattern, session, loadEntries]);

  async function handleSave() {
    if (!selectedKey || !detail || detail.readOnly) return;
    setBusy(true);
    setError(null);
    try {
      const ttlParsed = ttlDraft.trim() === "" ? undefined : Number(ttlDraft);
      const updated = await updateAdminCacheEntry({
        key: selectedKey,
        value: valueDraft,
        ttlSeconds: Number.isFinite(ttlParsed) ? ttlParsed : undefined,
      });
      setDetail(updated);
      setValueDraft(updated.value);
      setEntries((prev) =>
        prev.map((row) =>
          row.key === selectedKey
            ? { ...row, ttlSeconds: updated.ttlSeconds, sizeBytes: updated.value.length }
            : row,
        ),
      );
    } catch (e) {
      setError(toUserMessage(e, "저장 실패"));
    } finally {
      setBusy(false);
    }
  }

  async function handleDelete() {
    if (!selectedKey || detail?.readOnly) return;
    if (
      !(await confirm({
        title: "캐시 키를 삭제할까요?",
        description: <code className="break-all text-cell">{selectedKey}</code>,
        confirmLabel: "삭제",
        tone: "danger",
      }))
    ) {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await deleteAdminCacheEntry(selectedKey);
      setEntries((prev) => prev.filter((row) => row.key !== selectedKey));
      setSelectedKey(null);
      setDetail(null);
      setValueDraft("");
    } catch (e) {
      setError(toUserMessage(e, "삭제 실패"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <PageHeader
        title="캐시 관리"
        subtitle="Redis hot cache 조회 · 수정 · 삭제 (관리자 전용)"
      />
      <PageScrollShell className="space-y-4 p-4">
        <div className="flex flex-wrap items-center gap-2 text-cell">
          <Link
            href="/admin"
            className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]")}
          >
            ← 관리자 홈
          </Link>
          <button
            type="button"
            disabled={busy}
            onClick={() => void loadEntries(true)}
            className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] hover:bg-[var(--surface-elevated)]")}
          >
            {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
            새로고침
          </button>
        </div>

        {error && (
          <p className="rounded-md border border-[var(--loss)]/40 bg-[var(--loss)]/10 px-3 py-2 text-cell text-[var(--loss)]">
            {error}
          </p>
        )}

        <section className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-3">
          <h2 className="text-section font-semibold text-[var(--text-primary)]">네임스페이스</h2>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {namespaces.map((ns) => (
              <button
                key={ns.id}
                type="button"
                onClick={() => {
                  setNamespaceId(ns.id);
                  setSearchInput("");
                  setSearchTerm("");
                }}
                className={cn(buttonVariants({ variant: "outline", size: "xs" }),
                  namespaceId === ns.id && !searchInput.trim()
                    ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                    : "border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
                )}
              >
                {ns.label}
              </button>
            ))}
          </div>
          <p className="mt-2 text-micro text-[var(--text-muted)]">
            {namespaces.find((n) => n.id === namespaceId)?.description ?? "사용자 패턴"}
          </p>
          <label className="mt-3 block text-micro text-[var(--text-muted)]">
            키워드 검색
            <input
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="예: 005930, detail, rankings"
              className="mt-1 w-full rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 py-1.5 font-code text-cell text-[var(--text-primary)]"
            />
          </label>
          <p className="mt-1 text-micro text-[var(--text-muted)]">
            키워드는 부분 일치로 검색됩니다. Redis glob(<code className="font-code">*</code>,{" "}
            <code className="font-code">?</code>)을 쓰면 그대로 패턴으로 사용합니다.
          </p>
          <p className="mt-1 font-code text-micro text-[var(--text-muted)]">SCAN 패턴: {activePattern}</p>
        </section>

        <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
          <section className="rounded-lg border border-[var(--border)] bg-[var(--surface)]">
            <div className="border-b border-[var(--border)] px-3 py-2 text-cell font-medium text-[var(--text-secondary)]">
              키 목록 ({entries.length})
            </div>
            <div className="max-h-[420px] overflow-y-auto">
              {entries.length === 0 ? (
                <p className="p-4 text-cell text-[var(--text-muted)]">항목 없음</p>
              ) : (
                <ul>
                  {entries.map((row) => (
                    <li key={row.key}>
                      <button
                        type="button"
                        onClick={() => void loadDetail(row.key)}
                        className={cn(
                          "w-full border-b border-[var(--border)]/60 px-3 py-2 text-left hover:bg-[var(--surface-elevated)]",
                          selectedKey === row.key && "bg-[var(--brand-muted)]/40",
                        )}
                      >
                        <p className="truncate font-code text-cell text-[var(--text-primary)]">{row.key}</p>
                        <p className="mt-0.5 text-micro text-[var(--text-muted)]">
                          {row.type} · TTL {formatTTL(row.ttlSeconds)} · {formatSize(row.sizeBytes)}
                        </p>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
            {nextCursor > 0 && (
              <div className="border-t border-[var(--border)] p-2">
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => void loadEntries(false, cursor)}
                  className="w-full rounded-md border border-[var(--border)] py-1.5 text-cell hover:bg-[var(--surface-elevated)]"
                >
                  더 불러오기
                </button>
              </div>
            )}
          </section>

          <section className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-3">
            <h2 className="text-section font-semibold text-[var(--text-primary)]">값 편집</h2>
            {!detail ? (
              <p className="mt-4 text-cell text-[var(--text-muted)]">왼쪽에서 키를 선택하세요.</p>
            ) : (
              <div className="mt-3 space-y-3">
                <p className="break-all font-code text-cell text-[var(--text-secondary)]">{detail.key}</p>
                <div className="flex flex-wrap gap-2 text-micro text-[var(--text-muted)]">
                  <span>type: {detail.type}</span>
                  <span>TTL: {formatTTL(detail.ttlSeconds)}</span>
                  <span>format: {detail.valueFormat}</span>
                  {detail.readOnly && <span className="text-[var(--loss)]">읽기 전용</span>}
                </div>
                <label className="block text-micro text-[var(--text-muted)]">
                  TTL (초, 비우면 기존 유지)
                  <input
                    value={ttlDraft}
                    onChange={(e) => setTtlDraft(e.target.value)}
                    disabled={detail.readOnly}
                    placeholder="3600"
                    className="mt-1 w-full rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 py-1.5 font-code text-cell disabled:opacity-50"
                  />
                </label>
                <label className="block text-micro text-[var(--text-muted)]">
                  값
                  <textarea
                    value={valueDraft}
                    onChange={(e) => setValueDraft(e.target.value)}
                    readOnly={detail.readOnly}
                    rows={16}
                    spellCheck={false}
                    className="mt-1 w-full resize-y rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-2 py-1.5 font-code text-cell leading-relaxed text-[var(--text-primary)] disabled:opacity-70"
                  />
                </label>
                <div className="flex gap-2">
                  <button
                    type="button"
                    disabled={busy || detail.readOnly}
                    onClick={() => void handleSave()}
                    className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-white disabled:opacity-50")}
                  >
                    <Save className="h-3.5 w-3.5" />
                    저장
                  </button>
                  <button
                    type="button"
                    disabled={busy || detail.readOnly}
                    onClick={() => void handleDelete()}
                    className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--loss)]/50 text-[var(--loss)] disabled:opacity-50")}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                    삭제
                  </button>
                </div>
              </div>
            )}
          </section>
        </div>

        <p className="text-micro text-[var(--text-muted)]">
          `toss:oauth:*` 토큰 키는 보안상 조회·수정할 수 없습니다. `apibudget:*`는 읽기 전용입니다.
        </p>
      </PageScrollShell>
    </>
  );
}
