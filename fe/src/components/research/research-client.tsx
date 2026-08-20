"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { Clock3, Eye, Heart, MessageSquare, PenLine } from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { PageColumns } from "@/components/layout/page-columns";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { Card } from "@/components/ui/card";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { ResearchQuickLookModal } from "@/components/research/research-quick-look-modal";
import { ResearchSideRail } from "@/components/research/research-side-rail";
import {
  ContrarianBadge,
  daysUntilDue,
  DirectionBadge,
  HorizonProgress,
  StatusBadge,
  toneClass,
} from "@/components/research/research-shared";
import {
  ResearchAxisBars,
  ResearchLevelAxis,
  ResearchSparkline,
  ResearchThumbnail,
} from "@/components/research/research-visuals";
import {
  useResearchCalls,
  type ResearchScope,
  type ResearchSort,
} from "@/hooks/use-research";
import { formatDateKST, formatPercent, formatPrice } from "@/lib/format";
import { markdownExcerpt, readingMinutes } from "@/lib/markdown";
import {
  VISIBILITY_LABELS,
  type ResearchCall,
  type ResearchDirection,
} from "@/lib/research-types";
import { cn } from "@/lib/utils";

/**
 * 투자 리포트 목록 (사용자에게 보이는 이름은 "리포트", 코드·API는 research).
 *
 * 카드는 **가로형**이다 — 왼쪽에 표지·발행 시점 차트·5축 점수, 오른쪽에 제목·발췌·
 * 진행축. 근거(서버가 잰 것)와 주장(작성자가 쓴 것)이 나란히 서야 둘을 견줄 수 있다.
 *
 * 여기 그려지는 그림은 전부 목록 응답에 이미 있는 값이다(`chart.bars`,
 * `evidence.breakdown`, `toTargetPct`) — 추가 요청 0.
 *
 * 작성자 순위표는 없다 — 줄을 세우면 리포트가 대회 참가작이 되고, 대회는 보상
 * 구조 없이는 유지되지 않는다(docs/RESEARCH_CALL_DESIGN.md §15-1).
 */

// 전체 리포트가 먼저이자 기본값이다 — 처음 들어온 사람에게 빈 "내 리포트"를
// 보여주면 이 화면이 무엇을 하는 곳인지 알 수 없다.
const SCOPES: { id: ResearchScope; label: string }[] = [
  { id: "feed", label: "전체 리포트" },
  { id: "mine", label: "내 리포트" },
];

const DIRECTION_FILTERS: { id: ResearchDirection | "all"; label: string }[] = [
  { id: "all", label: "전체 의견" },
  { id: "buy", label: "매수" },
  { id: "sell", label: "매도" },
  { id: "hold", label: "관망" },
];

// Raw return is deliberately absent as a sort option — ordering by headline
// gain promotes whatever the market carried, which is the failure this feature
// exists to avoid.
const SORTS: { id: ResearchSort; label: string }[] = [
  { id: "recent", label: "최신" },
  { id: "views", label: "조회 순" },
  { id: "likes", label: "추천 순" },
];

export function ResearchClient() {
  const {
    scope,
    setScope,
    calls,
    allCalls,
    totalCount,
    summary,
    loading,
    error,
    directionFilter,
    setDirectionFilter,
    contrarianOnly,
    setContrarianOnly,
    sort,
    setSort,
  } = useResearchCalls("feed");

  const [openId, setOpenId] = useState<number | null>(null);
  const [symbolFilter, setSymbolFilter] = useState<string | null>(null);

  const visible = useMemo(
    () => (symbolFilter ? calls.filter((c) => c.symbol === symbolFilter) : calls),
    [calls, symbolFilter],
  );
  const openCall = useMemo(
    () => allCalls.find((c) => c.id === openId) ?? null,
    [allCalls, openId],
  );
  const symbolFilterName = symbolFilter
    ? allCalls.find((c) => c.symbol === symbolFilter)?.symbolName || symbolFilter
    : null;

  return (
    <>
      <PageHeader
        title="리포트"
        subtitle="종목 논거를 글로 남기고, 발행 시점 근거와 함께 시장에 채점받습니다"
      />
      <PageScrollShell>
        <PageColumns
          rail={
            <ResearchSideRail
              summary={summary}
              allCalls={allCalls}
              onOpenCall={setOpenId}
              onPickSymbol={(symbol) =>
                setSymbolFilter((cur) => (cur === symbol ? null : symbol))
              }
              symbolFilter={symbolFilter}
            />
          }
        >
          <div className="space-y-2.5">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <FilterGroup
                options={SCOPES}
                value={scope}
                onChange={(id) => setScope(id as ResearchScope)}
              />
              <Link
                href="/reports/new"
                className="inline-flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-[var(--brand)] px-3.5 py-2 text-xs font-medium text-white transition-opacity hover:opacity-90 sm:flex-none"
              >
                <PenLine className="h-3.5 w-3.5" />
                리포트 쓰기
              </Link>
            </div>

            {/* 왼쪽은 "무엇을 보느냐"(의견), 오른쪽 끝은 "어떤 순서로 보느냐"(정렬).
                좁은 화면에서는 정렬 그룹이 아래 줄로 내려가되 계속 오른쪽에 붙는다. */}
            <div className="flex flex-wrap items-center gap-1.5">
              <FilterGroup
                options={DIRECTION_FILTERS}
                value={directionFilter}
                onChange={(id) => setDirectionFilter(id as ResearchDirection | "all")}
              />
              <button
                type="button"
                onClick={() => setContrarianOnly(!contrarianOnly)}
                className={cn(
                  "shrink-0 rounded-full px-2.5 py-1 text-xs font-medium transition-colors",
                  contrarianOnly
                    ? "bg-amber-500/20 text-amber-400"
                    : "bg-[var(--surface-elevated)] text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                )}
                title="우리 점수와 반대 방향인 리포트만 봅니다"
              >
                역발상만
              </button>
              {symbolFilterName && (
                <button
                  type="button"
                  onClick={() => setSymbolFilter(null)}
                  className="shrink-0 rounded-full bg-[var(--brand-muted)] px-2.5 py-1 text-xs font-medium text-[var(--brand)]"
                >
                  {symbolFilterName} ✕
                </button>
              )}
              <div className="ml-auto">
                <FilterGroup
                  options={SORTS}
                  value={sort}
                  onChange={(id) => setSort(id as ResearchSort)}
                />
              </div>
            </div>
          </div>

          {error && <Card className="mt-3 p-4 text-sm text-[var(--loss)]">{error}</Card>}

          {loading ? (
            <Card className="mt-3 p-6 text-center text-sm text-[var(--text-muted)]">
              불러오는 중…
            </Card>
          ) : visible.length === 0 ? (
            <div className="mt-3">
              <EmptyState scope={scope} filtered={totalCount > 0} />
            </div>
          ) : (
            /* 가로형 카드는 한 장이 이미 넓다 — 세로 1단으로 쌓는다. 2단으로 쪼개면
               왼쪽 근거 칸이 차트를 담기에 좁아진다. */
            <div className="mt-3 grid min-w-0 gap-3">
              {visible.map((call) => (
                <ReportCard key={call.id} call={call} onOpen={() => setOpenId(call.id)} />
              ))}
            </div>
          )}

          <p className="px-1 pb-2 pt-4 text-[10px] leading-relaxed text-[var(--text-muted)]">
            투자 리포트는 작성자 개인의 의견이며, 이 서비스는 발행 시점 데이터를 박제하고 결과를
            채점할 뿐 특정 종목의 매수·매도를 권유하지 않습니다.
          </p>
        </PageColumns>

        <ResearchQuickLookModal
          call={openCall}
          open={openId !== null}
          onClose={() => setOpenId(null)}
        />
      </PageScrollShell>
    </>
  );
}

/**
 * 가로형 카드 — 왼쪽 근거(표지·차트·5축), 오른쪽 주장(제목·발췌·진행축).
 *
 * 카드 전체가 퀵룩을 여는 버튼이다. 전문으로 가는 링크는 모달 안에 있다 — 목록에서
 * 곧장 페이지를 떠나면 비교하던 다른 리포트와 스크롤 위치를 잃는다.
 */
function ReportCard({ call, onOpen }: { call: ResearchCall; onOpen: () => void }) {
  const currency = call.currency ?? "KRW";
  const alpha = call.verdictAlpha ?? call.horizon?.alpha;
  const ret = call.verdictReturn ?? call.horizon?.return ?? call.current?.changeRate;
  const excerpt = markdownExcerpt(call.bodyMd, 150);
  const dueDays = daysUntilDue(call.dueAt, call.status);

  return (
    <button
      type="button"
      onClick={onOpen}
      className="grid min-w-0 gap-3 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 text-left transition-colors hover:border-[var(--brand)] sm:grid-cols-[180px_minmax(0,1fr)]"
    >
      <div className="min-w-0 space-y-2">
        <span className="block aspect-[16/10] overflow-hidden rounded-lg">
          <ResearchThumbnail call={call} />
        </span>
        <ResearchSparkline call={call} height={48} showLegend={false} />
        <ResearchAxisBars call={call} />
      </div>

      <div className="min-w-0 space-y-2">
        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          <StockAvatar symbol={call.symbol} name={call.symbolName} size="xs" />
          <span className="truncate text-xs font-medium text-[var(--text-secondary)]">
            {call.symbolName || call.symbol}
          </span>
          <DirectionBadge direction={call.direction} />
          <StatusBadge status={call.status} />
          {call.contrarian && <ContrarianBadge />}
          {call.visibility !== "public" && (
            <span className="text-[10px] text-[var(--text-muted)]">
              {VISIBILITY_LABELS[call.visibility]}
            </span>
          )}
          {dueDays !== null && (
            <span className="ml-auto shrink-0 rounded-md bg-[var(--brand-muted)] px-1.5 py-0.5 text-[10px] font-semibold text-[var(--brand)]">
              D-{dueDays}
            </span>
          )}
        </div>

        <h2 className="text-base font-semibold leading-snug text-[var(--text-primary)]">
          {call.title}
        </h2>

        {excerpt && (
          <p className="line-clamp-2 text-[13px] leading-relaxed text-[var(--text-muted)]">
            {excerpt}
          </p>
        )}

        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-[var(--text-muted)]">
          {call.authorName && (
            <span className="text-[var(--text-secondary)]">{call.authorName}</span>
          )}
          <span>{formatDateKST(call.publishedAt)}</span>
          <span className="inline-flex items-center gap-1">
            <Clock3 className="h-3 w-3" />
            {readingMinutes(call.bodyMd)}분
          </span>
          <span className="inline-flex items-center gap-1">
            <Eye className="h-3 w-3" />
            {call.viewCount.toLocaleString()}
          </span>
          <span className="inline-flex items-center gap-1">
            <Heart className={cn("h-3 w-3", call.liked && "fill-rose-400 text-rose-400")} />
            {call.likeCount}
          </span>
          <span className="inline-flex items-center gap-1">
            <MessageSquare className="h-3 w-3" />
            {call.commentCount}
          </span>
          {call.rating && (
            <span title="논거 평가 — 명확성·데이터 근거·리스크 언급 평균">
              평가 {call.rating.overall.toFixed(1)}
            </span>
          )}
          {call.revisions > 1 && <span>수정 {call.revisions - 1}회</span>}
        </div>

        <ResearchLevelAxis call={call} />

        <div className="flex flex-col gap-2 border-t border-[var(--border)] pt-2.5 sm:flex-row sm:items-center sm:gap-4">
          <div className="min-w-0 flex-1">
            <HorizonProgress
              progress={call.progress}
              publishedAt={call.publishedAt}
              dueAt={call.dueAt}
              status={call.status}
            />
            <p className="mt-1 text-[10px] text-[var(--text-muted)]">
              {formatPrice(call.basePrice, currency)}
              {call.targetPrice !== undefined && (
                <> → 목표 {formatPrice(call.targetPrice, currency)}</>
              )}
              {call.scoreAtPublish != null && <> · 발행시 점수 {call.scoreAtPublish}</>}
            </p>
          </div>
          <div className="flex shrink-0 gap-4 sm:flex-col sm:gap-0 sm:text-right">
            <span className="text-[10px] text-[var(--text-muted)] sm:block">
              알파{" "}
              <span className={cn("text-sm font-semibold tabular-nums", toneClass(alpha))}>
                {alpha === undefined ? "—" : `${formatPercent(alpha)}p`}
              </span>
            </span>
            <span className="text-[10px] text-[var(--text-muted)] sm:block">
              수익률{" "}
              <span className={cn("text-xs tabular-nums", toneClass(ret))}>
                {ret === undefined ? "—" : formatPercent(ret)}
              </span>
            </span>
          </div>
        </div>
      </div>
    </button>
  );
}

function EmptyState({ scope, filtered }: { scope: ResearchScope; filtered: boolean }) {
  if (filtered) {
    return (
      <Card className="p-6 text-center text-sm text-[var(--text-muted)]">
        조건에 맞는 리포트가 없습니다.
      </Card>
    );
  }
  return (
    <Card className="space-y-3 p-6 text-center">
      <p className="text-sm text-[var(--text-secondary)]">
        {scope === "mine" ? "아직 작성한 리포트가 없습니다." : "공개된 리포트가 없습니다."}
      </p>
      <p className="text-xs leading-relaxed text-[var(--text-muted)]">
        종목을 고르면 발행 시점의 점수·시세·뉴스가 자동으로 박제되고,
        <br />
        투자 포인트와 주요 가격대가 담긴 초안이 마크다운으로 미리 채워집니다.
      </p>
      <Link
        href="/reports/new"
        className="inline-flex items-center gap-1.5 rounded-lg bg-[var(--brand)] px-3.5 py-2 text-xs font-medium text-white"
      >
        <PenLine className="h-3.5 w-3.5" />첫 리포트 쓰기
      </Link>
    </Card>
  );
}

function FilterGroup({
  options,
  value,
  onChange,
}: {
  options: { id: string; label: string }[];
  value: string;
  onChange: (id: string) => void;
}) {
  return (
    <div className="flex shrink-0 gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5">
      {options.map((opt) => (
        <button
          key={opt.id}
          type="button"
          onClick={() => onChange(opt.id)}
          className={cn(
            "shrink-0 rounded-md px-2.5 py-1 text-xs font-medium transition-colors",
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
