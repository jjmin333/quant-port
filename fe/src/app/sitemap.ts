import type { MetadataRoute } from "next";
import { connection } from "next/server";
import { SITE_URL } from "@/lib/seo";
import { fetchResearchCallsServer } from "@/lib/research-api.server";
import { fetchCommunitySitemapServer } from "@/lib/community-api.server";
import { isAdminOnlyRoute, isHiddenRoute } from "@/lib/hidden-features";
import { fetchChangesServer } from "@/components/changes/changes-seo-list";
import { fetchIndexableStocks } from "@/lib/stock-seo.server";
import { comparePath, groupBySector, sectorPath, stylePath } from "@/lib/stock-seo";
import { STOCK_STYLES } from "@/lib/stock-style";

/*
 * **빌드 때 미리 굽지 않는다** (2026-10-09). 예전 `revalidate = 600`은 `next build`가 사이트맵을
 * 미리 렌더했는데, 빌드 컨테이너에서는 BE에 닿지 않아 종목·리포트가 빈 채로 구워졌다 — 실측:
 * 배포 직후 사이트맵이 정적 주소 21개뿐이었고, 그 판이 첫 재검증(10분)까지, nginx·Cloudflare
 * 캐시(1시간)를 타면 더 오래 나갔다. 배포가 잦아 구글이 그 판을 받을 확률이 작지 않다.
 * 이제 요청 때 그리고, BE 호출은 각 fetch의 `revalidate`(리포트 1분 · 종목 1시간)로 접힌다.
 */

/**
 * `/sitemap.xml` — **정적 공개 URL만** 싣는다.
 *
 * 동적 주소는 서버가 실제 글자를 들고 있는 것만 싣는다.
 * - `/stocks/<종목>`: **시장별(국내·미국) 최근 20거래일 거래대금 상위 500씩만** 싣는다(2026-09-24 사용자
 *   결정, 10-09 시장별 몫). 그 종목들만 서버가 요약 한 줄을 그린 채 색인 대상이고, 나머지 1만 5천 개는
 *   여전히 `noindex`다(`app/(main)/stocks/[symbol]/page.tsx`). 목록은 BE가 한 시간 접어 둔다.
 * `/reports/<id>`는 **싣는다.** 공개 발행물이고, 이 앱에서 서버가 실제 글자를
 * 들고 있는 유일한 콘텐츠다. 목록을 부르느라 BE를 때리는 것이 걱정이었는데
 * `revalidate`로 접으니 주기당 한 번이라 문제가 되지 않는다. **공개·비철회·비숨김만**
 * 싣는다 — 「링크 공개(unlisted)」를 사이트맵에 넣으면 작성자가 고른 설정을
 * 우리가 뒤집는 것이 된다.
 *
 * `lastModified`를 빌드 시각으로 주지 않는다 — 내용이 안 바뀐 배포에서도 날짜만
 * 새로 찍히면 크롤러에게 거짓말을 하는 셈이고, 반복되면 이 신호를 무시하게 된다.
 */
const ROUTES: { path: string; changeFrequency: MetadataRoute.Sitemap[number]["changeFrequency"]; priority: number }[] = [
  { path: "/", changeFrequency: "daily", priority: 1 },
  { path: "/news/domestic", changeFrequency: "hourly", priority: 0.9 },
  { path: "/news/us", changeFrequency: "hourly", priority: 0.9 },
  { path: "/earnings/domestic", changeFrequency: "daily", priority: 0.8 },
  { path: "/earnings/us", changeFrequency: "daily", priority: 0.8 },
  { path: "/earnings/surprises", changeFrequency: "daily", priority: 0.7 },
  { path: "/market", changeFrequency: "hourly", priority: 0.8 },
  { path: "/analysis", changeFrequency: "daily", priority: 0.7 },
  // 색인·canonical을 내보내면서 사이트맵에서 빠져 있던 공개 화면(2026-10-09 색인 점검).
  { path: "/stocks", changeFrequency: "daily", priority: 0.8 },
  { path: "/screener", changeFrequency: "daily", priority: 0.7 },
  { path: "/changes", changeFrequency: "daily", priority: 0.7 },
  { path: "/finance/rates", changeFrequency: "daily", priority: 0.6 },
  { path: "/real-estate", changeFrequency: "weekly", priority: 0.6 },
  { path: "/real-estate/auctions", changeFrequency: "daily", priority: 0.5 },
  { path: "/community", changeFrequency: "hourly", priority: 0.7 },
  { path: "/reports", changeFrequency: "daily", priority: 0.7 },
  { path: "/finance/savings", changeFrequency: "daily", priority: 0.6 },
  { path: "/finance/loans", changeFrequency: "daily", priority: 0.6 },
  /*
   * 약관·개인정보처리방침은 검색 유입을 노리는 글이 아니다. 그래도 싣는 이유는
   * **투자 정보가 YMYL**이라, 운영 주체와 약관이 보이는 사이트인지가 품질 평가에
   * 직접 들어가기 때문이다. 우선순위는 낮게 둔다.
   */
  { path: "/legal/terms", changeFrequency: "yearly", priority: 0.2 },
  { path: "/legal/privacy", changeFrequency: "yearly", priority: 0.2 },
];

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  await connection();
  const staticUrls = ROUTES.map(({ path, changeFrequency, priority }) => ({
    url: `${SITE_URL}${path}`,
    changeFrequency,
    priority,
  }));

  // 실패하면 빈 배열이라 정적 URL만 남는다 — 사이트맵이 통째로 깨지지 않는다.
  const reports = await fetchResearchCallsServer(200);
  const reportUrls = reports
    .filter((call) => call.visibility === "public" && !call.retractedAt && !call.hiddenAt)
    .map((call) => ({
      url: `${SITE_URL}/reports/${call.id}`,
      // 여기서는 진짜 수정 시각을 안다. 정적 URL과 달리 거짓말이 아니다.
      lastModified: new Date(call.updatedAt || call.publishedAt),
      changeFrequency: "weekly" as const,
      priority: 0.6,
    }));

  // 공개 리포트가 있는 작성자 — 작성자 페이지는 그때만 색인한다(2026-10-09).
  const authorUrls = [...new Set(reports
    .filter((call) => call.visibility === "public" && !call.retractedAt && !call.hiddenAt)
    .map((call) => call.userId))]
    .map((userId) => ({
      url: `${SITE_URL}/reports/authors/${userId}`,
      changeFrequency: "weekly" as const,
      priority: 0.4,
    }));

  // 실패하면 빈 목록 — 종목 주소만 빠지고 나머지는 그대로 나간다.
  const indexable = await fetchIndexableStocks();
  const stockUrls = indexable.map(({ symbol }) => ({
    url: `${SITE_URL}/stocks/${encodeURIComponent(symbol)}`,
    changeFrequency: "daily" as const,
    priority: 0.5,
  }));

  // 업종 순위 · 스타일 · 업종 대표 비교(2026-10-10 사업화 검토 — 롱테일). 비교는 업종마다 거래대금 1위와 2~5위 네 쌍만
  // 싣는다 — 종목 페이지가 가까운 순위 셋과 잇는 링크로 나머지는 발견된다.
  const listUrls: MetadataRoute.Sitemap = STOCK_STYLES.map((st) => ({
    url: `${SITE_URL}${stylePath(st.slug)}`,
    changeFrequency: "daily" as const,
    priority: 0.6,
  }));
  for (const assetType of ["domestic", "us"]) {
    for (const g of groupBySector(indexable, assetType)) {
      if (g.sector === "기타") continue;
      listUrls.push({ url: `${SITE_URL}${sectorPath(assetType, g.sector)}`, changeFrequency: "daily", priority: 0.6 });
      for (const other of g.stocks.slice(1, 5)) {
        listUrls.push({ url: `${SITE_URL}${comparePath(g.stocks[0].symbol, other.symbol)}`, changeFrequency: "daily", priority: 0.4 });
      }
    }
  }

  // 커뮤니티 글 — 공개 글만(서버가 visible만 준다), 최근 1,000개. 검색 색인 허용(2026-09-27 결정).
  const communityUrls = (await fetchCommunitySitemapServer()).map((row) => ({
    url: `${SITE_URL}/community/${row.id}`,
    lastModified: new Date(row.updatedAt),
    changeFrequency: "weekly" as const,
    priority: 0.5,
  }));

  // 날짜별 「변화」 보관 주소(2026-10-10) — 서버가 보관 중인 날짜만. 지난 날은 바뀌지 않는다.
  const changeDates = (await fetchChangesServer("domestic"))?.dates ?? [];
  const changeUrls = changeDates.map((d) => ({
    url: `${SITE_URL}/changes/${d}`,
    lastModified: new Date(`${d}T09:00:00+09:00`),
    changeFrequency: "monthly" as const,
    priority: 0.4,
  }));

  // 노출을 끈 기능(lib/hidden-features, 2026-10-10)의 주소는 싣지 않는다 — 열면 홈으로 간다.
  return [...staticUrls, ...reportUrls, ...authorUrls, ...communityUrls, ...changeUrls, ...listUrls, ...stockUrls]
    .filter((entry) => {
      const path = new URL(entry.url).pathname;
      return !isHiddenRoute(path) && !isAdminOnlyRoute(path);
    });
}
