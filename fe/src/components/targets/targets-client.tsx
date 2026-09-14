"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { LayoutGrid, List, X } from "lucide-react";
import { PageColumns } from "@/components/layout/page-columns";
import { PageActions, PageScrollShell } from "@/components/layout/page-content-shell";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { PriceHistoryChart } from "@/components/charts/price-history-chart";
import { TargetAxis } from "@/components/targets/target-axis";
import { TargetQuickLookModal } from "@/components/targets/target-quick-look-modal";
import { TargetGapCell, TargetStatusBadge } from "@/components/targets/target-shared";
import { TargetsSideRail } from "@/components/targets/targets-side-rail";
import { MenuEmptyState } from "@/components/common/menu-empty-state";
import { LoadErrorState } from "@/components/common/load-error-state";
import { menuIcon } from "@/components/common/menu-icon";
import {
  SampleModeButton,
  SampleRibbon,
  SampleWrap,
  useAutoSample,
  useSampleMode,
} from "@/components/common/sample-mode";
import { useAuth } from "@/components/providers/auth-provider";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { PriceDisplay } from "@/components/ui/price-display";
import { SectionCard } from "@/components/ui/section-card";
import { useSparklineCandles } from "@/hooks/use-sparkline-candles";
import { useStockScores } from "@/hooks/use-stock-scores";
import { formatPrice } from "@/lib/format";
import { fetchUserHoldingsClient } from "@/lib/holdings-import-api";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import { MENU_COPY } from "@/lib/menu-copy";
import { fetchSampleHoldings, fetchSampleTargets } from "@/lib/sample-api";
import type { QuoteSummary, StockInfo } from "@/lib/quotes-types";
import {
  inferTargetCurrency,
  removePriceTarget,
  savePriceTarget,
  usePriceTargetsState,
  type PriceTarget,
} from "@/lib/price-targets";
import {
  buildTargetView,
  sortTargetViews,
  summarizeTargets,
  TARGET_SORTS,
  type TargetSort,
  type TargetStatus,
  type TargetView,
} from "@/lib/target-stats";
import { setTargetsView, useTargetsView, type TargetsView } from "@/lib/targets-view";
import type { UserHolding } from "@/lib/user-holdings-types";
import { cn } from "@/lib/utils";
import { toast } from "sonner";

const COPY = MENU_COPY.targets;

type StatusFilter = "all" | TargetStatus;

const STATUS_FILTERS: { id: StatusFilter; label: string }[] = [
  { id: "all", label: "전체" },
  { id: "reached", label: "도달" },
  { id: "near", label: "임박" },
  { id: "stop_near", label: "손절 근접" },
  { id: "open", label: "진행 중" },
];

/**
 * 목표가 관리.
 *
 * 예전 행은 「현재가 / 목표가 / 목표까지 +11.4%」 네 숫자였다. 목표가를 관리한다는 건
 * **어디서 출발해 지금 어디쯤 왔는지**, **틀렸을 때 어디서 접을지**, **무엇이 곧 닿는지**를
 * 아는 일인데 그 셋 다 화면에 없었다.
 *
 * 목록 보기는 두 가지다(사용자 결정): 진행축 행(`row`)과 카드 그리드(`card`).
 * 고른 값은 `targets-view.ts`에 남아 다음 방문에도 유지된다.
 */
export function TargetsClient() {
  const { session } = useAuth();
  const { active: sample, setActive: setSample } = useSampleMode();
  const view = useTargetsView();

  /*
   * 저장소를 구독한다(`useSyncExternalStore`) — 보유종목 화면의 같은 카드도 함께 갱신된다.
   * 예시 데이터는 이 스토어를 거치지 않는다: 스토어를 공유하므로 여기에 예시를 밀어
   * 넣으면 다른 화면까지 예시로 물든다.
   */
  const { targets: realTargets, error: loadError, retry: retryLoad } = usePriceTargetsState();
  const [sampleTargets, setSampleTargets] = useState<PriceTarget[]>([]);
  const targets = sample ? sampleTargets : realTargets;

  /*
    **실패는 「없음」이 아니다.** 불러오지 못했을 때 0건 분기로 내려가면 온보딩
    튜토리얼이 뜨고, 목표가를 등록해 둔 사용자는 자기 기록이 지워진 줄 안다.
  */
  const loadFailed = !sample && Boolean(loadError) && realTargets.length === 0;
  /*
    「내 것이 아직 없다」 — **`sample`을 빼고** 잰다 (2026-09-14 사용자 요청 4).
    이 값이 `useAutoSample`로 가서 예시를 켠다. `!sample`이 섞이면 켜짐 → 0건 아님 →
    꺼짐이 무한히 반복된다. 안내 카드는 그 위에 `!sample`을 얹은 `isEmpty`로 선다.
  */
  const hasNothingYet = !loadFailed && realTargets.length === 0;
  useAutoSample(hasNothingYet);
  const isEmpty = !sample && hasNothingYet;
  const [retrying, setRetrying] = useState(false);

  const [stock, setStock] = useState<StockInfo | null>(null);
  const [targetPrice, setTargetPrice] = useState("");
  const [targetPrice2, setTargetPrice2] = useState("");
  const [stopPrice, setStopPrice] = useState("");
  const [note, setNote] = useState("");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [quotes, setQuotes] = useState<Record<string, QuoteSummary>>({});
  const [holdings, setHoldings] = useState<UserHolding[]>([]);
  const [searchFocusKey, setSearchFocusKey] = useState(0);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [sort, setSort] = useState<TargetSort>("near");
  const [openId, setOpenId] = useState<string | null>(null);

  useEffect(() => {
    if (!sample) return;
    let alive = true;
    Promise.all([fetchSampleTargets(), fetchSampleHoldings()])
      .then(([rows, held]) => {
        if (!alive) return;
        setSampleTargets(
          rows.map((r) => ({
            id: String(r.id),
            symbol: r.symbol,
            targetPrice: r.targetPrice,
            targetPrice2: r.targetPrice2,
            stopPrice: r.stopPrice,
            basePrice: r.basePrice,
            baseDate: r.baseDate,
            reachedDate: r.reachedDate,
            peakPrice: r.peakPrice,
            troughPrice: r.troughPrice,
            currency: r.currency === "USD" ? "USD" : "KRW",
            note: r.note,
            createdAt: r.createdAt,
          })),
        );
        // 첫 화면은 서버가 준 시세로 바로 그린다(위 배치 요청이 곧 덮어쓴다).
        // 메모 문구를 「1차 목표 도달 시…」처럼 조건형으로 바꾼 뒤로는 도달 여부가
        // 시세를 따라 바뀌어도 설명과 어긋나지 않는다.
        setQuotes(
          Object.fromEntries(
            rows.map((r) => [
              r.symbol,
              {
                symbol: r.symbol,
                name: r.name,
                currentPrice: r.price,
                changeRate: r.changeRate,
                currency: r.currency === "USD" ? "USD" : "KRW",
              } as unknown as QuoteSummary,
            ]),
          ),
        );
        setHoldings(
          held.map(
            (h) =>
              ({
                symbol: h.symbol,
                name: h.name,
                quantity: h.quantity,
                avgPrice: h.avgPrice,
                costCurrency: h.currency,
              }) as unknown as UserHolding,
          ),
        );
      })
      .catch(() => {
        if (alive) setSampleTargets([]);
      });
    return () => {
      alive = false;
    };
  }, [sample]);

  /*
   * 목표가만 적어 두면 "얼마를 더 가야 하는지"를 매번 딴 화면에서 확인해야 했다.
   * 실패해도 목록은 그대로 뜬다 — 시세는 있으면 좋은 부가 정보이지 목록의 전제가 아니다.
   */
  const symbolKey = useMemo(
    () => [...new Set(targets.map((t) => t.symbol))].sort().join(","),
    [targets],
  );

  useEffect(() => {
    // 예시도 실시간 시세를 받는다 — 목표가·등록가가 등록일의 실제 종가에서 나오므로
    // 현재가만 진짜여도 축이 어긋나지 않는다.
    if (!symbolKey) return;
    let alive = true;
    fetchQuotesBatchClient(symbolKey.split(","))
      .then((rows) => {
        if (alive) setQuotes(Object.fromEntries(rows.map((q) => [q.symbol, q])));
      })
      .catch(() => {
        // 시세 없이도 목표가 목록은 유효하다
      });
    return () => {
      alive = false;
    };
  }, [symbolKey]);

  /*
   * 보유 종목은 레일의 「목표가 없는 보유 종목」·「평단 대비」 두 카드에만 쓴다.
   * 실패해도 조용히 넘어간다 — 목표가 화면이 보유 조회에 매달릴 이유가 없다.
   */
  useEffect(() => {
    if (sample || !session) return;
    let alive = true;
    fetchUserHoldingsClient()
      .then((rows) => {
        if (alive) setHoldings(rows);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [sample, session]);

  const views = useMemo(
    () => targets.map((t) => buildTargetView(t, quotes[t.symbol])),
    [targets, quotes],
  );
  const summary = useMemo(() => summarizeTargets(views), [views]);

  const visible = useMemo(() => {
    const filtered =
      statusFilter === "all" ? views : views.filter((v) => v.status === statusFilter);
    return sortTargetViews(filtered, sort);
  }, [views, statusFilter, sort]);

  // 점수 배지는 퀵룩 모달만 쓴다. 목록 전체를 미리 받아두면 요청 하나가 더 나가고,
  // 정작 배지가 필요한 건 연 종목 하나뿐이다.
  const openView = useMemo(() => views.find((v) => v.target.id === openId) ?? null, [views, openId]);
  const scores = useStockScores(openView ? [openView.target.symbol] : []);

  const resetForm = useCallback(() => {
    setStock(null);
    setTargetPrice("");
    setTargetPrice2("");
    setStopPrice("");
    setNote("");
    setEditingId(null);
  }, []);

  const saveTarget = useCallback(async () => {
    const trimmedSymbol = (stock?.symbol ?? "").trim().toUpperCase();
    const price = Number(targetPrice);
    if (!trimmedSymbol || Number.isNaN(price) || price <= 0) return;
    const currency = inferTargetCurrency(trimmedSymbol);

    // 빈 칸은 0으로 보낸다 — 서버가 NULLIF로 지워, "설정 안 함"과 "0으로 설정"이
    // 섞이지 않는다.
    const second = Number(targetPrice2);
    const stop = Number(stopPrice);

    await savePriceTarget({
      id: editingId ?? undefined,
      symbol: trimmedSymbol,
      targetPrice: price,
      targetPrice2: Number.isFinite(second) && second > 0 ? second : 0,
      stopPrice: Number.isFinite(stop) && stop > 0 ? stop : 0,
      currency,
      note: note.trim(),
    });
    resetForm();
  }, [stock, targetPrice, targetPrice2, stopPrice, note, editingId, resetForm]);

  const startEdit = useCallback(
    (target: PriceTarget) => {
      setEditingId(target.id);
      const quote = quotes[target.symbol];
      setStock({
        symbol: target.symbol,
        name: quote?.name ?? target.symbol,
        currency: target.currency,
        market: /^\d{6}$/.test(target.symbol.trim()) ? "domestic" : "us",
      });
      setTargetPrice(String(target.targetPrice));
      setTargetPrice2(target.targetPrice2 ? String(target.targetPrice2) : "");
      setStopPrice(target.stopPrice ? String(target.stopPrice) : "");
      setNote(target.note);
    },
    [quotes],
  );

  /*
    삭제 뒤 **실행 취소** (§A-2).

    예전에는 「삭제」 한 번에 확인도 복구도 없이 사라졌다. 목표가는 등록 시점 주가
    (`basePrice`·`baseDate`)를 함께 박제하는데 그건 서버가 등록 순간에 잰 값이라
    다시 등록하면 **오늘 값으로 새로 찍힌다** — 「등록 후 등락」이 리셋된다는 뜻이다.
    그래서 되돌리기가 확인 창보다 실질적으로 더 필요한 자리다.
  */
  const removeTarget = useCallback(
    async (id: string) => {
      const removed = realTargets.find((t) => t.id === id);
      await removePriceTarget(id);
      if (editingId === id) resetForm();
      if (!removed) return;
      toast(`${removed.symbol} 목표가를 삭제했습니다.`, {
        action: {
          label: "실행 취소",
          onClick: () => {
            void savePriceTarget({
              symbol: removed.symbol,
              targetPrice: removed.targetPrice,
              targetPrice2: removed.targetPrice2,
              stopPrice: removed.stopPrice,
              currency: removed.currency,
              note: removed.note,
            });
          },
        },
      });
    },
    [editingId, resetForm, realTargets],
  );

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
              loginHref="/login?next=%2Ftargets"
              onDismiss={() => setSample(false)}
              // 안내 카드가 들고 있던 첫 동작 — 카드가 예시 뒤로 물러났으니 여기 선다.
              action={{ label: "종목 찾아보기", href: "/analysis" }}
            />
          </div>
        )}

        {loadFailed ? (
          <LoadErrorState
            message={loadError ?? "목표가를 불러오지 못했습니다."}
            retrying={retrying}
            onRetry={async () => {
              setRetrying(true);
              try {
                await retryLoad();
              } finally {
                setRetrying(false);
              }
            }}
          />
        ) : isEmpty ? (
          <MenuEmptyState
            icon={menuIcon("targets")}
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
              <>
                <TargetForm
                  editingId={editingId}
                  stock={stock}
                  setStock={setStock}
                  targetPrice={targetPrice}
                  setTargetPrice={setTargetPrice}
                  targetPrice2={targetPrice2}
                  setTargetPrice2={setTargetPrice2}
                  stopPrice={stopPrice}
                  setStopPrice={setStopPrice}
                  note={note}
                  setNote={setNote}
                  searchFocusKey={searchFocusKey}
                  onSearchFocus={() => setSearchFocusKey((k) => k + 1)}
                  onSave={saveTarget}
                  onReset={resetForm}
                  currentPrice={stock ? (quotes[stock.symbol]?.currentPrice ?? null) : null}
                />
                <TargetsSideRail
                  views={views}
                  holdings={holdings}
                  onOpenTarget={setOpenId}
                  onAddTarget={(symbol, name) => {
                    resetForm();
                    setStock({
                      symbol,
                      name,
                      currency: inferTargetCurrency(symbol),
                      market: /^\d{6}$/.test(symbol) ? "domestic" : "us",
                    });
                  }}
                />
              </>
            }
          >
            <SampleWrap active={sample}>
              <div className="space-y-3">
                <SummaryTiles
                  summary={summary}
                  active={statusFilter}
                  onPick={(id) => setStatusFilter((cur) => (cur === id ? "all" : id))}
                />

                <div className="flex flex-wrap items-center gap-1.5">
                  {STATUS_FILTERS.map((f) => {
                    const count =
                      f.id === "all"
                        ? views.length
                        : views.filter((v) => v.status === f.id).length;
                    if (f.id !== "all" && count === 0) return null;
                    return (
                      <button
                        key={f.id}
                        type="button"
                        onClick={() => setStatusFilter(f.id)}
                        className={cn(
                          "rounded-full px-2.5 py-1 text-cell font-medium transition-colors",
                          statusFilter === f.id
                            ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                            : "bg-[var(--surface-elevated)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                        )}
                      >
                        {f.label} {count}
                      </button>
                    );
                  })}

                  <div className="ml-auto flex items-center gap-1.5">
                    <div className="flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5">
                      {TARGET_SORTS.map((s) => (
                        <button
                          key={s.id}
                          type="button"
                          onClick={() => setSort(s.id)}
                          className={cn(
                            "rounded-md px-2 py-1 text-cell font-medium transition-colors",
                            sort === s.id
                              ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
                              : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                          )}
                        >
                          {s.label}
                        </button>
                      ))}
                    </div>
                    <ViewToggle view={view} />
                  </div>
                </div>

                <Card className={cn(view === "row" && "p-0")}>
                  {view === "row" && (
                    <CardHeader className="mb-0 border-b border-[var(--border)] px-4 py-3">
                      <CardTitle>목표가 목록</CardTitle>
                      <Badge variant="info">{visible.length}건</Badge>
                    </CardHeader>
                  )}

                  {visible.length === 0 ? (
                    <p className="p-6 text-center text-sm text-[var(--text-muted)]">
                      {views.length === 0
                        ? "등록된 목표가가 없습니다. 오른쪽에서 추가하세요."
                        : "조건에 맞는 목표가가 없습니다."}
                    </p>
                  ) : view === "row" ? (
                    <ul className="divide-y divide-[var(--border)]">
                      {visible.map((v) => (
                        <TargetRow
                          key={v.target.id}
                          view={v}
                          sample={sample}
                          onOpen={() => setOpenId(v.target.id)}
                          onEdit={() => startEdit(v.target)}
                          onRemove={() => removeTarget(v.target.id)}
                        />
                      ))}
                    </ul>
                  ) : (
                    <div className="grid gap-3 sm:grid-cols-2 2xl:grid-cols-3">
                      {visible.map((v) => (
                        <TargetCard
                          key={v.target.id}
                          view={v}
                          onOpen={() => setOpenId(v.target.id)}
                        />
                      ))}
                    </div>
                  )}
                </Card>
              </div>
            </SampleWrap>
          </PageColumns>
        )}

        <TargetQuickLookModal
          view={openView}
          score={openView ? (scores.get(openView.target.symbol) ?? null) : null}
          open={openId !== null}
          readOnly={sample}
          onClose={() => setOpenId(null)}
          onEdit={() => {
            if (openView) startEdit(openView.target);
          }}
          onRemove={() => {
            if (openView) removeTarget(openView.target.id);
          }}
        />
      </PageScrollShell>
    </>
  );
}

/* ── 상단 요약 타일 ──────────────────────────────────────────────────────── */

function SummaryTiles({
  summary,
  active,
  onPick,
}: {
  summary: ReturnType<typeof summarizeTargets>;
  active: StatusFilter;
  onPick: (id: StatusFilter) => void;
}) {
  const tiles: {
    id: StatusFilter;
    label: string;
    value: string;
    foot: string;
    tone?: string;
  }[] = [
    {
      id: "all",
      label: "등록",
      value: String(summary.total),
      foot: `시세 확인 ${summary.quoted}/${summary.total}`,
    },
    {
      id: "reached",
      label: "목표 도달",
      value: String(summary.reached),
      foot: summary.reached > 0 ? "다음 목표를 정할 때" : "아직 없습니다",
      tone: summary.reached > 0 ? "text-emerald-500" : undefined,
    },
    {
      id: "near",
      label: "도달 임박",
      value: String(summary.near),
      foot: "목표까지 5% 이내",
      tone: summary.near > 0 ? "text-amber-500" : undefined,
    },
    {
      id: "open",
      label: "평균 남은 거리",
      value:
        summary.avgToTarget === null
          ? "—"
          : `${summary.avgToTarget > 0 ? "+" : ""}${summary.avgToTarget.toFixed(1)}%`,
      foot:
        summary.avgSinceBase === null
          ? "등록가가 기록된 목표부터 집계"
          : `등록 후 평균 ${summary.avgSinceBase > 0 ? "+" : ""}${summary.avgSinceBase.toFixed(1)}%`,
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
      {tiles.map((t) => (
        <button
          key={t.label}
          type="button"
          onClick={() => onPick(t.id)}
          className={cn(
            "rounded-xl border bg-[var(--surface)] px-3 py-2.5 text-left transition-colors",
            active === t.id && t.id !== "all"
              ? "border-[var(--brand)]"
              : "border-[var(--border)] hover:border-[var(--brand)]",
          )}
        >
          <p className="text-cell text-[var(--text-muted)]">{t.label}</p>
          <p className={cn("mt-0.5 font-num text-xl font-bold tabular-nums", t.tone)}>{t.value}</p>
          <p className="mt-0.5 truncate text-micro text-[var(--text-muted)]">{t.foot}</p>
        </button>
      ))}
    </div>
  );
}

function ViewToggle({ view }: { view: TargetsView }) {
  const options: { id: TargetsView; label: string; Icon: typeof List }[] = [
    { id: "row", label: "행", Icon: List },
    { id: "card", label: "카드", Icon: LayoutGrid },
  ];
  return (
    <div className="flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5">
      {options.map(({ id, label, Icon }) => (
        <button
          key={id}
          type="button"
          onClick={() => setTargetsView(id)}
          aria-pressed={view === id}
          title={`${label} 보기`}
          className={cn(
            "rounded-md p-1.5 transition-colors",
            view === id
              ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
              : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
          )}
        >
          <Icon className="size-3.5" />
        </button>
      ))}
    </div>
  );
}

/* ── 행 보기 ─────────────────────────────────────────────────────────────── */

function TargetRow({
  view,
  sample,
  onOpen,
  onEdit,
  onRemove,
}: {
  view: TargetView;
  sample: boolean;
  onOpen: () => void;
  onEdit: () => void;
  onRemove: () => void;
}) {
  const { target, quote, current } = view;

  return (
    <li>
      <div
        role="button"
        tabIndex={0}
        onClick={onOpen}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onOpen();
          }
        }}
        className="flex cursor-pointer items-center gap-3 px-4 py-3 transition-colors hover:bg-[var(--surface-elevated)]"
      >
        <StockAvatar
          symbol={target.symbol}
          name={quote?.name}
          currency={target.currency}
          size="sm"
          className="shrink-0"
        />

        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5">
            <p className="truncate text-sm font-medium text-[var(--text-primary)]">
              {quote?.name || target.symbol}
            </p>
            <span className="shrink-0 text-cell text-[var(--text-muted)]">{target.symbol}</span>
            <TargetStatusBadge status={view.status} />
          </div>
          {(target.note || view.sinceBasePct !== null) && (
            <p className="mt-0.5 truncate text-cell text-[var(--text-muted)]">
              {view.sinceBasePct !== null && (
                <span
                  className={cn(
                    "font-num font-medium",
                    view.sinceBasePct > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                  )}
                >
                  등록 후 {view.sinceBasePct > 0 ? "+" : ""}
                  {view.sinceBasePct.toFixed(1)}%
                </span>
              )}
              {view.sinceBasePct !== null && target.note && " · "}
              {target.note}
            </p>
          )}
        </div>

        <div className="hidden w-[190px] shrink-0 md:block">
          <TargetAxis view={view} />
        </div>

        <div className="hidden shrink-0 text-right sm:block">
          <p className="text-micro text-[var(--text-muted)]">현재가</p>
          {current !== null ? (
            <PriceDisplay
              value={current}
              currency={target.currency}
              size="sm"
              className="font-num"
            />
          ) : (
            <span className="font-num text-sm text-[var(--text-muted)]">—</span>
          )}
        </div>

        <TargetGapCell view={view} className="w-[76px] shrink-0" />

        {/* 예시 행은 서버에 없다 — 눌러도 아무 일이 없을 버튼을 그리지 않는다. */}
        {!sample && (
          <div
            className="flex shrink-0 gap-2"
            onClick={(e) => e.stopPropagation()}
            role="presentation"
          >
            <button
              type="button"
              onClick={onEdit}
              className="text-cell text-[var(--text-muted)] hover:text-[var(--brand)]"
            >
              수정
            </button>
            <button
              type="button"
              onClick={onRemove}
              className="text-cell text-[var(--text-muted)] hover:text-rose-400"
            >
              삭제
            </button>
          </div>
        )}
      </div>
    </li>
  );
}

/* ── 카드 보기 ───────────────────────────────────────────────────────────── */

function TargetCard({
  view,
  onOpen,
}: {
  view: TargetView;
  onOpen: () => void;
}) {
  const { target, quote, current, status } = view;
  /*
   * 카드는 화면에 여섯 장 안팎이라 종목당 캔들 한 번이 감당된다. 행 보기에서는
   * 부르지 않는다 — 스무 줄이면 스무 요청이 된다.
   *
   * 예시에서도 부른다. 예전에는 막았는데, 그때는 카드의 가격이 픽스처라 실제 캔들과
   * 같이 그리면 「현재 214,500원」 옆에 1,359,000원짜리 차트가 붙어 한 카드가 서로 다른
   * 두 종목을 말했다. 지금은 가격도 캔들도 같은 시세에서 온다.
   */
  const { points, times } = useSparklineCandles(target.symbol, true);

  return (
    <button
      type="button"
      onClick={onOpen}
      className={cn(
        "flex flex-col gap-2 rounded-xl border bg-[var(--surface)] p-3 text-left transition-colors hover:border-[var(--brand)]",
        status === "reached"
          ? "border-emerald-500/60"
          : status === "near"
            ? "border-amber-500/60"
            : "border-[var(--border)]",
      )}
    >
      <div className="flex min-w-0 items-center gap-2">
        <StockAvatar
          symbol={target.symbol}
          name={quote?.name}
          currency={target.currency}
          size="xs"
          className="shrink-0"
        />
        <span className="min-w-0 flex-1 truncate text-sm font-medium text-[var(--text-primary)]">
          {quote?.name || target.symbol}
        </span>
        <TargetStatusBadge status={status} />
      </div>

      <div className="flex items-baseline justify-between gap-2">
        <span className="min-w-0">
          <span className="block text-micro text-[var(--text-muted)]">현재</span>
          <span className="block font-num text-base font-bold tabular-nums">
            {current !== null ? formatPrice(current, target.currency) : "—"}
          </span>
        </span>
        <span className="min-w-0 text-right">
          <span className="block text-micro text-[var(--text-muted)]">목표</span>
          <span className="block font-num text-base font-bold tabular-nums text-[var(--profit)]">
            <PriceDisplay value={target.targetPrice} currency={target.currency} size="inherit" />
          </span>
        </span>
      </div>

      {points.length >= 2 && (
        <PriceHistoryChart
          points={points}
          times={times}
          currency={target.currency}
          markerPrice={target.targetPrice}
          markerLabel="목표"
          title=""
          height={64}
        />
      )}

      <TargetAxis view={view} />

      <div className="flex items-center justify-between gap-2 text-cell">
        <span className="min-w-0 truncate text-[var(--text-muted)]">
          {target.note || (target.baseDate ? `${target.baseDate} 등록` : "메모 없음")}
        </span>
        <span
          className={cn(
            "shrink-0 font-num font-semibold tabular-nums",
            status === "reached" ? "text-emerald-500" : "text-[var(--text-secondary)]",
          )}
        >
          {status === "reached"
            ? "도달"
            : view.toTargetPct === null
              ? "—"
              : `${view.toTargetPct > 0 ? "+" : ""}${view.toTargetPct.toFixed(1)}%`}
        </span>
      </div>
    </button>
  );
}

/* ── 등록 폼 ─────────────────────────────────────────────────────────────── */

function TargetForm({
  editingId,
  stock,
  setStock,
  targetPrice,
  setTargetPrice,
  targetPrice2,
  setTargetPrice2,
  stopPrice,
  setStopPrice,
  note,
  setNote,
  searchFocusKey,
  onSearchFocus,
  onSave,
  onReset,
  currentPrice,
}: {
  editingId: string | null;
  stock: StockInfo | null;
  setStock: (s: StockInfo | null) => void;
  targetPrice: string;
  setTargetPrice: (v: string) => void;
  targetPrice2: string;
  setTargetPrice2: (v: string) => void;
  stopPrice: string;
  setStopPrice: (v: string) => void;
  note: string;
  setNote: (v: string) => void;
  searchFocusKey: number;
  onSearchFocus: () => void;
  onSave: () => void;
  onReset: () => void;
  currentPrice: number | null;
}) {
  const currency = stock ? inferTargetCurrency(stock.symbol) : "KRW";
  const gap =
    currentPrice && currentPrice > 0 && Number(targetPrice) > 0
      ? ((Number(targetPrice) - currentPrice) / currentPrice) * 100
      : null;

  return (
    <SectionCard
      title={editingId ? "목표가 수정" : "목표가 추가"}
      action={
        editingId ? (
          <button
            type="button"
            onClick={onReset}
            className="text-cell text-muted-foreground hover:text-foreground"
          >
            취소
          </button>
        ) : undefined
      }
    >
      <div className="space-y-2">
        {stock ? (
          <div className="flex h-8 min-w-0 items-center rounded-md border border-input bg-transparent shadow-xs dark:bg-input/30">
            <div className="flex min-w-0 flex-1 items-center gap-2 px-2.5">
              <StockAvatar
                symbol={stock.symbol}
                name={stock.name}
                currency={stock.currency}
                size="xs"
                className="shrink-0"
              />
              <span className="min-w-0 flex-1 truncate text-cell">{stock.name || stock.symbol}</span>
              <span className="shrink-0 text-micro text-muted-foreground">{stock.symbol}</span>
            </div>
            <button
              type="button"
              className="mr-1 flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-accent"
              onClick={() => setStock(null)}
              aria-label="종목 선택 취소"
            >
              <X className="size-3.5" />
            </button>
          </div>
        ) : (
          <StockCatalogSearchField
            key={searchFocusKey}
            placeholder="종목명·코드 검색"
            suggestionsId="targets-add-suggestions"
            autoFocus={searchFocusKey > 0}
            inputClassName="h-8 py-1 pl-9 pr-2.5 text-cell md:text-cell"
            onSelect={(picked) => {
              setStock(picked);
              onSearchFocus();
            }}
          />
        )}

        <LabeledPrice
          label="목표가"
          required
          value={targetPrice}
          onChange={setTargetPrice}
          hint={
            gap === null
              ? undefined
              : `현재가 대비 ${gap > 0 ? "+" : ""}${gap.toFixed(1)}%`
          }
        />
        <LabeledPrice
          label="2차 목표"
          value={targetPrice2}
          onChange={setTargetPrice2}
          placeholder="선택"
        />
        <LabeledPrice
          label="손절가"
          value={stopPrice}
          onChange={setStopPrice}
          placeholder="선택"
        />

        <input
          type="text"
          value={note}
          onChange={(e) => setNote(e.target.value)}
          placeholder="메모 (선택)"
          className="h-8 w-full rounded-lg border bg-background px-2.5 text-cell placeholder:text-muted-foreground focus:border-[var(--brand)] focus:outline-none"
        />

        {/* 등록가 박제는 새로 만들 때만 일어난다 — 수정에서는 그 자리가 이미 고정돼 있다. */}
        {stock && !editingId && (
          <p className="text-micro leading-relaxed text-muted-foreground">
            지금 주가({currentPrice ? formatPrice(currentPrice, currency) : "확인 중"})가 등록가로
            함께 기록됩니다. 이후 진행률은 이 값을 기준으로 그려집니다.
          </p>
        )}

        <button
          type="button"
          onClick={onSave}
          disabled={!stock || !(Number(targetPrice) > 0)}
          className="h-8 w-full rounded-lg bg-[var(--brand)] text-cell font-medium text-white hover:opacity-90 disabled:opacity-50"
        >
          {editingId ? "수정" : "추가"}
        </button>
      </div>
    </SectionCard>
  );
}

function LabeledPrice({
  label,
  value,
  onChange,
  placeholder,
  hint,
  required,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  hint?: string;
  required?: boolean;
}) {
  return (
    <div className="space-y-0.5">
      <div className="flex items-baseline justify-between gap-2">
        <label className="text-micro text-muted-foreground">
          {label}
          {required && <span className="ml-0.5 text-[var(--profit)]">*</span>}
        </label>
        {hint && <span className="font-num text-micro text-muted-foreground">{hint}</span>}
      </div>
      <input
        type="number"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        min="0"
        step="any"
        className="h-8 w-full rounded-lg border bg-background px-2.5 text-cell placeholder:text-muted-foreground focus:border-[var(--brand)] focus:outline-none"
      />
    </div>
  );
}
