import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { PredictionsClient } from "@/components/predictions/predictions-client";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({
  title: "목표가 예측",
  description: "목표가·손절가와 기한을 적어 둔 내 예측과 그 성적입니다.",
  noindex: true,
});

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
