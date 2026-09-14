import type { MetadataRoute } from "next";
import { SITE_URL } from "@/lib/seo";

/**
 * `/robots.txt`.
 *
 * **robots의 Disallow는 색인 금지가 아니라 크롤링 금지다.** 막힌 주소도 다른
 * 곳에서 링크되면 제목 없이 색인될 수 있다. 그래서 개인 데이터·관리자 화면은
 * 여기서 막는 것과 **별개로** 각 화면에 `noindex` 메타를 함께 붙였다
 * (`lib/seo.ts`의 `NOINDEX`). 둘 중 하나만으로는 새지 않는다는 보장이 없다.
 *
 * **`/api/`를 통째로 막았다가 되돌렸다(2026-09-08).** BE 레이트 리밋을 아끼려는
 * 판단이었는데, 그 대가가 너무 컸다 — 이 앱의 화면은 내용을 전부 `/api/v1/...`에서
 * 받아 그리고, **구글은 렌더링 중에 페이지가 부르는 fetch에도 robots.txt를 적용한다.**
 * 즉 API를 막으면 구글 렌더러가 페이지를 열어도 데이터를 못 받아 영원히
 * 「불러오는 중…」만 본다(실측: 색인 대상 8페이지 전부 본문이 사이드바뿐이었다).
 *
 * 그래서 **공개 조회 경로만 열고 나머지는 계속 막는다.** 여기 없는 것(인증·개인
 * 데이터·관리자·업로드)은 어차피 401이라 크롤러가 얻을 게 없고, robots.txt는
 * 공개 파일이므로 비공개 경로를 굳이 나열해 알려 주지도 않는다.
 */
const PUBLIC_API_PREFIXES = [
  "/api/v1/news/",
  "/api/v1/earnings/",
  "/api/v1/research/",
  "/api/v1/stocks/",
  "/api/v1/quotes/",
  "/api/v1/rankings/",
  "/api/v1/indices/",
  "/api/v1/market/",
  "/api/v1/media/",
];

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      // 구글은 Disallow/Allow 중 **더 긴 쪽**을 따른다. 그래서 `/api/`를 막아 두고
      // 공개 접두어를 Allow로 되살리는 이 조합이 성립한다.
      allow: ["/", ...PUBLIC_API_PREFIXES],
      disallow: [
        "/api/",
        "/admin",
        "/dev",
        // 개인 데이터 — 로그인해야 내용이 있으므로 크롤러에게는 빈 화면이다.
        "/holdings",
        "/watchlist",
        "/journal",
        "/predictions",
        "/stamps",
        "/targets",
        "/settings",
        "/login",
        "/signup",
        // 글쓰기 화면. 리포트 본문(`/reports/<id>`)은 공개 색인 대상이다.
        "/reports/new",
        "/reports/authors/",
      ],
    },
    sitemap: `${SITE_URL}/sitemap.xml`,
    host: SITE_URL,
  };
}
