import type { PageRoute } from "@/lib/page-meta-routes";
import type { PageMetaOverride } from "@/lib/page-meta-types";
import { SITE_NAME } from "@/lib/seo";

/*
코드의 기본값 위에 DB의 덮어쓰기를 얹는 **규칙 하나** (2026-09-15 사용자 요청 2).

읽어 오는 일(`lib/page-meta.server`)과 나눈 이유는 시험이다 — 그쪽은 `fetch`와 Next의
캐시를 쓰므로 node 환경의 vitest에서 돌릴 수 없고, 틀리기 쉬운 것은 **어디서 읽느냐**가
아니라 **무엇이 이기느냐**다.
*/

/** 기본값 위에 덮어쓰기를 얹은 값들. 루트 레이아웃은 이 raw 값을 직접 쓴다. */
export type ResolvedPageMeta = {
  title: string;
  description: string;
  indexable: boolean;
  /** SNS 카드 제목. 안 적었으면 제목 뒤에 브랜드가 붙는다(홈은 예외). */
  cardTitle: string;
  cardDescription: string;
  /** 빈 문자열이면 기본 카드 이미지를 쓴다. */
  cardImage: string;
};

export function mergePageMeta(
  route: PageRoute,
  override: PageMetaOverride | null | undefined,
): ResolvedPageMeta {
  const title = override?.title?.trim() || route.title;
  const description = override?.description?.trim() || route.description;

  /*
    「안 정했다」와 「정했다」를 가른다 — 서버는 정하지 않은 화면에서 이 칸을 아예
    안 보낸다(`noindex,omitempty`). `??`가 아니라 `== null` 검사인 이유는 `false`가
    **정해진 값**이기 때문이다. 둘을 합치면 관리자가 한 번 켰다 끈 화면을 코드
    기본값으로 되돌릴 방법이 사라진다.
  */
  const indexable = override?.noindex == null ? route.indexable : !override.noindex;

  /*
    브랜드를 뒤에 붙이는 것은 **카드에서만** 한다. 탭 제목은 루트 레이아웃의 템플릿
    (`%s · Quant Port`)이 이미 같은 일을 하므로, 여기서 또 붙이면 브랜드가 두 번 선다.
    홈은 제목 자체에 브랜드가 들어 있어(`standaloneTitle`) 양쪽 다 안 붙인다.
  */
  return {
    title,
    description,
    indexable,
    cardTitle:
      override?.ogTitle?.trim() || (route.standaloneTitle ? title : `${title} · ${SITE_NAME}`),
    cardDescription: override?.ogDescription?.trim() || description,
    cardImage: override?.ogImage?.trim() ?? "",
  };
}
