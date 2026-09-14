"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Target, LayoutList, Rows3 } from "lucide-react";

import { MenuEmptyState } from "@/components/common/menu-empty-state";
import { menuIcon } from "@/components/common/menu-icon";
import {
  SampleModeButton,
  SampleRibbon,
  SampleWrap,
  useAutoSample,
  useSampleMode,
} from "@/components/common/sample-mode";
import { PageColumns } from "@/components/layout/page-columns";
import { PageActions, PageScrollShell } from "@/components/layout/page-content-shell";
import { PredictionCard, PredictionVerdictCard } from "@/components/predictions/prediction-card";
import { PredictionCreateDialog } from "@/components/predictions/prediction-create-dialog";
import { PredictionDetailModal } from "@/components/predictions/prediction-detail-modal";
import {
  PredictionScoreboard,
  PredictionSpread,
} from "@/components/predictions/prediction-overview";
import { PredictionRow, type RowDensity } from "@/components/predictions/prediction-row";
import { PredictionScorecardModal } from "@/components/predictions/prediction-scorecard-modal";
import { PredictionsSideRail } from "@/components/predictions/predictions-side-rail";
import { useAuth } from "@/components/providers/auth-provider";
import { useConfirm } from "@/components/providers/confirm-provider";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { useLoginGate } from "@/hooks/use-login-gate";
import { usePredictions } from "@/hooks/use-predictions";
import { fetchUserHoldingsClient } from "@/lib/holdings-import-api";
import { MENU_COPY } from "@/lib/menu-copy";
import {
  HORIZON_FILTERS,
  SORTS,
  STATUS_FILTERS,
  type Prediction,
  type PredictionHorizonFilter,
  type PredictionSort,
  type PredictionStatusFilter,
} from "@/lib/prediction-types";
import { fetchSampleHoldings } from "@/lib/sample-api";
import type { UserHolding } from "@/lib/user-holdings-types";
import { cn } from "@/lib/utils";

const COPY = MENU_COPY.predictions;

/**
 * 「목표가 예측」 — 목표가 관리와 스탬프를 하나로 합친 화면.
 *
 * # 층 순서가 이 화면의 주장이다
 *
 *  1. **조망 + 성적** — 스무 건이 돼도 「대체로 잘 가고 있나」에 한 줄로 답한다.
 *  2. **개봉 대기** — 만기가 지난 것. 이 화면에 다시 들어올 유일한 이유라, 결과를
 *     가려 두고 「열기」로 편다. 0건이면 층 자체가 사라진다.
 *  3. **목록** — 밀도 두 가지. 예측이 열 건을 넘으면 압축이 기본이 된다.
 *
 * 필터·집계·조망은 전부 **이미 받아 둔 배열 위에서** 센다. 서버 재요청 0.
 */
export function PredictionsClient() {
  const confirm = useConfirm();
  const params = useSearchParams();
  const { session } = useAuth();
  const gate = useLoginGate("예측 세우기");
  const { active: sample, setActive: setSample } = useSampleMode();

  const {
    all,
    visible,
    duePredictions,
    urgent,
    summary,
    symbols,
    quotes,
    loading,
    error,
    filters,
    setFilter,
    reload,
    openPrediction,
    annotate,
    remove,
  } = usePredictions({ sample });

  const [detailKey, setDetailKey] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [scorecardOpen, setScorecardOpen] = useState(false);
  /**
   * 밀도는 **개수에서 파생**한다 — 사용자가 직접 고르면 그 선택이 이긴다.
   * 이펙트로 세팅하면 첫 프레임이 축으로 그려졌다가 압축으로 한 번 더 튄다.
   */
  const [densityOverride, setDensityOverride] = useState<RowDensity | null>(null);
  /** 이번 방문에 이미 연 개봉 카드. 서버 `opened`와 별개로 화면이 즉시 뒤집히게 한다. */
  const [revealed, setRevealed] = useState<Set<string>>(new Set());
  const [holdings, setHoldings] = useState<UserHolding[]>([]);

  // 종목 딥링크 — 종목 화면의 예측 버튼이 `?symbol=`로 들어온다.
  const symbolParam = params.get("symbol");
  useEffect(() => {
    if (symbolParam) setFilter("symbol", symbolParam.toUpperCase());
  }, [symbolParam, setFilter]);

  // 예측이 쌓이면 축이 화면을 다 먹는다. 열 건이 넘어가면 압축이 기본이다.
  const density: RowDensity = densityOverride ?? (all.length > 10 ? "compact" : "axis");

  /*
   * 보유 종목의 출처가 바뀌면(예시↔내 계정↔비로그인) 이전 목록을 **렌더 중에** 비운다.
   * 이펙트에서 비우면 한 프레임 동안 남의 보유 종목이 레일에 남는다.
   */
  const holdingsKey = sample ? "sample" : session ? "user" : "none";
  const [lastHoldingsKey, setLastHoldingsKey] = useState(holdingsKey);
  if (lastHoldingsKey !== holdingsKey) {
    setLastHoldingsKey(holdingsKey);
    setHoldings([]);
  }

  useEffect(() => {
    if (holdingsKey === "none") return;
    let alive = true;
    const load = holdingsKey === "sample" ? fetchSampleHoldings() : fetchUserHoldingsClient();
    load
      .then((rows) => {
        if (!alive) return;
        setHoldings(
          rows.map(
            (h) =>
              ({
                symbol: h.symbol,
                name: h.name,
                quantity: h.quantity,
                avgPrice: h.avgPrice,
                costCurrency: (h as { currency?: string }).currency ?? "KRW",
              }) as unknown as UserHolding,
          ),
        );
      })
      .catch(() => {
        // 보유 조회가 실패해도 예측 목록은 유효하다 — 레일 두 칸만 비워 둔다.
      });
    return () => {
      alive = false;
    };
  }, [holdingsKey]);

  const detail = useMemo(
    () => all.find((p) => p.key === detailKey) ?? null,
    [all, detailKey],
  );

  /*
   * 개봉 덱은 **이번 방문에 연 것까지** 붙잡고 있는다. 열자마자 `due`가 풀리므로
   * 상태만 보고 그리면 「열어서 채점 보기」를 누른 카드가 그 자리에서 사라진다 —
   * 방금 한 행동의 결과가 화면에서 없어지는 것이라 취소된 것처럼 읽힌다.
   */
  const deck = useMemo(
    () => all.filter((p) => p.status === "due" || revealed.has(p.key)),
    [all, revealed],
  );

  const openDetail = useCallback((p: Prediction) => setDetailKey(p.key), []);

  const reveal = useCallback(
    (p: Prediction) => {
      setRevealed((prev) => new Set(prev).add(p.key));
      if (!sample) void openPrediction(p);
    },
    [openPrediction, sample],
  );

  const revealAll = useCallback(() => {
    setRevealed((prev) => {
      const next = new Set(prev);
      for (const p of duePredictions) next.add(p.key);
      return next;
    });
    if (!sample) for (const p of duePredictions) void openPrediction(p);
  }, [duePredictions, openPrediction, sample]);

  // 예시를 보고 있지 않고 진짜로 0건일 때만 안내 화면을 세운다. 실패를 0건으로
  // 그리면 예측을 등록해 둔 사용자는 자기 기록이 지워진 줄 안다.
  /*
    「내 것이 아직 없다」 — **`sample`을 빼고** 잰다 (2026-09-14 사용자 요청 4).
    이 값이 `useAutoSample`로 가서 예시를 켠다. `!sample`이 섞이면 켜짐 → 0건 아님 →
    꺼짐이 무한히 반복된다. 안내 카드는 그 위에 `!sample`을 얹은 `isEmpty`로 선다.
  */
  const hasNothingYet = !loading && !error && all.length === 0;
  useAutoSample(hasNothingYet);
  const isEmpty = !sample && hasNothingYet;

  return (
    <>
      <PageScrollShell>
        <PageActions>
          {!isEmpty && (
            <SampleModeButton active={sample} onToggle={setSample} emphasize={!session} />
          )}
          {!sample && (
            <button
              type="button"
              onClick={() => gate.run(() => setCreateOpen(true))}
              title={gate.authed ? undefined : "로그인하면 예측을 세울 수 있습니다"}
              className={cn(
                buttonVariants({ variant: "default", size: "sm" }),
                "bg-[var(--brand)] text-white hover:opacity-90",
              )}
            >
              <Target className="size-3.5" />
              {gate.label("예측 세우기")}
            </button>
          )}
        </PageActions>

        {sample && (
          <div className="mb-3">
            <SampleRibbon
              loggedIn={Boolean(session)}
              loginHref="/login?next=%2Fpredictions"
              onDismiss={() => setSample(false)}
              // 안내 카드가 들고 있던 첫 동작 — 카드가 예시 뒤로 물러났으니 여기 선다.
              action={{
                label: gate.label("예측 세우기"),
                onClick: () => gate.run(() => setCreateOpen(true)),
              }}
            />
          </div>
        )}

        {isEmpty ? (
          <MenuEmptyState
            icon={menuIcon("predictions")}
            headline={COPY.emptyHeadline}
            description={COPY.emptyDescription}
            steps={COPY.steps}
            primary={{
              label: gate.label("예측 세우기"),
              onClick: () => gate.run(() => setCreateOpen(true)),
            }}
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
              <PredictionsSideRail
                summary={summary}
                all={all}
                urgent={urgent}
                holdings={holdings}
                symbols={symbols}
                symbolFilter={filters.symbol}
                onSymbolFilter={(s) => setFilter("symbol", s)}
                onSelect={openDetail}
                onCreateFor={() => gate.run(() => setCreateOpen(true))}
              />
            }
          >
            <SampleWrap active={sample}>
              {/* ① 조망 + 성적 */}
              <Card className="p-3">
                <PredictionScoreboard
                  summary={summary}
                  onOpenScorecard={() => setScorecardOpen(true)}
                />
                {all.length > 0 && (
                  <div className="mt-3 border-t border-[var(--border)] pt-2">
                    <PredictionSpread predictions={all} onSelect={openDetail} />
                  </div>
                )}
              </Card>

              {/* ② 개봉 대기 — 0건이면 층 자체가 없다 */}
              {deck.length > 0 && (
                <Card className="mt-3 p-0">
                  <CardHeader className="mb-0 border-b border-[var(--border)] px-4 py-3">
                    <CardTitle>개봉할 때가 됐습니다</CardTitle>
                    <Badge variant="info">{deck.length}건</Badge>
                    <span className="ml-auto flex items-center gap-2">
                      <span className="hidden text-micro text-[var(--text-muted)] sm:inline">
                        결과는 열어야 보입니다
                      </span>
                      <button
                        type="button"
                        onClick={revealAll}
                        className={cn(buttonVariants({ variant: "outline", size: "xs" }))}
                      >
                        모두 열기
                      </button>
                    </span>
                  </CardHeader>
                  <div className="grid gap-2.5 p-3 sm:grid-cols-2 xl:grid-cols-3">
                    {deck.map((p) =>
                      revealed.has(p.key) ? (
                        <PredictionVerdictCard
                          key={p.key}
                          prediction={p}
                          onOpen={() => openDetail(p)}
                        />
                      ) : (
                        <PredictionCard
                          key={p.key}
                          prediction={p}
                          sealed
                          onOpen={() => openDetail(p)}
                          onReveal={() => reveal(p)}
                        />
                      ),
                    )}
                  </div>
                </Card>
              )}

              {/* ③ 필터 */}
              <Card className="mt-3 p-3">
                <div className="flex flex-wrap items-center gap-2">
                  <FilterGroup
                    label="상태"
                    options={STATUS_FILTERS}
                    value={filters.status}
                    onChange={(v) => setFilter("status", v as PredictionStatusFilter)}
                  />
                  <FilterGroup
                    label="기한"
                    options={HORIZON_FILTERS}
                    value={filters.horizon}
                    onChange={(v) => setFilter("horizon", v as PredictionHorizonFilter)}
                  />
                  <FilterGroup
                    label="정렬"
                    className="lg:ml-auto"
                    options={SORTS}
                    value={filters.sort}
                    onChange={(v) => setFilter("sort", v as PredictionSort)}
                  />
                  <div className="flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5">
                    <DensityButton
                      active={density === "axis"}
                      label="축 보기"
                      onClick={() => setDensityOverride("axis")}
                    >
                      <Rows3 className="size-3.5" />
                    </DensityButton>
                    <DensityButton
                      active={density === "compact"}
                      label="압축 보기"
                      onClick={() => setDensityOverride("compact")}
                    >
                      <LayoutList className="size-3.5" />
                    </DensityButton>
                  </div>
                </div>
                {filters.symbol && (
                  <div className="mt-2 flex items-center gap-2 text-micro text-[var(--text-muted)]">
                    <span>
                      <b className="text-[var(--text-primary)]">{filters.symbol}</b> 만 보는 중
                    </span>
                    <button
                      type="button"
                      onClick={() => setFilter("symbol", null)}
                      className="text-[var(--brand)]"
                    >
                      전체 보기
                    </button>
                  </div>
                )}
              </Card>

              {/* ④ 목록 */}
              <Card className="mt-3 p-0">
                <CardHeader className="mb-0 border-b border-[var(--border)] px-4 py-3">
                  <CardTitle>예측 목록</CardTitle>
                  {!loading && (
                    <Badge variant="info">
                      {visible.length}건
                      {visible.length !== all.length && ` / 전체 ${all.length}건`}
                    </Badge>
                  )}
                </CardHeader>

                {loading ? (
                  <div className="space-y-2 p-4">
                    <div className="h-16 animate-pulse rounded-lg bg-[var(--surface-elevated)]" />
                    <div className="h-16 animate-pulse rounded-lg bg-[var(--surface-elevated)]" />
                  </div>
                ) : error ? (
                  <div className="p-6 text-center">
                    <p className="text-sm text-[var(--text-muted)]">{error}</p>
                    <button
                      type="button"
                      onClick={() => void reload()}
                      className={cn(buttonVariants({ variant: "outline", size: "sm" }), "mt-3")}
                    >
                      다시 시도
                    </button>
                  </div>
                ) : visible.length === 0 ? (
                  <div className="p-6 text-center">
                    <Target className="mx-auto size-6 text-[var(--text-muted)]" />
                    <p className="mt-2 text-sm text-[var(--text-muted)]">
                      {all.length === 0
                        ? "아직 세운 예측이 없습니다. 종목을 고르고 목표가를 적어보세요."
                        : "조건에 맞는 예측이 없습니다."}
                    </p>
                  </div>
                ) : (
                  <>
                    {/* 넓은 화면 — 행. 폭 판단은 컨테이너 쿼리가 한다. */}
                    <div className="@container hidden sm:block">
                      <ul className="divide-y divide-[var(--border)]">
                        {visible.map((p) => (
                          <li key={p.key}>
                            <PredictionRow
                              prediction={p}
                              density={density}
                              onOpen={() => openDetail(p)}
                            />
                          </li>
                        ))}
                      </ul>
                    </div>
                    {/* 좁은 화면 — 카드. 행은 375px에서 성립하지 않는다. */}
                    <div className="grid gap-2.5 p-3 sm:hidden">
                      {visible.map((p) => (
                        <PredictionCard
                          key={p.key}
                          prediction={p}
                          onOpen={() => openDetail(p)}
                        />
                      ))}
                    </div>
                  </>
                )}
              </Card>
            </SampleWrap>
          </PageColumns>
        )}
      </PageScrollShell>

      <PredictionCreateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={() => void reload()}
      />

      <PredictionDetailModal
        prediction={detail}
        quote={detail ? quotes[detail.symbol] : undefined}
        readOnly={sample}
        onOpenChange={(open) => !open && setDetailKey(null)}
        onUpdate={async (input) => {
          if (detail) await annotate(detail, input);
        }}
        onOpen={() => detail && void openPrediction(detail)}
        onDelete={async () => {
          if (!detail) return;
          const ok = await confirm({
            title: "이 예측을 삭제할까요?",
            description:
              detail.source === "stamp"
                ? "등록 당시의 시세·점수·뉴스 스냅샷은 복구할 수 없습니다."
                : "등록가와 도달 기록이 함께 사라집니다.",
            confirmLabel: "삭제",
            tone: "danger",
          });
          if (!ok) return;
          await remove(detail);
          setDetailKey(null);
          void reload();
        }}
      />

      <PredictionScorecardModal
        open={scorecardOpen}
        onOpenChange={setScorecardOpen}
        predictions={all}
        summary={summary}
      />
    </>
  );
}

function FilterGroup({
  label,
  options,
  value,
  onChange,
  className,
}: {
  label: string;
  options: readonly { id: string; label: string }[];
  value: string;
  onChange: (id: string) => void;
  className?: string;
}) {
  return (
    <div
      role="group"
      aria-label={label}
      className={cn("flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5", className)}
    >
      {options.map((opt) => (
        <button
          key={opt.id}
          type="button"
          aria-pressed={value === opt.id}
          onClick={() => onChange(opt.id)}
          className={cn(
            "rounded-md px-2.5 py-1 text-cell font-medium transition-colors",
            value === opt.id
              ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
              : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

function DensityButton({
  active,
  label,
  onClick,
  children,
}: {
  active: boolean;
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      title={label}
      aria-label={label}
      aria-pressed={active}
      className={cn(
        "grid size-7 place-items-center rounded-md transition-colors",
        active
          ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
          : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
      )}
    >
      {children}
    </button>
  );
}
