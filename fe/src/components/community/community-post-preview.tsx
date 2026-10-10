"use client";

import { useEffect, useState } from "react";
import { PostText } from "@/components/community/community-bits";
import { CommunitySymbolCard } from "@/components/community/community-symbol-card";
import { previewMentions, type MentionSet } from "@/lib/mentions";

/** 올리기 전 모양 — 서버가 태그를 해석해 준다(저장 안 함). 「그때」 = 지금 값. */
export function CommunityPostPreview({ body }: { body: string }) {
  const [set, setSet] = useState<MentionSet | null>(null);
  const hasTags = /[@#]/.test(body);
  useEffect(() => {
    if (!hasTags) return;
    let alive = true;
    previewMentions(body)
      .then((s) => {
        if (alive) setSet(s);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [body, hasTags]);
  return (
    <div className="min-h-[9rem] rounded-lg border border-dashed border-[var(--border)] px-3 py-2.5">
      {body.trim() === "" ? (
        <p className="text-note text-[var(--text-muted)]">아직 쓴 내용이 없습니다.</p>
      ) : (
        <PostText
          text={body}
          mentions={hasTags ? set : null}
          className="text-note leading-relaxed text-[var(--text-primary)]"
          renderCard={(m) => <CommunitySymbolCard item={m} onOpen={() => {}} />}
        />
      )}
    </div>
  );
}
