"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { TopicPicker } from "@/components/common/topic-ui";
import { ResearchCompose } from "@/components/research/research-compose";
import { ResearchNoteCompose } from "@/components/research/research-note-compose";
import { ASSET_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { isContentTopic, type ContentTopic } from "@/lib/topics";

/**
 * 리포트 쓰기 입구 — 주제를 고른다(2026-10-09 요청 9). 주식은 기존 종목 콜 작성기, 부동산·금융상품은
 * 판정 없는 분석 리포트 작성기. `?topic=`으로 바로 들어올 수 있다. `?symbol=`이 있으면 주식이다.
 */
export function ResearchComposeEntry() {
  const params = useSearchParams();
  const [topic, setTopic] = useState<ContentTopic>(() => {
    const t = params.get("topic");
    return !ASSET_FEATURES_HIDDEN && !params.get("symbol") && isContentTopic(t) ? t : "stock";
  });
  const picker = <TopicPicker value={topic} onChange={setTopic} className="px-1" />;

  if (topic === "stock") return <ResearchCompose header={picker} />;
  return (
    <PageScrollShell width="reading" contentClassName="space-y-3">
      {picker}
      <ResearchNoteCompose key={topic} topic={topic} />
    </PageScrollShell>
  );
}
