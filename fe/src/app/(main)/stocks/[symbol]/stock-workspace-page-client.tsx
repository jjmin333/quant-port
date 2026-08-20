"use client";

import { use } from "react";
import { StockWorkspaceClient } from "@/components/stock-workspace/stock-workspace-client";

type StockWorkspacePageClientProps = {
  symbol: string;
};

export function StockWorkspacePageClient({ symbol }: StockWorkspacePageClientProps) {
  return (
    // 터미널형 아케타입 — /market·/quotes와 같은 중앙 컬럼에 맞춘다.
    <div className="flex min-h-0 flex-1 justify-center overflow-hidden">
      <StockWorkspaceClient
        symbol={symbol}
        loginNext={`/stocks/${encodeURIComponent(symbol)}`}
        className="page-shell-wide min-h-0"
      />
    </div>
  );
}

export function StockWorkspacePageLoader({
  params,
}: {
  params: Promise<{ symbol: string }>;
}) {
  const { symbol } = use(params);
  return <StockWorkspacePageClient symbol={decodeURIComponent(symbol)} />;
}
