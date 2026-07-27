"use client";

import { useEffect, useState } from "react";
import { PageHeader } from "@/components/layout/header";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { MarketIndicesBar } from "@/components/quotes/market-indices-bar";
import { QuotesDetailPanel } from "@/components/quotes/quotes-detail-panel";
import {
  QuotesSymbolList,
  QuotesSymbolListFooter,
} from "@/components/quotes/quotes-symbol-list";
import { fetchQuotesClient } from "@/lib/market-api";

export default function QuotesPage() {
  const [selectedSymbol, setSelectedSymbol] = useState("005930");
  const { activeQuery, mode } = useHeaderSearch();
  const quoteSearch = mode === "quotes" ? activeQuery : "";

  useEffect(() => {
    fetchQuotesClient("all")
      .then((items) => {
        if (items.length > 0) {
          setSelectedSymbol((prev) =>
            items.some((i) => i.symbol === prev) ? prev : items[0].symbol,
          );
        }
      })
      .catch(() => {
        // keep default
      });
  }, []);

  return (
    <>
      <PageHeader title="실시간 시세" subtitle="현재가 · 호가 · 체결 · 차트" />
      <MarketIndicesBar />
      <main className="flex min-h-0 flex-1 overflow-hidden">
        <div className="flex min-w-[var(--dashboard-content-min)] flex-1 overflow-x-auto">
          <div className="flex h-full min-w-[720px] flex-1">
            <div className="flex h-full shrink-0 flex-col">
              <QuotesSymbolList
                selectedSymbol={selectedSymbol}
                onSelect={setSelectedSymbol}
                searchQuery={quoteSearch}
              />
              <QuotesSymbolListFooter symbol={selectedSymbol} />
            </div>
            <QuotesDetailPanel symbol={selectedSymbol} />
          </div>
        </div>
      </main>
    </>
  );
}
