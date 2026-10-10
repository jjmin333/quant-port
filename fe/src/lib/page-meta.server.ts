import type { Metadata } from "next";
import { getApiBaseUrl } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import { mergePageMeta, type ResolvedPageMeta } from "@/lib/page-meta-merge";
import { pageRoute } from "@/lib/page-meta-routes";
import type { PageMetaOverride } from "@/lib/page-meta-types";
import { NOINDEX, OG_BASE, TWITTER_BASE } from "@/lib/seo";
import { fetchWithDeadline } from "@/lib/fetch-timeout";

/**
 * 화면 제목·설명을 **DB의 덮어쓰기와 합쳐** 만든다 (2026-09-15 사용자 요청 2).
 *
 * # 읽는 자리가 왜 서버인가
 *
 * `generateMetadata`는 HTML의 `<head>`를 만드는 자리라 브라우저가 아니라 Next 서버에서
 * 돈다. 크롤러가 받는 것이 이 결과이고, 그래서 클라이언트에서 나중에 고치는 방법은
 * 애초에 답이 아니다(네이버 Yeti는 JS를 사실상 안 돌린다 — `research-api.server` 머리글).
 *
 * # 캐시가 이 파일의 핵심이다
 *
 * 모든 화면이 렌더마다 이 표를 읽으면 인프라가 작은 쪽(OCI Always Free · DB 1 ECPU)이
 * 먼저 무너진다. **목록 한 벌**을 `revalidate`로 접어 두면 화면이 몇 개든 BE 호출은
 * 주기당 하나다. 관리자가 저장한 값은 최대 그 주기만큼 늦게 반영된다 — 검색 문구는
 * 초 단위로 급한 자료가 아니고, 급하면 관리 화면의 안내대로 잠시 기다리면 된다.
 *
 * # 실패는 삼킨다
 *
 * BE가 죽어 있어도 화면은 **코드의 기본값으로** 그려져야 한다. 제목을 못 읽었다고
 * 페이지가 500이 되면, 고치려고 만든 장치가 사이트를 세우는 장치가 된다.
 */

/** 루트 레이아웃이 이 타입을 쓴다 — 읽는 자리와 합치는 자리를 한 이름으로 잇는다. */
export type { ResolvedPageMeta };

/**
 * 덮어쓰기 한 벌을 접어 두는 시간.
 *
 * **1분이다.** Next의 재검증은 stale-while-revalidate라, 시간이 지난 뒤 첫 요청은
 * 아직 옛 값을 받고 그 다음 요청부터 새 값을 받는다 — 주기를 5분으로 두면 관리자가
 * 저장하고 최악 10분을 기다리며 「저장이 안 됐나」를 의심한다.
 *
 * 1분으로 두어도 BE 호출은 **Next 서버당 분당 한 번**이고 응답은 수십 바이트다.
 * 화면 수와 무관하다는 것이 이 캐시의 요점이므로, 주기를 줄여도 비용은 늘지 않는다.
 */
const PAGE_META_REVALIDATE = 60;

const EMPTY: PageMetaOverride[] = [];

async function fetchOverrides(): Promise<PageMetaOverride[]> {
  try {
    const res = await fetchWithDeadline(`${getApiBaseUrl()}/api/v1/page-meta`, {
      // 쿠키 없음 = 익명. 이 표는 공개 문구뿐이라 사용자마다 다를 것이 없다.
      next: { revalidate: PAGE_META_REVALIDATE },
    });
    if (!res.ok) return EMPTY;
    const body = (await res.json()) as ApiResponse<{ items?: PageMetaOverride[] }>;
    return body.data?.items ?? EMPTY;
  } catch {
    return EMPTY;
  }
}

async function overrideFor(path: string): Promise<PageMetaOverride | null> {
  const items = await fetchOverrides();
  return items.find((item) => item.path === path) ?? null;
}

/**
 * `path` 화면의 최종 문구. 목록(`lib/page-meta-routes`)에 없는 주소면 `null`.
 *
 * 호출부가 없는 주소를 넘긴 것은 실수이고, 조용히 빈 제목을 내는 것보다 기본값
 * 그대로가 낫다 — `resolvePageMetadata`는 그때 빈 객체를 돌려준다.
 *
 * 합치는 규칙 자체는 `lib/page-meta-merge`에 있다(그쪽이 시험을 받는다). 여기는
 * **어디서 읽어 오는가**만 안다.
 */
export async function resolvePageMeta(path: string): Promise<ResolvedPageMeta | null> {
  const route = pageRoute(path);
  if (!route) return null;
  return mergePageMeta(route, await overrideFor(path));
}

/** `path` 화면이 `generateMetadata`에서 그대로 돌려줄 메타 한 벌. */
export async function resolvePageMetadata(path: string): Promise<Metadata> {
  const resolved = await resolvePageMeta(path);
  if (!resolved) return {};
  const { title, description, indexable, cardTitle, cardDescription, cardImage } = resolved;

  return {
    title,
    description,
    /*
      canonical은 **색인하는 화면에만** 붙인다. 색인하지 말라고 해 놓고 정규 주소를
      다는 것은 모순이고, 그 모순은 검색엔진이 둘 중 하나를 골라 버리는 것으로 끝난다.
      템플릿 주소(`/stocks/[symbol]`)는 실제 주소가 아니라 canonical을 만들 수 없다.
    */
    ...(indexable && !path.includes("[") ? { alternates: { canonical: path } } : {}),
    ...(indexable ? {} : { robots: NOINDEX }),
    openGraph: {
      ...OG_BASE,
      ...(cardImage ? { images: [{ url: cardImage }] } : {}),
      title: cardTitle,
      description: cardDescription,
      ...(indexable && !path.includes("[") ? { url: path } : {}),
    },
    twitter: {
      ...TWITTER_BASE,
      ...(cardImage ? { images: [cardImage] } : {}),
      title: cardTitle,
      description: cardDescription,
    },
  };
}
