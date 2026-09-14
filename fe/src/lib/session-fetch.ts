const DEFAULT_API_URL = "http://localhost:8080";

function getApiBaseUrl(): string {
  if (typeof window !== "undefined") {
    return "";
  }
  return process.env.NEXT_PUBLIC_API_URL ?? DEFAULT_API_URL;
}

export const apiCredentials: RequestInit = {
  credentials: "include",
};

/** Access token default 15m — refresh proactively before expiry. */
const PROACTIVE_REFRESH_MS = 10 * 60 * 1000;

let inflightRefresh: Promise<boolean> | null = null;

/*
**한 번 실패한 갱신은 다시 시도하지 않는다** (2026-09-05 사용자 지적 6).

# 무엇이 잘못돼 있었나

`sessionFetch`는 401을 받으면 갱신을 한 번 시도하고 다시 부른다. 로그인한 사용자에게는
맞는 규칙이다 — 액세스 토큰이 15분마다 만료되므로 그 한 번이 사용자를 안 쫓아낸다.

**비로그인 사용자에게는 401이 정상이다.** 그래서 화면이 관심·보유·스탬프처럼 로그인이
필요한 값을 물을 때마다 401이 오고, 그때마다 `POST /auth/refresh`가 따라 나갔다.
실측(2026-09-05, 비로그인 홈): 탭이 다시 보일 때마다 `auth/me → 401`, `auth/refresh →
401`이 한 벌씩. 쿠키가 없으니 **답이 바뀔 수가 없는** 요청이다.

쿠키가 HttpOnly라 브라우저가 「갱신 쿠키가 있는가」를 물어볼 방법은 없다. 대신
**해 보고 나온 답을 기억한다**: 갱신이 실패했으면 그 뒤로는 묻지 않는다.

# 언제 다시 열리나

로그인·회원가입이 성공했을 때다(`markRefreshUsable`). 그때 진짜 쿠키가 생기므로
「못 한다」는 기억이 더 이상 맞지 않는다. 로그아웃은 반대로 다시 닫는다.

「탭을 열어 두는 동안 쿠키가 저절로 생기는」 경우는 없다 — 다른 탭에서 로그인하면
이 탭의 기억이 남지만, 그 탭은 로그인 직후 화면을 새로 그리고 이 탭도 다음 새로고침에
풀린다. 그 한 경우를 위해 매 401마다 되묻는 값을 치를 이유가 없다.
*/
let refreshKnownImpossible = false;

/** 로그인·회원가입 성공 — 갱신 쿠키가 생겼으니 「못 한다」는 기억을 지운다. */
export function markRefreshUsable(): void {
  refreshKnownImpossible = false;
}

/** 로그아웃 — 갱신 쿠키가 사라졌으니 더 묻지 않는다. */
export function markRefreshImpossible(): void {
  refreshKnownImpossible = true;
}

export function proactiveRefreshIntervalMs(): number {
  return PROACTIVE_REFRESH_MS;
}

/** Silently renew access token using the refresh cookie (7-day session window). */
export async function refreshSession(): Promise<boolean> {
  if (refreshKnownImpossible) return false;
  if (inflightRefresh) {
    return inflightRefresh;
  }

  inflightRefresh = (async () => {
    try {
      const res = await fetch(`${getApiBaseUrl()}/api/v1/auth/refresh`, {
        method: "POST",
        credentials: "include",
      });
      // 401·403은 「쿠키가 없거나 만료됐다」이고, 그건 다시 물어도 같은 답이다.
      // 5xx·네트워크 실패는 **서버 사정**이라 굳히지 않는다 — 그때까지 로그인해
      // 있던 사람이 서버가 잠깐 흔들렸다고 로그아웃당하면 안 된다.
      if (res.status === 401 || res.status === 403) refreshKnownImpossible = true;
      return res.ok;
    } catch {
      return false;
    } finally {
      inflightRefresh = null;
    }
  })();

  return inflightRefresh;
}

export async function sessionFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const url = `${getApiBaseUrl()}${path}`;
  const merged: RequestInit = {
    ...apiCredentials,
    ...init,
    headers: {
      ...(init.headers ?? {}),
    },
  };

  let res = await fetch(url, merged);
  if (res.status === 401 && !path.includes("/auth/refresh")) {
    const renewed = await refreshSession();
    if (renewed) {
      res = await fetch(url, merged);
    }
  }
  return res;
}
