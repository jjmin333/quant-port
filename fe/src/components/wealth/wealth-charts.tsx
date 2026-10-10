"use client";

import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Scatter,
  ScatterChart,
  Tooltip,
  XAxis,
  YAxis,
  ZAxis,
} from "recharts";
import { money, WEALTH_CLASS_COLOR, WEALTH_CLASS_LABEL, type AllocationRow, type IndexPoint } from "@/lib/wealth-api";
import { movingAverage } from "@/lib/re-trades";
import { cn } from "@/lib/utils";

/*
 * 순자산 화면들이 같이 쓰는 차트 (2026-10-01). 색은 자산군 표(WEALTH_CLASS_COLOR) 하나에서만 온다.
 */

const tooltipStyle = {
  background: "var(--surface)",
  border: "1px solid var(--border)",
  borderRadius: 8,
  fontSize: 11,
};
const tick = { fontSize: 11, fill: "var(--text-muted)" };

/** 자산군 비중 — 가로 막대 하나 + 범례. */
export function AllocationBar({ rows, className }: { rows: AllocationRow[]; className?: string }) {
  if (rows.length === 0) return null;
  return (
    <div className={className}>
      <div className="flex h-3 w-full overflow-hidden rounded-full bg-[var(--surface-elevated)]" role="img" aria-label="자산군 비중">
        {rows.map((r) => (
          <span key={r.cls} style={{ width: `${r.pct}%`, background: WEALTH_CLASS_COLOR[r.cls] }} title={`${WEALTH_CLASS_LABEL[r.cls]} ${r.pct.toFixed(1)}%`} />
        ))}
      </div>
      <ul className="mt-2 grid grid-cols-2 gap-x-4 gap-y-1 sm:grid-cols-3">
        {rows.map((r) => (
          <li key={r.cls} className="flex items-baseline gap-1.5 text-cell">
            <span className="size-2.5 shrink-0 translate-y-[1px] rounded-sm" style={{ background: WEALTH_CLASS_COLOR[r.cls] }} />
            <span className="text-[var(--text-secondary)]">{WEALTH_CLASS_LABEL[r.cls]}</span>
            <b className="ml-auto font-num tabular-nums">{r.pct.toFixed(1)}%</b>
            <span className="w-16 text-right font-num text-micro text-[var(--text-muted)] tabular-nums">{money(r.value)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export type LineSpec = { key: string; label: string; color: string; points: IndexPoint[] };

/** 여러 시계열을 날짜로 맞춰 한 차트에. 값 형식은 format이 정한다. */
export function MultiLineChart({
  lines,
  height = 180,
  format = (v: number) => v.toFixed(1),
  className,
}: {
  lines: LineSpec[];
  height?: number;
  format?: (v: number) => string;
  className?: string;
}) {
  const byDate = new Map<string, Record<string, number | string>>();
  for (const l of lines) {
    for (const p of l.points) {
      const row = byDate.get(p.date) ?? { date: p.date };
      row[l.key] = p.value;
      byDate.set(p.date, row);
    }
  }
  const data = [...byDate.values()].sort((a, b) => String(a.date).localeCompare(String(b.date)));
  if (data.length === 0) return <p className="py-6 text-center text-cell text-[var(--text-muted)]">아직 자료가 없습니다.</p>;
  return (
    <div className={cn("w-full", className)}>
      <ResponsiveContainer width="100%" height={height}>
        <LineChart data={data} margin={{ top: 6, right: 8, left: 0, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
          <XAxis dataKey="date" tick={tick} tickFormatter={(d: string) => d.slice(2, 7)} minTickGap={28} />
          <YAxis tick={tick} width={52} domain={["auto", "auto"]} tickFormatter={(v: number) => format(v)} />
          <Tooltip contentStyle={tooltipStyle} formatter={(v, name) => [format(Number(v)), String(name)]} />
          {lines.length > 1 && <Legend wrapperStyle={{ fontSize: 11 }} />}
          {lines.map((l) => (
            <Line key={l.key} type="monotone" dataKey={l.key} name={l.label} stroke={l.color} strokeWidth={1.8}
              dot={false} connectNulls isAnimationActive={false} />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}

export type TradePoint = { t: number; price: number; label: string; kind: "sale" | "jeonse" | "estimate"; direct?: boolean };

/**
 * 실거래 점 — 매매(파랑)·전세(주황)·추정가(큰 점). x축은 날짜 비례(시각 숫자 + time 눈금).
 * 2026-10-03 R1: 매매 3개월 이동평균선(직거래 제외) — 점이 흩어진 단지에서 흐름이 안 읽히던 것.
 */
export function TradeScatter({ points, height = 220 }: { points: TradePoint[]; height?: number }) {
  if (points.length === 0) return <p className="py-6 text-center text-cell text-[var(--text-muted)]">거래가 없습니다.</p>;
  const ma = movingAverage(points.filter((p) => p.kind === "sale" && !p.direct));
  const groups: { kind: TradePoint["kind"]; name: string; color: string }[] = [
    { kind: "sale", name: "매매", color: "#3b6ff5" },
    { kind: "jeonse", name: "전세", color: "#e07b39" },
    { kind: "estimate", name: "추정가", color: "#16a34a" },
  ];
  return (
    <ResponsiveContainer width="100%" height={height}>
      <ScatterChart margin={{ top: 6, right: 8, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
        <XAxis dataKey="t" type="number" domain={["auto", "auto"]} tick={tick} scale="time"
          tickFormatter={(t: number) => new Date(t).toISOString().slice(2, 7)} />
        <YAxis dataKey="price" type="number" tick={tick} width={52} domain={["auto", "auto"]} tickFormatter={(v: number) => money(v)} />
        <ZAxis range={[28, 28]} />
        <Tooltip contentStyle={tooltipStyle} cursor={{ strokeDasharray: "3 3" }}
          formatter={(v, name) => (name === "price" ? [money(Number(v)), "금액"] : [new Date(Number(v)).toISOString().slice(0, 10), "날짜"])} />
        <Legend wrapperStyle={{ fontSize: 11 }} />
        {groups.map((g) => {
          const data = points.filter((p) => p.kind === g.kind);
          return data.length ? <Scatter key={g.kind} name={g.name} data={data} fill={g.color} shape={g.kind === "estimate" ? "diamond" : "circle"} isAnimationActive={false} /> : null;
        })}
        {ma.length >= 2 && (
          <Scatter name="매매 3개월 평균" data={ma} fill="#1e3a8a" line={{ stroke: "#1e3a8a", strokeWidth: 2 }} shape={() => <g />} legendType="line" isAnimationActive={false} />
        )}
      </ScatterChart>
    </ResponsiveContainer>
  );
}

/** 국고채 수익률 곡선. */
export function CurveChart({ curve, height = 180 }: { curve: { years: number; value: number }[]; height?: number }) {
  const data = [...curve].sort((a, b) => a.years - b.years).map((c) => ({ x: `${c.years}년`, value: c.value }));
  if (data.length === 0) return <p className="py-6 text-center text-cell text-[var(--text-muted)]">아직 자료가 없습니다.</p>;
  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={data} margin={{ top: 6, right: 8, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
        <XAxis dataKey="x" tick={tick} />
        <YAxis tick={tick} width={44} domain={["auto", "auto"]} tickFormatter={(v: number) => `${v.toFixed(2)}`} unit="%" />
        <Tooltip contentStyle={tooltipStyle} formatter={(v) => [`${Number(v).toFixed(3)}%`, "국고채"]} />
        <Line type="monotone" dataKey="value" stroke="#3b6ff5" strokeWidth={2} dot={{ r: 3 }} isAnimationActive={false} />
      </LineChart>
    </ResponsiveContainer>
  );
}
