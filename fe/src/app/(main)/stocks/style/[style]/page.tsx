import Link from "next/link";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { JsonLd } from "@/components/seo/json-ld";
import { ScoredStockTable, ScoreListFootnote } from "@/components/stocks/scored-stock-table";
import { breadcrumbJsonLd, pageMetadata } from "@/lib/seo";
import { stylePath } from "@/lib/stock-seo";
import { fetchIndexableScored } from "@/lib/stock-seo.server";
import { STOCK_STYLES, stockStyle, stockStyleBySlug } from "@/lib/stock-style";

/*
 * 스타일 이름표별 목록 — `/stocks/style/quality-growth` (2026-10-10, Stockopedia StockRank Style 참고).
 * 이름표는 축 조합의 묘사다(`lib/stock-style.ts`). 시장별로 점수 순.
 */
type Props = { params: Promise<{ style: string }> };

const MARKETS = [
  { assetType: "domestic", label: "국내" },
  { assetType: "us", label: "미국" },
] as const;

async function load(slug: string) {
  const style = stockStyleBySlug(slug);
  if (!style) return null;
  const rows = (await fetchIndexableScored())
    .filter((s) => stockStyle(s.axes)?.slug === slug)
    .sort((a, b) => b.score - a.score);
  return { style, rows };
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { style } = await params;
  const data = await load(style);
  if (!data) return { title: "찾을 수 없는 스타일", robots: { index: false, follow: true } };
  return pageMetadata({
    title: `${data.style.label} 종목 — 국내·미국 AI 점수`,
    description: `${data.style.description} ${data.rows.length}개를 국내·미국으로 나눠 AI 점수 순으로 모았습니다. 매일 갱신, 투자 권유 아님.`,
    path: stylePath(data.style.slug),
  });
}

export default async function StyleListPage({ params }: Props) {
  const { style } = await params;
  const data = await load(style);
  if (!data) notFound();

  return (
    <>
      <JsonLd
        data={breadcrumbJsonLd([
          { name: "종목 모음", path: "/stocks" },
          { name: data.style.label, path: stylePath(data.style.slug) },
        ])}
      />
      <PageScrollShell contentClassName="space-y-4">
        <header className="space-y-1">
          <h1 className="text-page font-semibold text-[var(--text-primary)]">{data.style.label} 종목</h1>
          <p className="text-cell text-[var(--text-muted)]">
            {data.style.description}. 사라·팔라는 뜻이 아니라 다섯 축이 이렇게 생겼다는 묘사입니다.
          </p>
        </header>
        {data.rows.length === 0 && <p className="text-cell text-[var(--text-muted)]">오늘은 이 조건에 맞는 종목이 없습니다.</p>}
        {MARKETS.map((m) => {
          const rows = data.rows.filter((r) => r.assetType === m.assetType);
          if (rows.length === 0) return null;
          return (
            <section key={m.assetType} className="space-y-1.5">
              <h2 className="text-base font-semibold text-[var(--text-primary)]">
                {m.label} <span className="text-cell font-normal text-[var(--text-muted)]">({rows.length})</span>
              </h2>
              <ScoredStockTable rows={rows} showStyle={false} />
            </section>
          );
        })}
        <ScoreListFootnote />
        <nav aria-label="다른 스타일" className="space-y-1.5">
          <h2 className="text-cell font-semibold text-[var(--text-secondary)]">다른 스타일</h2>
          <ul className="flex flex-wrap gap-1.5">
            {STOCK_STYLES.filter((s) => s.slug !== data.style.slug).map((s) => (
              <li key={s.slug}>
                <Link
                  href={stylePath(s.slug)}
                  prefetch={false}
                  className="inline-flex rounded-full border border-[var(--border)] px-2 py-0.5 text-cell text-[var(--text-primary)] hover:bg-[var(--surface-elevated)]"
                >
                  {s.label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
      </PageScrollShell>
    </>
  );
}
