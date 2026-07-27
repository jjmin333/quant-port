"use client";

import { Suspense } from "react";
import LoginPage from "./login-page";

export default function Page() {
  return (
    <Suspense fallback={<div className="text-center text-sm text-[var(--text-muted)]">로딩…</div>}>
      <LoginPage />
    </Suspense>
  );
}
