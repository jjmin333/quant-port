import { Suspense } from "react";
import { StockWorkspacePageLoader } from "./stock-workspace-page-client";

type Props = { params: Promise<{ symbol: string }> };

export default function StockWorkspacePage({ params }: Props) {
  return (
    <Suspense
      fallback={
        <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
          종목 정보를 불러오는 중…
        </main>
      }
    >
      <StockWorkspacePageLoader params={params} />
    </Suspense>
  );
}
