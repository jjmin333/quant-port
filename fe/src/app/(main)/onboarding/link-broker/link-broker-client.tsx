"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, useEffect, useState } from "react";
import { PageHeader } from "@/components/layout/header";
import { useAuth } from "@/components/providers/auth-provider";
import { fetchBrokerStatus, linkTossBroker } from "@/lib/auth";
import type { BrokerLinkStatus } from "@/lib/auth-types";

export default function LinkBrokerPage() {
  const { session, refresh } = useAuth();
  const router = useRouter();
  const [status, setStatus] = useState<BrokerLinkStatus | null>(null);
  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    fetchBrokerStatus()
      .then(setStatus)
      .catch(() => setStatus(null));
  }, []);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const next = await linkTossBroker(clientId, clientSecret);
      setStatus(next);
      await refresh();
      router.push("/");
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "연동에 실패했습니다.");
    } finally {
      setSubmitting(false);
    }
  }

  if (session?.brokerLinked || status?.linked) {
    return (
      <>
        <PageHeader title="토스증권 계좌 연동" subtitle="Open API 키 등록" />
        <main className="flex-1 overflow-y-auto p-6">
          <div className="mx-auto max-w-lg rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-8 text-center">
            <h2 className="text-xl font-semibold text-[var(--text-primary)]">토스증권 연동 완료</h2>
            <p className="mt-2 text-sm text-[var(--text-muted)]">
              {status?.accounts?.length ?? 0}개 계좌가 등록되었습니다.
            </p>
            <Link
              href="/"
              className="mt-6 inline-flex h-10 items-center rounded-lg bg-[var(--accent)] px-5 text-sm font-semibold text-white"
            >
              대시보드로 이동
            </Link>
          </div>
        </main>
      </>
    );
  }

  return (
    <>
      <PageHeader title="토스증권 계좌 연동" subtitle="Open API 키 등록" />
      <main className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto max-w-lg rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-8">
          <p className="text-sm text-[var(--text-secondary)]">
            토스증권 WTS에서 발급한 Open API Client ID와 Secret을 입력하세요. 키는 AES-GCM으로
            암호화되어 저장됩니다.
          </p>

          <form onSubmit={onSubmit} className="mt-6 space-y-4">
            <label className="block">
              <span className="mb-1 block text-sm text-[var(--text-secondary)]">Client ID</span>
              <input
                type="text"
                required
                value={clientId}
                onChange={(e) => setClientId(e.target.value)}
                className="h-11 w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 font-mono text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-sm text-[var(--text-secondary)]">Client Secret</span>
              <input
                type="password"
                required
                value={clientSecret}
                onChange={(e) => setClientSecret(e.target.value)}
                className="h-11 w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 font-mono text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
              />
            </label>

            {error && (
              <p className="rounded-lg bg-rose-500/10 px-3 py-2 text-sm text-rose-400">{error}</p>
            )}

            <button
              type="submit"
              disabled={submitting}
              className="h-11 w-full rounded-lg bg-[var(--accent)] text-sm font-semibold text-white hover:opacity-90 disabled:opacity-50"
            >
              {submitting ? "연동 확인 중…" : "연동하기"}
            </button>
          </form>

          <p className="mt-4 text-xs text-[var(--text-muted)]">
            서버 공인 IP를 토스증권 Open API 허용 목록에 등록해야 합니다.
          </p>
        </div>
      </main>
    </>
  );
}
