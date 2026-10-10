/*
**끝나지 않는 요청은 실패가 아니라 정지다** (2026-09-20 SSR/CSR 점검 요청 3).

# 무엇이 잘못돼 있었나

앱 전체에서 타임아웃을 건 fetch는 **한 곳뿐**이었다 — `fetchMarketIndicesClient`의
8초 AbortController. 나머지(`apiRequest`·`serverApiRequest`와 `market-api`의 raw
fetch 열댓 곳)는 상한이 없었다.

상한이 없으면 실패 상태가 **영영 오지 않는다.** 화면은 `catch`를 기다리는데 promise가
pending으로 남아 있으니, 「로딩 중」이 참인 채로 굳는다. 홈의 「갱신 중…」이 정확히
그 모양이었다 — 사용자가 보는 것은 에러가 아니라 **영원한 스켈레톤**이고, 그건
새로고침 말고는 빠져나갈 길이 없다.

브라우저의 자체 상한에 기댈 수 없다. Chrome은 응답 헤더를 기다리는 데 300초를 쓰고,
연결이 살아 있는 한(nginx `proxy_read_timeout 60s`가 끊어 주기 전까지) 그 시계는
다시 시작한다. 모바일에서 전파가 약해 TCP가 붙잡혀 있는 경우가 가장 흔하다.

# 왜 8초인가

실측으로 정했다. 공개 API 여덟 개의 응답 시간이 2.9~14.8ms이고, 가장 느린
`/earnings/highlights`가 758ms다(2026-09-20, 운영 컨테이너). 8초는 그 **열 배가
넘고**, nginx의 60초보다 한참 짧다 — 사용자가 「멈췄다」고 느끼기 전에 화면이
스스로 답을 내놓는 쪽을 고른다.

SSR은 더 짧다(`SSR_TIMEOUT_MS`). 서버에서 기다리는 시간은 **첫 바이트가 늦어지는
시간**이라 사용자에게 그대로 전가되고, 어차피 클라이언트가 다시 받는다 —
선반입은 못 하면 건너뛰면 되는 일이지 페이지를 붙잡을 일이 아니다.

# SSE에는 걸지 않는다

시세 스트림은 `EventSource`가 열고 응답이 끝나지 않는다. 이 파일의 함수는 `fetch`
경로에만 쓰이므로 스트림과 무관하다 — 섞이지 않게 해 두는 것이 요점이다.
*/

/** 브라우저 요청 상한. 가장 느린 실측 응답(758ms)의 열 배 위. */
export const DEFAULT_FETCH_TIMEOUT_MS = 8_000;

/**
 * 서버 렌더 중의 상한. 이 시간은 **TTFB에 그대로 더해진다.**
 *
 * `/news/domestic`의 SSR 선반입이 붙어 있는 상태의 TTFB 중앙값이 25.5ms라
 * (데이터 없는 라우트는 11~12ms) 정상 경로는 여기 근처도 안 온다. 3초는
 * 「백엔드가 흔들릴 때 페이지를 몇 초나 붙잡을 것인가」에 대한 답이고, 답은
 * **거의 안 붙잡는다**이다.
 */
export const SSR_TIMEOUT_MS = 3_000;

/**
 * 상한을 넘겨 우리가 끊은 요청.
 *
 * `AbortError`와 **구분되어야 한다** — 호출부가 스스로 취소한 것(화면 이동·
 * 검색어 변경)은 사용자에게 보여줄 오류가 아니지만, 시간 초과는 보여줄 오류다.
 */
export class FetchTimeoutError extends Error {
  readonly timeoutMs: number;

  constructor(timeoutMs: number) {
    super(`request timed out after ${timeoutMs}ms`);
    this.name = "FetchTimeoutError";
    this.timeoutMs = timeoutMs;
  }
}

export function isFetchTimeout(err: unknown): boolean {
  return err instanceof FetchTimeoutError;
}

/**
 * `fetch`에 상한을 건다. 호출부가 이미 `signal`을 들고 있으면 **둘 다** 듣는다.
 *
 * `AbortSignal.timeout()`을 쓰지 않는 이유는 두 가지다. ⓐ 그쪽이 던지는
 * `TimeoutError`는 DOMException이라 `instanceof`로 우리 것과 갈라내기 번거롭고,
 * ⓑ 호출부 signal과 합치려면 `AbortSignal.any()`가 필요한데 그건 여기서 지원
 * 범위를 넓게 잡을 이유가 없는 의존이다. 직접 만든 컨트롤러 하나가 둘 다 푼다.
 */
export function defaultTimeoutMs(): number {
  /*
    `news-api.ts`처럼 **서버와 브라우저가 같이 쓰는 모듈**이 여럿이다. 호출부마다
    어느 쪽에서 도는지 따져 상한을 골라 넘기게 하면 빠뜨리는 곳이 생기므로,
    기본값 자체가 환경을 본다. 명시적으로 넘긴 값은 언제나 이긴다.
  */
  return typeof window === "undefined" ? SSR_TIMEOUT_MS : DEFAULT_FETCH_TIMEOUT_MS;
}

export async function fetchWithTimeout(
  url: string,
  init: RequestInit = {},
  timeoutMs: number = defaultTimeoutMs(),
): Promise<Response> {
  /*
    **서버에서는 끊지 않고 기다리기만 한다.**

    `news-api.ts`처럼 서버와 브라우저가 같이 쓰는 모듈이 여럿이고, 그 중 일부는
    `next: { revalidate }`를 얹는다. 거기에 중단 신호가 붙으면 Next의 데이터 캐시가
    본문을 가르다 터진다 — 실측한 오류와 이유는 `fetchWithDeadline` 머리글.

    호출부마다 어느 쪽에서 도는지 따져 함수를 고르게 하면 빠뜨리는 곳이 생기므로,
    갈림길을 여기 한 곳에 둔다. 서버에서 취소가 아쉬울 일도 없다 — 남은 요청은
    캐시를 채우고 끝난다.
  */
  if (typeof window === "undefined") return fetchWithDeadline(url, init, timeoutMs);

  const controller = new AbortController();
  let timedOut = false;

  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, timeoutMs);

  /*
    호출부의 취소를 우리 컨트롤러로 이어 붙인다. 이미 취소된 신호가 넘어오는
    경우도 있어(리렌더 경합) `aborted`를 먼저 본다 — 리스너만 걸면 그 이벤트는
    이미 지나갔으므로 영영 안 울린다.
  */
  const outer = init.signal;
  const relay = () => controller.abort();
  if (outer) {
    if (outer.aborted) controller.abort();
    else outer.addEventListener("abort", relay, { once: true });
  }

  try {
    return await fetch(url, { ...init, signal: controller.signal });
  } catch (err) {
    // 우리가 끊은 것만 시간 초과다. 호출부가 끊었으면 그쪽 오류를 그대로 올린다.
    if (timedOut && !outer?.aborted) throw new FetchTimeoutError(timeoutMs);
    throw err;
  } finally {
    clearTimeout(timer);
    outer?.removeEventListener("abort", relay);
  }
}

/**
 * **기다림에만 상한을 건다 — 요청은 끊지 않는다.**
 *
 * # 왜 따로 있나 (2026-09-20, 재배포 직후 실측)
 *
 * `next: { revalidate }`가 붙은 서버 fetch에 `signal`을 달았더니 Next 컨테이너가
 * 냉시동 첫 렌더에서 세 번 터졌다:
 *
 *   `⨯ TypeError: controller[kState].transformAlgorithm is not a function`
 *
 * Next의 데이터 캐시는 응답 본문을 `TransformStream`으로 **갈라** 한쪽은 호출부에,
 * 한쪽은 캐시에 흘린다. 거기에 중단 신호가 얹히면 그 갈래가 깨진다. 요청 수와
 * 오류 수가 셋으로 맞았고, 신호를 떼자 사라졌다.
 *
 * # 끊지 않아도 되는 이유
 *
 * 이 경로에서 필요한 것은 **TTFB의 상한**이지 요청의 취소가 아니다. 서버 렌더는
 * 어차피 기다린 만큼만 늦어지고, 남은 요청은 백그라운드에서 끝나 **캐시를 채운다** —
 * 다음 사람이 그 값을 쓴다. 브라우저 쪽은 다르다: 거기서는 화면을 떠난 요청이
 * 회선과 배터리를 쓰므로 진짜로 끊어야 하고, 그래서 `fetchWithTimeout`이 남는다.
 *
 * 버려진 promise의 거절은 여기서 삼킨다 — 안 그러면 경주에서 진 쪽이
 * `unhandledRejection`이 된다.
 */
export async function fetchWithDeadline(
  url: string,
  init: RequestInit = {},
  deadlineMs: number = SSR_TIMEOUT_MS,
): Promise<Response> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const deadline = new Promise<never>((_resolve, reject) => {
    timer = setTimeout(() => reject(new FetchTimeoutError(deadlineMs)), deadlineMs);
  });

  const request = fetch(url, init);
  request.catch(() => {
    /* 경주에서 져도 이 promise는 누군가 받아야 한다. */
  });

  try {
    return await Promise.race([request, deadline]);
  } finally {
    clearTimeout(timer);
  }
}

/**
 * 재시도 간격 — 2초 → 4초 (지수 백오프).
 *
 * **두 번까지만 물러선다.** 세 번째부터는 사용자가 기다리는 시간이 요청 상한
 * 셋을 합친 것(최대 24초 + 대기 6초)이 되는데, 그 정도면 「다시 시도」 버튼을
 * 보여 주는 편이 낫다 — 사용자는 그때 새로고침이 아닌 다른 선택(다른 화면으로
 * 가기)도 할 수 있어야 한다.
 */
const RETRY_DELAYS_MS = [2_000, 4_000];

/**
 * 재시도해도 답이 바뀔 수 있는 실패인가.
 *
 * **4xx는 재시도하지 않는다.** 400·404는 우리가 잘못 물은 것이고, 401·403은
 * 쿠키가 없다는 뜻이라 2초 뒤에도 같은 답이다 — `session-fetch.ts`가 같은 이유로
 * 갱신 재시도를 한 번만 하고 굳힌다. 429도 여기서 제외한다: 물러설 곳은
 * 요청 경로가 아니라 **부르는 주기**다(메모리 「가드는 호출 지점이 아니라
 * 함수 안에」와 같은 사건).
 */
function worthRetrying(err: unknown): boolean {
  if (isFetchTimeout(err)) return true;
  const status = (err as { status?: number } | null)?.status;
  if (typeof status === "number") return status >= 500;
  // 상태코드가 없으면 네트워크 계층 실패 — 전파가 돌아오면 성공할 수 있다.
  return true;
}

/**
 * 일시적 실패를 두 번까지 되물어 본다.
 *
 * 화면이 「다시 시도」를 띄우기 **전에** 조용히 한 번 더 해 보는 자리다. 이것이
 * 없으면 지하철에서 한 칸 지나는 동안의 끊김이 곧바로 에러 화면이 된다.
 */
export async function retryingFetch<T>(
  run: (attempt: number) => Promise<T>,
  delaysMs: readonly number[] = RETRY_DELAYS_MS,
): Promise<T> {
  let lastErr: unknown;
  for (let attempt = 0; attempt <= delaysMs.length; attempt += 1) {
    try {
      return await run(attempt);
    } catch (err) {
      lastErr = err;
      if (attempt === delaysMs.length || !worthRetrying(err)) throw err;
      await new Promise((resolve) => setTimeout(resolve, delaysMs[attempt]));
    }
  }
  throw lastErr;
}
