/** Mirrors be/internal/model/page_meta.go. */

/**
 * 한 화면의 제목·설명 **덮어쓰기**.
 *
 * 빈 문자열은 「이 칸은 코드 기본값을 쓴다」는 뜻이다(`lib/page-meta-routes`).
 * `noindex`만 세 값인 이유는 「아직 안 정했다」와 「색인해도 된다고 정했다」가 다른
 * 상태이기 때문이다 — 서버는 안 정한 화면에서 이 칸을 아예 안 보낸다.
 */
export type PageMetaOverride = {
  path: string;
  title: string;
  description: string;
  ogTitle: string;
  ogDescription: string;
  ogImage: string;
  noindex?: boolean;
  updatedAt?: string;
};

/** 관리 화면이 보내는 저장 요청. 전부 비우면 그 행은 지워진다. */
export type PageMetaInput = {
  path: string;
  title: string;
  description: string;
  ogTitle: string;
  ogDescription: string;
  ogImage: string;
  /** `null`이면 「안 정했다」로 되돌린다. */
  noindex: boolean | null;
};
