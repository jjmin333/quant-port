import type { ContentTopic } from "@/lib/topics";
export type JournalNote = {
  id: number;
  title: string;
  /** 주식·부동산·금융상품(2026-10-09 요청 9). 옛 응답에는 없다 — 없으면 주식. */
  topic?: ContentTopic;
  body: string;
  symbol?: string;
  symbolName?: string;
  /**
   * 종목의 **작성 당시** 가격. 서버가 저장 시점에 한 번 재고 이후 건드리지 않는다.
   * 종목을 연결하지 않았거나, 이 컬럼이 생기기 전에 쓴 메모면 없다.
   */
  priceAtWrite?: number;
  priceCurrency?: "KRW" | "USD";
  /** 그 가격이 속한 거래일(YYYY-MM-DD). */
  priceDate?: string;
  tags?: string[];
  pinned: boolean;
  createdAt: string;
  updatedAt: string;
};

export type JournalNoteInput = {
  title: string;
  topic?: ContentTopic;
  body: string;
  symbol?: string;
  symbolName?: string;
  tags?: string[];
  pinned?: boolean;
};

/**
 * 폼에 미리 놓는 태그. 자유 입력도 되지만, 흔한 네 개를 **모두가 같은 철자로** 쓰게
 * 만드는 것이 태그 필터를 쓸모 있게 하는 유일한 조건이다.
 */
export const JOURNAL_TAG_PRESETS = ["매수근거", "리스크", "관찰", "복기"] as const;

/**
 * 주제별 프리셋(2026-10-09 요청 9). 주식은 기존 넷 그대로. 부동산·금융상품 메모에는 여기에
 * 단지·지역·상품 이름 태그(자산 검색으로 붙인다)가 더해진다.
 */
export const JOURNAL_TOPIC_PRESETS: Record<ContentTopic, readonly string[]> = {
  stock: JOURNAL_TAG_PRESETS,
  real_estate: ["매수검토", "임장", "청약", "시세관찰"],
  finance: ["예금", "적금", "대출", "금리관찰"],
};

/** 프리셋 태그의 색. 나머지는 중립색 하나로 간다 — 자유 태그마다 색을 주면 목록이 얼룩진다. */
export const JOURNAL_TAG_TONE: Record<string, string> = {
  매수근거: "bg-[var(--brand-muted)] text-[var(--brand)]",
  리스크: "bg-amber-500/15 text-amber-500",
  관찰: "bg-[var(--surface-elevated)] text-[var(--text-secondary)]",
  복기: "bg-emerald-500/15 text-emerald-500",
};

/** 작성 시점 대비 등락(%). 가격이 없으면 null — 0%로 그리면 "변화 없음"이라는 거짓이 된다. */
export function noteReturnPct(note: JournalNote, currentPrice?: number): number | null {
  const base = note.priceAtWrite;
  if (!base || base <= 0 || !currentPrice || currentPrice <= 0) return null;
  return ((currentPrice - base) / base) * 100;
}
