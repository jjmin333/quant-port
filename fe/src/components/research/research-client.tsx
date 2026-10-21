"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { Clock3, Eye, Heart, MessageSquare, PenLine } from "lucide-react";
import { PageColumns } from "@/components/layout/page-columns";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { Card } from "@/components/ui/card";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { ResearchSideRail } from "@/components/research/research-side-rail";
import {
  ContrarianBadge,
  daysUntilDue,
  DirectionBadge,
  StatusBadge,
  toneClass,
} from "@/components/research/research-shared";
import {
  ResearchThumbnail,
} from "@/components/research/research-visuals";
import {
  useResearchCalls,
  type ResearchScope,
  type ResearchSort,
} from "@/hooks/use-research";
import { formatDateKST, formatPercent } from "@/lib/format";
import { markdownExcerpt, readingMinutes } from "@/lib/markdown";
import {
  VISIBILITY_LABELS,
  type ResearchCall,
  type ResearchDirection,
} from "@/lib/research-types";
import { cn } from "@/lib/utils";
import { TopicBadge } from "@/components/common/topic-ui";
import { ASSET_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { TOPIC_FILTERS, type ContentTopic } from "@/lib/topics";
import { buttonVariants } from "@/components/ui/button";

/**
 * 투자 리포트 목록 (사용자에게 보이는 이름은 "리포트", 코드·API는 research).
 *
 * # 이 화면은 읽을 것을 고르는 자리다 (2026-09-14 사용자 요청 4)
 *
 * 예전 카드는 왼쪽에 표지·스파크라인·5축 막대를, 오른쪽에 제목·발췌·진행축·알파·
 * 수익률을 담은 **계기판**이었다. 근거와 주장을 나란히 세워 견주자는 뜻이었는데,
 * 그러다 보니 카드 한 장에 차트 셋과 숫자 여섯이 서고 **제목과 발췌가 그 사이에서
 * 가장 안 읽히는 것**이 됐다. 그리고 카드를 누르면 글이 아니라 그 계기판을 한 번
 * 더 보여 주는 모달이 떴다.
 *
 * 지금은 제목·발췌·작성자가 카드의 주인공이고, 누르면 곧장 글로 간다. 견주는 일은
 * 사이드 레일(「곧 판정됩니다」·「가격대 근처」)과 정렬이 맡는다 — 그쪽이 원래
 * 그 일을 하려고 만든 자리다.
 *
 * 여기 그려지는 것은 전부 목록 응답에 이미 있는 값이다 — 추가 요청 0.
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

export function ResearchClient({ initialCalls = [] }: { initialCalls?: ResearchCall[] }) {
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
  } = useResearchCalls("feed", initialCalls);

  const [symbolFilter, setSymbolFilter] = useState<string | null>(null);
  // 주제(2026-10-09 요청 9) — 부동산·금융상품은 판정 없는 분석 리포트다. 의견·역발상 필터는 주식에만 뜻이 있다.
  const [topicFilter, setTopicFilter] = useState<ContentTopic | "">("");

  const visible = useMemo(
    () =>
      calls.filter(
        (c) =>
          (!symbolFilter || c.symbol === symbolFilter) &&
          (!topicFilter || (c.topic ?? "stock") === topicFilter),
      ),
    [calls, symbolFilter, topicFilter],
  );
  const stockCalls = useMemo(() => allCalls.filter((c) => c.status !== "note"), [allCalls]);
  const symbolFilterName = symbolFilter
    ? allCalls.find((c) => c.symbol === symbolFilter)?.symbolName || symbolFilter
    : null;

  return (
    <>
      <PageScrollShell>
        <PageColumns
          rail={
            <ResearchSideRail
              summary={summary}
              allCalls={stockCalls}
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
                className={cn(buttonVariants({ variant: "default", size: "default" }), "flex-1 bg-[var(--brand)] text-[var(--brand-foreground)] transition-opacity hover:opacity-90 sm:flex-none")}
              >
                <PenLine className="h-3.5 w-3.5" />
                리포트 쓰기
              </Link>
            </div>

            {/* 왼쪽은 "무엇을 보느냐"(의견), 오른쪽 끝은 "어떤 순서로 보느냐"(정렬).
                좁은 화면에서는 정렬 그룹이 아래 줄로 내려가되 계속 오른쪽에 붙는다. */}
            <div className="flex flex-wrap items-center gap-1.5">
              {!ASSET_FEATURES_HIDDEN && (
                <FilterGroup
                  options={TOPIC_FILTERS}
                  value={topicFilter}
                  onChange={(id) => setTopicFilter(id as ContentTopic | "")}
                />
              )}
              <FilterGroup
                options={DIRECTION_FILTERS}
                value={directionFilter}
                onChange={(id) => setDirectionFilter(id as ResearchDirection | "all")}
              />
              <button
                type="button"
                onClick={() => setContrarianOnly(!contrarianOnly)}
                className={cn(
                  "shrink-0 rounded-full px-2.5 py-1 text-cell font-medium transition-colors",
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
                  className="shrink-0 rounded-full bg-[var(--brand-muted)] px-2.5 py-1 text-cell font-medium text-[var(--brand)]"
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
            /* 한 줄에 한 장. 2단으로 쪼개면 제목과 발췌의 폭이 절반이 되어
               「읽을 것을 고른다」는 이 목록의 일이 다시 어려워진다. */
            <div className="mt-3 grid min-w-0 gap-3">
              {visible.map((call) => (
                <ReportCard key={call.id} call={call} />
              ))}
            </div>
          )}

          <p className="px-1 pb-2 pt-4 text-micro leading-relaxed text-[var(--text-muted)]">
            이 서비스는 발행 시점 데이터를 박제하고 결과를 채점합니다.
          </p>
        </PageColumns>
      </PageScrollShell>
    </>
  );
}

/**
 * 목록 카드 — 표지 한 장과 **읽을거리**(제목·발췌·작성자).
 *
 * # 카드는 이제 그냥 링크다 (2026-09-14 사용자 요청 4)
 *
 * 예전에는 좌클릭을 가로채 퀵룩 모달을 열었다. 그 판단의 근거는 「리포트를 훑는 일은
 * 하나를 정독하는 게 아니라 여럿을 견주는 일」이었고, 그것이 **이 메뉴가 만들어진
 * 목적과 어긋났다.** 이 화면은 남이 쓴 투자 인사이트를 **읽는** 곳이다. 글을 눌렀을
 * 때 나오는 것은 글이어야 하고, 모달은 그 글의 요약만 보여 주고 다시 한 번 누르게
 * 만들었다 — 게다가 그 모달에는 댓글이 없어서, 리포트에 답을 다는 길이 목록에서
 * 두 번 건너야 닿는 자리에 있었다.
 *
 * 견주는 일은 사이드 레일(「곧 판정됩니다」·「가격대 근처」)과 정렬이 맡는다.
 *
 * # 카드에서 걷어 낸 것
 *
 * 스파크라인·5축 막대·가격대 축·진행 막대·알파/수익률은 전부 **상세로 내렸다.**
 * 목록 카드 한 장에 차트 셋과 숫자 여섯이 있으면 제목과 발췌가 그 사이에서 가장
 * 안 읽히는 것이 된다. 성과 숫자 하나(알파)만 남긴다 — 「이 사람 말이 맞았나」는
 * 목록에서도 궁금한 하나다.
 */
function ReportCard({ call }: { call: ResearchCall }) {
  const alpha = call.verdictAlpha ?? call.horizon?.alpha;
  const excerpt = markdownExcerpt(call.bodyMd, 180);
  const dueDays = daysUntilDue(call.dueAt, call.status);

  return (
    /*
      표지는 **어느 폭에서나 오른쪽의 작은 사각형**이다.

      모바일에서 위로 올려 전체 폭 배너로 두면, 한 카드가 화면 하나를 거의 다 먹고
      제목이 접힌 아래에 선다(실측 375px: 표지만 235px). 읽을 것을 고르는 목록에서
      가장 늦게 보이면 안 되는 것이 제목이라, 표지는 88px 사각형으로 옆에 세운다 —
      네이버·미디엄 같은 읽기 피드가 모바일에서 쓰는 모양이다.
    */
    <Link
      href={`/reports/${call.id}`}
      className="group grid min-w-0 grid-cols-[minmax(0,1fr)_88px] gap-3 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 text-left transition-colors hover:border-[var(--brand)] sm:grid-cols-[minmax(0,1fr)_160px] sm:gap-4"
    >
      <div className="min-w-0 space-y-2">
        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          {call.status === "note" ? (
            <>
              <TopicBadge topic={call.topic} />
              <span className="text-cell font-medium text-[var(--text-secondary)]">분석 리포트</span>
            </>
          ) : (
            <>
              <StockAvatar symbol={call.symbol} name={call.symbolName} size="xs" />
              <span className="truncate text-cell font-medium text-[var(--text-secondary)]">
                {call.symbolName || call.symbol}
              </span>
              <DirectionBadge direction={call.direction} />
              <StatusBadge status={call.status} />
            </>
          )}
          {call.contrarian && <ContrarianBadge />}
          {call.visibility !== "public" && (
            <span className="text-micro text-[var(--text-muted)]">
              {VISIBILITY_LABELS[call.visibility]}
            </span>
          )}
          {dueDays !== null && (
            <span className="shrink-0 rounded-md bg-[var(--brand-muted)] px-1.5 py-0.5 text-micro font-semibold text-[var(--brand)]">
              D-{dueDays}
            </span>
          )}
        </div>

        <h2 className="text-lg font-semibold leading-snug text-[var(--text-primary)] group-hover:text-[var(--brand)]">
          {call.title}
        </h2>

        {excerpt && (
          <p className="line-clamp-3 text-note leading-relaxed text-[var(--text-secondary)]">
            {excerpt}
          </p>
        )}

        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-cell text-[var(--text-muted)]">
          {call.authorName && (
            <span className="font-medium text-[var(--text-secondary)]">{call.authorName}</span>
          )}
          {call.authorBadge && (
            <span className="rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] px-1.5 py-px text-micro font-semibold text-[var(--text-secondary)]">
              {call.authorBadge}
            </span>
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
          {/*
            댓글 수는 **색이 붙는다**(0이 아닐 때). 이 화면이 커뮤니티인 이유가
            그 숫자이고, 다른 회색 숫자 다섯 사이에 같은 회색으로 서면 아무도
            그게 대화라는 것을 모른다.
          */}
          <span
            className={cn(
              "inline-flex items-center gap-1",
              call.commentCount > 0 && "font-medium text-[var(--brand)]",
            )}
          >
            <MessageSquare className="h-3 w-3" />
            {call.commentCount}
          </span>
          {alpha !== undefined && (
            <span className="text-micro">
              알파{" "}
              <span className={cn("font-semibold tabular-nums", toneClass(alpha))}>
                {formatPercent(alpha)}p
              </span>
            </span>
          )}
        </div>
      </div>

      <span className="block aspect-square overflow-hidden rounded-lg sm:aspect-[4/3]">
        <ResearchThumbnail call={call} />
      </span>
    </Link>
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
      <p className="text-cell leading-relaxed text-[var(--text-muted)]">
        종목을 고르면 발행 시점의 점수·시세·뉴스가 자동으로 박제되고,
        <br />
        투자 포인트와 주요 가격대가 담긴 초안이 마크다운으로 미리 채워집니다.
      </p>
      <Link
        href="/reports/new"
        className={cn(buttonVariants({ variant: "default", size: "default" }), "bg-[var(--brand)] text-[var(--brand-foreground)]")}
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
            "shrink-0 rounded-md px-2.5 py-1 text-cell font-medium transition-colors",
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
