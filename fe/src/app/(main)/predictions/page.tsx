import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { PredictionsClient } from "@/components/predictions/predictions-client";
import { resolvePageMetadata } from "@/lib/page-meta.server";

/*
 * 제목·설명은 `lib/page-meta-routes`의 기본값이고, 관리자가 `/admin/seo`에서
 * 덮어쓸 수 있다 (2026-09-15 사용자 요청 2). `const metadata`가 아니라 함수인
 * 이유는 그 덮어쓰기를 **렌더 시점에** 읽어야 하기 때문이다.
 */
export function generateMetadata() {
  return resolvePageMetadata("/predictions");
}

/**
 * 「목표가 예측」 — 기존 `스탬프`와 `목표가 관리`를 하나로 합친 메뉴.
 *
 * `useSearchParams`(종목 딥링크)를 쓰므로 `Suspense`가 필요하다. 나머지 「내 투자」
 * 화면과 같은 껍데기 안에 두어야 비로그인 방문자가 401 에러 화면 대신 안내를 본다.
 */
export default function PredictionsPage() {
  return (
    <Suspense
      fallback={
        <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
          예측을 불러오는 중…
        </main>
      }
    >
      <AuthRequiredShell loginNext="/predictions" menu="predictions">
        <PredictionsClient />
      </AuthRequiredShell>
    </Suspense>
  );
}
