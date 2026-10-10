"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import { GitCompareArrows, X } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Pct, ScoreBadge } from "@/components/wealth/asset-intel-ui";
import { MultiLineChart } from "@/components/wealth/wealth-charts";
import { movingAverage } from "@/lib/re-trades";
import { fetchComplex, money, type ComplexData } from "@/lib/wealth-api";
import { cn } from "@/lib/utils";

/**
 * 단지 비교 (2026-10-03 사용자 결정 R2 — 아실·호갱노노의 「여러 단지 겹쳐보기」).
 * 담은 단지는 주소 `?cmp=시군구코드:단지키`(최대 4)에 실려 공유·새로고침이 된다. 다른 화면으로 옮겨도 유지.
 * 선은 단지마다 평당가(매매, 직거래·해제 제외)의 3개월 이동평균 — 평형이 달라도 같은 잣대.
 */
export const COMPARE_MAX = 4;
const COLORS = ["#3b6ff5", "#e07b39", "#16a34a", "#a855f7"];

export type CompareRef = { lawd: string; key: string };

/** `cmp` 값 하나 — 시군구코드(5자리) + ":" + 단지 키. 형식이 틀리면 버린다. */
export function parseCompare(values: string[]): CompareRef[] {
  const out: CompareRef[] = [];
  for (const v of values) {
    const m = /^(\d{5}):(.{1,120})$/.exec(v);
    if (m && !out.some((o) => o.key === m[2])) out.push({ lawd: m[1], key: m[2] });
  }
  return out.slice(0, COMPARE_MAX);
}

export function useCompare() {
  const params = useSearchParams();
  const router = useRouter();
  const list = parseCompare(params.getAll("cmp"));
  const write = (next: CompareRef[]) => {
    const p = new URLSearchParams(params.toString());
    p.delete("cmp");
    for (const c of next) p.append("cmp", `${c.lawd}:${c.key}`);
    router.replace(`/real-estate?${p}`, { scroll: false });
  };
  const has = (key: string) => list.some((c) => c.key === key);
  const toggle = (ref: CompareRef) => write(has(ref.key) ? list.filter((c) => c.key !== ref.key) : [...list, ref].slice(-COMPARE_MAX));
  return { list, has, toggle, clear: () => write([]), remove: (key: string) => write(list.filter((c) => c.key !== key)) };
}

export function CompareButton({ lawd, complexKey }: { lawd: string; complexKey: string }) {
  const c = useCompare();
  const on = c.has(complexKey);
  return (
    <button
      type="button"
      onClick={() => c.toggle({ lawd, key: complexKey })}
      aria-pressed={on}
      title={on ? "비교에서 빼기" : `비교에 담기(최대 ${COMPARE_MAX})`}
      className={cn("inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-micro hover:bg-muted", on && "border-[var(--brand)]/50 text-[var(--brand)]")}
    >
      <GitCompareArrows className="size-3.5" />
      {on ? `비교 중 (${c.list.length})` : "비교에 담기"}
    </button>
  );
}

type Loaded = { ref: CompareRef; data: ComplexData | null };

/** 평당가 3개월 이동평균 — 직거래·해제는 뺀다. */
function pyeongSeries(d: ComplexData) {
  const pts = d.sales
    .filter((t) => !t.cancelled && !t.direct && t.price && t.area > 0)
    .map((t) => ({ t: Date.parse(t.dealDate), price: (t.price! / t.area) * 3.3058 }));
  return movingAverage(pts).map((p) => ({ date: new Date(p.t).toISOString().slice(0, 10), value: p.price }));
}

export function ComplexCompareCard({ regionName }: { regionName: (lawd: string) => string }) {
  const c = useCompare();
  const [loaded, setLoaded] = useState<Loaded[]>([]);
  const listKey = JSON.stringify(c.list);

  useEffect(() => {
    let alive = true;
    const refs = JSON.parse(listKey) as CompareRef[];
    Promise.all(refs.map((ref) => fetchComplex(ref.key, ref.lawd).then((data) => ({ ref, data })).catch(() => ({ ref, data: null }))))
      .then((rows) => alive && setLoaded(rows));
    return () => { alive = false; };
  }, [listKey]);

  if (c.list.length === 0) return null;
  const rows = loaded.filter((l) => c.has(l.ref.key));
  const name = (l: Loaded) => l.data?.sales[0]?.complexName || l.data?.rents[0]?.complexName || l.ref.key.split("|").pop() || "단지";

  return (
    <Card className="mb-3 p-3">
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="text-section font-semibold">단지 비교</h2>
        <span className="text-micro text-[var(--text-muted)]">{c.list.length}/{COMPARE_MAX} · 평당가 3개월 이동평균(매매)</span>
        <button type="button" onClick={c.clear} className="ml-auto text-micro text-[var(--text-muted)] hover:underline">모두 빼기</button>
      </div>
      {c.list.length < 2 ? (
        <p className="mt-2 text-cell text-[var(--text-muted)]">단지 화면에서 「비교에 담기」를 하나 더 누르면 겹쳐 봅니다.</p>
      ) : (
        <MultiLineChart
          className="mt-2"
          height={200}
          format={(v) => money(v)}
          lines={rows.map((l, i) => ({ key: `c${i}`, label: name(l), color: COLORS[i % COLORS.length], points: l.data ? pyeongSeries(l.data) : [] }))}
        />
      )}
      <div className="mt-2 overflow-x-auto">
        <table className="w-full min-w-[560px] text-cell">
          <thead className="text-micro text-[var(--text-muted)]">
            <tr className="border-b">
              <th className="py-1 text-left">단지</th>
              <th className="text-right">평당가</th>
              <th className="text-right" title="최근 3개월 ㎡당 중앙가 vs 그 전 9개월">3개월 변화</th>
              <th className="text-right">6개월 거래</th>
              <th className="text-right">전세가율</th>
              <th className="text-right" title="최근 3개월 ÷ 12개월 월별 최고">고점 대비</th>
              <th className="text-right">점수</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {rows.map((l, i) => {
              const f = (l.data?.score?.facts ?? {}) as Record<string, number | string>;
              return (
                <tr key={l.ref.key} className="border-b last:border-b-0">
                  <td className="py-1">
                    <span className="mr-1.5 inline-block size-2.5 rounded-sm align-middle" style={{ background: COLORS[i % COLORS.length] }} />
                    <b>{name(l)}</b> <span className="text-micro text-[var(--text-muted)]">{regionName(l.ref.lawd)} {String(f.umdNm ?? "")}</span>
                  </td>
                  <td className="text-right font-num tabular-nums">{f.pyeongPrice ? money(Number(f.pyeongPrice)) : "-"}</td>
                  <td className="text-right">{f.momentumPct != null ? <Pct v={Number(f.momentumPct)} /> : "-"}</td>
                  <td className="text-right font-num tabular-nums">{f.trades6m != null ? `${f.trades6m}건` : "-"}</td>
                  <td className="text-right font-num tabular-nums">{Number(f.jeonseRatio) > 0 ? `${Number(f.jeonseRatio).toFixed(0)}%` : "-"}</td>
                  <td className="text-right font-num tabular-nums">{f.peakPct != null ? `${Number(f.peakPct).toFixed(0)}%` : "-"}</td>
                  <td className="text-right"><ScoreBadge score={l.data?.score} kind="complex" /></td>
                  <td className="pl-2 text-right">
                    <button type="button" onClick={() => c.remove(l.ref.key)} aria-label={`${name(l)} 비교에서 빼기`} className="text-[var(--text-muted)] hover:text-foreground"><X className="size-3.5" /></button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {rows.some((l) => !l.data?.score) && <p className="mt-1 text-micro text-[var(--text-muted)]">점수·지표는 최근 12개월 매매 5건 이상인 단지만 있습니다.</p>}
      </div>
    </Card>
  );
}
