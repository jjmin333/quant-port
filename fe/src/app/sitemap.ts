import type { MetadataRoute } from "next";
import { SITE_URL } from "@/lib/seo";
import { fetchResearchCallsServer } from "@/lib/research-api.server";

/**
 * 사이트맵도 리포트 목록과 같은 주기로 다시 굽는다. 크롤러가 몇 번을 오든 BE
 * 호출은 주기당 한 번이라, 작은 인프라에서도 부담이 되지 않는다.
 */
export const revalidate = 600;

/**
 * `/sitemap.xml` — **정적 공개 URL만** 싣는다.
 *
 * 동적 주소를 뺀 이유가 둘로 갈린다.
 * - `/stocks/<종목>`: 수천 개인데 전부 클라이언트에서 데이터를 받아 그린다.
 *   크롤러가 받는 HTML에는 종목 내용이 없어서, 사이트맵으로 불러 봐야 빈 문서
 *   수천 장을 색인시키는 꼴이 된다. 그래서 `noindex`이기도 하다.
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
  { path: "/market", changeFrequency: "hourly", priority: 0.8 },
  { path: "/analysis", changeFrequency: "daily", priority: 0.7 },
  { path: "/reports", changeFrequency: "daily", priority: 0.7 },
  /*
   * 약관·개인정보처리방침은 검색 유입을 노리는 글이 아니다. 그래도 싣는 이유는
   * **투자 정보가 YMYL**이라, 운영 주체와 약관이 보이는 사이트인지가 품질 평가에
   * 직접 들어가기 때문이다. 우선순위는 낮게 둔다.
   */
  { path: "/legal/terms", changeFrequency: "yearly", priority: 0.2 },
  { path: "/legal/privacy", changeFrequency: "yearly", priority: 0.2 },
];

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
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

  return [...staticUrls, ...reportUrls];
}
