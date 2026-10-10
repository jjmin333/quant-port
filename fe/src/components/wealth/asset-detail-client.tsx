"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, Pencil } from "lucide-react";
import { LoadErrorState } from "@/components/common/load-error-state";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { MultiLineChart, TradeScatter, type TradePoint } from "@/components/wealth/wealth-charts";
import { RegionMarketCard } from "@/components/wealth/region-market-card";
import { AssetFormDialog } from "@/components/wealth/wealth-forms";
import {
  ASSET_TYPE_LABEL,
  CONFIDENCE_LABEL,
  fetchAssetDetail,
  money,
  PROPERTY_TYPE_LABEL,
  signedPct,
  VALUATION_KIND_LABEL,
  type AssetDetailData,
  type ReEstimate,
  type ReTrade,
} from "@/lib/wealth-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 자산 한 개 — 평가의 근거를 그대로 보인다 (2026-10-01, 연구 문서 §6·§12).
 * 부동산은 「추정가 · 최근 실거래 · 최근 3개월 평균 · 동일 면적 중앙값 · 전세 시세」를 **이름을 달리해** 나란히 둔다.
 */
export function AssetDetailClient({ id }: { id: number }) {
  const [data, setData] = useState<AssetDetailData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [reload, setReload] = useState(0);
  const [edit, setEdit] = useState(false);

  useEffect(() => {
    let alive = true;
    fetchAssetDetail(id)
      .then((d) => alive && (setData(d), setError(null)))
      .catch((e: unknown) => alive && setError(e instanceof Error ? e.message : "불러오지 못했습니다"));
    return () => { alive = false; };
  }, [id, reload]);

  if (error) return <PageScrollShell><LoadErrorState message={error} onRetry={() => setReload((k) => k + 1)} /></PageScrollShell>;
  if (!data) return <PageScrollShell><p className="p-6 text-cell text-[var(--text-muted)]">불러오는 중…</p></PageScrollShell>;

  const a = data.asset;
  const est = a.assetType === "real_estate" && data.explain && "comps" in data.explain ? (data.explain as ReEstimate) : null;
  const detail = a.detail as Record<string, unknown>;
  return (
    <PageScrollShell>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <Link href="/holdings" className="inline-flex items-center gap-1 text-cell text-[var(--text-muted)] hover:text-foreground"><ArrowLeft className="size-3.5" /> 내 포트폴리오</Link>
        <h1 className="text-section font-semibold">{a.name}</h1>
        <span className="text-micro text-[var(--text-muted)]">
          {ASSET_TYPE_LABEL[a.assetType]}
          {typeof detail.propertyType === "string" ? ` · ${PROPERTY_TYPE_LABEL[detail.propertyType as keyof typeof PROPERTY_TYPE_LABEL] ?? ""}` : ""}
          {typeof detail.regionName === "string" && detail.regionName ? ` · ${detail.regionName}` : ""}
          {typeof detail.area === "number" ? ` · ${detail.area}㎡(${Math.round((detail.area as number) / 3.3058)}평)` : ""}
        </span>
        <Button size="sm" variant="outline" className="ml-auto" onClick={() => setEdit(true)}><Pencil className="size-3.5" /> 수정</Button>
      </div>

      <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_360px] [&>*]:min-w-0">
        <div className="space-y-3">
          <ValuationCard data={data} est={est} />
          {est && <CompsCard est={est} />}
          {data.sales && <TradesCard sales={data.sales} rents={data.rents ?? []} area={typeof detail.area === "number" ? (detail.area as number) : 0} est={est} />}
          <HistoryCard data={data} />
        </div>
        <div className="space-y-3">
          <ReturnBox data={data} />
          {data.market && <RegionMarketCard market={data.market} />}
        </div>
      </div>
      {edit && <AssetFormDialog open asset={a} onClose={() => setEdit(false)} onSaved={() => setReload((k) => k + 1)} />}
    </PageScrollShell>
  );
}

function ValuationCard({ data, est }: { data: AssetDetailData; est: ReEstimate | null }) {
  const v = data.now ?? data.asset.valuation;
  const ratio = data.asset.ownershipRatio || 1;
  return (
    <Card className="p-3">
      {!v ? (
        <p className="text-cell text-[var(--score-mid)]">{data.error ?? "아직 평가할 자료가 없습니다."}</p>
      ) : (
        <>
          <div className="flex flex-wrap items-baseline gap-2">
            <span className="rounded-md bg-[var(--brand-muted)] px-1.5 py-0.5 text-micro font-semibold text-[var(--brand)]">{VALUATION_KIND_LABEL[v.kind]}</span>
            <b className="font-num text-2xl tabular-nums">{money(v.value)}</b>
            {ratio < 1 && <span className="text-cell text-[var(--text-muted)]">내 지분 {(ratio * 100).toFixed(0)}% = {money(v.value * ratio)}</span>}
            {v.confidence && <span className="text-micro text-[var(--text-muted)]">{CONFIDENCE_LABEL[v.confidence as "high" | "medium" | "low"]}</span>}
          </div>
          {v.low != null && v.high != null && v.kind === "estimate" && (
            <p className="mt-1 text-cell text-[var(--text-secondary)]">범위 {money(v.low)} ~ {money(v.high)}</p>
          )}
          <p className="mt-1 text-micro text-[var(--text-muted)]">출처 {v.source} · {v.asOf} 기준</p>
          {est && (
            <dl className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
              <Fact label="최근 실거래가" value={est.lastTrade ? money(est.lastTrade.price) : "-"} sub={est.lastTrade ? `${est.lastTrade.date} · ${est.lastTradeDays}일 전` : ""} />
              <Fact label="최근 3개월 평균" value={est.recent3Avg ? money(est.recent3Avg) : "거래 없음"} sub={est.recent3N ? `${est.recent3N}건` : ""} />
              <Fact label={est.method === "region_unit" ? "㎡당 중앙가" : "동일 면적 중앙값"} value={est.method === "region_unit" ? money(est.unitPerM2) : money(est.sameAreaMedian)} sub={`${est.windowMonths}개월 · ${est.n}건`} />
              <Fact label="전세 시세" value={est.jeonse ? money(est.jeonse.median) : "-"} sub={est.jeonse ? `${est.jeonse.n}건 · 전세가율 ${((est.jeonse.median / est.value) * 100).toFixed(0)}%` : ""} />
            </dl>
          )}
          {est && (
            <ul className="mt-3 list-disc space-y-0.5 pl-4 text-micro text-[var(--text-muted)]">
              <li>근거: {est.method === "comparable" ? `같은 단지·비슷한 면적(±${Math.max(3, Math.round(((data.asset.detail as { area?: number }).area ?? 0) * 0.04))}㎡) 실거래 ${est.n}건, 최근 ${est.windowMonths}개월` : `같은 지역·비슷한 면적의 ㎡당 가격 ${est.n}건`}</li>
              {est.indexAdjusted && <li>거래가 모자라 한국부동산원 주간 매매지수로 거래 시점 → 오늘로 보정했습니다.</li>}
              {(est.excludedDirect > 0 || est.excludedCancelled > 0) && <li>직거래 {est.excludedDirect}건 · 해제 {est.excludedCancelled}건은 뺐습니다(시세와 다른 거래).</li>}
              <li>층·향·동·수리 상태는 반영하지 않았습니다. 감정평가가 아닙니다.</li>
            </ul>
          )}
          {!est && data.explain && <ExplainList explain={data.explain as Record<string, unknown>} />}
        </>
      )}
    </Card>
  );
}

function ExplainList({ explain }: { explain: Record<string, unknown> }) {
  const labels: Record<string, string> = {
    interest: "경과이자(세후)", taxRate: "세율(%)", discountYield: "할인 금리(%)", curve: "할인 곡선",
    price: "현재가", changeRate: "전일 대비(%)", koreanName: "이름", xauUsd: "금 현물($/oz)", usdKrw: "원/달러", perGram: "g당",
  };
  return (
    <dl className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3">
      {Object.entries(explain).map(([k, v]) => (
        <Fact key={k} label={labels[k] ?? k} value={typeof v === "number" ? (Math.abs(v) >= 1000 ? money(v) : v.toLocaleString("ko-KR", { maximumFractionDigits: 3 })) : String(v)} />
      ))}
    </dl>
  );
}

function Fact({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="rounded-md border px-2 py-1.5">
      <dt className="text-micro text-[var(--text-muted)]">{label}</dt>
      <dd className="font-num text-cell font-semibold tabular-nums">{value}</dd>
      {sub ? <dd className="text-micro text-[var(--text-muted)]">{sub}</dd> : null}
    </div>
  );
}

function CompsCard({ est }: { est: ReEstimate }) {
  if (est.comps.length === 0) return null;
  return (
    <Card className="p-0">
      <h2 className="border-b px-3 py-2 text-section font-semibold">비교한 거래</h2>
      <div className="max-h-72 overflow-y-auto">
        <table className="w-full text-cell">
          <thead className="sticky top-0 bg-card text-micro text-[var(--text-muted)]">
            <tr><th className="px-3 py-1 text-left">계약일</th><th className="text-right">면적</th><th className="text-right">층</th><th className="text-right">거래가</th><th className="px-3 text-right">{est.indexAdjusted ? "오늘 환산" : "면적 환산"}</th></tr>
          </thead>
          <tbody>
            {est.comps.map((c, i) => (
              <tr key={i} className="border-t">
                <td className="px-3 py-1">{c.date}</td>
                <td className="text-right font-num tabular-nums">{c.area}</td>
                <td className="text-right font-num tabular-nums">{c.floor ?? "-"}</td>
                <td className="text-right font-num tabular-nums">{money(c.price)}</td>
                <td className="px-3 text-right font-num tabular-nums">{money(c.adjusted)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

function TradesCard({ sales, rents, area, est }: { sales: ReTrade[]; rents: ReTrade[]; area: number; est: ReEstimate | null }) {
  const [sameArea, setSameArea] = useState(true);
  const [now] = useState(() => Date.now());
  const points = useMemo(() => {
    const tol = Math.max(3, area * 0.04);
    const ok = (t: ReTrade) => !sameArea || area <= 0 || Math.abs(t.area - area) <= tol;
    const pts: TradePoint[] = [];
    for (const t of sales) if (t.price && !t.cancelled && ok(t)) pts.push({ t: Date.parse(t.dealDate), price: t.price, label: t.dealDate, kind: "sale" });
    for (const t of rents) if (t.deposit && !t.monthlyRent && ok(t)) pts.push({ t: Date.parse(t.dealDate), price: t.deposit, label: t.dealDate, kind: "jeonse" });
    if (est) pts.push({ t: now, price: est.value, label: "추정가", kind: "estimate" });
    return pts;
  }, [sales, rents, area, sameArea, est, now]);
  return (
    <Card className="p-3">
      <div className="flex items-center gap-2">
        <h2 className="text-section font-semibold">이 단지 실거래</h2>
        <label className="ml-auto flex items-center gap-1 text-micro text-[var(--text-muted)]">
          <input type="checkbox" checked={sameArea} onChange={(e) => setSameArea(e.target.checked)} className="size-3.5" /> 같은 면적만
        </label>
      </div>
      <TradeScatter points={points} />
      <p className="mt-1 text-micro text-[var(--text-muted)]">매매 3년 · 전세 1년 · 출처 국토교통부 실거래가 공개시스템</p>
    </Card>
  );
}

function HistoryCard({ data }: { data: AssetDetailData }) {
  const pts = data.history.map((h) => ({ date: h.asOf, value: h.value }));
  return (
    <Card className="p-3">
      <h2 className="text-section font-semibold">평가 추이</h2>
      {pts.length < 2 ? (
        <p className="mt-1 text-cell text-[var(--text-muted)]">평가는 매일 한 번 쌓입니다 — 며칠 뒤 추이가 그려집니다.</p>
      ) : (
        <MultiLineChart lines={[{ key: "v", label: "평가액", color: "#3b6ff5", points: pts }]} format={(v) => money(v)} />
      )}
    </Card>
  );
}

function ReturnBox({ data }: { data: AssetDetailData }) {
  const a = data.asset;
  return (
    <Card className="p-3">
      <h2 className="text-section font-semibold">내 몫 손익</h2>
      <dl className="mt-2 space-y-1 text-cell">
        <Row k="평가액(내 지분)" v={money(a.myValue)} />
        <Row k="원가" v={a.myCost != null ? money(a.myCost) : "취득가 미입력"} />
        {a.pnl != null && <Row k="평가손익" v={<span className={trendTextClass(a.pnl)}>{money(a.pnl)} ({signedPct(a.returnPct)})</span>} />}
        {a.annualizedPct != null && <Row k="연환산" v={<span className={trendTextClass(a.annualizedPct)}>{signedPct(a.annualizedPct)}</span>} />}
        {a.acquiredAt && <Row k="취득일" v={a.acquiredAt} />}
      </dl>
      <p className="mt-2 text-micro text-[var(--text-muted)]">부동산 원가에는 입력한 취득 부대비용(취득세·중개보수)이 들어갑니다.</p>
    </Card>
  );
}

function Row({ k, v }: { k: string; v: React.ReactNode }) {
  return (
    <div className="flex items-baseline gap-2">
      <dt className="text-[var(--text-muted)]">{k}</dt>
      <dd className={cn("ml-auto font-num tabular-nums")}>{v}</dd>
    </div>
  );
}
