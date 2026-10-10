"use client";

import Link from "next/link";
import { Heart, MessageSquare, PenLine } from "lucide-react";
import { SectionCard } from "@/components/ui/section-card";
import type { CommunityPost } from "@/lib/community-types";
import { formatRelativeTime } from "@/lib/format";
import { stockWorkspaceHref } from "@/lib/stock-workspace-tabs";

/**
 * 종목 개요 맨 아래 「이 종목 이야기」 — 커뮤니티로 들어가는 종목 입구 (2026-09-27 요청 4).
 * 최근 세 글만. 전체는 `/community?symbol=` 필터로, 글쓰기는 이 종목을 붙인 채로 간다.
 * 글은 워크스페이스가 받아 넘긴다(탭 노출 판단과 한 요청, 2026-10-07). `null`은 받는 중.
 */
export function StockCommunityCard({
  symbol,
  name,
  posts,
}: {
  symbol: string;
  name: string;
  posts: CommunityPost[] | null;
}) {
  // 전체는 종목 화면 「커뮤니티」 탭(글 + 리포트, 2026-09-28 요청 3).
  const listHref = stockWorkspaceHref(symbol, "community");
  const writeHref = `/community/new?symbols=${encodeURIComponent(symbol)}`;

  return (
    <SectionCard
      title={`${name || symbol} 이야기`}
      href={posts && posts.length > 0 ? listHref : undefined}
      hrefLabel="커뮤니티 탭에서 더 보기"
      action={
        <Link href={writeHref} className="inline-flex items-center gap-1 text-cell text-[var(--brand)] hover:underline">
          <PenLine className="h-3 w-3" />
          글쓰기
        </Link>
      }
    >
      {posts === null ? (
        <p className="px-1 py-2 text-cell text-[var(--text-muted)]">불러오는 중…</p>
      ) : posts.length === 0 ? (
        <p className="px-1 py-2 text-cell text-[var(--text-muted)]">
          아직 이 종목 이야기가 없습니다. 글에 이 종목을 붙이면 점수·AI 요약이 함께 보입니다.
        </p>
      ) : (
        <ul className="grid grid-cols-[minmax(0,1fr)] divide-y divide-[var(--border)]">
          {posts.map((p) => (
            <li key={p.id}>
              <Link href={`/community/${p.id}`} className="flex min-w-0 items-center gap-2 px-1 py-2 text-cell hover:text-[var(--brand)]">
                <span className="block min-w-0 flex-1 truncate font-medium text-[var(--text-primary)]">{p.title}</span>
                <span className="inline-flex shrink-0 items-center gap-0.5 text-micro text-[var(--text-muted)]">
                  <Heart className="h-3 w-3" />
                  {p.likeCount}
                </span>
                <span className="inline-flex shrink-0 items-center gap-0.5 text-micro text-[var(--text-muted)]">
                  <MessageSquare className="h-3 w-3" />
                  {p.commentCount}
                </span>
                <span className="shrink-0 text-micro text-[var(--text-muted)]">{formatRelativeTime(p.createdAt)}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
