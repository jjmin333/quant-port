import { Api } from "@/api-client";
import { ACCESS_COOKIE, REFRESH_COOKIE, cookieNamed, getWithBearer, refreshWithToken } from "@/cookies";
import { PASSWORD, randomHangul, uniqueDisplayName, uniqueLoginId } from "@/data";
import { expect, test } from "@/fixtures";
import { newApiContext } from "@/test-user";
import type { AuthSession } from "@/types";

/**
 * 인증/세션 (INV-A군).
 *
 * 이 파일이 검증하는 것은 "로그인 버튼이 눌리는가"가 아니라 **세션이 정확히
 * 언제 유효하고 언제 무효한가**다. 토큰 회전과 로그아웃이 여기서 깨지면 다른
 * 어떤 테스트도 의미가 없다.
 */

test.describe("가입 · 로그인", () => {
  test("가입하면 201과 함께 두 쿠키가 심기고, 기본 권한은 일반 사용자다", async () => {
    const ctx = await newApiContext();
    const api = new Api(ctx, "signup");
    const email = uniqueLoginId("signup");

    const res = await api.send("POST", "/auth/signup", {
      loginId: email,
      password: PASSWORD,
      displayName: uniqueDisplayName("signup"),
    });

    expect(res.status(), "가입은 201 Created여야 한다").toBe(201);

    const session = (await res.json()).data as AuthSession;
    expect(session.user.email).toBe(email);
    expect(session.user.role, "가입 기본 권한은 user여야 한다").toBe("user");
    expect(
      session.user.isAdmin,
      "새 계정이 관리자로 만들어지면 관리자 경계가 무너진다",
    ).toBe(false);

    const access = await cookieNamed(ctx, ACCESS_COOKIE);
    const refresh = await cookieNamed(ctx, REFRESH_COOKIE);

    expect(access, `${ACCESS_COOKIE} 쿠키가 없다`).toBeDefined();
    expect(refresh, `${REFRESH_COOKIE} 쿠키가 없다`).toBeDefined();
    expect(access!.httpOnly, "access 쿠키는 JS에서 읽히면 안 된다").toBe(true);
    expect(refresh!.httpOnly, "refresh 쿠키는 JS에서 읽히면 안 된다").toBe(true);
    expect(access!.path).toBe("/");
    expect(
      refresh!.path,
      "refresh 쿠키는 /api/v1/auth 밖으로 새지 않아야 한다",
    ).toBe("/api/v1/auth");

    // 가입 응답이 곧 세션이라는 것을 상태로 확인한다 — 쿠키만 있고 세션이
    // 아니면 다음 요청에서 401이 난다.
    const me = await api.get<AuthSession>("/auth/me");
    expect(me.user.id).toBe(session.user.id);

    await api.send("POST", "/auth/logout");
    await ctx.dispose();
  });

  test("잘못된 비밀번호와 없는 계정은 구분되지 않는 401을 낸다", async ({ anon, user }) => {
    const wrongPassword = await anon.send("POST", "/auth/login", {
      email: user.email,
      password: `${PASSWORD}-wrong`,
    });
    const unknownAccount = await anon.send("POST", "/auth/login", {
      email: uniqueLoginId("nobody"),
      password: PASSWORD,
    });

    expect(wrongPassword.status()).toBe(401);
    expect(unknownAccount.status()).toBe(401);
    expect(
      (await unknownAccount.json()).error,
      "메시지가 다르면 계정 존재 여부를 열거할 수 있다",
    ).toBe((await wrongPassword.json()).error);
  });

  test("8~15자 밖의 비밀번호는 계정을 만들지 않는다 (2026-09-27 규칙)", async ({ anon }) => {
    for (const password of ["short", "a".repeat(16)]) {
      const loginId = uniqueLoginId("weak");
      const message = await anon.expectFailure("POST", "/auth/signup", 400, {
        loginId,
        password,
        displayName: uniqueDisplayName(),
      });
      expect(message).toContain("8-15");
      // 거부됐다면 그 아이디로 로그인도 안 돼야 한다 — 계정이 안 생겼다는 증거.
      await anon.expectFailure("POST", "/auth/login", 401, { loginId, password });
    }
  });

  test("같은 아이디로 두 번 가입할 수 없다", async ({ anon, user }) => {
    await anon.expectFailure("POST", "/auth/signup", 409, {
      loginId: user.email,
      password: PASSWORD,
      displayName: uniqueDisplayName(),
    });
  });

  test("같은 닉네임으로 두 번 가입할 수 없다 — 닉네임은 겹치지 않는다", async ({ anon, user }) => {
    const message = await anon.expectFailure("POST", "/auth/signup", 409, {
      loginId: uniqueLoginId(),
      password: PASSWORD,
      displayName: user.displayName,
    });
    expect(message).toContain("display name");
  });

  test("아이디는 영문 소문자로 시작하는 소문자·숫자 4~20자다", async ({ anon }) => {
    for (const bad of ["abc", "Abcdef", "1abcdef", "ab_cdef", "user@mail.com", "a".repeat(21)]) {
      const res = await anon.send("POST", "/auth/signup", {
        loginId: bad,
        password: PASSWORD,
        displayName: uniqueDisplayName(),
      });
      expect(res.status(), `아이디 ${bad} → ${res.status()} (기대 400)`).toBe(400);
    }
  });

  test("옛 화면이 보내는 email 키도 아이디로 받는다 — 기존 회원은 이메일이 곧 아이디", async ({ anon, user }) => {
    const res = await anon.send("POST", "/auth/login", { email: user.email, password: user.password });
    expect(res.status()).toBe(200);
    await anon.send("POST", "/auth/logout");
  });

  /*
   * 닉네임 길이는 **글자 수**로 잰다 — 인코딩과 무관해야 한다 (2026-08-23 고친 결함의 회귀 시험).
   * 한때 상한을 바이트로 재서 한글 닉네임의 실제 상한이 1/3이었다.
   * 2026-09-27부터 범위는 2~10자이고 중복 불가라 이름은 매번 난수로 만든다.
   */
  test("닉네임 길이는 바이트가 아니라 글자 수로 잰다", async ({ anon }) => {
    for (const [label, name] of [
      ["ASCII 10자", `q${uniqueDisplayName().slice(3)}`.padEnd(10, "x").slice(0, 10)],
      ["한글 10자", randomHangul(10)],
    ] as const) {
      const res = await anon.send("POST", "/auth/signup", {
        loginId: uniqueLoginId("dn-ok"),
        password: PASSWORD,
        displayName: name,
      });
      expect(
        res.status(),
        `${label}(${Buffer.byteLength(name)}B)가 거부됐다 — 상한이 다시 바이트로 돌아간 것`,
      ).toBe(201);
      expect(
        ((await res.json()) as { data: AuthSession }).data.user.displayName,
        `${label}가 잘려서 저장됐다`,
      ).toBe(name);
    }
  });

  test("닉네임이 DB 왕복 후에도 잘리지 않는다", async ({ anon }) => {
    const name = randomHangul(10);
    const loginId = uniqueLoginId("dn-roundtrip");
    await anon.post<AuthSession>("/auth/signup", { loginId, password: PASSWORD, displayName: name });

    const me = await anon.get<AuthSession>("/auth/me");
    expect(
      me.user.displayName,
      `저장된 닉네임이 ${[...me.user.displayName].length}자로 잘렸다 (기대 10자)`,
    ).toBe(name);

    await anon.send("POST", "/auth/logout");
  });

  test("닉네임 경계는 2자 이상 10자 이하다", async ({ anon }) => {
    const cases: Array<[label: string, name: string, status: number]> = [
      ["한글 1자", randomHangul(1), 400],
      ["한글 2자", randomHangul(2), 201],
      ["한글 10자", randomHangul(10), 201],
      ["한글 11자", randomHangul(11), 400],
      ["ASCII 11자", "a".repeat(11), 400],
    ];
    for (const [label, name, expected] of cases) {
      const res = await anon.send("POST", "/auth/signup", {
        loginId: uniqueLoginId("dn-edge"),
        password: PASSWORD,
        displayName: name,
      });
      expect(res.status(), `${label} → ${res.status()} (기대 ${expected})`).toBe(expected);
    }
  });
});

test.describe("토큰 회전", () => {
  test("refresh 토큰은 1회용이다 — 재사용하면 세션이 서지 않는다", async ({ user }) => {
    const original = await cookieNamed(user.ctx, REFRESH_COOKIE);
    expect(original, "가입 직후 refresh 쿠키가 있어야 한다").toBeDefined();

    const first = await refreshWithToken(original!.value);
    expect(first.status, `첫 사용은 성공해야 한다: ${first.body.slice(0, 200)}`).toBe(200);
    const rotated = await cookieNamed(first.context, REFRESH_COOKIE);
    expect(rotated!.value, "회전이 일어나면 값이 바뀌어야 한다").not.toBe(original!.value);
    await first.context.dispose();

    const replay = await refreshWithToken(original!.value);
    expect(
      replay.status,
      "이미 쓴 refresh 토큰이 다시 통하면 탈취된 토큰이 영구 세션이 된다",
    ).toBe(401);

    // 상태로도 확인한다: 재사용 시도가 만든 컨텍스트에는 쓸 수 있는 세션이 없다.
    const stale = new Api(replay.context, "replayed-refresh");
    await stale.expectFailure("GET", "/auth/me", 401);
    await replay.context.dispose();
  });

  test("회전해도 세션의 절대 만료 시각은 밀리지 않는다", async ({ user }) => {
    const before = await cookieNamed(user.ctx, REFRESH_COOKIE);
    const rotate = await refreshWithToken(before!.value);
    expect(rotate.status).toBe(200);
    const after = await cookieNamed(rotate.context, REFRESH_COOKIE);
    await rotate.context.dispose();

    expect(after, "회전 후에도 refresh 쿠키가 있어야 한다").toBeDefined();

    const driftSeconds = Math.abs(after!.expires - before!.expires);
    expect(
      driftSeconds,
      `회전이 만료를 연장하면 세션이 영원히 살아난다 ` +
        `(before=${before!.expires}, after=${after!.expires}, drift=${driftSeconds}s)`,
    ).toBeLessThan(5);
  });
});

test.describe("로그아웃", () => {
  test("로그아웃하면 그 계정의 refresh 토큰이 전부 무효가 된다", async ({ user }) => {
    // 로그인 하나를 더 만들어 "다른 기기"를 흉내낸다.
    const secondDevice = await newApiContext();
    const secondApi = new Api(secondDevice, "device-2");
    await secondApi.post<AuthSession>("/auth/login", {
      email: user.email,
      password: user.password,
    });
    const secondRefresh = await cookieNamed(secondDevice, REFRESH_COOKIE);
    expect(secondRefresh).toBeDefined();

    await user.api.post("/auth/logout");

    // 로그아웃한 세션은 물론이고,
    await user.api.expectFailure("GET", "/auth/me", 401);
    // 다른 기기의 refresh 토큰까지 죽어야 한다 (RevokeAllForUser).
    const replay = await refreshWithToken(secondRefresh!.value);
    expect(
      replay.status,
      "한 기기의 로그아웃이 다른 기기 세션을 남겨 두면 '전체 로그아웃'이 거짓말이 된다",
    ).toBe(401);

    await replay.context.dispose();
    await secondDevice.dispose();
  });
});

test.describe("토큰 위조", () => {
  const forged = [
    {
      name: "alg=none",
      // {"alg":"none","typ":"JWT"} . {"sub":"1","exp":9999999999} . (빈 서명)
      token:
        "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
        "eyJzdWIiOiIxIiwiZXhwIjo5OTk5OTk5OTk5fQ.",
    },
    { name: "서명 위조", token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxIn0.bogus" },
    { name: "JWT가 아닌 문자열", token: "not-a-token" },
  ];

  for (const { name, token } of forged) {
    test(`${name} 토큰으로는 보호 경로에 들어갈 수 없다`, async () => {
      const status = await getWithBearer("/auth/me", token);
      expect(status, `${name} 토큰이 통과하면 인증이 없는 것과 같다`).toBe(401);
    });
  }
});
