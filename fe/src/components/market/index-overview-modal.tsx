"use client";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { IndexTagChips } from "@/components/market/index-tag-chips";
import type { IndexScoreSummary } from "@/hooks/use-index-scores";
import { formatPercent } from "@/lib/format";
import type { MarketIndex } from "@/lib/quotes-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 「한 눈에 보기」 — 흐르는 띠를 **멈춘 채 한 판에** 펼친다.
 *
 * 홈 스트립은 열세 종을 한 줄로 흘려보낸다. 그 자체는 의도한 모양이지만(홈이 지수에
 * 내줄 수 있는 세로 공간은 한 줄이다), **모바일에서는 한 번에 한두 칸밖에 안 보인다**.
 * 나머지 열한 종을 보려면 띠가 지나가기를 기다리거나 손으로 밀어야 하고, 그러는 동안
 * 앞서 본 칸은 화면 밖으로 나가 버려 **두 지수를 나란히 견줄 수가 없다** — 지수를 보는
 * 일이 대체로 그 견주기인데도.
 *
 * 그래서 띠를 고치는 대신 **띠에서 나가는 문**을 하나 낸다. 여기서는 스크롤이 세로라
 * 열세 줄이 한 화면에 들어오고, 무엇보다 **가만히 있는다**.
 *
 * 셀 내용은 스트립과 같은 값이다 — 같은 훅이 만든 같은 스냅샷을 그대로 받는다(요청 0).
 * 다른 것은 폭에서 오는 여유뿐이라 키워드를 스트립의 1개가 아닌 **3개까지** 낸다.
 * 줄을 누르면 그 위로 지수 퀵룩이 열린다.
 */
export function IndexOverviewModal({
  open,
  indices,
  scoresById,
  updatedAt,
  onClose,
  onPick,
}: {
  open: boolean;
  indices: MarketIndex[];
  scoresById: Record<string, IndexScoreSummary>;
  updatedAt: string | null;
  onClose: () => void;
  onPick: (index: MarketIndex) => void;
}) {
  const rows = indices.filter((i) => i.value > 0);

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="max-h-[85vh] gap-3 overflow-y-auto p-4 sm:max-w-2xl">
        <DialogHeader className="space-y-1">
          <DialogTitle className="text-sm">오늘의 시장 · 지수 {rows.length}종</DialogTitle>
          <DialogDescription className="text-cell">
            {updatedAt
              ? `${new Date(updatedAt).toLocaleTimeString("ko-KR", {
                  timeZone: "Asia/Seoul",
                  hour: "2-digit",
                  minute: "2-digit",
                })} 기준입니다. 줄을 누르면 차트와 관련 뉴스가 열립니다.`
              : "줄을 누르면 차트와 관련 뉴스가 열립니다."}
          </DialogDescription>
        </DialogHeader>

        {/*
          모바일은 한 줄에 하나다. 375px에서 두 칸으로 나누면 칸당 160px이라 「필라델피아
          반도체」 같은 이름과 키워드가 동시에 들어가지 못해, 결국 스트립과 같은 잘림이
          모달 안에서 반복된다. 한 줄이면 열세 줄이 세로로 이어져 그냥 훑으면 된다.
        */}
        <ul className="min-w-0 divide-y sm:grid sm:grid-cols-2 sm:gap-x-4 sm:divide-y-0">
          {rows.map((index) => (
            <li key={index.id} className="sm:border-b">
              <button
                type="button"
                onClick={() => onPick(index)}
                className="flex w-full min-w-0 items-center gap-2.5 rounded-md px-1 py-2 text-left hover:bg-accent"
              >
                <span className="min-w-0 flex-1">
                  <span className="flex min-w-0 items-center gap-1">
                    <span className="min-w-0 truncate text-cell font-medium">{index.name}</span>
                    <IndexTagChips
                      tags={scoresById[index.id]?.tags}
                      changeRate={index.changeRate}
                      max={3}
                    />
                  </span>
                </span>
                <span className="shrink-0 text-right">
                  <span className="block font-num text-note font-bold tabular-nums leading-tight">
                    {index.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })}
                  </span>
                  <span
                    className={cn(
                      "block font-num text-cell tabular-nums leading-tight",
                      index.changeRate === 0
                        ? "text-muted-foreground"
                        : trendTextClass(index.changeRate),
                    )}
                  >
                    {formatPercent(index.changeRate)}
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      </DialogContent>
    </Dialog>
  );
}
