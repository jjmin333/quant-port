"use client";

import { CONTENT_TOPICS, TOPIC_BADGE_CLASS, isContentTopic, topicLabel, type ContentTopic } from "@/lib/topics";
import { cn } from "@/lib/utils";
import { ASSET_FEATURES_HIDDEN } from "@/lib/hidden-features";

/** 글 주제 고르기 — 커뮤니티·리포트·메모장 글쓰기 공통(2026-10-09 요청 9). 자산 기능을 숨긴 동안은 주식뿐이라 그리지 않는다(10-13). */
export function TopicPicker({
  value,
  onChange,
  className,
}: {
  value: ContentTopic;
  onChange: (t: ContentTopic) => void;
  className?: string;
}) {
  if (ASSET_FEATURES_HIDDEN) return null;
  return (
    <div className={cn("flex items-center gap-2", className)}>
      <span className="text-cell font-semibold text-[var(--text-secondary)]">주제</span>
      <div className="flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5" role="radiogroup" aria-label="글 주제">
        {CONTENT_TOPICS.map((t) => (
          <button
            key={t.id}
            type="button"
            role="radio"
            aria-checked={value === t.id}
            onClick={() => onChange(t.id)}
            className={cn(
              "rounded-md px-2.5 py-1 text-cell font-medium transition-colors",
              value === t.id
                ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
                : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
            )}
          >
            {t.label}
          </button>
        ))}
      </div>
    </div>
  );
}

/** 목록 행의 주제 배지 — 주식은 그리지 않는다(기존 글 대부분이라 소음). */
export function TopicBadge({ topic, className }: { topic?: string | null; className?: string }) {
  if (!isContentTopic(topic) || topic === "stock") return null;
  return (
    <span className={cn("shrink-0 rounded px-1.5 py-px text-micro font-semibold", TOPIC_BADGE_CLASS[topic], className)}>
      {topicLabel(topic)}
    </span>
  );
}
