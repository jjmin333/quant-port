import type { ResearchDirection, ResearchVisibility } from "@/lib/research-types";

/**
 * 작성 중인 리포트 초안을 브라우저에 남긴다.
 *
 * 왜 필요한가: 발행은 되돌릴 수 없는데(삭제 API가 없다) 그 앞에 **안전한 중간 상태가
 * 없었다**. 길게 쓰다가 탭을 닫거나 새로고침하면 본문이 통째로 사라졌고, 그 경험은
 * 「일단 발행해 두고 나중에 고치자」를 부추긴다 — 정확히 하면 안 되는 선택이다.
 * (발행 후 고칠 수 있는 것은 제목·본문·태그·공개범위뿐이고, 본문 수정은 이력으로 공개된다.)
 *
 * 왜 서버가 아니라 localStorage인가: 초안은 아직 발행물이 아니라 **입력 중인 폼**이다.
 * 서버에 두면 초안 목록·동기화·정리 주기 같은 것들이 따라오는데, 이 기능이 해결하려는
 * 문제(창을 닫으면 날아간다)는 브라우저 안에서 전부 해결된다.
 *
 * 종목별로 나눠 저장한다 — 종목을 바꾸면 근거 스냅샷이 통째로 바뀌므로 다른 글이다.
 */

const KEY_PREFIX = "quant-port-research-draft:";

/** 오래된 초안은 되살리지 않는다. 그 사이 시세·점수가 다른 세상이 됐다. */
const DRAFT_TTL_MS = 7 * 24 * 60 * 60 * 1000;

export type ResearchDraft = {
  symbol: string;
  symbolName: string;
  direction: ResearchDirection;
  targetInput: string;
  stopInput: string;
  horizonDays: number;
  conviction: number;
  title: string;
  body: string;
  visibility: ResearchVisibility;
  savedAt: number;
};

function keyFor(symbol: string): string {
  return `${KEY_PREFIX}${symbol.trim().toUpperCase()}`;
}

export function saveResearchDraft(draft: Omit<ResearchDraft, "savedAt">): void {
  if (typeof window === "undefined") return;
  const symbol = draft.symbol.trim();
  if (!symbol) return;
  // 본문이 비었으면 저장할 것이 없다. 빈 초안을 남기면 다음 방문에 「이어쓰기」가
  // 뜨는데 눌러도 아무것도 안 채워진다.
  if (!draft.body.trim() && !draft.title.trim()) {
    clearResearchDraft(symbol);
    return;
  }
  try {
    window.localStorage.setItem(
      keyFor(symbol),
      JSON.stringify({ ...draft, symbol, savedAt: Date.now() } satisfies ResearchDraft),
    );
  } catch {
    // 용량 초과·프라이빗 모드. 초안 저장 실패로 글쓰기를 막지는 않는다.
  }
}

export function loadResearchDraft(symbol: string): ResearchDraft | null {
  if (typeof window === "undefined") return null;
  if (!symbol.trim()) return null;
  try {
    const raw = window.localStorage.getItem(keyFor(symbol));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as ResearchDraft;
    if (!parsed || typeof parsed.body !== "string") return null;
    if (!Number.isFinite(parsed.savedAt) || Date.now() - parsed.savedAt > DRAFT_TTL_MS) {
      clearResearchDraft(symbol);
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

export function clearResearchDraft(symbol: string): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(keyFor(symbol));
  } catch {
    /* 위와 같다 */
  }
}
