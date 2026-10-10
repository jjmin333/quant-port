import { describe, expect, it } from "vitest";
import { displayNameProblem, loginIdProblem, passwordProblem } from "@/lib/auth-rules";

describe("가입 규칙 — 서버 auth.go와 같은 판정 (2026-09-27 요청 3)", () => {
  it("아이디: 소문자로 시작하는 소문자·숫자 4~20자", () => {
    for (const ok of ["abcd", "a123", "quant2026", "a".repeat(20)]) expect(loginIdProblem(ok)).toBeNull();
    for (const bad of ["abc", "a".repeat(21), "1abc", "Abcd", "ab_cd", "user@mail.com", "한글아이디"]) {
      expect(loginIdProblem(bad)).not.toBeNull();
    }
  });

  it("빈 칸은 아직 문제로 보지 않는다 — 입력 전부터 빨간 글씨를 띄우지 않는다", () => {
    expect(loginIdProblem("")).toBeNull();
    expect(displayNameProblem("")).toBeNull();
    expect(passwordProblem("")).toBeNull();
  });

  it("닉네임: 글자 수 2~10 (한글도 한 글자)", () => {
    expect(displayNameProblem("투자왕")).toBeNull();
    expect(displayNameProblem("열글자닉네임입니다요")).toBeNull();
    expect(displayNameProblem("열한글자닉네임입니다요")).not.toBeNull();
    expect(displayNameProblem("a")).not.toBeNull();
  });

  it("비밀번호: 8~15자", () => {
    expect(passwordProblem("1234567")).not.toBeNull();
    expect(passwordProblem("12345678")).toBeNull();
    expect(passwordProblem("123456789012345")).toBeNull();
    expect(passwordProblem("1234567890123456")).not.toBeNull();
  });
});
