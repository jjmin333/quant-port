/**
 * 커뮤니티 글·댓글 본문은 **평문**이다. HTML을 받지 않고, 화면이 줄바꿈과 링크만 살린다.
 * innerHTML을 쓰지 않으려고 본문을 조각(글자·링크)으로 나눠 React가 그리게 한다.
 */

export type TextSegment = { kind: "text"; text: string } | { kind: "link"; text: string; href: string };

// http(s)만 링크로 — javascript: 같은 스킴은 글자로 남는다.
const URL_RE = /https?:\/\/[^\s<>"'`]+/g;
// 문장 끝 구두점은 링크에서 뗀다 — 「…참고하세요 https://a.com/x.」의 마침표.
const TRAILING = /[.,!?;:)\]」』]+$/;

export function splitLinks(text: string): TextSegment[] {
  const out: TextSegment[] = [];
  let last = 0;
  for (const m of text.matchAll(URL_RE)) {
    const start = m.index ?? 0;
    let raw = m[0];
    const trail = raw.match(TRAILING)?.[0] ?? "";
    if (trail) raw = raw.slice(0, raw.length - trail.length);
    if (start > last) out.push({ kind: "text", text: text.slice(last, start) });
    out.push({ kind: "link", text: raw, href: raw });
    last = start + raw.length;
  }
  if (last < text.length) out.push({ kind: "text", text: text.slice(last) });
  return out;
}

/** 브라우저에서 올리기 전에 줄일 크기 — 긴 변 1600px(서버의 CompressMaxEdge와 같다). */
export const CLIENT_IMAGE_MAX_EDGE = 1600;

export function fitImageSize(w: number, h: number, maxEdge = CLIENT_IMAGE_MAX_EDGE): { w: number; h: number } {
  const long = Math.max(w, h);
  if (long <= maxEdge || long === 0) return { w, h };
  const scale = maxEdge / long;
  return { w: Math.max(1, Math.round(w * scale)), h: Math.max(1, Math.round(h * scale)) };
}
