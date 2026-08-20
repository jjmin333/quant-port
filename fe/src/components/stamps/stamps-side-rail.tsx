"use client";

import { useMemo } from "react";
import { AlarmClock } from "lucide-react";
import { IndexPulseCard } from "@/components/market/index-pulse-card";
import { daysUntilDue } from "@/components/stamps/stamp-seal";
import {
  SectionCard,
  SectionEmpty,
  SectionKeyValue,
} from "@/components/ui/section-card";
import { formatDateKST, formatPercent } from "@/lib/format";
import { bySymbol } from "@/lib/stamp-stats";
import {
  HORIZON_LABELS,
  STANCE_LABELS,
  type Stamp,
  type StampStance,
  type StampSummary,
} from "@/lib/stamp-types";
import { cn } from "@/lib/utils";

/**
 * 스탬프 화면의 우측 레일.
 *
 * 요약 4타일이 목록 위를 가로지르고, 종목 필터 칩이 그 아래를 한 줄 더 먹었다. 종목이
 * 스무 개만 돼도 칩이 세 줄로 접혀 목록이 화면 밖으로 밀려났다. 둘 다 "목록을 읽는 동안
 * 곁에 있으면 되는" 정보라 레일로 옮기고, 칩 대신 세로 목록으로 바꿨다 — 세로는 개수가
 * 늘어도 스크롤로 흡수되지 파본문을 밀어내지 않는다.
 *
 * 집계는 전부 이미 받아둔 스탬프에서 계산한다.
 */

const STANCE_TONE: Record<StampStance, string> = {
  bullish: "text-[var(--profit)]",
  bearish: "text-[var(--loss)]",
  neutral: "text-muted-foreground",
};

function toneClass(value: number | undefined): string {
  if (value === undefined) return "text-muted-foreground";
  if (value > 0) return "text-[var(--profit)]";
  if (value < 0) return "text-[var(--loss)]";
  return "text-muted-foreground";
}

type StampsSideRailProps = {
  summary: StampSummary | null;
  allStamps: Stamp[];
  dueStamps: Stamp[];
  totalCount: number;
  symbols: { symbol: string; name: string; count: number }[];
  symbolFilter: string | null;
  onSymbolFilter: (symbol: string | null) => void;
  onOpenStamp: (id: number) => void;
};

export function StampsSideRail({
  summary,
  allStamps,
  dueStamps,
  totalCount,
  symbols,
  symbolFilter,
  onSymbolFilter,
  onOpenStamp,
}: StampsSideRailProps) {
  const stanceCounts = useMemo(() => {
    const counts: Record<StampStance, number> = { bullish: 0, neutral: 0, bearish: 0 };
    for (const stamp of allStamps) counts[stamp.stance] += 1;
    return counts;
  }, [allStamps]);

  // E-2 — 아직 검증 중인 스탬프를 개봉일 순으로. 이미 개봉 대기인 것은 위 카드가
  // 맡으므로 여기서는 앞으로 열릴 것만 센다.
  const upcoming = useMemo(
    () =>
      allStamps
        .filter((s) => !s.due)
        .slice()
        .sort((a, b) => (a.dueAt < b.dueAt ? -1 : 1))
        .slice(0, 6),
    [allStamps],
  );

  const records = useMemo(() => bySymbol(allStamps).slice(0, 6), [allStamps]);

  // E-6 — 스탬프 이후 이 종목이 지나온 폭. "적중이지만 도중에 -6%를 버텨야 했다"가
  // 종가만 보는 표에서는 통째로 사라진다.
  const swings = useMemo(
    () =>
      allStamps
        .filter((s) => s.peakReturn !== undefined || s.troughReturn !== undefined)
        .slice()
        .sort(
          (a, b) =>
            Math.abs(b.troughReturn ?? 0) - Math.abs(a.troughReturn ?? 0),
        )
        .slice(0, 5),
    [allStamps],
  );

  return (
    <>
      <SectionCard title="내 스탬프 성적">
        <SectionKeyValue label="스탬프" value={`${summary?.total ?? totalCount}건`} />
        <SectionKeyValue label="종목" value={`${summary?.symbols ?? symbols.length}개`} />
        <SectionKeyValue
          label="적중률"
          value={
            summary?.hitRate === undefined
              ? "—"
              : `${Math.round(summary.hitRate)}% (${summary.hits}/${summary.judged})`
          }
        />
        <SectionKeyValue
          label="목표가 평균 오차"
          value={
            summary?.avgTargetError === undefined
              ? "—"
              : `${summary.avgTargetError.toFixed(1)}%`
          }
        />
        <SectionKeyValue
          label="평균 알파"
          value={summary?.avgAlpha === undefined ? "—" : formatPercent(summary.avgAlpha)}
          valueClassName={
            summary?.avgAlpha === undefined ? undefined : toneClass(summary.avgAlpha)
          }
        />
        <SectionKeyValue
          label="마지막 스탬프"
          value={summary?.lastStampedAt ? formatDateKST(summary.lastStampedAt) : "—"}
        />
      </SectionCard>

      <SectionCard
        title={
          <span className="flex items-center gap-1.5">
            <AlarmClock className="size-3.5" />
            개봉 대기
            {dueStamps.length > 0 && (
              <span className="font-mono text-[11px] text-[var(--brand)]">{dueStamps.length}</span>
            )}
          </span>
        }
      >
        {dueStamps.length === 0 ? (
          <SectionEmpty text="검증 기간이 끝난 스탬프가 없습니다" />
        ) : (
          <ul className="min-w-0">
            {dueStamps.slice(0, 5).map((stamp) => (
              <li key={stamp.id}>
                <button
                  type="button"
                  onClick={() => onOpenStamp(stamp.id)}
                  className="flex w-full min-w-0 items-center gap-2 rounded-md px-1 py-1.5 text-left text-xs hover:bg-accent"
                >
                  <span className="min-w-0 flex-1">
                    <span className="block truncate">{stamp.symbolName || stamp.symbol}</span>
                    <span className="block text-[11px] text-muted-foreground">
                      {stamp.targetHit
                        ? `${formatDateKST(stamp.targetHit.date)} 목표 도달`
                        : `${formatDateKST(stamp.stampedAt)} · ${HORIZON_LABELS[stamp.horizonKind]}`}
                    </span>
                  </span>
                  <span className={cn("shrink-0 text-[11px]", STANCE_TONE[stamp.stance])}>
                    {STANCE_LABELS[stamp.stance]}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      {/* E-2 — 개봉 예정 */}
      <SectionCard
        title={
          <span className="flex items-center gap-1.5">
            개봉 예정
            {upcoming.length > 0 && (
              <span className="font-mono text-[11px] text-[var(--brand)]">{upcoming.length}</span>
            )}
          </span>
        }
      >
        {upcoming.length === 0 ? (
          <SectionEmpty text="검증 중인 스탬프가 없습니다" />
        ) : (
          <ul className="relative min-w-0 pl-3.5">
            <span className="absolute inset-y-1 left-[3px] w-px bg-border" aria-hidden />
            {upcoming.map((stamp) => {
              const days = daysUntilDue(stamp.dueAt);
              const soon = days <= 1;
              return (
                <li key={stamp.id} className="relative">
                  <span
                    className={cn(
                      "absolute -left-[14px] top-[11px] size-[7px] rounded-full",
                      soon ? "bg-[var(--brand)]" : "bg-muted-foreground",
                    )}
                    aria-hidden
                  />
                  <button
                    type="button"
                    onClick={() => onOpenStamp(stamp.id)}
                    className="flex w-full min-w-0 items-center gap-2 rounded-md px-1 py-1.5 text-left text-xs hover:bg-accent"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block truncate">
                        <b className={cn("font-medium", soon && "text-[var(--brand)]")}>
                          {days <= 0 ? "오늘" : days === 1 ? "내일" : `${days}일 뒤`}
                        </b>{" "}
                        · {stamp.symbolName || stamp.symbol}
                      </span>
                      <span className="block text-[11px] text-muted-foreground">
                        {formatDateKST(stamp.dueAt)} · {HORIZON_LABELS[stamp.horizonKind]}
                      </span>
                    </span>
                    <span className={cn("shrink-0 text-[11px]", STANCE_TONE[stamp.stance])}>
                      {STANCE_LABELS[stamp.stance]}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </SectionCard>

      <SectionCard title="내 관점 분포">
        {allStamps.length === 0 ? (
          <SectionEmpty text="아직 찍은 스탬프가 없습니다" />
        ) : (
          <ul className="space-y-1">
            {(["bullish", "neutral", "bearish"] as StampStance[]).map((stance) => {
              const count = stanceCounts[stance];
              const pct = Math.round((count / allStamps.length) * 100);
              return (
                <li key={stance} className="flex items-center gap-2">
                  <span className={cn("w-8 shrink-0 text-[11px]", STANCE_TONE[stance])}>
                    {STANCE_LABELS[stance]}
                  </span>
                  <span className="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-muted">
                    <span
                      className="block h-full rounded-full bg-foreground/70"
                      style={{ width: `${pct}%` }}
                    />
                  </span>
                  <span className="w-12 shrink-0 text-right font-mono text-[11px] tabular-nums">
                    {count}건
                  </span>
                </li>
              );
            })}
          </ul>
        )}
      </SectionCard>

      {/* E-3 — 종목별 성적 */}
      <SectionCard title="종목별 성적">
        {records.length === 0 ? (
          <SectionEmpty text="스탬프를 찍은 종목이 없습니다" />
        ) : (
          <table className="w-full text-[11px]">
            <thead>
              <tr className="text-muted-foreground">
                <th className="pb-1 text-left font-medium">종목</th>
                <th className="pb-1 text-right font-medium">건</th>
                <th className="pb-1 text-right font-medium">적중</th>
                <th className="pb-1 text-right font-medium">α</th>
              </tr>
            </thead>
            <tbody>
              {records.map((r) => (
                <tr key={r.symbol} className="border-t border-border">
                  <td className="py-1">
                    <button
                      type="button"
                      onClick={() => onSymbolFilter(symbolFilter === r.symbol ? null : r.symbol)}
                      className={cn(
                        "block max-w-[110px] truncate text-left hover:underline",
                        symbolFilter === r.symbol && "font-medium text-[var(--brand)]",
                      )}
                    >
                      {r.name}
                    </button>
                  </td>
                  <td className="py-1 text-right font-mono tabular-nums">{r.count}</td>
                  <td className="py-1 text-right font-mono tabular-nums">
                    {r.hitRate === undefined ? "—" : `${Math.round(r.hitRate)}%`}
                  </td>
                  <td className={cn("py-1 text-right font-mono tabular-nums", toneClass(r.avgAlpha))}>
                    {r.avgAlpha === undefined ? "—" : r.avgAlpha.toFixed(1)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <p className="mt-1.5 text-[10px] text-muted-foreground">
          종목명을 누르면 목록이 그 종목으로 좁혀집니다
        </p>
      </SectionCard>

      {/* E-6 — 스탬프 후 최대 등락 */}
      <SectionCard title="스탬프 후 최대 등락">
        {swings.length === 0 ? (
          <SectionEmpty text="집계할 일봉이 아직 없습니다" />
        ) : (
          <>
            <table className="w-full text-[11px]">
              <thead>
                <tr className="text-muted-foreground">
                  <th className="pb-1 text-left font-medium">종목</th>
                  <th className="pb-1 text-right font-medium">최고</th>
                  <th className="pb-1 text-right font-medium">최저</th>
                  <th className="pb-1 text-right font-medium">현재</th>
                </tr>
              </thead>
              <tbody>
                {swings.map((s) => (
                  <tr key={s.id} className="border-t border-border">
                    <td className="py-1">
                      <button
                        type="button"
                        onClick={() => onOpenStamp(s.id)}
                        className="block max-w-[92px] truncate text-left hover:underline"
                      >
                        {s.symbolName || s.symbol}
                      </button>
                    </td>
                    <td className={cn("py-1 text-right font-mono tabular-nums", toneClass(s.peakReturn))}>
                      {s.peakReturn === undefined ? "—" : formatPercent(s.peakReturn)}
                    </td>
                    <td className={cn("py-1 text-right font-mono tabular-nums", toneClass(s.troughReturn))}>
                      {s.troughReturn === undefined ? "—" : formatPercent(s.troughReturn)}
                    </td>
                    <td
                      className={cn(
                        "py-1 text-right font-mono tabular-nums",
                        toneClass(s.current?.changeRate),
                      )}
                    >
                      {s.current?.changeRate === undefined
                        ? "—"
                        : formatPercent(s.current.changeRate)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <p className="mt-1.5 text-[10px] text-muted-foreground">
              맞은 스탬프도 도중에 얼마나 버텨야 했는지는 종가만 봐서는 안 보입니다
            </p>
          </>
        )}
      </SectionCard>

      <SectionCard title="종목별 스탬프">
        {symbols.length === 0 ? (
          <SectionEmpty text="스탬프를 찍은 종목이 없습니다" />
        ) : (
          <ul className="min-w-0">
            <li>
              <FilterRow
                label="전체 종목"
                count={totalCount}
                active={symbolFilter === null}
                onClick={() => onSymbolFilter(null)}
              />
            </li>
            {symbols.map((s) => (
              <li key={s.symbol}>
                <FilterRow
                  label={s.name || s.symbol}
                  count={s.count}
                  active={symbolFilter === s.symbol}
                  onClick={() => onSymbolFilter(symbolFilter === s.symbol ? null : s.symbol)}
                />
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <IndexPulseCard />
    </>
  );
}

function FilterRow({
  label,
  count,
  active,
  onClick,
}: {
  label: string;
  count: number;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        "flex h-[30px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-xs",
        // 활성 표시는 중립색이다 — 파랑은 이 앱에서 하락을 뜻한다 (docs/UI_SPEC.md §14).
        active ? "bg-accent font-medium" : "hover:bg-accent/60",
      )}
    >
      <span className="min-w-0 flex-1 truncate text-left">{label}</span>
      <span className="shrink-0 font-mono text-[11px] tabular-nums text-muted-foreground">
        {count}
      </span>
    </button>
  );
}
