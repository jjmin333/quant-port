"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { Card } from "@/components/ui/card";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  ContrarianBadge,
  DirectionBadge,
  StatusBadge,
  toneClass,
} from "@/components/research/research-shared";
import { fetchResearchAuthorClient } from "@/lib/research-api";
import type { ResearchAuthorProfile } from "@/lib/research-types";
import { formatDateTimeKST, formatPercent } from "@/lib/format";
import { cn } from "@/lib/utils";
import { PriceDisplay } from "@/components/ui/price-display";

export function ResearchAuthorClient({
  userId,
  initial = null,
}: {
  userId: number;
  /** 서버가 익명으로 미리 받은 프로필(공개 리포트만). 마운트 뒤 내 세션으로 다시 받아 덮는다. */
  initial?: ResearchAuthorProfile | null;
}) {
  const [profile, setProfile] = useState<ResearchAuthorProfile | null>(initial);
  const [loading, setLoading] = useState(initial === null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError(null);
    try {
      setProfile(await fetchResearchAuthorClient(userId));
    } catch {
      setError("작성자 정보를 불러오지 못했습니다.");
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    load();
  }, [load]);

  const summary = profile?.summary;
  const enoughSample = (summary?.resolved ?? 0) >= (summary?.minSample ?? 10);

  return (
    <>
      <PageScrollShell contentClassName="space-y-4">
        <Link
          href="/reports"
          className="inline-flex items-center gap-1 text-cell text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          리포트 목록
        </Link>

        {/* 작성자 이름은 화면 장식이 아니라 이 페이지가 누구의 것인지다 — 페이지
            헤더를 걷어내면서 본문의 h1으로 내려왔다. */}
        <h1 className="text-page font-semibold text-[var(--text-primary)]">
          {profile?.authorName || "작성자"}
        </h1>

        {error && <Card className="p-4 text-sm text-[var(--loss)]">{error}</Card>}

        {loading ? (
          <Card className="p-6 text-center text-sm text-[var(--text-muted)]">불러오는 중…</Card>
        ) : (
          <>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
              <Tile
                label="발행"
                value={`${summary?.total ?? 0}건`}
                hint={`검증 중 ${summary?.open ?? 0} · ${summary?.symbols ?? 0}개 종목`}
              />
              <Tile
                label="적중률"
                value={summary?.hitRate === undefined ? "—" : `${Math.round(summary.hitRate)}%`}
                hint={
                  summary
                    ? enoughSample
                      ? `${summary.hits}/${summary.resolved} 판정`
                      : `표본 ${summary.resolved}건 — ${summary.minSample}건 미만`
                    : ""
                }
                muted={!enoughSample}
              />
              <Tile
                label="평균 알파"
                value={summary?.avgAlpha === undefined ? "—" : formatPercent(summary.avgAlpha)}
                hint="벤치마크 대비"
                valueClass={toneClass(summary?.avgAlpha)}
              />
              <Tile
                label="철회율"
                value={
                  summary?.retractRate === undefined ? "—" : `${Math.round(summary.retractRate)}%`
                }
                hint={`철회 ${summary?.retracted ?? 0}건`}
              />
            </div>

            {summary && summary.contrarianResolved > 0 && (
              <Card className="p-3 text-cell text-[var(--text-muted)]">
                역발상 리포트(우리 점수와 반대 방향) {summary.contrarianResolved}건 중{" "}
                <strong className="text-[var(--text-secondary)]">{summary.contrarianHits}건 적중</strong>
                {summary.contrarianHitRate !== undefined &&
                  ` (${Math.round(summary.contrarianHitRate)}%)`}
                {summary.calibration !== undefined &&
                  ` · 캘리브레이션 ${summary.calibration.toFixed(2)}`}
              </Card>
            )}

            <div className="space-y-2">
              {profile && profile.calls.length > 0 ? (
                profile.calls.map((call) => {
                  const currency = call.currency ?? "KRW";
                  const alpha = call.verdictAlpha ?? call.horizon?.alpha;
                  return (
                    <Link
                      key={call.id}
                      href={`/reports/${call.id}`}
                      className="flex items-center gap-3 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 transition-colors hover:border-[var(--brand)]"
                    >
                      <StockAvatar symbol={call.symbol} name={call.symbolName} size="xs" />
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-1.5">
                          <span className="truncate text-sm font-medium text-[var(--text-primary)]">
                            {call.title}
                          </span>
                          <DirectionBadge direction={call.direction} />
                          <StatusBadge status={call.status} />
                          {call.contrarian && <ContrarianBadge />}
                        </div>
                        <div className="mt-0.5 flex flex-wrap items-center gap-x-3 text-cell text-[var(--text-muted)]">
                          <span>{call.symbolName || call.symbol}</span>
                          <span>{formatDateTimeKST(call.publishedAt)}</span>
                          <span><PriceDisplay value={call.basePrice} currency={currency} size="inherit" /></span>
                        </div>
                      </div>
                      <span className={cn("shrink-0 text-sm tabular-nums", toneClass(alpha))}>
                        {alpha === undefined ? "—" : `${formatPercent(alpha)}p`}
                      </span>
                    </Link>
                  );
                })
              ) : (
                <Card className="p-6 text-center text-sm text-[var(--text-muted)]">
                  공개된 리포트가 없습니다.
                </Card>
              )}
            </div>
          </>
        )}
      </PageScrollShell>
    </>
  );
}

function Tile({
  label,
  value,
  hint,
  valueClass,
  muted,
}: {
  label: string;
  value: string;
  hint: string;
  valueClass?: string;
  muted?: boolean;
}) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] px-3 py-2.5">
      <p className="text-cell text-[var(--text-muted)]">{label}</p>
      <p
        className={cn(
          "mt-0.5 text-lg font-semibold text-[var(--text-primary)]",
          muted && "text-[var(--text-muted)]",
          valueClass,
        )}
      >
        {value}
      </p>
      <p className="text-micro text-[var(--text-muted)]">{hint}</p>
    </div>
  );
}
