"use client";

import { useEffect, useState } from "react";
import { Clock3, Eye, Heart, MessageSquare } from "lucide-react";
import {
  ContrarianBadge,
  DirectionBadge,
  HorizonProgress,
  StatusBadge,
  daysUntilDue,
} from "@/components/research/research-shared";
import {
  ResearchAxisBars,
  ResearchSparkline,
  ResearchThumbnail,
} from "@/components/research/research-visuals";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { SectionCard, SectionEmpty } from "@/components/ui/section-card";
import { formatPercent, formatPrice } from "@/lib/format";
import { markdownExcerpt, readingMinutes } from "@/lib/markdown";
import { fetchResearchCallsClient } from "@/lib/research-api";
import type { ResearchCall } from "@/lib/research-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 홈 「최신 투자 리포트」 — 리포트 화면과 **같은 가로형 카드**.
 *
 * 예전 카드에는 방향 배지·종목명·제목·저자 넷뿐이었다. 그런데 목록 응답에는
 * `chart.bars`(200포인트)·`evidence.breakdown`(5축)·`progress`·`basePrice`/`targetPrice`·
 * 조회·좋아요가 **이미 전부 실려 온다**(실측). 즉 화면이 부족했던 게 아니라 받아 놓고
 * 안 쓴 것이었다. 그래서 여기서 채워도 **추가 요청은 0**이다.
 *
 * 좌·우 두 칸으로 나눈 이유는 성격이 다르기 때문이다 — 왼쪽은 **근거**(표지·발행 시점
 * 캔들·5축), 오른쪽은 **주장**(제목·발췌·목표 진행). 왼쪽 칸은 위아래로 늘어나
 * 빈자리를 남기지 않는다: 표지가 남는 높이를 가져가고, 5축은 값이 있는 축만 그린다.
 *
 * 카드 전체가 퀵룩을 여는 버튼이다. 전문 링크는 모달 안에 있다 — 목록에서 곧장 페이지를
 * 떠나면 비교하던 다른 리포트와 스크롤 위치를 잃는다.
 */
export function LatestReportsCard({ onPick }: { onPick: (call: ResearchCall) => void }) {
  const [calls, setCalls] = useState<ResearchCall[] | null>(null);

  useEffect(() => {
    let alive = true;
    fetchResearchCallsClient({ limit: 4 })
      .then((rows) => {
        if (alive) setCalls(rows.slice(0, 4));
      })
      .catch(() => {
        if (alive) setCalls([]);
      });
    return () => {
      alive = false;
    };
  }, []);

  return (
    <SectionCard title="최신 투자 리포트" href="/reports" hrefLabel="리포트">
      {calls === null ? (
        <div className="grid min-w-0 gap-3 xl:grid-cols-2">
          {Array.from({ length: 2 }).map((_, i) => (
            <div key={i} className="h-[196px] animate-pulse rounded-xl border bg-muted/40" />
          ))}
        </div>
      ) : calls.length === 0 ? (
        <SectionEmpty text="아직 발행된 리포트가 없습니다" />
      ) : (
        <div className="grid min-w-0 gap-3 xl:grid-cols-2">
          {calls.map((call) => (
            <ReportCard key={call.id} call={call} onOpen={() => onPick(call)} />
          ))}
        </div>
      )}
    </SectionCard>
  );
}

function ReportCard({ call, onOpen }: { call: ResearchCall; onOpen: () => void }) {
  const currency = call.currency ?? "KRW";
  const excerpt = markdownExcerpt(call.bodyMd, 130);
  const dueDays = daysUntilDue(call.dueAt, call.status);
  // 목표까지 남은 거리는 **서버가 계산해서 보낸다**(`toTargetPct`). 화면에서 다시 구하면
  // 리포트 목록과 상세가 다른 기준가로 다른 숫자를 말할 수 있다.
  const toTarget = call.toTargetPct;

  return (
    <button
      type="button"
      onClick={onOpen}
      className="grid min-w-0 grid-cols-[132px_minmax(0,1fr)] items-stretch gap-3 rounded-xl border bg-card p-3 text-left transition-colors hover:border-[var(--brand)]"
    >
      {/* 근거 — 위아래로 늘어나 아래쪽에 빈자리를 남기지 않는다. */}
      <div className="flex min-w-0 flex-col gap-2">
        <span className="block flex-1 overflow-hidden rounded-lg">
          <ResearchThumbnail call={call} />
        </span>
        <ResearchSparkline call={call} height={40} showLegend={false} />
        <ResearchAxisBars call={call} />
      </div>

      {/* 주장 */}
      <div className="flex min-w-0 flex-col gap-1.5">
        <div className="flex min-w-0 flex-wrap items-center gap-1">
          <StockAvatar
            symbol={call.symbol}
            name={call.symbolName}
            currency={currency}
            size="xs"
            className="shrink-0"
          />
          <span className="truncate text-[11px] text-muted-foreground">
            {call.symbolName || call.symbol}
          </span>
          <DirectionBadge direction={call.direction} />
          <StatusBadge status={call.status} />
          {call.contrarian && <ContrarianBadge />}
          {dueDays !== null && (
            <span className="ml-auto shrink-0 rounded-md bg-[var(--brand-muted)] px-1.5 py-0.5 text-[10px] font-semibold text-[var(--brand)]">
              D-{dueDays}
            </span>
          )}
        </div>

        <h3 className="line-clamp-2 text-[13px] font-semibold leading-snug">{call.title}</h3>

        {excerpt && (
          <p className="line-clamp-2 text-[11px] leading-relaxed text-muted-foreground">
            {excerpt}
          </p>
        )}

        <div className="flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[10px] text-muted-foreground">
          {call.authorName && <span className="text-foreground/70">{call.authorName}</span>}
          <span className="inline-flex items-center gap-0.5">
            <Clock3 className="size-3" />
            {readingMinutes(call.bodyMd)}분
          </span>
          <span className="inline-flex items-center gap-0.5">
            <Eye className="size-3" />
            {call.viewCount.toLocaleString()}
          </span>
          <span className="inline-flex items-center gap-0.5">
            <Heart className={cn("size-3", call.liked && "fill-rose-400 text-rose-400")} />
            {call.likeCount}
          </span>
          <span className="inline-flex items-center gap-0.5">
            <MessageSquare className="size-3" />
            {call.commentCount}
          </span>
        </div>

        {/* 남는 높이를 진행 블록이 가져가 카드 바닥까지 채운다. */}
        <div className="mt-auto pt-1.5">
          <HorizonProgress
            progress={call.progress}
            publishedAt={call.publishedAt}
            dueAt={call.dueAt}
            status={call.status}
          />
          <p className="mt-1 flex items-baseline justify-between gap-2 text-[10px] text-muted-foreground">
            <span className="truncate font-mono tabular-nums">
              {formatPrice(call.basePrice, currency)}
              {call.targetPrice !== undefined && (
                <> → {formatPrice(call.targetPrice, currency)}</>
              )}
            </span>
            {toTarget != null && (
              <span className={cn("shrink-0 font-mono tabular-nums", trendTextClass(toTarget))}>
                목표까지 {formatPercent(toTarget)}
              </span>
            )}
          </p>
        </div>
      </div>
    </button>
  );
}
