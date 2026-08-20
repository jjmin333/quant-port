"use client";

import { useMemo, useState } from "react";
import { LayoutGrid, Table as TableIcon } from "lucide-react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  formatKRW,
  formatPercent,
  formatPrice,
  formatPriceChange,
} from "@/lib/format";
import {
  sortHoldingMetrics,
  type HoldingMetrics,
  type HoldingSortKey,
} from "@/lib/holdings-calc";
import { setHoldingsView, useHoldingsView, type HoldingsView } from "@/lib/holdings-view";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 보유 목록 — 표와 카드 두 벌.
 *
 * 표가 기본인 이유: 한 종목이 한 줄이라 평단·손익·비중이 같은 세로선 위에 놓이고,
 * 스무 종목이 한 화면에 들어온다. 카드형은 한 종목당 100px을 써서 열 종목만 넘어도
 * 스크롤이 길어지지만, 종목이 몇 개 안 되거나 모바일에서는 읽기 쉽다.
 *
 * 어느 쪽이든 행을 누르면 상세 모달이 뜬다 — 종목 분석 화면으로 나가버리면 "내 평단·
 * 내 손익·내 비중"이라는 맥락이 사라진다.
 */

/**
 * 등락 강도 배경. 색은 기존 규칙대로 빨강(상승)·파랑(하락)이고 **농도로 크기**를 말한다.
 * ±2% 미만은 거의 칠하지 않아 평범한 날에 화면이 알록달록해지지 않게 한다.
 * (`score-ranking-panel`과 같은 규칙 — 두 목록이 다른 농도를 쓰면 비교가 안 된다.)
 */
function heatStyle(rate: number | undefined): React.CSSProperties | undefined {
  if (typeof rate !== "number" || rate === 0) return undefined;
  const abs = Math.abs(rate);
  const alpha = abs >= 5 ? "22%" : abs >= 2 ? "13%" : "6%";
  const base = rate > 0 ? "var(--profit)" : "var(--loss)";
  return { backgroundColor: `color-mix(in oklab, ${base} ${alpha}, transparent)` };
}

function formatMarketValueDisplay(m: HoldingMetrics, exchangeRate: number): string {
  if (m.currency === "USD" && m.marketValue != null && m.marketValue > 0) {
    return formatPrice(m.marketValue, "USD", {
      exchangeRate: exchangeRate > 0 ? exchangeRate : undefined,
      krwValue: m.marketValueKRW ?? undefined,
    });
  }
  if (m.marketValueKRW == null) return "—";
  return formatKRW(m.marketValueKRW);
}

function DayChangeBadge({ rate }: { rate: number }) {
  return (
    <div className="shrink-0 text-right leading-none">
      <p className="mb-0.5 text-[9px] font-medium text-muted-foreground">전일대비</p>
      <span
        style={heatStyle(rate)}
        className={cn(
          "inline-block rounded px-1.5 py-0.5 font-mono text-xs tabular-nums",
          trendTextClass(rate),
        )}
      >
        {formatPercent(rate)}
      </span>
    </div>
  );
}

export function HoldingsViewToggle() {
  const view = useHoldingsView();
  return (
    <div className="flex items-center gap-0.5 rounded-md bg-muted p-0.5">
      {(
        [
          ["table", "표", TableIcon],
          ["card", "카드", LayoutGrid],
        ] as const
      ).map(([value, label, Icon]) => (
        <button
          key={value}
          type="button"
          onClick={() => setHoldingsView(value as HoldingsView)}
          aria-pressed={view === value}
          className={cn(
            "flex h-6 items-center gap-1 rounded px-2 text-[11px] transition-colors",
            view === value
              ? "bg-card font-semibold text-foreground shadow-xs"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          <Icon className="size-3" />
          {label}
        </button>
      ))}
    </div>
  );
}

type ListProps = {
  metrics: HoldingMetrics[];
  loading: boolean;
  filterQuery?: string;
  exchangeRate: number;
  onSelect: (symbol: string) => void;
};

export function HoldingsList(props: ListProps) {
  const view = useHoldingsView();
  const { metrics, loading, filterQuery = "" } = props;

  const filtered = useMemo(() => {
    const q = filterQuery.trim().toLowerCase();
    if (!q) return metrics;
    return metrics.filter(
      (m) => m.symbol.toLowerCase().includes(q) || m.name.toLowerCase().includes(q),
    );
  }, [metrics, filterQuery]);

  if (loading) {
    return (
      <Card className="p-6 text-center text-xs text-muted-foreground">
        보유 종목을 불러오는 중…
      </Card>
    );
  }

  if (filtered.length === 0) {
    return (
      <Card className="p-8 text-center text-xs text-muted-foreground">
        {filterQuery.trim() ? "검색 결과가 없습니다." : "등록된 보유 종목이 없습니다."}
      </Card>
    );
  }

  return view === "card" ? (
    <HoldingCards {...props} metrics={filtered} />
  ) : (
    <HoldingTable {...props} metrics={filtered} />
  );
}

const COLUMNS: Array<{ key: HoldingSortKey; label: string; className?: string }> = [
  { key: "name", label: "종목", className: "text-left" },
  { key: "quantity", label: "수량" },
  { key: "avgPrice", label: "평단" },
  { key: "price", label: "현재가" },
  { key: "change", label: "전일대비" },
  { key: "value", label: "평가금액" },
  { key: "pnl", label: "평가손익" },
  { key: "weight", label: "비중" },
];

function HoldingTable({ metrics, exchangeRate, onSelect }: ListProps) {
  const [sort, setSort] = useState<HoldingSortKey>("value");
  const [desc, setDesc] = useState(true);

  const rows = useMemo(() => sortHoldingMetrics(metrics, sort, desc), [metrics, sort, desc]);
  const usesUsd = metrics.some((m) => m.currency === "USD");

  const totals = useMemo(() => {
    let value = 0;
    let cost = 0;
    let valueWithCost = 0;
    let hasValue = false;
    for (const m of rows) {
      if (m.marketValueKRW == null) continue;
      hasValue = true;
      value += m.marketValueKRW;
      if (m.costBasisKRW != null) {
        cost += m.costBasisKRW;
        valueWithCost += m.marketValueKRW;
      }
    }
    const pnl = cost > 0 ? valueWithCost - cost : null;
    return {
      hasValue,
      value,
      pnl,
      rate: pnl != null && cost > 0 ? (pnl / cost) * 100 : null,
    };
  }, [rows]);

  function toggle(key: HoldingSortKey) {
    if (sort === key) {
      setDesc((d) => !d);
      return;
    }
    setSort(key);
    // 이름은 가나다순이 자연스럽고 나머지 숫자는 큰 값이 위가 자연스럽다.
    setDesc(key !== "name");
  }

  return (
    <Card className="p-0">
      {usesUsd && (
        <p className="border-b px-3 py-1.5 text-[10px] text-muted-foreground">
          달러 종목은 현재 환율 {exchangeRate > 0 ? formatKRW(exchangeRate) : "—"} 기준으로
          환산했습니다.
          매입 시점 환율이 없어 환차손익은 따로 떼어내지 않습니다.
        </p>
      )}
      {/* 표는 좁은 화면에서 자기 안에서 가로 스크롤한다 — 페이지가 가로로 밀리면 안 된다. */}
      <div className="overflow-x-auto">
        <table className="w-full min-w-[680px] border-collapse text-xs">
          <thead>
            <tr>
              {COLUMNS.map((col) => (
                <th
                  key={col.key}
                  className={cn(
                    "border-b px-2 py-1.5 text-right text-[11px] font-medium whitespace-nowrap",
                    col.className,
                    sort === col.key ? "font-semibold text-foreground" : "text-muted-foreground",
                  )}
                >
                  <button
                    type="button"
                    onClick={() => toggle(col.key)}
                    className="hover:text-foreground"
                  >
                    {col.label}
                    {sort === col.key ? (desc ? " ▾" : " ▴") : ""}
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((m) => (
              <tr
                key={m.symbol}
                onClick={() => onSelect(m.symbol)}
                className="cursor-pointer border-b border-border/55 hover:bg-accent"
              >
                <td className="max-w-0 px-2 py-1.5">
                  <div className="flex min-w-0 items-center gap-2">
                    <StockAvatar
                      symbol={m.symbol}
                      name={m.name}
                      currency={m.currency}
                      assetType={m.row.quote?.assetType}
                      size="xs"
                      className="shrink-0"
                    />
                    <span className="min-w-0 truncate">
                      {m.name}
                      <span className="ml-1 text-[10px] text-muted-foreground">{m.symbol}</span>
                    </span>
                  </div>
                </td>
                <td className="px-2 py-1.5 text-right font-mono tabular-nums whitespace-nowrap">
                  {m.quantity}
                </td>
                <td className="px-2 py-1.5 text-right font-mono tabular-nums whitespace-nowrap">
                  {m.avgPrice == null ? (
                    <span className="text-muted-foreground">—</span>
                  ) : (
                    formatPrice(m.avgPrice, m.currency)
                  )}
                </td>
                <td className="px-2 py-1.5 text-right font-mono tabular-nums whitespace-nowrap">
                  {m.hasQuote ? (
                    formatPrice(m.currentPrice, m.currency)
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </td>
                <td className="px-2 py-1.5 text-right whitespace-nowrap">
                  {m.hasQuote ? (
                    <span
                      style={heatStyle(m.changeRate)}
                      className={cn(
                        "inline-block min-w-[58px] rounded px-1 text-right font-mono tabular-nums",
                        trendTextClass(m.changeRate),
                      )}
                    >
                      {formatPercent(m.changeRate)}
                    </span>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </td>
                <td className="px-2 py-1.5 text-right font-mono tabular-nums whitespace-nowrap">
                  {formatMarketValueDisplay(m, exchangeRate)}
                </td>
                <td className="px-2 py-1.5 text-right whitespace-nowrap">
                  {m.unrealizedPnlKRW == null || m.returnRate == null ? (
                    <span className="text-muted-foreground">—</span>
                  ) : (
                    <span className={cn("font-mono tabular-nums", trendTextClass(m.unrealizedPnlKRW))}>
                      {formatPriceChange(Math.round(m.unrealizedPnlKRW), "KRW")}
                      <span className="block text-[10px]">{formatPercent(m.returnRate)}</span>
                    </span>
                  )}
                </td>
                <td className="px-2 py-1.5 text-right font-mono tabular-nums whitespace-nowrap">
                  {m.weight == null ? (
                    <span className="text-muted-foreground">—</span>
                  ) : (
                    <>
                      {m.weight.toFixed(1)}%
                      <span className="mt-0.5 ml-auto block h-1 w-12 rounded-full bg-muted">
                        <span
                          className="block h-1 rounded-full bg-[var(--brand)]"
                          style={{ width: `${Math.min(100, m.weight)}%` }}
                        />
                      </span>
                    </>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
          {totals.hasValue && (
            <tfoot>
              <tr className="font-semibold">
                <td className="px-2 pt-2 text-left">합계</td>
                <td colSpan={4} />
                <td className="px-2 pt-2 text-right font-mono tabular-nums">
                  {formatKRW(totals.value)}
                </td>
                <td className="px-2 pt-2 text-right">
                  {totals.pnl == null || totals.rate == null ? (
                    <span className="text-muted-foreground">—</span>
                  ) : (
                    <span className={cn("font-mono tabular-nums", trendTextClass(totals.pnl))}>
                      {formatPriceChange(Math.round(totals.pnl), "KRW")}
                      <span className="block text-[10px]">{formatPercent(totals.rate)}</span>
                    </span>
                  )}
                </td>
                <td className="px-2 pt-2 text-right font-mono tabular-nums">100%</td>
              </tr>
            </tfoot>
          )}
        </table>
      </div>
    </Card>
  );
}

function HoldingCards({ metrics, exchangeRate, onSelect }: ListProps) {
  const rows = useMemo(() => sortHoldingMetrics(metrics, "value", true), [metrics]);

  return (
    <div className="grid gap-2 sm:grid-cols-2 2xl:grid-cols-3">
      {rows.map((m) => (
        <Card
          key={m.symbol}
          onClick={() => onSelect(m.symbol)}
          className="cursor-pointer p-3 transition-colors hover:bg-accent"
        >
          <div className="flex min-w-0 items-start gap-2">
            <StockAvatar
              symbol={m.symbol}
              name={m.name}
              currency={m.currency}
              assetType={m.row.quote?.assetType}
              size="md"
              className="shrink-0"
            />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-semibold">{m.name}</p>
              <p className="truncate text-[10px] text-muted-foreground">
                {m.symbol} · {m.currency}
                {m.row.brokerHint ? ` · ${m.row.brokerHint}` : ""}
              </p>
            </div>
            {m.hasQuote && <DayChangeBadge rate={m.changeRate} />}
          </div>

          <div className="mt-2 flex items-end justify-between gap-2 border-t pt-2">
            <div className="min-w-0">
              <p className="text-[10px] text-muted-foreground">평가금액</p>
              <p
                className={cn(
                  "font-mono font-bold tabular-nums",
                  m.currency === "USD" ? "text-sm leading-snug" : "text-base",
                )}
              >
                {formatMarketValueDisplay(m, exchangeRate)}
              </p>
            </div>
            <div className="min-w-0 text-right">
              <p className="text-[10px] text-muted-foreground">평가손익</p>
              {m.unrealizedPnlKRW == null || m.returnRate == null ? (
                <p className="font-mono text-sm text-muted-foreground">—</p>
              ) : (
                <p
                  className={cn(
                    "font-mono text-sm font-semibold tabular-nums",
                    trendTextClass(m.unrealizedPnlKRW),
                  )}
                >
                  {formatPercent(m.returnRate)}
                  <span className="block text-[10px] font-normal">
                    {formatPriceChange(Math.round(m.unrealizedPnlKRW), "KRW")}
                  </span>
                </p>
              )}
            </div>
          </div>

          <div className="mt-1.5 grid grid-cols-3 gap-1 text-[10px] text-muted-foreground">
            <span>
              수량 <b className="font-mono text-foreground">{m.quantity}</b>
            </span>
            <span>
              평단{" "}
              <b className="font-mono text-foreground">
                {m.avgPrice == null ? "—" : formatPrice(m.avgPrice, m.currency)}
              </b>
            </span>
            <span className="text-right">
              비중{" "}
              <b className="font-mono text-foreground">
                {m.weight == null ? "—" : `${m.weight.toFixed(1)}%`}
              </b>
            </span>
          </div>
        </Card>
      ))}
    </div>
  );
}

/** 목록 위 툴바에 함께 놓을 "직접 추가" 버튼 자리. */
export function HoldingsToolbar({
  count,
  onAdd,
  children,
}: {
  count: number;
  onAdd?: () => void;
  children?: React.ReactNode;
}) {
  return (
    <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <p className="text-xs font-semibold">보유 종목 {count}</p>
      <div className="flex items-center gap-2">
        {children}
        <HoldingsViewToggle />
        {onAdd && (
          <Button size="sm" variant="outline" onClick={onAdd}>
            + 종목 추가
          </Button>
        )}
      </div>
    </div>
  );
}
