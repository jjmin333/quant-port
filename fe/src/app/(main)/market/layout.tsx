import { pageMetadata } from "@/lib/seo";

/**
 * `/market`의 `page.tsx`는 클라이언트 컴포넌트라 `metadata`를 내보낼 수 없다
 * (Next 규칙). 그래서 이 레이아웃이 대신 들고 있는다 — 화면은 그대로 통과시킨다.
 */
export const metadata = pageMetadata({
  title: "실시간 차트",
  description:
    "국내외 종목의 실시간 시세와 차트를 봅니다. 거래대금·등락률 등으로 정렬한 목록에서 종목을 고르면 같은 화면에서 차트가 열립니다.",
  path: "/market",
});

export default function MarketLayout({ children }: { children: React.ReactNode }) {
  return children;
}
