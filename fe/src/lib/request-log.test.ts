import { describe, expect, it } from "vitest";
import {
  accessTokenDisplayName,
  readAccessTokenClaims,
} from "@/lib/access-token-claims";
import {
  formatKstTimestamp,
  formatRequestLog,
  shouldLogRequest,
} from "@/lib/request-log";

/** 테스트용 JWT 조립 — 서명은 쓰지 않으므로 자리만 채운다. */
function jwt(payload: Record<string, unknown>): string {
  const b64 = Buffer.from(JSON.stringify(payload), "utf8")
    .toString("base64")
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
  return `header.${b64}.signature`;
}

describe("formatKstTimestamp", () => {
  it("UTC를 KST(+09:00)로 옮겨 밀리초까지 찍는다", () => {
    expect(formatKstTimestamp(new Date("2026-09-03T14:10:22.418Z"))).toBe(
      "2026-09-03 23:10:22.418",
    );
  });

  it("자정을 넘기면 날짜도 함께 넘어간다", () => {
    expect(formatKstTimestamp(new Date("2026-09-03T15:30:00.000Z"))).toBe(
      "2026-09-04 00:30:00.000",
    );
  });
});

describe("shouldLogRequest", () => {
  it("사람이 연 화면은 남긴다", () => {
    expect(shouldLogRequest("/news/domestic")).toBe(true);
    expect(shouldLogRequest("/api/v1/rankings")).toBe(true);
  });

  it("빌드 산출물·HMR은 뺀다 — 한 번의 진입이 수십 줄을 만든다", () => {
    expect(shouldLogRequest("/_next/static/chunks/main.js")).toBe(false);
    expect(shouldLogRequest("/favicon.ico")).toBe(false);
  });
});

describe("readAccessTokenClaims", () => {
  it("sub·name·email을 꺼낸다", () => {
    const claims = readAccessTokenClaims(
      jwt({ sub: "42", name: "홍길동", email: "hong@example.com" }),
    );
    expect(claims).toEqual({ userId: "42", name: "홍길동", email: "hong@example.com" });
  });

  it("한글 이름이 깨지지 않는다", () => {
    expect(readAccessTokenClaims(jwt({ sub: "7", name: "김이박최" })).name).toBe("김이박최");
  });

  it("쿠키가 없거나 형태가 아니면 빈 값이다 — 프록시가 던지면 앱 전체가 죽는다", () => {
    const empty = { userId: "", name: "", email: "" };
    expect(readAccessTokenClaims(undefined)).toEqual(empty);
    expect(readAccessTokenClaims("")).toEqual(empty);
    expect(readAccessTokenClaims("not-a-jwt")).toEqual(empty);
    expect(readAccessTokenClaims("a.!!!not-base64!!!.c")).toEqual(empty);
    expect(readAccessTokenClaims(`header.${Buffer.from("[]").toString("base64")}.sig`)).toEqual(
      empty,
    );
  });
});

describe("accessTokenDisplayName", () => {
  it("이름이 있으면 이름", () => {
    expect(
      accessTokenDisplayName({ userId: "1", name: "홍길동", email: "hong@example.com" }),
    ).toBe("홍길동");
  });

  it("name 칸이 생기기 전 토큰은 이메일 로컬 파트로 버틴다", () => {
    expect(accessTokenDisplayName({ userId: "1", name: "", email: "hong@example.com" })).toBe(
      "hong",
    );
  });

  it("아무것도 없으면 빈 값", () => {
    expect(accessTokenDisplayName({ userId: "", name: "", email: "" })).toBe("");
  });
});

describe("formatRequestLog", () => {
  const at = new Date("2026-09-03T14:10:22.418Z");

  it("KST 시각 · 사용자 · 메서드 · 경로", () => {
    expect(
      formatRequestLog({
        at,
        method: "GET",
        pathname: "/news/domestic",
        user: { id: "42", name: "홍길동" },
      }),
    ).toBe("[2026-09-03 23:10:22.418 KST] user=42 name=홍길동 GET /news/domestic");
  });

  it("비로그인은 두 칸을 비운 채로 찍는다", () => {
    expect(
      formatRequestLog({
        at,
        method: "GET",
        pathname: "/news/domestic",
        user: { id: "", name: "" },
      }),
    ).toBe("[2026-09-03 23:10:22.418 KST] user= name= GET /news/domestic");
  });

  it("쿼리가 있으면 붙이고, 빈 물음표는 버린다", () => {
    const user = { id: "1", name: "김" };
    expect(
      formatRequestLog({ at, method: "GET", pathname: "/market", search: "?market=US", user }),
    ).toContain("GET /market?market=US");
    expect(formatRequestLog({ at, method: "GET", pathname: "/market", search: "?", user })).toContain(
      "GET /market",
    );
  });
});
