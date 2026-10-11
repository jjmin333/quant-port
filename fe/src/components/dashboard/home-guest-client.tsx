"use client";

import { useRouter } from "next/navigation";
import type { ReactNode } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";

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
