import Link from "next/link";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ScoreRadar } from "@/components/analysis/score-radar";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { JsonLd } from "@/components/seo/json-ld";
import { ScoreListFootnote } from "@/components/stocks/scored-stock-table";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import { breadcrumbJsonLd, NOINDEX, pageMetadata } from "@/lib/seo";
import type { ScoreBreakdown } from "@/lib/stock-score-api";
import { comparePath, parseComparePair, sectorPath, stylePath, type StockSeoSummary } from "@/lib/stock-seo";
import { fetchStockSeoDetails, fetchStockSeoSummary } from "@/lib/stock-seo.server";
import { stockStyle } from "@/lib/stock-style";

/*
 * 두 종목 비교 — `/compare/000660-vs-005930` (2026-10-10 사업화 검토: 「삼성전자 vs SK하이닉스」류 검색).
 * 레이더 두 개를 겹치고 같은 줄에 지표를 놓는다. 주소는 정렬된 쌍 하나뿐(`comparePath`) — 반대 순서로
 * 들어오면 정식 주소로 canonical. 둘 중 하나라도 색인 대상이 아니면 noindex.
 */
type Props = { params: Promise<{ pair: string }> };

type Side = { s: StockSeoSummary; f: FundamentalsData | null; overview: string | null };

async function load(pair: string): Promise<[Side, Side] | null> {
  const syms = parseComparePair(pair);
  if (!syms) return null;
  const sides: Side[] = [];
  for (const sym of syms) {
    const s = await fetchStockSeoSummary(sym);
    if (!s) return null;
    const d = await fetchStockSeoDetails(s.symbol);
    sides.push({ s, f: d.fundamentals, overview: d.overview });
  }
  return [sides[0], sides[1]];
}

function breakdown(s: StockSeoSummary): ScoreBreakdown {
  return Object.fromEntries(Object.entries(s.axes ?? {}).filter(([, v]) => typeof v === "number")) as ScoreBreakdown;
}

function num(v: number | undefined | null, digits: number, suffix: string, signed = false): string {
  if (v == null || !Number.isFinite(v)) return "–";
  return `${signed && v > 0 ? "+" : ""}${v.toFixed(digits)}${suffix}`;
}

function price(s: StockSeoSummary): string {
  if (s.close == null) return "–";
  return s.currency === "USD"
    ? `$${s.close.toLocaleString("en-US", { maximumFractionDigits: 2 })}`
    : `${Math.round(s.close).toLocaleString("ko-KR")}원`;
}

const AXES = [
  ["technical", "추세"],
  ["quality", "수익성"],
  ["growth", "성장"],
  ["valuation", "가치(쌀수록 높음)"],
  ["sentiment", "심리·수급"],
] as const;

function rows(a: Side, b: Side): [string, string, string][] {
  const axis = (x: Side, k: string) => {
    const v = x.s.axes?.[k];
    return typeof v === "number" ? `${Math.round(v)}` : "–";
  };
  const per = (x: Side) => (x.f?.per != null && x.f.per > 0 ? num(x.f.per, 1, "배") : "–");
  return [
    ["AI 종합 점수", ...([a, b].map((x) => (x.s.score == null ? "–" : `${Math.round(x.s.score)}${x.s.grade ? ` (${x.s.grade})` : ""}`)) as [string, string])],
    ...AXES.map(([k, label]) => [label, axis(a, k), axis(b, k)] as [string, string, string]),
    ["스타일", ...([a, b].map((x) => stockStyle(x.s.axes)?.label ?? "–") as [string, string])],
    ["업종", a.s.sectorLabel ?? "–", b.s.sectorLabel ?? "–"],
    ["종가", price(a.s), price(b.s)],
    ["등락률", num(a.s.changeRate, 2, "%", true), num(b.s.changeRate, 2, "%", true)],
    ["PER", per(a), per(b)],
    ["PBR", num(a.f?.pbr, 2, "배"), num(b.f?.pbr, 2, "배")],
    ["ROE", num(a.f?.roe, 1, "%"), num(b.f?.roe, 1, "%")],
    ["영업이익률", num(a.f?.operatingMargin, 1, "%"), num(b.f?.operatingMargin, 1, "%")],
    ["매출 성장(전년)", num(a.f?.revenueGrowthYoy, 1, "%", true), num(b.f?.revenueGrowthYoy, 1, "%", true)],
  ];
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { pair } = await params;
  const data = await load(pair);
  if (!data) return { title: "찾을 수 없는 비교", robots: { index: false, follow: true } };
  const [a, b] = data;
  const meta = pageMetadata({
    title: `${a.s.name} vs ${b.s.name} — AI 점수·재무 비교`,
    description: `${a.s.name}(${a.s.symbol})와 ${b.s.name}(${b.s.symbol})를 AI 점수 다섯 축(추세·수익성·성장·가치·수급)과 PER·PBR·ROE·영업이익률로 나란히 비교합니다. 투자 권유 아님.`,
    path: comparePath(a.s.symbol, b.s.symbol),
  });
  return a.s.indexable && b.s.indexable ? meta : { ...meta, robots: NOINDEX };
}

export default async function ComparePage({ params }: Props) {
  const { pair } = await params;
  const data = await load(pair);
  if (!data) notFound();
  const [a, b] = data;
  const path = comparePath(a.s.symbol, b.s.symbol);

  return (
    <>
      <JsonLd
        data={breadcrumbJsonLd([
          { name: "종목 모음", path: "/stocks" },
          { name: `${a.s.name} vs ${b.s.name}`, path },
        ])}
      />
      <PageScrollShell contentClassName="space-y-4">
        <header className="space-y-1">
          <h1 className="text-page font-semibold text-[var(--text-primary)]">
            {a.s.name} vs {b.s.name}
          </h1>
          <p className="text-cell text-[var(--text-muted)]">
            AI 점수 다섯 축과 재무 지표를 나란히 놓았습니다. 실선이 {a.s.name}, 점선이 {b.s.name}입니다.
          </p>
        </header>

        <div className="flex flex-wrap items-center gap-4">
          <ScoreRadar breakdown={breakdown(a.s)} compare={breakdown(b.s)} className="h-48 w-56" />
          <ul className="space-y-1 text-cell">
            {[a, b].map((x, i) => (
              <li key={x.s.symbol} className="flex items-center gap-2">
                <span
                  aria-hidden
                  className="inline-block w-6 border-t-2"
                  style={{ borderColor: i === 0 ? "var(--brand)" : "var(--text-secondary)", borderStyle: i === 0 ? "solid" : "dashed" }}
                />
                <Link href={`/stocks/${encodeURIComponent(x.s.symbol)}`} prefetch={false} className="font-medium text-[var(--text-primary)] hover:underline">
                  {x.s.name}
                </Link>
                <span className="text-micro text-[var(--text-muted)]">{x.s.symbol}</span>
              </li>
            ))}
          </ul>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full min-w-[360px] text-cell">
            <thead className="text-micro text-[var(--text-muted)]">
              <tr className="border-b border-[var(--border)] text-left">
                <th className="py-1.5 pr-2 font-normal">항목</th>
                <th className="py-1.5 pr-2 text-right font-normal">{a.s.name}</th>
                <th className="py-1.5 text-right font-normal">{b.s.name}</th>
              </tr>
            </thead>
            <tbody>
              {rows(a, b).map(([label, x, y]) => (
                <tr key={label} className="border-b border-[var(--border)] last:border-0">
                  <td className="py-1.5 pr-2 text-[var(--text-secondary)]">{label}</td>
                  <td className="py-1.5 pr-2 text-right tabular-nums text-[var(--text-primary)]">{x}</td>
                  <td className="py-1.5 text-right tabular-nums text-[var(--text-primary)]">{y}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {[a, b].map((x) =>
          x.overview ? (
            <section key={x.s.symbol} className="space-y-1">
              <h2 className="text-cell font-semibold text-[var(--text-secondary)]">{x.s.name} AI 요약</h2>
              <p className="text-cell leading-relaxed text-[var(--text-secondary)]">{x.overview}</p>
            </section>
          ) : null,
        )}

        <p className="flex flex-wrap gap-x-3 gap-y-1 text-cell">
          {[a, b].filter((x, i, all) => all.findIndex((y) => y.s.sectorLabel === x.s.sectorLabel) === i).map((x) =>
            x.s.sectorLabel ? (
              <Link key={x.s.symbol} href={sectorPath(x.s.assetType, x.s.sectorLabel)} prefetch={false} className="text-[var(--brand)] hover:underline">
                {x.s.sectorLabel} 업종 점수 순위
              </Link>
            ) : null,
          )}
          {[a, b].map((x, i) => {
            const st = stockStyle(x.s.axes);
            if (i === 1 && st && st.slug === stockStyle(a.s.axes)?.slug) return null;
            return st ? (
              <Link key={`st-${x.s.symbol}`} href={stylePath(st.slug)} prefetch={false} className="text-[var(--brand)] hover:underline">
                {st.label} 종목 더 보기
              </Link>
            ) : null;
          })}
        </p>
        <ScoreListFootnote />
      </PageScrollShell>
    </>
  );
}
