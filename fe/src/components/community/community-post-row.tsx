"use client";

import Link from "next/link";
import { Eye, Heart, ImageIcon, MessageSquare, Newspaper } from "lucide-react";
import { AuthorLine } from "@/components/community/community-bits";
import { TopicBadge } from "@/components/common/topic-ui";
import type { LiveQuote } from "@/hooks/use-live-quotes";
import type { CommunityPost } from "@/lib/community-types";
import { plainMentions } from "@/lib/mention-text";
import { formatPercent, formatRelativeTime } from "@/lib/format";
import { SCORE_TONE_TEXT, scoreTone } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 목록 한 줄 — 참고 사이트의 흐름(메타 · 제목 · 발췌 · 작성자와 수치 · 오른쪽 썸네일)에
 * **붙인 종목 칩**을 더했다. 칩은 지금 등락률과 AI 점수를 보여 준다: 글을 열기 전에도
 * 「이 이야기의 종목이 지금 어떤 상태인가」가 보이는 것이 이 커뮤니티의 차이다.
 *
 * 한 칸짜리 grid(minmax(0,1fr))로 폭을 가둔다 — 홈 카드에서 Safari가 버튼 안 flex 자식을
 * 말줄임 없이 밀어냈던 결함(2026-09-27 제보 2)을 처음부터 피한다.
 */
export function CommunityPostRow({
  post,
  quotes,
  onPickSymbol,
}: {
  post: CommunityPost;
  quotes: Map<string, LiveQuote>;
  onPickSymbol?: (symbol: string) => void;
}) {
  const thumb = post.images[0];
  return (
    <article className="relative grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-3 px-4 py-3.5 transition-colors hover:bg-[var(--surface-elevated)]/50">
      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-1">
        <div className="flex min-w-0 items-center gap-1.5 text-micro text-[var(--text-muted)]">
          <TopicBadge topic={post.topic} />
          {post.popular && <span className="font-bold text-[var(--brand)]">인기</span>}
          {post.popular && <span aria-hidden>·</span>}
          <time dateTime={post.createdAt}>{formatRelativeTime(post.createdAt)}</time>
          {post.status === "hidden" && (
            <span className="rounded bg-amber-500/15 px-1 font-semibold text-amber-700 dark:text-amber-400">
              숨김(신고 확인 중)
            </span>
          )}
        </div>
        <h3 className="min-w-0 text-section font-bold leading-snug text-[var(--text-primary)]">
          {/* 행 전체가 눌리게 — 제목 링크를 행 위로 늘린다(칩·버튼은 z-10으로 그 위에). */}
          <Link href={`/community/${post.id}`} className="after:absolute after:inset-0 hover:text-[var(--brand)]">
            <span className="line-clamp-2">{post.title}</span>
          </Link>
          {post.commentCount > 0 && (
            <span className="ml-1.5 align-middle text-micro font-semibold text-[var(--brand)]">
              <MessageSquare className="mr-0.5 inline h-3 w-3" />
              {post.commentCount}
            </span>
          )}
        </h3>
        {post.excerpt && (
          <p className="line-clamp-2 min-w-0 text-note leading-relaxed text-[var(--text-secondary)]">{plainMentions(post.excerpt)}</p>
        )}
        {(post.symbols.length > 0 || post.articleCount > 0) && (
          <div className="relative z-10 flex min-w-0 flex-wrap items-center gap-1.5 pt-0.5">
            {post.symbols.map((s) => {
              const q = quotes.get(s.symbol.toUpperCase());
              const rate = q && q.livePrice > 0 ? q.liveChangeRate : undefined;
              const score = s.score ?? s.snapScore;
              return (
                <button
                  key={s.symbol}
                  type="button"
                  onClick={() => onPickSymbol?.(s.symbol)}
                  className="inline-flex max-w-full items-center gap-1 rounded-full border border-[var(--border)] bg-[var(--surface)] px-2 py-0.5 text-micro hover:border-[var(--brand)]"
                  title={`${s.name || s.symbol} 이야기만 보기`}
                >
                  <span className="truncate font-semibold text-[var(--text-primary)]">{s.name || s.symbol}</span>
                  {rate != null && <span className={cn("font-num", trendTextClass(rate))}>{formatPercent(rate)}</span>}
                  {score != null && (
                    <span className={cn("font-num font-bold", SCORE_TONE_TEXT[scoreTone(score)])}>{score}점</span>
                  )}
                </button>
              );
            })}
            {post.articleCount > 0 && (
              <span className="inline-flex items-center gap-1 rounded-full border border-[var(--border)] px-2 py-0.5 text-micro text-[var(--text-muted)]">
                <Newspaper className="h-3 w-3" />
                기사 {post.articleCount}
              </span>
            )}
          </div>
        )}
        <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 pt-0.5 text-cell text-[var(--text-muted)]">
          <AuthorLine author={post.author} symbols={post.symbols} className="max-w-full" />
          <span className="inline-flex items-center gap-1" title="좋아요">
            <Heart className={cn("h-3 w-3", post.liked && "fill-rose-500 text-rose-500")} />
            {post.likeCount}
          </span>
          <span className="inline-flex items-center gap-1" title="댓글">
            <MessageSquare className="h-3 w-3" />
            {post.commentCount}
          </span>
          <span className="inline-flex items-center gap-1" title="조회">
            <Eye className="h-3 w-3" />
            {post.viewCount}
          </span>
        </div>
      </div>
      {thumb ? (
        <span className="relative block h-16 w-16 shrink-0 self-center overflow-hidden rounded-lg border border-[var(--border)] sm:h-20 sm:w-20">
          {/* eslint-disable-next-line @next/next/no-img-element -- 우리 미디어 경로(이미 압축본) */}
          <img src={thumb.url} alt="" loading="lazy" className="h-full w-full object-cover" />
          {post.imageCount > 1 && (
            <span className="absolute bottom-0.5 right-0.5 inline-flex items-center gap-0.5 rounded bg-black/60 px-1 text-[10px] text-white">
              <ImageIcon className="h-2.5 w-2.5" />
              {post.imageCount}
            </span>
          )}
        </span>
      ) : (
        <span />
      )}
    </article>
  );
}
