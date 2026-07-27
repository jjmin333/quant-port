import { PageHeader } from "@/components/layout/header";
import { StockDetailClient } from "@/components/stocks/stock-detail-client";

type Props = { params: Promise<{ symbol: string }> };

export default async function StockDetailPage({ params }: Props) {
  const { symbol } = await params;

  return (
    <>
      <PageHeader title={symbol} subtitle="종목 상세" />
      <main className="flex-1 overflow-y-auto p-6">
        <StockDetailClient symbol={symbol} />
      </main>
    </>
  );
}
