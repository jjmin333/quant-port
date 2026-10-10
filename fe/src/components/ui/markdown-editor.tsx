"use client";

import { useEffect, useRef, useState } from "react";
import {
  Bold,
  Code,
  Eye,
  Heading,
  Image as ImageIcon,
  Italic,
  Link2,
  List,
  ListOrdered,
  Loader2,
  Minus,
  PencilLine,
  Quote,
  Table,
} from "lucide-react";
import { Markdown } from "@/components/ui/markdown";
import { useMentionAssist } from "@/components/mentions/mention-assist";
import { MentionMarkdownPreview } from "@/components/mentions/mention-markdown";
import { useWideCompose } from "@/components/mentions/compose-side-preview";
import {
  MEDIA_IMAGE_ACCEPT,
  isUploadableImage,
  uploadImageClient,
  type MediaUpload,
} from "@/lib/media-api";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";

/**
 * The report body editor.
 *
 * Two things decide whether people write here: not facing raw syntax they have
 * to remember, and seeing what the post will look like before publishing. So the
 * toolbar writes the Markdown for them and 미리보기 renders with exactly the
 * same component the published page uses — no second renderer to drift.
 *
 * Images go through `uploadImage`, which defaults to the app-wide media client.
 * The editor never talks to a storage backend itself, so the day uploads move
 * to an object store, nothing in this file changes.
 */

type Action =
  | { kind: "wrap"; before: string; after: string; sample: string }
  | { kind: "line"; prefix: string; sample: string }
  | { kind: "block"; text: string };

type ToolbarItem = {
  id: string;
  label: string;
  icon: typeof Bold;
  action: Action;
};

const TOOLBAR: ToolbarItem[] = [
  { id: "h", label: "제목", icon: Heading, action: { kind: "line", prefix: "## ", sample: "제목" } },
  { id: "b", label: "굵게", icon: Bold, action: { kind: "wrap", before: "**", after: "**", sample: "굵은 글씨" } },
  { id: "i", label: "기울임", icon: Italic, action: { kind: "wrap", before: "*", after: "*", sample: "기울임" } },
  { id: "ul", label: "목록", icon: List, action: { kind: "line", prefix: "- ", sample: "항목" } },
  { id: "ol", label: "번호 목록", icon: ListOrdered, action: { kind: "line", prefix: "1. ", sample: "항목" } },
  { id: "q", label: "인용", icon: Quote, action: { kind: "line", prefix: "> ", sample: "인용문" } },
  { id: "a", label: "링크", icon: Link2, action: { kind: "wrap", before: "[", after: "](https://)", sample: "링크 텍스트" } },
  { id: "c", label: "코드", icon: Code, action: { kind: "wrap", before: "`", after: "`", sample: "코드" } },
  { id: "hr", label: "구분선", icon: Minus, action: { kind: "block", text: "\n---\n" } },
  {
    id: "t",
    label: "표",
    icon: Table,
    action: {
      kind: "block",
      text: "\n| 구분 | 값 |\n| --- | --- |\n| 항목 | 내용 |\n",
    },
  },
];

export function MarkdownEditor({
  value,
  onChange,
  placeholder,
  rows = 16,
  disabled,
  className,
  hint,
  clearOnFirstEdit,
  uploadImage = uploadImageClient,
  mentions = false,
  sidePreview = false,
}: {
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  rows?: number;
  disabled?: boolean;
  className?: string;
  /** Small caption under the editor. Defaults to the Markdown cheat sheet. */
  hint?: React.ReactNode;
  /**
   * Suggested text that is guidance, not content: while the body is still
   * exactly this, the first click in the editor clears it. Nobody should have
   * to delete a placeholder before they can start writing.
   */
  clearOnFirstEdit?: string;
  /** Injected so a caller can swap the storage backend or stub it in tests. */
  uploadImage?: (file: File) => Promise<MediaUpload>;
  /** 데이터 태그(@·#) 자동완성·도우미·미리보기를 켠다 — 리포트 본문(2026-09-28 요청 2). */
  mentions?: boolean;
  /** 페이지가 넓은 화면에서 옆 미리보기를 띄운다 — 그 폭에선 작성/미리보기 탭을 숨긴다(2026-10-01 요청 4). */
  sidePreview?: boolean;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const [tabState, setTab] = useState<"write" | "preview">("write");
  const wide = useWideCompose();
  const tab = sidePreview && wide ? "write" : tabState;
  const [uploading, setUploading] = useState(0);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);

  const showingHint = Boolean(clearOnFirstEdit) && value === clearOnFirstEdit;
  const assist = useMentionAssist({ textareaRef: ref, value, onChange, enabled: mentions });

  // An upload takes a round trip, during which the author may keep typing and
  // the `value` captured by the handler's closure goes stale. Inserting against
  // this ref instead is what keeps a slow upload from eating the sentence
  // someone wrote while waiting for it.
  const latest = useRef(value);
  useEffect(() => {
    latest.current = value;
  }, [value]);

  /** The text edits apply to, with an untouched suggestion already dropped. */
  function baseValue(): string {
    const current = latest.current;
    if (clearOnFirstEdit && current === clearOnFirstEdit) {
      onChange("");
      return "";
    }
    return current;
  }

  /** Whether the caret is real — after clearing the hint it is not. */
  function caretIn(current: string): [number, number] {
    const el = ref.current;
    if (!el || latest.current !== current) return [current.length, current.length];
    return [Math.min(el.selectionStart, current.length), Math.min(el.selectionEnd, current.length)];
  }

  function insertAtCursor(text: string) {
    const current = baseValue();
    const [start, end] = caretIn(current);
    const next = current.slice(0, start) + text + current.slice(end);
    latest.current = next;
    onChange(next);
    const caret = start + text.length;
    requestAnimationFrame(() => {
      ref.current?.focus();
      ref.current?.setSelectionRange(caret, caret);
    });
  }

  function apply(action: Action) {
    const el = ref.current;
    const current = baseValue();
    const [start, end] = caretIn(current);
    const selected = current.slice(start, end);

    let next = current;
    let caretStart = start;
    let caretEnd = end;

    if (action.kind === "wrap") {
      const inner = selected || action.sample;
      next = current.slice(0, start) + action.before + inner + action.after + current.slice(end);
      caretStart = start + action.before.length;
      caretEnd = caretStart + inner.length;
    } else if (action.kind === "line") {
      // Prefix every selected line, so turning three lines into a list is one
      // click rather than three.
      const lineStart = current.lastIndexOf("\n", start - 1) + 1;
      const lineEnd = current.indexOf("\n", end) === -1 ? current.length : current.indexOf("\n", end);
      const target = current.slice(lineStart, lineEnd) || action.sample;
      const prefixed = target
        .split("\n")
        .map((line) => (line.startsWith(action.prefix) ? line : action.prefix + line))
        .join("\n");
      next = current.slice(0, lineStart) + prefixed + current.slice(lineEnd);
      caretStart = lineStart + prefixed.length;
      caretEnd = caretStart;
    } else {
      const padded =
        start > 0 && !current.slice(0, start).endsWith("\n") ? `\n${action.text}` : action.text;
      next = current.slice(0, start) + padded + current.slice(end);
      caretStart = start + padded.length;
      caretEnd = caretStart;
    }

    latest.current = next;
    onChange(next);
    // The value lands on the next render; move the caret after it does, or the
    // author loses their place on every toolbar click.
    requestAnimationFrame(() => {
      el?.focus();
      el?.setSelectionRange(caretStart, caretEnd);
    });
  }

  async function addImages(files: File[]) {
    const images = files.filter(isUploadableImage);
    if (images.length === 0) {
      if (files.length > 0) setUploadError("이미지 파일만 넣을 수 있습니다.");
      return;
    }
    setUploadError(null);
    setUploading((n) => n + images.length);
    try {
      for (const file of images) {
        try {
          const uploaded = await uploadImage(file);
          const alt = (file.name || "이미지").replace(/\.[^.]+$/, "").replace(/[[\]]/g, "");
          insertAtCursor(`\n![${alt}](${uploaded.url})\n`);
        } catch (err) {
          setUploadError(toUserMessage(err, "이미지 업로드에 실패했습니다."));
        } finally {
          setUploading((n) => Math.max(0, n - 1));
        }
      }
    } finally {
      setUploading(0);
    }
  }

  return (
    <div className={cn("space-y-2", className)}>
      <div className="flex items-center justify-between gap-2">
        <div className={cn("flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5", sidePreview && wide && "invisible")}>
          <TabButton active={tab === "write"} onClick={() => setTab("write")} icon={PencilLine}>
            작성
          </TabButton>
          <TabButton active={tab === "preview"} onClick={() => setTab("preview")} icon={Eye}>
            미리보기
          </TabButton>
        </div>
        <span className="shrink-0 text-micro tabular-nums text-[var(--text-muted)]">
          {showingHint ? 0 : value.length.toLocaleString()}자
        </span>
      </div>

      {tab === "write" ? (
        <>
          {/* The toolbar scrolls sideways on a phone instead of wrapping into
              a second row that pushes the writing area off screen. */}
          <div className="-mx-1 flex gap-0.5 overflow-x-auto px-1 pb-0.5">
            {TOOLBAR.map((item) => (
              <button
                key={item.id}
                type="button"
                disabled={disabled}
                onClick={() => apply(item.action)}
                title={item.label}
                aria-label={item.label}
                className="shrink-0 rounded-md p-2 text-[var(--text-muted)] transition-colors hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)] disabled:opacity-40"
              >
                <item.icon className="h-4 w-4" />
              </button>
            ))}
            <span className="mx-0.5 my-1.5 w-px shrink-0 bg-[var(--border)]" />
            <button
              type="button"
              disabled={disabled || uploading > 0}
              onClick={() => fileRef.current?.click()}
              title="사진 넣기"
              aria-label="사진 넣기"
              className="shrink-0 rounded-md p-2 text-[var(--text-muted)] transition-colors hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)] disabled:opacity-40"
            >
              {uploading > 0 ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <ImageIcon className="h-4 w-4" />
              )}
            </button>
            <input
              ref={fileRef}
              type="file"
              accept={MEDIA_IMAGE_ACCEPT}
              multiple
              hidden
              onChange={(e) => {
                addImages(Array.from(e.target.files ?? []));
                // Reset so picking the same file twice fires onChange again.
                e.target.value = "";
              }}
            />
          </div>

          {mentions && assist.tools}
          <textarea
            ref={ref}
            value={value}
            onChange={(e) => {
              latest.current = e.target.value;
              onChange(e.target.value);
              if (mentions) assist.syncCaret();
            }}
            onKeyDown={mentions ? assist.onKeyDown : undefined}
            onKeyUp={mentions ? assist.syncCaret : undefined}
            onClick={mentions ? assist.syncCaret : undefined}
            onPointerDown={() => {
              // "클릭하면 사라진다" — the guidance goes away the moment the
              // author reaches for the editor, before they type anything.
              if (showingHint) baseValue();
            }}
            onFocus={() => {
              if (showingHint) baseValue();
            }}
            onPaste={(e) => {
              const files = Array.from(e.clipboardData?.files ?? []);
              if (files.length > 0) {
                e.preventDefault();
                addImages(files);
              }
            }}
            onDragOver={(e) => {
              if (e.dataTransfer.types.includes("Files")) {
                e.preventDefault();
                setDragging(true);
              }
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(e) => {
              const files = Array.from(e.dataTransfer.files ?? []);
              if (files.length > 0) {
                e.preventDefault();
                setDragging(false);
                addImages(files);
              }
            }}
            rows={rows}
            disabled={disabled}
            placeholder={placeholder}
            className={cn(
              "w-full resize-y rounded-xl border bg-[var(--surface-elevated)] px-3.5 py-3 text-section leading-[1.85] outline-none transition-colors focus:border-[var(--brand)] disabled:opacity-60",
              dragging ? "border-[var(--brand)]" : "border-[var(--border)]",
              // The suggestion reads as a placeholder because that is what it
              // is — real content should not look like guidance.
              showingHint ? "text-[var(--text-muted)]" : "text-[var(--text-primary)]",
            )}
          />

          {mentions && assist.popup}
          {uploadError && <p className="text-cell text-[var(--loss)]">{uploadError}</p>}
        </>
      ) : (
        <div className="min-h-[12rem] rounded-xl border border-[var(--border)] bg-[var(--surface-elevated)] px-3.5 py-3">
          {value.trim() === "" || showingHint ? (
            <p className="text-sm text-[var(--text-muted)]">아직 작성된 내용이 없습니다.</p>
          ) : (
            mentions ? <MentionMarkdownPreview source={value} /> : <Markdown source={value} />
          )}
        </div>
      )}

      {hint ?? (
        <p className="text-micro leading-relaxed text-[var(--text-muted)]">
          마크다운으로 작성됩니다 — <code className="font-code">## 제목</code>,{" "}
          <code className="font-code">**굵게**</code>, <code className="font-code">- 목록</code>,{" "}
          <code className="font-code">&gt; 인용</code>. 사진은 버튼·붙여넣기·끌어놓기로 넣을 수
          있습니다.
        </p>
      )}
    </div>
  );
}

function TabButton({
  active,
  onClick,
  icon: Icon,
  children,
}: {
  active: boolean;
  onClick: () => void;
  icon: typeof Bold;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-cell font-medium transition-colors",
        active
          ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
          : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
      )}
    >
      <Icon className="h-3.5 w-3.5" />
      {children}
    </button>
  );
}
