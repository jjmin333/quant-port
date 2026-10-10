import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { TargetsClient } from "@/components/targets/targets-client";
import { resolvePageMetadata } from "@/lib/page-meta.server";

/*
 * 제목·설명은 `lib/page-meta-routes`의 기본값이고, 관리자가 `/admin/seo`에서
 * 덮어쓸 수 있다 (2026-09-15 사용자 요청 2). `const metadata`가 아니라 함수인
 * 이유는 그 덮어쓰기를 **렌더 시점에** 읽어야 하기 때문이다.
 */
export function generateMetadata() {
  return resolvePageMetadata("/targets");
}

export default function TargetsPage() {
  return (
    <Suspense
      fallback={
        <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
          목표가를 불러오는 중…
        </main>
      }
    >
      <AuthRequiredShell
        loginNext="/targets"
        menu="targets"
      >
        <TargetsClient />
      </AuthRequiredShell>
    </Suspense>
  );
}
