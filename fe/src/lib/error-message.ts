import { PortfolioFetchError } from "@/lib/api-client";

/**
 * 서버·네트워크 오류를 **사용자가 읽을 수 있는 한 문장**으로 바꾼다.
 *
 * 왜 필요한가: 화면들이 `err.message`를 그대로 그렸다. 그 결과 한국어 서비스에서
 * 이런 것들이 사용자에게 보였다 (실측):
 *
 *   /market        → `internal`
 *   /analysis      → `internal` (3곳)
 *   /watchlist     → `internal`
 *   /news/domestic → `{"error":"internal"}`   ← JSON 봉투 원문
 *   /holdings      → `internal` / `Failed to fetch`
 *   /login         → `invalid email or password`
 *
 * 이 문자열들은 백엔드가 개발자에게 하는 말이지 사용자에게 하는 말이 아니다.
 * 「무엇이 잘못됐는지」도 「무엇을 하면 되는지」도 알려주지 않는다.
 *
 * 매핑을 한곳에 모은 이유: 화면마다 문구를 지어내면 같은 오류가 화면마다 다르게
 * 보이고, 백엔드 문구가 바뀌었을 때 어디를 고쳐야 하는지 알 수 없다.
 */

/** 서버가 내려주는 알려진 오류 문자열 → 사용자 문구. 소문자로 비교한다. */
const KNOWN: { match: (msg: string) => boolean; text: string }[] = [
  {
    match: (m) => m.includes("invalid email or password"),
    text: "이메일 또는 비밀번호가 올바르지 않습니다.",
  },
  {
    match: (m) => m.includes("email already registered"),
    text: "이미 가입된 이메일입니다.",
  },
  {
    match: (m) => m.includes("password must be at least"),
    text: "비밀번호는 8자 이상이어야 합니다.",
  },
  {
    match: (m) => m.includes("display name must be"),
    text: "표시 이름은 2~100자여야 합니다.",
  },
  {
    match: (m) => m.includes("invalid email address"),
    text: "이메일 주소 형식이 올바르지 않습니다.",
  },
  {
    match: (m) => m.includes("authentication required") || m.includes("unauthorized"),
    text: "로그인이 필요합니다. 다시 로그인해 주세요.",
  },
  {
    match: (m) => m.includes("rate limit") || m.includes("too many requests"),
    text: "요청이 너무 잦습니다. 잠시 후 다시 시도해 주세요.",
  },
  {
    match: (m) => m.includes("not found"),
    text: "요청한 자료를 찾을 수 없습니다.",
  },
  {
    /*
      네트워크 계층 실패. 브라우저마다 문구가 다르다 —
      Chrome `Failed to fetch`, Safari `Load failed`, Firefox `NetworkError…`.
    */
    match: (m) =>
      m.includes("failed to fetch") ||
      m.includes("load failed") ||
      m.includes("networkerror") ||
      m.includes("network request failed"),
    text: "네트워크에 연결하지 못했습니다. 연결 상태를 확인해 주세요.",
  },
  {
    match: (m) => m === "internal" || m.includes("internal server error"),
    text: "서버에서 문제가 발생했습니다. 잠시 후 다시 시도해 주세요.",
  },
];

/** `{"error":"internal"}` 같은 봉투가 문자열로 넘어온 경우 안쪽 값만 꺼낸다. */
function unwrapEnvelope(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed.startsWith("{")) return trimmed;
  try {
    const parsed = JSON.parse(trimmed) as { error?: unknown; message?: unknown };
    const inner = parsed.error ?? parsed.message;
    if (typeof inner === "string" && inner.trim()) return inner.trim();
  } catch {
    /* JSON이 아니면 원문 그대로 */
  }
  return trimmed;
}

/**
 * 사용자에게 보여줄 오류 문구.
 *
 * `fallback` 은 이 화면이 무엇을 하려다 실패했는지를 담는다 ("보유 종목을 불러오지
 * 못했습니다" 처럼). 알려진 오류가 아니면 그 문장을 쓴다 — **원문을 그대로 내보내는
 * 경로는 없다.**
 */
export function toUserMessage(err: unknown, fallback: string): string {
  const raw = err instanceof Error ? err.message : typeof err === "string" ? err : "";
  const unwrapped = unwrapEnvelope(raw);
  const lower = unwrapped.toLowerCase();

  for (const rule of KNOWN) {
    if (rule.match(lower)) return rule.text;
  }

  /*
    상태코드로 한 번 더 거른다. 백엔드는 매핑되지 않은 서비스 오류를 502로 내보내는데
    (`handler/toss_error.go`), 그 본문은 사용자에게 의미가 없다.
  */
  if (err instanceof PortfolioFetchError) {
    if (err.status === 401 || err.status === 403) return "로그인이 필요합니다. 다시 로그인해 주세요.";
    if (err.status === 429) return "요청이 너무 잦습니다. 잠시 후 다시 시도해 주세요.";
    if (err.status >= 500) return `${fallback} 잠시 후 다시 시도해 주세요.`;
  }

  /*
    한글이 섞여 있으면 서버가 사용자용으로 쓴 문구다(예: "본문을 작성해 주세요",
    "이번 달 발행 한도를 …"). 그건 그대로 보여주는 편이 낫다.
  */
  if (/[가-힣]/.test(unwrapped)) return unwrapped;

  return fallback;
}

/** 이 오류가 네트워크 단절인가 — 오프라인 배너를 띄울지 판단할 때 쓴다. */
export function isNetworkError(err: unknown): boolean {
  const raw = (err instanceof Error ? err.message : String(err ?? "")).toLowerCase();
  return (
    raw.includes("failed to fetch") ||
    raw.includes("load failed") ||
    raw.includes("networkerror") ||
    raw.includes("network request failed")
  );
}
