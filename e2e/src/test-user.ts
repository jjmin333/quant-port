import type { APIRequestContext, Browser, Page } from "@playwright/test";
import { request as playwrightRequest } from "@playwright/test";

import { Api } from "./api-client";
import { PASSWORD, uniqueDisplayName, uniqueEmail } from "./data";
import { API_URL, FE_URL, apiPath } from "./env";
import type {
  AuthSession,
  HoldingTrade,
  JournalNote,
  PriceTarget,
  ResearchCall,
  Stamp,
  UserHolding,
  WatchlistItem,
} from "./types";

/**
 * 테스트 1개 = 계정 1개.
 *
 * 계정을 공유하지 않는 이유는 격리 원칙만이 아니다. 이 도메인의 규칙 상당수가
 * **계정 단위 쿨다운·상한**이다 — 리서치 콜은 하루 5건에 같은 심볼 30일 쿨다운,
 * 스탬프는 심볼당 5분 쿨다운. 계정을 재사용하면 두 번째 실행부터 테스트가
 * "규칙 때문에" 실패하고, 그건 회귀가 아니라 픽스처 문제다.
 *
 * 대가: **계정 행은 지울 수 없다.** 계정 삭제 API가 없고 관리자 DB 콘솔에서도
 * `users`는 read-only다. `scripts/cleanup.sql`로 주기적으로 회수한다.
 */
export class TestUser {
  private constructor(
    readonly api: Api,
    readonly ctx: APIRequestContext,
    readonly email: string,
    readonly password: string,
    readonly id: number,
    readonly displayName: string,
  ) {}

  static async create(tag: string): Promise<TestUser> {
    const email = uniqueEmail(tag);
    const displayName = uniqueDisplayName(tag);
    const ctx = await newApiContext();
    const api = new Api(ctx, `anon:${tag}`);
    const session = await api.post<AuthSession>("/auth/signup", {
      email,
      password: PASSWORD,
      displayName,
    });
    const authed = new Api(ctx, `user#${session.user.id}`);
    return new TestUser(authed, ctx, email, PASSWORD, session.user.id, displayName);
  }

  /**
   * 이 계정의 쿠키를 그대로 물려받은 브라우저 페이지.
   *
   * 로그인 UI를 거치지 않고 세션을 만든다 — 로그인 화면 자체가 검증 대상인
   * 테스트는 이걸 쓰지 않고 직접 폼을 채운다. 나머지 UI 테스트에서 매번
   * 로그인 폼을 반복하면 로그인이 깨졌을 때 **관계 없는 테스트가 전부 빨개져서**
   * 원인을 가린다.
   *
   * 쿠키가 포트를 넘어 전달되는 이유: 쿠키는 포트로 스코프되지 않는다.
   * `:8080`이 심은 `qp_access`(Path=/)는 같은 host의 `:3000`에도 그대로 간다.
   */
  async newPage(browser: Browser): Promise<{ page: Page; close: () => Promise<void> }> {
    const state = await this.ctx.storageState();
    const context = await browser.newContext({ storageState: state, baseURL: FE_URL });
    const page = await context.newPage();
    return { page, close: () => context.close() };
  }

  /**
   * 이 계정이 만든 것을 전부 되돌린다.
   *
   * 삭제 API가 있는 것은 지우고, 없는 것(리서치 콜)은 **비공개로 되돌린다** —
   * 공개 피드는 전역이라, 테스트가 만든 콜이 남으면 다른 테스트가 보는 목록이
   * 실행할 때마다 달라진다. 콜은 설계상 지울 수 없고(적중률이 광고가 되지
   * 않으려면 그래야 한다) `visibility`만 편집 가능하다.
   *
   * 실패해도 던지지 않는다. 정리 중 오류로 테스트를 빨갛게 만들면 진짜 실패와
   * 구분이 안 된다 — 대신 경고로 남긴다.
   */
  async cleanup(): Promise<void> {
    // 테스트가 로그아웃으로 끝났거나 토큰이 만료됐을 수 있다. 그대로 정리에
    // 들어가면 전부 401로 실패하고 데이터가 남는다 — 먼저 세션을 되살린다.
    await this.tryEach("re-auth", async () => {
      const probe = await this.api.send("GET", "/auth/me");
      if (probe.ok()) return;
      await this.api.post("/auth/login", { email: this.email, password: this.password });
    });

    await this.tryEach("stamps", async () => {
      const rows = await this.api.get<Stamp[]>("/stamps");
      for (const row of rows) await this.api.send("DELETE", `/stamps/${row.id}`);
    });
    await this.tryEach("price-targets", async () => {
      const rows = await this.api.get<PriceTarget[]>("/user/price-targets");
      for (const row of rows) await this.api.send("DELETE", `/user/price-targets/${row.id}`);
    });
    await this.tryEach("journal", async () => {
      const rows = await this.api.get<JournalNote[]>("/journal/notes");
      for (const row of rows) await this.api.send("DELETE", `/journal/notes/${row.id}`);
    });
    await this.tryEach("watchlist", async () => {
      const rows = await this.api.get<WatchlistItem[]>("/watchlist");
      for (const row of rows) await this.api.send("DELETE", `/watchlist/${row.symbol}`);
    });
    await this.tryEach("trades", async () => {
      const rows = await this.api.get<HoldingTrade[]>("/user/holdings/trades");
      for (const row of rows) {
        await this.api.send("DELETE", `/user/holdings/trades/${row.id}?symbol=${row.symbol}`);
      }
    });
    await this.tryEach("holdings", async () => {
      const rows = await this.api.get<UserHolding[]>("/user/holdings");
      for (const row of rows) await this.api.send("DELETE", `/user/holdings/${row.symbol}`);
    });
    await this.tryEach("research", async () => {
      const rows = await this.api.get<ResearchCall[]>("/research/mine");
      for (const row of rows) {
        if (row.visibility === "private") continue;
        await this.api.send("PATCH", `/research/calls/${row.id}`, {
          title: row.title,
          bodyMd: row.bodyMd,
          visibility: "private",
        });
      }
    });
    await this.tryEach("cash", async () => {
      await this.api.send("PUT", "/user/cash-balances", { krw: 0, usd: 0 });
    });
    // 세션까지 끊어야 `user_refresh_tokens`에 행이 남지 않는다.
    await this.tryEach("logout", async () => {
      await this.api.send("POST", "/auth/logout");
    });
    await this.ctx.dispose();
  }

  private async tryEach(what: string, fn: () => Promise<void>): Promise<void> {
    try {
      await fn();
    } catch (err) {
      // eslint-disable-next-line no-console
      console.warn(`[cleanup] ${this.email} / ${what}: ${(err as Error).message}`);
    }
  }
}

/** 쿠키를 자체적으로 물고 있는 새 API 컨텍스트. 컨텍스트가 곧 세션이다. */
export async function newApiContext(
  options?: Parameters<typeof playwrightRequest.newContext>[0],
): Promise<APIRequestContext> {
  return playwrightRequest.newContext({
    baseURL: API_URL,
    extraHTTPHeaders: { "Content-Type": "application/json" },
    ...options,
  });
}

/** 이메일/비밀번호로 로그인한 API 컨텍스트. 관리자 계정처럼 이미 있는 계정용. */
export async function loginAs(email: string, password: string, label: string): Promise<Api> {
  const ctx = await newApiContext();
  const api = new Api(ctx, label);
  const res = await api.send("POST", "/auth/login", { email, password });
  if (!res.ok()) {
    throw new Error(
      `${label} 로그인 실패 (${res.status()}): ${(await res.text()).slice(0, 200)}\n` +
        `E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD 를 확인해 주세요.`,
    );
  }
  return api;
}
