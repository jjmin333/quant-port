"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import {
  fetchBrokerStatus,
  linkTossBroker,
  unlinkTossBroker,
} from "@/lib/auth";
import type { BrokerLinkStatus } from "@/lib/auth-types";

export function BrokerSettings() {
  const { session, refresh } = useAuth();
  const [status, setStatus] = useState<BrokerLinkStatus | null>(null);
  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    fetchBrokerStatus()
      .then(setStatus)
      .catch(() => setStatus(null))
      .finally(() => setLoading(false));
  }, []);

  async function onLink(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setMessage(null);
    setSubmitting(true);
    try {
      const next = await linkTossBroker(clientId, clientSecret);
      setStatus(next);
      setClientId("");
      setClientSecret("");
      setMessage("토스증권 연동이 완료되었습니다.");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "연동에 실패했습니다.");
    } finally {
      setSubmitting(false);
    }
  }

  async function onUnlink() {
    if (!confirm("토스증권 연동을 해제하시겠습니까?")) return;
    setSubmitting(true);
    setError(null);
    try {
      await unlinkTossBroker();
      setStatus({ linked: false, brokerCode: "toss" });
      setMessage("연동이 해제되었습니다.");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "해제에 실패했습니다.");
    } finally {
      setSubmitting(false);
    }
  }

  if (!session) {
    return (
      <div className="rounded-xl border border-[var(--border)] bg-[var(--background)] p-4 text-sm text-[var(--text-secondary)]">
        <p>토스증권 연동은 로그인 후 이용할 수 있습니다.</p>
        <Link href="/login?next=/settings" className="mt-3 inline-flex text-[var(--accent)] hover:underline">
          로그인하기
        </Link>
      </div>
    );
  }

  if (loading) {
    return <p className="text-sm text-[var(--text-muted)]">불러오는 중…</p>;
  }

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-base font-semibold text-[var(--text-primary)]">토스증권 Open API</h3>
        <p className="mt-1 text-sm text-[var(--text-muted)]">
          본인 계좌와 매칭되는 Client ID/Secret을 등록하면 보유 종목·거래내역 기능을 사용할 수
          있습니다.
        </p>
      </div>

      {status?.linked ? (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--background)] p-4">
          <p className="text-sm font-medium text-emerald-400">연동됨</p>
          {status.clientIdHint && (
            <p className="mt-1 text-xs text-[var(--text-muted)]">Client ID: {status.clientIdHint}</p>
          )}
          {status.accounts && status.accounts.length > 0 && (
            <ul className="mt-3 space-y-1 text-sm text-[var(--text-secondary)]">
              {status.accounts.map((acct) => (
                <li key={acct.id}>
                  {acct.accountName}
                  {acct.accountNo ? ` · ${acct.accountNo}` : ""}
                </li>
              ))}
            </ul>
          )}
          <button
            type="button"
            onClick={onUnlink}
            disabled={submitting}
            className="mt-4 rounded-lg border border-rose-500/40 px-4 py-2 text-sm text-rose-400 hover:bg-rose-500/10 disabled:opacity-50"
          >
            연동 해제
          </button>
        </div>
      ) : (
        <form onSubmit={onLink} className="space-y-4">
          <label className="block">
            <span className="mb-1 block text-sm text-[var(--text-secondary)]">Client ID</span>
            <input
              type="text"
              required
              value={clientId}
              onChange={(e) => setClientId(e.target.value)}
              className="h-10 w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-sm font-mono focus:border-[var(--accent)] focus:outline-none"
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-sm text-[var(--text-secondary)]">Client Secret</span>
            <input
              type="password"
              required
              value={clientSecret}
              onChange={(e) => setClientSecret(e.target.value)}
              className="h-10 w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-sm font-mono focus:border-[var(--accent)] focus:outline-none"
            />
          </label>
          <button
            type="submit"
            disabled={submitting}
            className="rounded-lg bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-white disabled:opacity-50"
          >
            {submitting ? "연동 중…" : "연동하기"}
          </button>
        </form>
      )}

      {message && <p className="text-sm text-emerald-400">{message}</p>}
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </div>
  );
}
