"use client";

import { useEffect, useMemo } from "react";
import { ErrorScreen } from "@/components/layout/error-screen";
import { reportClientError } from "@/lib/client-error-report";
import { cachedPageIntegrity } from "@/lib/page-integrity";

/**
 * `(main)` 안쪽 화면이 렌더 중에 죽으면 여기가 대신 그려진다.
 *
 * **`(main)/layout.tsx` 아래에 두는 것이 핵심이다.** 그 레이아웃이 `AppShell`
 * (사이드바·헤더)을 감싸고 있어서, 경계가 이 안에 있으면 **본문만 대체되고 셸은
 * 남는다** — 뉴스가 깨져도 사용자는 다른 메뉴로 이동할 수 있다. 루트에 두면
 * 화면 하나의 예외가 앱 전체를 내린다(이 파일이 생기기 전이 그랬다).
 *
 * 페이지에 우리가 넣지 않은 코드가 있는지는 **리포트에만** 싣는다(`page-integrity`) —
 * 화면에서 확장을 지목하던 문구는 우리 버그를 세 번 숨겼다(`error-screen.tsx` 참조).
 *
 * React 오류 경계는 **렌더·라이프사이클만** 잡는다. 이벤트 핸들러와 async
 * 오류는 `error-listeners.tsx`의 전역 리스너가 받는다.
 */
export default function MainError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const integrity = useMemo(() => cachedPageIntegrity(), []);

  useEffect(() => {
    reportClientError({ kind: "render", error, digest: error.digest, integrity });
  }, [error, integrity]);

  return <ErrorScreen onRetry={reset} />;
}
