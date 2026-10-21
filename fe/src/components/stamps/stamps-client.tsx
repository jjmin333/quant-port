"use client";

import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Stamp as StampIcon } from "lucide-react";
import { PageColumns } from "@/components/layout/page-columns";
import { PageActions, PageScrollShell } from "@/components/layout/page-content-shell";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { MenuEmptyState } from "@/components/common/menu-empty-state";
import { menuIcon } from "@/components/common/menu-icon";
import {
  SampleModeButton,
  SampleRibbon,
  SampleWrap,
  useAutoSample,
  useSampleMode,
} from "@/components/common/sample-mode";
import { useAuth } from "@/components/providers/auth-provider";
import { useLoginGate } from "@/hooks/use-login-gate";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { StanceBadge } from "@/components/stamps/stamp-detail";
import { StampCreateDialog } from "@/components/stamps/stamp-create-dialog";
import { StampDetailModal } from "@/components/stamps/stamp-detail-modal";
import { StampRevealModal } from "@/components/stamps/stamp-reveal-modal";
import { StampScorecardModal } from "@/components/stamps/stamp-scorecard-modal";
import { StampSeal } from "@/components/stamps/stamp-seal";
import { StampTargetGauge } from "@/components/stamps/stamp-target-gauge";
import { StampsSideRail } from "@/components/stamps/stamps-side-rail";
import { useStamps, type StampVerdictFilter } from "@/hooks/use-stamps";
import { formatDateKST, formatDateTimeKST, formatPercent } from "@/lib/format";
import { MENU_COPY } from "@/lib/menu-copy";
import { HORIZON_LABELS, type Stamp, type StampStance } from "@/lib/stamp-types";
import { cn } from "@/lib/utils";
import { useConfirm } from "@/components/providers/confirm-provider";
import { PriceDisplay } from "@/components/ui/price-display";
import { buttonVariants } from "@/components/ui/button";

const COPY = MENU_COPY.stamps;

const STANCE_FILTERS: { id: StampStance | "all"; label: string }[] = [
  { id: "all", label: "전체 관점" },
  { id: "bullish", label: "강세" },
  { id: "neutral", label: "관망" },
  { id: "bearish", label: "약세" },
];

const VERDICT_FILTERS: { id: StampVerdictFilter; label: string }[] = [
  { id: "all", label: "전체" },
  { id: "due", label: "개봉 대기" },
  { id: "pending", label: "검증 중" },
  { id: "hit", label: "적중" },
  { id: "miss", label: "빗나감" },
];

function toneClass(value: number | undefined): string {
  if (value === undefined) return "text-[var(--text-muted)]";
  if (value > 0) return "text-[var(--profit)]";
  if (value < 0) return "text-[var(--loss)]";
  return "text-[var(--text-secondary)]";
}

export function StampsClient() {
  const confirm = useConfirm();
  const params = useSearchParams();
  const { session } = useAuth();
  const gate = useLoginGate("스탬프 찍기");
  const { active: sample, setActive: setSample } = useSampleMode();
  const {
    stamps,
    grouped,
    summary,
    allStamps,
    dueStamps,
    totalCount,
    symbols,
    loading,
    error,
    symbolFilter,
    setSymbolFilter,
    stanceFilter,
    setStanceFilter,
    verdictFilter,
    setVerdictFilter,
    grouping,
    setGrouping,
    updateStamp,
    openStamp,
    removeStamp,
    reload,
  } = useStamps({ sample });

  const [detailId, setDetailId] = useState<number | null>(null);
  const [scorecardOpen, setScorecardOpen] = useState(false);
  const [revealOpen, setRevealOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  // 예시를 보고 있지 않고 진짜로 0건일 때만 안내 화면을 세운다. 로딩 중에 잠깐
  // 스쳐 지나가면 화면이 두 번 바뀌는 것처럼 보이므로 loading도 함께 본다.
  /*
    「내 것이 아직 없다」 — **`sample`을 빼고** 잰다 (2026-09-14 사용자 요청 4).
    이 값이 `useAutoSample`로 가서 예시를 켠다. `!sample`이 섞이면 켜짐 → 0건 아님 →
    꺼짐이 무한히 반복된다. 안내 카드는 그 위에 `!sample`을 얹은 `isEmpty`로 선다.
  */
  const hasNothingYet = !loading && !error && totalCount === 0;
  useAutoSample(loading ? null : hasNothingYet);
  const isEmpty = !sample && hasNothingYet;

  // Deep link from the stamp button ("내 스탬프 보기").
  const symbolParam = params.get("symbol");
  useEffect(() => {
    if (symbolParam) setSymbolFilter(symbolParam.toUpperCase());
  }, [symbolParam, setSymbolFilter]);

  // 목록이 갱신되면 열려 있는 모달도 새 값을 봐야 한다 — id로 다시 찾는다.
  const detailStamp = useMemo(
    () => allStamps.find((s) => s.id === detailId) ?? null,
    [allStamps, detailId],
  );

  return (
    <>
      <PageScrollShell>
        <PageActions>
          {/* 0건 안내 카드가 같은 버튼을 들고 있다 (§R-2). */}
          {!isEmpty && (
            <SampleModeButton active={sample} onToggle={setSample} emphasize={!session} />
          )}
          {/*
            **비로그인이어도 버튼을 보여 준다** (2026-09-04 사용자 요청 2).

            전에는 `session &&`으로 통째로 숨겼다. 숨기면 401은 안 나지만, 비로그인
            사용자는 이 화면에서 **무엇을 할 수 있는 곳인지** 알 수 없다 — 스탬프
            목록만 있고 찍는 길이 화면 어디에도 없다. 앱의 나머지(종목 헤더의 셋)는
            같은 상황에서 「로그인하고 저장」이라고 적힌 버튼을 보여 주므로, 여기만
            사라지면 규칙이 화면마다 다르다.

            예시 모드(`sample`)에서는 여전히 감춘다 — 그건 로그인 문제가 아니라
            「지금 보고 있는 것이 내 데이터가 아니다」라서다.
          */}
          {!sample && (
            <button
              type="button"
              onClick={() => gate.run(() => setCreateOpen(true))}
              title={gate.authed ? undefined : "로그인하면 스탬프를 찍을 수 있습니다"}
              className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-[var(--brand-foreground)] hover:opacity-90")}
            >
              <StampIcon className="size-3.5" />
              {gate.label("스탬프 찍기")}
            </button>
          )}
        </PageActions>
        {sample && (
          <div className="mb-3">
            <SampleRibbon
              loggedIn={Boolean(session)}
              loginHref="/login?next=%2Fstamps"
              onDismiss={() => setSample(false)}
              // 안내 카드가 들고 있던 첫 동작 — 카드가 예시 뒤로 물러났으니 여기 선다.
              action={{
                label: gate.label("스탬프 찍기"),
                onClick: () => gate.run(() => setCreateOpen(true)),
              }}
            />
          </div>
        )}

        {isEmpty ? (
          <MenuEmptyState
            icon={menuIcon("stamps")}
            headline={COPY.emptyHeadline}
            description={COPY.emptyDescription}
            steps={COPY.steps}
            // 0건 안내 카드의 버튼도 위 툴바 버튼과 같은 게이트를 지난다 — 0건인
            // 사용자는 대개 아직 로그인하지 않은 사용자다.
            primary={{
              label: gate.label("스탬프 찍기"),
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
            <StampsSideRail
              summary={summary}
              allStamps={allStamps}
              dueStamps={dueStamps}
              totalCount={totalCount}
              symbols={symbols}
              symbolFilter={symbolFilter}
              onSymbolFilter={setSymbolFilter}
              onOpenStamp={setDetailId}
            />
          }
        >
          <SampleWrap active={sample}>
          {/* A-1 — 성과 타일. 적중률 타일이 성적표 모달의 입구다. */}
          <div className="grid grid-cols-2 gap-2.5 @3xl:grid-cols-4">
            <button
              type="button"
              onClick={() => setScorecardOpen(true)}
              className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 text-left transition-colors hover:border-[var(--brand)]"
            >
              <div className="flex items-center gap-3">
                <HitRateRing value={summary?.hitRate} />
                <div className="min-w-0">
                  <p className="text-cell text-[var(--text-muted)]">
                    적중률 <span className="text-micro">클릭 →</span>
                  </p>
                  <p className="mt-0.5 text-micro leading-tight text-[var(--text-muted)]">
                    판정 {summary?.judged ?? 0}건 중 {summary?.hits ?? 0}건 적중
                    {summary?.avgTargetError !== undefined && (
                      <>
                        <br />
                        평균 오차 <b>{summary.avgTargetError.toFixed(1)}%</b>
                      </>
                    )}
                  </p>
                </div>
              </div>
            </button>

            <Tile
              label="평균 알파 (벤치마크 대비)"
              value={summary?.avgAlpha === undefined ? "—" : formatPercent(summary.avgAlpha)}
              valueClass={toneClass(summary?.avgAlpha)}
              foot={
                summary?.avgReturn === undefined
                  ? "판정된 스탬프가 쌓이면 계산됩니다"
                  : `내 평균 ${formatPercent(summary.avgReturn)}`
              }
            />

            <Tile
              label="개봉 대기"
              value={`${summary?.dueCount ?? dueStamps.length}건`}
              valueClass={
                (summary?.dueCount ?? dueStamps.length) > 0 ? "text-[var(--brand)]" : undefined
              }
              foot={
                dueStamps.length > 0
                  ? `가장 오래된 것 ${formatDateKST(dueStamps[dueStamps.length - 1].dueAt)} 만기`
                  : "검증이 끝난 스탬프가 없습니다"
              }
              onClick={dueStamps.length > 0 ? () => setRevealOpen(true) : undefined}
            />

            <Tile
              label="스탬프"
              value={`${summary?.total ?? totalCount}건`}
              foot={`${summary?.symbols ?? symbols.length}개 종목 · 마지막 ${
                summary?.lastStampedAt ? formatDateKST(summary.lastStampedAt) : "—"
              }`}
            />
          </div>

          <Card className="mt-3 space-y-2.5 p-3">
            <div className="flex flex-wrap items-center gap-2">
              <FilterGroup
                options={STANCE_FILTERS}
                value={stanceFilter}
                onChange={(v) => setStanceFilter(v as StampStance | "all")}
              />
              <FilterGroup
                options={VERDICT_FILTERS}
                value={verdictFilter}
                onChange={(v) => setVerdictFilter(v as StampVerdictFilter)}
              />
              <FilterGroup
                className="ml-auto"
                options={[
                  { id: "timeline", label: "타임라인" },
                  { id: "symbol", label: "종목별" },
                ]}
                value={grouping}
                onChange={(v) => setGrouping(v as "timeline" | "symbol")}
              />
            </div>
          </Card>

          <Card className="mt-3 p-0">
            <CardHeader className="mb-0 border-b border-[var(--border)] px-4 py-3">
              <CardTitle>스탬프 목록</CardTitle>
              {!loading && (
                <Badge variant="info">
                  {stamps.length}건{totalCount !== stamps.length && ` / 전체 ${totalCount}건`}
                </Badge>
              )}
            </CardHeader>

            {loading ? (
              <div className="space-y-2 p-4">
                <div className="h-16 animate-pulse rounded-lg bg-[var(--surface-elevated)]" />
                <div className="h-16 animate-pulse rounded-lg bg-[var(--surface-elevated)]" />
              </div>
            ) : error ? (
              <p className="p-4 text-center text-sm text-[var(--text-muted)]">{error}</p>
            ) : stamps.length === 0 ? (
              <div className="p-6 text-center">
                <StampIcon className="mx-auto h-6 w-6 text-[var(--text-muted)]" />
                <p className="mt-2 text-sm text-[var(--text-muted)]">
                  {totalCount === 0
                    ? "아직 찍은 스탬프가 없습니다. 종목 화면의 도장 버튼으로 지금 이 순간의 예상 주가를 저장해보세요."
                    : "조건에 맞는 스탬프가 없습니다."}
                </p>
              </div>
            ) : (
              <div className="@container">
                {grouped.map((group) => (
                  <section key={group.key}>
                    <h3 className="border-b border-[var(--border)] bg-[var(--surface-elevated)]/50 px-4 py-1.5 text-cell font-medium text-[var(--text-muted)]">
                      {group.label} · {group.items.length}건
                    </h3>
                    <ul className="divide-y divide-[var(--border)]">
                      {group.items.map((stamp) => (
                        <li key={stamp.id}>
                          <StampRow stamp={stamp} onOpen={() => setDetailId(stamp.id)} />
                        </li>
                      ))}
                    </ul>
                  </section>
                ))}
              </div>
            )}
          </Card>
          </SampleWrap>
        </PageColumns>
        )}
      </PageScrollShell>

      <StampCreateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={reload}
      />

      <StampDetailModal
        stamp={detailStamp}
        readOnly={sample}
        onOpenChange={(open) => !open && setDetailId(null)}
        onUpdate={(input) => updateStamp(detailStamp!.id, input)}
        onOpen={() => detailStamp && openStamp(detailStamp.id)}
        onDelete={async () => {
          if (!detailStamp) return;
          if (
            await confirm({
              title: "이 스탬프를 삭제할까요?",
              description: "찍을 당시의 시세·점수·뉴스 스냅샷은 복구할 수 없습니다.",
              confirmLabel: "삭제",
              tone: "danger",
            })
          ) {
            removeStamp(detailStamp.id);
            setDetailId(null);
          }
        }}
      />

      <StampScorecardModal
        open={scorecardOpen}
        onOpenChange={setScorecardOpen}
        stamps={allStamps}
        summary={summary}
        onOpenReveal={() => setRevealOpen(true)}
      />

      <StampRevealModal
        open={revealOpen}
        onOpenChange={setRevealOpen}
        stamps={dueStamps}
        onOpen={openStamp}
        onAnnotate={updateStamp}
      />
    </>
  );
}

function HitRateRing({ value }: { value?: number }) {
  const pct = value ?? 0;
  return (
    <div
      className="relative size-[54px] shrink-0 rounded-full"
      style={{
        background: `conic-gradient(var(--profit) 0 ${pct}%, var(--surface-elevated) ${pct}% 100%)`,
      }}
      role="img"
      aria-label={value === undefined ? "적중률 없음" : `적중률 ${Math.round(pct)}%`}
    >
      <span className="absolute inset-[7px] rounded-full bg-[var(--surface)]" />
      <span className="absolute inset-0 grid place-items-center text-note font-bold text-[var(--text-primary)]">
        {value === undefined ? "—" : `${Math.round(pct)}%`}
      </span>
    </div>
  );
}

function Tile({
  label,
  value,
  valueClass,
  foot,
  onClick,
}: {
  label: string;
  value: string;
  valueClass?: string;
  foot: string;
  onClick?: () => void;
}) {
  const Element = onClick ? "button" : "div";
  return (
    <Element
      {...(onClick ? { type: "button" as const, onClick } : {})}
      className={cn(
        "rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 text-left",
        onClick && "transition-colors hover:border-[var(--brand)]",
      )}
    >
      <p className="text-cell text-[var(--text-muted)]">{label}</p>
      <p className={cn("mt-0.5 text-xl font-bold tracking-tight tabular-nums", valueClass)}>
        {value}
      </p>
      <p className="mt-0.5 truncate text-micro text-[var(--text-muted)]">{foot}</p>
    </Element>
  );
}

/**
 * 목록 행 (B-1).
 *
 * 왼쪽 실링이 판정을, 가운데가 예측과 결과를, 오른쪽이 목표까지의 진행률 → 이후 경로 →
 * 4구간 수익률을 맡는다. 오른쪽 셋은 좁아지면 역순으로 접힌다 — 컨테이너 쿼리를 쓰는
 * 이유는 관심종목 행과 같다: 사이드바와 레일을 뺀 **본문 폭**이 기준이라야 데스크탑에서
 * 가로 오버플로가 나지 않는다.
 */
function StampRow({ stamp, onOpen }: { stamp: Stamp; onOpen: () => void }) {
  const currency = stamp.currency === "USD" ? "USD" : "KRW";

  return (
    <button
      type="button"
      onClick={onOpen}
      className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left hover:bg-[var(--surface-elevated)]/50"
    >
      <StampSeal stamp={stamp} />

      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-1.5">
          <StockAvatar symbol={stamp.symbol} name={stamp.symbolName} size="xs" />
          <span className="truncate text-cell font-semibold text-[var(--text-primary)]">
            {stamp.symbolName || stamp.symbol}
          </span>
          <StanceBadge stance={stamp.stance} />
          <Badge className="px-1.5 py-0 text-micro">확신 {stamp.conviction}</Badge>
        </div>

        <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-micro text-[var(--text-muted)]">
          <span>
            {formatDateTimeKST(stamp.stampedAt)} → {formatDateKST(stamp.dueAt)} 개봉
            <span className="ml-1 opacity-70">({HORIZON_LABELS[stamp.horizonKind]})</span>
          </span>
          <span className="tabular-nums text-[var(--text-secondary)]">
            <PriceDisplay value={stamp.price} currency={currency} size="inherit" />
            {stamp.current && (
              <>
                {" → "}
                <b><PriceDisplay value={stamp.current.price} currency={currency} size="inherit" /></b>
                <span className={cn("ml-1", toneClass(stamp.current.changeRate))}>
                  {stamp.current.changeRate === undefined
                    ? "—"
                    : formatPercent(stamp.current.changeRate)}
                </span>
              </>
            )}
          </span>
          <span className="tabular-nums">목표 <PriceDisplay value={stamp.targetPrice} currency={currency} size="inherit" /></span>
        </div>

        {stamp.note && (
          <p className="mt-0.5 line-clamp-1 text-micro text-[var(--text-secondary)]">
            {stamp.note}
          </p>
        )}
      </div>

      <StampTargetGauge stamp={stamp} className="hidden w-[132px] shrink-0 @2xl:block" />

      <span className="hidden h-7 w-[84px] shrink-0 @4xl:block">
        <StampPathSpark stamp={stamp} />
      </span>

      <div className="hidden shrink-0 gap-1 @3xl:flex">
        {stamp.returns.map((r) => (
          <HorizonCell key={r.horizon} label={r.horizon} value={r.return} />
        ))}
      </div>
    </button>
  );
}

/** 4구간 수익률 한 칸 — 숫자와 0선 기준 막대. */
function HorizonCell({ label, value }: { label: string; value?: number }) {
  // 막대는 ±10%를 꽉 찬 높이로 본다. 그 이상은 잘리지만 "크다"는 사실만 전하면 된다.
  const height = value === undefined ? 0 : Math.min(9, (Math.abs(value) / 10) * 9);
  return (
    <span className="w-[42px] text-center">
      <span className="block text-micro text-[var(--text-muted)]">{label}</span>
      <span className="relative my-0.5 block h-[18px]">
        <span className="absolute inset-x-[11px] top-[9px] h-px bg-[var(--border)]" />
        {value !== undefined && value !== 0 && (
          <span
            className={cn(
              "absolute inset-x-[11px] rounded-sm",
              value > 0
                ? "bottom-[9px] bg-[var(--profit)]"
                : "top-[9px] bg-[var(--loss)]",
            )}
            style={{ height: `${Math.max(1, height)}px` }}
          />
        )}
      </span>
      <span className={cn("block text-micro tabular-nums", toneClass(value))}>
        {value === undefined ? "—" : value.toFixed(1)}
      </span>
    </span>
  );
}

/**
 * 스탬프 이후 경로 미니 스파크. 목표가를 파선으로 함께 그려서 "얼마나 모자랐나"가
 * 선 하나로 읽히게 한다. 점은 4구간이 정산한 종가뿐 — 스탬프가 아는 이후 가격은
 * 그것뿐이라 없는 데이터를 그리지 않는다.
 */
function StampPathSpark({ stamp }: { stamp: Stamp }) {
  const points = stamp.returns
    .filter((r) => r.price !== undefined && r.asOfDate)
    .sort((a, b) => (a.asOfDate! < b.asOfDate! ? -1 : 1))
    .map((r) => r.price as number);

  if (points.length === 0) return null;

  const series = [stamp.price, ...points];
  const values = [...series, stamp.targetPrice].filter((v) => v > 0);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const W = 84;
  const H = 28;
  const y = (v: number) => 3 + (H - 6) * (1 - (v - min) / span);
  const x = (i: number) => (series.length === 1 ? 0 : (i / (series.length - 1)) * W);
  const last = series[series.length - 1];
  const rising = last >= stamp.price;

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="size-full" preserveAspectRatio="none" role="img">
      <line
        x1="0"
        y1={y(stamp.targetPrice)}
        x2={W}
        y2={y(stamp.targetPrice)}
        stroke={stamp.stance === "bearish" ? "var(--loss)" : "var(--profit)"}
        strokeWidth="1"
        strokeDasharray="2 2"
        strokeOpacity="0.5"
        vectorEffect="non-scaling-stroke"
      />
      <polyline
        points={series.map((v, i) => `${x(i)},${y(v)}`).join(" ")}
        fill="none"
        stroke={rising ? "var(--profit)" : "var(--loss)"}
        strokeWidth="1.5"
        vectorEffect="non-scaling-stroke"
      />
      <circle cx={x(0)} cy={y(stamp.price)} r="2.2" fill="var(--brand)" />
    </svg>
  );
}

function FilterGroup({
  options,
  value,
  onChange,
  className,
}: {
  options: { id: string; label: string }[];
  value: string;
  onChange: (id: string) => void;
  className?: string;
}) {
  return (
    <div className={cn("flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5", className)}>
      {options.map((opt) => (
        <button
          key={opt.id}
          type="button"
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
