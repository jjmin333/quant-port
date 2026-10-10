/**
 * 가입 입력 규칙 — 서버(`be/internal/service/auth.go`)와 **같은 규칙**을 화면에서 먼저 본다
 * (2026-09-27 사용자 요청 3). 서버가 최종 판정이고, 여기는 제출 전에 「무엇이 틀렸는지」를
 * 입력칸 바로 아래에서 말해 주려는 것이다.
 *
 * 길이는 **글자 수**로 센다(서버도 rune). 한글 닉네임 「투자왕」은 3자다.
 */

export const LOGIN_ID_MIN = 4;
export const LOGIN_ID_MAX = 20;
export const DISPLAY_NAME_MIN = 2;
export const DISPLAY_NAME_MAX = 10;
export const PASSWORD_MIN = 8;
export const PASSWORD_MAX = 15;

const LOGIN_ID_RE = /^[a-z][a-z0-9]*$/;

function charCount(s: string): number {
  return [...s].length;
}

/** 새 아이디 — 영문 소문자로 시작하는 소문자·숫자 4~20자. 문제가 없으면 null. */
export function loginIdProblem(id: string): string | null {
  if (id.length === 0) return null;
  if (id.length < LOGIN_ID_MIN || id.length > LOGIN_ID_MAX) {
    return `아이디는 ${LOGIN_ID_MIN}~${LOGIN_ID_MAX}자입니다.`;
  }
  if (/[A-Z]/.test(id)) return "대문자 대신 소문자를 써 주세요.";
  if (!LOGIN_ID_RE.test(id)) return "영문 소문자로 시작하고, 소문자와 숫자만 쓸 수 있습니다.";
  return null;
}

export function displayNameProblem(name: string): string | null {
  const n = charCount(name.trim());
  if (name.length === 0) return null;
  if (n < DISPLAY_NAME_MIN || n > DISPLAY_NAME_MAX) {
    return `닉네임은 ${DISPLAY_NAME_MIN}~${DISPLAY_NAME_MAX}자입니다.`;
  }
  return null;
}

export function passwordProblem(pw: string): string | null {
  const n = charCount(pw);
  if (pw.length === 0) return null;
  if (n < PASSWORD_MIN || n > PASSWORD_MAX) {
    return `비밀번호는 ${PASSWORD_MIN}~${PASSWORD_MAX}자입니다.`;
  }
  return null;
}
