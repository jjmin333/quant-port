import { Suspense } from "react";
import { headers } from "next/headers";
import { AppShell } from "@/components/layout/app-shell";
import { detectMobileFromUA } from "@/lib/shell-mode";

/**
 * **첫 HTML을 기기에 맞춰 그린다** (2026-09-03 사용자 결정).
 *
 * 예전에는 서버가 화면 폭을 몰라 언제나 데스크톱 배치를 그렸고, 폰에서는 그 배치가
 * 한 번 그려진 뒤 자바스크립트가 붙고 나서야 모바일로 바뀌었다 — 메뉴를 처음 열 때마다
 * 1~2초 동안 화면이 뭉개져 보였고 사용자에게는 고장으로 읽혔다.
 *
 * 요청 헤더의 User-Agent는 폭을 정확히 말해 주지 않지만 「폰인가」는 거의 언제나
 * 맞는다. 그 값으로 첫 프레임을 찍고, 마운트 뒤에는 실제 미디어 쿼리가 덮는다
 * (`detectMobileFromUA` 헤더 참조).
 *
 * **`headers()`를 읽으므로 이 레이아웃은 요청마다 그려진다.** 이 앱의 화면은 이미
 * 전부 클라이언트에서 데이터를 받아 오므로 정적으로 미리 그려 둘 것이 애초에 없었다.
 */
export default async function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const ua = (await headers()).get("user-agent") ?? "";

  return (
    <Suspense fallback={null}>
      <AppShell initialMobile={detectMobileFromUA(ua)}>{children}</AppShell>
    </Suspense>
  );
}
