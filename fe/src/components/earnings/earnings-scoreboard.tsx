"use client";

import { Card } from "@/components/ui/card";
import type { EarningsStats, EarningsStatsPick } from "@/lib/earnings-types";
import { cn } from "@/lib/utils";

/**
 * 상단 스코어보드 (W3) — 이 구간 시장 전체의 온도.
 *
 * 목록을 읽기 **전에** 전체 그림을 준다. 「상회 61%」는 개별 백 줄을 세어야 나오는
 * 값이고, 지금까지 이 화면에는 그 값이 없었다.
 *
 * **평균과 중위를 함께 적는다.** 실측에서 하루 평균이 −0.19%인데 중위는 +14.5%인
 * 날이 있었다 — 이마트 −193% 한 건이 평균을 끌어내린 것이다. 평균만 보여주면
 * 그날 시장을 반대로 읽는다.
 */
export function EarningsScoreboard({
  stats,
  rangeLabel,
}: {
  stats: EarningsStats | null;
  rangeLabel: string;
}) {
  if (!stats) {
    return (
      <Card className="p-3">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-[46px] animate-pulse rounded-md bg-muted" />
          ))}
        </div>
      </Card>
    );
  }

  return (
    <Card className="p-3">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <Tile label={`${rangeLabel} 발표`}>
          <span className="text-lg font-bold tabular-nums">{stats.total}</span>
          <span className="ml-0.5 text-xs font-normal text-muted-foreground">건</span>
          <Sub>
            {/* 셋을 나눠 적는다 — 「발표 20 · 예정 23」과 「43건」은 다른 정보다. */}
            {[
              stats.confirmed > 0 ? `발표 ${stats.confirmed}` : null,
              stats.scheduled > 0 ? `예정 ${stats.scheduled}` : null,
              stats.estimated > 0 ? `추정 ${stats.estimated}` : null,
            ]
              .filter(Boolean)
              .join(" · ") || "—"}
          </Sub>
        </Tile>

        <Tile label="컨센서스 상회">
          {stats.beatRate == null ? (
            <Empty />
          ) : (
            <>
              <span
                className={cn(
                  "text-lg font-bold tabular-nums",
                  stats.beatRate >= 50 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                )}
              >
                {stats.beatRate.toFixed(0)}%
              </span>
              <Sub>
                {stats.beats} / {stats.compared}건
              </Sub>
            </>
          )}
        </Tile>

        <Tile label="서프라이즈 중위">
          {stats.medianPct == null ? (
            <Empty />
          ) : (
            <>
              <Signed value={stats.medianPct} className="text-lg font-bold" />
              {/* 평균을 부제로 내린 이유: 극단값에 끌리는 쪽이라 주인공이 아니다. */}
              <Sub>평균 {signed(stats.avgPct)}</Sub>
            </>
          )}
        </Tile>

        <Tile label="최대 상회">
          <PickValue pick={stats.best} />
        </Tile>
        <Tile label="최대 하회">
          <PickValue pick={stats.worst} />
        </Tile>
      </div>

      {stats.compared === 0 && stats.total > 0 && (
        <p className="mt-2 border-t pt-2 text-[10px] leading-relaxed text-muted-foreground">
          이 구간은 <b>아직 발표 전</b>이거나 컨센서스가 집계되지 않아 비교 가능한 건이 없습니다.
        </p>
      )}
    </Card>
  );
}

function Tile({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="min-w-0">
      <p className="truncate text-[10px] text-muted-foreground">{label}</p>
      <div className="mt-0.5 min-w-0">{children}</div>
    </div>
  );
}

function Sub({ children }: { children: React.ReactNode }) {
  return <p className="truncate text-[10px] text-muted-foreground">{children}</p>;
}

function Empty() {
  return <span className="text-sm text-muted-foreground">—</span>;
}

function PickValue({ pick }: { pick?: EarningsStatsPick }) {
  if (!pick) return <Empty />;
  return (
    <>
      <p className="truncate text-[13px] font-semibold">{pick.name}</p>
      <Signed value={pick.pct} className="text-[11px]" />
    </>
  );
}

function Signed({ value, className }: { value: number; className?: string }) {
  return (
    <span
      className={cn(
        "font-mono tabular-nums",
        value >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
        className,
      )}
    >
      {value >= 0 ? "+" : ""}
      {value.toFixed(1)}%
    </span>
  );
}

function signed(value?: number): string {
  if (value == null) return "—";
  return `${value >= 0 ? "+" : ""}${value.toFixed(1)}%`;
}
