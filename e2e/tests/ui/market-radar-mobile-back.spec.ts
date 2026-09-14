import { expect, test } from "@/fixtures";

/**
 * 실시간 차트 — 목록↔상세 돌아가기 (회귀 방어).
 *
 * ① 목록→종목→돌아가기가 랭킹 목록으로 돌아와야 한다(홈으로 튀면 안 됨).
 * ② 돌아간 직후 URL에서 `symbol`이 빠져야 한다.
 *
 * # 돌아가는 길이 폭마다 다르다 (2026-09-03 사용자 결정)
 *
 * **폰에는 「목록으로」 버튼이 없다.** 스와이프와 브라우저 뒤로가기가 이미 그 일을
 * 하고, 버튼 하나가 왼쪽 34px과 현재가 줄의 들여쓰기를 함께 잡고 있었다
 * (`stock-workspace-header.tsx`의 `sm:hidden` 한 줄 헤더). 그래서 폰에서는
 * **제스처만** 검증한다.
 *
 * 버튼은 `sm`(640px) 이상이면서 아직 한 판인 폭 — 즉 **태블릿 띠(640~1023px)** 에만
 * 있다. 실측: 375·390 없음 / 768 있음 / 1440 없음(넓은 화면은 목록이 옆에 늘 떠
 * 있어 돌아갈 곳이 없다). 그 띠를 따로 덮지 않으면 버튼 경로는 아무도 안 지킨다.
 */

/** 상세가 열렸다는 신호 — 보기 폭과 무관하게 워크스페이스 탭이 뜬다. */
function detailOpened(page: import("@playwright/test").Page) {
  return page.getByRole("button", { name: "개요", exact: true });
}

test.describe("market radar back navigation", () => {
  test("폰: 브라우저 뒤로가기가 랭킹 목록으로 돌아온다", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto("/market");
    await page.waitForLoadState("domcontentloaded");

    const rankingRow = page.getByRole("button", { name: /\d+위$/ }).first();
    await expect(rankingRow).toBeVisible({ timeout: 30_000 });

    await rankingRow.click();
    await expect(page).toHaveURL(/symbol=/, { timeout: 15_000 });
    await expect(detailOpened(page)).toBeVisible({ timeout: 30_000 });

    /*
      폰 상세에 「목록으로」 버튼이 없다는 것 자체가 약속이다. 버튼이 되살아나면
      09-03 결정이 조용히 뒤집힌 것이므로 여기서 알려 준다.
    */
    await expect(page.getByRole("button", { name: "목록으로" })).toHaveCount(0);

    await page.goBack();

    await expect(page).toHaveURL(/\/market(?:\?|$)/);
    expect(page.url()).not.toMatch(/symbol=/);
    await expect(rankingRow).toBeVisible({ timeout: 5_000 });
  });

  test("태블릿: 「목록으로」 버튼이 랭킹 목록으로 돌아온다", async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto("/market");
    await page.waitForLoadState("domcontentloaded");

    const rankingRow = page.getByRole("button", { name: /\d+위$/ }).first();
    await expect(rankingRow).toBeVisible({ timeout: 30_000 });

    const rowLabel = (await rankingRow.getAttribute("aria-label")) ?? "";
    expect(rowLabel.length).toBeGreaterThan(0);

    await rankingRow.click();
    await expect(page).toHaveURL(/symbol=/, { timeout: 15_000 });

    /*
      상세가 **다 열린 뒤에** 누른다.

      「목록으로」 버튼은 워크스페이스가 마운트되기 전에 먼저 그려진다. 그 사이에
      필터 동기화 이펙트가 한 번 더 돌면 목록이 다시 핀 상태가 되고
      (`shouldPinListOnParamsChange`), 그러면 방금 본 버튼이 사라진 뒤에 클릭이
      도착한다 — 전체 실행처럼 dev 서버가 느릴 때만 열리는 창이다.
      폰 쪽 테스트는 이미 같은 조건을 기다린다.
    */
    await expect(detailOpened(page)).toBeVisible({ timeout: 30_000 });

    const backButton = page.getByRole("button", { name: "목록으로" });
    await expect(backButton).toBeVisible({ timeout: 30_000 });

    await backButton.click();

    await expect(page).toHaveURL(/\/market(?:\?|$)/);
    expect(page.url()).not.toMatch(/symbol=/);
    await expect(rankingRow).toBeVisible({ timeout: 5_000 });
  });
});
