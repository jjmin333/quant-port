"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { MessagesSquare, PenLine } from "lucide-react";
import { SectionCard } from "@/components/ui/section-card";
import { formatRelativeTime } from "@/lib/format";
import { fetchAssetPosts, type AssetPost } from "@/lib/notifications-api";

/**
 * 「이 자산 이야기」 — 단지·지역·상품·금리를 태그한 커뮤니티 글과 분석 리포트 (2026-10-10 사용자 요청 4).
 * 호갱노노의 단지 이야기처럼 자산 화면에서 글을 모아 본다. 글이 없으면 첫 글을 쓰는 입구만 남긴다.
 * `compact`는 미리보기 모달 안 — 카드 껍데기 없이 최대 3줄, 글이 없으면 아무것도 그리지 않는다.
 */
export function AssetPostsCard({
  kind,
  assetKey,
  compact = false,
}: {
  kind: "apt" | "region" | "product" | "rate";
  assetKey: string;
  compact?: boolean;
}) {
  const [posts, setPosts] = useState<AssetPost[] | null>(null);
  useEffect(() => {
    let alive = true;
    fetchAssetPosts(kind, assetKey)
      .then((p) => alive && setPosts(p))
      .catch(() => alive && setPosts([]));
    return () => {
      alive = false;
    };
  }, [kind, assetKey]);

  const topic = kind === "apt" || kind === "region" ? "real_estate" : "finance";
  const writeHref = `/community/new?topic=${topic}`;
  const list = (posts ?? []).slice(0, compact ? 3 : 10);

  const rows = (
    <ul className="divide-y divide-[var(--border)]">
      {list.map((p) => (
        <li key={`${p.type}-${p.id}`}>
          <Link
            href={p.type === "research" ? `/reports/${p.id}` : `/community/${p.id}`}
            className="flex items-baseline gap-2 px-3 py-2 text-cell hover:bg-[var(--surface-elevated)]"
          >
            <span className="shrink-0 rounded bg-[var(--surface-elevated)] px-1 text-micro text-[var(--text-muted)]">
              {p.type === "research" ? "리포트" : "커뮤니티"}
            </span>
            <span className="min-w-0 flex-1 truncate font-medium text-[var(--text-primary)]">{p.title}</span>
            <span className="shrink-0 text-micro text-[var(--text-muted)]" suppressHydrationWarning>
              {formatRelativeTime(p.createdAt)}
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );

  if (compact) {
    if (list.length === 0) return null;
    return (
      <div className="mt-3 overflow-hidden rounded-lg border border-[var(--border)]">
        <p className="flex items-center gap-1 px-3 pt-2 text-micro font-semibold text-[var(--text-muted)]">
          <MessagesSquare className="size-3" /> 이 자산 이야기
        </p>
        {rows}
      </div>
    );
  }

  return (
    <SectionCard
      title="이 자산 이야기"
      bodyClassName="p-0"
      action={
        <Link href={writeHref} className="inline-flex items-center gap-1 text-cell font-medium text-[var(--brand)] hover:underline">
          <PenLine className="size-3.5" /> 글쓰기
        </Link>
      }
    >
      {posts === null ? (
        <p className="p-3 text-cell text-[var(--text-muted)]">불러오는 중…</p>
      ) : list.length === 0 ? (
        <p className="p-3 text-cell leading-relaxed text-[var(--text-muted)]">
          아직 이 이야기를 한 글이 없습니다. 글에서 @{kind === "product" ? "상품" : kind === "rate" ? "금리" : kind === "region" ? "지역" : "아파트"}[이름]으로 태그하면 여기에 모입니다.
        </p>
      ) : (
        rows
      )}
    </SectionCard>
  );
}
