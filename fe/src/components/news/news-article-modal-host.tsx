"use client";

import { useCallback, useMemo, useState } from "react";
import { NewsArticleContext, articleModalContextHasContent } from "@/components/news/news-article-context";
import { NewsArticleModal } from "@/components/news/news-article-modal";
import { useModalHistory } from "@/hooks/use-modal-history";
import { leaveModal } from "@/lib/modal-history";
import { NewsReaderModal } from "@/components/news/news-reader-modal";
import { NewsUsHeadlineModal } from "@/components/news/news-us-headline-modal";
import { recordNewsView } from "@/lib/news-api";
import { newsUsesReader, type NewsArticle } from "@/lib/news-types";
import type { QuoteSummary } from "@/lib/quotes-types";

/**
 * 기사를 여는 **한 가지** 방법.
 *
 * 기사 모달은 셸이 셋이다 — 해외 헤드라인 / 리더 / iframe. 어느 것을 쓸지는 기사의
 * 시장과 프리뷰 정책이 정하지, 기사를 연 화면이 정할 일이 아니다. 그런데 지금까지는
 * **뉴스 화면만** 그 규칙을 알고 있었다. 홈·지수 모달·종목 퀵룩·종목 뉴스 목록은 전부
 * `NewsArticleModal`(iframe 셸) 하나를 context 없이 불렀고, 그 결과 같은 기사가
 * 어디서 열렸느냐에 따라 다른 물건이 됐다 — 뉴스 화면에서는 「관련 종목·지수」 미니
 * 차트와 같은 이슈 기사가 붙어 나오는데, 홈에서 연 같은 기사에는 아무것도 없었다.
 *
 * 규칙을 여기 한 곳에 둔다. 호출부는 `open(article)`만 부른다 —
 * 셸 선택·문맥 블록·조회 기록은 전부 이 훅이 맡는다.
 *
 * **시세는 안 넘겨도 된다.** `NewsRelatedEntityPanel`이 빠진 종목을 스스로 채우고,
 * 모달은 한 번에 하나만 열리므로 그 요청은 연 기사 하나 몫이다. 목록이 이미 시세를
 * 들고 있는 화면(뉴스)만 `quotes`로 넘겨 요청을 0으로 만든다.
 */

/** 기본값을 렌더마다 새로 만들면 아래 `useMemo`가 매번 깨진다. */
const NO_ARTICLES: NewsArticle[] = [];
const NO_QUOTES = new Map<string, QuoteSummary>();

type Shell = "us" | "reader" | "iframe";

function shellFor(article: NewsArticle): Shell {
  if (article.market === "us") return "us";
  return newsUsesReader(article) ? "reader" : "iframe";
}

export type NewsArticleModalHost = {
  /** 지금 열려 있는 기사. 닫혀 있으면 null. */
  article: NewsArticle | null;
  open: (article: NewsArticle) => void;
  close: () => void;
  /** 화면 어딘가에 그대로 렌더한다. 셋 중 하나만 실제로 뜬다. */
  modal: React.ReactNode;
};

export function useNewsArticleModal(
  options?: {
    /** 「같은 이슈」 묶기 후보 — 지금 화면에 있는 기사들. 없으면 그 블록이 뜨지 않는다. */
    pool?: NewsArticle[];
    /** 호출부가 이미 들고 있는 시세. 빠진 종목은 관련 종목 패널이 스스로 채운다. */
    quotes?: Map<string, QuoteSummary>;
  },
): NewsArticleModalHost {
  const pool = options?.pool ?? NO_ARTICLES;
  const quotes = options?.quotes ?? NO_QUOTES;

  const [article, setArticle] = useState<NewsArticle | null>(null);

  const open = useCallback((next: NewsArticle) => {
    void recordNewsView(next.id);
    setArticle(next);
  }, []);

  const close = useCallback(() => setArticle(null), []);
  // 문맥 블록의 링크(관련 종목 카드 등)는 화면을 떠나며 닫는다 — lib/modal-history 경합.
  const leaveClose = useCallback(() => leaveModal(close), [close]);

  /*
    뒤로가기로 기사 모달을 닫는다 (§C-3).

    셸이 셋(`NewsArticleModal`·`NewsReaderModal`·`NewsUsHeadlineModal`)이라 각각에
    붙이면 반드시 하나를 빠뜨린다 — 실제로 처음에 iframe 셸에만 붙였다가 국내 뉴스가
    쓰는 리더 셸에서는 그대로 사이트를 벗어나는 것을 실측으로 다시 잡았다. 어느 셸이
    뜨든 열림/닫힘을 아는 곳은 여기 하나뿐이므로 여기에 둔다.
  */
  useModalHistory(article !== null, close);

  /*
   * 문맥 블록은 셸 셋이 **같은 것을 공유한다**. 셸마다 따로 만들면 "관련 종목이
   * 국내 기사에서만 뜬다" 같은 어긋남이 곧 생긴다.
   */
  const context = useMemo(
    () =>
      article && articleModalContextHasContent(article, pool) ? (
        <NewsArticleContext
          key={article.id}
          article={article}
          pool={pool}
          quotes={quotes}
          onArticleClick={open}
          onNavigate={leaveClose}
        />
      ) : undefined,
    [article, pool, quotes, open, leaveClose],
  );

  const shell = article ? shellFor(article) : null;

  const modal = (
    <>
      <NewsArticleModal
        article={shell === "iframe" ? article : null}
        onClose={close}
        context={context}
      />
      <NewsReaderModal
        article={shell === "reader" ? article : null}
        onClose={close}
        context={context}
      />
      <NewsUsHeadlineModal
        article={shell === "us" ? article : null}
        onClose={close}
        context={context}
      />
    </>
  );

  return { article, open, close, modal };
}
