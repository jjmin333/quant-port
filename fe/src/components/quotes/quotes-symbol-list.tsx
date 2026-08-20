"use client";

import Link from "next/link";
import { stockWorkspaceHref } from "@/lib/stock-workspace-tabs";

/**
 * 「실시간 차트」(/market) 종목 패널 하단의 밸류에이션 링크.
 *
 * 이 파일에는 원래 `QuotesSymbolList`(종목 목록 전체)도 있었는데, 그것을 쓰던
 * `/quotes` 페이지가 사이드바 어디에도 없는 고아 라우트여서 함께 지웠다.
 * 목록 자체는 `/market`의 market-radar가 대신한다.
 *
 * 목록이 쓰던 `GET /api/v1/quotes`는 **남아 있다** — 관심 종목 화면이
 * `filter=watchlist`로 같은 엔드포인트를 쓴다.
 */
export function QuotesSymbolListFooter({ symbol }: { symbol: string }) {
  return (
    <div className="shrink-0 border-t border-[var(--border)] p-2">
      <Link
        href={stockWorkspaceHref(symbol, "valuation")}
        className="block rounded-lg bg-[var(--surface-elevated)] px-3 py-2 text-center text-xs text-[var(--brand)] hover:bg-[var(--brand-muted)]"
      >
        밸류에이션 →
      </Link>
    </div>
  );
}
