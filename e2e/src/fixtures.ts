import { test as base, type Browser, type Page } from "@playwright/test";

import { Api } from "./api-client";
import { ADMIN_EMAIL, ADMIN_PASSWORD, hasAdminCredentials } from "./env";
import { TestUser, loginAs, newApiContext } from "./test-user";

type Fixtures = {
  /** 비로그인 API. 공개 경로와 401 검증에 쓴다. */
  anon: Api;
  /** 이 테스트만의 새 계정. 종료 시 자기가 만든 것을 전부 되돌린다. */
  user: TestUser;
  /** 소유권 경계 검증용 두 번째 계정. 필요할 때만 만들어진다. */
  otherUser: TestUser;
  /** 이 계정으로 로그인된 브라우저 페이지 (`user` 픽스처와 같은 세션). */
  userPage: Page;
  /**
   * 관리자 API. 자격증명이 없으면 `null`이고, 쓰는 테스트가 스스로 skip한다.
   * 테스트가 관리자 계정을 만들지는 않는다 — 이유는 `env.ts` 참고.
   */
  adminApi: Api | null;
};

export const test = base.extend<Fixtures>({
  anon: async ({}, use) => {
    const ctx = await newApiContext();
    await use(new Api(ctx, "anon"));
    await ctx.dispose();
  },

  user: async ({}, use, testInfo) => {
    const account = await TestUser.create(testInfo.title);
    await use(account);
    await account.cleanup();
  },

  otherUser: async ({}, use, testInfo) => {
    const account = await TestUser.create(`${testInfo.title}-other`);
    await use(account);
    await account.cleanup();
  },

  userPage: async ({ user, browser }: { user: TestUser; browser: Browser }, use) => {
    const { page, close } = await user.newPage(browser);
    await use(page);
    await close();
  },

  adminApi: async ({}, use) => {
    if (!hasAdminCredentials) {
      await use(null);
      return;
    }
    const api = await loginAs(ADMIN_EMAIL, ADMIN_PASSWORD, "admin");
    await use(api);
    await api.ctx.dispose();
  },
});

export { expect } from "@playwright/test";
export { hasAdminCredentials };
