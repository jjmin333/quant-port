"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { PageHeader } from "@/components/layout/header";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { FundamentalsSection } from "@/components/analysis/fundamentals-section";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { fetchFundamentalsClient } from "@/lib/fundamentals-api";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import {
  fetchStockClient,
  searchStocksCatalogClient,
} from "@/lib/market-api";
import type { BriefingEvent, DashboardInsights } from "@/lib/insights-types";
import { formatDateKST, formatListDate } from "@/lib/format";
import type { StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";
import { uniqueStocksBySymbol } from "@/lib/stock-utils";

type StockRow = StockInfo & { fetchError?: string };

type AnalysisClientProps = {
  holdings: StockRow[];
  initialInsights: DashboardInsights | null;
  initialDisclosures: BriefingEvent[];
};

type SidebarTab = "holdings" | "search";

const SECTIONS = [
  { id: "fundamentals", label: "펀더멘털" },
  { id: "earnings", label: "실적" },
  { id: "news", label: "뉴스" },
  { id: "disclosures", label: "공시" },
  { id: "ai-analysis", label: "AI 분석" },
  { id: "analyst", label: "애널리스트" },
] as const;

export function AnalysisClient({
  holdings,
  initialInsights,
  initialDisclosures,
}: AnalysisClientProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const symbolParam = searchParams.get("symbol");
  const { activeQuery, mode, isSearchActive } = useHeaderSearch();

  const [sidebarTab, setSidebarTab] = useState<SidebarTab>("holdings");
  const [selectedSymbol, setSelectedSymbol] = useState<string | null>(
    symbolParam ?? holdings[0]?.symbol ?? null,
  );
  const [selectedStock, setSelectedStock] = useState<StockRow | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<StockInfo[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [fundamentals, setFundamentals] = useState<FundamentalsData | null>(null);
  const [fundamentalsLoading, setFundamentalsLoading] = useState(false);
  const [insights] = useState(initialInsights);
  const [disclosures] = useState(initialDisclosures);

  useEffect(() => {
    if (symbolParam) {
      setSelectedSymbol(symbolParam);
      setSidebarTab(holdings.some((h) => h.symbol === symbolParam) ? "holdings" : "search");
    }
  }, [symbolParam, holdings]);

  useEffect(() => {
    if (!selectedSymbol) {
      setSelectedStock(null);
      return;
    }

    const holding = holdings.find((h) => h.symbol === selectedSymbol);
    if (holding) {
      setSelectedStock(holding);
      return;
    }

    let cancelled = false;
    fetchStockClient(selectedSymbol)
      .then((stock) => {
        if (!cancelled) setSelectedStock(stock);
      })
      .catch(() => {
        if (!cancelled) {
          setSelectedStock({
            symbol: selectedSymbol,
            name: selectedSymbol,
            market: "—",
            currency: "USD",
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [selectedSymbol, holdings]);

  useEffect(() => {
    if (!selectedSymbol) {
      setFundamentals(null);
      return;
    }

    let cancelled = false;
    setFundamentalsLoading(true);
    fetchFundamentalsClient(selectedSymbol)
      .then((data) => {
        if (!cancelled) setFundamentals(data);
      })
      .catch(() => {
        if (!cancelled) setFundamentals(null);
      })
      .finally(() => {
        if (!cancelled) setFundamentalsLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [selectedSymbol]);

  useEffect(() => {
    if (mode !== "stock") return;
    setSearchQuery(activeQuery);
    if (activeQuery.trim()) {
      setSidebarTab("search");
    }
  }, [activeQuery, mode]);

  useEffect(() => {
    const q = searchQuery.trim();
    if (q.length < 1) {
      setSearchResults([]);
      return;
    }

    const timer = setTimeout(async () => {
      setSearchLoading(true);
      try {
        const res = await searchStocksCatalogClient(q, "", 20);
        if (res.items.length > 0) {
          setSearchResults(uniqueStocksBySymbol(res.items));
          return;
        }
        const symbolCandidate = q.trim().toUpperCase();
        if (/^[A-Z0-9.-]{1,12}$/.test(symbolCandidate)) {
          try {
            const stock = await fetchStockClient(symbolCandidate);
            setSearchResults([stock]);
            return;
          } catch {
            // fall through
          }
        }
        setSearchResults([]);
      } catch {
        setSearchResults([]);
      } finally {
        setSearchLoading(false);
      }
    }, 250);

    return () => clearTimeout(timer);
  }, [searchQuery]);

  const symbolEarnings = useMemo(
    () =>
      selectedSymbol
        ? (insights?.briefing.earnings ?? []).filter((e) => e.symbol === selectedSymbol)
        : [],
    [insights, selectedSymbol],
  );

  const symbolDisclosures = useMemo(
    () =>
      selectedSymbol
        ? disclosures.filter((d) => d.symbol === selectedSymbol)
        : [],
    [disclosures, selectedSymbol],
  );

  const filteredHoldings = useMemo(() => {
    const base =
      mode !== "stock" || !isSearchActive
        ? holdings
        : (() => {
            const q = activeQuery.trim().toLowerCase();
            if (!q) return holdings;
            return holdings.filter(
              (h) =>
                h.symbol.toLowerCase().includes(q) ||
                h.name.toLowerCase().includes(q),
            );
          })();
    return uniqueStocksBySymbol(base);
  }, [holdings, activeQuery, isSearchActive, mode]);

  function selectSymbol(symbol: string, tab: SidebarTab) {
    setSelectedSymbol(symbol);
    setSidebarTab(tab);
    router.replace(`/analysis?symbol=${encodeURIComponent(symbol)}`, { scroll: false });
  }

  return (
    <>
      <PageHeader
        title="종목 분석"
        subtitle="펀더멘털 · 실적 · 공시 · AI 분석을 한 화면에서 확인"
      />
      <main className="flex-1 overflow-y-auto p-4 sm:p-6">
        <div className="mx-auto flex min-w-0 max-w-6xl flex-col gap-6 xl:flex-row xl:items-start">
          <Card className="min-w-0 flex-1 overflow-hidden p-0 xl:max-w-sm xl:shrink-0">
            <div className="flex border-b border-[var(--border)]">
              <SidebarTabButton
                active={sidebarTab === "holdings"}
                onClick={() => setSidebarTab("holdings")}
                label="보유"
                count={filteredHoldings.length}
              />
              <SidebarTabButton
                active={sidebarTab === "search"}
                onClick={() => setSidebarTab("search")}
                label="검색"
              />
            </div>

            {sidebarTab === "holdings" ? (
              <HoldingsList
                holdings={filteredHoldings}
                selectedSymbol={selectedSymbol}
                onSelect={(symbol) => selectSymbol(symbol, "holdings")}
              />
            ) : (
              <SearchPanel
                query={searchQuery}
                onQueryChange={setSearchQuery}
                loading={searchLoading}
                results={searchResults}
                selectedSymbol={selectedSymbol}
                onSelect={(symbol) => selectSymbol(symbol, "search")}
              />
            )}
          </Card>

          <div className="min-w-0 flex-1 space-y-6">
            {!selectedStock ? (
              <Card className="flex min-h-[320px] items-center justify-center p-8 text-center text-sm text-[var(--text-muted)]">
                왼쪽에서 종목을 선택하거나 검색해 주세요.
              </Card>
            ) : (
              <>
                <Card className="sticky top-0 z-10 border-[var(--border)] bg-[var(--surface)]/95 p-3 backdrop-blur">
                  <div className="mb-2">
                    <h2 className="text-base font-semibold text-[var(--text-primary)]">
                      {selectedStock.name}
                    </h2>
                    <p className="text-xs text-[var(--text-muted)]">
                      {selectedStock.symbol} · {selectedStock.market} · {selectedStock.currency}
                    </p>
                  </div>
                  <nav className="flex flex-wrap gap-1.5">
                    {SECTIONS.map((section) => (
                      <a
                        key={section.id}
                        href={`#${section.id}`}
                        className="rounded-md border border-[var(--border)] px-2.5 py-1 text-xs text-[var(--text-secondary)] hover:border-[var(--accent)] hover:text-[var(--accent)]"
                      >
                        {section.label}
                      </a>
                    ))}
                  </nav>
                </Card>

                <section id="fundamentals" className="scroll-mt-28">
                  <FundamentalsSection
                    stock={selectedStock}
                    data={fundamentals}
                    loading={fundamentalsLoading}
                  />
                </section>

                <AnalysisSection id="earnings" title="실적 캘린더" badge={`${symbolEarnings.length}건`}>
                  {symbolEarnings.length === 0 ? (
                    <p className="text-sm text-[var(--text-muted)]">
                      {selectedStock.symbol} 관련 예정 실적이 없습니다.
                    </p>
                  ) : (
                    <ul className="space-y-2">
                      {symbolEarnings.map((item) => (
                        <li
                          key={`${item.date}-${item.event}`}
                          className="flex items-center justify-between gap-3 rounded-lg bg-[var(--surface-elevated)] px-4 py-3"
                        >
                          <p className="text-sm text-[var(--text-secondary)]">{item.event}</p>
                          <time className="shrink-0 text-xs tabular-nums text-[var(--text-muted)]">
                            {formatDateKST(item.date)}
                          </time>
                        </li>
                      ))}
                    </ul>
                  )}
                </AnalysisSection>

                <AnalysisSection id="news" title="뉴스" badge="준비 중">
                  <p className="text-sm text-[var(--text-secondary)]">
                    {selectedStock.name}({selectedStock.symbol}) 뉴스 API 연동 예정입니다. 종목
                    상세 페이지에서 시세와 차트를 확인할 수 있습니다.
                  </p>
                  <Link
                    href={`/stocks/${selectedStock.symbol}`}
                    className="mt-3 inline-flex text-sm text-[var(--accent)] hover:underline"
                  >
                    종목 상세 보기 →
                  </Link>
                </AnalysisSection>

                <AnalysisSection id="disclosures" title="공시" badge={`${symbolDisclosures.length}건`}>
                  {symbolDisclosures.length === 0 ? (
                    <p className="text-sm text-[var(--text-muted)]">
                      오늘 등록된 {selectedStock.symbol} 공시가 없습니다.
                    </p>
                  ) : (
                    <ul className="space-y-2">
                      {symbolDisclosures.map((item) => (
                        <li
                          key={`${item.date}-${item.event}`}
                          className="rounded-lg bg-[var(--surface-elevated)] px-4 py-3"
                        >
                          <p className="text-sm text-[var(--text-secondary)]">{item.event}</p>
                          <time className="mt-1 block text-xs tabular-nums text-[var(--text-muted)]">
                            {formatDateKST(item.date)}
                          </time>
                        </li>
                      ))}
                    </ul>
                  )}
                </AnalysisSection>

                <AnalysisSection
                  id="ai-analysis"
                  title="AI 분석"
                  badge={insights?.briefing.source === "llm" ? "AI" : "규칙"}
                >
                  {insights ? (
                    <div className="space-y-4">
                      <p className="text-sm leading-relaxed text-[var(--text-secondary)]">
                        {insights.briefing.summary}
                      </p>
                      {insights.briefing.highlights.length > 0 && (
                        <ul className="space-y-2">
                          {insights.briefing.highlights.slice(0, 5).map((item) => (
                            <li
                              key={item}
                              className="rounded-lg bg-[var(--surface-elevated)] px-4 py-3 text-sm text-[var(--text-secondary)]"
                            >
                              {item}
                            </li>
                          ))}
                        </ul>
                      )}
                      <p className="text-xs text-[var(--text-muted)]">
                        생성 시각 {formatDateKST(insights.briefing.generatedAt)} · 포트폴리오
                        브리핑 기준
                      </p>
                    </div>
                  ) : (
                    <p className="text-sm text-[var(--text-muted)]">AI 브리핑을 불러오지 못했습니다.</p>
                  )}
                </AnalysisSection>

                <AnalysisSection id="analyst" title="애널리스트 의견" badge="준비 중">
                  <p className="text-sm text-[var(--text-secondary)]">
                    애널리스트 컨센서스·목표가 데이터 연동 예정입니다.
                  </p>
                </AnalysisSection>
              </>
            )}
          </div>
        </div>
      </main>
    </>
  );
}

function SidebarTabButton({
  active,
  onClick,
  label,
  count,
}: {
  active: boolean;
  onClick: () => void;
  label: string;
  count?: number;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex-1 px-4 py-3 text-sm font-medium transition-colors",
        active
          ? "border-b-2 border-[var(--accent)] text-[var(--accent)]"
          : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
      )}
    >
      {label}
      {count != null ? ` (${count})` : ""}
    </button>
  );
}

function HoldingsList({
  holdings,
  selectedSymbol,
  onSelect,
}: {
  holdings: StockRow[];
  selectedSymbol: string | null;
  onSelect: (symbol: string) => void;
}) {
  if (holdings.length === 0) {
    return (
      <p className="p-5 text-sm text-[var(--text-muted)]">
        보유 종목이 없습니다. 검색 탭에서 종목을 찾아보세요.
      </p>
    );
  }

  return (
    <div className="max-h-[480px] overflow-y-auto">
      <table className="w-full text-sm">
        <thead className="sticky top-0 bg-[var(--surface)]">
          <tr className="text-left text-xs text-[var(--text-muted)]">
            <th className="px-4 py-2 font-medium">종목</th>
            <th className="hidden px-2 py-2 font-medium sm:table-cell">시장</th>
          </tr>
        </thead>
        <tbody>
          {holdings.map((stock) => (
            <tr
              key={stock.symbol}
              className={cn(
                "cursor-pointer border-t border-[var(--border)] transition-colors",
                stock.symbol === selectedSymbol
                  ? "bg-[var(--accent)]/10"
                  : "hover:bg-[var(--surface-elevated)]/60",
              )}
              onClick={() => onSelect(stock.symbol)}
            >
              <td className="px-4 py-3">
                <p className="font-medium text-[var(--text-primary)]">{stock.name}</p>
                <p className="text-xs text-[var(--text-muted)]">{stock.symbol}</p>
              </td>
              <td className="hidden px-2 py-3 text-xs text-[var(--text-secondary)] sm:table-cell">
                {stock.market}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function SearchPanel({
  query,
  onQueryChange,
  loading,
  results,
  selectedSymbol,
  onSelect,
}: {
  query: string;
  onQueryChange: (value: string) => void;
  loading: boolean;
  results: StockInfo[];
  selectedSymbol: string | null;
  onSelect: (symbol: string) => void;
}) {
  return (
    <div className="p-4">
      <input
        type="search"
        value={query}
        onChange={(e) => onQueryChange(e.target.value)}
        placeholder="종목명 또는 심볼 검색"
        className="w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-[var(--accent)]"
      />
      <p className="mt-2 text-xs text-[var(--text-muted)]">
        DB 종목 + Toss API로 ETF 등 미등록 종목도 검색됩니다.
      </p>
      <div className="mt-3 max-h-[400px] overflow-y-auto">
        {loading && <p className="text-sm text-[var(--text-muted)]">검색 중…</p>}
        {!loading && query.trim() && results.length === 0 && (
          <p className="text-sm text-[var(--text-muted)]">검색 결과가 없습니다.</p>
        )}
        <ul className="space-y-1">
          {results.map((stock) => (
            <li key={stock.symbol}>
              <button
                type="button"
                onClick={() => onSelect(stock.symbol)}
                className={cn(
                  "flex w-full items-center justify-between rounded-lg px-3 py-2 text-left transition-colors",
                  stock.symbol === selectedSymbol
                    ? "bg-[var(--accent)]/10"
                    : "hover:bg-[var(--surface-elevated)]",
                )}
              >
                <div>
                  <p className="text-sm font-medium text-[var(--text-primary)]">{stock.name}</p>
                  <p className="text-xs text-[var(--text-muted)]">
                    {stock.symbol}
                    {stock.securityType ? ` · ${stock.securityType}` : ""}
                  </p>
                </div>
                <span className="text-xs text-[var(--text-muted)]">{stock.market}</span>
              </button>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}

function AnalysisSection({
  id,
  title,
  badge,
  children,
}: {
  id: string;
  title: string;
  badge?: string;
  children: ReactNode;
}) {
  return (
    <section id={id} className="scroll-mt-28">
      <Card className="p-5">
        <CardHeader className="mb-4 px-0 pt-0">
          <CardTitle>{title}</CardTitle>
          {badge ? <Badge variant="info">{badge}</Badge> : null}
        </CardHeader>
        {children}
      </Card>
    </section>
  );
}
