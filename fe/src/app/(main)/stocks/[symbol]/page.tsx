import { Suspense } from "react";
import type { Metadata } from "next";
import { StockWorkspacePageLoader } from "./stock-workspace-page-client";
import { resolvePageMetadata } from "@/lib/page-meta.server";
import { OG_BASE, TWITTER_BASE } from "@/lib/seo";
import { sectorPeers, stockPageTitle, stockSummarySentence } from "@/lib/stock-seo";
import { fetchIndexableStocks, fetchStockSeoDetails, fetchStockSeoSummary } from "@/lib/stock-seo.server";
import { StockSeoSummaryBar } from "@/components/stocks/stock-seo-summary-bar";

type Props = { params: Promise<{ symbol: string }> };

/*
 * **상위 1,000종목만 색인한다** (2026-09-24 사용자 결정 · 10-09부터 국내 500 + 미국 500).
 *
 * 예전에는 전부 noindex였다 — 내용을 전부 클라이언트에서 받아 그려 크롤러가 받는 HTML에 종목
 * 이야기가 한 줄도 없었고, 그런 주소 수천 개는 사이트 전체를 「얇은 문서 더미」로 만든다.
 * 이제 서버가 **요약 한 줄**(이름 · 시장 · 섹터 · 점수 · 종가 · 최근 변화)을 그리고, 최근 20거래일
 * 거래대금 상위 1,000종목에만 색인·canonical을 준다(목록은 BE `/seo/stocks`, 한 시간 캐시).
 * 나머지는 같은 한 줄을 그리되 noindex 그대로다 — 화면이 종목마다 달라지지 않게.
 *
 * 제목·설명의 기본값은 `lib/page-meta-routes`이고 관리자가 `/admin/seo`에서 덮어쓸 수 있다.
 * 색인 종목은 그 위에 종목 이름으로 된 제목·설명을 얹는다.
 */
export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { symbol } = await params;
  const base = await resolvePageMetadata("/stocks/[symbol]");
  const summary = await fetchStockSeoSummary(decodeURIComponent(symbol));
  if (!summary) return base;

  const title = stockPageTitle(summary);
  const description = stockSummarySentence(summary);
  if (!summary.indexable) return { ...base, title, description };

  const path = `/stocks/${encodeURIComponent(summary.symbol)}`;
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { robots: _noindex, ...indexable } = base;
  return {
    ...indexable,
    title,
    description,
    alternates: { canonical: path },
    // 그림은 싣지 않는다 — 같은 폴더의 `opengraph-image.tsx`(종목 레이더 카드)가 채운다(2026-10-10).
    // 공통 그림(`OG_BASE.images`)을 함께 넘기면 그쪽이 이겨 모든 종목이 사이트 공통 카드로 나갔다.
    openGraph: { ...ogNoImage, title, description, url: path },
    twitter: { ...twitterNoImage, title, description },
  };
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
const { images: _ogImages, ...ogNoImage } = OG_BASE;
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const { images: _twImages, ...twitterNoImage } = TWITTER_BASE;

export default async function StockWorkspacePage({ params }: Props) {
  const { symbol } = await params;
  // generateMetadata와 같은 주소·같은 revalidate라 Next가 한 번만 부른다.
  const summary = await fetchStockSeoSummary(decodeURIComponent(symbol));
  // 요약 바 재료 — 요약이 없으면(모르는 종목·BE 실패) 부르지 않는다. 모두 1시간 캐시.
  const details = summary ? await fetchStockSeoDetails(summary.symbol) : null;
  const peers = summary ? sectorPeers(await fetchIndexableStocks(), summary) : [];
  return (
    <>
      {summary && (
        /*
          서버가 그리는 요약 — 크롤러(JS를 안 도는 Yeti 포함)가 받는 HTML의 본문이다.
          접힌 바 한 줄이 예전 요약 한 줄 자리이고, 펼치면 지표·최근 변화·같은 섹터 종목이 나온다.
        */
        <StockSeoSummaryBar
          summary={summary}
          qa={details?.qa ?? null}
          fundamentals={details?.fundamentals ?? null}
          earnings={details?.earnings ?? null}
          overview={details?.overview ?? null}
          peers={peers}
        />
      )}
      <Suspense
        fallback={
          <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
            종목 정보를 불러오는 중…
          </main>
        }
      >
        <StockWorkspacePageLoader params={params} initialName={summary?.name} />
      </Suspense>
    </>
  );
}
