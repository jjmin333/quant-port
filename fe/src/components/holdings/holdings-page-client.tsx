"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { MenuEmptyState } from "@/components/common/menu-empty-state";
import { menuIcon } from "@/components/common/menu-icon";
import {
  SampleModeButton,
  SampleRibbon,
  useSampleMode,
} from "@/components/common/sample-mode";
import { useAuth } from "@/components/providers/auth-provider";
import { PageHeader } from "@/components/layout/header";
import { PageColumns } from "@/components/layout/page-columns";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { HoldingDetailModal } from "@/components/holdings/holding-detail-modal";
import { HoldingsAllocationCard } from "@/components/holdings/holdings-allocation-card";
import { HoldingsContributionCard } from "@/components/holdings/holdings-contribution-card";
import { HoldingsList, HoldingsToolbar } from "@/components/holdings/holdings-list";
import { HoldingsScoreCard } from "@/components/holdings/holdings-score-card";
import { HoldingsSideRail } from "@/components/holdings/holdings-side-rail";
import { HoldingsSummaryCard } from "@/components/holdings/holdings-summary-card";
import { HoldingsValuationChart } from "@/components/holdings/holdings-valuation-chart";
import { Card } from "@/components/ui/card";
import { useExchangeRate } from "@/hooks/use-exchange-rate";
import {
  deleteUserHoldingClient,
  fetchUserHoldingsClient,
} from "@/lib/holdings-import-api";
import {
  buildHoldingMetrics,
  computeHoldingsTotals,
  mergeCashIntoTotals,
  withWeights,
  type HoldingRow,
} from "@/lib/holdings-calc";
import { apiRequest } from "@/lib/api-client";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import { MENU_COPY } from "@/lib/menu-copy";
import { fetchSampleHoldings } from "@/lib/sample-api";
import { mergeQuoteSummariesForSymbols } from "@/lib/merge-quotes";
import { fetchScoresBatchClient, type StockScoreData } from "@/lib/stock-score-api";
import type { QuoteSummary } from "@/lib/quotes-types";
import type { UserHolding } from "@/lib/user-holdings-types";
import {
  EMPTY_CASH_BALANCES,
  type UserCashBalances,
} from "@/lib/user-cash-balances-types";
import { fetchUserCashBalancesClient } from "@/lib/user-cash-balances-api";

/** 참조가 매번 바뀌면 하위 memo가 전부 무효가 된다 — 빈 맵은 하나만 만든다. */
const EMPTY_SCORES = new Map<string, StockScoreData>();

/** 관심종목과 동일 — 평가손익·오늘 손익이 장중 시세를 따라가도록 주기 갱신. */
const QUOTES_POLL_MS = 5_000;

/**
 * 보유종목 화면.
 *
 * 배치 원칙: **합계는 본문 위, 목록은 가운데, 내 맥락은 레일**.
 * 레일은 `xl`(1280px) 아래에서 본문 밑으로 쌓이므로, 총 평가금액처럼 제일 먼저 보는
 * 값을 레일에 두면 좁은 화면에서 맨 아래로 내려간다.
 *
 * 블록마다 자기 로딩·빈 상태를 갖는다 — 점수 배치가 실패해도 목록과 손익은 그대로
 * 보여야 한다.
 */
const COPY = MENU_COPY.holdings;

export function HoldingsPageClient() {
  const { session } = useAuth();
  const { active: sample, setActive: setSample } = useSampleMode();
  // `null` = 아직 한 번도 못 받음. 이걸 로딩 표시로 삼으면 새로고침(매매 기록 추가 등)
  // 때 목록이 잠깐 비지 않는다 — 방금 추가한 종목을 보려는 참에 화면이 깜빡이면 안 된다.
  const [holdings, setHoldings] = useState<UserHolding[] | null>(null);
  const [cashBalances, setCashBalances] = useState<UserCashBalances | null>(null);
  const [quotes, setQuotes] = useState<QuoteSummary[]>([]);
  // 점수는 "어떤 심볼 집합에 대한 결과인지"와 함께 들고 있는다 — 별도 loading 플래그를
  // 이펙트에서 동기로 켜면 캐스케이드 렌더가 된다.
  const [scoreState, setScoreState] = useState<{
    key: string;
    map: Map<string, StockScoreData>;
  } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  // 매매 이력이 바뀌면 평가금액 추이를 다시 읽어야 한다.
  const [tradesVersion, setTradesVersion] = useState(0);
  const [reloadKey, setReloadKey] = useState(0);

  const { activeQuery, mode, isSearchActive } = useHeaderSearch();
  const stockFilter = mode === "stock" && isSearchActive ? activeQuery : "";
  const exchangeRate = useExchangeRate(true);

  /*
   * 목록 새로고침은 **버전 번호를 올려** 이펙트를 다시 돌리는 방식이다.
   * `load()` 같은 함수를 이펙트에서 부르면 그 안의 setState가 이펙트 본문에서 동기로
   * 실행되는 것으로 취급돼(캐스케이드 렌더) 린트가 막는다. 상태 변경을 전부 프라미스
   * 콜백 안으로 몰면 그 문제가 사라지고, 취소 처리도 `alive` 하나로 끝난다.
   */
  const reload = useCallback(() => setReloadKey((k) => k + 1), []);

  const refreshQuotes = useCallback(async (symbols: string[]) => {
    if (symbols.length === 0) {
      setQuotes([]);
      return;
    }
    const batch = await fetchQuotesBatchClient(symbols);
    setQuotes((prev) => mergeQuoteSummariesForSymbols(symbols, prev, batch ?? []));
  }, []);

  // 예시 모드는 보유·현금·시세 세 요청(전부 인증 뒤에 있다)을 한 픽스처로 대신한다.
  useEffect(() => {
    if (!sample) return;
    let alive = true;
    fetchSampleHoldings()
      .then((rows) => {
        if (!alive) return;
        setError(null);
        setHoldings(
          rows.map((r) => ({
            id: r.id,
            symbol: r.symbol,
            name: r.name,
            quantity: r.quantity,
            avgPrice: r.avgPrice,
            costCurrency: r.costCurrency,
            source: r.source,
            updatedAt: r.updatedAt,
          })) as UserHolding[],
        );
        setCashBalances({ krw: 3_200_000, usd: 5_000 } as UserCashBalances);
        setQuotes(
          rows.map((r) => {
            const prevClose =
              r.changeRate === -100 ? r.price : r.price / (1 + r.changeRate / 100);
            return {
              symbol: r.symbol,
              name: r.name,
              currentPrice: r.price,
              changeRate: r.changeRate,
              prevClose,
              // 변화폭까지 채워야 한다 — 목록이 등락률과 금액을 함께 그린다.
              change: r.price - prevClose,
              currency: r.currency === "USD" ? "USD" : "KRW",
            };
          }) as unknown as QuoteSummary[],
        );
      })
      .catch(() => {
        if (!alive) return;
        setHoldings([]);
        setCashBalances(EMPTY_CASH_BALANCES);
      });
    return () => {
      alive = false;
    };
  }, [sample]);

  useEffect(() => {
    if (sample) return;
    let alive = true;
    Promise.all([fetchUserHoldingsClient(), fetchUserCashBalancesClient()])
      .then(async ([userRows, cash]) => {
        if (!alive) return;
        const nextHoldings = userRows ?? [];
        const symbols = nextHoldings.map((h) => h.symbol);
        setError(null);
        setHoldings(nextHoldings);
        setCashBalances(cash ?? EMPTY_CASH_BALANCES);
        if (symbols.length === 0) {
          setQuotes([]);
          return;
        }
        try {
          await refreshQuotes(symbols);
        } catch {
          // 목록은 이미 반영됐고, 시세는 이전 값·폴링으로 회복한다.
        }
      })
      .catch((err: unknown) => {
        if (!alive) return;
        setError(err instanceof Error ? err.message : "불러오기 실패");
        // 실패해도 로딩 상태에 갇히지 않게 빈 목록으로 확정한다. 오류 배너가 이유를 말한다.
        setHoldings((prev) => prev ?? []);
        setCashBalances((prev) => prev ?? EMPTY_CASH_BALANCES);
      });
    return () => {
      alive = false;
    };
  }, [sample, reloadKey, refreshQuotes]);

  const holdingSymbols = useMemo(
    () => (holdings ?? []).map((h) => h.symbol),
    [holdings],
  );

  useEffect(() => {
    // 예시에서는 실시간 시세를 덮어쓰지 않는다. 평단은 픽스처 값에 맞춰 정해둔 것이라
    // 현재가만 실제 시세로 갈아끼우면 평가손익이 +231% 같은 엉뚱한 숫자가 된다.
    if (sample || holdingSymbols.length === 0) return;
    const id = window.setInterval(() => {
      void refreshQuotes(holdingSymbols).catch(() => {
        // 폴링 실패는 조용히 — mergeQuoteSummary가 마지막 시세를 유지한다.
      });
    }, QUOTES_POLL_MS);
    return () => window.clearInterval(id);
  }, [sample, holdingSymbols, refreshQuotes]);

  const loading = holdings === null || cashBalances === null;
  const isEmpty = !sample && !loading && !error && (holdings?.length ?? 0) === 0;

  // 점수는 목록과 **따로** 불러온다. 배치 API가 죽어도 손익은 보여야 한다.
  const scoreKey = (holdings ?? []).map((h) => h.symbol).join(",");
  const scores = scoreState?.key === scoreKey ? scoreState.map : EMPTY_SCORES;
  const scoresLoading = Boolean(scoreKey) && scoreState?.key !== scoreKey;

  useEffect(() => {
    if (!scoreKey) return;
    let alive = true;
    fetchScoresBatchClient(scoreKey.split(","))
      .then((rows) => {
        if (!alive) return;
        setScoreState({
          key: scoreKey,
          map: new Map(rows.map((r) => [r.symbol.toUpperCase(), r])),
        });
      })
      .catch(() => {
        if (alive) setScoreState({ key: scoreKey, map: new Map() });
      });
    return () => {
      alive = false;
    };
  }, [scoreKey]);

  const rows: HoldingRow[] = useMemo(() => {
    const quoteMap = new Map((quotes ?? []).map((q) => [q.symbol.toUpperCase(), q]));
    return (holdings ?? []).map((h) => ({
      ...h,
      quote: quoteMap.get(h.symbol.toUpperCase()),
    }));
  }, [holdings, quotes]);

  const { metrics, totals } = useMemo(() => {
    const base = buildHoldingMetrics(rows, exchangeRate);
    const computed = mergeCashIntoTotals(
      computeHoldingsTotals(base, exchangeRate),
      cashBalances ?? EMPTY_CASH_BALANCES,
      exchangeRate,
    );
    return { metrics: withWeights(base, computed), totals: computed };
  }, [rows, exchangeRate, cashBalances]);

  // 레일의 일정·뉴스는 상위 몇 종목만 조회하므로 비중 순서가 중요하다.
  const byWeight = useMemo(
    () => [...metrics].sort((a, b) => (b.marketValueKRW ?? 0) - (a.marketValueKRW ?? 0)),
    [metrics],
  );

  const selectedMetrics = useMemo(
    () => metrics.find((m) => m.symbol === selected) ?? null,
    [metrics, selected],
  );

  /**
   * 수량·평단 직접 수정. 백엔드에 단건 수정 엔드포인트가 없어 **목록 전체를 PUT** 한다.
   * 종목 수가 수십 개 수준이라 비용은 문제없지만, 다른 탭에서 동시에 고치면 나중 저장이
   * 이긴다. 그래서 저장 직후 목록을 다시 읽어 서버 상태를 화면의 진실로 삼는다.
   */
  const saveHolding = useCallback(
    async (symbol: string, quantity: number, avgPrice: number | null) => {
      const next = (holdings ?? []).map((h) =>
        h.symbol === symbol
          ? { ...h, quantity, avgPrice: avgPrice ?? undefined, source: "manual" }
          : h,
      );
      await apiRequest("/api/v1/user/holdings", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ holdings: next }),
      });
      reload();
    },
    [holdings, reload],
  );

  const deleteHolding = useCallback(
    async (symbol: string) => {
      await deleteUserHoldingClient(symbol);
      setSelected(null);
      setTradesVersion((v) => v + 1);
      reload();
    },
    [reload],
  );

  const onTradesChanged = useCallback(() => {
    setTradesVersion((v) => v + 1);
    reload();
  }, [reload]);

  return (
    <>
      <PageHeader
        title={COPY.title}
        subtitle={COPY.subtitle}
        actions={
          <SampleModeButton
            active={sample}
            onToggle={setSample}
            emphasize={isEmpty || !session}
          />
        }
      />
      <PageScrollShell>
        {sample && (
          <div className="mb-3">
            <SampleRibbon
              loggedIn={Boolean(session)}
              loginHref="/login?next=%2Fholdings"
              onDismiss={() => setSample(false)}
            />
          </div>
        )}

        {isEmpty ? (
          <MenuEmptyState
            icon={menuIcon("holdings")}
            headline={COPY.emptyHeadline}
            description={COPY.emptyDescription}
            steps={COPY.steps}
            primary={{ label: "종목 찾아보기", href: "/analysis" }}
            secondary={
              <SampleModeButton
                active={false}
                onToggle={() => setSample(true)}
                emphasize
                className="border border-[var(--sample-border)] px-3.5 py-1.5"
              />
            }
            footnote={COPY.emptyFootnote}
          />
        ) : (
        <PageColumns
          rail={
            <HoldingsSideRail
              metrics={byWeight}
              onAdded={onTradesChanged}
              exchangeRate={exchangeRate}
            />
          }
        >
          <div className="space-y-3">
            {error && (
              <Card className="border-[var(--danger)]/40 bg-[var(--danger)]/10 p-3 text-xs">
                {error}
              </Card>
            )}

            <HoldingsSummaryCard
              totals={loading ? null : totals}
              loading={loading}
              exchangeRate={exchangeRate}
              onChanged={onTradesChanged}
            />

            <div className="grid gap-3 2xl:grid-cols-2">
              <HoldingsAllocationCard metrics={metrics} totals={loading ? null : totals} />
              <HoldingsContributionCard
                metrics={metrics}
                totals={loading ? null : totals}
                onSelect={setSelected}
              />
            </div>

            <HoldingsScoreCard
              metrics={metrics}
              totals={loading ? null : totals}
              scores={scores}
              loading={loading || scoresLoading}
              onSelect={setSelected}
            />

            {/*
              평가금액 추이는 매매 이력에서 만든다 — 예시에는 이력이 없고 API도 인증
              뒤에 있어 "authentication required"만 뜬다. 빈 카드를 세우느니 감춘다.
            */}
            {!sample && (
              <HoldingsValuationChart
                reloadKey={tradesVersion}
                untrackedHoldings={metrics
                  .filter((m) => m.quantity > 0)
                  .map((m) => ({
                    symbol: m.symbol,
                    name: m.name,
                    quantity: m.quantity,
                    avgPrice: m.avgPrice,
                    currency: m.currency,
                  }))}
                onTradesChanged={onTradesChanged}
              />
            )}

            <div className="space-y-2">
              <HoldingsToolbar count={metrics.length} />
              <HoldingsList
                metrics={metrics}
                loading={loading}
                filterQuery={stockFilter}
                exchangeRate={exchangeRate}
                onSelect={setSelected}
              />
            </div>
          </div>
        </PageColumns>
        )}
      </PageScrollShell>

      <HoldingDetailModal
        metrics={selectedMetrics}
        score={selectedMetrics ? scores.get(selectedMetrics.symbol.toUpperCase()) : null}
        open={selected != null}
        onClose={() => setSelected(null)}
        onSaveHolding={saveHolding}
        onDeleteHolding={deleteHolding}
        onTradesChanged={onTradesChanged}
      />
    </>
  );
}
