import type { AuthSession } from "@/lib/auth-types";

export function isAdminSession(session: AuthSession | null | undefined): boolean {
  if (!session?.user) return false;
  return session.user.isAdmin === true || session.user.role === "admin";
}

export function isAdminNavVisible(href: string, session: AuthSession | null | undefined): boolean {
  if (!href.startsWith("/admin")) return true;
  return isAdminSession(session);
}
