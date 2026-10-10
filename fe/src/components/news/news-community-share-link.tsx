"use client";

import Link from "next/link";
import { MessagesSquare } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { communityShareHref } from "@/lib/community-share";
import { COMMUNITY_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { leaveModal } from "@/lib/modal-history";
import type { NewsArticle } from "@/lib/news-types";
import { cn } from "@/lib/utils";

/**
 * 기사 모달 바닥의 「커뮤니티에 공유」 — 셸 셋(iframe·리더·해외 헤드라인)이 같이 쓴다.
 *
 * **`leaveModal`로 닫는다.** `onClose`로 닫으면 모달이 쌓아 둔 히스토리 항목을 걷는
 * `back()`이 글쓰기 화면으로 가는 이동을 밀어내 「눌러도 아무 일도 없다」
 * (2026-09-28 사용자 제보 — `lib/modal-history`의 그 경합 그대로).
 */
export function NewsCommunityShareLink({
  article,
  onClose,
}: {
  article: Pick<NewsArticle, "id" | "headline" | "headlineKo" | "symbols">;
  onClose: () => void;
}) {
  // 커뮤니티 비노출(lib/hidden-features) 동안은 세 셸 모두에서 빠진다.
  if (COMMUNITY_FEATURES_HIDDEN) return null;
  return (
    <Link
      href={communityShareHref(article)}
      onClick={() => leaveModal(onClose)}
      className={cn(
        buttonVariants({ variant: "outline", size: "sm" }),
        "border-[var(--border)] bg-[var(--surface)] text-[var(--text-primary)] hover:bg-[var(--background)]",
      )}
    >
      <MessagesSquare className="h-3.5 w-3.5" />
      커뮤니티에 공유
    </Link>
  );
}
