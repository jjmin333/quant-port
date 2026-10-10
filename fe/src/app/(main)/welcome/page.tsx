import { PageScrollShell } from "@/components/layout/page-content-shell";
import { WelcomePicker } from "@/components/onboarding/welcome-picker";
import { resolvePageMetadata } from "@/lib/page-meta.server";

export function generateMetadata() {
  return resolvePageMetadata("/welcome");
}

/**
 * 가입 직후 화면 (2026-09-26 재방문 개선 B). `?next=`는 고르기를 마친 뒤 돌아갈 곳이다 —
 * 로그인 관문에서 가입으로 넘어온 사람이 보던 화면을 잃지 않게 한다.
 */
export default async function WelcomePage({
  searchParams,
}: {
  searchParams: Promise<{ next?: string | string[] }>;
}) {
  const { next } = await searchParams;
  return (
    <PageScrollShell>
      <WelcomePicker next={typeof next === "string" ? next : undefined} />
    </PageScrollShell>
  );
}
