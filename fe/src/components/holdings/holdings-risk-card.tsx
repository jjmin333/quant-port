"use client";

import { useEffect, useMemo, useState } from "react";
import { Card } from "@/components/ui/card";
import { apiRequest } from "@/lib/api-client";
import { fetchIndexDailyCloses } from "@/lib/daily-close-api";
import type { HoldingMetrics } from "@/lib/holdings-calc";
import { computePortfolioRisk, type Close } from "@/lib/portfolio-risk";
import { HoldingsWhatIf } from "@/components/holdings/holdings-what-if";

/**
 * 「위험」 카드 — 자산 배분 옆 (2026-09-24 사용자 결정).
 *
 * 지금 비중으로 지난 반년을 들고 있었다면의 변동성 · 최대낙폭 · 지수 베타, 그리고 함께
 * 움직이는 종목 쌍. 계산은 `lib/portfolio-risk.ts`, 종가는 저장분을 한 번에 읽는다
 * (`/stocks/daily-closes/batch` — 외부 호출 없음).
 */
export function HoldingsRiskCard({ metrics }: { metrics: HoldingMetrics[] }) {
  const holdings = useMemo(
    () =>
      metrics
        .filter((m) => m.quantity > 0 && (m.marketValueKRW ?? 0) > 0)
        .slice(0, 40)
        .map((m) => ({
          symbol: m.symbol,
          name: m.name,
          weight: m.marketValueKRW ?? 0,
          currency: m.currency,
          sector: m.row.sectorLabel,
        })),
    [metrics],
  );
  /*
    불러오기는 **종목 목록**에만 묶는다. 예전에는 평가금액까지 키에 넣어, 실시간 시세가 바뀔 때마다
    종가를 다시 부르고 카드 속(「바꿔 보기」)이 새로 마운트돼 닫혔다(2026-09-24 실측). 비중은
    아래 계산(useMemo)에서만 쓴다.
  */
  const key = holdings
    .map((h) => h.symbol)
    .sort()
    .join(",");
  // 원화 비중이 절반 이상이면 코스피, 아니면 S&P 500과 비교한다.
  const krw = holdings.filter((h) => h.currency === "KRW").reduce((s, h) => s + h.weight, 0);
  const total = holdings.reduce((s, h) => s + h.weight, 0);
  const indexId = total > 0 && krw / total >= 0.5 ? "kospi" : "sp500";
  const indexLabel = indexId === "kospi" ? "코스피" : "S&P 500";

  const [data, setData] = useState<{
    key: string;
    closes?: Record<string, Close[]>;
    index?: Close[] | null;
    error?: string;
  } | null>(null);

  useEffect(() => {
    if (holdings.length === 0) return;
    let alive = true;
    const symbols = holdings.map((h) => h.symbol).join(",");
    Promise.all([
      apiRequest<Record<string, Close[]>>(
        `/api/v1/stocks/daily-closes/batch?symbols=${encodeURIComponent(symbols)}&days=200`,
      ),
      fetchIndexDailyCloses(indexId, 120)
        .then((s) => s.rows.map((r) => ({ d: r.tradeDate, c: r.close })))
        .catch(() => null),
    ])
      .then(([closes, index]) => alive && setData({ key, closes, index }))
      .catch(
        (e: unknown) =>
          alive && setData({ key, error: e instanceof Error ? e.message : "종가를 불러오지 못했습니다" }),
      );
    return () => {
      alive = false;
    };
    // key가 종목·비중을 대표한다 — holdings 배열의 새 참조마다 다시 부르지 않는다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, indexId]);

  const current = data?.key === key ? data : null;
  const risk = useMemo(
    () =>
      current?.closes
        ? computePortfolioRisk(
            holdings.map((h) => ({ symbol: h.symbol, name: h.name, weight: h.weight })),
            current.closes,
            current.index ?? null,
          )
        : null,
    [current, holdings],
  );

  if (holdings.length === 0) return null;

  return (
    <Card className="p-3">
      <div className="flex items-baseline justify-between gap-2">
        <h2 className="text-section font-semibold text-[var(--text-primary)]">위험</h2>
        {risk && <span className="text-micro text-[var(--text-muted)]">최근 {risk.days}거래일</span>}
      </div>
      {current?.error ? (
        <p className="mt-2 text-cell text-[var(--text-muted)]">{current.error}</p>
      ) : !current ? (
        <p className="mt-2 text-cell text-[var(--text-muted)]">계산하는 중…</p>
      ) : !risk ? (
        <p className="mt-2 text-cell text-[var(--text-muted)]">
          종가가 40거래일 이상 쌓인 종목이 없어 위험을 잴 수 없습니다.
        </p>
      ) : (
        <>
          <dl className="mt-2 grid grid-cols-3 gap-2">
            <div className="rounded-md border border-[var(--border)] px-2 py-1.5">
              <dt className="text-micro text-[var(--text-muted)]">변동성(연)</dt>
              <dd className="font-num text-cell font-semibold tabular-nums">{risk.volatilityPct.toFixed(1)}%</dd>
            </div>
            <div className="rounded-md border border-[var(--border)] px-2 py-1.5">
              <dt className="text-micro text-[var(--text-muted)]">최대낙폭</dt>
              <dd className="font-num text-cell font-semibold tabular-nums text-[var(--loss)]">
                {risk.maxDrawdownPct.toFixed(1)}%
              </dd>
            </div>
            <div className="rounded-md border border-[var(--border)] px-2 py-1.5">
              <dt className="text-micro text-[var(--text-muted)]">{indexLabel} 베타</dt>
              <dd className="font-num text-cell font-semibold tabular-nums">
                {risk.beta == null ? "-" : risk.beta.toFixed(2)}
              </dd>
            </div>
          </dl>
          <div className="mt-2">
            <p className="text-micro font-semibold text-[var(--text-muted)]">함께 움직이는 쌍</p>
            {risk.pairs.length === 0 ? (
              <p className="text-cell text-[var(--text-muted)]">상관 0.6 넘게 함께 움직이는 쌍이 없습니다.</p>
            ) : (
              <ul className="mt-1 space-y-1">
                {risk.pairs.map((p) => (
                  <li key={`${p.a}-${p.b}`} className="flex items-baseline gap-2 text-cell">
                    <span className="min-w-0 flex-1 truncate">
                      {p.aName} · {p.bName}
                    </span>
                    <span className="shrink-0 font-num tabular-nums text-[var(--text-muted)]">
                      상관 {p.corr.toFixed(2)} · 합계 비중 {p.weight.toFixed(0)}%
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <p className="mt-2 text-micro text-[var(--text-muted)]">
            지금 비중으로 과거를 들고 있었다고 가정한 값입니다(실제 계좌 곡선이 아님 · 환율 미반영).
            {risk.skipped.length > 0 && ` 종가가 부족한 ${risk.skipped.length}종목은 빠졌습니다.`}
          </p>
          <HoldingsWhatIf
            holdings={holdings.map((h) => ({ symbol: h.symbol, name: h.name, weight: h.weight, sector: h.sector }))}
            closes={current!.closes!}
            index={current!.index ?? null}
          />
        </>
      )}
    </Card>
  );
}
