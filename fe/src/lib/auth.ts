import { apiRequest, ensureOk, parseJson } from "@/lib/api-client";
import { clearFetchCache } from "@/lib/fetch-cache";
import {
  markRefreshImpossible,
  markRefreshUsable,
  refreshSession,
  sessionFetch,
} from "@/lib/session-fetch";
import type { AuthSession } from "@/lib/auth-types";

export { apiCredentials } from "@/lib/session-fetch";

function authFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  return apiRequest<T>(
    path,
    {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...(init.headers ?? {}),
      },
    },
    "Request failed",
  );
}

/** 가입 — 아이디·비밀번호·닉네임 (2026-09-27 요청 3, 이메일 없음). */
export async function signup(
  loginId: string,
  password: string,
  displayName: string,
): Promise<AuthSession> {
  const session = await authFetch<AuthSession>("/api/v1/auth/signup", {
    method: "POST",
    body: JSON.stringify({ loginId, password, displayName }),
  });
  // 이제 갱신 쿠키가 있다 — 비로그인 동안 굳어 있던 「갱신 못 함」을 푼다.
  markRefreshUsable();
  return session;
}

/** 로그인 — 기존 회원은 가입 때 쓴 이메일이 그대로 아이디다. */
export async function login(loginId: string, password: string): Promise<AuthSession> {
  const session = await authFetch<AuthSession>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ loginId, password }),
  });
  // 로그인 전에 받아 둔 응답 중에는 비로그인 상태의 것이 섞여 있다(관심·보유
  // 표시가 빠진 시세 등). "누구인지"가 바뀌었으니 통째로 버린다.
  clearFetchCache();
  // 이제 갱신 쿠키가 있다 — 비로그인 동안 굳어 있던 「갱신 못 함」을 푼다.
  markRefreshUsable();
  return session;
}

export async function logout(): Promise<void> {
  await authFetch<{ ok: boolean }>("/api/v1/auth/logout", { method: "POST" });
  // 로그아웃 뒤에 이전 사용자의 응답이 남아 있으면 안 된다.
  clearFetchCache();
  // 갱신 쿠키가 사라졌다 — 이 뒤의 401에 갱신을 붙이지 않는다.
  markRefreshImpossible();
}

export async function fetchMe(): Promise<AuthSession> {
  return authFetch<AuthSession>("/api/v1/auth/me");
}

/**
 * 앱이 뜰 때 한 번 묻는 「지금 누구인가」 — **비로그인이면 null이고, 오류가 아니다.**
 *
 * 예전에는 여기서 `/auth/me`를 불렀다. 비로그인 방문자에게 그건 401이고, 401은
 * `sessionFetch`가 갱신을 붙여 `/auth/refresh`도 401이 됐다 — 처음 온 사람과 검색엔진
 * 렌더러가 **모든 화면에서 콘솔 오류 두 줄**을 봤다(2026-09-26 외부 방문자 점검).
 * `/auth/session`은 언제나 200이고 갱신 쿠키가 있는지(`refreshable`)를 함께 준다.
 *
 * - 세션이 있으면 그대로.
 * - 없고 갱신 쿠키도 없으면 「갱신 못 함」을 굳힌다 — 이 뒤의 401에 갱신을 붙이지 않는다.
 * - 없는데 갱신 쿠키는 있으면(액세스 토큰만 만료) 갱신하고 다시 묻는다.
 *
 * BE가 아직 이 경로를 모르면(404 — FE가 먼저 배포된 경우) 예전 `/auth/me`로 돌아간다.
 */
export async function fetchInitialSession(): Promise<AuthSession | null> {
  const res = await sessionFetch("/api/v1/auth/session", { cache: "no-store" });
  if (res.status === 404) return fetchMe();
  await ensureOk(res, "Request failed");
  const body = await parseJson<{ data: AuthSession | null; refreshable?: boolean }>(res);
  if (body.data) return body.data;
  if (!body.refreshable) {
    markRefreshImpossible();
    return null;
  }
  if (await refreshSession()) return fetchMe();
  return null;
}

/*
계정 관리 (§A-6).

이 앱에는 **비밀번호를 바꿀 방법도, 탈퇴할 방법도 없었다.** 비밀번호를 잊으면 계정에
다시 들어올 길이 없고, 그만두려는 사람은 데이터를 남긴 채 떠나야 했다.

셋 다 현재 비밀번호를 다시 받는다 — 로그인 세션은 오래 유지되고 남의 자리에 열린 채
남아 있을 수 있으므로, 계정 자체를 바꾸는 행동은 「지금 이 사람이 주인인가」를 한 번 더
묻는 것이 맞다(검증은 서버가 한다).
*/

/** 비밀번호 변경. 성공하면 **모든 기기의 세션이 끊긴다** — 다시 로그인해야 한다. */
export async function changePassword(
  currentPassword: string,
  newPassword: string,
): Promise<void> {
  await authFetch<{ ok: boolean }>("/api/v1/auth/password", {
    method: "POST",
    body: JSON.stringify({ currentPassword, newPassword }),
  });
  clearFetchCache();
}

export async function changeDisplayName(displayName: string): Promise<AuthSession> {
  const session = await authFetch<AuthSession>("/api/v1/auth/display-name", {
    method: "POST",
    body: JSON.stringify({ displayName }),
  });
  clearFetchCache();
  return session;
}

/** 회원 탈퇴. 소유 데이터는 지우고, 발행한 리포트·댓글은 남되 작성자 이름이 사라진다. */
export async function deleteAccount(password: string): Promise<void> {
  await authFetch<{ ok: boolean }>("/api/v1/auth/delete", {
    method: "POST",
    body: JSON.stringify({ password }),
  });
  clearFetchCache();
}
