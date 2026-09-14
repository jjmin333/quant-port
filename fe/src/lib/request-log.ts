/*
요청 로그 한 줄.

# 왜 우리가 따로 찍나

Next dev 서버가 이미 요청마다 한 줄을 찍는다:

	GET /news/domestic 200 in 44ms (next.js: 9ms, proxy.ts: 5ms, application-code: 30ms)

이 줄에는 상태코드와 구간별 소요가 다 들어 있는데 **시각과 사람이 없다.** 형식은
Next가 정하므로 우리가 끼워 넣을 수 없고, 프록시는 라우트가 그려지기 **전에** 끝나므로
상태·소요를 알 수 없다. 그래서 둘로 나눈다 — 우리 줄이 「언제·누가」를, Next 줄이
「무엇이·얼마나」를 말하고, 경로가 둘을 잇는다.

	[2026-09-03 23:10:22.418 KST] user=42 name=홍길동 GET /news/domestic
	GET /news/domestic 200 in 44ms (next.js: 9ms, proxy.ts: 5ms, application-code: 30ms)

# 시각을 오프셋으로 계산하는 이유

`Intl`의 timeZone은 런타임의 ICU 데이터에 기대는데 프록시는 엣지 런타임에서 돈다.
한국은 서머타임이 없어 +09:00이 고정이므로, 고정 오프셋 덧셈이 **무손실이면서** ICU에
의존하지 않는다(BE의 `MYSQL_TIME_ZONE=+09:00`이 같은 근거를 쓴다).
*/

const KST_OFFSET_MS = 9 * 60 * 60 * 1000;

/** `2026-09-03 23:10:22.418` — 밀리초까지. 1초 폴링이 도는 앱이라 초로는 뭉갠다. */
export function formatKstTimestamp(date: Date): string {
  const shifted = new Date(date.getTime() + KST_OFFSET_MS);
  return shifted.toISOString().slice(0, 23).replace("T", " ");
}

/*
로그에서 빼는 경로.

빌드 산출물·HMR·RSC 페이로드는 사람이 누른 것이 아니다. 한 번의 페이지 진입이 이 경로로
수십 건을 만들어 내므로, 남겨 두면 정작 읽고 싶은 줄이 그 사이에 묻힌다.
*/
const IGNORED_PREFIXES = ["/_next/", "/__next", "/favicon.ico"];

export function shouldLogRequest(pathname: string): boolean {
  return !IGNORED_PREFIXES.some((prefix) => pathname.startsWith(prefix));
}

export type RequestLogUser = {
  /** 비로그인이면 빈 문자열 — 그 자리를 비운 채로 찍는다(사용자 요구). */
  id: string;
  name: string;
};

export function formatRequestLog(input: {
  at: Date;
  method: string;
  pathname: string;
  search?: string;
  user: RequestLogUser;
}): string {
  const { at, method, pathname, search = "", user } = input;
  const target = search && search !== "?" ? `${pathname}${search}` : pathname;
  return `[${formatKstTimestamp(at)} KST] user=${user.id} name=${user.name} ${method} ${target}`;
}
