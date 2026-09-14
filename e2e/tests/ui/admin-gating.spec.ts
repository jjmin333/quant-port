import { expect, test } from "@/fixtures";

/**
 * 관리자 화면 노출 (브라우저 필수).
 *
 * 왜 브라우저인가: 관리자 메뉴 숨김과 화면 접근 차단이 **클라이언트에서만**
 * 일어난다(`isAdminSession` → 사이드바 필터, 패널의 `router.replace`). BE는
 * 403을 잘 내지만, 그건 이미 `tests/api/authorization.spec.ts`가 본다. 여기서
 * 확인하는 것은 **일반 사용자가 관리자 화면을 마주치지 않는가**다.
 *
 * 보안 경계 자체는 BE에 있다는 점을 분명히 해 둔다 — 이 테스트가 통과해도
 * 데이터를 지키는 것은 서버의 403이다. 여기는 사용자 경험이 그 위에 어긋나지
 * 않는지를 본다.
 */

const ADMIN_PAGES = ["/admin", "/admin/batch", "/admin/database", "/admin/llm"];

test.describe("일반 사용자", () => {
  test("사이드바에 관리 메뉴가 하나도 없다", async ({ userPage }) => {
    await userPage.goto("/");

    // 로그인 상태가 반영된 뒤에 세어야 한다 — 「내 투자」가 보이면 반영된 것.
    await expect(userPage.getByRole("link", { name: "내 포트폴리오" }).first()).toBeVisible();

    for (const label of ["배치 관리", "데이터베이스 관리", "LLM 모델 관리", "캐시 관리"]) {
      await expect(
        userPage.getByRole("link", { name: label }),
        `일반 사용자에게 "${label}" 메뉴가 보인다`,
      ).toHaveCount(0);
    }
  });

  for (const path of ADMIN_PAGES) {
    test(`${path} 에 직접 들어가도 머무를 수 없다`, async ({ userPage }) => {
      await userPage.goto(path);

      // 관리자 패널은 비관리자를 홈으로 돌려보낸다(/dashboard → 미들웨어가 / 로).
      await expect(
        userPage,
        `${path} 에 일반 사용자가 남아 있다 — 화면이 관리자 UI를 그리게 된다`,
      ).toHaveURL(/\/$/);

      // 그리고 관리자 데이터가 한 조각도 그려지지 않았다.
      await expect(userPage.getByRole("heading", { name: "배치 관리" })).toHaveCount(0);
      await expect(userPage.getByRole("heading", { name: "데이터베이스 관리" })).toHaveCount(0);
    });
  }
});

test.describe("비로그인", () => {
  test("관리자 화면에 접근해도 관리자 데이터가 그려지지 않는다", async ({ page }) => {
    const forbidden: string[] = [];
    page.on("response", (res) => {
      const path = new URL(res.url()).pathname;
      if (path.startsWith("/api/v1/admin/") && res.ok()) forbidden.push(path);
    });

    await page.goto("/admin/database");

    // 관리자 API가 단 한 건도 성공하면 안 된다.
    expect(
      forbidden,
      `비로그인인데 관리자 API가 성공했다: ${forbidden.join(", ")}`,
    ).toEqual([]);
    await expect(page.getByRole("table")).toHaveCount(0);
  });
});
