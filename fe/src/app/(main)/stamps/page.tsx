import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { StampsClient } from "@/components/stamps/stamps-client";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({
  title: "스탬프",
  description: "봉인해 둔 예측과 채점 결과입니다.",
  noindex: true,
});

/**
 * 스탬프만 `AuthRequiredShell` 밖에 있어서, 로그인하지 않으면 목록이 401을 받고
 * "스탬프를 불러오지 못했습니다"라는 에러 화면이 떴다. 나머지 「내 투자」 메뉴와
 * 같은 껍데기 안으로 넣는다.
 */
export default function StampsPage() {
  return (
    <Suspense
      fallback={
        <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
          스탬프를 불러오는 중…
        </main>
      }
    >
      <AuthRequiredShell
        loginNext="/stamps"
        menu="stamps"
      >
        <StampsClient />
      </AuthRequiredShell>
    </Suspense>
  );
}
