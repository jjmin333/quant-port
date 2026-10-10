"use client";

import { HelpHint } from "@/components/ui/help-hint";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Sparkles } from "lucide-react";
import { AssetBriefBody } from "@/components/dashboard/asset-pulse-cards";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { Card } from "@/components/ui/card";
import { AINoteLine } from "@/components/wealth/asset-intel-ui";
import { CurveChart, MultiLineChart } from "@/components/wealth/wealth-charts";
import { rateDiff } from "@/components/holdings/holdings-wealth-section";
import { spreadByMonth } from "@/lib/rate-spread";
import { fetchRates, fetchWealth, type RateItem, type RatesData, type WealthData } from "@/lib/wealth-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 「금리」 (2026-10-01) — 한국은행 ECOS. 예금·채권 평가의 할인율과 대출 비교의 기준이 같은 표에서 온다.
 * 2026-10-03 F4: 맨 위 금리 브리핑(홈과 같은 글) · 장단기 차·예대금리차 추이 · 「내 금리 vs 시장 평균」(로그인).
 */
const GROUPS: { title: string; keys: string[] }[] = [
  { title: "정책·단기", keys: ["base", "call", "kofr", "cd91", "cp91"] },
  { title: "국고채", keys: ["ktb1", "ktb2", "ktb3", "ktb5", "ktb10", "ktb20", "ktb30"] },
  { title: "회사채(3년)", keys: ["corpAA", "corpBBB"] },
  { title: "예금은행 신규 평균", keys: ["deposit1y", "saving", "mortgage", "mortgageFixed", "mortgageVar", "jeonseLoan", "creditLoan"] },
];

export function RatesClient() {
  const [data, setData] = useState<RatesData | null>(null);
  useEffect(() => {
    let alive = true;
    fetchRates().then((d) => alive && setData(d)).catch(() => alive && setData({ items: [], curve: [], trend: {}, source: "" }));
    return () => { alive = false; };
  }, []);
  const byKey = new Map((data?.items ?? []).map((i) => [i.key, i]));
  const ktb3 = byKey.get("ktb3")?.value;
  const aa = byKey.get("corpAA")?.value;

  return (
    <PageScrollShell>
      {data?.brief ? (
        <Card className="mb-3 p-3">
          <h2 className="mb-2 flex items-center gap-1.5 text-section font-semibold"><Sparkles className="size-3.5 text-[var(--brand)]" />금리 브리핑</h2>
          <AssetBriefBody brief={data.brief} stamp={data.brief.updatedAt ? `${data.brief.updatedAt.slice(5, 10).replace("-", "/")} 기준` : "금리"} />
        </Card>
      ) : (
        <AINoteLine note={data?.note} className="mb-3" />
      )}
      <MyRatesCard byKey={byKey} />
      <div className="grid gap-3 xl:grid-cols-2 [&>*]:min-w-0">
        {GROUPS.map((g) => (
          <Card key={g.title} className="p-0">
            <h2 className="border-b px-3 py-2 text-section font-semibold">{g.title}</h2>
            <ul>
              {g.keys.map((k) => {
                const i = byKey.get(k);
                if (!i) return null;
                const d = i.prev != null ? i.value - i.prev : null;
                return (
                  <li key={k} className="flex items-baseline gap-2 border-b px-3 py-1.5 text-cell last:border-b-0">
                    <span className="text-[var(--text-secondary)]">{i.label}</span>
                    <span className="text-micro text-[var(--text-muted)]">{i.cycle === "M" ? i.date.slice(0, 7) : i.date}</span>
                    <b className="ml-auto font-num tabular-nums">{i.value.toFixed(2)}%</b>
                    <span className={cn("w-14 text-right font-num text-micro tabular-nums", d != null && trendTextClass(d))}>
                      {d != null ? rateDiff(d) : ""}
                    </span>
                  </li>
                );
              })}
            </ul>
          </Card>
        ))}
        <Card className="p-3">
          <h2 className="text-section font-semibold">국고채 수익률 곡선</h2>
          <CurveChart curve={data?.curve ?? []} />
          {ktb3 != null && aa != null && (
            <p className="mt-1 text-micro text-[var(--text-muted)]">신용 스프레드(회사채 AA- − 국고채 3년) {(aa - ktb3).toFixed(2)}%p</p>
          )}
        </Card>
        <Card className="p-3">
          <h2 className="flex items-center gap-1.5 text-section font-semibold">금리 차 추이 <span className="text-micro font-normal text-[var(--text-muted)]">%p · 월말</span>
            <HelpHint>장단기 차가 줄거나 음수면 시장이 앞으로 금리 하락을 본다는 뜻으로 읽힙니다. 예대금리차는 은행이 예금보다 대출에 얼마나 더 받는지입니다.</HelpHint>
          </h2>
          <MultiLineChart format={(v) => `${v.toFixed(2)}%p`} lines={[
            { key: "term", label: "장단기 차 (국고채 10년 − 3년)", color: "#3b6ff5", points: spreadByMonth(data?.trend.ktb10 ?? [], data?.trend.ktb3 ?? []) },
            { key: "loanDep", label: "예대금리차 (주담대 − 정기예금)", color: "#e07b39", points: spreadByMonth(data?.trend.mortgage ?? [], data?.trend.deposit1y ?? []) },
          ]} />
        </Card>
        <Card className="p-3">
          <h2 className="text-section font-semibold">5년 추이</h2>
          <MultiLineChart format={(v) => `${v.toFixed(2)}%`} lines={[
            { key: "base", label: "기준금리", color: "#111827", points: data?.trend.base ?? [] },
            { key: "ktb3", label: "국고채 3년", color: "#3b6ff5", points: data?.trend.ktb3 ?? [] },
            { key: "deposit1y", label: "정기예금 1년", color: "#16a34a", points: data?.trend.deposit1y ?? [] },
            { key: "mortgage", label: "주담대", color: "#e07b39", points: data?.trend.mortgage ?? [] },
          ]} />
        </Card>
      </div>
      <p className="mt-3 text-micro text-[var(--text-muted)]">
        출처 {data?.source || "한국은행 경제통계시스템(ECOS)"}. 예금은행 평균금리는 전달 신규취급액 기준(월 1회). 개별 상품 금리는{" "}
        <Link href="/finance/savings" className="text-[var(--brand)] hover:underline">예·적금 분석</Link> ·{" "}
        <Link href="/finance/loans" className="text-[var(--brand)] hover:underline">대출 분석</Link>.
      </p>
    </PageScrollShell>
  );
}

/**
 * 「내 금리 vs 시장 평균」(F4) — 내 포트폴리오의 예·적금(금리)과 대출을 같은 종류의 예금은행 신규 평균과 견준다.
 * 비로그인이거나 금리를 넣은 것이 없으면 그리지 않는다.
 */
function MyRatesCard({ byKey }: { byKey: Map<string, RateItem> }) {
  const { session } = useAuth();
  const [w, setW] = useState<WealthData | null>(null);
  useEffect(() => {
    if (!session) return;
    let alive = true;
    fetchWealth().then((d) => alive && setW(d)).catch(() => {});
    return () => { alive = false; };
  }, [session]);
  if (!session || !w) return null;
  type Line = { name: string; mine: number; market?: RateItem; good: "lower" | "higher" };
  const lines: Line[] = [];
  for (const a of w.assets) {
    const r = Number(a.detail?.rate);
    if ((a.assetType === "deposit" || a.assetType === "saving") && r > 0) {
      lines.push({ name: a.name, mine: r, market: byKey.get(a.assetType === "deposit" ? "deposit1y" : "saving"), good: "higher" });
    }
  }
  for (const l of w.liabilities) {
    if (l.rate == null || l.balance <= 0 || l.kind === "jeonse_deposit") continue;
    lines.push({ name: l.name || "대출", mine: l.rate, market: byKey.get(l.kind === "mortgage" ? "mortgage" : l.kind === "credit" ? "creditLoan" : ""), good: "lower" });
  }
  if (lines.length === 0) return null;
  return (
    <Card className="mb-3 p-0">
      <h2 className="border-b px-3 py-2 text-section font-semibold">내 금리 vs 시장 평균 <span className="text-micro font-normal text-[var(--text-muted)]">예금은행 신규 취급 평균(전달)</span></h2>
      <ul>
        {lines.map((x, i) => {
          const d = x.market ? x.mine - x.market.value : null;
          const better = d != null && (x.good === "higher" ? d > 0 : d < 0);
          return (
            <li key={i} className="flex flex-wrap items-baseline gap-2 border-b px-3 py-1.5 text-cell last:border-b-0">
              <span className="min-w-0 flex-1 truncate">{x.name}</span>
              <b className="font-num tabular-nums">{x.mine.toFixed(2)}%</b>
              {x.market && d != null ? (
                <span className="text-micro text-[var(--text-muted)]">
                  {x.market.label} {x.market.value.toFixed(2)}% 대비{" "}
                  <b className={cn("font-num tabular-nums", better ? "text-[var(--score-high)]" : "text-[var(--profit)]")}>
                    {d > 0 ? "+" : ""}{d.toFixed(2)}%p {Math.abs(d) < 0.005 ? "" : better ? "유리" : "불리"}
                  </b>
                </span>
              ) : (
                <span className="text-micro text-[var(--text-muted)]">비교할 평균 없음</span>
              )}
            </li>
          );
        })}
      </ul>
    </Card>
  );
}
