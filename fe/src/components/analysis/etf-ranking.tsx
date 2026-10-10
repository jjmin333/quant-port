"use client";

import { useEffect, useState } from "react";
import { RadarMarketTabs } from "@/components/market-radar/radar-toolbar-parts";
import { ETF_TAB } from "@/components/analysis/score-ranking-toolbar";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { SCORE_RANKING_MARKET_OPTIONS } from "@/lib/score-ranking-config";
import {
  ETF_PEER_LABEL,
  fetchEtfTop,
  formatKrwShort,
  type EtfPeerGroup,
  type EtfTop,
} from "@/lib/etf-score-api";
import { cn } from "@/lib/utils";

const GROUPS: { id: EtfPeerGroup; label: string }[] = [
  { id: "plain", label: ETF_PEER_LABEL.plain },
  { id: "lowvol", label: ETF_PEER_LABEL.lowvol },
  { id: "leveraged", label: ETF_PEER_LABEL.leveraged },
];

/**
 * AI 랭킹의 「국내 ETF」 탭 (2026-09-24 사용자 결정).
 *
 * 주식 랭킹의 축·기간 손잡이가 없다 — ETF 점검은 괴리·추적·유동성·규모 넷의 백분위
 * 평균 하나뿐이다. 비교 집단(일반 / 채권·금리 / 레버리지·인버스)을 섞지 않도록 하나를 고른다.
 * 같은 지수를 따르는 상품끼리 고르는 데 쓰라는 목록이다 — 「무엇을 살까」가 아니라
 * 「같은 것을 산다면 어느 그릇이 나은가」.
 */
export function EtfRanking({
  activeSymbol,
  onSelect,
  onMarket,
  className,
}: {
  activeSymbol?: string | null;
  onSelect: (symbol: string) => void;
  onMarket: (market: "domestic" | "overseas") => void;
  className?: string;
}) {
  const [group, setGroup] = useState<EtfPeerGroup>("plain");
  const [result, setResult] = useState<{ group: EtfPeerGroup; data?: EtfTop; error?: string } | null>(
    null,
  );

  useEffect(() => {
    let alive = true;
    fetchEtfTop(group, 100)
      .then((d) => alive && setResult({ group, data: d }))
      .catch(
        (e: unknown) =>
          alive &&
          setResult({ group, error: e instanceof Error ? e.message : "ETF 점검을 불러오지 못했습니다" }),
      );
    return () => {
      alive = false;
    };
  }, [group]);

  const current = result?.group === group ? result : null;
  const items = current?.data?.items ?? [];

  return (
    <div className={cn("flex min-h-0 min-w-0 flex-col overflow-hidden", className)}>
      <RadarMarketTabs<string>
        options={[...SCORE_RANKING_MARKET_OPTIONS, ETF_TAB]}
        value={ETF_TAB.id}
        onChange={(m) => {
          if (m !== ETF_TAB.id) onMarket(m as "domestic" | "overseas");
        }}
      />
      <div className="flex shrink-0 items-center gap-2 border-b border-[var(--border)] px-3 py-1.5">
        <MiniToggle options={GROUPS} value={group} onChange={setGroup} />
        <span className="ml-auto truncate text-micro text-[var(--text-muted)]">
          {current?.data?.tradeDate ? `${current.data.tradeDate} 기준` : ""}
        </span>
      </div>
      <p className="shrink-0 border-b border-[var(--border)] px-3 py-1 text-micro text-[var(--text-muted)]">
        괴리율·추적오차·유동성·규모의 집단 내 백분위 평균 · 총보수 제외 · 국내 출처: KRX 통계정보
      </p>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {current?.error ? (
          <p className="p-4 text-center text-cell text-[var(--text-muted)]">{current.error}</p>
        ) : !current ? (
          <p className="p-4 text-center text-cell text-[var(--text-muted)]">불러오는 중…</p>
        ) : items.length === 0 ? (
          <p className="p-4 text-center text-cell text-[var(--text-muted)]">
            아직 점검한 ETF가 없습니다 — 매일 오전 10시에 전 영업일 자료로 갱신됩니다.
          </p>
        ) : (
          <ul>
            {items.map((it) => (
              <li key={it.symbol}>
                <button
                  type="button"
                  onClick={() => onSelect(it.symbol)}
                  className={cn(
                    "flex w-full min-w-0 items-center gap-2 border-b border-[var(--border)]/60 px-3 py-1.5 text-left text-cell hover:bg-muted/40",
                    activeSymbol === it.symbol && "bg-sidebar-accent",
                  )}
                >
                  <span className="w-6 shrink-0 text-right font-num text-[var(--text-muted)] tabular-nums">
                    {it.rank}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate">{it.name || it.symbol}</span>
                    <span className="block truncate text-micro text-[var(--text-muted)]">
                      괴리 {it.premiumPct != null ? `${it.premiumPct.toFixed(2)}%` : "-"} · 추적{" "}
                      {it.trackingErrorPct != null ? `${it.trackingErrorPct.toFixed(2)}%` : "-"} · 거래{" "}
                      {formatKrwShort(it.tradeValueMedian)}
                    </span>
                  </span>
                  <b className="w-8 shrink-0 text-right font-num tabular-nums">{Math.round(it.score)}</b>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
