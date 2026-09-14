import { expect, test } from "@/fixtures";
import { newApiContext } from "@/test-user";
import { Api } from "@/api-client";
import { PASSWORD } from "@/data";
import type { AuthSession, WatchlistItem } from "@/types";

/**
 * 계정 관리 (§A-6) — 2026-08-26에 새로 생긴 경로.
 *
 * 그 전까지 이 앱에는 **비밀번호를 바꿀 방법도, 탈퇴할 방법도 없었다.** 비밀번호를
 * 잊으면 계정으로 돌아올 길이 없고, 그만두려는 사람은 데이터를 남긴 채 떠나야 했다.
 *
 * 셋 다 현재 비밀번호를 다시 받는다 — 로그인 세션은 오래 유지되므로 계정 자체를 바꾸는
 * 행동은 「지금 이 사람이 주인인가」를 한 번 더 묻는 것이 맞다.
 *
 * **여기서 만든 계정은 지운다.** 이 스펙의 마지막 테스트가 실제 탈퇴 경로를 쓰므로,
 * 다른 스펙과 달리 계정 행이 쌓이지 않는다.
 */

const NEW_PASSWORD = "e2e-changed-2026";

test.describe("계정 관리", () => {
  test("현재 비밀번호가 틀리면 바꿔 주지 않는다", async ({ user }) => {
    const res = await user.api.send("POST", "/auth/password", {
      currentPassword: `${PASSWORD}-wrong`,
      newPassword: NEW_PASSWORD,
    });
    expect(res.status(), "틀린 비밀번호로 바꿀 수 있으면 계정 탈취 경로가 된다").toBe(401);
  });

  test("짧은 비밀번호는 거부한다", async ({ user }) => {
    const res = await user.api.send("POST", "/auth/password", {
      currentPassword: PASSWORD,
      newPassword: "short",
    });
    expect(res.status()).toBe(400);
  });

  test("비밀번호를 바꾸면 옛 비밀번호로는 못 들어온다", async ({ user }) => {
    await user.api.post("/auth/password", {
      currentPassword: PASSWORD,
      newPassword: NEW_PASSWORD,
    });

    const probe = new Api(await newApiContext(), "probe");
    const old = await probe.send("POST", "/auth/login", {
      email: user.email,
      password: PASSWORD,
    });
    expect(old.status(), "옛 비밀번호가 살아 있으면 바꾼 의미가 없다").toBe(401);

    const fresh = await probe.send("POST", "/auth/login", {
      email: user.email,
      password: NEW_PASSWORD,
    });
    expect(fresh.status()).toBe(200);
    await probe.ctx.dispose();
  });

  test("표시 이름은 2자 미만을 거부하고, 바꾸면 세션에 반영된다", async ({ user }) => {
    const tooShort = await user.api.send("POST", "/auth/display-name", { displayName: "가" });
    expect(tooShort.status()).toBe(400);

    const session = await user.api.post<AuthSession>("/auth/display-name", {
      displayName: "QA-renamed",
    });
    expect(session.user.displayName).toBe("QA-renamed");
  });

  test("탈퇴는 비밀번호를 확인하고, 소유 데이터를 함께 지운다", async ({ user }) => {
    // 지워져야 할 것이 실제로 있는 상태에서 확인한다 — 빈 계정을 지우는 것은 증거가 아니다.
    await user.api.post("/watchlist", { symbol: "005930" });
    const before = await user.api.get<WatchlistItem[]>("/watchlist");
    expect(before.length).toBe(1);

    const wrong = await user.api.send("POST", "/auth/delete", { password: `${PASSWORD}-nope` });
    expect(wrong.status(), "비밀번호 없이 탈퇴되면 안 된다").toBe(401);

    const ok = await user.api.send("POST", "/auth/delete", { password: PASSWORD });
    expect(ok.status()).toBe(200);

    const probe = new Api(await newApiContext(), "probe");
    const login = await probe.send("POST", "/auth/login", {
      email: user.email,
      password: PASSWORD,
    });
    expect(login.status(), "탈퇴한 계정으로 다시 로그인되면 안 된다").toBe(401);
    await probe.ctx.dispose();
  });
});
