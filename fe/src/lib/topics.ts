/**
 * 글 주제 — 커뮤니티·투자 리포트·메모장 공통 (2026-10-09 요청 9, BE `model.Topic*`).
 *
 * 주식이 아닌 리포트는 「분석 리포트」다: 종목·목표가·판정이 없고 성적에 들지 않는다.
 * 대상은 본문의 @아파트·@지역·@상품·@금리 태그가 말한다.
 */
export type ContentTopic = "stock" | "real_estate" | "finance";

export const CONTENT_TOPICS: { id: ContentTopic; label: string }[] = [
  { id: "stock", label: "주식" },
  { id: "real_estate", label: "부동산" },
  { id: "finance", label: "금융상품" },
];

/** 목록 필터 — 빈 값은 전체. */
export const TOPIC_FILTERS: { id: "" | ContentTopic; label: string }[] = [{ id: "", label: "전체 주제" }, ...CONTENT_TOPICS];

export function topicLabel(topic?: string | null): string {
  return CONTENT_TOPICS.find((t) => t.id === topic)?.label ?? "주식";
}

export function isContentTopic(v: string | null | undefined): v is ContentTopic {
  return v === "stock" || v === "real_estate" || v === "finance";
}

/** 글쓰기 도움말 — 주제마다 붙일 태그가 다르다. */
export const TOPIC_TAG_HINT: Record<ContentTopic, string> = {
  stock: "@종목 이름으로 종목을 태그할 수 있습니다.",
  real_estate: "@아파트[단지명] · @지역[시군구]으로 단지와 지역을 태그하면 시세가 함께 보입니다.",
  finance: "@상품[상품명] · @금리[이름]으로 상품과 금리를 태그하면 금리가 함께 보입니다.",
};

/** 주제 배지 색 — 주식은 표시하지 않는다(기존 글 대부분이 주식이라 소음). */
export const TOPIC_BADGE_CLASS: Record<ContentTopic, string> = {
  stock: "",
  real_estate: "bg-amber-500/10 text-amber-700 dark:text-amber-300",
  finance: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
};
