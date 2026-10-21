"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { FormEvent, useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { toUserMessage } from "@/lib/error-message";
import { signupHref } from "@/lib/auth-routes";
import { BrandLockup } from "@/components/layout/brand-mark";

export default function LoginPage() {
  const { login } = useAuth();
  const searchParams = useSearchParams();
  const [loginId, setLoginId] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(loginId, password, searchParams.get("next") ?? undefined);
    } catch (err) {
      setError(toUserMessage(err, "로그인에 실패했습니다."));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-8 shadow-xl">
      <div className="mb-8 text-center">
        <BrandLockup className="justify-center" />
        <h1 className="mt-2 text-2xl font-semibold text-[var(--text-primary)]">로그인</h1>
        <p className="mt-1 text-sm text-[var(--text-muted)]">통합 자산 관리 계정으로 접속하세요.</p>
      </div>

      <form onSubmit={onSubmit} className="space-y-4">
        <label className="block">
          <span className="mb-1 block text-sm text-[var(--text-secondary)]">아이디</span>
          {/*
            기존 회원은 가입 때 쓴 **이메일이 그대로 아이디**다 (2026-09-27 요청 3). 그래서
            type="email"을 쓰지 않는다 — 새 아이디(영문·숫자)를 브라우저가 형식 오류로 막는다.
          */}
          <input
            type="text"
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            required
            value={loginId}
            onChange={(e) => setLoginId(e.target.value)}
            className="h-11 w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-sm text-[var(--text-primary)] focus:border-[var(--brand)] focus:outline-none"
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-sm text-[var(--text-secondary)]">비밀번호</span>
          <input
            type="password"
            autoComplete="current-password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="h-11 w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-sm text-[var(--text-primary)] focus:border-[var(--brand)] focus:outline-none"
          />
        </label>

        {error && (
          <p className="rounded-lg bg-rose-500/10 px-3 py-2 text-sm text-rose-400">{error}</p>
        )}

        <button
          type="submit"
          disabled={submitting}
          className="h-11 w-full rounded-lg bg-[var(--brand)] text-sm font-semibold text-[var(--brand-foreground)] hover:opacity-90 disabled:opacity-50"
        >
          {submitting ? "로그인 중…" : "로그인"}
        </button>
      </form>

      <p className="mt-6 text-center text-sm text-[var(--text-muted)]">
        계정이 없으신가요?{" "}
        <Link href={signupHref(searchParams.get("next"))} className="text-[var(--brand)] hover:underline">
          회원가입
        </Link>
      </p>

      {searchParams.get("next") && (
        <p className="mt-2 text-center text-cell text-[var(--text-muted)]">
          로그인 후 이전 페이지로 이동합니다.
        </p>
      )}
    </div>
  );
}
