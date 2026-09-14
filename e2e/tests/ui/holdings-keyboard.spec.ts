import { SYMBOLS } from "@/data";
import { expect, test } from "@/fixtures";
import type { TestUser } from "@/test-user";

/**
 * 보유 목록 — **키보드만으로** 종목 상세를 연다 (회귀 방어).
 *
 * # 왜 이 테스트가 있나
 *
 * 2026-09-06에 보유 목록 기본 보기가 표 → 카드로 바뀌었는데, 그 카드는
 * `onClick`만 달린 `<div>`였다. 마우스로는 멀쩡했고 화면도 정상으로 보였지만
 * **탭으로 닿지 않고 Enter로 눌리지 않았다** — 기본 보기가 카드이므로 모든
 * 사용자가 그 상태를 만났다.
 *
 * 이 앱은 같은 결함을 이미 한 번 겪었다(BUG-01: 모달 안 자동완성이 마우스로
 * 안 눌림). 그때 얻은 규칙이 **「클릭과 키보드를 둘 다 본다」** 이고,
 * `.click()`만 하는 테스트는 이 계열을 영원히 놓친다 — Playwright의 `.click()`은
 * 포커스도 role도 보지 않고 좌표를 때리기 때문이다.
 *
 * 그래서 여기서는 **`.click()`을 쓰지 않는다.** 키보드로 포커스를 옮기고
 * Enter를 눌러, 실제로 키보드 사용자가 밟는 경로만 지난다.
 *
 * 두 보기를 모두 돈다 — 기본값이 다시 바뀌어도 한쪽이 조용히 썩지 않게.
 */

const S = SYMBOLS.trade;

async function seedHolding(user: TestUser): Promise<void> {
  await user.api.post("/user/holdings/trades", {
    symbol: S,
    side: "buy",
    quantity: 3,
    price: 100,
    currency: "USD",
    tradeDate: "2026-02-02",
  });
}

/** 보기 모드는 localStorage에 있다 (`fe/src/lib/holdings-view.ts`). */
async function useView(page: import("@playwright/test").Page, view: "card" | "table") {
  await page.addInitScript((v) => {
    window.localStorage.setItem("quant-port-holdings-view", v);
  }, view);
}

for (const view of ["card", "table"] as const) {
  test(`보유 ${view} 보기: 키보드로 포커스하고 Enter로 상세를 연다`, async ({
    userPage,
    user,
  }) => {
    await seedHolding(user);
    await useView(userPage, view);
    await userPage.goto("/holdings");

    /*
      항목은 보기와 무관하게 접근 가능한 이름을 갖는다 (`{이름} {심볼}`).
      이름이 없으면 스크린리더 사용자에게는 "빈 항목"이고, 그 자체가 결함이다.
    */
    const item = userPage.locator(`[aria-label$="${S}"]`).first();
    await expect(item, "보유 항목에 접근 가능한 이름이 없다").toBeVisible();

    // ① 포커스를 받을 수 있어야 한다 — `tabIndex`가 없으면 여기서 죽는다.
    await item.focus();
    await expect(item, "보유 항목이 포커스를 받지 못한다 — 키보드로 닿을 수 없다").toBeFocused();

    // ② Enter로 눌려야 한다 — `onKeyDown`이 없으면 아무 일도 일어나지 않는다.
    await userPage.keyboard.press("Enter");

    await expect(
      userPage.getByRole("dialog"),
      "Enter를 눌러도 상세가 열리지 않는다 — 키보드 사용자는 이 화면의 주 동작을 못 한다",
    ).toBeVisible();
  });
}
