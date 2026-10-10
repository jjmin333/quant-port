"use client";

import { useMemo, useState } from "react";
import { Card } from "@/components/ui/card";
import { formatKRW } from "@/lib/format";
import type { HoldingMetrics, HoldingsTotals } from "@/lib/holdings-calc";
import { cn } from "@/lib/utils";

/**
 * 구성 도넛 — 종목별 / 섹터별 / 통화별 / 자산별.
 *
 * 숫자 목록("비중 상위 5")만으로는 한 종목에 몰려 있는지가 안 보인다. 원은 그걸 즉시
 * 말한다. 종목이 여섯을 넘으면 상위 6개만 색을 주고 나머지는 "기타"로 묶는다 —
 * 스무 조각짜리 도넛은 색을 구분할 수 없어 정보가 아니라 무늬가 된다.
 *
 * 섹터는 서버가 준다(`stock_sectors` — DART KSIC · SEC SIC를 공통 11섹터로, 2026-09-23).
 * 모르는 종목(ETF · 비금융 지주 · 아직 안 받은 종목)은 「미분류」 조각으로 **그대로 보인다** —
 * 빼고 그리면 나머지 섹터의 비중이 실제보다 커 보인다.
 *
 * recharts를 쓰지 않고 SVG로 직접 그린 이유: 조각 6개짜리 도넛에 차트 라이브러리를
 * 얹으면 번들만 커지고, `stroke-dasharray` 여섯 줄이면 끝나는 일이다.
 */

/**
 * 색 다섯 개까지만 이름을 준다. 여섯 번째부터는 전부 "기타" 한 조각으로 묶고 회색을
 * 쓴다 — 회색이 **기타에만** 쓰여야 "이건 묶인 것"이라는 뜻이 산다. 예전에 6번째 종목과
 * 기타가 둘 다 회색이라 도넛에 구분되지 않는 회색 두 조각이 붙어 있었다.
 */
const SLICE_COLORS = [
  "var(--chart-1)",
  "var(--chart-2)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
];
const REST_COLOR = "var(--muted-foreground)";
/**
 * 「미분류」는 기타와 다른 뜻이다(묶인 것이 아니라 모르는 것). 같은 회색 두 조각이
 * 붙지 않도록 같은 색을 **흐리게** 쓴다 — 도넛과 범례가 같은 불투명도를 쓴다.
 */
const UNKNOWN_OPACITY = 0.4;
const UNKNOWN_KEY = "__unknown";
const NAMED_SLICES = SLICE_COLORS.length;

const R = 42;
const CIRCUMFERENCE = 2 * Math.PI * R;

type Slice = {
  key: string;
  label: string;
  valueKRW: number;
  pct: number;
  color: string;
  /** 도넛에서 이 조각이 시작하는 지점(원둘레 단위). 렌더 중에 누적하지 않고 미리 계산한다. */
  offset: number;
  opacity?: number;
};

type Mode = "symbol" | "sector" | "currency" | "asset";

export function HoldingsAllocationCard({
  metrics,
  totals,
}: {
  metrics: HoldingMetrics[];
  totals: HoldingsTotals | null;
}) {
  const [mode, setMode] = useState<Mode>("symbol");

  const slices = useMemo<Slice[]>(() => {
    const withOffsets = (list: Array<Omit<Slice, "offset">>): Slice[] => {
      let running = 0;
      return list.map((s) => {
        const slice = { ...s, offset: running };
        running += (s.pct / 100) * CIRCUMFERENCE;
        return slice;
      });
    };

    if (!totals?.canTotalAssets || totals.totalAssetsKRW <= 0) return [];

    const denom = totals.totalAssetsKRW;

    if (mode === "asset") {
      const rows: Array<[string, string, number, string]> = [
        ["stock", "주식", totals.canTotal ? totals.marketValueKRW : 0, SLICE_COLORS[0]!],
        ["cash", "현금", totals.cashTotalKRW, SLICE_COLORS[2]!],
      ];
      return withOffsets(
        rows
          .filter(([, , v]) => v > 0)
          .map(([key, label, valueKRW, color]) => ({
            key,
            label,
            valueKRW,
            pct: (valueKRW / denom) * 100,
            color,
          })),
      );
    }

    if (mode === "currency") {
      const usdCashKRW = Math.max(0, totals.cashTotalKRW - totals.cashKRW);
      const rows: Array<[string, string, number]> = [
        ["krw", `국내 ${totals.krwCount}종목`, totals.krwValueKRW + totals.cashKRW],
        ["usd", `해외 ${totals.usdCount}종목`, totals.usdValueKRW + usdCashKRW],
      ];
      return withOffsets(
        rows
          .filter(([, , v]) => v > 0)
          .map(([key, label, valueKRW]) => ({
            key,
            label,
            valueKRW,
            pct: (valueKRW / denom) * 100,
            color: key === "krw" ? SLICE_COLORS[0] : SLICE_COLORS[3],
          })),
      );
    }

    const stockDenom = totals.canTotal ? totals.marketValueKRW : 0;
    if (stockDenom <= 0) return [];

    if (mode === "sector") {
      /*
       * 비중의 분모는 다른 탭과 같은 **총 자산**이다. 탭마다 분모가 다르면 「IT 40%」와
       * 「삼성전자 25%」를 나란히 읽을 수 없다. 그래서 현금은 조각 없이 합계에서만 빠진다.
       */
      const bySector = new Map<string, { label: string; value: number }>();
      let unknownValue = 0;
      for (const m of metrics) {
        const v = m.marketValueKRW ?? 0;
        if (v <= 0) continue;
        const key = m.row.sector;
        if (!key) {
          unknownValue += v;
          continue;
        }
        const cur = bySector.get(key) ?? { label: m.row.sectorLabel || key, value: 0 };
        cur.value += v;
        bySector.set(key, cur);
      }
      const ranked = [...bySector.entries()].sort((a, b) => b[1].value - a[1].value);
      const named = ranked.slice(0, NAMED_SLICES);
      const rest = ranked.slice(NAMED_SLICES);
      const out: Array<Omit<Slice, "offset">> = named.map(([key, v], i) => ({
        key,
        label: v.label,
        valueKRW: v.value,
        pct: (v.value / denom) * 100,
        color: SLICE_COLORS[i]!,
      }));
      if (rest.length > 0) {
        const restValue = rest.reduce((sum, [, v]) => sum + v.value, 0);
        out.push({
          key: "__rest",
          label: `기타 ${rest.length}섹터`,
          valueKRW: restValue,
          pct: (restValue / denom) * 100,
          color: REST_COLOR,
        });
      }
      if (unknownValue > 0) {
        out.push({
          key: UNKNOWN_KEY,
          label: "미분류",
          valueKRW: unknownValue,
          pct: (unknownValue / denom) * 100,
          color: REST_COLOR,
          opacity: UNKNOWN_OPACITY,
        });
      }
      return withOffsets(out);
    }

    const priced = metrics
      .filter((m) => m.marketValueKRW != null && m.marketValueKRW > 0)
      .sort((a, b) => (b.marketValueKRW ?? 0) - (a.marketValueKRW ?? 0));

    const top = priced.slice(0, NAMED_SLICES);
    const rest = priced.slice(NAMED_SLICES);
    const out: Array<Omit<Slice, "offset">> = top.map((m, i) => ({
      key: m.symbol,
      label: m.name,
      valueKRW: m.marketValueKRW ?? 0,
      pct: ((m.marketValueKRW ?? 0) / denom) * 100,
      color: SLICE_COLORS[i],
    }));
    if (rest.length > 0) {
      const restValue = rest.reduce((sum, m) => sum + (m.marketValueKRW ?? 0), 0);
      out.push({
        key: "__rest",
        label: `기타 ${rest.length}종목`,
        valueKRW: restValue,
        pct: (restValue / denom) * 100,
        color: REST_COLOR,
      });
    }
    return withOffsets(out);
  }, [metrics, totals, mode]);

  const headline = useMemo(() => {
    if (slices.length === 0) return null;
    if (mode === "asset") {
      const stock = slices.find((s) => s.key === "stock");
      return { caption: "주식 비중", value: `${Math.round(stock?.pct ?? 0)}%` };
    }
    if (mode === "currency") {
      const krw = slices.find((s) => s.key === "krw");
      return { caption: "원화 비중", value: `${Math.round(krw?.pct ?? 0)}%` };
    }
    if (mode === "sector") {
      const n = slices.filter((s) => s.key !== UNKNOWN_KEY && s.key !== "__rest").length;
      return { caption: "섹터", value: String(n) };
    }
    return { caption: "종목", value: String(metrics.filter((m) => m.hasQuote).length) };
  }, [slices, mode, metrics]);

  const top3 = useMemo(
    () => slices.filter((s) => s.key !== "__rest").slice(0, 3).reduce((sum, s) => sum + s.pct, 0),
    [slices],
  );

  if (!totals?.canTotalAssets || slices.length === 0) {
    return (
      <Card className="p-3">
        <p className="mb-2 text-cell font-semibold">구성</p>
        <p className="py-8 text-center text-cell text-muted-foreground">
          {totals && !totals.canTotalAssets
            ? "환율을 불러오는 중이라 비중을 계산하지 못했습니다"
            : "표시할 자산이 없습니다"}
        </p>
      </Card>
    );
  }

  return (
    <Card className="p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <p className="text-cell font-semibold">구성</p>
        <div className="flex items-center gap-0.5 rounded-md bg-muted p-0.5">
          {(
            [
              ["symbol", "종목"],
              ["sector", "섹터"],
              ["currency", "통화"],
              ["asset", "자산"],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              type="button"
              onClick={() => setMode(value)}
              aria-pressed={mode === value}
              className={cn(
                "rounded px-2 py-0.5 text-cell transition-colors",
                mode === value
                  ? "bg-card font-semibold text-foreground shadow-xs"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      <div className="flex min-w-0 flex-wrap items-center gap-4">
        <svg viewBox="0 0 100 100" className="size-[132px] shrink-0" role="img" aria-label="구성 비중">
          <g transform="rotate(-90 50 50)" fill="none" strokeWidth="14">
            {slices.map((s) => (
              <circle
                key={s.key}
                cx="50"
                cy="50"
                r={R}
                stroke={s.color}
                strokeOpacity={s.opacity}
                strokeDasharray={`${((s.pct / 100) * CIRCUMFERENCE).toFixed(2)} ${CIRCUMFERENCE.toFixed(2)}`}
                strokeDashoffset={(-s.offset).toFixed(2)}
              />
            ))}
          </g>
          {headline && (
            <>
              <text x="50" y="47" textAnchor="middle" fontSize="7" fill="var(--muted-foreground)">
                {headline.caption}
              </text>
              <text
                x="50"
                y="59"
                textAnchor="middle"
                fontSize="14"
                fontWeight="700"
                fill="var(--foreground)"
              >
                {headline.value}
              </text>
            </>
          )}
        </svg>

        <ul className="min-w-0 flex-1 basis-40">
          {slices.map((s) => (
            <li key={s.key} className="flex min-w-0 items-center gap-2 py-0.5 text-cell">
              <span
                className="size-2 shrink-0 rounded-[2px]"
                style={{ backgroundColor: s.color, opacity: s.opacity }}
                aria-hidden
              />
              <span className="min-w-0 flex-1 truncate">{s.label}</span>
              <b className="shrink-0 font-num tabular-nums">{s.pct.toFixed(1)}%</b>
            </li>
          ))}
        </ul>
      </div>

      <div className="mt-2 border-t pt-1.5">
        {mode === "symbol" ? (
          <>
            <Row label="가장 큰 종목" value={`${slices[0]?.label ?? "—"} ${slices[0]?.pct.toFixed(1) ?? "—"}%`} />
            <Row label="상위 3종목 합" value={`${top3.toFixed(1)}%`} />
          </>
        ) : mode === "sector" ? (
          <SectorFooter slices={slices} />
        ) : mode === "asset" ? (
          <>
            <Row
              label="주식 평가금액"
              value={totals.canTotal ? formatKRW(totals.marketValueKRW) : "—"}
            />
            <Row label="현금" value={totals.cashTotalKRW > 0 ? formatKRW(totals.cashTotalKRW) : "—"} />
            <Row label="총 자산" value={formatKRW(totals.totalAssetsKRW)} />
          </>
        ) : (
          <>
            <Row label="국내 평가금액" value={formatKRW(totals.krwValueKRW)} />
            <Row
              label="해외 평가금액"
              value={totals.usdValueKRW > 0 ? formatKRW(totals.usdValueKRW) : "—"}
            />
            {/* 환율이 1% 흔들리면 원화 평가금액이 얼마나 달라지는지 — 해외 비중이
                큰 포트폴리오에서 이건 종목 등락만큼 큰 숫자다. */}
            <Row
              label="환율 1% 변동 시"
              value={totals.usdValueKRW > 0 ? `±${formatKRW(Math.round(totals.usdValueKRW * 0.01))}` : "—"}
            />
          </>
        )}
      </div>
    </Card>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex min-w-0 items-center justify-between gap-2 py-0.5">
      <span className="shrink-0 text-cell text-muted-foreground">{label}</span>
      <b className="min-w-0 truncate font-num text-cell tabular-nums">{value}</b>
    </div>
  );
}

function SectorFooter({ slices }: { slices: Slice[] }) {
  const top = slices.find((s) => s.key !== UNKNOWN_KEY && s.key !== "__rest");
  const unknown = slices.find((s) => s.key === UNKNOWN_KEY);
  return (
    <>
      <Row label="가장 큰 섹터" value={top ? `${top.label} ${top.pct.toFixed(1)}%` : "—"} />
      {unknown && (
        // 미분류가 왜 있는지 한 줄로 — 빈칸을 고장으로 읽지 않게.
        <Row label="미분류 (ETF·지주 등)" value={`${unknown.pct.toFixed(1)}%`} />
      )}
    </>
  );
}
