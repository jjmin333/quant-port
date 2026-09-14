import { expect, test } from "@/fixtures";
import type { Page } from "@playwright/test";
import type { UserHolding } from "@/types";

/**
 * 모달 안에서 종목을 **마우스로** 고를 수 있어야 한다.
 *
 * 2026-08-26 QA에서 잡은 P0. 자동완성 패널은 `createPortal(document.body)`로 나가는데,
 * Radix Dialog가 열리면 `react-remove-scroll`이 `document.body`에 `pointer-events: none`을
 * 건다. 그래서 목록은 멀쩡히 보이는데 **클릭만 먹지 않았다** — 실측으로 옵션 중앙 좌표의
 * `document.elementFromPoint()`가 옵션이 아니라 다이얼로그 오버레이를 돌려줬고,
 * 「스탬프 찍기」의 유일한 주 행동이 마우스 사용자에게 불가능했다(키보드 ↓+Enter로만 가능).
 *
 * **키보드로 우회하지 않는다.** 이 테스트가 지키려는 것이 정확히 마우스 경로다.
 */

/** 옵션이 화면에서 실제로 클릭을 받는 자리에 있는지 — 보이는 것만으로는 부족하다. */
async function optionIsHittable(page: Page): Promise<boolean> {
  return page.evaluate(() => {
    const listbox = document.querySelector('[role="listbox"]');
    const option = listbox?.querySelector('[role="option"]');
    if (!option) return false;
    const rect = option.getBoundingClientRect();
    const top = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
    return Boolean(top && (option === top || option.contains(top)));
  });
}

test.describe("모달 안 종목 검색", () => {
  test("스탬프 찍기 — 마우스로 종목을 고르면 폼이 열리고 모달은 닫히지 않는다", async ({
    userPage,
  }) => {
    await userPage.goto("/stamps");
    await userPage.getByRole("button", { name: "스탬프 찍기" }).first().click();

    const dialog = userPage.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByPlaceholder(/종목명·티커 검색/).fill("삼성전자");

    const option = userPage.getByRole("option").first();
    await expect(option, "자동완성이 떠야 한다").toBeVisible();
    expect(
      await optionIsHittable(userPage),
      "옵션이 보이지만 클릭을 받지 못한다 — body의 pointer-events가 포털까지 내려온 것",
    ).toBe(true);

    await option.click();

    // 고른 결과가 폼에 반영됐는가. "클릭이 됐다"가 아니라 "선택이 됐다"를 본다.
    await expect(dialog.getByText("종목 변경")).toBeVisible();
    await expect(dialog.getByText("예상 주가", { exact: false })).toBeVisible();
    await expect(dialog, "종목을 고르는 클릭이 모달을 닫으면 안 된다").toBeVisible();
  });

  test("종목 일괄 추가 — 같은 경로가 보유 화면 모달에서도 동작한다", async ({ user, userPage }) => {
    // 「일괄 추가」 버튼은 사이드 레일 안이라 보유가 하나는 있어야 화면에 나온다.
    await user.api.post("/user/holdings/trades", {
      symbol: "MSFT",
      side: "buy",
      quantity: 1,
      price: 400,
      currency: "USD",
      tradeDate: "2026-02-02",
    });

    await userPage.goto("/holdings");
    await userPage.getByRole("button", { name: "일괄 추가" }).first().click();

    const dialog = userPage.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByPlaceholder("종목 검색").first().fill("삼성전자");

    await expect(userPage.getByRole("option").first()).toBeVisible();
    expect(await optionIsHittable(userPage)).toBe(true);
    await userPage.getByRole("option").first().click();
    await expect(dialog.getByText("삼성전자")).toBeVisible();
  });
});

/**
 * 보유가 0건인 사람도 **첫 종목을 추가할 수 있어야 한다** (§BUG-06).
 *
 * 추가 폼은 사이드 레일 안에 있고 그 레일은 0건이면 렌더되지 않는다. 예전 안내 카드의
 * CTA는 `/analysis`로 보냈고 거기에도, 종목 상세에도 보유 추가 버튼이 없어서 **가입 직후
 * 사용자는 진입점이 아예 없었다.**
 */
test.describe("첫 보유 종목 추가", () => {
  test("빈 보유 화면의 CTA가 추가 폼을 열고, 실제로 저장된다", async ({ user, userPage }) => {
    await userPage.goto("/holdings");

    await userPage.getByRole("button", { name: "종목 추가하기" }).click();
    const dialog = userPage.getByRole("dialog");
    await expect(dialog).toBeVisible();

    await dialog.getByPlaceholder("종목명·코드 검색").fill("NVDA");
    await userPage.getByRole("option").first().click();
    await userPage.getByLabel("수량").fill("7");
    await userPage.getByLabel(/매수 단가/).fill("150");
    await userPage.getByRole("button", { name: "보유에 추가" }).click();

    // 화면 문구가 아니라 API로 확인한다.
    await expect
      .poll(async () => (await user.api.get<UserHolding[]>("/user/holdings")).length, {
        message: "빈 상태에서 추가한 첫 보유가 저장되지 않았다",
      })
      .toBe(1);
    const holdings = await user.api.get<UserHolding[]>("/user/holdings");
    expect(holdings[0]?.symbol).toBe("NVDA");
    expect(Number(holdings[0]?.quantity)).toBe(7);
  });
});
