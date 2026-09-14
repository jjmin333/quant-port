"use client";

import { Suspense } from "react";
import { useParams } from "next/navigation";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { EarningsClient } from "@/components/earnings/earnings-client";
import { JournalClient } from "@/components/journal/journal-client";
import {
  JournalLabClient,
  type JournalLabVariant,
} from "@/components/journal/lab/journal-lab-client";
import { ResearchClient } from "@/components/research/research-client";
import { StampsClient } from "@/components/stamps/stamps-client";
import { TargetsClient } from "@/components/targets/targets-client";
import { WatchlistClient } from "@/components/watchlist/watchlist-client";
import {
  WatchlistLabClient,
  type LabVariant,
} from "@/components/watchlist/lab/watchlist-lab-client";
import { HeaderSearchProvider } from "@/components/providers/header-search-provider";
import { QuickLookProvider } from "@/components/providers/quick-look-provider";
import { ShellModeProvider } from "@/components/providers/shell-mode-provider";
import { findSample } from "@/components/dev-samples/sample-registry";
import { LayoutLab } from "@/lib/layout-lab";

/**
 * 시안 한 장 — **실서비스 화면 그대로**를 실험대 값으로 감싸 그린다
 * (`components/dev-samples/sample-registry` 머리말).
 *
 * 갤러리가 이 주소를 폭 375·768·1280짜리 `<iframe>`으로 세 번 연다. 컨테이너
 * 쿼리로 흉내 내지 않고 진짜 프레임을 쓰는 이유: 화면의 배치는 Tailwind의 `sm:`·
 * `xl:` 분기가 정하는데 그 분기는 **뷰포트**를 본다. 한 창 안에서 폭만 375px로 줄인
 * 상자는 여전히 데스크톱 분기를 타므로, 모바일을 보려던 프레임이 PC 배치를 보여 준다.
 *
 * # 앱 껍데기(헤더·좌측 내비)는 없다
 *
 * 그 둘이 차지하는 자리는 어느 시안에서나 같으므로 비교에 보태는 것이 없고, 375px
 * 프레임에서 세로 예산만 먹는다. 대신 `PageScrollShell`이 `flex-1`을 요구하므로
 * 높이를 가진 flex 부모는 여기서 준다.
 *
 * # 예시 데이터
 *
 * 「내 투자」 넷은 로그인해야 데이터가 있다. 갤러리가 주소에 `?sample=1`을 붙여 열고,
 * `AuthRequiredShell`이 그 값을 보고 공개 픽스처로 실제 화면을 채운다 — 실서비스에서
 * 「예시로 둘러보기」를 눌렀을 때와 같은 경로다.
 */
export default function SampleViewPage() {
  const params = useParams<{ page: string; variant: string }>();
  const { page, variant } = findSample(params.page, params.variant);

  if (!page || !variant) {
    return (
      <div className="p-6 text-cell text-[var(--text-muted)]">
        그런 시안이 없습니다 — {params.page} / {params.variant}
      </div>
    );
  }

  /*
    앱 껍데기(헤더·좌측 내비)는 빼지만 **그 껍데기가 세우는 제공자 셋은 필요하다** —
    화면들이 헤더 검색·퀵룩·좁은화면 판정을 그 안에서 읽는다. 빼면 `useHeaderSearch
    must be used within HeaderSearchProvider`로 화면이 통째로 죽는다(실측).
    `initialMobile`을 false로 두는 이유: 프레임이 실제 폭을 갖고 있으므로 마운트
    직후 진짜 미디어 쿼리가 덮는다.
  */
  return (
    <LayoutLab value={variant.lab}>
      <ShellModeProvider initialMobile={false}>
        <QuickLookProvider>
          <HeaderSearchProvider>
            <div className="flex h-dvh flex-col bg-[var(--background)]">
              <Suspense
                fallback={
                  <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
                    불러오는 중…
                  </main>
                }
              >
                <Screen id={page.id} variant={variant.id} />
              </Suspense>
            </div>
          </HeaderSearchProvider>
        </QuickLookProvider>
      </ShellModeProvider>
    </LayoutLab>
  );
}

function Screen({ id, variant }: { id: string; variant: string }) {
  switch (id) {
    /*
      관심종목 개편 시안 — 「현재」만 지금 화면이고 나머지 넷은 새 몸통이다
      (`components/watchlist/lab`). 로그인 밖에서도 보여야 하므로 실서비스와 같은
      `AuthRequiredShell` + `?sample=1` 경로를 그대로 탄다.
    */
    case "watchlist-v2":
      return (
        <AuthRequiredShell loginNext="/watchlist" menu="watchlist">
          {variant === "current" ? (
            <WatchlistClient />
          ) : (
            <WatchlistLabClient variant={variant as LabVariant} />
          )}
        </AuthRequiredShell>
      );
    /*
      메모장 개편 시안 — 「현재」만 지금 화면이고 나머지 셋은 새 몸통이다
      (`components/journal/lab`). 새 몸통은 시안용 픽스처를 스스로 들고 있어
      로그인·예시 모드와 무관하게 그려지므로 `AuthRequiredShell`을 거치지 않는다.
    */
    case "journal-v2":
      return variant === "current" ? (
        <AuthRequiredShell loginNext="/journal" menu="journal">
          <JournalClient />
        </AuthRequiredShell>
      ) : (
        <JournalLabClient variant={variant as JournalLabVariant} />
      );
    case "earnings":
      return <EarningsClient market="us" />;
    case "reports":
      return <ResearchClient />;
    case "watchlist":
      return (
        <AuthRequiredShell loginNext="/watchlist" menu="watchlist">
          <WatchlistClient />
        </AuthRequiredShell>
      );
    case "stamps":
      return (
        <AuthRequiredShell loginNext="/stamps" menu="stamps">
          <StampsClient />
        </AuthRequiredShell>
      );
    case "journal":
      return (
        <AuthRequiredShell loginNext="/journal" menu="journal">
          <JournalClient />
        </AuthRequiredShell>
      );
    case "targets":
      return (
        <AuthRequiredShell loginNext="/targets" menu="targets">
          <TargetsClient />
        </AuthRequiredShell>
      );
    default:
      return null;
  }
}
