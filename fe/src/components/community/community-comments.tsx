"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Flag, Heart, Loader2, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { AuthorLine, PostText, communityErrorText } from "@/components/community/community-bits";
import { useConfirm } from "@/components/providers/confirm-provider";
import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import {
  addCommunityComment,
  deleteCommunityComment,
  fetchCommunityComments,
  reportCommunity,
  toggleCommunityCommentLike,
} from "@/lib/community-api";
import { COMMUNITY_LIMITS, type CommunityComment, type CommunitySymbol } from "@/lib/community-types";
import { toUserMessage } from "@/lib/error-message";
import { formatRelativeTime } from "@/lib/format";
import { cn } from "@/lib/utils";

/**
 * 댓글 — 답글은 한 단계만(서버가 답글의 답글을 원 댓글에 단다). 지운 댓글도 답글이 있으면
 * 「삭제된 댓글입니다」 자리로 남아 스레드가 끊기지 않는다.
 */
export function CommunityComments({
  postId,
  loggedIn,
  symbols,
  onCountChange,
}: {
  postId: number;
  loggedIn: boolean;
  symbols: CommunitySymbol[];
  onCountChange?: (delta: number) => void;
}) {
  const confirm = useConfirm();
  const { session } = useAuth();
  const isAdmin = isAdminSession(session);
  const [comments, setComments] = useState<CommunityComment[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [replyTo, setReplyTo] = useState<number | null>(null);
  const [replyDraft, setReplyDraft] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setComments(await fetchCommunityComments(postId));
      setError(null);
    } catch (err) {
      setError(toUserMessage(err, "댓글을 불러오지 못했습니다."));
      setComments([]);
    }
  }, [postId]);

  useEffect(() => {
    let alive = true;
    Promise.resolve().then(() => {
      if (alive) void load();
    });
    return () => {
      alive = false;
    };
  }, [load, loggedIn]);

  const threads = useMemo(() => {
    const roots = (comments ?? []).filter((c) => c.parentId == null);
    const replies = new Map<number, CommunityComment[]>();
    for (const c of comments ?? []) {
      if (c.parentId != null) replies.set(c.parentId, [...(replies.get(c.parentId) ?? []), c]);
    }
    return roots.map((r) => ({ root: r, replies: replies.get(r.id) ?? [] }));
  }, [comments]);

  async function submit(body: string, parentId?: number) {
    const text = body.trim();
    if (!text) return;
    setBusy(true);
    try {
      await addCommunityComment(postId, text, parentId);
      if (parentId) {
        setReplyDraft("");
        setReplyTo(null);
      } else {
        setDraft("");
      }
      onCountChange?.(1);
      await load();
    } catch (err) {
      toast.error(communityErrorText(err) ?? toUserMessage(err, "댓글을 올리지 못했습니다."));
    } finally {
      setBusy(false);
    }
  }

  async function like(c: CommunityComment) {
    if (!loggedIn) {
      toast.message("좋아요는 로그인 후 누를 수 있습니다.");
      return;
    }
    try {
      const res = await toggleCommunityCommentLike(c.id);
      setComments((prev) =>
        (prev ?? []).map((x) => (x.id === c.id ? { ...x, liked: res.liked, likeCount: res.likeCount } : x)),
      );
    } catch (err) {
      toast.error(toUserMessage(err, "처리하지 못했습니다."));
    }
  }

  async function remove(c: CommunityComment) {
    const ok = await confirm({ title: "이 댓글을 지울까요?", confirmLabel: "삭제", tone: "danger" });
    if (!ok) return;
    try {
      await deleteCommunityComment(c.id);
      onCountChange?.(-1);
      await load();
    } catch (err) {
      toast.error(communityErrorText(err) ?? toUserMessage(err, "지우지 못했습니다."));
    }
  }

  async function report(c: CommunityComment) {
    if (!loggedIn) {
      toast.message("신고는 로그인 후 할 수 있습니다.");
      return;
    }
    const ok = await confirm({
      title: "이 댓글을 신고할까요?",
      description: "서로 다른 회원 3명이 신고하면 운영자 확인 전까지 숨겨집니다.",
      confirmLabel: "신고",
      tone: "danger",
    });
    if (!ok) return;
    try {
      await reportCommunity("comment", c.id, "");
      toast.success("신고가 접수됐습니다.");
    } catch (err) {
      toast.error(toUserMessage(err, "신고하지 못했습니다."));
    }
  }

  const renderComment = (c: CommunityComment, isReply: boolean) => {
    const removed = c.status !== "visible";
    return (
      <div key={c.id} className={cn("grid min-w-0 grid-cols-[minmax(0,1fr)] gap-1 py-2.5", isReply && "pl-6")}>
        {removed ? (
          <p className="text-cell italic text-[var(--text-muted)]">
            {c.status === "hidden" ? "신고로 숨겨진 댓글입니다." : "삭제된 댓글입니다."}
          </p>
        ) : (
          <>
            <div className="flex min-w-0 items-center gap-2 text-cell">
              {isReply && <span className="text-[var(--text-muted)]">↳</span>}
              <AuthorLine author={c.author} symbols={symbols} />
              <span className="shrink-0 text-micro text-[var(--text-muted)]">{formatRelativeTime(c.createdAt)}</span>
            </div>
            <PostText text={c.body} className="text-note leading-relaxed text-[var(--text-primary)]" />
            <div className="flex items-center gap-3 text-micro text-[var(--text-muted)]">
              <button type="button" onClick={() => void like(c)} className="inline-flex items-center gap-1 hover:text-rose-500">
                <Heart className={cn("h-3 w-3", c.liked && "fill-rose-500 text-rose-500")} />
                {c.likeCount}
              </button>
              {!isReply && loggedIn && (
                <button
                  type="button"
                  onClick={() => {
                    setReplyTo(replyTo === c.id ? null : c.id);
                    setReplyDraft("");
                  }}
                  className="hover:text-[var(--text-secondary)]"
                >
                  답글쓰기
                </button>
              )}
              {c.mine || isAdmin ? (
                <button type="button" onClick={() => void remove(c)} className="inline-flex items-center gap-0.5 hover:text-rose-500">
                  <Trash2 className="h-3 w-3" />
                  삭제
                </button>
              ) : (
                <button type="button" onClick={() => void report(c)} className="inline-flex items-center gap-0.5 hover:text-[var(--text-secondary)]">
                  <Flag className="h-3 w-3" />
                  신고
                </button>
              )}
            </div>
          </>
        )}
      </div>
    );
  };

  return (
    <section className="space-y-3">
      <h2 className="text-section font-bold text-[var(--text-primary)]">
        댓글 {comments ? comments.filter((c) => c.status === "visible").length : ""}
      </h2>

      {loggedIn ? (
        <Composer
          value={draft}
          onChange={setDraft}
          onSubmit={() => void submit(draft)}
          busy={busy}
          placeholder="의견을 남겨 보세요"
        />
      ) : (
        <p className="rounded-xl border border-[var(--border)] bg-[var(--surface-elevated)]/50 px-4 py-3 text-cell text-[var(--text-muted)]">
          댓글은{" "}
          <Link href={`/login?next=${encodeURIComponent(`/community/${postId}`)}`} className="text-[var(--brand)] hover:underline">
            로그인
          </Link>{" "}
          후 쓸 수 있습니다.
        </p>
      )}

      {error && <p className="text-cell text-[var(--loss)]">{error}</p>}
      {comments === null ? (
        <p className="flex items-center gap-2 text-cell text-[var(--text-muted)]">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          댓글 불러오는 중…
        </p>
      ) : threads.length === 0 ? (
        <p className="text-cell text-[var(--text-muted)]">첫 댓글을 남겨 보세요.</p>
      ) : (
        <div className="divide-y divide-[var(--border)]">
          {threads.map(({ root, replies }) => (
            <div key={root.id}>
              {renderComment(root, false)}
              {replies.map((r) => renderComment(r, true))}
              {replyTo === root.id && (
                <div className="pb-3 pl-6">
                  <Composer
                    value={replyDraft}
                    onChange={setReplyDraft}
                    onSubmit={() => void submit(replyDraft, root.id)}
                    busy={busy}
                    placeholder="답글을 남겨 보세요"
                    autoFocus
                  />
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function Composer({
  value,
  onChange,
  onSubmit,
  busy,
  placeholder,
  autoFocus,
}: {
  value: string;
  onChange: (v: string) => void;
  onSubmit: () => void;
  busy: boolean;
  placeholder: string;
  autoFocus?: boolean;
}) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface-elevated)]/40 p-2.5">
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        maxLength={COMMUNITY_LIMITS.commentMax}
        rows={2}
        autoFocus={autoFocus}
        className="w-full resize-y bg-transparent text-note text-[var(--text-primary)] outline-none placeholder:text-[var(--text-muted)]"
      />
      <div className="flex items-center justify-between">
        <span className="text-micro text-[var(--text-muted)]">
          {[...value].length}/{COMMUNITY_LIMITS.commentMax}
        </span>
        <button
          type="button"
          disabled={busy || value.trim() === ""}
          onClick={onSubmit}
          className="rounded-lg bg-[var(--brand)] px-3 py-1.5 text-cell font-semibold text-[var(--brand-foreground)] hover:opacity-90 disabled:opacity-40"
        >
          {busy ? "등록 중…" : "등록"}
        </button>
      </div>
    </div>
  );
}
