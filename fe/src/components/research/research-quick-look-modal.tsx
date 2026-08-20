"use client";

import Link from "next/link";
import { ArrowRight, Clock3, Eye, Heart, MessageSquare } from "lucide-react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
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
} from "@/components/research/research-visuals";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { formatDateKST, formatPercent, formatPrice } from "@/lib/format";
import { markdownExcerpt, readingMinutes } from "@/lib/markdown";
import type { ResearchCall } from "@/lib/research-types";
import { cn } from "@/lib/utils";

/**
 * 리포트 퀵룩.
 *
 * 예전에는 카드를 누르면 **곧장 페이지로 떠났다**. 리포트를 훑는 일은 하나를 정독하는
 * 게 아니라 여럿을 견주는 일이라, 세 개를 비교하려면 뒤로가기를 세 번 눌러야 했다.
 * 근거만 모달에서 훑고, 마음에 들 때 전문으로 넘어간다 — 관심·보유·뉴스가 이미 이 흐름.
 *
 * 여기 있는 값은 전부 목록 응답에 이미 들어 있다. 모달을 연다고 요청이 나가지 않는다.
 */
export function ResearchQuickLookModal({
  call,
  open,
  onClose,
}: {
  call: ResearchCall | null;
  open: boolean;
  onClose: () => void;
}) {
  const currency = call?.currency ?? "KRW";
  const alpha = call?.verdictAlpha ?? call?.horizon?.alpha;
  const ret = call?.verdictReturn ?? call?.horizon?.return ?? call?.current?.changeRate;
  const ev = call?.expectedValue;

  const dueDays = call ? daysUntilDue(call.dueAt, call.status) : null;

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="flex max-h-[88dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-xl">
        {call ? (
          <>
            <DialogHeader className="shrink-0 border-b px-4 py-3 pr-10 text-left">
              <DialogTitle className="flex min-w-0 items-start gap-2 text-sm">
                <StockAvatar
                  symbol={call.symbol}
                  name={call.symbolName}
                  currency={call.currency}
                  size="sm"
                  className="mt-0.5 shrink-0"
                />
                <span className="min-w-0 flex-1 leading-snug">{call.title}</span>
              </DialogTitle>
              <DialogDescription className="flex flex-wrap items-center gap-1.5 text-[11px]">
                <span>{call.symbolName || call.symbol}</span>
                <DirectionBadge direction={call.direction} />
                <StatusBadge status={call.status} />
                {call.contrarian && <ContrarianBadge />}
                {call.authorName && <span>· {call.authorName}</span>}
                <span>· {formatDateKST(call.publishedAt)}</span>
              </DialogDescription>
            </DialogHeader>

            <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-3">
              <ResearchSparkline call={call} height={64} />
              <ResearchLevelAxis call={call} />

              <div className="grid gap-2.5 sm:grid-cols-2">
                <Panel title="발행 시점 박제">
                  <Row label="기준가" value={formatPrice(call.basePrice, currency)} />
                  <Row
                    label="목표 / 손절"
                    value={
                      <>
                        <span className="text-[var(--profit)]">
                          {call.targetPrice !== undefined
                            ? formatPrice(call.targetPrice, currency)
                            : "—"}
                        </span>
                        {" / "}
                        <span className="text-[var(--loss)]">
                          {call.stopPrice !== undefined
                            ? formatPrice(call.stopPrice, currency)
                            : "—"}
                        </span>
                      </>
                    }
                  />
                  <Row
                    label="종합점수"
                    value={
                      call.scoreAtPublish != null
                        ? `${call.scoreAtPublish}${call.gradeAtPublish ? ` (${call.gradeAtPublish})` : ""}`
                        : "—"
                    }
                  />
                  {call.evidence?.peerPct != null && (
                    <Row label="동종 상위" value={`${call.evidence.peerPct}%`} />
                  )}
                  {call.evidence?.per != null && (
                    <Row label="PER / PBR" value={`${call.evidence.per.toFixed(1)} / ${call.evidence.pbr?.toFixed(2) ?? "—"}`} />
                  )}
                  <Row label="확신도" value={`${call.conviction}%`} />
                </Panel>

                <Panel title="기대값">
                  {/*
                    도달 확률(`odds`)은 발행 미리보기에만 실린다 — 발행된 콜의 응답에는
                    없다. 없는 값을 "0%"로 그리느니 이 칸에서 빼는 쪽이 정직하다.
                  */}
                  {!ev && (
                    <p className="pb-1 text-[11px] text-[var(--text-muted)]">
                      목표가가 없어 기대값을 계산하지 않았습니다.
                    </p>
                  )}
                  {ev && (
                    <>
                      {ev.riskReward !== undefined && (
                        <Row label="손익비" value={`${ev.riskReward.toFixed(2)} : 1`} />
                      )}
                      {ev.statedEV !== undefined && (
                        <Row
                          label="기대값 (확신 기준)"
                          value={formatPercent(ev.statedEV)}
                          tone={ev.statedEV}
                        />
                      )}
                      {ev.historicalEV !== undefined && (
                        <Row
                          label="기대값 (실제 적중률)"
                          value={formatPercent(ev.historicalEV)}
                          tone={ev.historicalEV}
                        />
                      )}
                      {ev.statedEV !== undefined && ev.historicalEV !== undefined && (
                        <p className="mt-1 text-[10px] leading-relaxed text-[var(--text-muted)]">
                          두 기대값의 차이가 이 작성자의 낙관 정도입니다.
                        </p>
                      )}
                    </>
                  )}
                </Panel>
              </div>

              {call.evidence?.breakdown && (
                <Panel title="발행 시점 5축">
                  <ResearchAxisBars call={call} />
                </Panel>
              )}

              {markdownExcerpt(call.bodyMd, 260) && (
                <p className="text-[13px] leading-relaxed text-[var(--text-secondary)]">
                  {markdownExcerpt(call.bodyMd, 260)}
                </p>
              )}

              {call.context?.news && call.context.news.length > 0 && (
                <Panel title="발행 시점 뉴스">
                  {call.context.news.slice(0, 3).map((n, i) => (
                    <p key={n.id ?? i} className="truncate text-[11px] text-[var(--text-muted)]">
                      · {n.headline}
                    </p>
                  ))}
                </Panel>
              )}

              <HorizonProgress
                progress={call.progress}
                publishedAt={call.publishedAt}
                dueAt={call.dueAt}
                status={call.status}
              />
            </div>

            <div className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1 border-t px-4 py-2.5 text-[11px] text-[var(--text-muted)]">
              <span className="inline-flex items-center gap-1">
                <Clock3 className="size-3" />
                {readingMinutes(call.bodyMd)}분
              </span>
              <span className="inline-flex items-center gap-1">
                <Eye className="size-3" />
                {call.viewCount.toLocaleString()}
              </span>
              <span className="inline-flex items-center gap-1">
                <Heart className={cn("size-3", call.liked && "fill-rose-400 text-rose-400")} />
                {call.likeCount}
              </span>
              <span className="inline-flex items-center gap-1">
                <MessageSquare className="size-3" />
                {call.commentCount}
              </span>
              {dueDays !== null && <span>D-{dueDays}</span>}
              <span className="font-mono">
                알파{" "}
                <b className={cn("text-xs font-semibold", toneClass(alpha))}>
                  {alpha === undefined ? "—" : `${formatPercent(alpha)}p`}
                </b>
                {ret !== undefined && (
                  <>
                    {" · 수익률 "}
                    <b className={cn("text-xs", toneClass(ret))}>{formatPercent(ret)}</b>
                  </>
                )}
              </span>
              <Link
                href={`/reports/${call.id}`}
                className="ml-auto inline-flex items-center gap-1 rounded-lg bg-[var(--brand)] px-3 py-1.5 text-xs font-medium text-white hover:opacity-90"
              >
                전문 읽기
                <ArrowRight className="size-3" />
              </Link>
            </div>
          </>
        ) : (
          <div className="p-6 text-center text-sm text-[var(--text-muted)]">불러오는 중…</div>
        )}
      </DialogContent>
    </Dialog>
  );
}

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-[var(--border)] px-3 py-2">
      <p className="mb-1.5 text-[10px] font-semibold text-[var(--text-muted)]">{title}</p>
      {children}
    </div>
  );
}

function Row({
  label,
  value,
  tone,
}: {
  label: string;
  value: React.ReactNode;
  tone?: number;
}) {
  return (
    <div className="flex items-baseline justify-between gap-2 py-0.5 text-[11px]">
      <span className="shrink-0 text-[var(--text-muted)]">{label}</span>
      <span
        className={cn(
          "min-w-0 truncate text-right font-mono tabular-nums",
          tone !== undefined && toneClass(tone),
        )}
      >
        {value}
      </span>
    </div>
  );
}
