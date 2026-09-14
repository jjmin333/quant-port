import type { Metadata } from "next";
import { NOINDEX } from "@/lib/seo";

/**
 * 로그인·회원가입은 색인하지 않는다.
 *
 * 두 화면 모두 `"use client"`라 `page.tsx`에서 `metadata`를 내보낼 수 없어서
 * 레이아웃이 대신 든다. 제목은 여기 두지 않는다 — 한 레이아웃이 두 화면을
 * 덮으므로 「로그인」이 회원가입 탭에도 붙어 버린다. 색인 대상이 아니라 제목이
 * 검색에 쓰일 일은 없다.
 */
export const metadata: Metadata = { robots: NOINDEX };

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-full items-center justify-center bg-[var(--background)] p-6">
      <div className="w-full max-w-md">{children}</div>
    </div>
  );
}
