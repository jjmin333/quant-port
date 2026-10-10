"use client";

import { useState } from "react";
import { Card } from "@/components/ui/card";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { MultiLineChart } from "@/components/wealth/wealth-charts";
import { money, type RegionMarket } from "@/lib/wealth-api";

/*
 * 지역 시장 — 한국부동산원 R-ONE 지수 + 우리가 받은 실거래 월별 집계 (2026-10-01).
 * 지수는 같은 단위끼리 묶어 보인다: 주간(매매·전세) / 월간(실거래가격지수·매매·월세) / 전월세 전환율 / 상업용.
 */

const GROUPS: { id: string; label: string; keys: string[]; colors: string[] }[] = [
  { id: "weekly", label: "주간 지수", keys: ["apt_sale_weekly", "apt_jeonse_weekly"], colors: ["#3b6ff5", "#e07b39"] },
  { id: "monthly", label: "월간 지수", keys: ["apt_real_trade", "apt_sale_monthly", "apt_rent_monthly", "offi_sale"], colors: ["#16a34a", "#3b6ff5", "#c25b8a", "#7a5af5"] },
  { id: "conv", label: "전월세 전환율", keys: ["apt_conversion"], colors: ["#e07b39"] },
  { id: "commercial", label: "상업용", keys: ["office_rent", "retail_rent", "office_vacancy", "retail_vacancy"], colors: ["#3b6ff5", "#16a34a", "#e07b39", "#c25b8a"] },
];

export function RegionMarketCard({ market }: { market: RegionMarket }) {
  const available = GROUPS.filter((g) => market.indices.some((s) => g.keys.includes(s.key)));
  const [group, setGroup] = useState(available[0]?.id ?? "weekly");
  const g = available.find((x) => x.id === group) ?? available[0];
  const [years, setYears] = useState<"1" | "3" | "5">("3");
  // 자르는 기준은 자료의 마지막 날 — 렌더 중에 지금 시각을 읽지 않는다.
  const lastDate = market.indices.reduce((m, s) => (s.points.length && s.points[s.points.length - 1].date > m ? s.points[s.points.length - 1].date : m), "");
  const cut = lastDate ? `${Number(lastDate.slice(0, 4)) - Number(years)}${lastDate.slice(4)}` : "";
  const lines = !g ? [] : market.indices
    .filter((s) => g.keys.includes(s.key))
    .map((s) => ({ key: s.key, label: s.label, color: g.colors[g.keys.indexOf(s.key)], points: s.points.filter((p) => p.date >= cut) }));
  const latest = (key: string) => {
    const s = market.indices.find((x) => x.key === key);
    if (!s || s.points.length < 2) return null;
    const last = s.points[s.points.length - 1];
    const prev = s.points[s.points.length - 2];
    return { last, change: last.value - prev.value, unit: s.unit };
  };
  const wk = latest("apt_sale_weekly");
  const months = market.months ?? [];
  const recentMonths = months.slice(-6);

  return (
    <Card className="p-3">
      <div className="flex flex-wrap items-baseline gap-2">
        <h2 className="text-section font-semibold">{market.name} 시장</h2>
        {wk && (
          <span className="text-micro text-[var(--text-muted)]">
            주간 매매지수 {wk.last.value.toFixed(2)} ({wk.change >= 0 ? "+" : ""}{wk.change.toFixed(2)}, {wk.last.date})
          </span>
        )}
      </div>
      {available.length === 0 ? (
        <p className="mt-2 text-cell text-[var(--text-muted)]">이 지역 지수를 아직 받지 못했습니다.</p>
      ) : (
        <>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <MiniToggle options={available.map((x) => ({ id: x.id, label: x.label }))} value={g?.id ?? null} onChange={setGroup} />
            <MiniToggle options={[{ id: "1", label: "1년" }, { id: "3", label: "3년" }, { id: "5", label: "5년" }]} value={years} onChange={setYears} />
          </div>
          <MultiLineChart lines={lines} height={190} format={(v) => v.toFixed(1)} />
        </>
      )}
      {recentMonths.length > 0 && (
        <table className="mt-2 w-full text-micro">
          <thead className="text-[var(--text-muted)]">
            <tr><th className="text-left">아파트 실거래</th>{recentMonths.map((m) => <th key={m.ym} className="text-right">{Number(m.ym.slice(4))}월</th>)}</tr>
          </thead>
          <tbody>
            <tr><td className="text-[var(--text-muted)]">매매(건)</td>{recentMonths.map((m) => <td key={m.ym} className="text-right font-num tabular-nums">{m.count}</td>)}</tr>
            <tr><td className="text-[var(--text-muted)]">평균 평당가</td>{recentMonths.map((m) => <td key={m.ym} className="text-right font-num tabular-nums">{m.avgPerM2 ? money(m.avgPerM2 * 3.3058) : "-"}</td>)}</tr>
            <tr><td className="text-[var(--text-muted)]">전월세(건)</td>{recentMonths.map((m) => <td key={m.ym} className="text-right font-num tabular-nums">{m.rentCount}</td>)}</tr>
          </tbody>
        </table>
      )}
      <p className="mt-2 text-micro text-[var(--text-muted)]">
        출처 {market.source} · 최근 두 달은 신고 기한(30일) 중 · 상업용은 시도 기준
      </p>
    </Card>
  );
}
