"use client";

import { useEffect, useMemo, useState } from "react";
import { UserRoundCog } from "lucide-react";
import {
  SectionCard,
  SectionEmpty,
  SectionSkeleton,
} from "@/components/ui/section-card";
import { fetchInsiderClient } from "@/lib/insider-api";
import { formatDateKST } from "@/lib/format";
import type { InsiderTransaction } from "@/lib/insider-types";
import { cn } from "@/lib/utils";

/**
 * 내부자 거래 요약 (옵션 2 카드형, 08-11 채택).
 *
 * 지금까지는 `내부자 거래` 탭을 직접 열어야만 보였다. 임원이 사고 있는지 팔고 있는지는
 * 종목을 보는 내내 곁에 둘 만한 정보다.
 *
 * 상단 3칸은 **최근 90일 집계**다. 건수만 세면 5,000주와 5주가 같은 무게가 되므로
 * 순증(주식 수)을 함께 낸다. `sharesChange`가 없는 행은 건수에서만 세고 순증에서는
 * 빼는데, 없는 값을 0으로 더하면 "변화 없음"이라는 없는 사실이 된다.
 */
const WINDOW_DAYS = 90;

export function StockInsiderCard({ symbol }: { symbol: string }) {
  /*
   * 90일 창을 응답이 도착한 시점에 잘라 담는다. `Date.now()`를 useMemo 안에서 부르면
   * 같은 입력에 다른 결과가 나오는 순수하지 않은 계산이 된다(react-hooks/purity).
   * "지금"이 의미를 갖는 자리는 데이터가 들어오는 순간뿐이다.
   */
  const [recent, setRecent] = useState<InsiderTransaction[] | null>(null);

  useEffect(() => {
    let alive = true;
    fetchInsiderClient(symbol)
      .then((data) => {
        if (!alive) return;
        const cutoff = Date.now() - WINDOW_DAYS * 24 * 60 * 60 * 1000;
        setRecent(
          (data.transactions ?? []).filter((t) => {
            const at = new Date(t.date).getTime();
            return Number.isFinite(at) && at >= cutoff;
          }),
        );
      })
      .catch(() => {
        if (alive) setRecent([]);
      });
    return () => {
      alive = false;
    };
  }, [symbol]);

  const stats = useMemo(() => {
    if (!recent) return null;
    let buys = 0;
    let sells = 0;
    let net = 0;
    let netKnown = false;
    for (const t of recent) {
      const change = t.sharesChange;
      if (typeof change === "number" && change !== 0) {
        netKnown = true;
        net += change;
        if (change > 0) buys += 1;
        else sells += 1;
      }
    }
    return { buys, sells, net, netKnown };
  }, [recent]);

  return (
    <SectionCard
      title={
        <span className="flex items-center gap-1.5">
          <UserRoundCog className="size-3.5" />
          내부자 거래
        </span>
      }
      action={<span className="text-[11px] text-muted-foreground">최근 {WINDOW_DAYS}일</span>}
    >
      {recent === null ? (
        <SectionSkeleton rows={4} />
      ) : recent.length === 0 ? (
        <SectionEmpty text={`최근 ${WINDOW_DAYS}일 내 신고된 거래가 없습니다`} />
      ) : (
        <>
          <div className="mb-2 flex gap-2 border-b pb-2">
            <Metric label="매수" value={`${stats?.buys ?? 0}건`} tone="up" />
            <Metric label="매도" value={`${stats?.sells ?? 0}건`} tone="down" />
            <Metric
              label="순증"
              value={
                stats?.netKnown
                  ? `${stats.net > 0 ? "+" : ""}${stats.net.toLocaleString("ko-KR")}주`
                  : "—"
              }
              tone={!stats?.netKnown ? "flat" : stats.net >= 0 ? "up" : "down"}
            />
          </div>

          <ul className="min-w-0">
            {recent.slice(0, 5).map((t, i) => {
              const change = t.sharesChange;
              const isBuy = typeof change === "number" && change > 0;
              return (
                <li
                  key={`${t.date}-${t.reporterName}-${i}`}
                  className="flex min-w-0 items-center gap-2 rounded-md px-1 py-1"
                >
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-xs">{t.reporterName}</span>
                    <span className="block truncate text-[11px] text-muted-foreground">
                      {formatDateKST(t.date)}
                      {t.relation ? ` · ${t.relation}` : ""}
                    </span>
                  </span>
                  <span
                    className={cn(
                      "shrink-0 whitespace-nowrap font-mono text-[11px] tabular-nums",
                      typeof change !== "number"
                        ? "text-muted-foreground"
                        : isBuy
                          ? "text-[var(--profit)]"
                          : "text-[var(--loss)]",
                    )}
                  >
                    {typeof change === "number"
                      ? `${isBuy ? "매수" : "매도"} ${Math.abs(change).toLocaleString("ko-KR")}`
                      : (t.transactionType ?? "—")}
                  </span>
                </li>
              );
            })}
          </ul>
        </>
      )}
    </SectionCard>
  );
}

function Metric({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "up" | "down" | "flat";
}) {
  return (
    <div className="min-w-0 flex-1">
      <p className="text-[11px] text-muted-foreground">{label}</p>
      <p
        className={cn(
          "truncate font-mono text-sm font-semibold tabular-nums",
          tone === "up" && "text-[var(--profit)]",
          tone === "down" && "text-[var(--loss)]",
          tone === "flat" && "text-muted-foreground",
        )}
      >
        {value}
      </p>
    </div>
  );
}
