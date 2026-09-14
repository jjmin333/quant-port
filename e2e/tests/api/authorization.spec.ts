import { SYMBOLS } from "@/data";
import { expect, hasAdminCredentials, test } from "@/fixtures";
import type { AuthSession } from "@/types";

/**
 * 인가 경계 (INV-A2, A3, B군).
 *
 * 여기서 검증하는 것은 상태코드 하나가 아니라 **경계의 종류**다.
 *  - 401 = 너가 누군지 모른다
 *  - 403 = 누군지는 알지만 자격이 없다
 *  - 404 = 네 것이 아니다 (존재 여부를 알려주지 않는다)
 * 셋이 뒤섞이면 클라이언트는 재로그인해야 할 때 권한 안내를 띄우게 된다.
 */

/** 대표 보호 경로 — 각 도메인에서 하나씩. 전수가 아니라 미들웨어 부착 확인이 목적. */
const PROTECTED: Array<[method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE", path: string]> = [
  ["GET", "/auth/me"],
  ["GET", "/watchlist"],
  ["POST", "/watchlist"],
  ["GET", "/journal/notes"],
  ["GET", "/user/price-targets"],
  ["GET", "/stamps"],
  ["POST", "/stamps"],
  ["GET", "/user/holdings"],
  ["GET", "/user/holdings/trades"],
  ["POST", "/user/holdings/trades"],
  ["GET", "/user/cash-balances"],
  ["GET", "/research/mine"],
  ["POST", "/research/calls"],
  ["POST", "/media/images"],
  ["GET", "/import/holdings/jobs/any-id"],
];

/** 관리자 전용 경로 — 읽기만 골랐다. 부작용 없이 경계만 확인하기 위해. */
const ADMIN_ONLY = [
  "/admin/news/stats",
  "/admin/api-quotas",
  "/admin/llm-settings",
  "/admin/batch/jobs",
  "/admin/score/settings",
  "/admin/cache/namespaces",
  "/admin/db/tables",
  "/admin/research/calls",
];

test.describe("인증 경계", () => {
  for (const [method, path] of PROTECTED) {
    test(`비로그인은 ${method} ${path} 에서 401을 받는다`, async ({ anon }) => {
      await anon.expectFailure(method, path, 401);
    });
  }

  test("공개 경로는 비로그인도 200을 받는다", async ({ anon }) => {
    // 반대 방향 확인 — 인증 미들웨어를 넓게 붙여 공개 화면까지 잠그는 회귀를 잡는다.
    for (const path of [
      "/quotes/indices",
      // 랭킹은 type이 필수다 — 없으면 400이고, 그건 인증 문제가 아니다.
      "/rankings?type=TOP_GAINERS&marketCountry=KR&count=5",
      "/research/calls",
      "/sample/holdings",
    ]) {
      const res = await anon.send("GET", path);
      expect(res.status(), `${path} 는 비로그인도 열려 있어야 한다`).toBe(200);
    }
  });
});

test.describe("관리자 경계", () => {
  for (const path of ADMIN_ONLY) {
    test(`일반 사용자는 ${path} 에서 403을 받는다`, async ({ user }) => {
      const message = await user.api.expectFailure("GET", path, 403);
      expect(
        message,
        "권한 부족을 401로 내면 클라이언트가 멀쩡한 세션을 버리고 재로그인시킨다",
      ).toContain("admin");
    });
  }

  test("일반 사용자는 리서치 콜을 숨길 수 없다", async ({ user }) => {
    await user.api.expectFailure("PUT", "/admin/research/calls/1/hidden", 403, { hidden: true });
  });

  test("일반 사용자는 배치를 실행할 수 없다", async ({ user }) => {
    await user.api.expectFailure("POST", "/admin/batch/run?job=stock_scores", 403);
  });

  test("비로그인은 관리자 경로에서 403이 아니라 401을 받는다", async ({ anon }) => {
    await anon.expectFailure("GET", "/admin/news/stats", 401);
  });

  test("관리자 계정은 같은 경로에서 200을 받는다", async ({ adminApi }) => {
    test.skip(
      !hasAdminCredentials,
      "E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD 가 없으면 건너뛴다 (테스트는 관리자 계정을 만들지 않는다)",
    );
    const me = await adminApi!.get<AuthSession>("/auth/me");
    expect(me.user.isAdmin, "관리자 판정 필드가 꺼져 있으면 FE 메뉴도 숨는다").toBe(true);

    for (const path of ADMIN_ONLY) {
      const res = await adminApi!.send("GET", path);
      expect(res.status(), `관리자가 ${path} 에서 막히면 운영이 멈춘다`).toBe(200);
    }
  });
});

test.describe("소유권 경계", () => {
  test("남의 관심종목·메모·목표가는 보이지도 지워지지도 않는다", async ({ user, otherUser }) => {
    await user.api.post("/watchlist", { symbol: "MSFT" });
    const note = await user.api.post<{ id: number }>("/journal/notes", {
      title: "owner only",
      body: "body",
      symbol: "MSFT",
    });
    const target = await user.api.post<{ id: number }>("/user/price-targets", {
      symbol: "MSFT",
      targetPrice: 600,
    });

    // B의 목록에는 A의 것이 하나도 없다.
    expect(await otherUser.api.get<unknown[]>("/watchlist")).toHaveLength(0);
    expect(await otherUser.api.get<unknown[]>("/journal/notes")).toHaveLength(0);
    expect(await otherUser.api.get<unknown[]>("/user/price-targets")).toHaveLength(0);

    // B가 id를 알아내도 지우지 못한다. 그리고 A의 것은 그대로 남아 있어야 한다.
    await otherUser.api.send("DELETE", `/journal/notes/${note.id}`);
    await otherUser.api.send("DELETE", `/user/price-targets/${target.id}`);
    await otherUser.api.send("DELETE", "/watchlist/MSFT");

    expect(
      await user.api.get<unknown[]>("/journal/notes"),
      "B의 삭제 요청이 A의 메모를 지웠다",
    ).toHaveLength(1);
    expect(
      await user.api.get<unknown[]>("/user/price-targets"),
      "B의 삭제 요청이 A의 목표가를 지웠다",
    ).toHaveLength(1);
    expect(
      await user.api.get<unknown[]>("/watchlist"),
      "B의 삭제 요청이 A의 관심종목을 지웠다",
    ).toHaveLength(1);
  });

  test("남의 보유·매매 이력은 목록에 섞이지 않는다", async ({ user, otherUser }) => {
    await user.api.post("/user/holdings/trades", {
      symbol: "NVDA",
      side: "buy",
      quantity: 3,
      price: 100,
      currency: "USD",
      tradeDate: "2026-02-02",
    });

    expect(await otherUser.api.get<unknown[]>("/user/holdings")).toHaveLength(0);
    expect(await otherUser.api.get<unknown[]>("/user/holdings/trades")).toHaveLength(0);
  });

  test("남의 스크린샷 임포트 잡은 존재조차 확인되지 않는다", async ({ otherUser }) => {
    // 잡 ID를 알아도 소유자가 아니면 404 — "있는데 권한 없음"이 아니라 "없음".
    await otherUser.api.expectFailure("GET", "/import/holdings/jobs/00000000000000000000000000000000", 404);
  });
});

/*
리서치 댓글 소유권.

권한 자체는 처음부터 막혀 있었다 — 리포지터리가 `user_id`까지 걸어 UPDATE 하므로
남의 댓글은 한 줄도 바뀌지 않는다. 문제는 **그 거부를 502로 알린 것**이다
(실측 2026-09-06). 평범한 권한 거부가 게이트웨이 장애로 보고되면 5xx 알람이
사용자 행동에 울리고, 클라이언트는 「다시 시도할 문제」와 「하면 안 되는 일」을
구분할 수 없다. 이 앱은 같은 결함을 검증 실패 두 곳에서 이미 한 번 고쳤다
(`contract.spec.ts`).

그래서 **상태코드와 결과를 함께** 본다 — 하나만 보면 다음에 또 놓친다.
*/
test.describe("리서치 댓글 소유권", () => {
  test("남의 댓글은 4xx로 거부되고 원문이 그대로 남는다", async ({ user, otherUser }) => {
    const call = await user.api.post<{ id: number }>("/research/calls", {
      symbol: SYMBOLS.trade,
      title: "소유권 검증",
      direction: "buy",
      horizonDays: 90,
      conviction: 60,
      bodyMd: "본문",
      visibility: "public",
    });
    const comment = await user.api.post<{ id: number }>(`/research/calls/${call.id}/comments`, {
      body: "주인만 고칠 수 있다",
    });

    for (const [method, path] of [
      ["PUT", `/research/comments/${comment.id}`],
      ["DELETE", `/research/comments/${comment.id}`],
    ] as const) {
      const res = await otherUser.api.send(method, path, { body: "남이 덮어씀" });
      expect(res.ok(), `${method} ${path} 가 남에게 허용되면 안 된다`).toBe(false);
      expect(
        res.status(),
        `권한 거부는 4xx다 — 5xx면 알람이 사용자 행동에 울린다 (실제 ${res.status()})`,
      ).toBeLessThan(500);
    }

    // 상태코드와 무관한 진짜 불변식 — 원문이 그대로다.
    const after = await user.api.get<Array<{ id: number; body: string }>>(
      `/research/calls/${call.id}/comments`,
    );
    const mine = after.find((c) => c.id === comment.id);
    expect(mine?.body, "남의 요청이 내 댓글 본문을 바꿨다").toBe("주인만 고칠 수 있다");

    // 콜은 삭제할 수 없다 — 비공개로 되돌려 정리한다.
    await user.api.patch(`/research/calls/${call.id}`, { visibility: "private" });
  });
});
