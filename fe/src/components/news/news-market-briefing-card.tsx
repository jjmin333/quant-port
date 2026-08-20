"use client";

import { useState } from "react";
import { Bot, ChevronRight } from "lucide-react";
import type { NewsMarketBriefing } from "@/lib/news-market-briefing";
import { formatBriefingTimestamp } from "@/lib/news-market-briefing";
import { cn } from "@/lib/utils";

type NewsMarketBriefingCardProps = {
  briefing: NewsMarketBriefing | null;
  title?: string;
  className?: string;
};

export function NewsMarketBriefingCard({
  briefing,
  title = "KOSPI LAB AI 시장 브리핑",
  className,
}: NewsMarketBriefingCardProps) {
  const [expanded, setExpanded] = useState(false);

  if (!briefing) {
    return null;
  }

  return (
    <section
      className={cn(
        "rounded-2xl border border-[var(--border)] bg-[var(--surface)] px-5 py-5 sm:px-6 sm:py-6",
        className,
      )}
    >
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-[var(--brand-muted)] text-[var(--brand)]">
            <Bot className="h-4 w-4" aria-hidden />
          </span>
          <p className="text-[13px] font-semibold tracking-tight text-[var(--text-primary)]">
            {title}
          </p>
        </div>
        <div className="text-right">
          <p className="text-[11px] leading-relaxed text-[var(--text-muted)]">
            {formatBriefingTimestamp(briefing.generatedAt)}
          </p>
          <span className="mt-1 inline-block rounded-md border border-[var(--border)] px-2 py-0.5 text-[10px] font-medium text-[var(--text-secondary)]">
            AI 핵심요약
          </span>
        </div>
      </div>

      <h2 className="text-[17px] font-bold leading-[1.45] text-[var(--text-primary)] sm:text-lg">
        {briefing.headline}
      </h2>

      <p className="mt-3 text-[13px] leading-[1.65] text-[var(--text-secondary)]">
        {briefing.summary}
      </p>

      {expanded && briefing.highlights.length > 0 && (
        <ul className="mt-4 space-y-2 border-t border-[var(--border)] pt-4">
          {briefing.highlights.map((line) => (
            <li
              key={line}
              className="text-[13px] leading-[1.55] text-[var(--text-secondary)] before:mr-2 before:text-[var(--brand)] before:content-['•']"
            >
              {line}
            </li>
          ))}
        </ul>
      )}

      <div className="mt-5 flex justify-end">
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          className="inline-flex items-center gap-1 rounded-full border border-[var(--brand)] px-4 py-1.5 text-[13px] font-medium text-[var(--brand)] transition-colors hover:bg-[var(--brand-muted)]"
        >
          {expanded ? "접기" : "자세히 보기"}
          <ChevronRight
            className={cn("h-3.5 w-3.5 transition-transform", expanded && "rotate-90")}
            aria-hidden
          />
        </button>
      </div>
    </section>
  );
}
