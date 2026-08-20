import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { isAuthPage } from "@/lib/auth-routes";

const REMOVED_ACCOUNT_PATHS = [
  "/onboarding/link-broker",
  "/transactions",
  "/portfolio",
  "/risk",
  "/backtest",
] as const;

function isRemovedAccountPath(pathname: string): boolean {
  return REMOVED_ACCOUNT_PATHS.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  );
}

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const hasAccess = request.cookies.has("qp_access");

  if (pathname === "/dashboard" || pathname.startsWith("/dashboard/")) {
    return NextResponse.redirect(new URL("/", request.url));
  }

  if (isRemovedAccountPath(pathname)) {
    return NextResponse.redirect(new URL("/", request.url));
  }

  if (hasAccess && isAuthPage(pathname)) {
    return NextResponse.redirect(new URL("/", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    "/((?!_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp)$).*)",
  ],
};
