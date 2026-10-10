"use client";

import { useMemo } from "react";

import { StockAvatar } from "@/components/stocks/stock-avatar";
import { SectionCard, SectionEmpty } from "@/components/ui/section-card";
import { useScoreChanges } from "@/hooks/use-score-changes";
import { compactDriverLabel, formatDelta, shortDriverLabel } from "@/lib/score-explain-api";
import { SCORE_TONE_BADGE, scoreTone } from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";

/**
 * 「점수가 달라진 종목」 — 보유·관심 레일 (2026-09-17 사용자 결정 C).
 *
 * # 왜 목록 행이 아니라 카드인가
 *
 * 처음 제안은 보유·관심 **목록 행**에 사유를 붙이는 것이었다. 관심 목록의 점수 칸은
 * 폭 36px(`WL_COL.score`)이고 보유 목록에는 점수 칸이 아예 없다 — 375px에서 그 줄에
 * 「가치 축 결측」을 밀어 넣으면 옆 칸의 수익률부터 잘린다. 사유는 **읽을 수 있어야**
 * 값이 있으므로, 레일의 한 칸으로 옮겼다. 레일은 좁은 화면에서 본문 아래로 내려온다.
 *
 * # 왜 요약 입구만 쓰는가
 *
 * 종목마다 설명 전체(`/score/explain`, 15KB)를 부르면 20종목에 300KB · 20조회다.
 * 이 카드는 `/stocks/scores/changes` 한 번으로 끝난다(10종목 2.6KB 실측).
 *
 * # 점수는 하루 한 번이다
 *
 * 그래서 이 카드는 하루 종일 같은 말을 한다. 시세가 흐르는 화면 옆에서 「멈춘 값」으로
 * 오해되지 않도록 **기준일을 제목 줄에** 적는다.
 */
export function ScoreMoversCard({
  symbols,
  names,
  onSelect,
  title = "점수가 달라진 종목",
  limit = 5,
  className,
}: {
  symbols: string[];
  /** 심볼 → 표시 이름. 없으면 심볼을 그대로 쓴다. */
  names?: Map<string, string>;
  onSelect?: (symbol: string) => void;
  title?: string;
  limit?: number;
  className?: string;
}) {
  const changes = useScoreChanges(symbols);

  const { movers, tradeDate, comparable } = useMemo(() => {
    const rows = symbols
      .map((s) => changes.get(s.trim().toUpperCase()))
      .filter((r): r is NonNullable<typeof r> => Boolean(r));
    const withDelta = rows.filter((r) => r.available && typeof r.delta === "number");
    return {
      // 크게 움직인 순. 0점 변화는 아래로 밀리고 `limit`에서 잘린다.
      movers: [...withDelta]
        .sort((a, b) => Math.abs(b.delta ?? 0) - Math.abs(a.delta ?? 0))
        .filter((r) => (r.delta ?? 0) !== 0)
        .slice(0, limit),
      tradeDate: rows[0]?.tradeDate ?? null,
      comparable: withDelta.length,
    };
  }, [changes, symbols, limit]);

  if (symbols.length === 0) return null;

  return (
    <SectionCard
      title={title}
      action={
        tradeDate ? (
          <span className="shrink-0 text-micro tabular-nums text-muted-foreground">
            {tradeDate} 기준
          </span>
        ) : undefined
      }
      className={className}
    >
      {movers.length === 0 ? (
        /*
          「비교할 것이 없다」와 「비교했는데 그대로다」는 다른 답이다 (요구 8).
          0으로 뭉뚱그리면 배치가 안 돈 날과 조용한 날이 같아 보인다.
        */
        <SectionEmpty
          text={
            comparable === 0
              ? "비교할 이전 회차가 없습니다."
              : "전일 대비 점수가 달라진 종목이 없습니다."
          }
        />
      ) : (
        <ul className="min-w-0">
          {movers.map((row) => {
            const delta = row.delta ?? 0;
            const cause = compactDriverLabel(row.topDriver);
            // 잘린 짧은 말 뒤에 온전한 문장을 남긴다.
            const causeFull = shortDriverLabel(row.topDriver) ?? row.topDriver?.detail;
            const name = names?.get(row.symbol.toUpperCase()) ?? row.symbol;
            const inner = (
              <>
                <StockAvatar symbol={row.symbol} name={name} size="xs" className="shrink-0" />
                <span className="min-w-0 flex-1 truncate">{name}</span>
                {row.score != null && (
                  <span
                    className={cn(
                      "shrink-0 rounded px-1 text-micro font-semibold",
                      SCORE_TONE_BADGE[scoreTone(row.score)],
                    )}
                  >
                    {row.score}
                  </span>
                )}
                <span
                  className={cn(
                    "w-7 shrink-0 text-right font-num tabular-nums",
                    delta > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                  )}
                >
                  {formatDelta(delta)}
                </span>
                {/*
                  사유는 좁아지면 먼저 잘리는 쪽이다 — 왼쪽의 종목명과 변화량이 이 줄의
                  요점이고, 전문은 종목을 눌러 여는 화면이 말한다.
                */}
                <span
                  className="w-[3.75rem] shrink-0 truncate text-right text-micro text-muted-foreground"
                  title={causeFull ?? undefined}
                >
                  {cause ?? ""}
                </span>
              </>
            );
            return (
              <li key={row.symbol}>
                {onSelect ? (
                  <button
                    type="button"
                    onClick={() => onSelect(row.symbol)}
                    className="flex h-[30px] w-full min-w-0 items-center gap-2 rounded-md px-1 text-left text-cell hover:bg-accent"
                  >
                    {inner}
                  </button>
                ) : (
                  <div className="flex h-[30px] w-full min-w-0 items-center gap-2 px-1 text-cell">
                    {inner}
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </SectionCard>
  );
}
