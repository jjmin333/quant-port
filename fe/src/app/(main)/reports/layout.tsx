import { AdminOnlyGate } from "@/components/layout/admin-only-gate";

/** 관리자 전용 문 — `lib/hidden-features` ADMIN_ONLY_ROUTE_PREFIXES에 있을 때만 닫힌다(10-13 공개). */
export default function Layout({ children }: { children: React.ReactNode }) {
  return <AdminOnlyGate>{children}</AdminOnlyGate>;
}
