import Link from "next/link";
import { getApiBaseUrl } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import { fetchWithDeadline } from "@/lib/fetch-timeout";
import type { EarningsEvent, EarningsMarket } from "@/lib/earnings-types";
import { todayKST } from "@/lib/earnings-range";
import { fetchIndexableStocks } from "@/lib/stock-seo.server";

/*
실적 일정 화면의 **서버가 그리는 목록** (2026-10-10, BUSINESS_REVIEW §4.2 · 성장 보고서 기회 2위).

화면 본문(`EarningsClient`)은 클라이언트에서만 그려져 크롤러가 받는 HTML에 종목 링크가 0개였다.
「엔비디아 실적 발표일」 같은 검색은 분기마다 몰리는데, 그 수요가 닿을 종목 페이지로 가는 길이 없었다.
종목 페이지의 접힌 요약 바와 같은 모양(접힌 한 줄)으로, 앞으로 2주 안 발표 가운데 **색인 대상 종목만** 건다.
*/

const LIST_DAYS = 14;
const MAX_ROWS = 60;

function addDays(ymd: string, days: number): string {
  const d = new Date(`${ymd}T00:00:00+09:00`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
}

async function fetchUpcoming(market: EarningsMarket): Promise<EarningsEvent[]> {
  const from = todayKST();
  const to = addDays(from, LIST_DAYS);
  try {
    const res = await fetchWithDeadline(
      `${getApiBaseUrl()}/api/v1/earnings/calendar?from=${from}&to=${to}&market=${market}`,
      { next: { revalidate: 3600 } },
    );
    if (!res.ok) return [];
    const body = (await res.json()) as ApiResponse<EarningsEvent[]>;
    return body.data ?? [];
  } catch {
    return [];
  }
}

const STATUS: Record<string, string> = { confirmed: "확정", scheduled: "예정", estimated: "추정" };

export async function EarningsSeoList({ market }: { market: EarningsMarket }) {
  const [events, indexable] = await Promise.all([fetchUpcoming(market), fetchIndexableStocks()]);
  const allowed = new Set(indexable.map((s) => s.symbol));
  const seen = new Set<string>();
  const rows = events
    .filter((e) => allowed.has(e.symbol) && !seen.has(e.symbol) && seen.add(e.symbol))
    .sort((a, b) => a.date.localeCompare(b.date))
    .slice(0, MAX_ROWS);
  if (rows.length === 0) return null;

  const label = market === "us" ? "해외" : "국내";
  return (
    <details className="group/seo shrink-0 border-b border-[var(--border)]">
      <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-1 text-micro text-[var(--text-muted)] [&::-webkit-details-marker]:hidden">
        <span className="min-w-0 flex-1 truncate">
          앞으로 2주 {label} 주요 실적 발표 {rows.length}건 — {rows.slice(0, 3).map((r) => r.name ?? r.symbol).join(" · ")}
        </span>
        <span className="shrink-0 text-[var(--brand)]">
          <span className="group-open/seo:hidden">목록 펼치기 ▾</span>
          <span className="hidden group-open/seo:inline">접기 ▴</span>
        </span>
      </summary>
      <ul className="grid gap-x-4 gap-y-0.5 px-3 pb-3 pt-1 text-cell sm:grid-cols-2 lg:grid-cols-3">
        {rows.map((e) => (
          <li key={e.symbol} className="truncate">
            <span className="tabular-nums text-[var(--text-muted)]">{e.date.slice(5).replace("-", "/")}</span>{" "}
            <Link
              href={`/stocks/${encodeURIComponent(e.symbol)}`}
              prefetch={false}
              className="text-[var(--text-primary)] hover:underline"
            >
              {e.name ?? e.symbol}
            </Link>{" "}
            <span className="text-[var(--text-muted)]">
              {e.fiscalYear} {e.fiscalPeriod} 실적{STATUS[e.dateStatus] ? `(${STATUS[e.dateStatus]})` : ""}
            </span>
          </li>
        ))}
      </ul>
    </details>
  );
}
