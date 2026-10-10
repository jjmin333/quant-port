import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { CommunityComposeClient } from "@/components/community/community-compose-client";
import { resolvePageMetadata } from "@/lib/page-meta.server";

export function generateMetadata() {
  return resolvePageMetadata("/community/new");
}

type Props = { searchParams: Promise<Record<string, string | string[] | undefined>> };

// 로그인 뒤 글쓰기로 돌아올 때 들고 와야 하는 것 — 공유한 기사·붙인 종목.
const CARRIED = ["article", "title", "source", "symbols"] as const;

/**
 * 커뮤니티 글쓰기 — 로그인해야 쓴다(`?article=` 공유 진입도 여기로 온다).
 *
 * 로그인 뒤 돌아올 주소에 **쿼리를 실어 보낸다**(2026-09-28). 예전엔 `/community/new`만 넘겨서,
 * 비로그인으로 「커뮤니티에 공유」·「이 종목으로 글쓰기」를 누른 사람이 로그인하고 나면 빈 글쓰기를 봤다.
 */
export default async function CommunityComposePage({ searchParams }: Props) {
  const sp = await searchParams;
  const q = new URLSearchParams();
  for (const key of CARRIED) {
    const v = sp[key];
    if (typeof v === "string" && v) q.set(key, v.slice(0, 300));
  }
  const next = q.size > 0 ? `/community/new?${q.toString()}` : "/community/new";
  return (
    <Suspense>
      <AuthRequiredShell loginNext={next}>
        <CommunityComposeClient />
      </AuthRequiredShell>
    </Suspense>
  );
}
