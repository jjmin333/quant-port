import { describe, expect, it } from "vitest";
import { selectBreakingArticles } from "@/lib/news-breaking-filter";
import { NEWS_PUBLISHERS } from "@/lib/news-publishers";
import type { NewsArticle, NewsMarket } from "@/lib/news-types";

const NOW = Date.parse("2026-09-05T22:00:00+09:00");
const HOUR = 60 * 60 * 1000;

function article(
  over: Partial<NewsArticle> & { id: string; headline: string; hoursAgo: number },
): NewsArticle {
  const { hoursAgo, ...rest } = over;
  return {
    market: "domestic" as NewsMarket,
    headlineKo: undefined,
    publisher: NEWS_PUBLISHERS[0],
    source: {
      id: "rss",
      name: "RSS",
      market: "domestic",
      badgeClass: "",
    },
    tags: [],
    publishedAt: new Date(NOW - hoursAgo * HOUR).toISOString(),
    viewCount: 0,
    ...rest,
  } as NewsArticle;
}

describe("selectBreakingArticles", () => {
  it("제목에 [속보]가 붙은 최근 기사를 고른다", () => {
    const rows = [
      article({ id: "a", headline: "[속보] 코스피 3,000 돌파", hoursAgo: 1 }),
      article({ id: "b", headline: "코스피 마감 시황", hoursAgo: 1 }),
    ];
    expect(selectBreakingArticles(rows, NOW).map((a) => a.id)).toEqual(["a"]);
  });

  it("`속보` 태그도 같은 규칙으로 걸린다", () => {
    const rows = [article({ id: "a", headline: "환율 급등", tags: ["속보"], hoursAgo: 2 })];
    expect(selectBreakingArticles(rows, NOW)).toHaveLength(1);
  });

  it("24시간이 지난 속보는 더 이상 속보가 아니다", () => {
    // 시간 창이 이 필터의 핵심이다 — 없으면 어제 [속보]가 오늘 목록에 계속 선다.
    const rows = [
      article({ id: "fresh", headline: "[속보] 방금", hoursAgo: 23 }),
      article({ id: "stale", headline: "[속보] 어제", hoursAgo: 25 }),
    ];
    expect(selectBreakingArticles(rows, NOW).map((a) => a.id)).toEqual(["fresh"]);
  });

  it("속보가 없으면 빈 배열 — 칩이 0을 적고 잠기는 근거다", () => {
    const rows = [article({ id: "a", headline: "평범한 기사", hoursAgo: 1 })];
    expect(selectBreakingArticles(rows, NOW)).toEqual([]);
  });

  it("건수와 목록 길이는 같은 함수에서 나온다 — 어긋날 수 없다", () => {
    const rows = [
      article({ id: "a", headline: "[속보] 하나", hoursAgo: 1 }),
      article({ id: "b", headline: "【속보】 둘", hoursAgo: 3 }),
      article({ id: "c", headline: "보통 기사", hoursAgo: 1 }),
    ];
    const picked = selectBreakingArticles(rows, NOW);
    expect(picked).toHaveLength(2);
    expect(selectBreakingArticles(rows, NOW).length).toBe(picked.length);
  });

  it("빈 목록에도 죽지 않는다", () => {
    expect(selectBreakingArticles([], NOW)).toEqual([]);
  });
});
