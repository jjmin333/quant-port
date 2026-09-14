import { expect, test } from "@/fixtures";

/**
 * 퀵룩 모달 바닥의 「종목 분석 보기」 (회귀 방어).
 *
 * # 무엇이 고장났었나 (2026-09-06 사용자 지적 1)
 *
 * 눌러도 **아무 일도 일어나지 않았다.** 모달만 닫히고 화면은 그대로였다.
 *
 * 모달은 열려 있는 동안 히스토리 항목 하나를 점유하고(`hooks/use-modal-history`),
 * 닫히면 그 항목을 `history.back()`으로 걷어낸다. 그런데 이 버튼은 **닫기와 이동을
 * 한 클릭 안에서** 한다. Next `<Link>`의 이동은 갈 화면의 페이로드를 기다리므로,
 * 그 사이 먼저 나간 되돌리기가 진행 중이던 이동을 밀어냈다 — 실측:
 *
 *     back → replace:/holdings(__NA) → pop@/holdings     (이동이 통째로 사라졌다)
 *
 * 순서는 갈 화면이 얼마나 빨리 준비되는지에 달려 있어, 준비된 화면에서는 우연히
 * 되기도 했다. 그래서 「가끔 된다」로 보였다.
 *
 * # 여기서 지키는 것
 *
 * ① 눌렀을 때 실제로 `/analysis`에 **도착**하고 그 종목이 그려진다.
 * ② 뒤로가기 **한 번**이면 원래 화면으로 돌아온다 — 걷지 않고 남긴 항목은 주소가
 *    같아 사용자에게 보이지 않아야 한다.
 *
 * 브라우저가 필요한 이유: 고장이 히스토리 항목의 **경합**이라 응답만 봐서는 보이지
 * 않는다. 눌러 보는 것 말고 이 버그를 잡는 방법이 없다.
 */

const SAMPLE_HOLDING = { name: "삼성전자", symbol: "005930" };

test.describe("퀵룩 — 화면을 떠나는 버튼", () => {
  test("「종목 분석 보기」가 그 종목의 분석 화면으로 실제로 이동한다", async ({ page }) => {
    // 비로그인도 볼 수 있는 예시 데이터로 연다 — 퀵룩은 로그인과 무관하다.
    await page.goto("/holdings?sample=1");

    const row = page.getByRole("button", { name: new RegExp(SAMPLE_HOLDING.name) }).first();
    await expect(row).toBeVisible({ timeout: 30_000 });
    await row.click();

    const leave = page.getByRole("link", { name: "종목 분석 보기" });
    await expect(leave).toBeVisible({ timeout: 30_000 });
    await leave.click();

    // ① 도착했는가. 주소만 보면 라우터가 되돌려 놓은 경우를 놓친다 — 화면도 본다.
    await expect(page).toHaveURL(new RegExp(`symbol=${SAMPLE_HOLDING.symbol}`), {
      timeout: 30_000,
    });
    await expect(
      page.getByRole("heading", { name: SAMPLE_HOLDING.name }).first(),
      "이동은 했는데 그 종목이 아니면 버튼이 하는 말과 다르다",
    ).toBeVisible({ timeout: 30_000 });

    // ② 뒤로가기 한 번이면 원래 화면. 모달이 남긴 항목은 주소가 같아 보이지 않는다.
    await page.goBack();
    await expect(page).toHaveURL(/\/holdings/, { timeout: 15_000 });
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });
});
