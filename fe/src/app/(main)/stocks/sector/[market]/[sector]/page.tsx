import Link from "next/link";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { JsonLd } from "@/components/seo/json-ld";
import { ScoredStockTable, ScoreListFootnote } from "@/components/stocks/scored-stock-table";
import { breadcrumbJsonLd, pageMetadata } from "@/lib/seo";
import { assetTypeOfSegment, comparePath, groupBySector, MARKET_SEGMENT_LABEL, sectorPath } from "@/lib/stock-seo";
import { fetchIndexableScored } from "@/lib/stock-seo.server";

/*
 * 업종별 AI 점수 순위 — `/stocks/sector/kr/IT` (2026-10-10 사업화 검토: 「반도체 저평가 종목」류 롱테일 검색).
 * 서버가 표를 그린다. 대상은 색인 대상(시장별 거래대금 상위 500) 안의 그 업종 종목뿐이다.
 */
type Props = { params: Promise<{ market: string; sector: string }> };

async function load(market: string, rawSector: string) {
  const assetType = assetTypeOfSegment(market);
  if (!assetType) return null;
  const sector = decodeURIComponent(rawSector);
  const all = await fetchIndexableScored();
  const rows = all.filter((s) => s.assetType === assetType && s.sectorLabel === sector).sort((a, b) => b.score - a.score);
  if (rows.length === 0) return null;
  const groups = groupBySector(all, assetType);
  /*
    사이트맵이 싣는 「업종 대표 비교」(그룹 1위 × 2~5위)를 여기서 잇는다. 전에는 사이트맵으로만
    발견돼 자체 크롤이 고아 19건으로 잡았다(2026-10-21 티켓 QP-07). 같은 `groupBySector` 순서를 쓴다.
  */
  const group = groups.find((g) => g.sector === sector);
  const pairs = group ? group.stocks.slice(1, 5).map((o) => ({ a: group.stocks[0], b: o })) : [];
  return { assetType, sector, rows, pairs, others: groups.filter((g) => g.sector !== sector && g.sector !== "기타") };
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { market, sector } = await params;
  const data = await load(market, sector);
  if (!data) return { title: "찾을 수 없는 업종", robots: { index: false, follow: true } };
  const label = MARKET_SEGMENT_LABEL[market];
  const top = data.rows.slice(0, 3).map((r) => r.name).join(", ");
  return pageMetadata({
    title: `${label} ${data.sector} 업종 AI 점수 순위`,
    description: `${label} ${data.sector} 업종 ${data.rows.length}종목을 수익성·성장·가치·추세·수급 다섯 축 AI 점수로 줄 세웠습니다. 상위: ${top}. 매일 갱신, 투자 권유 아님.`,
    path: sectorPath(data.assetType, data.sector),
  });
}

export default async function SectorRankingPage({ params }: Props) {
  const { market, sector } = await params;
  const data = await load(market, sector);
  if (!data) notFound();
  const label = MARKET_SEGMENT_LABEL[market];
  const path = sectorPath(data.assetType, data.sector);

  return (
    <>
      <JsonLd
        data={breadcrumbJsonLd([
          { name: "종목 모음", path: "/stocks" },
          { name: `${label} ${data.sector}`, path },
        ])}
      />
      <PageScrollShell contentClassName="space-y-4">
        <header className="space-y-1">
          <h1 className="text-page font-semibold text-[var(--text-primary)]">
            {label} {data.sector} 업종 AI 점수 순위
          </h1>
          <p className="text-cell text-[var(--text-muted)]">
            {data.rows.length}종목 · 1위 {data.rows[0].name}({Math.round(data.rows[0].score)}점). 이름을 누르면 점수의 근거와 바뀐
            이유를 볼 수 있습니다.
          </p>
        </header>
        <ScoredStockTable rows={data.rows} />
        <ScoreListFootnote />
        {data.pairs.length > 0 && (
          <nav aria-label="업종 대표 비교" className="space-y-1.5">
            <h2 className="text-cell font-semibold text-[var(--text-secondary)]">{data.sector} 대표 종목 비교</h2>
            <ul className="flex flex-wrap gap-1.5">
              {data.pairs.map(({ a, b }) => (
                <li key={b.symbol}>
                  <Link
                    href={comparePath(a.symbol, b.symbol)}
                    prefetch={false}
                    className="inline-flex rounded-full border border-[var(--border)] px-2 py-0.5 text-cell text-[var(--text-primary)] hover:bg-[var(--surface-elevated)]"
                  >
                    {a.name} vs {b.name}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        )}
        {data.others.length > 0 && (
          <nav aria-label="다른 업종" className="space-y-1.5">
            <h2 className="text-cell font-semibold text-[var(--text-secondary)]">{label} 다른 업종</h2>
            <ul className="flex flex-wrap gap-1.5">
              {data.others.map((g) => (
                <li key={g.sector}>
                  <Link
                    href={sectorPath(data.assetType, g.sector)}
                    prefetch={false}
                    className="inline-flex rounded-full border border-[var(--border)] px-2 py-0.5 text-cell text-[var(--text-primary)] hover:bg-[var(--surface-elevated)]"
                  >
                    {g.sector}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        )}
      </PageScrollShell>
    </>
  );
}
