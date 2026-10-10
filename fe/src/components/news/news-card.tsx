"use client";

import type { NewsArticle } from "@/lib/news-types";
import { newsArticleLink, newsUsesReader } from "@/lib/news-types";
import { newsDisplayHeadline } from "@/lib/news-display";
import { newsCardSummary } from "@/lib/news-card-summary";
import { prefetchNewsReader } from "@/lib/news-api";
import { formatRelativeTime } from "@/lib/format-news";
import { isArticleBreakingNews } from "@/lib/news-breaking";
import { NewsCardThumbnail } from "@/components/news/news-card-thumbnail";
import { NewsBreakingBadge } from "@/components/news/news-breaking-badge";
import { NewsKeywordBadges } from "@/components/news/news-keyword-badges";
import { NewsChipRow } from "@/components/news/news-chip-row";
import type { QuoteSummary } from "@/lib/quotes-types";
import type { MarketIndex } from "@/lib/quotes-types";
import { articleDashboardIndexIds } from "@/lib/news-dashboard-indices";
import { cn } from "@/lib/utils";
import { newsTopicLabel } from "@/lib/news-types";

type NewsCardProps = {
  article: NewsArticle;
  onClick?: (article: NewsArticle) => void;
  /** C1 — 종목·지수 칩 시세. 목록 전체가 한 배치 요청을 공유한다. */
  quotes?: Map<string, QuoteSummary>;
  /** 그 배치가 아직 돌아오지 않았다 — 칩이 종목코드를 대신 그리지 않게. */
  quotesLoading?: boolean;
  indices?: MarketIndex[];
};

export function NewsCard({ article, onClick, quotes, quotesLoading, indices }: NewsCardProps) {
  const link = newsArticleLink(article);
  const clickable = Boolean(onClick || link);
  const headline = newsDisplayHeadline(article);
  const summary = newsCardSummary(article);
  const isBreaking = isArticleBreakingNews(article);
  const isUs = article.market === "us";
  const hasChips = Boolean(
    quotes &&
      indices &&
      ((article.symbols?.length ?? 0) > 0 || articleDashboardIndexIds(article).length > 0),
  );

  const handlePrefetch = () => {
    if (!isUs && newsUsesReader(article) && link) {
      prefetchNewsReader(link, { market: article.market, articleId: article.id });
    }
  };

  return (
    <article
      onMouseEnter={handlePrefetch}
      className={cn(
        "relative border-b border-[var(--border)] py-2.5 transition-colors last:border-b-0",
        article.highlighted
          ? "bg-violet-500/[0.04] hover:bg-violet-500/[0.07]"
          : "hover:bg-[var(--surface-elevated)]/40",
        clickable && "cursor-pointer",
      )}
    >
      {/*
        # 카드를 덮는 버튼 (2026-09-06)

        예전에는 `<article>` 자체가 `role="button"`이었다. 그런데 이 카드 안에는
        **종목 칩(링크)** 이 들어 있어 「버튼 안의 링크」가 됐다
        (axe `nested-interactive` — 목록 한 장에 72건).

        제목만 버튼으로 줄이는 방법도 있지만, 뉴스 목록은 카드 아무 데나 눌러 여는
        화면이라 클릭 영역이 제목 두 줄로 줄면 손해가 크다. 그래서 **카드를 덮는
        투명 버튼**을 형제로 깔고 칩만 그 위로 띄운다 — 절대 위치라 흐름을 차지하지
        않으므로 레이아웃은 그대로고, 칩은 자기 클릭을 그대로 받는다.

        버튼에 글자가 없으므로 이름은 제목에서 가져온다.
      */}
      {clickable && (
        <button
          type="button"
          aria-label={headline}
          onClick={() => onClick?.(article)}
          onFocus={handlePrefetch}
          className="absolute inset-0 cursor-pointer bg-transparent focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-[var(--brand)]"
        />
      )}
      <div className="grid min-h-24 grid-cols-[5rem_minmax(0,1fr)] items-center gap-3">
        <NewsCardThumbnail imageUrl={article.imageUrl} compact />

        <div className="flex max-h-24 min-w-0 flex-col justify-center gap-1 overflow-hidden">
          {/*
            # 메타 줄은 **칸이 정해진 격자다** (2026-09-05 사용자 지적 3)

            예전에는 flex 한 줄이었다. 언론사 배지가 **자기 글자만큼** 넓어지므로
            그 뒤에 붙는 키워드 배지의 시작점이 기사마다 달랐다 — 실측(375px):
            「매일경제」(56px) 뒤 키워드는 x=164, 「hosu-news」(75px) 뒤 키워드는
            x=183. 목록을 세로로 훑으면 배지 기둥이 19px씩 좌우로 흔들린다.

            그래서 칸을 나눈다: 언론사 · 키워드 나머지 · 시각 자기 폭.

            # 언론사 칸의 **고정폭은 걷었다** (2026-09-06 사용자 지적 1 · B안)

            고정폭(4.75rem)은 배지 기둥을 세워 주는 대신 **모바일에서 값이 너무 비쌌다.**
            실측(375px): 글 칸 259px 중 언론사가 76px(29%)을 늘 가져가고, 키워드 칸이
            139px(54%)인데 **키워드가 없는 기사에서는 그 139px이 통째로 빈칸**이었다.
            메타 줄 하나가 가로 한 줄을 다 차지하는 것처럼 보인 이유가 이것이다.

            지금은 배지가 **자기 글자만큼만** 넓어진다(`auto`). 대신 상한은 예전
            고정폭 그대로 4.75rem이라, 긴 이름(「파이낸셜뉴스」·FinancialJuice)의
            말줄임 지점은 바뀌지 않는다 — 목록이 예전보다 넓어지는 일은 없고 좁아지기만
            한다. 글자는 왼쪽으로 붙인다: 폭이 내용에 맞으므로 가운데 정렬로 메울 빈
            칸이 더는 없다.

            기둥 정렬은 여기서 포기한 것이 맞다. 색이 서로 다른 배지가 25종이라
            (`lib/news-publishers`) 시작점이 같은 것보다 **줄이 짧은 것**이 좁은 화면에서
            더 크게 벌어 준다는 사용자 판단이다.

            키워드 묶음은 `flex-nowrap`이다. 줄바꿈을 허용하면 두 번째 줄이 부모의
            `max-h`에 잘려, 어떤 기사만 배지가 반쯤 보이는 상태가 된다.
          */}
          <div className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-1">
            <span
              title={article.publisher.name}
              className={cn(
                "inline-flex max-w-[4.75rem] items-center truncate rounded-md border px-1.5 py-px text-micro font-semibold",
                article.publisher.badgeClass,
              )}
            >
              {article.publisher.name}
            </span>
            <div className="flex min-w-0 items-center gap-1 overflow-hidden">
              {isBreaking && <NewsBreakingBadge className="px-1.5 py-px text-micro" />}
              {newsTopicLabel(article.topic) && (
                <span className="shrink-0 rounded-md bg-[var(--brand)]/10 px-1.5 py-px text-micro font-semibold text-[var(--brand)]">
                  {newsTopicLabel(article.topic)}
                </span>
              )}
              <NewsKeywordBadges
                keywords={article.keywords}
                max={2}
                className="min-w-0 flex-nowrap gap-0.5 overflow-hidden [&_span]:max-w-[4.5rem] [&_span]:px-1.5 [&_span]:py-px [&_span]:text-micro"
              />
            </div>
            <time
              dateTime={article.publishedAt}
              className="text-right text-micro text-[var(--text-muted)]"
              /* 상대시각은 SSR 과 hydration 의 Date.now() 가 달라 분 경계에서
                 어긋난다(React #418). 이 자리의 정답은 이 속성이다 — 값은
                 hydration 때 클라이언트 것으로 맞춰진다. */
              suppressHydrationWarning
            >
              {formatRelativeTime(article.publishedAt)}
            </time>
          </div>

          {/*
            `shrink-0`이 이 줄의 핵심이다.

            부모가 `max-h`로 묶인 flex 컬럼이라, 내용이 예산을 넘으면 flex가 항목을
            **눌러서** 맞춘다. 그러면 `line-clamp-2`가 두 줄을 그려 놨는데 상자만
            한 줄 반으로 줄어들고, `overflow-hidden`이 남은 반 줄을 글자 중간에서
            잘라 냈다(실측: 2줄 36px이 필요한데 h2가 26.6px). 잘릴 것을 정하는 건
            줄임표를 붙이는 `line-clamp`여야지 상자를 미는 힘이 아니다.
          */}
          <h2 className="line-clamp-2 shrink-0 text-note font-bold leading-snug text-[var(--text-primary)]">
            {headline}
          </h2>

          {hasChips ? (
            /*
             * 종목이 걸린 기사는 요약 대신 종목 칩 줄을 쓴다. 둘 다 넣으면 행 높이가
             * 기사마다 달라져 목록이 들쭉날쭉해진다 — 시세가 요약보다 값이 크다.
             */
            /*
              덮개 버튼 위로 띄운다 — 안 그러면 칩이 아니라 카드가 눌린다.

              **한 줄을 넘지 않는다** (2026-09-11 사용자 요청 4). 예전에는 칩 묶음이
              `flex-wrap`이라 좁은 화면에서 두 줄이 됐고, 이 글 칸은 `max-h-24`로 묶여
              있어 둘째 줄이 가로로 베였다(실측 375px: 96px 칸에 내용 101px, 83행 중
              15행). 몇 개가 들어가는지는 `NewsChipRow`가 재서 정하고 나머지는 「+N」이
              말한다 — 스무 줄 위 키워드 묶음의 `flex-nowrap`과 같은 규칙이다.

              폭은 **글 칸 전체**다(2026-10-09 요청 4). `w-fit`이면 이 칸의 폭이 「지금 그린 칩」의 폭이 되고,
              NewsChipRow는 그 폭으로 몇 개가 들어가는지 잰다 — 한 번 0개가 되면 13px(「+2」)로 줄어
              다시는 칩이 안 나왔다(실측: 국내 뉴스 목록 거의 모든 행이 「+N」만). 빈자리는 아래 덮개 버튼이 받게
              클릭을 통과시키고, 칩 링크만 받는다.
            */
            <div className="pointer-events-none relative z-10 w-full [&_a]:pointer-events-auto">
              <NewsChipRow
                article={article}
                quotes={quotes!}
                quotesLoading={quotesLoading}
                indices={indices!}
              />
            </div>
          ) : (
            summary && (
              /*
                좁은 화면에서는 요약이 한 줄이다. 제목 2줄을 온전히 지키고 남는
                예산이 딱 한 줄이라, 두 줄을 그려 봐야 둘째 줄이 잘려 나간다.
              */
              <p className="line-clamp-1 text-cell leading-snug text-[var(--text-secondary)] sm:line-clamp-2">
                {summary}
              </p>
            )
          )}
        </div>
      </div>
    </article>
  );
}
