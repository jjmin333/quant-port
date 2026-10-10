import Link from "next/link";
import type { Metadata } from "next";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { JsonLd } from "@/components/seo/json-ld";
import { getApiBaseUrl } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import type { EarningsEvent } from "@/lib/earnings-types";
import { fetchWithDeadline } from "@/lib/fetch-timeout";
import { breadcrumbJsonLd, pageMetadata } from "@/lib/seo";
import { fetchIndexableStocks } from "@/lib/stock-seo.server";

/*
 * 실적 서프라이즈 모음 — 최근 90일, 시장별 예상을 가장 크게 넘은·밑돈 발표 (2026-10-10 사업화 검토:
 * 실적 시즌 「어닝 서프라이즈 종목」 검색). BE `/earnings/surprises`(best·worst 각 100)를 서버가 그린다.
 * **색인 대상(거래대금 상위) 종목만** 싣는다 — 추정치가 0에 가까운 소형주의 「+6,000%」를 걸러 내고,
 * noindex 종목 페이지로 링크를 흘리지 않는다.
 */
export const metadata: Metadata = pageMetadata({
  title: "실적 서프라이즈 종목 — 국내·미국 최근 90일",
  description:
    "최근 90일 국내·미국 실적 발표 가운데 예상(컨센서스)을 가장 크게 넘거나 밑돈 종목과 다음 날 주가 반응을 모았습니다. 매일 갱신.",
  path: "/earnings/surprises",
});

const MARKETS = [
  { id: "domestic", label: "국내" },
  { id: "us", label: "미국" },
] as const;

const METRIC: Record<string, string> = { opIncome: "영업이익", eps: "EPS", revenue: "매출" };
const SHOW = 20;

async function surprises(market: string, order: "best" | "worst"): Promise<EarningsEvent[]> {
  try {
    const res = await fetchWithDeadline(
      `${getApiBaseUrl()}/api/v1/earnings/surprises?market=${market}&order=${order}&limit=100`,
      { next: { revalidate: 3600 } },
    );
    if (!res.ok) return [];
    return ((await res.json()) as ApiResponse<EarningsEvent[]>).data ?? [];
  } catch {
    return [];
  }
}

function pct(v: number | undefined): string {
  if (v == null || !Number.isFinite(v)) return "–";
  return `${v > 0 ? "+" : ""}${v.toFixed(1)}%`;
}

function SurpriseTable({ rows }: { rows: EarningsEvent[] }) {
  if (rows.length === 0) return <p className="text-cell text-[var(--text-muted)]">해당 발표가 없습니다.</p>;
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[440px] text-cell">
        <thead className="text-micro text-[var(--text-muted)]">
          <tr className="border-b border-[var(--border)] text-left">
            <th className="py-1.5 pr-2 font-normal">종목</th>
            <th className="py-1.5 pr-2 font-normal">발표</th>
            <th className="py-1.5 pr-2 text-right font-normal">예상 대비</th>
            <th className="py-1.5 text-right font-normal">다음 날 주가</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((e) => (
            <tr key={`${e.symbol}-${e.date}`} className="border-b border-[var(--border)] last:border-0">
              <td className="py-1.5 pr-2">
                <Link href={`/stocks/${encodeURIComponent(e.symbol)}`} prefetch={false} className="font-medium text-[var(--text-primary)] hover:underline">
                  {e.name || e.symbol}
                </Link>
              </td>
              <td className="py-1.5 pr-2 tabular-nums text-[var(--text-secondary)]">
                {e.date} · {e.fiscalYear} {e.fiscalPeriod}
                {e.basis === "개별" && <span className="text-micro text-[var(--text-muted)]"> · 개별 기준</span>}
              </td>
              <td className="py-1.5 pr-2 text-right tabular-nums text-[var(--text-primary)]">
                {e.surprise ? `${METRIC[e.surprise.metric] ?? e.surprise.metric} ${pct(e.surprise.pct)}` : "–"}
              </td>
              <td className="py-1.5 text-right tabular-nums text-[var(--text-secondary)]">{pct(e.reaction?.pct)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default async function EarningsSurprisesPage() {
  const indexable = new Set((await fetchIndexableStocks()).map((s) => s.symbol));
  const keep = (list: EarningsEvent[]) => list.filter((e) => indexable.has(e.symbol) && e.surprise).slice(0, SHOW);
  const sections = [];
  for (const m of MARKETS) {
    sections.push({ ...m, best: keep(await surprises(m.id, "best")), worst: keep(await surprises(m.id, "worst")) });
  }

  return (
    <>
      <JsonLd data={breadcrumbJsonLd([{ name: "실적 서프라이즈", path: "/earnings/surprises" }])} />
      <PageScrollShell contentClassName="space-y-5">
        <header className="space-y-1">
          <h1 className="text-page font-semibold text-[var(--text-primary)]">실적 서프라이즈 종목</h1>
          <p className="text-cell text-[var(--text-muted)]">
            최근 90일 발표 중 예상(컨센서스)을 가장 크게 넘거나 밑돈 종목입니다. 국내는 영업이익, 미국은 EPS 기준이고, 주가 반응은
            발표 다음 거래일 종가 기준입니다. 다가오는 발표는{" "}
            <Link href="/earnings/domestic" prefetch={false} className="text-[var(--brand)] hover:underline">
              국내
            </Link>
            {" · "}
            <Link href="/earnings/us" prefetch={false} className="text-[var(--brand)] hover:underline">
              미국 실적 일정
            </Link>
            에서 볼 수 있습니다.
          </p>
        </header>
        {sections.map((m) => (
          <section key={m.id} className="space-y-3">
            <h2 className="text-base font-semibold text-[var(--text-primary)]">{m.label}</h2>
            <div className="space-y-1.5">
              <h3 className="text-cell font-semibold text-[var(--profit)]">예상을 넘은 발표</h3>
              <SurpriseTable rows={m.best} />
            </div>
            <div className="space-y-1.5">
              <h3 className="text-cell font-semibold text-[var(--loss)]">예상을 밑돈 발표</h3>
              <SurpriseTable rows={m.worst} />
            </div>
          </section>
        ))}
        <p className="text-micro text-[var(--text-muted)]">
          예상치 출처: 국내는 FnGuide 잠정실적 컨센서스(연결 우선, 연결이 없으면 개별), 미국은 Finnhub. 투자 권유가 아닙니다.
        </p>
      </PageScrollShell>
    </>
  );
}
