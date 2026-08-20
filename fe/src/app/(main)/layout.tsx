import { Suspense } from "react";
import { AppShell } from "@/components/layout/app-shell";

export default function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <Suspense fallback={null}>
      <AppShell>{children}</AppShell>
    </Suspense>
  );
}
