import { resolvePageMetadata } from "@/lib/page-meta.server";

/**
 * `/market`의 메타데이터를 대신 들고 있는 자리 — 화면은 그대로 통과시킨다.
 *
 * 원래 이유는 「`page.tsx`가 클라이언트 컴포넌트라 `metadata`를 못 내보낸다」였다.
 * **2026-09-22에 그 이유가 사라졌다** — SSR 선반입을 붙이며 페이지를 서버
 * 컴포넌트로 되돌렸다. 그래도 여기 두는 것은 같은 값을 두 곳에서 내보내지 않기
 * 위해서다. 옮길 때는 페이지로 옮기고 이 파일을 지우는 한 걸음이어야 한다.
 */
/*
 * 제목·설명은 `lib/page-meta-routes`의 기본값이고, 관리자가 `/admin/seo`에서
 * 덮어쓸 수 있다 (2026-09-15 사용자 요청 2). `const metadata`가 아니라 함수인
 * 이유는 그 덮어쓰기를 **렌더 시점에** 읽어야 하기 때문이다.
 */
export function generateMetadata() {
  return resolvePageMetadata("/market");
}

export default function MarketLayout({ children }: { children: React.ReactNode }) {
  return children;
}
