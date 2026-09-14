/**
 * 브라우저에서 난 오류를 서버 로그로 흘려보낸다.
 *
 * **왜 필요한가.** 앱에 오류 경계가 하나도 없던 동안, 렌더 중 예외 하나가 화면
 * 전체를 내렸고 **우리는 그 사실을 몰랐다.** 2026-09-01에 사용자 브라우저에서
 * 같은 오류가 69회 났는데 서버에는 아무 흔적도 없었다 — dev 서버 로그에 우연히
 * 남아 있어서 알았을 뿐이다. 사용자가 「버그다」라고 말해 주기 전에 우리가 먼저
 * 아는 것이 이 파일의 목적이다.
 *
 * **화면을 막지 않는다.** `sendBeacon`으로 던지고 응답을 기다리지 않는다.
 * 리포트가 실패해도 사용자에게는 아무 일도 일어나지 않아야 한다.
 */

import { inspectPageIntegrity, type PageIntegrity } from "@/lib/page-integrity";

const ENDPOINT = "/api/v1/client-errors";

/**
 * 한 번 뜬 페이지에서 보낼 수 있는 최대 건수.
 *
 * 오류는 **폭주하는 성질**이 있다 — 실측된 사례가 1초에 69건이었다. 렌더 오류는
 * React가 재시도하면서 같은 예외를 반복해서 던지기 때문이다. 상한이 없으면
 * 리포터 자신이 사용자의 회선과 서버 로그를 태운다.
 */
const MAX_PER_PAGE = 5;

/** 같은 오류를 다시 보내기까지의 최소 간격. */
const DEDUPE_WINDOW_MS = 30_000;

let sentCount = 0;
const lastSentAt = new Map<string, number>();

export type ClientErrorKind = "render" | "window" | "unhandledrejection";

type ReportInput = {
  kind: ClientErrorKind;
  error: unknown;
  /** Next가 서버 오류에 붙이는 식별자(있을 때만). */
  digest?: string;
  /**
   * 페이지에 우리가 넣지 않은 코드가 있는지. 오류 화면이 이미 조사했다면
   * 그 결과를 넘겨 준다 — 같은 조사를 두 번 하지 않는다.
   */
  integrity?: PageIntegrity;
};

function messageOf(error: unknown): string {
  if (error instanceof Error) return error.message || error.name;
  if (typeof error === "string") return error;
  try {
    return JSON.stringify(error).slice(0, 500);
  } catch {
    return String(error);
  }
}

function stackOf(error: unknown): string {
  return error instanceof Error && typeof error.stack === "string" ? error.stack : "";
}

/**
 * 오류 하나를 보고한다. 던지지 않는다 — 이 함수가 실패해서 화면이 더 나빠지는
 * 일은 없어야 한다.
 */
export function reportClientError({ kind, error, digest, integrity }: ReportInput): void {
  if (typeof window === "undefined") return;

  try {
    const message = messageOf(error);
    if (!message) return;

    if (sentCount >= MAX_PER_PAGE) return;

    // 경로는 pathname만 싣는다 — 쿼리스트링에는 검색어처럼 개인적인 값이
    // 들어갈 수 있고, 오류를 찾는 데는 어느 화면이었는지면 충분하다.
    const pathname = window.location.pathname;

    const key = `${kind}|${message}|${pathname}`;
    const now = Date.now();
    const prev = lastSentAt.get(key);
    if (prev != null && now - prev < DEDUPE_WINDOW_MS) return;
    lastSentAt.set(key, now);
    sentCount += 1;

    // 페이지가 손대어졌는지는 **오류가 났을 때만** 조사한다(빈 iframe 하나를
    // 만들었다 지운다). 평소 경로에서는 이 비용이 들지 않는다.
    const checked = integrity ?? inspectPageIntegrity();

    const payload = JSON.stringify({
      kind,
      message,
      // **원본 스택을 그대로 싣는다.** 소스맵을 거친 프레임은 확장이 번들을
      // 고쳐 넣은 코드를 「우리 소스의 엉뚱한 줄」로 옮겨 놓아, 진짜 위치를
      // 가린다. 컴파일된 파일·줄이 있어야 실제로 실행된 코드를 짚을 수 있다.
      stack: stackOf(error).slice(0, 3000),
      pathname,
      digest: digest ?? "",
      foreignGlobals: checked.foreignGlobals,
      extensionResources: checked.extensionResources,
    });

    // sendBeacon은 화면이 사라지는 중에도 전송을 보장하고, 응답을 기다리지
    // 않는다. 없는 브라우저에서는 keepalive fetch로 같은 성질을 흉내 낸다.
    const blob = new Blob([payload], { type: "application/json" });
    if (typeof navigator.sendBeacon === "function" && navigator.sendBeacon(ENDPOINT, blob)) {
      return;
    }
    void fetch(ENDPOINT, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: payload,
      keepalive: true,
    }).catch(() => {});
  } catch {
    // 리포터가 원인이 되어 화면을 더 망가뜨리지 않는다.
  }
}
