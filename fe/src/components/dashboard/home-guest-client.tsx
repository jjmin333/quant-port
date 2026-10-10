"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type ReactNode } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { MiniToggle } from "@/components/ui/mini-toggle";
import type { GuestMover } from "@/lib/home-guest.server";
import { cn } from "@/lib/utils";

/** 서버가 비로그인으로 보고 그렸는데 브라우저에서 세션이 살아나면(토큰 갱신) 걷는다. */
export function GuestOnly({ children }: { children: ReactNode }) {
  const { session } = useAuth();
  return session ? null : <>{children}</>;
}

/** 히어로 검색 — 고르면 그 종목 화면으로. */
export function GuestHeroSearch() {
  const router = useRouter();
  return (
    <StockCatalogSearchField
      placeholder="종목명 또는 티커 — 예: 삼성전자, NVDA"
      suggestionsId="guest-hero-suggestions"
      onSelect={(stock) => router.push(`/stocks/${encodeURIComponent(stock.symbol)}`)}
      className="w-full"
      inputClassName="h-11 rounded-lg bg-white text-[15px] text-neutral-900 placeholder:text-neutral-500"
    />
  );
}

const MARKETS = [
  { id: "domestic", label: "국내" },
  { id: "us", label: "해외" },
] as const;

function signed(v: number): string {
  return `${v > 0 ? "+" : v < 0 ? "−" : ""}${Math.abs(Math.round(v))}`;
}

/** 오늘 점수가 크게 바뀐 종목 — 국내·해외 전환. 줄마다 가장 크게 움직인 축(잔차 0 귀인). */
export function GuestMovers({ domestic, us, date }: { domestic: GuestMover[]; us: GuestMover[]; date?: string }) {
  const [market, setMarket] = useState<"domestic" | "us">(domestic.length > 0 ? "domestic" : "us");
  const rows = market === "domestic" ? domestic : us;
  return (
    <div className="flex h-full flex-col">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <h2 className="text-cell font-semibold text-[var(--text-primary)]">오늘 점수가 크게 바뀐 종목</h2>
        <span className="text-micro text-[var(--text-muted)]">{date ? `${date} · ` : ""}데이터 공백으로 생긴 변동은 뺐습니다</span>
        <MiniToggle className="ml-auto" options={[...MARKETS]} value={market} onChange={(v) => setMarket(v as "domestic" | "us")} />
      </div>
      {rows.length === 0 ? (
        <p className="mt-3 text-cell text-[var(--text-muted)]">오늘은 크게 바뀐 종목이 없습니다.</p>
      ) : (
        <ul className="mt-2 divide-y divide-[var(--border)]">
          {rows.map((r) => (
            <li key={r.symbol}>
              <Link
                href={`/stocks/${encodeURIComponent(r.symbol)}`}
                className="flex items-center gap-2 py-2 text-cell hover:bg-[var(--surface-elevated)]/60"
              >
                <span className="min-w-0 flex-1 truncate font-medium text-[var(--text-primary)]">{r.name}</span>
                {r.driver && (
                  <span className="hidden shrink-0 rounded-md bg-[var(--surface-elevated)] px-1.5 py-0.5 text-micro text-[var(--text-muted)] sm:inline">
                    {r.driver.label} {Math.round(r.driver.from)}→{Math.round(r.driver.to)}
                  </span>
                )}
                <span className="shrink-0 tabular-nums text-[var(--text-muted)]">
                  {r.from} → {r.to}
                </span>
                <span
                  className={cn(
                    "w-9 shrink-0 text-right font-semibold tabular-nums",
                    r.delta >= 0 ? "text-[var(--score-high)]" : "text-[var(--score-low)]",
                  )}
                >
                  {signed(r.delta)}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
      <p className="mt-auto pt-2 text-micro text-[var(--text-muted)]">
        회색 칸은 어제 대비 변화를 축별로 나눴을 때 가장 크게 움직인 축입니다. <Link href="/changes" className="text-[var(--brand)] hover:underline">오늘의 변화 전체 →</Link>
      </p>
    </div>
  );
}
