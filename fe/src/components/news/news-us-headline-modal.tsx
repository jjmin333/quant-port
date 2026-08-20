"use client";

import { useEffect, useState } from "react";
import { ExternalLink, X } from "lucide-react";
import type { NewsArticle } from "@/lib/news-types";
import { newsArticleLink } from "@/lib/news-types";
import { newsDisplayHeadline } from "@/lib/news-display";
import { formatNewsDateTime, formatNewsDateTimeLong, formatRelativeTime } from "@/lib/format-news";
import { isArticleBreakingNews } from "@/lib/news-breaking";
import { NewsBreakingBadge } from "@/components/news/news-breaking-badge";
import { cn } from "@/lib/utils";

type NewsUsHeadlineModalProps = {
  article: NewsArticle | null;
  onClose: () => void;
  /** 관련 종목·같은 이슈·액션 블록 (NewsArticleContext) */
  context?: React.ReactNode;
};

export function NewsUsHeadlineModal({ article, onClose, context }: NewsUsHeadlineModalProps) {
  useEffect(() => {
    if (!article) {
      return;
    }
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", onKeyDown);

    return () => {
      document.body.style.overflow = prev;
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [article, onClose]);

  if (!article) {
    return null;
  }

  return (
    <NewsUsHeadlineModalFrame
      key={article.id}
      article={article}
      onClose={onClose}
      context={context}
    />
  );
}

function NewsUsHeadlineModalFrame({
  article,
  onClose,
  context,
}: {
  article: NewsArticle;
  onClose: () => void;
  context?: React.ReactNode;
}) {
  const [showOriginal, setShowOriginal] = useState(false);

  const link = newsArticleLink(article);
  const translatedHeadline = newsDisplayHeadline(article);
  const originalHeadline = article.headline.trim();
  const hasTranslation = Boolean(article.headlineKo?.trim()) && originalHeadline !== translatedHeadline;
  const bodyText = showOriginal || !hasTranslation ? originalHeadline : translatedHeadline;
  const isBreaking = isArticleBreakingNews(article);

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-4">
      <button
        type="button"
        aria-label="닫기"
        className="absolute inset-0 bg-black/60 backdrop-blur-[1px]"
        onClick={onClose}
      />

      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="news-us-headline-title"
        className="relative flex h-[92vh] w-full max-w-3xl flex-col overflow-hidden rounded-t-2xl border border-[var(--border)] bg-[var(--surface-elevated)] shadow-2xl sm:h-[85vh] sm:rounded-2xl"
      >
        <header className="shrink-0 border-b border-[var(--border)] px-4 py-4 sm:px-5">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 space-y-2">
              <div className="flex flex-wrap items-center gap-1.5">
                <span
                  className={cn(
                    "inline-flex items-center rounded-md border px-2 py-0.5 text-[11px] font-semibold",
                    article.publisher.badgeClass,
                  )}
                >
                  {article.publisher.name}
                </span>
                {isBreaking && <NewsBreakingBadge className="text-[11px]" />}
              </div>
              <p className="text-[11px] text-[var(--text-muted)]">
                {formatRelativeTime(article.publishedAt)}
                <span className="mx-1.5">·</span>
                {formatNewsDateTime(article.publishedAt)}
                <span className="mx-1.5">·</span>
                출처: {article.publisher.name}
              </p>
              <h2
                id="news-us-headline-title"
                className="text-lg font-bold leading-snug text-[var(--text-primary)] sm:text-xl"
              >
                {translatedHeadline}
              </h2>
            </div>

            <button
              type="button"
              aria-label="닫기"
              onClick={onClose}
              className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface)] hover:text-[var(--text-primary)]"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </header>

        <div className="flex min-h-0 flex-1 flex-col bg-[var(--background)]">
          {hasTranslation && (
            <div className="flex shrink-0 gap-2 border-b border-[var(--border)] px-4 py-3 sm:px-5">
              <button
                type="button"
                onClick={() => setShowOriginal(false)}
                className={cn(
                  "rounded-full px-4 py-1.5 text-[13px] font-medium transition-colors",
                  !showOriginal
                    ? "bg-violet-500/20 text-violet-300"
                    : "text-[var(--text-muted)] hover:text-[var(--text-primary)]",
                )}
              >
                번역
              </button>
              <button
                type="button"
                onClick={() => setShowOriginal(true)}
                className={cn(
                  "rounded-full px-4 py-1.5 text-[13px] font-medium transition-colors",
                  showOriginal
                    ? "bg-violet-500/20 text-violet-300"
                    : "text-[var(--text-muted)] hover:text-[var(--text-primary)]",
                )}
              >
                원문
              </button>
            </div>
          )}

          <div className="min-h-0 flex-1 overflow-y-auto px-4 py-5 sm:px-8 sm:py-6">
            <p className="text-[15px] leading-7 text-[var(--text-primary)]">{bodyText}</p>
            <p className="mt-8 text-[12px] text-[var(--text-muted)]">
              기사 원문 등록 시간 {formatNewsDateTimeLong(article.publishedAt)}
            </p>

            {/*
             * 해외 헤드라인 모달은 본문이 한 문단뿐이라 아래가 크게 빈다. 관련 종목과
             * 같은 이슈 기사가 그 자리를 메우면서, 헤드라인만 보고 닫던 화면이
             * 「그래서 무슨 일인가」까지 답하게 된다.
             */}
            {context && (
              <div className="mt-6 border-t border-[var(--border)] pt-4">{context}</div>
            )}
          </div>
        </div>

        <footer className="flex shrink-0 items-center justify-between gap-3 border-t border-[var(--border)] px-4 py-3 sm:px-5">
          <span className="text-[11px] text-[var(--text-muted)]">헤드라인 · {article.source.name}</span>
          {link && (
            <a
              href={link}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 py-1.5 text-xs font-medium text-[var(--text-primary)] hover:bg-[var(--background)]"
            >
              원문 새 탭에서 열기
              <ExternalLink className="h-3.5 w-3.5" />
            </a>
          )}
        </footer>
      </div>
    </div>
  );
}
