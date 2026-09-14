import { describe, expect, it } from "vitest";

import { PortfolioFetchError } from "@/lib/api-client";
import { isSignedOutError } from "@/lib/auth-error";

/*
「로그인하지 않았다」와 「물어봤는데 답을 못 받았다」를 가른다.

이 구분이 없을 때 실제로 일어난 일(2026-09-06 실측): `/auth/*` 가 IP 레이트
리밋에 걸려 `/auth/me` 가 429를 주자, 로그인한 사용자의 헤더가 「로그인 /
회원가입」으로 바뀌고 보유 데이터가 화면에서 사라졌다. 쿠키는 살아 있어서
새로고침하면 돌아왔다 — 즉 서버는 그를 쫓아낸 적이 없다.
*/
describe("isSignedOutError", () => {
  it("401·403만 로그아웃으로 본다 — 서버가 아니라고 말한 경우다", () => {
    expect(isSignedOutError(new PortfolioFetchError("unauthorized", 401))).toBe(true);
    expect(isSignedOutError(new PortfolioFetchError("forbidden", 403))).toBe(true);
  });

  it("429는 로그아웃이 아니다 — 잠시 뒤 다시 물으면 될 일이다", () => {
    expect(isSignedOutError(new PortfolioFetchError("too many requests", 429))).toBe(false);
  });

  it("5xx는 로그아웃이 아니다 — 서버 사정으로 사용자를 쫓아내면 안 된다", () => {
    for (const status of [500, 502, 503, 504]) {
      expect(
        isSignedOutError(new PortfolioFetchError("server error", status)),
        `${status} 에서 세션을 비우면 서버가 흔들릴 때마다 로그아웃된다`,
      ).toBe(false);
    }
  });

  it("상태 코드가 없는 오류(네트워크 단절)는 로그아웃이 아니다", () => {
    expect(isSignedOutError(new TypeError("Failed to fetch"))).toBe(false);
    expect(isSignedOutError(new Error("boom"))).toBe(false);
    expect(isSignedOutError(undefined)).toBe(false);
    expect(isSignedOutError(null)).toBe(false);
  });
});
