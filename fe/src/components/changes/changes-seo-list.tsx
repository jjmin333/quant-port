import Link from "next/link";
import { getApiBaseUrl } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import { fetchWithDeadline } from "@/lib/fetch-timeout";
import { groupChanges, type ChangesResponse } from "@/lib/changes-api";

/*
「오늘의 변화」의 **서버가 그리는 목록** (2026-10-10, BUSINESS_REVIEW §4.2).

화면(`ChangesClient`)은 클라이언트에서만 그려져 크롤러가 받는 HTML에 종목 링크가 0개였다. 매일 새로
쌓이는 사건(급등락·공시·실적·점수 급변)은 경쟁사가 못 만드는 날짜별 콘텐츠라, 날짜마다 주소
(`/changes/YYYY-MM-DD`)를 주고 그날 시가총액 상위 종목의 첫 사건을 링크와 함께 서버에서 그린다.
*/

const ROWS_PER_MARKET = 30;
export const CHANGE_DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

export async function fetchChangesServer(market: "domestic" | "us", date?: string): Promise<ChangesResponse | null> {
  const qs = new URLSearchParams({ market });
  if (date) qs.set("date", date);
  try {
    const res = await fetchWithDeadline(`${getApiBaseUrl()}/api/v1/changes?${qs}`, { next: { revalidate: 1800 } });
    if (!res.ok) return null;
    const body = (await res.json()) as ApiResponse<ChangesResponse>;
    return body.data ?? null;
  } catch {
    return null;
  }
}

const MARKETS = [
  { id: "domestic", label: "국내" },
  { id: "us", label: "해외" },
] as const;

export async function ChangesSeoList({ date }: { date?: string }) {
  const [domestic, us] = await Promise.all([fetchChangesServer("domestic", date), fetchChangesServer("us", date)]);
  const day = domestic?.date || us?.date;
  if (!day) return null;
  const sections = MARKETS.map((m) => ({
    ...m,
    groups: groupChanges((m.id === "domestic" ? domestic : us)?.items ?? []).slice(0, ROWS_PER_MARKET),
  })).filter((s) => s.groups.length > 0);
  if (sections.length === 0) return null;
  const otherDates = (domestic?.dates ?? us?.dates ?? []).filter((d) => d !== day).slice(0, 10);
  const headline = sections.flatMap((s) => s.groups.slice(0, 2).map((g) => g.name)).slice(0, 4).join(" · ");

  return (
    <details className="group/seo shrink-0 border-b border-[var(--border)]">
      <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-1 text-micro text-[var(--text-muted)] [&::-webkit-details-marker]:hidden">
        <span className="min-w-0 flex-1 truncate">
          {day} 주식 변화 — {headline}
        </span>
        <span className="shrink-0 text-[var(--brand)]">
          <span className="group-open/seo:hidden">목록 펼치기 ▾</span>
          <span className="hidden group-open/seo:inline">접기 ▴</span>
        </span>
      </summary>
      <div className="space-y-3 px-3 pb-3 pt-1 text-cell">
        {sections.map((s) => (
          <section key={s.id}>
            <h2 className="mb-1 text-micro font-semibold text-[var(--text-muted)]">
              {s.label} · 시가총액 상위 {s.groups.length}종목
            </h2>
            <ul className="space-y-0.5">
              {s.groups.map((g) => (
                <li key={g.symbol} className="truncate">
                  <Link
                    href={`/stocks/${encodeURIComponent(g.symbol)}`}
                    prefetch={false}
                    className="font-medium text-[var(--text-primary)] hover:underline"
                  >
                    {g.name}
                  </Link>{" "}
                  <span className="text-[var(--text-secondary)]">{g.events[0]?.title}</span>
                </li>
              ))}
            </ul>
          </section>
        ))}
        {otherDates.length > 0 && (
          <nav aria-label="지난 변화">
            <h2 className="mb-1 text-micro font-semibold text-[var(--text-muted)]">지난 날짜</h2>
            <ul className="flex flex-wrap gap-1.5">
              {otherDates.map((d) => (
                <li key={d}>
                  <Link
                    href={`/changes/${d}`}
                    prefetch={false}
                    className="rounded-full border border-[var(--border)] px-2 py-0.5 tabular-nums text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
                  >
                    {d}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        )}
      </div>
    </details>
  );
}
