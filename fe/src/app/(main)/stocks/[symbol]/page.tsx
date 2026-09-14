import { Suspense } from "react";
import { StockWorkspacePageLoader } from "./stock-workspace-page-client";
import { pageMetadata } from "@/lib/seo";

type Props = { params: Promise<{ symbol: string }> };

/*
 * **색인하지 않는다.** 이 화면은 종목 수만큼(수천 개) 주소가 생기는데 내용은
 * 전부 클라이언트에서 받아 그린다 — 크롤러가 받는 HTML에는 종목 이야기가 한 줄도
 * 없다. 그런 주소 수천 개가 색인되면 사이트 전체가 얇은 문서 더미로 평가된다.
 * 서버에서 종목 내용을 그리게 되면 그때 색인 대상으로 올린다(`docs/SEO.md`).
 */
export const metadata = pageMetadata({
  title: "종목",
  description: "종목의 시세·차트·점수·뉴스를 한 화면에서 봅니다.",
  noindex: true,
});

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
