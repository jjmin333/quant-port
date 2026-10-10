"use client";

import { use } from "react";
import { StockWorkspaceClient } from "@/components/stock-workspace/stock-workspace-client";

type StockWorkspacePageClientProps = {
  symbol: string;
  /** 서버 요약에서 온 종목명 — 첫 HTML의 제목(h1)에 쓴다. */
  initialName?: string;
};

export function StockWorkspacePageClient({ symbol, initialName }: StockWorkspacePageClientProps) {
  return (
    // 터미널형 아케타입 — /market·/quotes와 같은 중앙 컬럼에 맞춘다.
    <div className="flex min-h-0 flex-1 justify-center overflow-hidden">
      <StockWorkspaceClient
        symbol={symbol}
        initialName={initialName}
        className="page-shell-wide min-h-0"
      />
    </div>
  );
}

export function StockWorkspacePageLoader({
  params,
  initialName,
}: {
  params: Promise<{ symbol: string }>;
  initialName?: string;
}) {
  const { symbol } = use(params);
  return <StockWorkspacePageClient symbol={decodeURIComponent(symbol)} initialName={initialName} />;
}
