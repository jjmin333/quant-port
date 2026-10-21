"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2, Plus, RefreshCw, Save, Search, Trash2, X } from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import {
  createAdminDBRow,
  deleteAdminDBRow,
  fetchAdminDBRows,
  fetchAdminDBTable,
  fetchAdminDBTables,
  updateAdminDBRow,
  type DBRow,
  type DBRowPage,
  type DBTableDetail,
  type DBTableSummary,
} from "@/lib/admin";
import { cn } from "@/lib/utils";
import { useConfirm } from "@/components/providers/confirm-provider";
import { toUserMessage } from "@/lib/error-message";

const PAGE_SIZE = 25;

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "—";
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

function formatRows(n: number): string {
  if (!n) return "0";
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
  return String(n);
}

const ACCESS_BADGE: Record<string, { label: string; className: string }> = {
  full: { label: "편집 가능", className: "bg-[var(--brand-muted)] text-[var(--brand)]" },
  "read-only": { label: "읽기 전용", className: "bg-[var(--surface-elevated)] text-[var(--text-muted)]" },
  redacted: { label: "민감 · 마스킹", className: "bg-[rgba(239,68,68,0.12)] text-[#EF4444]" },
};

/** Truncated cell text; JSON/長문 컬럼이 표를 밀어내지 않도록 제한합니다. */
function cellText(value: string | null): string {
  if (value === null) return "NULL";
  if (value.length <= 120) return value;
  return `${value.slice(0, 120)}…`;
}

type EditorState = {
  mode: "create" | "edit";
  original: DBRow | null;
  draft: Record<string, string | null>;
};

export function AdminDBPanel() {
  const confirm = useConfirm();
  const router = useRouter();
  const { session, loading } = useAuth();

  const [tables, setTables] = useState<DBTableSummary[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [detail, setDetail] = useState<DBTableDetail | null>(null);
  const [page, setPage] = useState<DBRowPage | null>(null);
  const [offset, setOffset] = useState(0);
  const [searchInput, setSearchInput] = useState("");
  const [searchTerm, setSearchTerm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [editor, setEditor] = useState<EditorState | null>(null);

  const isAdmin = isAdminSession(session);

  useEffect(() => {
    if (!loading && !isAdmin) router.replace("/");
  }, [loading, isAdmin, router]);

  const loadTables = useCallback(async () => {
    try {
      const list = await fetchAdminDBTables();
      setTables(list);
      setSelected((prev) => prev ?? list.find((t) => t.exists)?.name ?? null);
    } catch (e) {
      setError(toUserMessage(e, "테이블 목록 조회 실패"));
    }
  }, []);

  useEffect(() => {
    if (isAdmin) void loadTables();
  }, [isAdmin, loadTables]);

  const loadRows = useCallback(
    async (table: string, nextOffset: number, term: string) => {
      setBusy(true);
      setError(null);
      try {
        const [d, p] = await Promise.all([
          fetchAdminDBTable(table),
          fetchAdminDBRows({ table, limit: PAGE_SIZE, offset: nextOffset, q: term }),
        ]);
        setDetail(d);
        setPage(p);
        setOffset(nextOffset);
      } catch (e) {
        setError(toUserMessage(e, "행 조회 실패"));
        setPage(null);
      } finally {
        setBusy(false);
      }
    },
    [],
  );

  useEffect(() => {
    if (!selected) return;
    setEditor(null);
    void loadRows(selected, 0, searchTerm);
  }, [selected, searchTerm, loadRows]);

  const grouped = useMemo(() => {
    const map = new Map<string, DBTableSummary[]>();
    for (const t of tables) {
      const list = map.get(t.groupLabel) ?? [];
      list.push(t);
      map.set(t.groupLabel, list);
    }
    return [...map.entries()];
  }, [tables]);

  const editableColumns = useMemo(
    () => (detail?.columns ?? []).filter((c) => !c.redacted),
    [detail],
  );

  const canWrite = detail?.access === "full";

  function primaryKeyOf(row: DBRow): Record<string, unknown> {
    const key: Record<string, unknown> = {};
    for (const col of detail?.primaryKey ?? []) key[col] = row[col];
    return key;
  }

  function openCreate() {
    const draft: Record<string, string | null> = {};
    for (const c of editableColumns) {
      if (c.readOnly) continue;
      draft[c.name] = "";
    }
    setEditor({ mode: "create", original: null, draft });
  }

  function openEdit(row: DBRow) {
    setEditor({ mode: "edit", original: row, draft: { ...row } });
  }

  async function submitEditor() {
    if (!editor || !detail) return;
    setBusy(true);
    setError(null);
    try {
      if (editor.mode === "create") {
        const values: Record<string, unknown> = {};
        for (const [k, v] of Object.entries(editor.draft)) {
          if (v === "") continue; // let the column default apply
          values[k] = v;
        }
        await createAdminDBRow(detail.name, values);
        setNotice("행이 추가되었습니다.");
      } else if (editor.original) {
        // Send only the columns the operator actually changed.
        const values: Record<string, unknown> = {};
        for (const c of editableColumns) {
          if (c.readOnly || c.isPrimary) continue;
          const before = editor.original[c.name];
          const after = editor.draft[c.name];
          if (before !== after) values[c.name] = after;
        }
        if (Object.keys(values).length === 0) {
          setNotice("변경된 값이 없습니다.");
          setEditor(null);
          return;
        }
        await updateAdminDBRow(detail.name, primaryKeyOf(editor.original), values);
        setNotice("행이 수정되었습니다.");
      }
      setEditor(null);
      await loadRows(detail.name, offset, searchTerm);
    } catch (e) {
      setError(toUserMessage(e, "저장 실패"));
    } finally {
      setBusy(false);
    }
  }

  async function removeRow(row: DBRow) {
    if (!detail) return;
    const label = (detail.primaryKey ?? []).map((k) => `${k}=${row[k]}`).join(", ");
    if (
      !(await confirm({
        title: "이 행을 삭제할까요?",
        description: (
          <>
            <p>
              {detail.name} ({label})
            </p>
            <p className="text-[var(--text-muted)]">되돌릴 수 없습니다.</p>
          </>
        ),
        confirmLabel: "삭제",
        tone: "danger",
      }))
    ) {
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await deleteAdminDBRow(detail.name, primaryKeyOf(row));
      setNotice("행이 삭제되었습니다.");
      await loadRows(detail.name, offset, searchTerm);
    } catch (e) {
      setError(toUserMessage(e, "삭제 실패"));
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return (
      <>
        <PageHeader title="데이터베이스 관리" />
        <PageScrollShell>
          <div className="flex items-center gap-2 text-sm text-[var(--text-muted)]">
            <Loader2 className="h-4 w-4 animate-spin" /> 불러오는 중…
          </div>
        </PageScrollShell>
      </>
    );
  }
  if (!isAdmin) return null;

  return (
    <>
      <PageHeader title="데이터베이스 관리" subtitle="테이블 용도 확인 및 행 관리 (관리자 전용)" />
      <PageScrollShell>
        <div className="space-y-4">
          {error && (
            <div className="rounded-lg border border-[rgba(239,68,68,0.4)] bg-[rgba(239,68,68,0.08)] px-3 py-2 text-cell text-[#EF4444]">
              {error}
            </div>
          )}
          {notice && (
            <div className="flex items-center justify-between rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-cell text-[var(--text-secondary)]">
              <span>{notice}</span>
              <button onClick={() => setNotice(null)} aria-label="닫기">
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          )}

          <div className="grid gap-4 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)]">
            {/* ---- table catalog ---- */}
            <aside className="space-y-3">
              {grouped.map(([groupLabel, list]) => (
                <div key={groupLabel} className="rounded-lg border border-[var(--border)] bg-[var(--surface)]">
                  <div className="border-b border-[var(--border)] px-3 py-2 text-cell font-semibold uppercase tracking-wide text-[var(--text-muted)]">
                    {groupLabel}
                  </div>
                  <ul className="p-1">
                    {list.map((t) => (
                      <li key={t.name}>
                        <button
                          onClick={() => setSelected(t.name)}
                          disabled={!t.exists}
                          className={cn(
                            "flex w-full flex-col gap-0.5 rounded px-2 py-1.5 text-left transition-colors",
                            selected === t.name
                              ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                              : "text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
                            !t.exists && "cursor-not-allowed opacity-40",
                          )}
                        >
                          <span className="truncate text-cell font-medium">{t.label}</span>
                          <span className="truncate font-code text-micro text-[var(--text-muted)]">
                            {t.name} · {formatRows(t.estimatedRows)}행
                          </span>
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </aside>

            {/* ---- selected table ---- */}
            <section className="min-w-0 space-y-3">
              {detail && (
                <div className="space-y-2 rounded-lg border border-[var(--border)] bg-[var(--surface)] p-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="text-section font-semibold text-[var(--text-primary)]">{detail.label}</h2>
                    <code className="font-code text-cell text-[var(--text-muted)]">{detail.name}</code>
                    <span
                      className={cn(
                        "rounded px-1.5 py-0.5 text-micro font-medium",
                        ACCESS_BADGE[detail.access]?.className,
                      )}
                    >
                      {ACCESS_BADGE[detail.access]?.label}
                    </span>
                  </div>
                  <p className="text-cell leading-relaxed text-[var(--text-secondary)]">{detail.purpose}</p>
                  {detail.note && (
                    <p className="rounded border border-[var(--border)] bg-[var(--surface-elevated)] px-2 py-1.5 text-cell text-[var(--text-muted)]">
                      ⚠ {detail.note}
                    </p>
                  )}
                  <dl className="flex flex-wrap gap-x-4 gap-y-1 text-cell text-[var(--text-muted)]">
                    <div>PK: <span className="font-code">{detail.primaryKey.join(", ") || "—"}</span></div>
                    <div>행: {formatRows(detail.estimatedRows)}</div>
                    <div>데이터: {formatBytes(detail.dataBytes)}</div>
                    <div>인덱스: {formatBytes(detail.indexBytes)}</div>
                  </dl>
                </div>
              )}

              {/* ---- toolbar ---- */}
              <div className="flex flex-wrap items-center gap-2">
                <div className="relative min-w-0 flex-1">
                  <Search className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--text-muted)]" />
                  <input
                    value={searchInput}
                    onChange={(e) => setSearchInput(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && setSearchTerm(searchInput)}
                    placeholder="텍스트 컬럼 검색 후 Enter"
                    className="w-full rounded border border-[var(--border)] bg-[var(--surface)] py-1.5 pl-7 pr-2 text-cell text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
                  />
                </div>
                <button
                  onClick={() => selected && loadRows(selected, offset, searchTerm)}
                  className="flex items-center gap-1 rounded border border-[var(--border)] px-2 py-1.5 text-cell text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
                >
                  <RefreshCw className={cn("h-3.5 w-3.5", busy && "animate-spin")} /> 새로고침
                </button>
                {canWrite && (
                  <button
                    onClick={openCreate}
                    className="flex items-center gap-1 rounded bg-[var(--brand)] px-2 py-1.5 text-cell font-medium text-[var(--brand-foreground)]"
                  >
                    <Plus className="h-3.5 w-3.5" /> 행 추가
                  </button>
                )}
              </div>

              {/* ---- rows ---- */}
              <div className="overflow-x-auto rounded-lg border border-[var(--border)] bg-[var(--surface)]">
                <table className="w-full min-w-max text-left text-cell">
                  <thead className="border-b border-[var(--border)] text-[var(--text-muted)]">
                    <tr>
                      {canWrite && <th className="w-20 px-2 py-1.5" />}
                      {(page?.columns ?? []).map((c) => (
                        <th key={c.name} className="whitespace-nowrap px-2 py-1.5 font-medium">
                          {c.name}
                          {c.isPrimary && <span className="ml-1 text-[var(--brand)]">PK</span>}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {(page?.rows ?? []).map((row, i) => (
                      <tr key={i} className="border-b border-[var(--border)] last:border-0">
                        {canWrite && (
                          <td className="whitespace-nowrap px-2 py-1">
                            <div className="flex gap-1">
                              <button
                                onClick={() => openEdit(row)}
                                className="rounded p-1 text-[var(--text-muted)] hover:text-[var(--brand)]"
                                aria-label="수정"
                              >
                                <Save className="h-3.5 w-3.5" />
                              </button>
                              <button
                                onClick={() => removeRow(row)}
                                className="rounded p-1 text-[var(--text-muted)] hover:text-[#EF4444]"
                                aria-label="삭제"
                              >
                                <Trash2 className="h-3.5 w-3.5" />
                              </button>
                            </div>
                          </td>
                        )}
                        {(page?.columns ?? []).map((c) => (
                          <td
                            key={c.name}
                            className={cn(
                              "max-w-[24rem] truncate px-2 py-1 font-code",
                              row[c.name] === null ? "text-[var(--text-muted)] italic" : "text-[var(--text-secondary)]",
                            )}
                            title={row[c.name] ?? "NULL"}
                          >
                            {cellText(row[c.name])}
                          </td>
                        ))}
                      </tr>
                    ))}
                    {!busy && (page?.rows.length ?? 0) === 0 && (
                      <tr>
                        <td
                          colSpan={(page?.columns.length ?? 1) + (canWrite ? 1 : 0)}
                          className="px-3 py-6 text-center text-cell text-[var(--text-muted)]"
                        >
                          표시할 행이 없습니다.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>

              {/* ---- pagination ---- */}
              {page && page.total > PAGE_SIZE && (
                <div className="flex items-center justify-between text-cell text-[var(--text-muted)]">
                  <span>
                    {offset + 1}–{Math.min(offset + PAGE_SIZE, page.total)} / {page.total.toLocaleString()}
                  </span>
                  <div className="flex gap-2">
                    <button
                      disabled={offset === 0 || busy}
                      onClick={() => selected && loadRows(selected, Math.max(0, offset - PAGE_SIZE), searchTerm)}
                      className="rounded border border-[var(--border)] px-2 py-1 disabled:opacity-40"
                    >
                      이전
                    </button>
                    <button
                      disabled={offset + PAGE_SIZE >= page.total || busy}
                      onClick={() => selected && loadRows(selected, offset + PAGE_SIZE, searchTerm)}
                      className="rounded border border-[var(--border)] px-2 py-1 disabled:opacity-40"
                    >
                      다음
                    </button>
                  </div>
                </div>
              )}
            </section>
          </div>
        </div>

        {/* ---- editor ---- */}
        {editor && detail && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
            <div className="max-h-[85vh] w-full max-w-lg overflow-y-auto rounded-lg border border-[var(--border)] bg-[var(--surface)] p-4">
              <div className="mb-3 flex items-center justify-between">
                <h3 className="text-sm font-semibold text-[var(--text-primary)]">
                  {editor.mode === "create" ? "행 추가" : "행 수정"} · {detail.label}
                </h3>
                <button onClick={() => setEditor(null)} aria-label="닫기">
                  <X className="h-4 w-4 text-[var(--text-muted)]" />
                </button>
              </div>

              <div className="space-y-2.5">
                {editableColumns.map((c) => {
                  const locked =
                    c.readOnly || (editor.mode === "edit" && c.isPrimary);
                  return (
                    <label key={c.name} className="block">
                      <span className="mb-0.5 flex items-baseline gap-1.5 text-cell text-[var(--text-secondary)]">
                        <span className="font-code">{c.name}</span>
                        <span className="text-micro text-[var(--text-muted)]">{c.columnType}</span>
                        {!c.nullable && <span className="text-micro text-[#EF4444]">필수</span>}
                        {locked && <span className="text-micro text-[var(--text-muted)]">자동/PK</span>}
                      </span>
                      <input
                        value={editor.draft[c.name] ?? ""}
                        disabled={locked}
                        onChange={(e) =>
                          setEditor((prev) =>
                            prev ? { ...prev, draft: { ...prev.draft, [c.name]: e.target.value } } : prev,
                          )
                        }
                        placeholder={c.nullable ? "비우면 NULL" : ""}
                        className="w-full rounded border border-[var(--border)] bg-[var(--background)] px-2 py-1.5 font-code text-cell text-[var(--text-primary)] outline-none focus:border-[var(--brand)] disabled:opacity-50"
                      />
                    </label>
                  );
                })}
              </div>

              <div className="mt-4 flex justify-end gap-2">
                <button
                  onClick={() => setEditor(null)}
                  className="rounded border border-[var(--border)] px-3 py-1.5 text-cell text-[var(--text-secondary)]"
                >
                  취소
                </button>
                <button
                  onClick={submitEditor}
                  disabled={busy}
                  className="flex items-center gap-1 rounded bg-[var(--brand)] px-3 py-1.5 text-cell font-medium text-[var(--brand-foreground)] disabled:opacity-50"
                >
                  {busy && <Loader2 className="h-3.5 w-3.5 animate-spin" />} 저장
                </button>
              </div>
            </div>
          </div>
        )}
      </PageScrollShell>
    </>
  );
}
