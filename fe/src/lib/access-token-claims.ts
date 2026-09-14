/*
액세스 토큰(`qp_access` 쿠키)에서 **누구인지만** 꺼낸다.

# 왜 검증하지 않나

여기서 하는 일은 요청 로그에 사람 이름을 적는 것뿐이고, FE에는 서명 검증에 쓸 비밀키가
없다(있어서도 안 된다). 위조된 토큰이 로그에 엉뚱한 이름을 남길 수는 있지만, **권한은
이 값으로 결정되지 않는다** — 모든 인가는 BE가 서명을 검증한 뒤에 한다.

그래서 이 함수의 반환값을 인가·분기에 쓰면 안 된다. 로그와 표시에만 쓴다.

# 왜 직접 디코딩하나

프록시는 요청마다 도는 자리다(fe/src/proxy.ts). 이름 하나 때문에 `/auth/session`을
왕복하면 페이지 요청 수만큼 API 호출이 붙는다. 토큰이 이미 쿠키로 들어와 있으므로
가운데 조각만 읽으면 된다.
*/

export type AccessTokenClaims = {
  /** JWT `sub` — BE가 문자열로 넣은 사용자 ID. 못 읽으면 빈 문자열. */
  userId: string;
  /** JWT `name` — 표시 이름. 이 칸이 생기기 전 토큰에는 없다. */
  name: string;
  email: string;
};

const EMPTY: AccessTokenClaims = { userId: "", name: "", email: "" };

/** base64url → UTF-8 문자열. 한글 이름이 들어오므로 바이트를 거쳐 디코딩한다. */
function decodeSegment(segment: string): string | null {
  const base64 = segment.replace(/-/g, "+").replace(/_/g, "/");
  const padded = base64 + "=".repeat((4 - (base64.length % 4)) % 4);
  try {
    const binary = atob(padded);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
    return new TextDecoder("utf-8").decode(bytes);
  } catch {
    /*
      `atob`는 base64가 아닌 글자를 만나면 던진다. 쿠키는 사용자가 보내는 값이라
      아무 문자열이나 올 수 있고, 그것 때문에 프록시가 죽으면 **앱 전체가 죽는다**.
    */
    return null;
  }
}

export function readAccessTokenClaims(token: string | undefined | null): AccessTokenClaims {
  const raw = token?.trim();
  if (!raw) return EMPTY;

  const parts = raw.split(".");
  if (parts.length !== 3) return EMPTY;

  const json = decodeSegment(parts[1]);
  if (!json) return EMPTY;

  let payload: unknown;
  try {
    payload = JSON.parse(json);
  } catch {
    return EMPTY;
  }
  if (!payload || typeof payload !== "object") return EMPTY;

  const claims = payload as Record<string, unknown>;
  return {
    userId: asText(claims.sub),
    name: asText(claims.name),
    email: asText(claims.email),
  };
}

function asText(value: unknown): string {
  if (typeof value === "string") return value.trim();
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  return "";
}

/**
 * 로그에 적을 사용자명.
 *
 * 이름이 없는 옛 토큰에서는 이메일의 **로컬 파트**를 쓴다 — 주소를 통째로 남기면
 * 로그가 곧 이메일 목록이 된다. 토큰이 한 번 갱신되면(액세스 15분) 진짜 이름이 온다.
 */
export function accessTokenDisplayName(claims: AccessTokenClaims): string {
  if (claims.name) return claims.name;
  if (!claims.email) return "";
  const at = claims.email.indexOf("@");
  return at > 0 ? claims.email.slice(0, at) : claims.email;
}
