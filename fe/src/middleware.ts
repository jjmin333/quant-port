import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { isAuthPage } from "@/lib/auth-routes";
import { isHiddenAccountPath } from "@/lib/account-features";

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const hasAccess = request.cookies.has("qp_access");

  if (pathname === "/dashboard" || pathname.startsWith("/dashboard/")) {
    return NextResponse.redirect(new URL("/", request.url));
  }

  if (isHiddenAccountPath(pathname)) {
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
