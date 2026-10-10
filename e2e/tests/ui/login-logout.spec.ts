import { SYMBOLS } from "@/data";
import { expect, test } from "@/fixtures";
import { waitForHydration, holdingItem } from "@/ui";
import type { AuthSession } from "@/types";

/**
 * 로그인 → 세션 → 로그아웃 (브라우저 필수).
 *
 * 왜 이건 API로 못 하나: 세션 쿠키가 **HttpOnly**라 브라우저 밖에서는 "발급됐다"는
 * 것까지만 확인할 수 있다. 실제로 물어야 하는 것은 그 다음이다 — 브라우저가 그
 * 쿠키를 보관해서 **Next rewrite를 거친 다음 요청에 붙이는가**, 그리고 로그아웃이
 * 그 보관분을 실제로 지우는가. 이 경로에는 FE 미들웨어, AuthProvider,
 * `session-fetch`, rewrite가 전부 끼어 있고 어느 하나가 깨져도 API 계약은 멀쩡하다.
 *
 * 그래서 여기서만 로그인 폼을 직접 채운다. 다른 UI 테스트는 세션을 주입받는다 —
 * 로그인이 깨졌을 때 무관한 테스트까지 빨개지면 원인이 묻힌다.
 */

test.describe("세션 수명주기", () => {
  test("폼 로그인이 보호 화면의 내 데이터까지 이어지고, 로그아웃이 그것을 되돌린다", async ({
    page,
    user,
  }) => {
    // 사전 조건은 API로 만든다 — 검증 대상은 폼이지 매매 등록 화면이 아니다.
    await user.api.post("/user/holdings/trades", {
      symbol: SYMBOLS.trade,
      side: "buy",
      quantity: 7,
      price: 123,
      currency: "USD",
      tradeDate: "2026-04-01",
    });

    await page.goto("/login");
    await waitForHydration(page);
    await page.getByLabel("아이디").fill(user.email);
    await page.getByLabel("비밀번호").fill(user.password);
    await page.getByRole("button", { name: "로그인" }).click();

    // 1) 로그인은 홈으로 돌려보낸다.
    await expect(page, "로그인 후 홈으로 이동해야 한다").toHaveURL("/");

    // 2) 브라우저가 두 쿠키를 실제로 보관하고 있고, 스크립트로는 못 읽는다.
    const cookies = await page.context().cookies();
    const access = cookies.find((c) => c.name === "qp_access");
    const refresh = cookies.find((c) => c.name === "qp_refresh");
    expect(access, "브라우저에 qp_access 가 없다 — 다음 요청이 전부 비로그인이 된다").toBeDefined();
    expect(refresh, "브라우저에 qp_refresh 가 없다 — 15분 뒤 세션이 끊긴다").toBeDefined();
    expect(access!.httpOnly, "access 쿠키가 JS에 노출되면 XSS 한 번으로 세션이 털린다").toBe(true);
    expect(
      await page.evaluate(() => document.cookie),
      "document.cookie 에 세션 쿠키가 보이면 안 된다",
    ).not.toContain("qp_access");

    // 3) 화면이 누구로 로그인했는지 정확히 말한다.
    await page.getByRole("button", { name: "계정 메뉴" }).first().click();
    await expect(
      page.getByText(user.email, { exact: true }),
      "계정 메뉴가 다른 사람의 이메일을 보여주면 세션이 섞인 것",
    ).toBeVisible();
    await page.keyboard.press("Escape");

    // 4) 보호 화면이 **이 계정의 데이터**를 그린다. 로그인 안내가 아니라.
    await page.goto("/holdings");
    const row = holdingItem(page, SYMBOLS.trade);
    await expect(row, `보유 목록에 ${SYMBOLS.trade} 행이 있어야 한다`).toBeVisible();
    await expect(row, "API에 넣은 수량 7이 화면에 그대로 와야 한다").toContainText("7");

    // 5) 로그아웃.
    await page.getByRole("button", { name: "계정 메뉴" }).first().click();
    await page.getByRole("menuitem", { name: "로그아웃" }).click();

    // 헤더가 비로그인 상태로 바뀌는 것이 로그아웃이 반영된 신호다.
    await expect(page.getByRole("link", { name: "회원가입" }).first()).toBeVisible();

    // 6) 쿠키가 실제로 사라졌는지 — 화면 상태만 바뀌고 쿠키가 남으면 다음
    //    새로고침에 세션이 되살아난다.
    const afterLogout = await page.context().cookies();
    expect(
      afterLogout.find((c) => c.name === "qp_access")?.value ?? "",
      "로그아웃 후에도 access 쿠키 값이 남아 있다",
    ).toBe("");

    // 7) 그리고 보호 화면은 다시 잠긴다 — 방금 보였던 보유 행이 없어야 한다.
    await page.goto("/holdings");
    await expect(
      page.getByRole("link", { name: /로그인하고 시작하기/ }),
      "로그아웃 뒤에도 보호 화면이 열려 있으면 세션 무효화가 화면에 반영되지 않은 것",
    ).toBeVisible();
    await expect(holdingItem(page, SYMBOLS.trade)).toHaveCount(0);
  });

  test("틀린 비밀번호는 화면에 오류를 남기고 세션을 만들지 않는다", async ({ page, user, anon }) => {
    await page.goto("/login");
    await waitForHydration(page);
    await page.getByLabel("아이디").fill(user.email);
    await page.getByLabel("비밀번호").fill(`${user.password}-wrong`);
    await page.getByRole("button", { name: "로그인" }).click();

    /*
     * 사용자에게 실패가 보여야 한다 — 조용히 아무 일도 안 일어나면 안 된다.
     *
     * 문구는 **한국어여야 한다.** 예전에는 백엔드 원문(`invalid email or password`)이
     * 그대로 화면에 나왔고, 이 단언도 그 영문을 허용하고 있었다 — 테스트가 버그를
     * 함께 굳히고 있었던 셈이다. 지금은 `toUserMessage`가 사용자 문구로 바꾼다.
     */
    await expect(page.getByText(/이메일 또는 비밀번호가 올바르지 않습니다/)).toBeVisible();
    await expect(
      page.getByText(/invalid email or password/),
      "백엔드 원문이 사용자에게 그대로 보이면 안 된다",
    ).toHaveCount(0);
    await expect(page, "실패했는데 홈으로 넘어가면 안 된다").toHaveURL(/\/login/);

    // 그리고 쿠키가 생기지 않았다.
    const cookies = await page.context().cookies();
    expect(cookies.find((c) => c.name === "qp_access")).toBeUndefined();

    // 계정 자체는 멀쩡하다 — 실패한 시도가 계정을 잠그거나 지우지 않았다.
    const session = await anon.post<AuthSession>("/auth/login", {
      email: user.email,
      password: user.password,
    });
    expect(session.user.id).toBe(user.id);
  });
});
