import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { isAuthPage } from "@/lib/auth-routes";
import {
  accessTokenDisplayName,
  readAccessTokenClaims,
} from "@/lib/access-token-claims";
import { formatRequestLog, shouldLogRequest } from "@/lib/request-log";
import { isHiddenRoute } from "@/lib/hidden-features";

/*
`middleware.ts`에서 이름만 바꿔 왔다.

Next 16이 `middleware` 파일 규약을 폐기하고 `proxy`로 이름을 바꿨다
(node_modules/next/dist/docs/01-app/03-api-reference/03-file-conventions/proxy.md).
동작은 같다 — 파일명과 내보내는 함수 이름만 바뀐다. 지금까지도 Next는 이 자리를
`proxy.ts`로 부르고 있었다: 요청 로그의 「proxy.ts: 5ms」가 바로 이 함수의 시간이다.
*/

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

/*
요청 한 줄을 남긴다 — **언제·누가**.

Next가 찍는 줄은 「무엇이·얼마나」만 말한다(상태코드·구간별 소요). 형식을 우리가 바꿀
수 없고, 이 함수는 라우트가 그려지기 전에 끝나므로 상태·소요를 알 수도 없다. 그래서
경로를 열쇠 삼아 두 줄이 짝을 이룬다 (`lib/request-log` 참고).

사용자는 쿠키의 액세스 토큰에서 읽는다. **검증하지 않는다** — 여기서 하는 일은 로그를
사람이 읽게 하는 것뿐이고, 인가는 전부 BE가 서명을 검증한 뒤에 한다.
*/
function logRequest(request: NextRequest) {
  const { pathname, search } = request.nextUrl;
  if (!shouldLogRequest(pathname)) return;

  const claims = readAccessTokenClaims(request.cookies.get("qp_access")?.value);
  console.log(
    formatRequestLog({
      at: new Date(),
      method: request.method,
      pathname,
      search,
      user: { id: claims.userId, name: accessTokenDisplayName(claims) },
    }),
  );
}

export function proxy(request: NextRequest) {
  logRequest(request);

  const { pathname } = request.nextUrl;
  const hasAccess = request.cookies.has("qp_access");

  if (pathname === "/dashboard" || pathname.startsWith("/dashboard/")) {
    return NextResponse.redirect(new URL("/", request.url));
  }

  // 노출을 끈 기능(2026-10-10) — 되살릴 수 있으므로 307(임시)로 홈에 보낸다.
  if (isRemovedAccountPath(pathname) || isHiddenRoute(pathname)) {
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
