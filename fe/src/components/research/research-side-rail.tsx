"use client";

import { useMemo } from "react";
import Link from "next/link";
import { Eye, PenLine } from "lucide-react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { SectionCard, SectionEmpty } from "@/components/ui/section-card";
import { daysUntilDue, toneClass } from "@/components/research/research-shared";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { formatPercent } from "@/lib/format";
import type { ResearchCall, ResearchSummary } from "@/lib/research-types";
import { cn } from "@/lib/utils";

/**
 * 리포트 목록 화면의 우측 레일.
 *
 * 예전엔 "내 성적표" 4타일이 목록 위를 가로로 가로질렀다. 전체 폭이 되면서 타일 하나가
 * 380px까지 벌어졌는데 안에 든 건 숫자 하나뿐이라 빈 칸만 커졌고, 정작 읽어야 할
 * 리포트 목록은 그만큼 아래로 밀렸다. 성적표는 목록을 읽는 내내 옆에 있으면 되는
 * 종류의 정보라 레일이 제자리다.
 *
 * 집계 위젯은 **화면에 이미 있는 목록에서 센다** — 순위를 따로 요청하면 목록과 집계가
 * 서로 다른 시점을 말하게 된다. (작성자 순위표는 두지 않는다 —
 * docs/RESEARCH_CALL_DESIGN.md §15-1.)
 */

type ResearchSideRailProps = {
  summary: ResearchSummary | null;
  allCalls: ResearchCall[];
  /** 목록의 퀵룩 모달을 연다. */
  onOpenCall?: (id: number) => void;
  /** 종목으로 목록을 거른다. */
  onPickSymbol?: (symbol: string) => void;
  symbolFilter?: string | null;
};

export function ResearchSideRail({
  summary,
  allCalls,
  onOpenCall,
  onPickSymbol,
  symbolFilter,
}: ResearchSideRailProps) {
  return (
    <>
      <ScorecardCard summary={summary} />
      <DueSoonCard calls={allCalls} onOpenCall={onOpenCall} />
      <NearLevelCard calls={allCalls} onOpenCall={onOpenCall} />
      <DirectionMixCard calls={allCalls} />
      <TopSymbolsCard calls={allCalls} onPickSymbol={onPickSymbol} symbolFilter={symbolFilter} />
      <MostReadCard calls={allCalls} />
      <ComposeGuideCard />
    </>
  );
}

/* ── 곧 판정됩니다 ─────────────────────────────────────────────────────────── */

/**
 * 만기가 가까운 순.
 *
 * 만기 종가로 알파가 확정되므로, 이 목록이 「이제 곧 결과가 나온다」를 말하는
 * 유일한 자리다. 목록 정렬(최신·조회·추천)로는 절대 위로 오지 않는다.
 */
function DueSoonCard({
  calls,
  onOpenCall,
}: {
  calls: ResearchCall[];
  onOpenCall?: (id: number) => void;
}) {
  const rows = useMemo(
    () =>
      calls
        .map((c) => ({ call: c, days: daysUntilDue(c.dueAt, c.status) }))
        .filter((r): r is { call: ResearchCall; days: number } => r.days !== null)
        .sort((a, b) => a.days - b.days)
        .slice(0, 5),
    [calls],
  );

  return (
    <SectionCard
      title="곧 판정됩니다"
      action={
        rows.length > 0 ? (
          <span className="rounded-md bg-amber-500/15 px-1.5 py-0.5 text-[10px] font-semibold text-amber-500">
            {rows.length}
          </span>
        ) : undefined
      }
    >
      {rows.length === 0 ? (
        <SectionEmpty text="검증 중인 리포트가 없습니다" />
      ) : (
        <>
          <ul className="min-w-0 space-y-1">
            {rows.map(({ call, days }) => {
              const ret =
                call.horizon?.return ?? call.current?.changeRate;
              return (
                <li key={call.id}>
                  <button
                    type="button"
                    onClick={() => onOpenCall?.(call.id)}
                    className="grid w-full min-w-0 grid-cols-[38px_minmax(0,1fr)_46px] items-center gap-2 rounded-md px-1 py-1 text-left hover:bg-accent"
                  >
                    <span
                      className={cn(
                        "shrink-0 font-mono text-[11px] font-semibold tabular-nums",
                        days <= 7 ? "text-amber-500" : "text-muted-foreground",
                      )}
                    >
                      D-{days}
                    </span>
                    <span className="h-1.5 min-w-0 rounded-full bg-accent">
                      <span
                        className="block h-full rounded-full bg-[var(--brand)]"
                        style={{
                          width: `${Math.max(2, Math.min(100, call.progress * 100))}%`,
                        }}
                      />
                    </span>
                    <span
                      className={cn(
                        "shrink-0 text-right font-mono text-[11px] tabular-nums",
                        toneClass(ret),
                      )}
                    >
                      {ret === undefined ? "—" : formatPercent(ret)}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
          <p className="mt-1.5 text-[10px] leading-tight text-muted-foreground">
            막대 = 기간 진행. 만기 종가로 알파가 확정됩니다
          </p>
        </>
      )}
    </SectionCard>
  );
}

/* ── 목표·손절에 근접한 리포트 ──────────────────────────────────────────────── */

const NEAR_LEVEL_PCT = 5;

/**
 * 목표가 또는 손절가 5% 이내로 들어온 검증 중 리포트.
 *
 * `toTargetPct`·`toStopPct`가 이미 응답에 있는데 목록 카드에서 한 번 지나가면
 * 끝이었다. 곧 결판날 콜을 위로 올리는 게 이 화면을 매일 볼 이유다.
 */
function NearLevelCard({
  calls,
  onOpenCall,
}: {
  calls: ResearchCall[];
  onOpenCall?: (id: number) => void;
}) {
  const rows = useMemo(
    () =>
      calls
        .filter((c) => c.status === "open")
        .map((c) => {
          const toTarget = c.toTargetPct;
          const toStop = c.toStopPct;
          const nearTarget = toTarget !== undefined && Math.abs(toTarget) <= NEAR_LEVEL_PCT;
          const nearStop = toStop !== undefined && Math.abs(toStop) <= NEAR_LEVEL_PCT;
          if (!nearTarget && !nearStop) return null;
          // 손절이 더 가까우면 그쪽을 말한다 — 목표까지 남은 거리만 적으면
          // 손절선 코앞이라는 사실을 감춘다.
          const stopCloser =
            nearStop && (!nearTarget || Math.abs(toStop as number) < Math.abs(toTarget as number));
          return {
            call: c,
            kind: stopCloser ? ("stop" as const) : ("target" as const),
            pct: (stopCloser ? toStop : toTarget) as number,
          };
        })
        .filter((r): r is { call: ResearchCall; kind: "stop" | "target"; pct: number } =>
          Boolean(r),
        )
        .sort((a, b) => Math.abs(a.pct) - Math.abs(b.pct))
        .slice(0, 5),
    [calls],
  );

  if (rows.length === 0) return null;

  return (
    <SectionCard title="목표·손절에 근접">
      <ul className="min-w-0">
        {rows.map(({ call, kind, pct }) => (
          <li key={call.id}>
            <button
              type="button"
              onClick={() => onOpenCall?.(call.id)}
              className="flex w-full min-w-0 items-center gap-2 rounded-md px-1 py-1.5 text-left hover:bg-accent"
            >
              <span className="line-clamp-2 min-w-0 flex-1 text-xs leading-snug">{call.title}</span>
              <span
                className={cn(
                  "shrink-0 rounded-md px-1.5 py-0.5 text-[10px] font-semibold",
                  kind === "stop"
                    ? "bg-[var(--loss)]/15 text-[var(--loss)]"
                    : "bg-amber-500/15 text-amber-500",
                )}
              >
                {kind === "stop" ? "손절" : "목표"} {pct > 0 ? "+" : ""}
                {pct.toFixed(1)}%
              </span>
            </button>
          </li>
        ))}
      </ul>
    </SectionCard>
  );
}

/* ── 의견 분포 ─────────────────────────────────────────────────────────────── */

/** 지금 목록에 실린 리포트의 매수/매도/관망 비율. 시장 심리가 아니라 이 화면의 심리다. */
function DirectionMixCard({ calls }: { calls: ResearchCall[] }) {
  const mix = useMemo(() => {
    const total = calls.length;
    if (total === 0) return null;
    const count = (d: ResearchCall["direction"]) => calls.filter((c) => c.direction === d).length;
    return {
      total,
      rows: [
        { label: "매수", n: count("buy"), tone: "bg-[var(--profit)]" },
        { label: "매도", n: count("sell"), tone: "bg-[var(--loss)]" },
        { label: "관망", n: count("hold"), tone: "bg-muted-foreground" },
      ],
    };
  }, [calls]);

  if (!mix) return null;

  return (
    <SectionCard title="의견 분포" action={<span className="text-[10px] text-muted-foreground">{mix.total}건 기준</span>}>
      <div className="space-y-1">
        {mix.rows.map((r) => (
          <div key={r.label} className="flex items-center gap-2 text-[11px]">
            <span className="w-7 shrink-0 text-muted-foreground">{r.label}</span>
            <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-accent">
              <span
                className={cn("block h-full rounded-full", r.tone)}
                style={{ width: `${(r.n / mix.total) * 100}%` }}
              />
            </span>
            <span className="w-8 shrink-0 text-right font-mono tabular-nums">
              {Math.round((r.n / mix.total) * 100)}%
            </span>
          </div>
        ))}
      </div>
    </SectionCard>
  );
}

/* ── 내 성적표 ────────────────────────────────────────────────────────────── */

function ScorecardCard({ summary }: { summary: ResearchSummary | null }) {
  const enoughSample = (summary?.resolved ?? 0) >= (summary?.minSample ?? 10);

  return (
    <SectionCard title="내 성적표">
      <dl className="grid grid-cols-2 gap-2">
        <Metric
          label="발행"
          value={`${summary?.total ?? 0}건`}
          hint={`검증 중 ${summary?.open ?? 0} · ${summary?.symbols ?? 0}종목`}
        />
        <Metric
          label="적중률"
          value={summary?.hitRate === undefined ? "—" : `${Math.round(summary.hitRate)}%`}
          hint={
            summary
              ? enoughSample
                ? `${summary.hits}/${summary.resolved} 판정`
                : `표본 ${summary.resolved}건 — ${summary.minSample}건 미만`
              : "판정된 리포트 기준"
          }
          muted={!enoughSample}
        />
        <Metric
          label="평균 알파"
          value={summary?.avgAlpha === undefined ? "—" : formatPercent(summary.avgAlpha)}
          hint="벤치마크 대비 초과수익"
          valueClass={toneClass(summary?.avgAlpha)}
        />
        <Metric
          label="철회율"
          value={summary?.retractRate === undefined ? "—" : `${Math.round(summary.retractRate)}%`}
          hint={`철회 ${summary?.retracted ?? 0}건 — 통계에 남습니다`}
        />
      </dl>
    </SectionCard>
  );
}

function Metric({
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
    <div className="min-w-0">
      <dt className="text-[11px] text-muted-foreground">{label}</dt>
      <dd
        className={cn(
          "mt-0.5 font-mono text-base font-semibold tabular-nums",
          muted && "text-muted-foreground",
          valueClass,
        )}
      >
        {value}
      </dd>
      <p className="text-[10px] leading-tight text-muted-foreground">{hint}</p>
    </div>
  );
}

/* ── 많이 다룬 종목 ────────────────────────────────────────────────────────── */

function TopSymbolsCard({
  calls,
  onPickSymbol,
  symbolFilter,
}: {
  calls: ResearchCall[];
  onPickSymbol?: (symbol: string) => void;
  symbolFilter?: string | null;
}) {
  const rows = useMemo(() => {
    const bySymbol = new Map<
      string,
      { symbol: string; name: string; total: number; buy: number; sell: number }
    >();
    for (const call of calls) {
      const cur = bySymbol.get(call.symbol) ?? {
        symbol: call.symbol,
        name: call.symbolName || call.symbol,
        total: 0,
        buy: 0,
        sell: 0,
      };
      cur.total += 1;
      if (call.direction === "buy") cur.buy += 1;
      if (call.direction === "sell") cur.sell += 1;
      bySymbol.set(call.symbol, cur);
    }
    return [...bySymbol.values()].sort((a, b) => b.total - a.total).slice(0, 6);
  }, [calls]);

  return (
    <SectionCard title="많이 다룬 종목">
      {rows.length === 0 ? (
        <SectionEmpty text="집계할 리포트가 없습니다" />
      ) : (
        /* 종목 이름을 누르면 **목록을 거른다**. 종목 분석으로 떠나는 링크는 따로
           둔다 — 리포트를 훑던 중에 화면을 떠날 이유는 없다. */
        <ul className="min-w-0">
          {rows.map((row) => (
            <li key={row.symbol}>
              <div
                className={cn(
                  "flex h-[30px] min-w-0 items-center gap-2 rounded-md px-1 text-xs",
                  symbolFilter === row.symbol ? "bg-accent font-medium" : "hover:bg-accent/60",
                )}
              >
                <button
                  type="button"
                  onClick={() => onPickSymbol?.(row.symbol)}
                  aria-pressed={symbolFilter === row.symbol}
                  className="flex min-w-0 flex-1 items-center gap-2 text-left"
                >
                  <StockAvatar symbol={row.symbol} name={row.name} size="xs" className="shrink-0" />
                  <span className="min-w-0 flex-1 truncate">{row.name}</span>
                </button>
                <span className="shrink-0 font-mono text-[11px] tabular-nums text-[var(--profit)]">
                  {row.buy}
                </span>
                <span className="shrink-0 font-mono text-[11px] tabular-nums text-[var(--loss)]">
                  {row.sell}
                </span>
                <Link
                  href={analysisSymbolHref(row.symbol, "ai")}
                  className="w-8 shrink-0 text-right font-mono text-xs tabular-nums text-muted-foreground hover:text-foreground"
                  title="종목 분석 보기"
                >
                  {row.total}건
                </Link>
              </div>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}

/* ── 많이 본 리포트 ────────────────────────────────────────────────────────── */

function MostReadCard({ calls }: { calls: ResearchCall[] }) {
  const rows = useMemo(
    () =>
      [...calls]
        .filter((c) => c.viewCount > 0)
        .sort((a, b) => b.viewCount - a.viewCount)
        .slice(0, 5),
    [calls],
  );

  return (
    <SectionCard title="많이 본 리포트">
      {rows.length === 0 ? (
        <SectionEmpty text="아직 조회된 리포트가 없습니다" />
      ) : (
        <ol className="min-w-0">
          {rows.map((call, i) => (
            <li key={call.id}>
              <Link
                href={`/reports/${call.id}`}
                className="flex min-w-0 gap-2 rounded-md px-1 py-1.5 hover:bg-accent"
              >
                <span className="w-3 shrink-0 pt-px text-[11px] font-medium text-muted-foreground">
                  {i + 1}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="line-clamp-2 block text-xs leading-snug">{call.title}</span>
                  <span className="mt-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                    <Eye className="size-3" />
                    {call.viewCount.toLocaleString("ko-KR")}
                    <span className="truncate">· {call.symbolName || call.symbol}</span>
                  </span>
                </span>
              </Link>
            </li>
          ))}
        </ol>
      )}
    </SectionCard>
  );
}

/* ── 작성 안내 ────────────────────────────────────────────────────────────── */

function ComposeGuideCard() {
  return (
    <SectionCard title="리포트는 이렇게 채점됩니다">
      <ol className="space-y-1.5 text-[11px] leading-relaxed text-muted-foreground">
        <li>
          <span className="font-medium text-foreground">1. 발행 시점이 박제됩니다.</span> 점수·시세·뉴스·공시를
          서버가 그 자리에서 기록합니다.
        </li>
        <li>
          <span className="font-medium text-foreground">2. 값은 고칠 수 없습니다.</span> 방향·목표가·기간은
          발행 후 수정되지 않고, 본문 수정은 이력이 남습니다.
        </li>
        <li>
          <span className="font-medium text-foreground">3. 알파로 채점합니다.</span> 원수익률이 아니라
          벤치마크 대비 초과수익이 성적입니다.
        </li>
      </ol>
      <Link
        href="/reports/new"
        className="mt-2.5 flex items-center justify-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-[11px] font-medium hover:bg-accent"
      >
        <PenLine className="size-3" />
        리포트 쓰기
      </Link>
    </SectionCard>
  );
}
