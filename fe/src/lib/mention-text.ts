/**
 * 태그 글자 규칙 중 **순수한 것만** — 서버 페이지(SEO 설명)에서도 쓰려고 `lib/mentions.ts`에서 떼었다.
 * 서버 `service/mention_parse.go`의 `mentionTokenRe`와 같은 식. 그룹 1 = 앞 경계 글자, 그룹 2 = 토큰.
 */
export const MENTION_TOKEN_RE =
  /(^|[^0-9A-Za-z가-힣_])([@#](?:(?:아파트|지역|상품|금리)\[[^\]\n]{1,60}\]|\[[^\]\n]{1,40}\]|[0-9A-Za-z가-힣&.-]{1,30})(?::[0-9A-Za-z가-힣·%]{1,20})?)/g;

/** 자산 태그의 종류 낱말(2026-10-03) — 서버 `model.AssetKindWords`와 같다. */
export const ASSET_KIND_WORDS = { apt: "아파트", region: "지역", product: "상품", rate: "금리" } as const;
export type AssetKind = keyof typeof ASSET_KIND_WORDS;
/** 미리보기·기사 칩에만 있는 종류 — 시·도와 전국(2026-10-10). 글 태그 낱말은 없다. */
export type PreviewAssetKind = AssetKind | "sido";

/** 목록 요약·설명용 — 시길·대괄호를 걷고 「삼성전자 점수」처럼 읽히게. */
export function plainMentions(text: string): string {
  return text.replace(MENTION_TOKEN_RE, (_all, pre: string, tok: string) => {
    const body = tok.slice(1).replace(/^(?:아파트|지역|상품|금리)(?=\[)/, "").replace(/^\[([^\]]*)\]/, "$1");
    return pre + body.replace(/:/, " ");
  });
}
