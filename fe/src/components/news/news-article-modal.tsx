"use client";

import { useEffect } from "react";
import { ExternalLink, X } from "lucide-react";
import type { NewsArticle } from "@/lib/news-types";
import { formatRelativeTime } from "@/lib/format-news";
import { canUseIframePreview, embedPolicyLabel } from "@/lib/news-embed";
import {
  NewsPreviewZoomToolbar,
  NewsPreviewZoomViewport,
  useNewsPreviewZoom,
} from "@/components/news/news-preview-zoom";
import { cn } from "@/lib/utils";

type NewsArticleModalProps = {
  article: NewsArticle | null;
  onClose: () => void;
};

export function NewsArticleModal({ article, onClose }: NewsArticleModalProps) {
  const previewZoom = useNewsPreviewZoom();

  useEffect(() => {
    previewZoom.reset();
  }, [article?.id, previewZoom.reset]);

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

  const hasUrl = Boolean(article.url && article.url !== "#");
  const showIframe = hasUrl && canUseIframePreview(article.embedPolicy);
  const previewLabel = embedPolicyLabel(article.embedPolicy, article.publisher.name);

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
        aria-labelledby="news-modal-title"
        className="relative flex h-[92vh] w-full max-w-4xl flex-col overflow-hidden rounded-t-2xl border border-[var(--border)] bg-[var(--surface-elevated)] shadow-2xl sm:h-[85vh] sm:rounded-2xl"
      >
        <header className="flex shrink-0 items-start justify-between gap-3 border-b border-[var(--border)] px-4 py-4 sm:px-5">
          <div className="min-w-0 space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <span
                className={cn(
                  "inline-flex items-center rounded-md border px-2 py-0.5 text-[11px] font-semibold",
                  article.publisher.badgeClass,
                )}
              >
                {article.publisher.name}
              </span>
              <span
                className={cn(
                  "text-[11px] font-medium",
                  article.embedPolicy === "blocked"
                    ? "text-amber-400"
                    : article.embedPolicy === "ok"
                      ? "text-emerald-400"
                      : "text-[var(--text-muted)]",
                )}
              >
                {previewLabel}
              </span>
              {article.tags.map((tag) => (
                <span key={tag} className="text-[11px] font-medium text-[var(--text-muted)]">
                  {tag}
                </span>
              ))}
              <time dateTime={article.publishedAt} className="text-[11px] text-[var(--text-muted)]">
                {formatRelativeTime(article.publishedAt)}
              </time>
            </div>
            <h2
              id="news-modal-title"
              className="text-base font-semibold leading-snug text-[var(--text-primary)] sm:text-lg"
            >
              {article.headline}
            </h2>
            {article.summary && (
              <p className="text-sm leading-relaxed text-[var(--text-secondary)]">{article.summary}</p>
            )}
            {article.symbols && article.symbols.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {article.symbols.map((symbol) => (
                  <span
                    key={symbol}
                    className="rounded bg-[var(--surface)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--text-muted)]"
                  >
                    {symbol}
                  </span>
                ))}
              </div>
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

        <div className="relative flex min-h-0 flex-1 flex-col bg-[var(--background)]">
          {showIframe ? (
            <>
              <div className="flex shrink-0 items-center justify-between gap-2 border-b border-[var(--border)] px-4 py-2 sm:px-5">
                <p className="text-[11px] text-emerald-400/90">{previewLabel}</p>
                <NewsPreviewZoomToolbar {...previewZoom} />
              </div>
              <div className="relative min-h-0 flex-1 overflow-auto">
                <NewsPreviewZoomViewport zoom={previewZoom.zoom} className="h-full">
                  <iframe
                    title={article.headline}
                    src={article.url}
                    className="h-full min-h-[120vh] w-full border-0 bg-white"
                    sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
                    referrerPolicy="no-referrer-when-downgrade"
                  />
                </NewsPreviewZoomViewport>
              </div>
            </>
          ) : hasUrl ? (
            <ReaderFallback article={article} previewZoom={previewZoom} />
          ) : (
            <div className="flex h-full items-center justify-center px-6 text-sm text-[var(--text-muted)]">
              원문 URL이 없습니다.
            </div>
          )}
        </div>

        <footer className="flex shrink-0 items-center justify-between gap-3 border-t border-[var(--border)] px-4 py-3 sm:px-5">
          <span className="text-[11px] text-[var(--text-muted)]">
            수집: {article.source.name}
          </span>
          {hasUrl && (
            <a
              href={article.url}
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

function ReaderFallback({
  article,
  previewZoom,
}: {
  article: NewsArticle;
  previewZoom: ReturnType<typeof useNewsPreviewZoom>;
}) {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 items-center justify-between gap-2 border-b border-[var(--border)] px-4 py-2 sm:px-5">
        <p className="text-xs font-medium text-amber-400">
          {embedPolicyLabel(article.embedPolicy, article.publisher.name)}
        </p>
        <NewsPreviewZoomToolbar {...previewZoom} />
      </div>
      <div className="min-h-0 flex-1 overflow-auto px-4 py-6 sm:px-8">
        <NewsPreviewZoomViewport zoom={previewZoom.zoom}>
          <div className="mx-auto w-full max-w-2xl space-y-4">
            <p className="text-sm leading-7 text-[var(--text-primary)]">
              {article.summary?.trim() ||
                "이 언론사는 iframe 미리보기를 막습니다. 본문 전체는 원문 링크에서 확인해 주세요."}
            </p>
            <p className="text-xs leading-relaxed text-[var(--text-muted)]">
              요약만 표시됩니다. 전체 기사는 하단 「원문 새 탭에서 열기」를 이용해 주세요.
            </p>
          </div>
        </NewsPreviewZoomViewport>
      </div>
    </div>
  );
}
