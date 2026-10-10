import { Suspense } from "react";
import { ChangesClient } from "@/components/changes/changes-client";
import { ChangesSeoList } from "@/components/changes/changes-seo-list";
import { resolvePageMetadata } from "@/lib/page-meta.server";

/*
 * 제목·설명은 `lib/page-meta-routes`의 기본값이고, 관리자가 `/admin/seo`에서
 * 덮어쓸 수 있다 — 다른 화면과 같은 규약이다.
 */
export function generateMetadata() {
  return resolvePageMetadata("/changes");
}

/**
 * 「오늘의 변화」 (2026-09-23 사용자 결정 — 새 메뉴). 누구나 본다 — 「내 종목만」은
 * 로그인했을 때만 켜진다. 데이터는 점수 배치가 매일 00:25 끝에서 채운다.
 */
export default function ChangesPage() {
  return (
    <>
      {/* 서버가 그리는 최신 날짜 목록 + 지난 날짜 링크(2026-10-10). */}
      <ChangesSeoList />
      <Suspense
        fallback={
          <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
            변화를 불러오는 중…
          </main>
        }
      >
        <ChangesClient />
      </Suspense>
    </>
  );
}
