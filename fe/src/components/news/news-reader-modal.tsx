"use client";

import { useEffect, useState } from "react";
import { ExternalLink, Loader2, X } from "lucide-react";
import type { NewsArticle, NewsReaderContent } from "@/lib/news-types";
import { newsArticleLink } from "@/lib/news-types";
import { newsDisplayHeadline } from "@/lib/news-display";
import { fetchNewsReader } from "@/lib/news-api";
import { formatRelativeTime } from "@/lib/format-news";
import {
  NewsPreviewZoomToolbar,
  NewsPreviewZoomViewport,
  useNewsPreviewZoom,
} from "@/components/news/news-preview-zoom";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";
import { buttonVariants } from "@/components/ui/button";
import { NewsCommunityShareLink } from "@/components/news/news-community-share-link";

type NewsReaderModalProps = {
  article: NewsArticle | null;
  onClose: () => void;
  /** 관련 종목·같은 이슈·액션 블록 (NewsArticleContext) */
  context?: React.ReactNode;
};

export function NewsReaderModal({ article, onClose, context }: NewsReaderModalProps) {
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
    <NewsReaderModalFrame
      key={article.id}
      article={article}
      onClose={onClose}
      context={context}
    />
  );
}

function NewsReaderModalFrame({
  article,
  onClose,
  context,
}: {
  article: NewsArticle;
  onClose: () => void;
  context?: React.ReactNode;
}) {
  const previewZoom = useNewsPreviewZoom();
  const [content, setContent] = useState<NewsReaderContent | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const link = newsArticleLink(article);
  const articleId = article.id;
  const articleMarket = article.market;

  useEffect(() => {
    if (!link) return;

    let cancelled = false;

    fetchNewsReader(link, { market: articleMarket, articleId })
      .then((data) => {
        if (!cancelled) {
          setContent(data);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(toUserMessage(err, "본문을 불러오지 못했습니다"));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [link, articleId, articleMarket]);

  const title = content?.title || newsDisplayHeadline(article);
  const heroImage = content?.imageUrl || article.imageUrl;
  const hasUrl = Boolean(link);
  const isUs = article.market === "us";
  const loadingLabel = isUs ? "본문 추출 및 번역 중…" : "본문 추출 중…";
  const footerLabel = content?.translated
    ? `${content.siteName ? `${content.siteName} · ` : ""}번역본 · Reader`
    : `${content?.siteName ? `${content.siteName} · ` : ""}Reader 추출`;

  if (!link) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
        <button
          type="button"
          aria-label="닫기"
          className="absolute inset-0 bg-black/60"
          onClick={onClose}
        />
        <div className="relative rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-6 py-4 text-sm text-[var(--text-muted)]">
          원문 URL이 없습니다.
        </div>
      </div>
    );
  }

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
        aria-labelledby="news-reader-title"
        className="relative flex h-[92vh] w-full max-w-3xl flex-col overflow-hidden rounded-t-2xl border border-[var(--border)] bg-[var(--surface-elevated)] shadow-2xl sm:h-[85vh] sm:rounded-2xl"
      >
        <header className="flex shrink-0 items-start justify-between gap-3 border-b border-[var(--border)] px-4 py-4 sm:px-5">
          <div className="min-w-0 space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <span
                className={cn(
                  "inline-flex items-center rounded-md border px-2 py-0.5 text-cell font-semibold",
                  article.publisher.badgeClass,
                )}
              >
                {article.publisher.name}
              </span>
              <span className="rounded-md bg-violet-500/15 px-2 py-0.5 text-cell font-semibold text-violet-700 dark:text-violet-300">
                Reader
              </span>
              {isUs && (
                <span className="rounded-md bg-sky-500/15 px-2 py-0.5 text-cell font-semibold text-sky-300">
                  한국어
                </span>
              )}
              <time dateTime={article.publishedAt} className="text-cell text-[var(--text-muted)]">
                {formatRelativeTime(article.publishedAt)}
              </time>
            </div>
            <h2
              id="news-reader-title"
              className="text-base font-semibold leading-snug text-[var(--text-primary)] sm:text-lg"
            >
              {title}
            </h2>
            {content?.originalTitle && content.originalTitle !== title && (
              <p className="text-cell leading-relaxed text-[var(--text-muted)]">{content.originalTitle}</p>
            )}
            {content?.byline && (
              <p className="text-cell text-[var(--text-muted)]">{content.byline}</p>
            )}
            {content?.excerpt && (
              <p className="text-read text-[var(--text-secondary)]">{content.excerpt}</p>
            )}
          </div>

          <button
            type="button"
            aria-label="닫기"
            onClick={onClose}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface)] hover:text-[var(--text-primary)]"
          >
            <X className="h-4 w-4" />
          </button>
        </header>

        <div className="flex min-h-0 flex-1 flex-col bg-[var(--background)]">
          <div className="flex shrink-0 items-center justify-end border-b border-[var(--border)] px-4 py-2 sm:px-5">
            <NewsPreviewZoomToolbar {...previewZoom} />
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto px-4 py-5 sm:px-8 sm:py-6">
            {loading && (
              <div className="flex h-40 items-center justify-center gap-2 text-sm text-[var(--text-muted)]">
                <Loader2 className="h-4 w-4 animate-spin" />
                {loadingLabel}
              </div>
            )}
            {error && (
              <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">
                {error}
              </div>
            )}
            {!loading && !error && content && (
              <NewsPreviewZoomViewport zoom={previewZoom.zoom}>
                <div className="mx-auto max-w-2xl space-y-4">
                  {heroImage ? (
                    <div className="overflow-hidden rounded-lg border border-[var(--border)]">
                      {/* eslint-disable-next-line @next/next/no-img-element */}
                      <img src={heroImage} alt="" className="max-h-64 w-full object-cover" />
                    </div>
                  ) : null}
                  {content.contentHtml ? (
                    <article
                      className="prose prose-invert max-w-none text-article text-[var(--text-primary)] [&_a]:text-[var(--brand)] [&_img]:max-w-full [&_p]:mb-4"
                      dangerouslySetInnerHTML={{ __html: content.contentHtml }}
                    />
                  ) : (
                    <article className="whitespace-pre-wrap text-article text-[var(--text-primary)]">
                      {content.textContent}
                    </article>
                  )}
                </div>
              </NewsPreviewZoomViewport>
            )}

            {!loading && !error && context && (
              <div className="mx-auto mt-6 max-w-2xl border-t border-[var(--border)] pt-4">
                {context}
              </div>
            )}
          </div>
        </div>

        <footer className="flex shrink-0 items-center justify-between gap-3 border-t border-[var(--border)] px-4 py-3 sm:px-5">
          <span className="text-cell text-[var(--text-muted)]">{footerLabel}</span>
          <span className="flex items-center gap-2">
          <NewsCommunityShareLink article={article} onClose={onClose} />
          {hasUrl && (
            <a
              href={link}
              target="_blank"
              rel="noopener noreferrer"
              className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] bg-[var(--surface)] text-[var(--text-primary)] hover:bg-[var(--background)]")}
            >
              원문 새 탭에서 열기
              <ExternalLink className="h-3.5 w-3.5" />
            </a>
          )}
          </span>
        </footer>
      </div>
    </div>
  );
}
