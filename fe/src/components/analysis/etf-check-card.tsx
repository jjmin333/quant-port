"use client";

import { useEffect, useState } from "react";
import { Card } from "@/components/ui/card";
import {
  ETF_PART_META,
  ETF_PEER_LABEL,
  fetchEtfCheck,
  formatEtfPartValue,
  type EtfCheck,
} from "@/lib/etf-score-api";
import { cn } from "@/lib/utils";

/**
 * 「ETF 점검」 카드 — 국내 ETF 종목 화면의 점수 자리 (2026-09-24 사용자 결정).
 *
 * 점수가 없으면(국내 ETF가 아니거나 상장 20거래일 미만) `onMissing`으로 알려 부모가
 * 원래 점수 카드를 그대로 쓰게 한다 — 빈 카드를 세우지 않는다.
 */
export function EtfCheckCard({
  symbol,
  onMissing,
}: {
  symbol: string;
  onMissing?: () => void;
}) {
  const [result, setResult] = useState<{ symbol: string; data: EtfCheck | null } | null>(null);

  useEffect(() => {
    let alive = true;
    fetchEtfCheck(symbol)
      .then((d) => {
        if (!alive) return;
        setResult({ symbol, data: d });
        if (!d) onMissing?.();
      })
      .catch(() => {
        if (!alive) return;
        setResult({ symbol, data: null });
        onMissing?.();
      });
    return () => {
      alive = false;
    };
  }, [symbol, onMissing]);

  const data = result?.symbol === symbol ? result.data : undefined;
  if (data === null) return null;

  return (
    <Card className="p-3">
      <div className="flex items-baseline justify-between gap-2">
        <h2 className="text-section font-semibold text-[var(--text-primary)]">ETF 점검</h2>
        {data && (
          <span className="text-micro text-[var(--text-muted)]">
            {data.tradeDate} · {ETF_PEER_LABEL[data.peerGroup]} ETF {data.peerCount}개 중
          </span>
        )}
      </div>
      {!data ? (
        <p className="mt-2 text-cell text-[var(--text-muted)]">불러오는 중…</p>
      ) : (
        <>
          <div className="mt-2 flex items-baseline gap-2">
            <b className="font-num text-2xl tabular-nums">{Math.round(data.score)}</b>
            <span className="text-cell text-[var(--text-muted)]">/ 100</span>
            {data.indexName && (
              <span className="ml-auto truncate text-micro text-[var(--text-muted)]">
                기초지수 {data.indexName}
                {data.beta != null && Math.abs(data.beta - 1) > 0.2 && ` · 배율 ${data.beta.toFixed(1)}`}
              </span>
            )}
          </div>
          <ul className="mt-2 grid grid-cols-2 gap-2">
            {ETF_PART_META.map((m) => {
              const part = data.parts[m.id];
              return (
                <li key={m.id} className="rounded-md border border-[var(--border)] px-2 py-1.5" title={m.hint}>
                  <div className="flex items-baseline justify-between gap-1">
                    <span className="text-micro text-[var(--text-muted)]">{m.label}</span>
                    <span className="font-num text-cell tabular-nums">{formatEtfPartValue(m.id, part)}</span>
                  </div>
                  <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
                    <div
                      className={cn("h-full rounded-full bg-[var(--brand)]", !part && "opacity-0")}
                      style={{ width: `${part?.pct ?? 0}%` }}
                    />
                  </div>
                  <span className="text-micro text-[var(--text-muted)]">
                    {part ? `상위 ${Math.max(1, 100 - part.pct)}%` : "잴 수 없음"}
                  </span>
                </li>
              );
            })}
          </ul>
          <p className="mt-2 text-micro text-[var(--text-muted)]">
            같은 집단 ETF 안의 백분위 평균입니다. 총보수는 자료가 없어 빠졌습니다 — 운용사 공시로
            확인하세요. 주식 AI 점수와는 다른 잣대입니다. 출처: KRX 통계정보
          </p>
        </>
      )}
    </Card>
  );
}
