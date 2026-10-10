"use client";

import { useEffect, useMemo, useState, useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, Eye, Flag, Heart, Link2, MessageSquare, Newspaper, Pencil, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { AuthorLine, PostText, communityErrorText } from "@/components/community/community-bits";
import { CommunityComments } from "@/components/community/community-comments";
import { CommunitySymbolCard } from "@/components/community/community-symbol-card";
import { CommunityPostPreview } from "@/components/community/community-post-preview";
import { useMentionAssist } from "@/components/mentions/mention-assist";
import { PageColumns } from "@/components/layout/page-columns";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { useConfirm } from "@/components/providers/confirm-provider";
import { useSymbolQuickLook } from "@/components/stocks/symbol-quick-look-modal";
import { Card } from "@/components/ui/card";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { useUpcomingEvents } from "@/hooks/use-upcoming-events";
import {
  deleteCommunityPost,
  fetchCommunityPost,
  recordCommunityView,
  reportCommunity,
  toggleCommunityPostLike,
  updateCommunityPost,
} from "@/lib/community-api";
import { COMMUNITY_LIMITS, type CommunityPost } from "@/lib/community-types";
import { toUserMessage } from "@/lib/error-message";
import { formatDateTimeKST, formatRelativeTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { isAdminSession } from "@/lib/admin-features";
import { TopicBadge, TopicPicker } from "@/components/common/topic-ui";
import { isContentTopic, type ContentTopic } from "@/lib/topics";

/**
 * 커뮤니티 글 (2026-09-27 사용자 요청 4 — 두 번째 참고 사진의 흐름 + 우리 데이터).
 *
 * 서버가 첫 HTML에 본문을 싣고(검색 색인), 마운트 뒤 내 세션으로 다시 받아 좋아요·내 글
 * 표시를 덮는다. 조회는 마운트 때 한 번 기록한다(서버가 한 사람 한 번만 센다).
 */
export function CommunityDetailClient({ id, initialPost }: { id: number; initialPost: CommunityPost | null }) {
  const { session, loading: authLoading } = useAuth();
  const router = useRouter();
  const confirm = useConfirm();
  const quickLook = useSymbolQuickLook();
  const [post, setPost] = useState<CommunityPost | null>(initialPost);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [editTitle, setEditTitle] = useState("");
  const [editTopic, setEditTopic] = useState<ContentTopic>("stock");
  const [editBody, setEditBody] = useState("");
  const [saving, setSaving] = useState(false);
  // 수정 화면도 글쓰기와 같은 태그 도우미·미리보기(2026-09-28). 훅은 조건 없이 — 편집 중에만 켠다.
  const editRef = useRef<HTMLTextAreaElement>(null);
  const [editPreview, setEditPreview] = useState(false);
  const editAssist = useMentionAssist({ textareaRef: editRef, value: editBody, onChange: setEditBody, enabled: editing });

  useEffect(() => {
    if (authLoading) return;
    let alive = true;
    fetchCommunityPost(id)
      .then((p) => {
        if (alive) {
          setPost(p);
          setError(null);
        }
      })
      .catch((err) => {
        if (alive) setError(communityErrorText(err) ?? toUserMessage(err, "글을 불러오지 못했습니다."));
      });
    return () => {
      alive = false;
    };
  }, [id, authLoading, session?.user.id]);

  useEffect(() => {
    void recordCommunityView(id);
  }, [id]);

  // 붙인 종목 + 본문에 태그한 종목 — 칩의 지금 가격·카드의 일정이 같은 요청으로 온다.
  const allSymbols = useMemo(() => {
    const m = new Map<string, string>();
    for (const s of [...(post?.symbols ?? []), ...(post?.mentions?.symbols ?? [])]) {
      m.set(s.symbol.toUpperCase(), s.name || s.symbol);
    }
    return m;
  }, [post?.symbols, post?.mentions]);
  const symbols = useMemo(() => [...allSymbols.keys()], [allSymbols]);
  const quotes = useLiveQuotes(symbols);
  const eventTargets = useMemo(
    () => [...allSymbols].map(([symbol, name]) => ({ symbol, name })),
    [allSymbols],
  );
  const { bySymbol: eventsBySymbol } = useUpcomingEvents(eventTargets, 3);

  if (!post) {
    return (
      <PageScrollShell>
        <Card className="p-8 text-center text-sm text-[var(--text-muted)]">
          {error ?? "불러오는 중…"}
          <div className="mt-3">
            <Link href="/community" className="text-[var(--brand)] hover:underline">
              목록으로
            </Link>
          </div>
        </Card>
      </PageScrollShell>
    );
  }

  const loggedIn = Boolean(session);

  async function like() {
    if (!post) return;
    if (!loggedIn) {
      toast.message("좋아요는 로그인 후 누를 수 있습니다.");
      return;
    }
    try {
      const res = await toggleCommunityPostLike(post.id);
      setPost({ ...post, liked: res.liked, likeCount: res.likeCount });
    } catch (err) {
      toast.error(toUserMessage(err, "처리하지 못했습니다."));
    }
  }

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(window.location.href);
      toast.success("링크를 복사했습니다.");
    } catch {
      toast.error("링크를 복사하지 못했습니다.");
    }
  }

  async function report() {
    if (!post) return;
    if (!loggedIn) {
      toast.message("신고는 로그인 후 할 수 있습니다.");
      return;
    }
    const ok = await confirm({
      title: "이 글을 신고할까요?",
      description: "욕설·광고·리딩방 유도 등. 서로 다른 회원 3명이 신고하면 운영자 확인 전까지 숨겨집니다.",
      confirmLabel: "신고",
      tone: "danger",
    });
    if (!ok) return;
    try {
      await reportCommunity("post", post.id, "");
      toast.success("신고가 접수됐습니다.");
    } catch (err) {
      toast.error(toUserMessage(err, "신고하지 못했습니다."));
    }
  }

  async function remove() {
    if (!post) return;
    const ok = await confirm({ title: "이 글을 지울까요?", description: "댓글도 함께 보이지 않게 됩니다.", confirmLabel: "삭제", tone: "danger" });
    if (!ok) return;
    try {
      await deleteCommunityPost(post.id);
      toast.success("글을 지웠습니다.");
      router.push("/community");
    } catch (err) {
      toast.error(communityErrorText(err) ?? toUserMessage(err, "지우지 못했습니다."));
    }
  }

  async function saveEdit() {
    if (!post) return;
    setSaving(true);
    try {
      const next = await updateCommunityPost(post.id, editTitle, editBody, editTopic);
      setPost(next);
      setEditing(false);
      setEditPreview(false);
    } catch (err) {
      toast.error(communityErrorText(err) ?? toUserMessage(err, "고치지 못했습니다."));
    } finally {
      setSaving(false);
    }
  }

  return (
    <PageScrollShell>
      <PageColumns>
        <div className="mx-auto w-full max-w-3xl space-y-4">
          <Link href="/community" className="inline-flex items-center gap-1 text-cell text-[var(--text-muted)] hover:text-[var(--text-secondary)]">
            <ArrowLeft className="h-3.5 w-3.5" />
            커뮤니티
          </Link>

          <Card className="space-y-4 p-4 sm:p-6">
            {post.status === "hidden" && (
              <p className="rounded-lg bg-amber-500/10 px-3 py-2 text-cell text-amber-700 dark:text-amber-400">
                신고가 모여 다른 회원에게는 숨겨진 글입니다. 운영자가 확인한 뒤 다시 보이거나 그대로 숨겨집니다.
              </p>
            )}
            <header className="space-y-2">
              {editing ? (
                <>
                  <TopicPicker value={editTopic} onChange={setEditTopic} />
                  <input
                    value={editTitle}
                    onChange={(e) => setEditTitle(e.target.value)}
                    maxLength={COMMUNITY_LIMITS.titleMax}
                    className="w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 py-2 text-lg font-bold"
                  />
                </>
              ) : (
                <h1 className="text-xl font-bold leading-snug text-[var(--text-primary)] sm:text-2xl">
                  <TopicBadge topic={post.topic} className="mr-2 align-middle" />
                  {post.title}
                </h1>
              )}
              <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-cell text-[var(--text-muted)]">
                <AuthorLine author={post.author} symbols={post.symbols} maxBadges={3} />
                <time dateTime={post.createdAt} title={formatDateTimeKST(post.createdAt)}>
                  {formatRelativeTime(post.createdAt)}
                </time>
                {post.editedAt && <span>(수정됨)</span>}
                <span className="inline-flex items-center gap-1">
                  <Eye className="h-3 w-3" />
                  {post.viewCount}
                </span>
              </div>
            </header>

            {editing ? (
              <div className="space-y-2">
                {editAssist.tools}
                {editPreview ? (
                  <CommunityPostPreview body={editBody} />
                ) : (
                  <textarea
                    ref={editRef}
                    value={editBody}
                    onChange={(e) => {
                      setEditBody(e.target.value);
                      editAssist.syncCaret();
                    }}
                    onKeyDown={editAssist.onKeyDown}
                    onKeyUp={editAssist.syncCaret}
                    onClick={editAssist.syncCaret}
                    maxLength={COMMUNITY_LIMITS.bodyMax}
                    rows={8}
                    aria-label="본문 고치기"
                    className="w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 py-2 text-note leading-relaxed"
                  />
                )}
                {!editPreview && editAssist.popup}
                <div className="flex items-center justify-between text-micro text-[var(--text-muted)]">
                  <button
                    type="button"
                    onClick={() => setEditPreview((v) => !v)}
                    aria-pressed={editPreview}
                    className={cn("rounded-md border px-2 py-0.5", editPreview ? "border-[var(--brand)] text-[var(--brand)]" : "border-[var(--border)]")}
                  >
                    {editPreview ? "쓰기로" : "미리보기"}
                  </button>
                  <span>
                    {editBody.length}/{COMMUNITY_LIMITS.bodyMax} · 새로 넣은 태그만 지금 값으로 기록됩니다
                  </span>
                </div>
              </div>
            ) : (
              <PostText
                text={post.body ?? ""}
                className="text-note leading-relaxed text-[var(--text-primary)] sm:text-[15px]"
                mentions={post.mentions}
                quotes={quotes}
                renderCard={(m) => (
                  <CommunitySymbolCard
                    item={m}
                    quote={quotes.get(m.symbol.toUpperCase())}
                    event={eventsBySymbol.get(m.symbol.toUpperCase())?.[0] ?? eventsBySymbol.get(m.symbol)?.[0]}
                    onOpen={() => quickLook.open(m.symbol)}
                  />
                )}
              />
            )}

            {post.images.length > 0 && (
              <div className={cn("grid gap-2", post.images.length > 1 && "grid-cols-2")}>
                {post.images.map((img) => (
                  <a key={img.url} href={img.url} target="_blank" rel="noopener noreferrer" className="block overflow-hidden rounded-lg border border-[var(--border)]">
                    {/* eslint-disable-next-line @next/next/no-img-element -- 우리 미디어 경로(이미 압축본) */}
                    <img
                      src={img.url}
                      alt=""
                      loading="lazy"
                      width={img.width || undefined}
                      height={img.height || undefined}
                      className="h-auto max-h-[480px] w-full object-contain bg-[var(--surface-elevated)]"
                    />
                  </a>
                ))}
              </div>
            )}

            {post.symbols.length > 0 && (
              <div className="space-y-2">
                {post.symbols.map((s) => (
                  <CommunitySymbolCard
                    key={s.symbol}
                    item={s}
                    quote={quotes.get(s.symbol.toUpperCase())}
                    event={eventsBySymbol.get(s.symbol)?.[0]}
                    onOpen={() => quickLook.open(s.symbol)}
                  />
                ))}
              </div>
            )}

            {post.articles.length > 0 && (
              <div className="space-y-2">
                {post.articles.map((a) => (
                  <a
                    key={a.articleId}
                    href={a.url || "#"}
                    target="_blank"
                    rel="noopener noreferrer nofollow"
                    className="block rounded-xl border border-[var(--border)] border-l-4 border-l-[var(--text-secondary)] p-3 hover:border-[var(--brand)]"
                  >
                    <span className="flex items-center gap-1.5 text-micro text-[var(--text-muted)]">
                      <Newspaper className="h-3 w-3" />
                      {a.source}
                      {a.publishedAt && <span>· {a.publishedAt.slice(2, 10).replace(/-/g, ".")}</span>}
                    </span>
                    <span className="mt-1 block text-note font-semibold text-[var(--text-primary)]">{a.title}</span>
                  </a>
                ))}
              </div>
            )}

            {editing ? (
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setEditing(false)} className="rounded-lg border border-[var(--border)] px-3 py-1.5 text-cell">
                  취소
                </button>
                <button
                  type="button"
                  disabled={saving}
                  onClick={() => void saveEdit()}
                  className="rounded-lg bg-[var(--brand)] px-3 py-1.5 text-cell font-semibold text-white disabled:opacity-50"
                >
                  {saving ? "저장 중…" : "저장"}
                </button>
              </div>
            ) : (
              <div className="flex flex-wrap items-center gap-4 border-t border-[var(--border)] pt-3 text-cell text-[var(--text-muted)]">
                <button type="button" onClick={() => void like()} className="inline-flex items-center gap-1 hover:text-rose-500" aria-pressed={post.liked}>
                  <Heart className={cn("h-4 w-4", post.liked && "fill-rose-500 text-rose-500")} />
                  {post.likeCount}
                </button>
                <span className="inline-flex items-center gap-1">
                  <MessageSquare className="h-4 w-4" />
                  {post.commentCount}
                </span>
                <button type="button" onClick={() => void copyLink()} className="inline-flex items-center gap-1 hover:text-[var(--text-secondary)]">
                  <Link2 className="h-4 w-4" />
                  링크복사
                </button>
                <span className="ml-auto flex items-center gap-3">
                  {post.mine || isAdminSession(session) ? (
                    <>
                      <button
                        type="button"
                        onClick={() => {
                          setEditTitle(post.title);
                          setEditTopic(isContentTopic(post.topic) ? post.topic : "stock");
                          setEditBody(post.body ?? "");
                          setEditing(true);
                        }}
                        className="inline-flex items-center gap-1 hover:text-[var(--text-secondary)]"
                      >
                        <Pencil className="h-3.5 w-3.5" />
                        수정
                      </button>
                      <button type="button" onClick={() => void remove()} className="inline-flex items-center gap-1 hover:text-rose-500">
                        <Trash2 className="h-3.5 w-3.5" />
                        삭제
                      </button>
                    </>
                  ) : (
                    <button type="button" onClick={() => void report()} className="inline-flex items-center gap-1 hover:text-[var(--text-secondary)]">
                      <Flag className="h-3.5 w-3.5" />
                      신고
                    </button>
                  )}
                </span>
              </div>
            )}
          </Card>

          <Card className="p-4 sm:p-6">
            <CommunityComments
              postId={post.id}
              loggedIn={loggedIn}
              symbols={post.symbols}
              onCountChange={(d) => setPost((p) => (p ? { ...p, commentCount: Math.max(0, p.commentCount + d) } : p))}
            />
          </Card>

          <p className="px-1 text-micro leading-relaxed text-[var(--text-muted)]">
            회원 개인의 의견이며 투자 권유가 아닙니다. 종목 카드의 점수·요약은 Quant Port의 자동 분석이고, 「작성 시」 가격은 글을 올린 순간의 시세입니다.
          </p>
        </div>
      </PageColumns>
      {quickLook.modal}
    </PageScrollShell>
  );
}
