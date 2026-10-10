import { describe, expect, it } from "vitest";
import { safeNext, signupHref, welcomeHref } from "@/lib/auth-routes";

describe("safeNext", () => {
  it("앱 안 경로는 그대로 돌려준다", () => {
    expect(safeNext("/watchlist")).toBe("/watchlist");
    expect(safeNext("/news/us?article=abc")).toBe("/news/us?article=abc");
  });

  it("다른 호스트로 읽히는 값은 버린다 (열린 리다이렉트)", () => {
    expect(safeNext("//evil.example")).toBeUndefined();
    expect(safeNext("/\\evil.example")).toBeUndefined();
    expect(safeNext("https://evil.example")).toBeUndefined();
    expect(safeNext("javascript:alert(1)")).toBeUndefined();
  });

  it("로그인·가입 화면 자신은 받지 않는다", () => {
    expect(safeNext("/login")).toBeUndefined();
    expect(safeNext("/signup?next=/x")).toBeUndefined();
  });

  it("비었으면 없다", () => {
    expect(safeNext(null)).toBeUndefined();
    expect(safeNext("")).toBeUndefined();
  });
});

describe("signupHref · welcomeHref", () => {
  it("보던 곳을 실어 보낸다", () => {
    expect(signupHref("/watchlist")).toBe("/signup?next=%2Fwatchlist");
    expect(welcomeHref("/watchlist")).toBe("/welcome?next=%2Fwatchlist");
  });

  it("믿을 수 없거나 홈이면 싣지 않는다", () => {
    expect(signupHref("//evil.example")).toBe("/signup");
    expect(welcomeHref("/")).toBe("/welcome");
    expect(welcomeHref(undefined)).toBe("/welcome");
  });
});
