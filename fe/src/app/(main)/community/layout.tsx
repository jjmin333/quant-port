import { AdminOnlyGate } from "@/components/layout/admin-only-gate";

/** 2026-10-12부터 관리자 전용 — `lib/hidden-features` ADMIN_ONLY_ROUTE_PREFIXES. */
export default function Layout({ children }: { children: React.ReactNode }) {
  return <AdminOnlyGate>{children}</AdminOnlyGate>;
}
