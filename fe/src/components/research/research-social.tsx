"use client";

import { useCallback, useEffect, useState } from "react";
import { Bookmark, Heart, MessageSquare, Trash2 } from "lucide-react";
import { Card } from "@/components/ui/card";
import {
  addResearchCommentClient,
  deleteResearchCommentClient,
  fetchResearchCommentsClient,
  rateResearchClient,
  toggleResearchReactionClient,
  updateResearchCommentClient,
} from "@/lib/research-api";
import {
  RATING_AXES,
  type ResearchCall,
  type ResearchComment,
} from "@/lib/research-types";
import { formatDateTimeKST } from "@/lib/format";
import { cn } from "@/lib/utils";

/** Like / bookmark row. Optimistic, with the server state as the correction. */
export function ReactionBar({ call }: { call: ResearchCall }) {
  const [liked, setLiked] = useState(call.liked);
  const [bookmarked, setBookmarked] = useState(call.bookmarked);
  const [likes, setLikes] = useState(call.likeCount);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setLiked(call.liked);
    setBookmarked(call.bookmarked);
    setLikes(call.likeCount);
  }, [call.liked, call.bookmarked, call.likeCount]);

  async function toggle(kind: "like" | "bookmark") {
    if (busy) return;
    setBusy(true);
    const wasLiked = liked;
    const wasBookmarked = bookmarked;
    if (kind === "like") {
      setLiked(!wasLiked);
      setLikes((n) => n + (wasLiked ? -1 : 1));
    } else {
      setBookmarked(!wasBookmarked);
    }
    try {
      const active = await toggleResearchReactionClient(call.id, kind);
      if (kind === "like") {
        setLiked(active);
        setLikes((n) => n + (active === !wasLiked ? 0 : active ? 1 : -1));
      } else {
        setBookmarked(active);
      }
    } catch {
      // Roll back — the count on screen must match what the server stored.
      setLiked(wasLiked);
      setBookmarked(wasBookmarked);
      setLikes(call.likeCount);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex items-center gap-2">
      <button
        type="button"
        onClick={() => toggle("like")}
        disabled={busy}
        aria-pressed={liked}
        className={cn(
          "inline-flex items-center gap-1 rounded-lg border border-[var(--border)] px-2.5 py-1.5 text-xs transition-colors disabled:opacity-60",
          liked ? "text-rose-400" : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]",
        )}
      >
        <Heart className={cn("h-3.5 w-3.5", liked && "fill-rose-400")} />
        {likes}
      </button>
      <button
        type="button"
        onClick={() => toggle("bookmark")}
        disabled={busy}
        aria-pressed={bookmarked}
        className={cn(
          "inline-flex items-center gap-1 rounded-lg border border-[var(--border)] px-2.5 py-1.5 text-xs transition-colors disabled:opacity-60",
          bookmarked
            ? "text-amber-400"
            : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]",
        )}
      >
        <Bookmark className={cn("h-3.5 w-3.5", bookmarked && "fill-amber-400")} />
        {bookmarked ? "저장됨" : "저장"}
      </button>
      <span className="inline-flex items-center gap-1 text-xs text-[var(--text-muted)]">
        <MessageSquare className="h-3.5 w-3.5" />
        {call.commentCount}
      </span>
    </div>
  );
}

/**
 * 3-axis rating of the *argument*.
 *
 * It closes when the verdict lands: rating a settled call is re-scoring the
 * outcome, which the alpha numbers already do better. The server enforces this
 * too — the UI just explains why the form is gone.
 */
export function RatingPanel({ call, onRated }: { call: ResearchCall; onRated: () => void }) {
  const [values, setValues] = useState({
    clarity: call.myRating?.clarity ?? 3,
    evidence: call.myRating?.evidence ?? 3,
    risk: call.myRating?.risk ?? 3,
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const summary = call.rating;

  return (
    <Card className="space-y-2.5 p-4">
      <div className="flex items-baseline justify-between">
        <p className="text-[11px] font-medium text-[var(--text-muted)]">논거 평가</p>
        {summary && (
          <p className="text-sm font-semibold text-[var(--text-primary)]">
            {summary.overall.toFixed(1)}
            <span className="ml-1 text-[10px] font-normal text-[var(--text-muted)]">
              / 5 · {summary.count}명
            </span>
          </p>
        )}
      </div>

      {summary ? (
        <div className="space-y-1">
          {RATING_AXES.map((axis) => {
            const v = summary[axis.id];
            return (
              <div key={axis.id} className="flex items-center gap-2">
                <span className="w-16 shrink-0 text-[10px] text-[var(--text-muted)]">
                  {axis.label}
                </span>
                <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
                  <span
                    className="block h-full rounded-full bg-[var(--brand)]"
                    style={{ width: `${(v / 5) * 100}%` }}
                  />
                </span>
                <span className="w-6 shrink-0 text-right text-[10px] tabular-nums text-[var(--text-secondary)]">
                  {v.toFixed(1)}
                </span>
              </div>
            );
          })}
        </div>
      ) : (
        <p className="text-[11px] text-[var(--text-muted)]">아직 평가가 없습니다.</p>
      )}

      {call.canRate ? (
        <div className="space-y-2 border-t border-[var(--border)] pt-2.5">
          {RATING_AXES.map((axis) => (
            <div key={axis.id} className="flex items-center gap-2">
              <span className="w-16 shrink-0 text-[10px] text-[var(--text-muted)]">
                {axis.label}
              </span>
              <div className="flex gap-1">
                {[1, 2, 3, 4, 5].map((n) => (
                  <button
                    key={n}
                    type="button"
                    onClick={() => setValues((v) => ({ ...v, [axis.id]: n }))}
                    className={cn(
                      "h-6 w-6 rounded text-[11px] tabular-nums transition-colors",
                      values[axis.id] === n
                        ? "bg-[var(--brand)] text-white"
                        : "bg-[var(--surface-elevated)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                    )}
                  >
                    {n}
                  </button>
                ))}
              </div>
            </div>
          ))}
          <button
            type="button"
            disabled={saving}
            onClick={async () => {
              setSaving(true);
              setError(null);
              try {
                await rateResearchClient(call.id, values);
                onRated();
              } catch (err) {
                setError(err instanceof Error ? err.message : "평가에 실패했습니다.");
              } finally {
                setSaving(false);
              }
            }}
            className="w-full rounded-lg bg-[var(--brand)] py-1.5 text-xs font-medium text-white disabled:opacity-40"
          >
            {call.myRating ? "평가 수정" : "평가 남기기"}
          </button>
          {error && <p className="text-[11px] text-[var(--loss)]">{error}</p>}
        </div>
      ) : (
        <p className="border-t border-[var(--border)] pt-2 text-[10px] text-[var(--text-muted)]">
          {call.canEdit
            ? "자기 리포트는 평가할 수 없습니다."
            : call.status === "open"
              ? "평가하려면 로그인이 필요합니다."
              : "판정이 끝난 리포트는 평가할 수 없습니다 — 결과를 보고 매기면 논거 평가가 아니라 성과 재측정이 됩니다."}
        </p>
      )}
    </Card>
  );
}

/** Comment thread. Replies go exactly one level deep. */
export function CommentThread({ callId }: { callId: number }) {
  const [comments, setComments] = useState<ResearchComment[]>([]);
  const [loading, setLoading] = useState(true);
  const [body, setBody] = useState("");
  const [replyTo, setReplyTo] = useState<number | null>(null);
  const [replyBody, setReplyBody] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setComments(await fetchResearchCommentsClient(callId));
    } finally {
      setLoading(false);
    }
  }, [callId]);

  useEffect(() => {
    load();
  }, [load]);

  async function submit(text: string, parentId?: number) {
    const trimmed = text.trim();
    if (!trimmed || busy) return;
    setBusy(true);
    try {
      await addResearchCommentClient(callId, trimmed, parentId);
      setBody("");
      setReplyBody("");
      setReplyTo(null);
      await load();
    } finally {
      setBusy(false);
    }
  }

  const total = comments.reduce((n, c) => n + 1 + (c.replies?.length ?? 0), 0);

  return (
    <Card className="space-y-3 p-4">
      <p className="text-[11px] font-medium text-[var(--text-muted)]">댓글 {total}</p>

      <div className="flex gap-2">
        <input
          value={body}
          onChange={(e) => setBody(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) submit(body);
          }}
          maxLength={2000}
          placeholder="이 논거에 대한 의견을 남겨주세요"
          className="min-w-0 flex-1 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
        />
        <button
          type="button"
          onClick={() => submit(body)}
          disabled={busy || body.trim() === ""}
          className="shrink-0 rounded-lg bg-[var(--brand)] px-3 py-1.5 text-xs font-medium text-white disabled:opacity-40"
        >
          등록
        </button>
      </div>

      {loading ? (
        <p className="text-[11px] text-[var(--text-muted)]">불러오는 중…</p>
      ) : comments.length === 0 ? (
        <p className="text-[11px] text-[var(--text-muted)]">첫 댓글을 남겨보세요.</p>
      ) : (
        <ul className="space-y-3">
          {comments.map((comment) => (
            <li key={comment.id} className="space-y-2">
              <CommentRow comment={comment} onChanged={load} onReply={() => setReplyTo(comment.id)} />
              {comment.replies?.map((reply) => (
                <div key={reply.id} className="ml-5 border-l border-[var(--border)] pl-3">
                  <CommentRow comment={reply} onChanged={load} />
                </div>
              ))}
              {replyTo === comment.id && (
                <div className="ml-5 flex gap-2 pl-3">
                  <input
                    value={replyBody}
                    onChange={(e) => setReplyBody(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && !e.shiftKey) submit(replyBody, comment.id);
                    }}
                    autoFocus
                    maxLength={2000}
                    placeholder="답글"
                    className="min-w-0 flex-1 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2.5 py-1 text-[11px] text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
                  />
                  <button
                    type="button"
                    onClick={() => submit(replyBody, comment.id)}
                    disabled={busy || replyBody.trim() === ""}
                    className="shrink-0 rounded-lg bg-[var(--brand)] px-2.5 py-1 text-[11px] font-medium text-white disabled:opacity-40"
                  >
                    등록
                  </button>
                  <button
                    type="button"
                    onClick={() => setReplyTo(null)}
                    className="shrink-0 text-[11px] text-[var(--text-muted)]"
                  >
                    취소
                  </button>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

function CommentRow({
  comment,
  onChanged,
  onReply,
}: {
  comment: ResearchComment;
  onChanged: () => void;
  onReply?: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(comment.body);

  if (comment.deleted) {
    return <p className="text-[11px] italic text-[var(--text-muted)]">삭제된 댓글입니다.</p>;
  }

  return (
    <div className="space-y-1">
      <div className="flex flex-wrap items-center gap-2 text-[10px] text-[var(--text-muted)]">
        <span className="font-medium text-[var(--text-secondary)]">
          {comment.authorName || `사용자 ${comment.userId}`}
        </span>
        <span>{formatDateTimeKST(comment.createdAt)}</span>
        {onReply && (
          <button type="button" onClick={onReply} className="hover:text-[var(--text-secondary)]">
            답글
          </button>
        )}
        {comment.canEdit && !editing && (
          <>
            <button
              type="button"
              onClick={() => setEditing(true)}
              className="hover:text-[var(--text-secondary)]"
            >
              수정
            </button>
            <button
              type="button"
              onClick={async () => {
                if (!window.confirm("댓글을 삭제할까요?")) return;
                await deleteResearchCommentClient(comment.id);
                onChanged();
              }}
              className="inline-flex items-center gap-0.5 hover:text-rose-400"
            >
              <Trash2 className="h-3 w-3" />
              삭제
            </button>
          </>
        )}
      </div>

      {editing ? (
        <div className="flex gap-2">
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            maxLength={2000}
            className="min-w-0 flex-1 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2.5 py-1 text-[11px] text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
          />
          <button
            type="button"
            onClick={async () => {
              await updateResearchCommentClient(comment.id, draft);
              setEditing(false);
              onChanged();
            }}
            className="shrink-0 rounded-lg bg-[var(--brand)] px-2.5 py-1 text-[11px] font-medium text-white"
          >
            저장
          </button>
          <button
            type="button"
            onClick={() => {
              setDraft(comment.body);
              setEditing(false);
            }}
            className="shrink-0 text-[11px] text-[var(--text-muted)]"
          >
            취소
          </button>
        </div>
      ) : (
        <p className="whitespace-pre-wrap break-words text-xs text-[var(--text-secondary)]">
          {comment.body}
        </p>
      )}
    </div>
  );
}
