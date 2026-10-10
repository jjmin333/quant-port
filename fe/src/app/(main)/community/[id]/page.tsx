import { Suspense } from "react";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { CommunityDetailClient } from "@/components/community/community-detail-client";
import { JsonLd } from "@/components/seo/json-ld";
import { fetchCommunityPostServer } from "@/lib/community-api.server";
import type { CommunityPost } from "@/lib/community-types";
import { SITE_URL, breadcrumbJsonLd, NOINDEX, pageMetadata } from "@/lib/seo";

type Props = { params: Promise<{ id: string }> };

function parseId(raw: string): number | null {
  const id = Number(raw);
  return Number.isInteger(id) && id > 0 ? id : null;
}

function describe(post: CommunityPost): string {
  const text = (post.body ?? "").replace(/\s+/g, " ").trim();
  const names = post.symbols.map((s) => s.name || s.symbol).join(", ");
  const base = text.length > 150 ? `${text.slice(0, 150)}…` : text;
  return base || (names ? `${names}에 대한 커뮤니티 글입니다.` : "Quant Port 커뮤니티 글입니다.");
}

/**
 * 글마다 제목·설명을 붙인다. 숨김·삭제 글은 서버 선반입이 익명이라 애초에 404 → noindex.
 */
export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const id = parseId((await params).id);
  if (id === null) return { robots: NOINDEX };
  const post = await fetchCommunityPostServer(id);
  if (!post) return { title: "커뮤니티", robots: NOINDEX };
  return {
    ...pageMetadata({ title: post.title, description: describe(post), path: `/community/${id}` }),
    authors: post.author.displayName ? [{ name: post.author.displayName }] : undefined,
  };
}

/** 토론 글 구조화 데이터 — 검색이 「커뮤니티 글」로 알아보게 한다. */
function forumPostingJsonLd(post: CommunityPost, path: string) {
  const url = `${SITE_URL}${path}`;
  return {
    "@context": "https://schema.org",
    "@type": "DiscussionForumPosting",
    "@id": `${url}#post`,
    url,
    mainEntityOfPage: url,
    headline: post.title,
    text: (post.body ?? "").slice(0, 500),
    datePublished: post.createdAt,
    ...(post.editedAt ? { dateModified: post.editedAt } : {}),
    ...(post.author.displayName ? { author: { "@type": "Person", name: post.author.displayName } } : {}),
    ...(post.images[0] ? { image: `${SITE_URL}${post.images[0].url}` } : {}),
    interactionStatistic: [
      { "@type": "InteractionCounter", interactionType: "https://schema.org/LikeAction", userInteractionCount: post.likeCount },
      { "@type": "InteractionCounter", interactionType: "https://schema.org/CommentAction", userInteractionCount: post.commentCount },
    ],
    inLanguage: "ko-KR",
  };
}

export default async function CommunityPostPage({ params }: Props) {
  const id = parseId((await params).id);
  if (id === null) notFound();
  const post = await fetchCommunityPostServer(id);
  return (
    <>
      {post && (
        <>
          <JsonLd data={forumPostingJsonLd(post, `/community/${id}`)} />
          <JsonLd
            data={breadcrumbJsonLd([
              { name: "커뮤니티", path: "/community" },
              { name: post.title, path: `/community/${id}` },
            ])}
          />
        </>
      )}
      <Suspense>
        <CommunityDetailClient id={id} initialPost={post} />
      </Suspense>
    </>
  );
}
