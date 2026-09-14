"use client";

import { useEffect, useMemo } from "react";
import { ErrorScreen } from "@/components/layout/error-screen";
import { reportClientError } from "@/lib/client-error-report";
import { cachedPageIntegrity } from "@/lib/page-integrity";

/**
 * 루트 레이아웃 자체가 죽었을 때의 마지막 그물.
 *
 * 여기까지 오면 `(main)/error.tsx`는 이미 함께 무너졌다는 뜻이라 **셸을 살릴 수
 * 없다.** Next의 규칙대로 `<html>`·`<body>`를 직접 그린다.
 *
 * Next에도 기본 global-error가 있지만 영문 「Application error: a client-side
 * exception has occurred」 한 줄이라, 사용자에게는 서비스가 통째로 죽은 것처럼
 * 보이고 할 수 있는 일도 알려 주지 않는다. 같은 상황에서 우리 말로,
 * 복구 버튼과 함께 보여 주려고 덮어쓴다.
 *
 * **`globals.css`를 여기서 다시 불러온다** — 루트 레이아웃이 무너진 뒤라
 * 그쪽에서 넣은 스타일을 기대할 수 없다.
 */
import "./globals.css";

export default function GlobalError({
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

  return (
    <html lang="ko" className="h-full antialiased">
      <body className="min-h-full bg-[var(--background)]">
        <ErrorScreen
          onRetry={reset}
          className="flex min-h-[100dvh] flex-col items-center justify-center gap-5 px-6 py-16 text-center"
        />
      </body>
    </html>
  );
}
