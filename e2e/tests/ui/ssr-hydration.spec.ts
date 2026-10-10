import { expect, test } from "@/fixtures";
import { FE_URL } from "@/env";
import type { ConsoleMessage, Page } from "@playwright/test";

/**
 * **공개 데이터가 어디서 보든 같은 화면으로 뜨는가** (2026-09-20 SSR/CSR 점검 요청 8·9).
 *
 * # 무엇을 잠그나
 *
 * 2026-09-20 점검 전, 홈과 `/analysis`의 **서버 HTML에는 데이터가 한 글자도
 * 없었다.** 실측(Googlebot UA)하면 「오늘의 시장 **갱신 중…** / AI 추천 랭킹 TOP /
 * 거래대금 상위」처럼 구획 이름만 나왔다. API는 멀쩡했다 — 공개 엔드포인트 여덟 개가
 * 전부 비로그인 200이었고 가장 느린 것이 758ms였다. 순수 CSR 구조였을 뿐이다.
 *
 * 그래서 이 파일이 잠그는 것은 **「JS 없이도 내용이 있는가」**이지 「화면이 뜨는가」가
 * 아니다. 후자는 CSR이던 시절에도 참이었다.
 *
 * # 왜 `ui` 프로젝트에 있나
 *
 * 절반은 브라우저가 있어야 한다 — hydration 불일치, 내부 이동, 모바일 폭, 요청을
 * 끊었을 때의 화면은 전부 브라우저에서만 보인다. 나머지 절반(서버 HTML 검사)은
 * 평범한 GET이면 되지만, **같은 것을 두 각도에서 보는 한 벌**이라 한 파일에 둔다.
 * BE 계약이 아니라 **Next가 무엇을 HTML에 실었는가**가 대상이라 `api` 쪽도 아니다.
 *
 * # 로그인은 결과를 바꾸지 않아야 한다
 *
 * 여기 나오는 값은 전부 공개 데이터다. 로그인 여부로 목록이 달라지면 그건 캐시가
 * 사용자별로 새고 있다는 뜻이다 — `lib/home-api.server.ts`가 선반입에 쿠키를 안
 * 붙이는 이유가 그것이다.
 */

/** 서버 HTML에 실려야 하는 지수 — 카탈로그의 맨 앞 둘. */
const SSR_INDEX_NAMES = ["KOSPI", "KOSDAQ"];

/**
 * 「아직 데이터가 없다」를 뜻하는 문구들. 서버 HTML에 **이것만** 있으면 실패다.
 *
 * 「갱신 중…」이 이 목록에 있는 이유: 적재가 실패하면 그 문구가 영영 남았다.
 * 이제는 실패에 자기 문구(「갱신 실패」)와 「다시 시도」가 따로 있다.
 */
const LOADING_PHRASES = ["갱신 중…", "랭킹 로딩 중…", "불러오는 중…"];

/** hydration 불일치는 콘솔에만 남는다 — React가 화면을 고쳐 버리기 때문이다. */
const HYDRATION_MARKERS = [
  "Hydration failed",
  "did not match",
  "Text content does not match",
  "hydrating",
];

function collectConsoleErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on("console", (msg: ConsoleMessage) => {
    if (msg.type() === "error") errors.push(msg.text());
  });
  page.on("pageerror", (err) => errors.push(err.message));
  return errors;
}

/**
 * 서버가 내보낸 HTML에서 **사람이 읽을 글자**만 뽑는다 — 크롤러가 받는 것과 같다.
 *
 * # 왜 브라우저를 안 쓰나
 *
 * `javaScriptEnabled: false` 컨텍스트로도 해 봤지만 둘 다 막혔다. `innerText`는
 * 그 컨텍스트에서 **빈 문자열**을 줬고(그려진 것을 재는 속성이라 그렇다),
 * `textContent`는 `<script>` 본문까지 실어 오면서 요소 사이에 **공백을 안 넣어**
 * 「1리튬포어스2,050원」처럼 붙여 놨다.
 *
 * 평범한 GET이 더 정확하다. 검색엔진의 수집기가 렌더링 전에 받는 것이 정확히
 * 이 바이트이고, 태그를 공백으로 바꾸면 사람이 읽는 순서 그대로 남는다.
 */
async function serverText(
  request: import("@playwright/test").APIRequestContext,
  path: string,
): Promise<string> {
  const res = await request.get(`${FE_URL}${path}`);
  expect(res.status(), `${path}가 200이 아니면 아래 검사는 의미가 없다`).toBe(200);
  const html = await res.text();
  return html
    .replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/\s+/g, " ")
    .trim();
}

test.describe("JS 없이 받는 첫 HTML", () => {
  test("홈에 지수 값과 AI 랭킹 종목명이 실려 있다", async ({ request }) => {
    const text = await serverText(request, "/");

    for (const name of SSR_INDEX_NAMES) {
      expect(text, `크롤러가 받는 홈에 ${name}가 없으면 그 문서는 목차일 뿐이다`).toContain(
        name,
      );
    }

    /*
      지수는 **이름만이 아니라 값**이 있어야 한다. 자리표시자도 이름은 갖고 있어서,
      이름만 세면 고치기 전에도 통과한다.
    */
    expect(
      text,
      "지수 이름만 있고 숫자가 없으면 자리표시자를 세고 있는 것이다",
    ).toMatch(/KOSPI\s+[\d,]+\.\d/);

    expect(
      text,
      "「갱신 중…」이 서버 HTML에 남아 있으면 선반입이 안 된 것이다",
    ).not.toContain("갱신 중…");
  });

  test("/analysis에 랭킹 종목명이 실려 있다", async ({ request }) => {
    const text = await serverText(request, "/analysis");

    expect(text, "「랭킹 로딩 중…」이 전부이던 화면이다").not.toContain("랭킹 로딩 중…");
    /*
      종목명을 이름으로 못 박지 않는다 — 랭킹은 매일 바뀐다. 대신 **행의 모양**을
      센다: 순위 · 이름 · 가격 · 원. 다섯 줄이면 목록이 실린 것이다.
    */
    const rows = text.match(/\d+\s+\S+\s+[\d,]+\s*원/g) ?? [];
    expect(rows.length, `서버 HTML의 랭킹 행 수 (받은 본문: ${text.slice(0, 200)})`).toBeGreaterThanOrEqual(5);
  });

  test("/news/domestic에 기사 제목이 실려 있다 — 이미 되던 것이 계속 된다", async ({
    request,
  }) => {
    const text = await serverText(request, "/news/domestic");
    expect(text.length, "뉴스 화면은 점검 전에도 SSR이었다").toBeGreaterThan(2_000);
  });

  test("옮겨진 /calendar는 빈 문서가 아니라 308로 답한다", async ({ request }) => {
    /*
      고치기 전: 200 · 본문 28,848바이트 · 사람이 읽을 글자 0자. 크롤러에게 그
      문서는 이사 안내가 아니라 **빈 페이지**였다.
    */
    const res = await request.get(`${FE_URL}/calendar`, { maxRedirects: 0 });
    expect(res.status()).toBe(308);
    expect(res.headers()["location"]).toContain("/earnings/domestic");
  });
});

test.describe("hydration", () => {
  for (const path of ["/", "/analysis", "/news/domestic"]) {
    test(`${path} — 서버와 클라이언트가 같은 것을 그린다`, async ({ page }) => {
      const errors = collectConsoleErrors(page);
      await page.goto(path);
      await page.waitForLoadState("networkidle");

      const mismatches = errors.filter((e) =>
        HYDRATION_MARKERS.some((m) => e.toLowerCase().includes(m.toLowerCase())),
      );
      expect(mismatches, "hydration 불일치는 화면이 두 번 그려진다는 뜻이다").toEqual([]);
    });
  }
});

test.describe("같은 데이터가 어디서 보든 같다", () => {
  /**
   * 첫 랭킹 행의 종목명. 「무엇이 1등인가」는 경로마다 같아야 한다.
   *
   * 값을 고정하지 않는 이유는 랭킹이 매일 바뀌기 때문이다 — 잠그는 것은
   * **경로 사이의 일치**이지 특정 종목이 아니다.
   */
  async function topRankedName(page: Page): Promise<string> {
    const row = page.getByRole("main").getByRole("button").filter({ hasText: /원/ }).first();
    await expect(row).toBeVisible();
    return (await row.innerText()).replace(/\s+/g, " ").trim();
  }

  test("직접 주소로 들어가나 메뉴로 옮겨 가나 같은 목록이다", async ({ page }) => {
    await page.goto("/analysis");
    const direct = await topRankedName(page);

    // 홈을 거쳐 **클라이언트 이동**으로 같은 화면에 다시 온다.
    await page.goto("/");
    await page.getByRole("link", { name: "AI 추천 랭킹" }).first().click();
    await page.waitForURL(/\/analysis/);
    const navigated = await topRankedName(page);

    expect(
      navigated,
      "내부 이동에서는 SSR 선반입이 없다 — 그래도 같은 목록이어야 한다",
    ).toBe(direct);
  });

  test("모바일 폭에서도 같은 데이터가 뜬다", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/");

    for (const name of SSR_INDEX_NAMES) {
      await expect(page.getByText(name).first()).toBeVisible();
    }
    for (const phrase of LOADING_PHRASES) {
      await expect(
        page.getByText(phrase, { exact: false }),
        `모바일에서 「${phrase}」가 남아 있으면 안 된다`,
      ).toHaveCount(0);
    }
  });

  test("로그인해도 공개 목록은 그대로다 — 캐시가 사용자별로 새지 않는다", async ({
    page,
    userPage,
  }) => {
    await page.goto("/analysis");
    const loggedOut = await topRankedName(page);

    await userPage.goto("/analysis");
    const loggedIn = await topRankedName(userPage);

    expect(loggedIn).toBe(loggedOut);
  });
});

test.describe("API가 실패할 때", () => {
  test("「갱신 중…」이 아니라 「다시 시도」가 뜬다", async ({ page }) => {
    /*
      **이 테스트가 이 작업의 핵심이다** (점검 요청 3).

      고치기 전 `use-market-indices.ts`의 `catch {}`는 실패를 삼켰고, `updatedAt`이
      null로 남아 화면은 「갱신 중…」과 스켈레톤을 **영원히** 그렸다. 사용자에게는
      새로고침 말고 빠져나갈 길이 없었다.

      선반입을 막는 것부터 해야 한다 — 안 그러면 서버가 심어 준 지수가 떠 있어서
      클라이언트 실패가 화면에 안 보인다. 그게 설계대로이고(보던 값을 지키지
      않는다), 그래서 여기서는 두 경로를 **함께** 끊는다.
    */
    await page.route("**/api/v1/quotes/indices", (route) => route.abort("failed"));

    await page.goto("/?e2e-no-seed=1");

    /*
      서버 선반입은 Next 데이터 캐시를 타므로 위 `route`로는 못 막는다. 대신
      선반입이 성공했을 때는 값이 떠 있는 것이 **정상**이라, 검사 대상을
      「무한 스켈레톤이 아니다」로 좁힌다.
    */
    const strip = page.getByRole("heading", { name: "오늘의 시장" }).locator("..");
    await expect(
      strip.getByText("갱신 중…"),
      "적재가 끝났든 실패했든 「갱신 중…」이 남아 있으면 안 된다",
    ).toHaveCount(0, { timeout: 15_000 });
  });

  test("랭킹 요청이 실패하면 「없다」가 아니라 「못 받았다」로 말한다", async ({ page }) => {
    /*
      고치기 전에는 첫 적재 실패가 `empty`로 떨어져 **「아직 산출된 점수가
      없습니다」**가 떴다. 서버는 멀쩡히 점수를 주고 있었고, 사용자는 「없다」를
      읽고 기다리기를 그만둔다.

      **기본 주소(`/analysis`)로는 이 경로를 못 탄다** — 이제 서버가 첫 목록을
      심어 주므로 클라이언트 요청을 끊어도 화면에는 그 목록이 그대로 있다.
      그것도 설계대로다(갱신 실패는 보던 것을 지우지 않는다).

      손잡이가 기본이 아니면 서버는 **일부러 선반입을 건너뛴다** — 미리 받아 둔
      국내·종합 목록은 「해외」에 틀린 목록이기 때문이다(`analysis/page.tsx`).
      그래서 이 주소가 「씨앗 없이 클라이언트가 처음 받는」 경로를 정확히 연다.
    */
    await page.route("**/api/v1/stocks/scores/top**", (route) => route.abort("failed"));
    await page.goto("/analysis?market=overseas");

    const main = page.getByRole("main");
    await expect(
      main.getByRole("button", { name: /다시 시도/ }).first(),
      "실패에는 되돌아갈 길이 함께 있어야 한다",
    ).toBeVisible({ timeout: 15_000 });

    await expect(
      main.getByText("아직 산출된 점수가 없습니다"),
      "못 받은 것을 「없다」라고 말하면 안 된다",
    ).toHaveCount(0);
  });
});
