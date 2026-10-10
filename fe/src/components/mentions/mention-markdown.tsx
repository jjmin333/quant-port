"use client";

import { useEffect, useMemo, useState } from "react";
import { CommunitySymbolCard } from "@/components/community/community-symbol-card";
import { mentionMarkdownRenderer } from "@/components/mentions/mention-inline";
import { useSymbolQuickLook } from "@/components/stocks/symbol-quick-look-modal";
import { Markdown } from "@/components/ui/markdown";
import { useLiveQuotes } from "@/hooks/use-live-quotes";
import { useUpcomingEvents } from "@/hooks/use-upcoming-events";
import { previewMentions, type MentionSet } from "@/lib/mentions";

/**
 * 태그가 살아 있는 리포트 본문 — 상세 화면과 편집기 미리보기가 같이 쓴다(2026-09-28 요청 2).
 * 태그가 없으면 그냥 `<Markdown>`이고, 시세·일정 요청도 나가지 않는다(빈 목록).
 */
export function MentionMarkdown({ source, mentions }: { source: string; mentions?: MentionSet | null }) {
  const symbols = useMemo(() => (mentions?.symbols ?? []).map((s) => s.symbol.toUpperCase()), [mentions]);
  const quotes = useLiveQuotes(symbols);
  const targets = useMemo(
    () => (mentions?.symbols ?? []).map((s) => ({ symbol: s.symbol, name: s.name || s.symbol })),
    [mentions],
  );
  const { bySymbol } = useUpcomingEvents(targets, 3);
  const quickLook = useSymbolQuickLook();

  const renderer = mentionMarkdownRenderer(mentions, quotes, (m) => (
    <CommunitySymbolCard
      item={m}
      quote={quotes.get(m.symbol.toUpperCase())}
      event={bySymbol.get(m.symbol)?.[0]}
      onOpen={() => quickLook.open(m.symbol)}
    />
  ));

  return (
    <>
      <Markdown source={source} renderer={renderer} />
      {quickLook.modal}
    </>
  );
}

/** 편집기 미리보기 — 서버가 태그를 해석해 준 뒤 같은 모양으로 그린다(저장 안 함). */
export function MentionMarkdownPreview({ source }: { source: string }) {
  const [set, setSet] = useState<MentionSet | null>(null);
  const hasTags = /[@#]/.test(source);
  useEffect(() => {
    if (!hasTags) return;
    let alive = true;
    previewMentions(source)
      .then((s) => {
        if (alive) setSet(s);
      })
      .catch(() => {
        // 미리보기 실패는 태그 없이 그린다.
      });
    return () => {
      alive = false;
    };
  }, [source, hasTags]);
  return <MentionMarkdown source={source} mentions={hasTags ? set : null} />;
}
