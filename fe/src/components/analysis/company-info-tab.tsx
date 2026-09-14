"use client";

import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";
import { Card } from "@/components/ui/card";
import { formatListDate, formatMarketCap } from "@/lib/format";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";

type CompanyInfoTabProps = {
  stock: StockInfo;
  quote: QuoteDetail | null;
  fundamentals: FundamentalsData | null;
};

const SEGMENT_COLORS = ["#3B82F6", "#10B981", "#14B8A6", "#F59E0B", "#8B5CF6"];

export function CompanyInfoTab({ stock, quote, fundamentals }: CompanyInfoTabProps) {
  const marketLabel = stock.currency === "KRW" ? "국내" : "해외";
  const shares = stock.sharesOutstanding
    ? Number.parseInt(stock.sharesOutstanding.replace(/,/g, ""), 10)
    : 0;

  const summary =
    fundamentals?.coverageTier === "not_applicable"
      ? fundamentals.message ?? "ETF·펀드 등은 개별 기업 재무제표가 제공되지 않습니다."
      : `${stock.name}(${stock.symbol}) — ${stock.market} 상장 ${marketLabel} 종목입니다.${
          fundamentals?.fiscalYear ? ` 최근 회계연도 FY${fundamentals.fiscalYear} 공시 기준.` : ""
        }`;

  const stats = [
    {
      label: "시가총액",
      value:
        quote && quote.marketCap > 0
          ? formatMarketCap(quote.marketCap, stock.currency)
          : "—",
    },
    {
      label: "발행주식수",
      value: shares > 0 ? `${shares.toLocaleString("ko-KR")}주` : "—",
    },
    {
      label: "상장일",
      value: stock.listDate ? formatListDate(stock.listDate) : "—",
    },
    { label: "시장", value: stock.market || "—" },
    { label: "통화", value: stock.currency },
    {
      label: "회계연도",
      value: fundamentals?.fiscalYear ? `FY${fundamentals.fiscalYear}` : "—",
    },
  ];

  const segmentData = buildSegmentPlaceholder(stock, fundamentals);

  return (
    <Card className="p-0">
      <div className="border-b border-[var(--border)] px-4 py-3">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="text-section font-semibold text-[var(--text-primary)]">기업 정보</h2>
            <p className="mt-0.5 text-micro text-[var(--text-muted)]">
              {fundamentals?.dataSource === "dart"
                ? "출처: DART 공시"
                : fundamentals?.dataSource === "edgar"
                  ? "출처: EDGAR · 기업 IR"
                  : "출처: 종목 마스터 · 시세"}
            </p>
          </div>
        </div>
        <p className="mt-2 rounded-lg bg-[var(--surface-elevated)]/60 px-3 py-2 text-read text-[var(--text-secondary)]">
          {summary}
        </p>
      </div>

      <dl className="grid grid-cols-2 gap-px border-b border-[var(--border)] bg-[var(--border)] sm:grid-cols-3">
        {stats.map((item) => (
          <div key={item.label} className="bg-[var(--surface)] px-3 py-2.5">
            <dt className="text-micro text-[var(--text-muted)]">{item.label}</dt>
            <dd className="mt-0.5 text-cell font-medium text-[var(--text-primary)]">{item.value}</dd>
          </div>
        ))}
      </dl>

      <div className="px-4 py-3">
        <h3 className="text-cell font-semibold text-[var(--text-primary)]">매출·산업 구성</h3>
        <p className="mt-0.5 text-micro text-[var(--text-muted)]">
          세그먼트별 매출 비중 (원천 데이터 연동 전 · 추정/데모)
        </p>
        {segmentData.length > 0 ? (
          <div className="mt-3 grid items-center gap-4 sm:grid-cols-[140px_1fr]">
            <div className="mx-auto h-[120px] w-[120px]">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={segmentData}
                    dataKey="value"
                    nameKey="name"
                    innerRadius={36}
                    outerRadius={56}
                    strokeWidth={0}
                    isAnimationActive={false}
                  >
                    {segmentData.map((_, i) => (
                      <Cell key={i} fill={SEGMENT_COLORS[i % SEGMENT_COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip
                    formatter={(v) => [`${Number(v).toFixed(1)}%`, "비중"]}
                    contentStyle={{
                      background: "var(--surface)",
                      border: "1px solid var(--border)",
                      borderRadius: 8,
                      fontSize: 11,
                    }}
                  />
                </PieChart>
              </ResponsiveContainer>
            </div>
            <ul className="space-y-1.5 text-cell text-[var(--text-secondary)]">
              {segmentData.map((seg, i) => (
                <li key={seg.name} className="flex items-center gap-2">
                  <span
                    className="h-2 w-2 shrink-0 rounded-full"
                    style={{ background: SEGMENT_COLORS[i % SEGMENT_COLORS.length] }}
                  />
                  <span className="flex-1">{seg.name}</span>
                  <span className="tabular-nums text-[var(--text-muted)]">{seg.value.toFixed(1)}%</span>
                </li>
              ))}
            </ul>
          </div>
        ) : (
          <p className="mt-3 text-cell text-[var(--text-muted)]">세그먼트 매출 데이터가 없습니다.</p>
        )}
      </div>
    </Card>
  );
}

function buildSegmentPlaceholder(
  stock: StockInfo,
  fundamentals: FundamentalsData | null,
): { name: string; value: number }[] {
  if (fundamentals?.coverageTier === "not_applicable") return [];
  if (stock.securityType?.toUpperCase().includes("ETF")) return [];

  if (stock.symbol === "005930") {
    return [
      { name: "DX (TV·가전·IT)", value: 56.3 },
      { name: "DS (반도체)", value: 39.0 },
      { name: "SDC (디스플레이)", value: 9.0 },
      { name: "Harman (오디오)", value: 4.7 },
    ];
  }

  return [
    { name: "핵심 사업", value: 62 },
    { name: "성장 사업", value: 28 },
    { name: "기타", value: 10 },
  ];
}
