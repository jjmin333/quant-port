"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { apiRequest } from "@/lib/api-client";
import { fetchIndexDailyCloses } from "@/lib/daily-close-api";
import {
  applyWhatIf,
  computePortfolioRisk,
  sectorShares,
  type Close,
  type PortfolioRisk,
  type WhatIfHolding,
} from "@/lib/portfolio-risk";
import type { StockInfo } from "@/lib/quotes-types";
import type { StockSeoSummary } from "@/lib/stock-seo";
import { cn } from "@/lib/utils";

type Result = {
  label: string;
  before: { risk: PortfolioRisk | null; weightPct: number; topSector?: { sector: string; pct: number } };
  after: { risk: PortfolioRisk | null; weightPct: number; topSector?: { sector: string; pct: number } };
  note?: string;
};

function weightOf(holdings: WhatIfHolding[], symbol: string): number {
  const total = holdings.reduce((s, h) => s + h.weight, 0);
  const h = holdings.find((x) => x.symbol === symbol);
  return total > 0 && h ? (h.weight / total) * 100 : 0;
}

function fmt(v: number | null | undefined, digits = 1, suffix = "%"): string {
  return v == null || !Number.isFinite(v) ? "-" : `${v.toFixed(digits)}${suffix}`;
}

/**
 * 「바꿔 보기」 — 위험 카드 안의 접힌 줄 (2026-09-24 사용자 결정).
 *
 * 종목과 수량(음수는 매도)을 넣으면 **지금 비중 가정**의 위험 숫자와 그 종목 비중 · 가장 큰
 * 섹터 비중의 전→후를 보여 준다. 저장하지 않고, 사라·팔라는 말을 하지 않는다. 가격은 저장된
 * 마지막 종가(달러 종목은 저장된 원/달러로 환산)다.
 */
export function HoldingsWhatIf({
  holdings,
  closes,
  index,
}: {
  holdings: WhatIfHolding[];
  closes: Record<string, Close[]>;
  index: Close[] | null;
}) {
  const [open, setOpen] = useState(false);
  const [picked, setPicked] = useState<StockInfo | null>(null);
  const [qty, setQty] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<Result | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function run() {
    const n = Number(qty.replace(/,/g, ""));
    if (!picked || !Number.isFinite(n) || n === 0) {
      setError("종목과 0이 아닌 수량을 넣어 주세요(파는 수량은 음수).");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const symbol = picked.symbol.toUpperCase();
      const [batch, summary, fx] = await Promise.all([
        closes[symbol]
          ? Promise.resolve({ [symbol]: closes[symbol]! })
          : apiRequest<Record<string, Close[]>>(
              `/api/v1/stocks/daily-closes/batch?symbols=${encodeURIComponent(symbol)}&days=200`,
            ),
        apiRequest<StockSeoSummary | null>(`/api/v1/seo/stocks/${encodeURIComponent(symbol)}`).catch(
          () => null,
        ),
        picked.currency === "USD"
          ? fetchIndexDailyCloses("usd-krw", 5).then((s) => s.rows.at(-1)?.close ?? null)
          : Promise.resolve(1),
      ]);
      const series = batch[symbol] ?? [];
      const last = series.at(-1)?.c;
      if (!last || !fx) {
        setError("이 종목의 저장된 종가(또는 환율)가 없어 계산할 수 없습니다.");
        return;
      }
      const deltaKRW = n * last * fx;
      const after = applyWhatIf(holdings, {
        symbol,
        name: picked.name,
        deltaKRW,
        sector: summary?.sectorLabel,
      });
      const allCloses = { ...closes, [symbol]: series };
      const strip = (hs: WhatIfHolding[]) => hs.map(({ symbol: s, name, weight }) => ({ symbol: s, name, weight }));
      setResult({
        label: `${picked.name} ${n > 0 ? "+" : ""}${n.toLocaleString("ko-KR")}주 (약 ${Math.round(
          Math.abs(deltaKRW) / 10000,
        ).toLocaleString("ko-KR")}만원)`,
        before: {
          risk: computePortfolioRisk(strip(holdings), allCloses, index),
          weightPct: weightOf(holdings, symbol),
          topSector: sectorShares(holdings)[0],
        },
        after: {
          risk: computePortfolioRisk(strip(after), allCloses, index),
          weightPct: weightOf(after, symbol),
          topSector: sectorShares(after)[0],
        },
        note:
          n < 0 && !holdings.some((h) => h.symbol === symbol)
            ? "보유하지 않은 종목은 팔 수 없어 그대로입니다."
            : undefined,
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "계산하지 못했습니다");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mt-2 border-t border-[var(--border)] pt-2">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-1 text-cell font-semibold text-[var(--text-muted)] hover:text-foreground"
        aria-expanded={open}
      >
        {open ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
        바꿔 보기
      </button>
      {open && (
        <div className="mt-2 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <StockCatalogSearchField
              placeholder={picked ? `${picked.name} (${picked.symbol})` : "종목 검색"}
              suggestionsId="holdings-what-if-suggestions"
              onSelect={(s) => {
                setPicked(s);
                setResult(null);
              }}
              className="min-w-[12rem] flex-1"
            />
            <input
              value={qty}
              onChange={(e) => setQty(e.target.value)}
              inputMode="numeric"
              placeholder="수량 (파는 건 −)"
              aria-label="바꿀 수량"
              className="w-28 rounded-md border border-[var(--border)] bg-transparent px-2 py-1 text-cell outline-none focus:border-[var(--brand)]"
            />
            <button
              type="button"
              onClick={() => void run()}
              disabled={busy}
              className="rounded-md border border-[var(--brand)] px-2.5 py-1 text-cell font-semibold text-[var(--brand)] disabled:opacity-50"
            >
              {busy ? "계산 중…" : "계산"}
            </button>
          </div>
          {error && <p className="text-cell text-[var(--text-muted)]">{error}</p>}
          {result && (
            <div>
              <p className="text-cell font-semibold">{result.label}</p>
              <table className="mt-1 w-full text-cell">
                <thead className="text-micro text-[var(--text-muted)]">
                  <tr>
                    <th className="py-0.5 text-left font-medium" />
                    <th className="py-0.5 text-right font-medium">지금</th>
                    <th className="py-0.5 text-right font-medium">바꾸면</th>
                  </tr>
                </thead>
                <tbody className="font-num tabular-nums">
                  {(
                    [
                      ["그 종목 비중", fmt(result.before.weightPct), fmt(result.after.weightPct)],
                      [
                        "가장 큰 섹터",
                        result.before.topSector
                          ? `${result.before.topSector.sector} ${fmt(result.before.topSector.pct, 0)}`
                          : "-",
                        result.after.topSector
                          ? `${result.after.topSector.sector} ${fmt(result.after.topSector.pct, 0)}`
                          : "-",
                      ],
                      ["변동성(연)", fmt(result.before.risk?.volatilityPct), fmt(result.after.risk?.volatilityPct)],
                      ["최대낙폭", fmt(result.before.risk?.maxDrawdownPct), fmt(result.after.risk?.maxDrawdownPct)],
                      [
                        "베타",
                        fmt(result.before.risk?.beta, 2, ""),
                        fmt(result.after.risk?.beta, 2, ""),
                      ],
                    ] as const
                  ).map(([label, a, b]) => (
                    <tr key={label} className="border-t border-[var(--border)]">
                      <td className="py-0.5 font-sans text-[var(--text-muted)]">{label}</td>
                      <td className="py-0.5 text-right">{a}</td>
                      <td className={cn("py-0.5 text-right", a !== b && "font-semibold")}>{b}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <p className="mt-1 text-micro text-[var(--text-muted)]">
                마지막 종가 기준 가정 · 저장하지 않음
                {result.note ? ` ${result.note}` : ""}
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
