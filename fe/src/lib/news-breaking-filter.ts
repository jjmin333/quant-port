import { isArticleBreakingNews } from "@/lib/news-breaking";
import type { NewsArticle } from "@/lib/news-types";

/**
 * 「속보만」 필터가 고르는 기사들 (2026-09-05 사용자 요청 5).
 *
 * # 왜 함수 하나인가
 *
 * 화면은 이 목록을 **두 번** 쓴다 — 칩에 적을 건수(「🚨 속보 3」)와, 눌렀을 때 그릴
 * 목록. 두 자리에서 각자 `filter`를 쓰면 언젠가 한쪽만 조건이 바뀌어 **적힌 수와
 * 나오는 수가 어긋난다**. 그 어긋남은 「3건이라며 눌렀더니 1건」으로 보이고, 사용자는
 * 필터가 아니라 데이터를 의심하게 된다.
 *
 * # 시각을 인자로 받는 이유
 *
 * 속보 판정에는 **시간 창**이 있다(`isArticleBreakingNews` → `BREAKING_MAX_AGE_MS`).
 * 「[속보]」가 붙었어도 오래된 기사는 속보가 아니다. `Date.now()`를 안에서 부르면
 * 같은 목록이 1초 뒤에 다른 답을 낼 수 있고, 무엇보다 테스트가 시계에 매달린다.
 */
export function selectBreakingArticles(
  articles: NewsArticle[],
  nowMs: number = Date.now(),
): NewsArticle[] {
  return articles.filter((article) => isArticleBreakingNews(article, nowMs));
}
