"use client";

import Link from "next/link";
import { FormEvent, useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { toUserMessage } from "@/lib/error-message";

export default function SignupPage() {
  const { signup } = useAuth();
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await signup(email, password, displayName.trim());
    } catch (err) {
      setError(toUserMessage(err, "회원가입에 실패했습니다."));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-8 shadow-xl">
      <div className="mb-8 text-center">
        <p className="text-sm font-bold tracking-[0.18em] text-[var(--brand)]">QUANT PORT</p>
        <h1 className="mt-2 text-2xl font-semibold text-[var(--text-primary)]">회원가입</h1>
        <p className="mt-1 text-sm text-[var(--text-muted)]">
          닉네임과 8자 이상 비밀번호로 계정을 만드세요.
        </p>
      </div>

      <form onSubmit={onSubmit} className="space-y-4">
        <label className="block">
          <span className="mb-1 block text-sm text-[var(--text-secondary)]">이메일</span>
          <input
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="h-11 w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-sm text-[var(--text-primary)] focus:border-[var(--brand)] focus:outline-none"
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-sm text-[var(--text-secondary)]">닉네임</span>
          <input
            type="text"
            autoComplete="nickname"
            required
            minLength={2}
            maxLength={100}
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="2~100자"
            className="h-11 w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-sm text-[var(--text-primary)] focus:border-[var(--brand)] focus:outline-none"
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-sm text-[var(--text-secondary)]">비밀번호</span>
          <input
            type="password"
            autoComplete="new-password"
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
          className="h-11 w-full rounded-lg bg-[var(--brand)] text-sm font-semibold text-white hover:opacity-90 disabled:opacity-50"
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
