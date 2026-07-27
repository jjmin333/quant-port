import { AlertTriangle } from "lucide-react";
import { Card } from "@/components/ui/card";
import { BrokerLinkRequiredPanel, isBrokerLinkErrorMessage } from "@/components/auth/broker-link-required-panel";
import {
  LoginRequiredPanel,
  isAuthErrorMessage,
} from "@/components/auth/login-required-panel";
import type { PortfolioDiagnostics } from "@/lib/portfolio-types";

export function PortfolioErrorBanner({
  message,
  diagnostics,
  loginNext,
}: {
  message: string;
  diagnostics?: PortfolioDiagnostics | null;
  loginNext?: string;
}) {
  if (isAuthErrorMessage(message)) {
    return <LoginRequiredPanel loginNext={loginNext} />;
  }

  if (isBrokerLinkErrorMessage(message)) {
    return <BrokerLinkRequiredPanel />;
  }

  return (
    <Card className="border-rose-500/40 bg-rose-500/5 p-4">
      <div className="flex gap-3">
        <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-rose-400" />
        <div className="space-y-2 text-sm">
          <p className="font-medium text-rose-300">실시간 데이터를 불러오지 못했습니다</p>
          <p className="text-[var(--text-secondary)]">{message}</p>
          {diagnostics?.lastError && (
            <p className="font-mono text-xs text-[var(--text-muted)]">{diagnostics.lastError}</p>
          )}
          {diagnostics?.hints && diagnostics.hints.length > 0 && (
            <ul className="list-disc space-y-1 pl-4 text-[var(--text-muted)]">
              {diagnostics.hints.map((hint) => (
                <li key={hint}>{hint}</li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </Card>
  );
}

export function PortfolioSourceBadge({ provider, fetchedAt }: { provider: string; fetchedAt: string }) {
  const time = new Date(fetchedAt).toLocaleString("ko-KR", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });

  return (
    <p className="text-xs text-[var(--text-muted)]">
      {provider === "toss" ? "토스증권 Open API" : provider} · {time} 갱신
    </p>
  );
}

export function PortfolioRefreshStatus({
  summary,
  isRefreshing,
  isLiveRefreshing,
  pollError,
}: {
  summary: {
    source: { provider: string; fetchedAt: string };
    exchangeRate: number;
    liveRefreshIntervalSec?: number;
    fullRefreshIntervalSec?: number;
    refreshIntervalSec?: number;
  };
  isRefreshing: boolean;
  isLiveRefreshing?: boolean;
  pollError: string | null;
}) {
  const liveSec = summary.liveRefreshIntervalSec ?? 5;
  const fullSec = summary.fullRefreshIntervalSec ?? summary.refreshIntervalSec ?? 60;

  return (
    <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-[var(--text-muted)]">
      <PortfolioSourceBadge provider={summary.source.provider} fetchedAt={summary.source.fetchedAt} />
      <div className="flex flex-wrap items-center gap-2">
        <span>USD/KRW {summary.exchangeRate.toLocaleString("ko-KR")}</span>
        <span aria-hidden>·</span>
        <span>
          시세 {liveSec}초 · 보유 {fullSec}초 갱신
        </span>
        {(isLiveRefreshing || isRefreshing) && (
          <span className="text-[var(--accent)]">시세 갱신 중…</span>
        )}
        {pollError && <span className="text-rose-400">{pollError}</span>}
      </div>
    </div>
  );
}
