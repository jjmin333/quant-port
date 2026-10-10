import type { Metadata } from "next";

/**
 * 검색·SNS 노출에 쓰는 값 한 벌.
 *
 * **여기 말고 다른 데서 도메인을 문자열로 쓰지 않는다.** 예전에 전역 메타에만
 * 있던 설명("여러 증권사 계좌를 하나로 통합하는 투자 플랫폼")은 토스 계좌 연동을
 * 걷어낸 뒤로도 그대로 남아 **없는 기능을 검색 결과에 광고하고 있었다**. 값이 한
 * 곳에 있으면 다음에 포지셔닝이 바뀔 때 고칠 자리가 하나다.
 */
export const SITE_URL = "https://www.quant-port.com";
export const SITE_NAME = "Quant Port";

/**
 * GA4 측정 ID (2026-09-25 사용자 결정으로 설치).
 *
 * 공개 값이다 — 모든 방문자의 HTML에 그대로 실린다. 비밀은 GA 데이터 API용 서비스 계정이고
 * 그것은 FE가 아니라 worker-growth만 읽는다(docs/GROWTH.md).
 */
export const GA_MEASUREMENT_ID = "G-T8RNF91MRH";

/**
 * 홈 title/description.
 *
 * 2026-10-10 포지셔닝 교체: 「흩어진 기록을 한곳에」(보유 관리)에서 **국내·미국 전 종목
 * 점수와 그 설명**으로(docs/BUSINESS_REVIEW.md). 홈 히어로 문구와 같은 말을 쓴다 —
 * 검색 결과 문구와 착지한 화면이 달랐다.
 */
export const SITE_TITLE = "Quant Port — 이 종목, 같은 업종 안에서 몇 등일까";
export const SITE_DESCRIPTION =
  "국내·미국 전 종목을 수익성·성장·가치·추세·수급 다섯 축으로 매일 채점합니다. " +
  "같은 업종 안 순위, 점수가 바뀐 이유, 점수의 과거 성적, 실적 일정과 서프라이즈를 한 화면에서. 투자 권유가 아닙니다.";

/**
 * 색인하지 않는 화면에 붙인다 — 관리자·개발용·로그인·개인 데이터.
 *
 * `follow`까지 끄는 이유: 이 화면들에서 크롤러가 따라갈 만한 공개 링크가 없다.
 * 개인 데이터 화면은 로그인 전에는 빈 껍데기라 크롤러에게는 애초에 내용이 없고,
 * 관리자 화면은 링크를 따라가게 두면 관리 경로 목록만 수집된다.
 */
export const NOINDEX: Metadata["robots"] = {
  index: false,
  follow: false,
  googleBot: { index: false, follow: false },
};

/**
 * 화면마다 다시 깔아야 하는 og/twitter 밑값.
 *
 * **Next의 메타 병합이 얕아서** 화면이 `openGraph`를 하나라도 적으면 루트에
 * 적어 둔 `siteName`·`locale`·`type`이 통째로 사라진다. 그래서 밑값을 상수로
 * 빼 두고 화면 쪽에서 펼쳐 넣는다(`pageMetadata`가 대신 해 준다).
 * **`images`도 여기 있어야 한다.** `app/opengraph-image.tsx`(파일 규칙)가 붙여
 * 주는 그림은 화면이 `openGraph`를 직접 적는 순간 **같이 지워진다** — 실측으로
 * 확인했다(`/news/domestic`, `/market`에 og:image가 통째로 없었다). 그러면 홈
 * 말고는 카카오톡·슬랙에 붙일 때 그림 없는 카드가 나간다.
 */
const OG_IMAGE = "/opengraph-image";

// `as const`가 아니라 `satisfies`인 이유: `as const`는 배열까지 readonly로 얼려서
// Next의 `images` 타입(가변 배열)에 못 넣는다. `satisfies`는 `type: "website"`의
// 리터럴 좁힘은 유지하면서 배열은 가변으로 남긴다.
export const OG_BASE = {
  type: "website",
  siteName: SITE_NAME,
  locale: "ko_KR",
  images: [{ url: OG_IMAGE, width: 1200, height: 630, alt: SITE_NAME }],
} satisfies NonNullable<Metadata["openGraph"]>;

export const TWITTER_BASE = {
  card: "summary_large_image",
  images: [OG_IMAGE],
} satisfies NonNullable<Metadata["twitter"]>;

type PageMetaInput = {
  title: string;
  description: string;
  /** 이 화면의 정규 주소. 넘기면 canonical과 og:url이 함께 붙는다. */
  path?: string;
  /** 색인하지 않을 화면 */
  noindex?: boolean;
};

/**
 * 페이지별 메타 한 줄짜리 헬퍼.
 *
 * canonical을 `path`에서 **함께** 만드는 게 요점이다. 따로 적게 두면 한쪽만
 * 고쳐지고, 그 순간 구글은 두 주소를 다른 문서로 센다.
 */
export function pageMetadata({ title, description, path, noindex }: PageMetaInput): Metadata {
  return {
    title,
    description,
    ...(path ? { alternates: { canonical: path } } : {}),
    ...(noindex ? { robots: NOINDEX } : {}),
    openGraph: {
      ...OG_BASE,
      title: `${title} · ${SITE_NAME}`,
      description,
      ...(path ? { url: path } : {}),
    },
    twitter: { ...TWITTER_BASE, title: `${title} · ${SITE_NAME}`, description },
  };
}

/* ------------------------------------------------------------------------- *
 * 구조화 데이터 (JSON-LD)
 *
 * 화면에 안 보이는 대신 **검색엔진이 문서의 뼈대를 추측하지 않아도 되게** 한다.
 * 값은 전부 화면에 실제로 그려지는 것에서 나와야 한다 — 본문에 없는 사실을
 * 여기에만 적으면 구조화 데이터 정책 위반이고, 리치 결과에서 통째로 빠진다.
 * ------------------------------------------------------------------------- */

/** `@/lib/research-types`를 여기서 import하지 않으려고 최소 형태만 받는다. */
type ArticleLike = {
  title: string;
  bodyMd: string;
  publishedAt: string;
  updatedAt?: string;
  authorName?: string;
  thumbnailUrl?: string;
  tags?: string[];
};

export function articleJsonLd(article: ArticleLike, path: string) {
  const url = `${SITE_URL}${path}`;
  return {
    "@context": "https://schema.org",
    "@type": "Article",
    "@id": `${url}#article`,
    mainEntityOfPage: url,
    url,
    headline: article.title,
    datePublished: article.publishedAt,
    ...(article.updatedAt ? { dateModified: article.updatedAt } : {}),
    /*
     * 작성자가 없으면 `author`를 아예 빼는 게 맞다 — 사이트 이름을 대신 넣으면
     * 사람이 쓴 글의 저자를 우리가 지어내는 셈이 된다.
     */
    ...(article.authorName ? { author: { "@type": "Person", name: article.authorName } } : {}),
    publisher: { "@id": `${SITE_URL}/#organization` },
    ...(article.thumbnailUrl ? { image: article.thumbnailUrl } : {}),
    ...(article.tags?.length ? { keywords: article.tags.join(", ") } : {}),
    inLanguage: "ko-KR",
  };
}

/**
 * 검색 결과에서 주소 대신 「Quant Port › 투자 리포트 › 글 제목」으로 보이게 한다.
 * 홈은 넣는 쪽에서 적지 않는다 — 여기서 맨 앞에 붙인다.
 */
export function breadcrumbJsonLd(trail: { name: string; path: string }[]) {
  const items = [{ name: SITE_NAME, path: "/" }, ...trail];
  return {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: items.map((item, i) => ({
      "@type": "ListItem",
      position: i + 1,
      name: item.name,
      item: `${SITE_URL}${item.path}`,
    })),
  };
}
