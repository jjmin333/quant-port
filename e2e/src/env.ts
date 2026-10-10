/**
 * 테스트가 무엇을 향해 도는지 한곳에서 정한다.
 *
 * BE와 FE 주소를 나눠 두는 이유: **API 계층 테스트는 BE를 직접 친다**. 브라우저
 * 경로(`:3000`)는 Next rewrite를 한 겹 더 거치는데, 그 겹이 검증 대상인 계약을
 * 가릴 수 있다(예: `X-Forwarded-For` 전달, 압축, 상태코드 변환). UI 테스트만
 * `:3000`을 쓰고, 그때는 rewrite를 거치는 것 자체가 검증 대상이다.
 */

function required(name: string, fallback: string): string {
  const raw = (process.env[name] ?? "").trim();
  return raw === "" ? fallback : raw.replace(/\/+$/, "");
}

/** BE API 오리진. API 계층 테스트가 직접 친다. */
export const API_URL = required("E2E_API_URL", "http://localhost:8080");

/** FE 오리진. 브라우저 테스트의 baseURL. */
export const FE_URL = required("E2E_FE_URL", "http://localhost:3000");

export const API_PREFIX = "/api/v1";

/** `${API_URL}/api/v1/...` 를 만든다. 테스트 본문에서 문자열을 이어 붙이지 않기 위해. */
export function apiPath(path: string): string {
  return `${API_PREFIX}${path.startsWith("/") ? path : `/${path}`}`;
}

/**
 * 관리자 계정은 **환경변수로만** 받는다.
 *
 * BE는 `model.DefaultAdminEmail`과 일치하는 이메일로 가입하면 그 계정을 관리자로
 * 만든다(`repository.UserRepo.Create`). 테스트가 그 이메일로 가입을 시도하면
 * 실제 관리자 계정을 만들어 버리거나 남의 계정을 건드리게 된다 — 그래서 관리자
 * 계정은 **절대 테스트가 만들지 않고**, 주어졌을 때만 관리자 테스트를 돌린다.
 */
export const ADMIN_EMAIL = (process.env.E2E_ADMIN_EMAIL ?? "").trim();
export const ADMIN_PASSWORD = (process.env.E2E_ADMIN_PASSWORD ?? "").trim();
export const hasAdminCredentials = ADMIN_EMAIL !== "" && ADMIN_PASSWORD !== "";

/**
 * 테스트가 만든 계정을 나중에 골라낼 수 있게 하는 접두사.
 *
 * 계정 삭제 API가 없다(관리자 DB 콘솔에서도 `users`는 read-only). 그래서 테스트
 * 계정은 쌓인다 — 이 접두사가 유일한 회수 수단이고, `scripts/cleanup.sql`이 이걸
 * 기준으로 지운다. 바꾸려면 그 SQL도 같이 바꿔야 한다.
 */
export const TEST_EMAIL_PREFIX = "qa-e2e-";
export const TEST_EMAIL_DOMAIN = "quant-port.test";

/**
 * 2026-09-27부터 가입은 아이디라(요청 3) 새 테스트 계정에는 도메인이 없다 — 이 접두사가
 * 회수·통계 제외의 표식이다(`scripts/cleanup.sql`, BE `growth/activity.go`의 realUser).
 * `_`는 SQL LIKE에서 한 글자 와일드카드라 쓰지 않았다.
 */
export const TEST_LOGIN_PREFIX = "qae2e";
