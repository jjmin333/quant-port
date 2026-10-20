"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { MenuEmptyState } from "@/components/common/menu-empty-state";
import { AddHoldingDialog } from "@/components/holdings/add-holding-dialog";
import { menuIcon } from "@/components/common/menu-icon";
import {
  SampleModeButton,
  SampleRibbon,
  useAutoSample,
  useSampleMode,
} from "@/components/common/sample-mode";
import { useAuth } from "@/components/providers/auth-provider";
import { PageColumns } from "@/components/layout/page-columns";
import { PageActions, PageScrollShell } from "@/components/layout/page-content-shell";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { HoldingDetailModal } from "@/components/holdings/holding-detail-modal";
import { HoldingsAllocationCard } from "@/components/holdings/holdings-allocation-card";
import { HoldingsRiskCard } from "@/components/holdings/holdings-risk-card";
import { HoldingsContributionCard } from "@/components/holdings/holdings-contribution-card";
import { HoldingsDiagnosisCard } from "@/components/holdings/holdings-diagnosis-card";
import { HoldingsList, HoldingsToolbar } from "@/components/holdings/holdings-list";
import { HoldingsScoreCard } from "@/components/holdings/holdings-score-card";
import { HoldingsSideRail } from "@/components/holdings/holdings-side-rail";
import { HoldingsSummaryCard } from "@/components/holdings/holdings-summary-card";
import { HoldingsValuationChart } from "@/components/holdings/holdings-valuation-chart";
import { NetWorthCard, useWealth, WealthAssetsSection, type AddKind } from "@/components/holdings/holdings-wealth-section";
import { AssetFormDialog, LiabilityFormDialog } from "@/components/wealth/wealth-forms";
import type { AssetType } from "@/lib/wealth-api";
import { ASSET_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { Card } from "@/components/ui/card";
import { useExchangeRate } from "@/hooks/use-exchange-rate";
import { useHoldingsDiagnosis } from "@/hooks/use-holdings-diagnosis";
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
import {
  applyStreamPricesToQuotes,
  mergeQuoteSummariesForSymbols,
} from "@/lib/merge-quotes";
import { useQuoteStreamByMarket } from "@/hooks/use-quote-stream";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { PRICE_POLL_MS, PRICE_POLL_MS_WHILE_STREAMING } from "@/lib/price-cadence";
import { fetchScoresBatchClient, type StockScoreData } from "@/lib/stock-score-api";
import type { QuoteSummary } from "@/lib/quotes-types";
import type { UserHolding } from "@/lib/user-holdings-types";
import {
  EMPTY_CASH_BALANCES,
  type UserCashBalances,
} from "@/lib/user-cash-balances-types";
import { fetchUserCashBalancesClient } from "@/lib/user-cash-balances-api";
import { toUserMessage } from "@/lib/error-message";

/** 참조가 매번 바뀌면 하위 memo가 전부 무효가 된다 — 빈 맵은 하나만 만든다. */
const EMPTY_SCORES = new Map<string, StockScoreData>();

/*
관심종목과 동일 — 평가손익·오늘 손익이 장중 시세를 따라가도록 주기 갱신.

**5초에서 2초로 맞췄다 (2026-09-04 사용자 요청 1).** 같은 가격이 화면마다 다른
박자로 늙는 것을 없애는 것이 목적이라, 숫자는 이제 `lib/price-cadence` 한 곳에만
있다. 요청 수는 오히려 줄었다 — 아래 SSE가 붙어 있는 동안 이 폴링은 30초로
물러나고, 끊겼을 때만 2초로 돌아온다.
*/
const QUOTES_POLL_MS = PRICE_POLL_MS;

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
    보유 이유 점검은 **로그인한 실제 보유**에만 묻는다.

    예시 모드의 보유는 매매 이력이 없어 보유 기간이 없고, 보유 기간이 없으면 실적·
    내부자를 자를 기준이 없다 — 남는 근거가 점수 하나뿐이라 판정이 언제나 「판단
    보류」가 된다. 예시로 둘러보는 사람에게 보류만 다섯 줄 보여 줄 이유가 없다.

    `tradesVersion`을 키로 쓰는 이유는 매매가 바뀌면 그 보유 기간이 바뀌기 때문이다.
  */
  const diagnosis = useHoldingsDiagnosis(tradesVersion, Boolean(session) && !sample);

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

  /*
   * 모드가 바뀌면 이전 모드의 데이터를 **먼저 버린다.**
   *
   * 아래 두 적재 효과는 성공했을 때만 값을 갈아끼우고, 실패 경로는
   * `setHoldings((prev) => prev ?? [])`로 **이전 값을 유지**한다(폴링이 잠깐
   * 실패해도 목록이 사라지지 않게 하려는 의도이고, 같은 모드 안에서는 그게 맞다).
   * 그런데 모드를 넘나들 때는 그 유지가 곧 **예시 숫자가 실데이터인 척 남는 것**이
   * 된다 — 비로그인으로 예시를 끄면 실데이터 요청이 전부 401이라 예시 포트폴리오가
   * 그대로 화면에 남았다. 모드 전환은 "값이 아직 없음"에서 다시 시작해야 한다.
   *
   * effect가 아니라 **렌더 중에** 되돌린다(React의 "prop이 바뀔 때 state 조정"
   * 패턴). effect로 하면 초기화 전에 한 프레임이 먼저 그려져 이전 모드의 숫자가
   * 잠깐 보이는데, 이 화면에서 그 한 프레임이 정확히 고치려는 증상이다.
   */
  const [renderedSample, setRenderedSample] = useState(sample);
  if (renderedSample !== sample) {
    setRenderedSample(sample);
    setHoldings(null);
    setCashBalances(null);
    setQuotes([]);
    setError(null);
  }

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
            sector: r.sector,
            sectorLabel: r.sectorLabel,
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
        setError(toUserMessage(err, "불러오기 실패"));
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

  /*
    가격을 푸시로도 받는다 (2026-09-04 사용자 요청 1).

    구독 대상을 **시세 목록**(`quotes`)에서 만드는 이유는 자산 구분이 거기 있기
    때문이다. 보유 목록은 한 사람 안에 국내·해외가 섞여 있고, 서버는 그 값으로
    어느 거래소 달력을 볼지 정한다 — `useQuoteStreamByMarket`이 시장별로 나눠
    연결한다(그 머리글 참조).

    예시 모드에서는 붙지 않는다. 그 숫자는 픽스처라 스트림이 덮으면 평단과 현재가가
    서로 다른 세계의 값이 된다.
  */
  const streamRows = useMemo(
    () =>
      sample
        ? []
        : quotes.map((q) => ({ symbol: q.symbol, assetType: q.assetType as string })),
    [quotes, sample],
  );
  const { prices: streamPrices, streaming } = useQuoteStreamByMarket(streamRows);

  /*
    스트림 가격은 **상태에 저장하지 않고 렌더 중에 얹는다.**

    이펙트에서 `setQuotes`를 하면 틱마다 렌더가 두 번 돈다 — 이 화면은 그 목록으로
    평가손익·비중·기여도를 다시 계산하므로 두 배가 그대로 비용이다. 아래 계산들이
    전부 `liveQuotes`를 읽는다.
  */
  const liveQuotes = useMemo(
    () => applyStreamPricesToQuotes(quotes, streamPrices),
    [quotes, streamPrices],
  );

  /*
   * 예시도 실시간 시세를 받는다. 예전에는 막아뒀는데, 그때는 평단이 손으로 적은
   * 값이라 현재가만 진짜로 갈아끼우면 평가손익이 +231% 같은 숫자가 됐기 때문이다.
   * 지금은 평단이 **매수일의 실제 종가**라 두 끝이 같은 세계에 있다.
   *
   * 주기 폴링은 **보이는 동안만** 돈다 (2026-09-15 사용자 요청 3). 예전에는 숨은
   * 탭도 보유 종목 전부를 계속 당겼고, 창을 다섯 띄우면 그대로 다섯 배였다.
   * 첫 적재는 위 `load()`가 하므로 여기는 되풀이만 맡는다.
   */
  useVisibleInterval(
    () => {
      void refreshQuotes(holdingSymbols).catch(() => {
        // 폴링 실패는 조용히 — mergeQuoteSummary가 마지막 시세를 유지한다.
      });
    },
    streaming ? PRICE_POLL_MS_WHILE_STREAMING : QUOTES_POLL_MS,
    holdingSymbols.length > 0,
  );

  const loading = holdings === null || cashBalances === null;
  /*
    주식 밖의 자산·부채(2026-10-01 — 순자산 메뉴를 이 화면으로 합침). 예시 여부와 상관없이 로그인이면 읽는다 —
    아래 「아직 없다」 판정에 들어가야 집만 등록한 사람이 예시 화면으로 넘어가지 않는다.
  */
  // 부동산·금융상품 비노출(lib/hidden-features, 2026-10-10)이면 자산·부채 칸을 통째로 끈다 — 이 화면은 주식만.
  const wealthOn = Boolean(session) && !ASSET_FEATURES_HIDDEN;
  const wealth = useWealth(wealthOn);
  const wealthEmpty = !wealthOn || (wealth.data != null && wealth.data.assets.length === 0 && wealth.data.liabilities.length === 0);
  const wealthLoading = wealthOn && wealth.data == null;
  const [assetAdd, setAssetAdd] = useState<AssetType | null>(null);
  const [liabAdd, setLiabAdd] = useState(false);
  /*
    「내 것이 아직 없다」 — **`sample`을 빼고** 잰다 (2026-09-14 사용자 요청 4).
    `useAutoSample`이 이 값으로 예시를 켜는데, `!sample`이 섞이면 켜짐 → 0건 아님 →
    꺼짐이 무한히 반복된다. 화면이 안내 카드를 세우는 조건은 그 위에 `!sample`을
    얹은 `isEmpty`다 — 예시를 **끈** 사람에게만 카드가 보인다.
  */
  const hasNothingYet = !loading && !wealthLoading && !error && (holdings?.length ?? 0) === 0 && wealthEmpty;
  useAutoSample(loading || wealthLoading ? null : hasNothingYet);
  const isEmpty = !sample && hasNothingYet;
  // 주식은 없고 부동산·금융상품만 있는 사람 — 빈 주식 분석 카드 대신 자산 목록을 위로(2026-10-02).
  const stocksEmpty = !sample && !loading && (holdings?.length ?? 0) === 0;
  /** 0건 안내에서 여는 추가 폼. 사이드 레일이 없는 상태의 유일한 진입점이다. */
  const [addOpen, setAddOpen] = useState(false);
  const onAdd = (kind: AddKind) => {
    if (kind === "stock") setAddOpen(true);
    else if (kind === "liability") setLiabAdd(true);
    else setAssetAdd(kind);
  };
  const wealthSection = !sample && wealthOn && (
    <WealthAssetsSection wealth={wealth.data} rates={wealth.rates} onChanged={wealth.refresh} onAdd={onAdd} />
  );

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
    // 스트림 가격이 얹힌 목록을 쓴다 — 평가손익·오늘 손익이 지금 값을 따라간다.
    const quoteMap = new Map(liveQuotes.map((q) => [q.symbol.toUpperCase(), q]));
    return (holdings ?? []).map((h) => ({
      ...h,
      quote: quoteMap.get(h.symbol.toUpperCase()),
    }));
  }, [holdings, liveQuotes]);

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
      <PageScrollShell>
        {/*
          0건 안내 카드가 이미 같은 버튼을 들고 있다 (§R-2). 두 곳에 다 두면 한 화면에
          「예시로 둘러보기」가 둘이 되어 무엇이 다른지 묻게 만든다 — 실측으로 빈
          보유 화면의 보이는 버튼이 그 문구 ×2 였다.
        */}
        <PageActions>
          {isEmpty ? null : (
            <SampleModeButton active={sample} onToggle={setSample} emphasize={!session} />
          )}
        </PageActions>
        {sample && (
          <div className="mb-3">
            <SampleRibbon
              loggedIn={Boolean(session)}
              loginHref="/login?next=%2Fholdings"
              onDismiss={() => setSample(false)}
              // 안내 카드가 들고 있던 첫 동작 — 카드가 예시 뒤로 물러났으니 여기 선다.
              action={{ label: "종목 추가하기", onClick: () => setAddOpen(true) }}
            />
          </div>
        )}

        {isEmpty ? (
          <MenuEmptyState
            icon={menuIcon("holdings")}
            headline={COPY.emptyHeadline}
            description={COPY.emptyDescription}
            steps={COPY.steps}
            /*
              예전 CTA는 `/analysis`(AI 추천 랭킹)로 보냈다. 그런데 이 카드가 약속한
              「종목을 추가한다 — 직접 입력하거나 잔고 화면을 캡처해 올리면…」의 두
              진입점은 사이드 레일 안에 있고, 그 레일은 지금 이 0건 분기에서 렌더되지
              않는다. 결국 첫 사용자는 어디로도 갈 수 없었다. 약속한 그 폼을 바로 연다.
            */
            primary={{ label: "종목 추가하기", onClick: () => setAddOpen(true) }}
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
        ) : null}
        {isEmpty && wealthOn && (
          <div className="mt-3 flex flex-wrap items-center justify-center gap-2 text-cell">
            <span className="text-[var(--text-muted)]">주식 말고도:</span>
            {([["real_estate", "부동산"], ["deposit", "예금"], ["saving", "적금"], ["crypto", "코인"], ["gold", "금"]] as const).map(([k, label]) => (
              <button key={k} type="button" onClick={() => setAssetAdd(k)}
                className="h-7 rounded-full border px-2.5 text-[var(--text-secondary)] hover:bg-muted">+ {label}</button>
            ))}
            <button type="button" onClick={() => setLiabAdd(true)} className="h-7 rounded-full border px-2.5 text-[var(--text-secondary)] hover:bg-muted">+ 대출</button>
          </div>
        )}
        {isEmpty ? null : (
        <PageColumns
          rail={
            <HoldingsSideRail
              metrics={byWeight}
              onAdded={onTradesChanged}
              exchangeRate={exchangeRate}
              sample={sample}
            />
          }
        >
          <div className="space-y-3">
            {error && (
              <Card className="border-[var(--danger)]/40 bg-[var(--danger)]/10 p-3 text-cell">
                {error}
              </Card>
            )}

            {!sample && wealthOn && (
              <NetWorthCard metrics={metrics} totals={loading ? null : totals} wealth={wealth.data} onAdd={onAdd} />
            )}
            {stocksEmpty && wealthSection}

            <HoldingsSummaryCard
              totals={loading ? null : totals}
              loading={loading}
              exchangeRate={exchangeRate}
              onChanged={onTradesChanged}
            />

            {/*
              「얼마 벌었나」 다음이 「왜 들고 있나」다 (2026-09-18 사용자 결정 A).
              모바일에서도 두 번째 카드라 스크롤 없이 닿는다.
            */}
            {!stocksEmpty && (<>
            <HoldingsDiagnosisCard
              data={diagnosis.data}
              loading={diagnosis.loading}
              error={diagnosis.error}
            />

            <div className="grid gap-3 2xl:grid-cols-2">
              <HoldingsAllocationCard metrics={metrics} totals={loading ? null : totals} />
              {/* 위험 — 자산 배분 옆(좁은 화면에선 아래) (2026-09-24 사용자 결정). */}
              {!loading && <HoldingsRiskCard metrics={metrics} />}
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
            </>)}

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

            {!stocksEmpty && wealthSection}
          </div>
        </PageColumns>
        )}
        <AddHoldingDialog open={addOpen} onOpenChange={setAddOpen} onAdded={onTradesChanged} />
        {assetAdd && (
          <AssetFormDialog key={assetAdd} open prefill={{ assetType: assetAdd }} onClose={() => setAssetAdd(null)} onSaved={wealth.refresh} />
        )}
        {liabAdd && (
          <LiabilityFormDialog open assets={wealth.data?.assets ?? []} onClose={() => setLiabAdd(false)} onSaved={wealth.refresh} />
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
