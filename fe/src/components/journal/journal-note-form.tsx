"use client";

import { useState } from "react";
import { X } from "lucide-react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { JOURNAL_TAG_PRESETS, JOURNAL_TAG_TONE } from "@/lib/journal-types";
import type { JournalNote, JournalNoteInput } from "@/lib/journal-types";
import type { StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

/**
 * 메모 작성·수정 폼.
 *
 * 목록 위에도, 상세 모달 안에도 같은 폼이 뜬다. 예전에는 수정 폼이 목록 **맨 위**에
 * 따로 열려서, 정작 고치는 중인 행은 저 아래에 있었다.
 *
 * 가격 입력란은 없다. 작성 시점 가격은 서버가 저장하는 순간 재고, 화면은 그걸 읽기만
 * 한다 — 사용자가 적을 수 있으면 그건 박제가 아니다.
 */

const MAX_TAGS = 6;

type DraftStock = { symbol: string; name: string } | null;

export function JournalNoteForm({
  initial,
  onCancel,
  onSubmit,
  autoFocus,
  compact,
}: {
  initial?: JournalNote;
  onCancel: (() => void) | null;
  onSubmit: (input: JournalNoteInput) => Promise<void>;
  autoFocus?: boolean;
  /** 모달 안에서는 카드 껍데기 없이 내용만 그린다. */
  compact?: boolean;
}) {
  const [title, setTitle] = useState(initial?.title ?? "");
  const [body, setBody] = useState(initial?.body ?? "");
  const [tags, setTags] = useState<string[]>(initial?.tags ?? []);
  const [tagDraft, setTagDraft] = useState("");
  const [stock, setStock] = useState<DraftStock>(
    initial?.symbol ? { symbol: initial.symbol, name: initial.symbolName || initial.symbol } : null,
  );
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const toggleTag = (tag: string) => {
    setTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : prev.length >= MAX_TAGS ? prev : [...prev, tag],
    );
  };

  const commitTagDraft = () => {
    const next = tagDraft.trim();
    if (!next) return;
    setTagDraft("");
    toggleTag(next);
  };

  const handleSelect = (info: StockInfo) => setStock({ symbol: info.symbol, name: info.name });

  const handleSubmit = async () => {
    if (!body.trim()) {
      setFormError("내용을 입력해주세요.");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      await onSubmit({
        title,
        body,
        symbol: stock?.symbol,
        symbolName: stock?.name,
        tags,
        pinned: initial?.pinned,
      });
      if (!initial) {
        setTitle("");
        setBody("");
        setTags([]);
        setStock(null);
      }
    } catch {
      setFormError("저장에 실패했습니다. 다시 시도해주세요.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-3">
      <input
        type="text"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        placeholder="제목 (선택)"
        autoFocus={autoFocus}
        className="h-9 w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
      />

      {stock ? (
        <div className="flex items-center gap-2 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2">
          <StockAvatar symbol={stock.symbol} name={stock.name} size="xs" />
          <span className="text-sm text-[var(--text-primary)]">{stock.name}</span>
          <span className="text-xs text-[var(--text-muted)]">{stock.symbol}</span>
          <button
            type="button"
            onClick={() => setStock(null)}
            aria-label="종목 연동 해제"
            className="ml-auto rounded-md p-1 text-[var(--text-muted)] hover:text-rose-400"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      ) : (
        <StockCatalogSearchField
          placeholder="종목 연동 (선택) — 종목명·티커 검색"
          onSelect={handleSelect}
        />
      )}

      {/* 종목을 연결한 새 메모에만 안내한다 — 수정할 때는 이미 박제가 끝났다. */}
      {stock && !initial && (
        <p className="text-[11px] text-[var(--text-muted)]">
          저장하면 <b className="font-medium text-[var(--text-secondary)]">{stock.name}</b>의 지금
          주가가 이 메모에 함께 기록됩니다. 나중에 다시 읽을 때 그때와 비교됩니다.
        </p>
      )}

      <textarea
        value={body}
        onChange={(e) => setBody(e.target.value)}
        placeholder="투자 메모, 매매 근거, 시장 관찰…"
        rows={compact ? 6 : 4}
        className="w-full resize-y rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
      />

      <div className="space-y-1.5">
        <div className="flex flex-wrap items-center gap-1.5">
          {JOURNAL_TAG_PRESETS.map((tag) => (
            <button
              key={tag}
              type="button"
              onClick={() => toggleTag(tag)}
              aria-pressed={tags.includes(tag)}
              className={cn(
                "rounded-full px-2.5 py-1 text-[11px] font-medium transition-opacity",
                tags.includes(tag)
                  ? JOURNAL_TAG_TONE[tag]
                  : "bg-[var(--surface-elevated)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
              )}
            >
              {tag}
            </button>
          ))}
          <input
            type="text"
            value={tagDraft}
            onChange={(e) => setTagDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                commitTagDraft();
              }
            }}
            onBlur={commitTagDraft}
            placeholder="+ 태그"
            className="h-6 w-20 rounded-full border border-dashed border-[var(--border)] bg-transparent px-2.5 text-[11px] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
          />
        </div>
        {/* 직접 입력한 태그는 프리셋 줄에 안 뜨므로 여기서 지울 수 있어야 한다. */}
        {tags.filter((t) => !JOURNAL_TAG_PRESETS.includes(t as (typeof JOURNAL_TAG_PRESETS)[number])).length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5">
            {tags
              .filter((t) => !JOURNAL_TAG_PRESETS.includes(t as (typeof JOURNAL_TAG_PRESETS)[number]))
              .map((tag) => (
                <button
                  key={tag}
                  type="button"
                  onClick={() => toggleTag(tag)}
                  className="inline-flex items-center gap-1 rounded-full bg-[var(--surface-elevated)] px-2.5 py-1 text-[11px] text-[var(--text-secondary)] hover:text-rose-400"
                >
                  {tag}
                  <X className="h-3 w-3" />
                </button>
              ))}
          </div>
        )}
      </div>

      {formError && <p className="text-xs text-rose-400">{formError}</p>}

      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={handleSubmit}
          disabled={saving}
          className="rounded-lg bg-[var(--brand)] px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
        >
          {saving ? "저장 중…" : "저장"}
        </button>
        {onCancel && (
          <button
            type="button"
            onClick={onCancel}
            className="rounded-lg px-4 py-2 text-sm font-medium text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
          >
            취소
          </button>
        )}
      </div>
    </div>
  );
}
