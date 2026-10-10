import type { MouseEvent } from "react";

/**
 * 링크를 「앱 안 동작」으로 가로챌 클릭인가 — 수정 키 없는 왼쪽 클릭만. Ctrl/⌘·Shift·가운데
 * 클릭은 새 탭·새 창을 노린 것이라 링크의 기본 동작에 맡긴다. 키보드 Enter도 여기서 참이다.
 */
export function isPlainLeftClick(e: MouseEvent): boolean {
  return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
}
