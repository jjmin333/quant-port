"use client";

import { PriceDisplay } from "@/components/ui/price-display";
import { formatChangeRate } from "@/lib/format";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import type { QuoteDetail } from "@/lib/quotes-types";
import { quoteChangeBaseline, quoteChangeBasisLabel } from "@/lib/quotes-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * Viewport width at which the metrics rail earns its 240px.
 *
 * `lg` suits pages where the workspace owns the whole content area. 마켓 레이더
 * already spends ~320px on the ranking list, so there the rail must wait for
 * `xl` — at 1024–1279 it left the tab content only ~208px wide, too narrow for
 * the financial tables and charts it has to hold.
 */
export type MetricsRailBreakpoint = "lg" | "xl";

const RAIL_VISIBILITY: Record<MetricsRailBreakpoint, string> = {
  lg: "hidden lg:flex lg:w-[240px]",
  xl: "hidden xl:flex xl:w-[240px]",
};

/** Inverse of RAIL_VISIBILITY — for the grid that stands in below the rail. */
export const METRICS_GRID_VISIBILITY: Record<MetricsRailBreakpoint, string> = {
  lg: "lg:hidden",
  xl: "xl:hidden",
};

type StockMetricsColumnProps = {
  quote: QuoteDetail | null;
  fundamentals: FundamentalsData | null;
  /** full: 시세 + 52주 범위. fundamentals: 재무 지표만 (종목 분석). */
  variant?: "full" | "fundamentals";
  railFrom?: MetricsRailBreakpoint;
  className?: string;
};

type MetricRow =
  | { label: string; price: number }
  | { label: string; text: string; tone?: number };

type MetricGroup = { title: string; rows: MetricRow[] };

/**
 * 시세 rows for a loaded quote — shared by the desktop rail and the touch grid
 * (StockMetricsGrid).
 *
 * Scope is deliberately narrow: only 시세 and 52주 범위 live in this rail. 거래
 * (거래량/거래대금/시가총액) and 재무 (PER/PBR/ROE) were removed because they
 * duplicate the 기업 정보 and 밸류에이션 tabs, and the rail's job is the
 * at-a-glance price picture, not a second copy of the whole workspace.
 */
function buildQuoteMetricGroups(quote: QuoteDetail): MetricGroup[] {
  const changeBaseline = quoteChangeBaseline(quote);

  return [
    {
      title: "시세",
      rows: [
        { label: "전일", price: quote.prevClose },
        ...(quote.regularClose != null && quote.regularClose > 0
          ? [{ label: "정규장 종가", price: quote.regularClose } satisfies MetricRow]
          : []),
        { label: "시가", price: quote.open },
        { label: "고가", price: quote.high },
        { label: "저가", price: quote.low },
        {
          label: quoteChangeBasisLabel(quote),
          text: formatChangeRate(quote.changeRate, changeBaseline > 0),
          tone: quote.changeRate,
        },
      ],
    },
  ];
}

function MetricSection({
  title,
  rows,
  currency,
}: {
  title: string;
  rows: MetricRow[];
  currency: "KRW" | "USD";
}) {
  return (
    <section className="border-b border-[var(--border)] last:border-b-0">
      <h3 className="bg-[var(--surface-elevated)]/50 px-2.5 py-1.5 text-[10px] font-semibold uppercase tracking-wide text-[var(--text-muted)]">
        {title}
      </h3>
      <dl>
        {rows.map((row) => (
          <div
            key={row.label}
            className="flex items-center justify-between gap-2 border-t border-[var(--border)]/50 px-2.5 py-1.5"
          >
            <dt className="text-[10px] text-[var(--text-muted)]">{row.label}</dt>
            <dd
              className={cn(
                "text-right text-[11px] font-medium tabular-nums",
                "tone" in row && row.tone != null
                  ? trendTextClass(row.tone)
                  : "text-[var(--text-primary)]",
              )}
            >
              {"price" in row ? (
                row.price > 0 ? (
                  <PriceDisplay value={row.price} currency={currency} size="sm" className="text-[11px]" />
                ) : (
                  "—"
                )
              ) : (
                row.text
              )}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

export function StockMetricsColumn({
  quote,
  fundamentals,
  variant = "full",
  railFrom = "lg",
  className,
}: StockMetricsColumnProps) {
  const railVisibility = RAIL_VISIBILITY[railFrom];

  if (variant === "fundamentals") {
    const financeRows: MetricRow[] = [
      {
        label: "PER",
        text:
          fundamentals?.per != null && fundamentals.per > 0
            ? fundamentals.per.toFixed(2)
            : "—",
      },
      {
        label: "PBR",
        text:
          fundamentals?.pbr != null && fundamentals.pbr > 0
            ? fundamentals.pbr.toFixed(2)
            : "—",
      },
      {
        label: "ROE",
        text:
          fundamentals?.roe != null && fundamentals.roe !== 0
            ? `${fundamentals.roe.toFixed(2)}%`
            : "—",
      },
      {
        label: "EPS",
        text:
          fundamentals?.epsBasic != null && fundamentals.epsBasic !== 0
            ? fundamentals.epsBasic.toFixed(2)
            : "—",
      },
    ];

    return (
      <aside
        className={cn(
          "shrink-0 flex-col overflow-y-auto border-r border-[var(--border)] bg-[var(--surface)]",
          railVisibility,
          className,
        )}
      >
        <MetricSection title="재무" rows={financeRows} currency={quote?.currency ?? "KRW"} />
      </aside>
    );
  }

  if (!quote) {
    return (
      // Hidden below the rail breakpoint like the loaded state: without that,
      // the loading placeholder rendered a 220px dead column on phones.
      <aside
        className={cn(
          "w-[220px] shrink-0 items-center justify-center border-r border-[var(--border)] bg-[var(--surface)] text-[11px] text-[var(--text-muted)]",
          railVisibility,
          className,
        )}
      >
        시세 로딩 중…
      </aside>
    );
  }

  const currency = quote.currency;
  const range52 =
    quote.week52High > quote.week52Low
      ? ((quote.currentPrice - quote.week52Low) / (quote.week52High - quote.week52Low)) * 100
      : 50;

  const groups = buildQuoteMetricGroups(quote);

  return (
    <aside
      className={cn(
        "shrink-0 flex-col overflow-y-auto border-r border-[var(--border)] bg-[var(--surface)]",
        railVisibility,
        className,
      )}
    >
      {groups.map((group) => (
        <MetricSection key={group.title} title={group.title} rows={group.rows} currency={currency} />
      ))}
      {quote.week52High > 0 && quote.week52Low > 0 && (
        <div className="border-b border-[var(--border)] px-2.5 py-2">
          <p className="text-[10px] text-[var(--text-muted)]">52주 범위</p>
          <div className="mt-1.5 flex items-center gap-1.5 text-[10px] text-[var(--text-muted)]">
            <PriceDisplay value={quote.week52Low} currency={currency} size="sm" className="text-[10px]" />
            <div className="relative h-1 flex-1 rounded-full bg-[var(--surface-elevated)]">
              <div
                className="absolute top-1/2 h-2 w-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-[var(--text-primary)]"
                style={{ left: `${Math.min(100, Math.max(0, range52))}%` }}
              />
            </div>
            <PriceDisplay value={quote.week52High} currency={currency} size="sm" className="text-[10px]" />
          </div>
        </div>
      )}
    </aside>
  );
}

/**
 * The same 시세 rows and 52주 범위 as the desktop rail, laid out as cards for the
 * 개요 tab on touch viewports where the rail is hidden. Without this, phones had
 * no way to reach 전일/시가/고가/저가/52주 범위 at all.
 */
export function StockMetricsGrid({
  quote,
  className,
}: {
  quote: QuoteDetail;
  className?: string;
}) {
  const currency = quote.currency;
  const groups = buildQuoteMetricGroups(quote);
  const range52 =
    quote.week52High > quote.week52Low
      ? ((quote.currentPrice - quote.week52Low) / (quote.week52High - quote.week52Low)) * 100
      : 50;

  return (
    <div className={cn("space-y-2", className)}>
      <div className="grid grid-cols-1 gap-2">
        {groups.map((group) => (
          <section
            key={group.title}
            className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-2.5"
          >
            <h3 className="text-[10px] font-semibold uppercase tracking-wide text-[var(--text-muted)]">
              {group.title}
            </h3>
            {/* Two columns even at 375px: 시세 rows are short label/value pairs,
                so a single column would leave the row half empty. */}
            <dl className="mt-1.5 grid grid-cols-2 gap-x-3 gap-y-1">
              {group.rows.map((row) => (
                <div key={row.label} className="flex items-baseline justify-between gap-2">
                  <dt className="shrink-0 text-[11px] text-[var(--text-muted)]">{row.label}</dt>
                  <dd
                    className={cn(
                      "truncate text-right text-[12px] font-medium tabular-nums",
                      "tone" in row && row.tone != null
                        ? trendTextClass(row.tone)
                        : "text-[var(--text-primary)]",
                    )}
                  >
                    {"price" in row ? (
                      row.price > 0 ? (
                        <PriceDisplay
                          value={row.price}
                          currency={currency}
                          size="sm"
                          className="text-[12px]"
                        />
                      ) : (
                        "—"
                      )
                    ) : (
                      row.text
                    )}
                  </dd>
                </div>
              ))}
            </dl>
          </section>
        ))}
      </div>

      {quote.week52High > 0 && quote.week52Low > 0 && (
        <section className="rounded-lg border border-[var(--border)] bg-[var(--surface)] p-2.5">
          <h3 className="text-[10px] font-semibold uppercase tracking-wide text-[var(--text-muted)]">
            52주 범위
          </h3>
          <div className="mt-2 flex items-center gap-2 text-[11px] text-[var(--text-muted)]">
            <PriceDisplay value={quote.week52Low} currency={currency} size="sm" className="text-[11px]" />
            <div className="relative h-1.5 flex-1 rounded-full bg-[var(--surface-elevated)]">
              <div
                className="absolute top-1/2 h-2.5 w-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-[var(--text-primary)]"
                style={{ left: `${Math.min(100, Math.max(0, range52))}%` }}
              />
            </div>
            <PriceDisplay value={quote.week52High} currency={currency} size="sm" className="text-[11px]" />
          </div>
        </section>
      )}
    </div>
  );
}
