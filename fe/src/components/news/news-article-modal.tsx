"use client";

import { useEffect } from "react";
import { createPortal } from "react-dom";
import { ExternalLink, X } from "lucide-react";
import { NewsCommunityShareLink } from "@/components/news/news-community-share-link";
import type { NewsArticle } from "@/lib/news-types";
import { newsArticleLink } from "@/lib/news-types";
import { formatRelativeTime } from "@/lib/format-news";
import { canUseIframePreview, embedPolicyLabel } from "@/lib/news-embed";
import {
  NewsPreviewZoomToolbar,
  NewsPreviewZoomViewport,
  useNewsPreviewZoom,
} from "@/components/news/news-preview-zoom";
import { NewsKeywordBadges } from "@/components/news/news-keyword-badges";
import { NewsBreakingBadge } from "@/components/news/news-breaking-badge";
import { isArticleBreakingNews } from "@/lib/news-breaking";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

type NewsArticleModalProps = {
  article: NewsArticle | null;
  onClose: () => void;
  /** 관련 종목·같은 이슈·액션 블록 (NewsArticleContext) */
  context?: React.ReactNode;
};

export function NewsArticleModal({ article, onClose, context }: NewsArticleModalProps) {
  // 뒤로가기 처리는 셸이 아니라 `useNewsArticleModal`(호스트)이 맡는다 — 셸이 셋이라
  // 여기에 두면 하나를 빠뜨리고, 둘에 두면 히스토리 항목이 두 번 쌓인다.
  useEffect(() => {
    if (!article) {
      return;
    }
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        onClose();
      }
    };
    window.addEventListener("keydown", onKeyDown, true);

    return () => {
      document.body.style.overflow = prev;
      window.removeEventListener("keydown", onKeyDown, true);
    };
  }, [article, onClose]);

  if (!article || typeof document === "undefined") {
    return null;
  }

  return createPortal(
    <NewsArticleModalFrame
      key={article.id}
      article={article}
      onClose={onClose}
      context={context}
    />,
    document.body,
  );
}

function NewsArticleModalFrame({
  article,
  onClose,
  context,
}: {
  article: NewsArticle;
  onClose: () => void;
  context?: React.ReactNode;
}) {
  const previewZoom = useNewsPreviewZoom();

  const link = newsArticleLink(article);
  const hasUrl = Boolean(link);
  const showIframe = hasUrl && article.previewMode === "iframe" && canUseIframePreview(article.embedPolicy);
  const previewLabel = embedPolicyLabel(article.embedPolicy, article.publisher.name);
  const isBreaking = isArticleBreakingNews(article);

  return (
    <div className="fixed inset-0 z-[70] flex items-end justify-center p-0 sm:items-center sm:p-4">
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
                  "inline-flex items-center rounded-md border px-2 py-0.5 text-cell font-semibold",
                  article.publisher.badgeClass,
                )}
              >
                {article.publisher.name}
              </span>
              {isBreaking && <NewsBreakingBadge className="text-cell" />}
              <span
                className={cn(
                  "text-cell font-medium",
                  article.embedPolicy === "blocked"
                    ? "text-amber-400"
                    : article.embedPolicy === "ok"
                      ? "text-emerald-400"
                      : "text-[var(--text-muted)]",
                )}
              >
                {previewLabel}
              </span>
              {article.tags
                .filter((tag) => tag !== "속보")
                .map((tag) => (
                <span key={tag} className="text-cell font-medium text-[var(--text-muted)]">
                  {tag}
                </span>
              ))}
              <time dateTime={article.publishedAt} className="text-cell text-[var(--text-muted)]">
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
              <p className="text-read text-[var(--text-secondary)]">{article.summary}</p>
            )}
            {/* 종목 코드 나열은 옆 「관련 종목」 패널이 이름·시세와 함께 보여준다. */}
            <NewsKeywordBadges keywords={article.keywords} max={8} />
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

        <div className="flex min-h-0 flex-1 bg-[var(--background)]">
        <div className="relative flex min-h-0 flex-1 flex-col">
          {showIframe ? (
            <>
              <div className="flex shrink-0 items-center justify-between gap-2 border-b border-[var(--border)] px-4 py-2 sm:px-5">
                <p className="text-cell text-emerald-400/90">{previewLabel}</p>
                <NewsPreviewZoomToolbar {...previewZoom} />
              </div>
              <div className="relative min-h-0 flex-1 overflow-auto">
                <NewsPreviewZoomViewport zoom={previewZoom.zoom} className="h-full">
                  <iframe
                    title={article.headline}
                    src={link}
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

          {/*
           * 관련 종목·같은 이슈·액션은 본문 옆 세로 칸에 둔다. 본문 아래에 쌓으면
           * iframe이 화면을 다 쓰는 이 모달에서는 스크롤을 끝까지 내려야 보인다.
           * `lg` 미만에서는 폭이 없어 숨긴다 — 리더/헤드라인 모달은 본문이 흐르는
           * 구조라 아래에 붙일 수 있지만, 여기는 그럴 수 없다.
           */}
          {context && (
            <aside className="hidden w-[264px] shrink-0 overflow-y-auto border-l border-[var(--border)] p-3 lg:block">
              {context}
            </aside>
          )}
        </div>

        <footer className="flex shrink-0 items-center justify-between gap-3 border-t border-[var(--border)] px-4 py-3 sm:px-5">
          <span className="text-cell text-[var(--text-muted)]">
            수집: {article.source.name}
          </span>
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
        <p className="text-cell font-medium text-amber-400">
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
            <p className="text-cell leading-relaxed text-[var(--text-muted)]">
              요약만 표시됩니다. 전체 기사는 하단 「원문 새 탭에서 열기」를 이용해 주세요.
            </p>
          </div>
        </NewsPreviewZoomViewport>
      </div>
    </div>
  );
}
