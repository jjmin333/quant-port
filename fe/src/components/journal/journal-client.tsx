"use client";

import { useCallback, useEffect, useState } from "react";
import { PageHeader } from "@/components/layout/header";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { formatDateKST } from "@/lib/format";

const STORAGE_KEY = "quant-port-journal";

type JournalEntry = {
  id: string;
  title: string;
  body: string;
  createdAt: string;
};

function loadEntries(): JournalEntry[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as JournalEntry[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function saveEntries(entries: JournalEntry[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(entries));
}

export function JournalClient() {
  const [entries, setEntries] = useState<JournalEntry[]>([]);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setEntries(loadEntries());
    setMounted(true);
  }, []);

  const addEntry = useCallback(() => {
    const trimmedTitle = title.trim();
    const trimmedBody = body.trim();
    if (!trimmedTitle && !trimmedBody) return;

    const entry: JournalEntry = {
      id: crypto.randomUUID(),
      title: trimmedTitle || "무제",
      body: trimmedBody,
      createdAt: new Date().toISOString(),
    };

    const next = [entry, ...entries];
    setEntries(next);
    saveEntries(next);
    setTitle("");
    setBody("");
  }, [title, body, entries]);

  const removeEntry = useCallback(
    (id: string) => {
      const next = entries.filter((e) => e.id !== id);
      setEntries(next);
      saveEntries(next);
    },
    [entries],
  );

  return (
    <>
      <PageHeader title="AI 투자일지" subtitle="매매 기록 · 투자 메모" />
      <main className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto min-w-[var(--dashboard-content-min)] max-w-3xl space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>새 일지 작성</CardTitle>
            </CardHeader>
            <div className="space-y-3">
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="제목"
                className="h-9 w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--accent)] focus:outline-none"
              />
              <textarea
                value={body}
                onChange={(e) => setBody(e.target.value)}
                placeholder="투자 메모, 매매 근거, 시장 관찰…"
                rows={4}
                className="w-full resize-y rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--accent)] focus:outline-none"
              />
              <button
                type="button"
                onClick={addEntry}
                className="rounded-lg bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white hover:opacity-90"
              >
                저장
              </button>
            </div>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>일지 목록</CardTitle>
              {mounted && <Badge variant="info">{entries.length}건</Badge>}
            </CardHeader>

            {!mounted ? (
              <div className="h-24 animate-pulse rounded-lg bg-[var(--surface-elevated)]" />
            ) : entries.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">작성된 일지가 없습니다.</p>
            ) : (
              <ul className="space-y-3">
                {entries.map((entry) => (
                  <li
                    key={entry.id}
                    className="rounded-lg bg-[var(--surface-elevated)] px-4 py-3"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="font-medium text-[var(--text-primary)]">{entry.title}</p>
                        <time className="text-xs text-[var(--text-muted)]">
                          {formatDateKST(entry.createdAt)}
                        </time>
                      </div>
                      <button
                        type="button"
                        onClick={() => removeEntry(entry.id)}
                        className="shrink-0 text-xs text-[var(--text-muted)] hover:text-rose-400"
                      >
                        삭제
                      </button>
                    </div>
                    {entry.body && (
                      <p className="mt-2 whitespace-pre-wrap text-sm text-[var(--text-secondary)]">
                        {entry.body}
                      </p>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>
      </main>
    </>
  );
}
