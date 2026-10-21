"use client";

import Link from "next/link";
import { FormEvent, useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { toUserMessage } from "@/lib/error-message";
import {
  DISPLAY_NAME_MAX,
  DISPLAY_NAME_MIN,
  LOGIN_ID_MAX,
  LOGIN_ID_MIN,
  PASSWORD_MAX,
  PASSWORD_MIN,
  displayNameProblem,
  loginIdProblem,
  passwordProblem,
} from "@/lib/auth-rules";
import { BrandLockup } from "@/components/layout/brand-mark";

const INPUT_CLASS =
  "h-11 w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-sm text-[var(--text-primary)] focus:border-[var(--brand)] focus:outline-none aria-[invalid=true]:border-rose-400";

/**
 * 회원가입 — **아이디 · 닉네임 · 비밀번호** (2026-09-27 사용자 요청 3, 이메일 없음).
 *
 * 규칙은 입력칸 바로 아래에서 말한다(`lib/auth-rules` — 서버와 같은 규칙). 제출하고 나서
 * 서버 오류로 처음 알게 하면, 세 칸 중 무엇이 틀렸는지 다시 읽어야 한다.
 */
export default function SignupPage() {
  const { signup } = useAuth();
  const [loginId, setLoginId] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const idProblem = loginIdProblem(loginId);
  const nameProblem = displayNameProblem(displayName);
  const pwProblem = passwordProblem(password);
  const ready =
    loginId !== "" && displayName.trim() !== "" && password !== "" && !idProblem && !nameProblem && !pwProblem;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!ready) return;
    setError(null);
    setSubmitting(true);
    try {
      // `useSearchParams`를 쓰면 이 화면 전체를 Suspense로 감싸야 한다. 값은 제출할 때 한 번만
      // 필요하므로 그 순간에 읽는다.
      const next = new URLSearchParams(window.location.search).get("next") ?? undefined;
      await signup(loginId, password, displayName.trim(), next);
    } catch (err) {
      setError(toUserMessage(err, "회원가입에 실패했습니다."));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-8 shadow-xl">
      <div className="mb-8 text-center">
        <BrandLockup className="justify-center" />
        <h1 className="mt-2 text-2xl font-semibold text-[var(--text-primary)]">회원가입</h1>
        <p className="mt-1 text-sm text-[var(--text-muted)]">
          아이디 · 닉네임 · 비밀번호만 있으면 됩니다.
        </p>
      </div>

      <form onSubmit={onSubmit} className="space-y-4" noValidate>
        <Field
          label="아이디"
          hint={`영문 소문자로 시작, 소문자·숫자 ${LOGIN_ID_MIN}~${LOGIN_ID_MAX}자`}
          problem={idProblem}
        >
          <input
            type="text"
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            required
            maxLength={LOGIN_ID_MAX}
            value={loginId}
            onChange={(e) => setLoginId(e.target.value.trim())}
            aria-invalid={Boolean(idProblem)}
            className={INPUT_CLASS}
          />
        </Field>
        <Field
          label="닉네임"
          hint={`${DISPLAY_NAME_MIN}~${DISPLAY_NAME_MAX}자 · 다른 회원과 겹칠 수 없습니다`}
          problem={nameProblem}
        >
          <input
            type="text"
            autoComplete="nickname"
            required
            maxLength={DISPLAY_NAME_MAX}
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            aria-invalid={Boolean(nameProblem)}
            className={INPUT_CLASS}
          />
        </Field>
        <Field label="비밀번호" hint={`${PASSWORD_MIN}~${PASSWORD_MAX}자`} problem={pwProblem}>
          <input
            type="password"
            autoComplete="new-password"
            required
            maxLength={PASSWORD_MAX}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            aria-invalid={Boolean(pwProblem)}
            className={INPUT_CLASS}
          />
        </Field>

        {error && (
          <p className="rounded-lg bg-rose-500/10 px-3 py-2 text-sm text-rose-400">{error}</p>
        )}

        <button
          type="submit"
          disabled={submitting || !ready}
          className="h-11 w-full rounded-lg bg-[var(--brand)] text-sm font-semibold text-[var(--brand-foreground)] hover:opacity-90 disabled:opacity-50"
        >
          {submitting ? "가입 중…" : "회원가입"}
        </button>
      </form>

      <p className="mt-6 text-center text-sm text-[var(--text-muted)]">
        이미 계정이 있으신가요?{" "}
        <Link href="/login" className="text-[var(--brand)] hover:underline">
          로그인
        </Link>
      </p>
    </div>
  );
}

function Field({
  label,
  hint,
  problem,
  children,
}: {
  label: string;
  hint: string;
  problem: string | null;
  children: React.ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm text-[var(--text-secondary)]">{label}</span>
      {children}
      <span
        className={
          problem
            ? "mt-1 block text-cell text-rose-400"
            : "mt-1 block text-cell text-[var(--text-muted)]"
        }
      >
        {problem ?? hint}
      </span>
    </label>
  );
}
