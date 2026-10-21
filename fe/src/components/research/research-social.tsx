"use client";

import { useCallback, useEffect, useState } from "react";
import { Bookmark, Heart, MessageSquare, ThumbsUp, Trash2 } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Card } from "@/components/ui/card";
import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import { useLoginHref } from "@/components/news/news-login-gate";
import {
  addResearchCommentClient,
  deleteResearchCommentClient,
  fetchResearchCommentsClient,
  rateResearchClient,
  toggleResearchCommentLikeClient,
  toggleResearchReactionClient,
  updateResearchCommentClient,
} from "@/lib/research-api";
import { activityBadge, relativeTime } from "@/lib/comment-meta";
import {
  RATING_AXES,
  type ResearchCall,
  type ResearchComment,
} from "@/lib/research-types";
import { formatDateTimeKST } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useConfirm } from "@/components/providers/confirm-provider";
import { toUserMessage } from "@/lib/error-message";
import { buttonVariants } from "@/components/ui/button";

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
        // 접근 가능한 이름이 숫자뿐이었다 — 스크린리더에 "0 버튼"으로 읽혔다.
        // 숫자는 상태이지 이름이 아니다.
        aria-label={liked ? "추천 취소" : "추천"}
        className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] disabled:opacity-60",
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
        className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] disabled:opacity-60",
          bookmarked
            ? "text-amber-400"
            : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]",
        )}
      >
        <Bookmark className={cn("h-3.5 w-3.5", bookmarked && "fill-amber-400")} />
        {bookmarked ? "저장됨" : "저장"}
      </button>
      <span className="inline-flex items-center gap-1 text-cell text-[var(--text-muted)]">
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
        <p className="text-cell font-medium text-[var(--text-muted)]">논거 평가</p>
        {summary && (
          <p className="text-sm font-semibold text-[var(--text-primary)]">
            {summary.overall.toFixed(1)}
            <span className="ml-1 text-micro font-normal text-[var(--text-muted)]">
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
                <span className="w-16 shrink-0 text-micro text-[var(--text-muted)]">
                  {axis.label}
                </span>
                <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
                  <span
                    className="block h-full rounded-full bg-[var(--brand)]"
                    style={{ width: `${(v / 5) * 100}%` }}
                  />
                </span>
                <span className="w-6 shrink-0 text-right text-micro tabular-nums text-[var(--text-secondary)]">
                  {v.toFixed(1)}
                </span>
              </div>
            );
          })}
        </div>
      ) : (
        <p className="text-cell text-[var(--text-muted)]">아직 평가가 없습니다.</p>
      )}

      {call.canRate ? (
        <div className="space-y-2 border-t border-[var(--border)] pt-2.5">
          {RATING_AXES.map((axis) => (
            <div key={axis.id} className="flex items-center gap-2">
              <span className="w-16 shrink-0 text-micro text-[var(--text-muted)]">
                {axis.label}
              </span>
              <div className="flex gap-1">
                {[1, 2, 3, 4, 5].map((n) => (
                  <button
                    key={n}
                    type="button"
                    onClick={() => setValues((v) => ({ ...v, [axis.id]: n }))}
                    className={cn(
                      "h-6 w-6 rounded text-cell tabular-nums transition-colors",
                      values[axis.id] === n
                        ? "bg-[var(--brand)] text-[var(--brand-foreground)]"
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
                setError(toUserMessage(err, "평가에 실패했습니다."));
              } finally {
                setSaving(false);
              }
            }}
            className="w-full rounded-lg bg-[var(--brand)] py-1.5 text-cell font-medium text-[var(--brand-foreground)] disabled:opacity-40"
          >
            {call.myRating ? "평가 수정" : "평가 남기기"}
          </button>
          {error && <p className="text-cell text-[var(--loss)]">{error}</p>}
        </div>
      ) : (
        <p className="border-t border-[var(--border)] pt-2 text-micro text-[var(--text-muted)]">
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

/**
 * Comment thread. Replies go exactly one level deep.
 *
 * # 2026-09-14 사용자 요청 4로 고친 것
 *
 *   · **한 줄 입력칸 → 여러 줄.** 리포트에 다는 답은 「좋아요」가 아니라 반론이라
 *     보통 두세 문장이다. `<input>`은 쓴 글의 첫 줄만 보여 주고, Enter가 곧 등록이라
 *     줄을 바꾸려다 미완성 댓글이 올라갔다. 이제 Enter는 줄바꿈이고 등록은 버튼
 *     (또는 ⌘/Ctrl+Enter)이다.
 *   · **비로그인 안내.** 예전에는 비로그인 상태로 등록을 누르면 요청이 401로 죽고
 *     **화면에는 아무 일도 일어나지 않았다** — 글이 사라지지도 않고 오류도 없다.
 *     이제 입력칸 자리에 로그인 링크가 서고, 돌아올 자리를 `next`에 싣는다.
 *   · **오류를 보여 준다.** 등록 실패가 조용했다.
 *
 * # 2026-09-14 사용자 요청 3으로 고친 것
 *
 * 「댓글 목록」에서 **읽을 수 있는 토론**으로. 스무 줄이 쌓이면 예전 모양에서는
 * 전부 같은 무게로 보였다 — 누가 썼는지도, 어느 것이 읽을 만한지도, 언제 쓴
 * 것인지도 한눈에 안 갈렸다.
 *
 *   · **추천.** 좋은 반박이 스무 번째 줄에 묻히지 않게. 답글에도 붙는다.
 *   · **글쓴이 표시.** 리포트를 쓴 사람이 답하면 「작성자」가 붙는다 — 토론에서
 *     가장 중요한 한 줄이 그 사람의 답이고, 그게 어느 것인지는 이름만 봐서는
 *     모른다. 활동량 배지(`100+`)는 처음 보는 이름과 오래 써 온 이름을 가른다.
 *   · **상대 시각.** 「2026. 9. 14. 오후 2:31」이 아니라 「3시간 전」 — 스무 줄이
 *     같은 폭의 숫자 덩어리를 달고 있으면 그 열은 읽히지 않는다. 하루가 넘으면
 *     날짜로 돌아간다(`relativeTime`).
 *   · **답글 접기.** 답글이 셋을 넘으면 접어 둔다. 한 줄의 곁가지가 본 줄기보다
 *     길면 다음 댓글이 화면 밖으로 밀린다.
 *   · **제목을 스스로 든다.** 「댓글 및 피드백 N개」 — 상세 화면이 이 수를 따로
 *     세면 두 숫자가 어긋난다.
 *
 * `authorUserId` — 리포트를 쓴 사람. 「작성자」 배지의 근거다.
 */
export function CommentThread({
  callId,
  authorUserId,
}: {
  callId: number;
  authorUserId?: number;
}) {
  const { session } = useAuth();
  const loginHref = useLoginHref();
  const [comments, setComments] = useState<ResearchComment[]>([]);
  const [loading, setLoading] = useState(true);
  const [body, setBody] = useState("");
  const [replyTo, setReplyTo] = useState<number | null>(null);
  const [replyBody, setReplyBody] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
    setError(null);
    try {
      await addResearchCommentClient(callId, trimmed, parentId);
      setBody("");
      setReplyBody("");
      setReplyTo(null);
      await load();
    } catch (e) {
      // 쓴 글은 **지우지 않는다.** 실패했는데 입력칸까지 비면 다시 써야 한다.
      setError(toUserMessage(e, "댓글을 등록하지 못했습니다"));
    } finally {
      setBusy(false);
    }
  }

  const total = comments.reduce((n, c) => n + 1 + (c.replies?.length ?? 0), 0);
  const signedIn = Boolean(session);

  return (
    <Card className="space-y-3 p-4">
      {/*
        제목을 이 컴포넌트가 든다 (2026-09-14 사용자 요청 3). 수를 세는 자리가
        하나여야 상세 화면의 숫자와 어긋나지 않는다.
      */}
      <h2 className="text-section font-semibold text-[var(--text-primary)]">
        댓글 및 피드백 <span className="tabular-nums text-[var(--brand)]">{total}</span>개
      </h2>

      {signedIn ? (
        <div className="space-y-2">
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            onKeyDown={(e) => {
              // ⌘/Ctrl+Enter로도 등록된다 — 긴 글을 쓰고 마우스로 옮겨 가지 않게.
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) submit(body);
            }}
            rows={3}
            maxLength={2000}
            placeholder="이 논거에 대한 의견을 남겨주세요 — 어디에 동의하고 어디가 다른가요?"
            className="w-full resize-y rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2 text-cell leading-relaxed text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
          />
          <div className="flex items-center justify-between gap-2">
            <span className="text-micro text-[var(--text-muted)]">
              {body.length > 0 && `${body.length} / 2000`}
            </span>
            <button
              type="button"
              onClick={() => submit(body)}
              disabled={busy || body.trim() === ""}
              className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-[var(--brand-foreground)] disabled:opacity-40")}
            >
              등록
            </button>
          </div>
        </div>
      ) : (
        <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2.5 text-cell text-[var(--text-secondary)]">
          댓글을 쓰려면{" "}
          <Link href={loginHref} className="font-medium text-[var(--brand)] hover:underline">
            로그인
          </Link>
          이 필요합니다. 읽는 것은 로그인 없이도 됩니다.
        </div>
      )}

      {error && <p className="text-cell text-[var(--loss)]">{error}</p>}

      {loading ? (
        <p className="text-cell text-[var(--text-muted)]">불러오는 중…</p>
      ) : comments.length === 0 ? (
        <p className="text-cell text-[var(--text-muted)]">첫 댓글을 남겨보세요.</p>
      ) : (
        <ul className="divide-y divide-[var(--border)]">
          {comments.map((comment) => (
            <li key={comment.id} className="space-y-2 py-3 first:pt-1">
              {/*
                답글 버튼은 로그인했을 때만 선다. 비로그인에 보이면 누른 뒤에야
                안 된다는 것을 알게 되는데, 그때 알려 주는 장치가 없었다(등록이
                401로 조용히 죽었다).
              */}
              <CommentRow
                comment={comment}
                authorUserId={authorUserId}
                onChanged={load}
                onReply={signedIn ? () => setReplyTo(comment.id) : undefined}
              />

              {/*
                답글 (2026-09-14 사용자 요청 3).

                셋을 넘으면 접는다. 한 줄의 곁가지가 본 줄기보다 길면 다음 댓글이
                화면 밖으로 밀리고, 목록을 훑는 사람은 그 아래에 무엇이 더 있는지
                모른다. 셋인 이유는 그 아래로는 접는 버튼이 접히는 것보다 자리를
                더 먹기 때문이다.
              */}
              <ReplyList
                replies={comment.replies ?? []}
                authorUserId={authorUserId}
                onChanged={load}
              />

              {replyTo === comment.id && (
                <div className="ml-5 space-y-1.5 border-l-2 border-[var(--border)] pl-3">
                  {/*
                    답글도 **여러 줄**이다 (2026-09-14 사용자 요청 3). 본 댓글은
                    09-14 오전에 여러 줄이 됐는데 답글만 한 줄짜리로 남아, Enter가
                    곧 등록이라 줄을 바꾸려다 미완성 답글이 올라갔다 — 고쳤다고
                    적어 둔 바로 그 고장이 옆 칸에 그대로 있었다.
                  */}
                  <textarea
                    value={replyBody}
                    onChange={(e) => setReplyBody(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) submit(replyBody, comment.id);
                    }}
                    autoFocus
                    rows={2}
                    maxLength={2000}
                    placeholder="답글"
                    className="w-full resize-y rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2.5 py-1.5 text-cell leading-relaxed text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
                  />
                  <div className="flex items-center justify-end gap-1.5">
                    <button
                      type="button"
                      onClick={() => setReplyTo(null)}
                      className="shrink-0 px-1.5 text-cell text-[var(--text-muted)] hover:text-[var(--text-primary)]"
                    >
                      취소
                    </button>
                    <button
                      type="button"
                      onClick={() => submit(replyBody, comment.id)}
                      disabled={busy || replyBody.trim() === ""}
                      className={cn(buttonVariants({ variant: "default", size: "xs" }), "bg-[var(--brand)] text-[var(--brand-foreground)] disabled:opacity-40")}
                    >
                      등록
                    </button>
                  </div>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

/** 답글 묶음 — 셋까지 펴고 나머지는 접는다. 근거는 부르는 쪽 주석에. */
function ReplyList({
  replies,
  authorUserId,
  onChanged,
}: {
  replies: ResearchComment[];
  authorUserId?: number;
  onChanged: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  if (replies.length === 0) return null;

  const visible = expanded ? replies : replies.slice(0, REPLY_PREVIEW);
  const hidden = replies.length - visible.length;

  return (
    <div className="ml-5 space-y-2 border-l-2 border-[var(--border)] pl-3">
      {visible.map((reply) => (
        <CommentRow
          key={reply.id}
          comment={reply}
          authorUserId={authorUserId}
          onChanged={onChanged}
        />
      ))}
      {hidden > 0 && (
        <button
          type="button"
          onClick={() => setExpanded(true)}
          className="text-cell font-medium text-[var(--brand)] hover:underline"
        >
          답글 {hidden}개 더 보기
        </button>
      )}
    </div>
  );
}

const REPLY_PREVIEW = 3;

function CommentRow({
  comment,
  authorUserId,
  onChanged,
  onReply,
}: {
  comment: ResearchComment;
  authorUserId?: number;
  onChanged: () => void;
  onReply?: () => void;
}) {
  const confirm = useConfirm();
  const { session } = useAuth();
  const canManage = comment.canEdit || isAdminSession(session);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(comment.body);

  if (comment.deleted) {
    return <p className="text-cell italic text-[var(--text-muted)]">삭제된 댓글입니다.</p>;
  }

  const name = comment.authorName || `사용자 ${comment.userId}`;
  const isAuthor = authorUserId !== undefined && comment.userId === authorUserId;
  // 게시 계정은 관리자가 정한 뱃지만(2026-10-16) — 활동 수 뱃지 대신.
  const badge = comment.authorBadge || activityBadge(comment.authorActivity);
  /*
    상대 시각이 안 되는 날짜(하루 초과·못 읽는 값)에는 원래 표기로 돌아간다.
    「9일 전」은 그게 언제인지 세어 봐야 알고, 댓글은 보통 글과 같은 주에 몰린다.
  */
  const when = relativeTime(comment.createdAt) ?? formatDateTimeKST(comment.createdAt);

  return (
    <div className="flex gap-2.5">
      {/*
        이름 앞의 동그라미. 사진이 아니라 **첫 글자**다 — 이 앱에는 프로필 사진이
        없고, 만들어 내면 없는 기능이 있는 것처럼 보인다. 여기서 하는 일은 스무 줄이
        쌓였을 때 같은 사람의 줄을 눈으로 묶어 주는 것 하나다.
      */}
      <span
        aria-hidden
        className="mt-0.5 grid size-7 shrink-0 place-items-center rounded-full bg-[var(--surface-elevated)] text-cell font-semibold text-[var(--text-muted)]"
      >
        {name.slice(0, 1)}
      </span>

      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex flex-wrap items-center gap-x-1.5 gap-y-1 text-micro text-[var(--text-muted)]">
          <span className="font-medium text-[var(--text-secondary)]">{name}</span>
          {/*
            「작성자」가 제일 먼저다. 토론에서 가장 중요한 한 줄은 글쓴이의 답이고,
            그게 어느 것인지는 이름만 봐서는 모른다.
          */}
          {isAuthor && (
            <span className="rounded bg-[var(--brand-muted)] px-1 py-px font-semibold text-[var(--brand)]">
              작성자
            </span>
          )}
          {badge && (
            <span
              className="rounded-full border border-[var(--border)] px-1.5 py-px"
              title="이 사람이 남긴 리포트와 댓글 수"
            >
              {badge}
            </span>
          )}
          <span>·</span>
          <span title={formatDateTimeKST(comment.createdAt)}>{when}</span>
        </div>

        {editing ? (
          <div className="space-y-1.5">
            {/*
              수정도 **여러 줄**이다 (2026-09-14 사용자 요청 3). 쓸 때는 여러 줄인데
              고칠 때 한 줄이면, 두 문단짜리 댓글의 오타 하나를 고치려고 첫 줄만
              보이는 칸에서 좌우로 스크롤해야 한다.
            */}
            <textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              rows={3}
              maxLength={2000}
              className="w-full resize-y rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2.5 py-1.5 text-cell leading-relaxed text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
            />
            <div className="flex items-center justify-end gap-1.5">
              <button
                type="button"
                onClick={() => {
                  setDraft(comment.body);
                  setEditing(false);
                }}
                className="shrink-0 px-1.5 text-cell text-[var(--text-muted)] hover:text-[var(--text-primary)]"
              >
                취소
              </button>
              <button
                type="button"
                onClick={async () => {
                  await updateResearchCommentClient(comment.id, draft);
                  setEditing(false);
                  onChanged();
                }}
                className={cn(buttonVariants({ variant: "default", size: "xs" }), "bg-[var(--brand)] text-[var(--brand-foreground)]")}
              >
                저장
              </button>
            </div>
          </div>
        ) : (
          <p className="whitespace-pre-wrap break-words text-read text-[var(--text-secondary)]">
            {comment.body}
          </p>
        )}

        {/*
          동작은 **본문 아래**에 둔다. 예전에는 이름 줄에 함께 있어서, 「답글」과
          「삭제」가 글을 읽기도 전에 보였다 — 읽고 나서 하는 일이 읽기 전에 서 있는
          모양이다. 추천은 그중에서도 가장 자주 쓰는 것이라 맨 앞이다.
        */}
        {!editing && (
          <div className="flex flex-wrap items-center gap-3 pt-0.5 text-micro text-[var(--text-muted)]">
            <CommentLikeButton comment={comment} />
            {onReply && (
              <button type="button" onClick={onReply} className="hover:text-[var(--text-primary)]">
                답글
              </button>
            )}
            {canManage && (
              <>
                <button
                  type="button"
                  onClick={() => setEditing(true)}
                  className="hover:text-[var(--text-primary)]"
                >
                  수정
                </button>
                <button
                  type="button"
                  onClick={async () => {
                    if (!(await confirm({ title: "댓글을 삭제할까요?", confirmLabel: "삭제", tone: "danger" }))) return;
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
        )}
      </div>
    </div>
  );
}

/**
 * 댓글 추천 (2026-09-14 사용자 요청 3).
 *
 * 글 추천(`ReactionBar`)과 **같은 방식**이다 — 낙관적으로 먼저 그리고 서버 값으로
 * 바로잡는다. 실패하면 되돌린다: 화면의 숫자가 서버에 저장된 것과 달라지면, 다음에
 * 새로고침했을 때 사용자는 자기가 누른 것이 사라졌다고 읽는다.
 *
 * 비로그인에게도 **수는 보인다.** 추천은 「이 반론이 읽을 만한가」를 말하는
 * 신호이고, 그 신호는 읽기만 하는 사람에게 가장 쓸모 있다. 누르면 로그인으로 간다.
 */
function CommentLikeButton({ comment }: { comment: ResearchComment }) {
  const { session } = useAuth();
  const loginHref = useLoginHref();
  const router = useRouter();
  const [liked, setLiked] = useState(comment.liked);
  const [count, setCount] = useState(comment.likeCount);
  const [busy, setBusy] = useState(false);

  /*
    목록을 다시 받으면 서버 값이 진실이다 — 남이 누른 것도 여기서 따라온다.

    이펙트가 아니라 **렌더 중 조정**이다. 이펙트로 하면 예전 값이 한 프레임 그려진
    뒤에 바뀌어 숫자가 깜빡이고, `setState`가 이펙트 안에서 도는 것 자체를 이 저장소의
    린트가 막는다. 모달들이 같은 자리에서 같은 처리를 한다(React가 권하는 방식).
  */
  const [seen, setSeen] = useState({ liked: comment.liked, count: comment.likeCount });
  if (seen.liked !== comment.liked || seen.count !== comment.likeCount) {
    setSeen({ liked: comment.liked, count: comment.likeCount });
    setLiked(comment.liked);
    setCount(comment.likeCount);
  }

  async function toggle() {
    if (!session) {
      router.push(loginHref);
      return;
    }
    if (busy) return;
    setBusy(true);
    const was = liked;
    setLiked(!was);
    setCount((n) => n + (was ? -1 : 1));
    try {
      const active = await toggleResearchCommentLikeClient(comment.id);
      setLiked(active);
      // 서버가 우리 예상과 다른 상태를 말하면 수도 그쪽에 맞춘다.
      if (active !== !was) setCount(comment.likeCount + (active ? 1 : 0));
    } catch {
      setLiked(was);
      setCount(comment.likeCount);
    } finally {
      setBusy(false);
    }
  }

  return (
    <button
      type="button"
      onClick={toggle}
      disabled={busy}
      aria-pressed={liked}
      aria-label={liked ? "추천 취소" : "추천"}
      className={cn(
        "inline-flex items-center gap-1 disabled:opacity-60",
        liked ? "font-semibold text-rose-400" : "hover:text-[var(--text-primary)]",
      )}
    >
      <ThumbsUp className={cn("h-3 w-3", liked && "fill-rose-400")} />
      {count > 0 && <span className="tabular-nums">{count}</span>}
    </button>
  );
}
