import { expect, type APIRequestContext, type APIResponse } from "@playwright/test";

import { apiPath } from "./env";

/**
 * BE 응답 봉투를 벗기는 얇은 래퍼.
 *
 * 직접 `request.get()`을 쓰지 않는 이유는 두 가지다.
 *
 *  1. **실패 메시지.** `expect(res).toBeOK()`만 쓰면 "예상 2xx, 실제 502"까지만
 *     보인다. 이 프로젝트는 매핑되지 않은 서비스 에러를 전부 502로 내보내기
 *     때문에(`handler/toss_error.go`) 상태코드만으로는 원인을 못 찾는다. 그래서
 *     실패할 때 메서드·경로·상태·본문을 한 줄에 담아 던진다.
 *  2. **봉투.** 성공은 `{data}`, 실패는 `{error}`로 고정돼 있다
 *     (`pkg/response`). 이 규약 자체도 계약 테스트의 대상이라, 벗기는 곳을
 *     한 군데로 모아 두면 규약이 깨졌을 때 한 곳에서 잡힌다.
 */

export type Envelope<T> = { data: T };
export type ErrorEnvelope = { error: string };

/** 429를 받았을 때 물러설 간격. 토큰 버킷이 다시 차기를 기다리는 시간이다. */
const RATE_LIMIT_BACKOFF_MS = [250, 500, 1_000, 2_000, 4_000];

export class Api {
  constructor(
    readonly ctx: APIRequestContext,
    /** 실패 메시지에 "누구로서 부른 요청인지"를 남긴다 (anon / user#12 …). */
    readonly label: string,
  ) {}

  /**
   * 상태코드를 검증하지 않는 원시 호출. 실패 경로를 검증할 때 쓴다.
   *
   * 429만 되짚어 다시 보낸다. 이건 임의의 대기가 아니라 **서버가 명시적으로
   * 보낸 신호에 따르는 것**이다 — BE는 비로그인 요청을 IP당 30 req/s(버스트 60)로
   * 세는데(`internal/middleware/ratelimit.go`), 병렬 워커가 계정을 만들며 내는
   * 트래픽은 사람 한 명보다 당연히 촘촘하다. 대안은 테스트 환경에서
   * `RATE_LIMIT_ENABLED=false`로 리미터를 꺼 버리는 것인데, 그러면 검증 대상
   * 환경에서 실제 보호 장치가 빠진다.
   *
   * 429는 이 스위트의 어떤 단언에서도 기대 상태가 아니므로, 여기서 흡수해도
   * 가리는 정보가 없다. 상한을 넘기면 그대로 429를 돌려주어 실패로 드러난다.
   */
  async send(
    method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE",
    path: string,
    body?: unknown,
  ): Promise<APIResponse> {
    let res = await this.sendOnce(method, path, body);
    for (let attempt = 0; res.status() === 429 && attempt < RATE_LIMIT_BACKOFF_MS.length; attempt++) {
      // 재시도를 조용히 삼키면 "왜 느린지" "왜 가끔 깨지는지"를 못 본다. BE
      // 로그에서 실제로 81건의 429를 확인했고, 전부 이 백오프가 흡수한 것이었다.
      // eslint-disable-next-line no-console
      console.warn(
        `[rate-limit] ${method} ${path} → 429, ${RATE_LIMIT_BACKOFF_MS[attempt]}ms 후 재시도 ` +
          `(${attempt + 1}/${RATE_LIMIT_BACKOFF_MS.length})`,
      );
      await new Promise((resolve) => setTimeout(resolve, RATE_LIMIT_BACKOFF_MS[attempt]));
      res = await this.sendOnce(method, path, body);
    }
    return res;
  }

  private async sendOnce(
    method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE",
    path: string,
    body?: unknown,
  ): Promise<APIResponse> {
    const url = apiPath(path);
    const options = body === undefined ? undefined : { data: body };
    switch (method) {
      case "GET":
        return this.ctx.get(url, options);
      case "POST":
        return this.ctx.post(url, options ?? { data: {} });
      case "PUT":
        return this.ctx.put(url, options ?? { data: {} });
      case "PATCH":
        return this.ctx.patch(url, options ?? { data: {} });
      case "DELETE":
        return this.ctx.delete(url, options);
    }
  }

  /** 2xx를 요구하고 `data`를 꺼낸다. 실패하면 본문까지 담아 즉시 실패한다. */
  async ok<T>(
    method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE",
    path: string,
    body?: unknown,
  ): Promise<T> {
    const res = await this.send(method, path, body);
    if (!res.ok()) {
      throw new Error(await describe(res, method, path, this.label));
    }
    const text = await res.text();
    if (text.trim() === "") {
      throw new Error(`[${this.label}] ${method} ${path} → ${res.status()} 이지만 본문이 비어 있다`);
    }
    let parsed: unknown;
    try {
      parsed = JSON.parse(text);
    } catch {
      throw new Error(`[${this.label}] ${method} ${path} → JSON이 아니다: ${text.slice(0, 300)}`);
    }
    if (parsed === null || typeof parsed !== "object" || !("data" in parsed)) {
      throw new Error(
        `[${this.label}] ${method} ${path} → 성공 응답에 "data" 봉투가 없다: ${text.slice(0, 300)}`,
      );
    }
    return (parsed as Envelope<T>).data;
  }

  get<T>(path: string): Promise<T> {
    return this.ok<T>("GET", path);
  }
  post<T>(path: string, body?: unknown): Promise<T> {
    return this.ok<T>("POST", path, body);
  }
  put<T>(path: string, body?: unknown): Promise<T> {
    return this.ok<T>("PUT", path, body);
  }
  patch<T>(path: string, body?: unknown): Promise<T> {
    return this.ok<T>("PATCH", path, body);
  }
  del<T>(path: string, body?: unknown): Promise<T> {
    return this.ok<T>("DELETE", path, body);
  }

  /**
   * 실패를 기대하는 호출. 상태코드를 단언하고 `error` 문자열을 돌려준다.
   *
   * 상태코드만 보고 넘어가지 않는 이유: 401과 403은 뜻이 다르고(인증 없음 vs
   * 권한 없음), 404는 "없다"와 "네 것이 아니다" 둘 다에 쓰인다. 어느 쪽인지는
   * 메시지에 있다.
   */
  async expectFailure(
    method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE",
    path: string,
    expectedStatus: number,
    body?: unknown,
  ): Promise<string> {
    const res = await this.send(method, path, body);
    expect(
      res.status(),
      `[${this.label}] ${method} ${path} 는 ${expectedStatus}여야 한다 — 실제 본문: ${(
        await res.text()
      ).slice(0, 300)}`,
    ).toBe(expectedStatus);
    const text = await res.text();
    if (text.trim() === "") return "";
    try {
      return (JSON.parse(text) as ErrorEnvelope).error ?? "";
    } catch {
      return text;
    }
  }
}

async function describe(
  res: APIResponse,
  method: string,
  path: string,
  label: string,
): Promise<string> {
  const body = (await res.text()).slice(0, 500);
  return `[${label}] ${method} ${path} → ${res.status()} ${res.statusText()}\n본문: ${body}`;
}
