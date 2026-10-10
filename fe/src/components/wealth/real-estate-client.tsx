"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, Search } from "lucide-react";
import { toast } from "sonner";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { MiniToggle } from "@/components/ui/mini-toggle";
import {
  AINoteLine, Pct, ScoreBadge, ScoreCard, type ScoreKind,
} from "@/components/wealth/asset-intel-ui";
import { ComplexCompareCard, CompareButton } from "@/components/wealth/complex-compare";
import { WatchStar } from "@/components/wealth/re-watch";
import { RegionMarketCard } from "@/components/wealth/region-market-card";
import { TradeScatter, type TradePoint } from "@/components/wealth/wealth-charts";
import { areaType, groupAreas, supplyPyeong } from "@/lib/re-trades";
import { AssetFormDialog, useRegions } from "@/components/wealth/wealth-forms";
import {
  CONFIDENCE_LABEL,
  fetchComplex,
  fetchComplexes,
  fetchComplexRankings,
  fetchRealEstateOverview,
  fetchRegionMarket,
  money,
  type ComplexData,
  type AINote,
  type AreaIntel,
  type ComplexRankRow,
  type RankingSort,
  type RealEstateOverview,
  type ReComplex,
  type RegionMarket,
  type RegionOverviewRow,
  type ReRegion,
  type ReTrade,
  type ScoreView,
} from "@/lib/wealth-api";
import { cn } from "@/lib/utils";
import { AssetPostsCard } from "@/components/community/asset-posts-card";

/**
 * 「부동산 시세」 (2026-10-01 개편 — 호갱노노·아실·네이버부동산의 화면 구성을 참고).
 *
 *   전국(시도 표·AI 요약) → 시도(시도 점수·AI 요약·시군구 표·시도 단지 랭킹) → 시군구(점수 근거·AI 요약·단지 랭킹)
 *   → 단지(평형 칩 · 시세 요약 카드 · 기간별 실거래 점그래프 · 실거래/전월세 표: 기간 최고가·해제·직거래 표시).
 *   2026-10-02: 시도를 누르면 구를 안 골라도 시도 전체 기준으로 보인다(사용자 요청).
 *
 * 매물(호가)은 쓰지 않는다 — 공식 API가 없고 데이터베이스권 판례가 있다(docs/ASSET_EXPANSION_RESEARCH.md §4.4).
 * 주소(?sido=&lawd=&complex=)에 상태를 실어 공유·뒤로가기가 된다.
 */
const SORTS: { id: RankingSort; label: string }[] = [
  { id: "score", label: "시장 점수" },
  { id: "momentumPct", label: "많이 오른" },
  { id: "trades6m", label: "거래 많은" },
  { id: "jeonseRatio", label: "전세가율 높은" },
  { id: "pyeongPrice", label: "평당가 높은" },
  // 2026-10-03 R4 — 고점 대비(최근 3개월 ÷ 12개월 월별 최고): 많이 내린 순 · 고점에 붙은(신고가 근접) 순.
  { id: "peakDrop", label: "고점 대비 내린" },
  { id: "peakPct", label: "신고가 근접" },
];

type AreaSort = "score" | "w4" | "w12" | "vol";
const AREA_SORTS: { id: AreaSort; label: string }[] = [
  { id: "score", label: "점수" },
  { id: "w4", label: "4주 상승" },
  { id: "w12", label: "12주 상승" },
  { id: "vol", label: "거래 증가" },
];
const AREA_SORT_KEY: Record<AreaSort, string> = { score: "", w4: "w4Pct", w12: "w12Pct", vol: "volTrendPct" };

function sortByFacts<T extends { score: ScoreView }>(rows: T[], sort: AreaSort): T[] {
  const key = AREA_SORT_KEY[sort];
  return [...rows].sort((a, b) => (key ? Number(b.score.facts[key] ?? 0) - Number(a.score.facts[key] ?? 0) : b.score.score - a.score.score));
}

const sidoCode = (lawd: string) => lawd.slice(0, 2);

export function RealEstateClient() {
  const params = useSearchParams();
  const router = useRouter();
  const regions = useRegions();
  const lawd = params.get("lawd") ?? "";
  const complexKey = params.get("complex") ?? "";
  const sido = params.get("sido") || (lawd ? sidoCode(lawd) : "");
  const region = regions?.find((r) => r.lawdCd === lawd) ?? null;
  const sidoName = regions?.find((r) => sidoCode(r.lawdCd) === sido)?.sido ?? "";
  const go = (next: { sido?: string; lawd?: string; complex?: string }) => {
    const p = new URLSearchParams();
    if (next.sido && !next.lawd) p.set("sido", next.sido);
    if (next.lawd) p.set("lawd", next.lawd);
    if (next.complex) p.set("complex", next.complex);
    for (const c of params.getAll("cmp")) p.append("cmp", c); // 비교 바구니는 화면을 옮겨도 남는다(R2)
    const qs = p.toString();
    router.replace(`/real-estate${qs ? `?${qs}` : ""}`, { scroll: false });
  };
  const regionName = (l: string) => regions?.find((r) => r.lawdCd === l)?.name ?? "";

  return (
    <PageScrollShell>
      <RegionBar regions={regions} sido={sido} lawd={lawd} onSido={(s) => go({ sido: s })} onPick={(l) => go({ lawd: l })} />
      <ComplexCompareCard regionName={regionName} />
      {complexKey ? (
        <ComplexView key={complexKey} complexKey={complexKey} lawd={lawd} region={region} onBack={() => go({ lawd })} />
      ) : lawd ? (
        <RegionView key={lawd} lawd={lawd} region={region} onPick={(k) => go({ lawd, complex: k })} />
      ) : sido ? (
        <SidoView key={sido} sido={sido} name={sidoName} regionName={regionName}
          onRegion={(l) => go({ lawd: l })} onComplex={(k, l) => go({ lawd: l, complex: k })} />
      ) : (
        <NationView onSido={(s) => go({ sido: s })} onRegion={(l) => go({ lawd: l })} />
      )}
      <p className="mt-4 text-micro text-[var(--text-muted)]">
        출처 국토교통부 실거래가 공개시스템 · 한국부동산원 부동산통계정보(R-ONE)
      </p>
    </PageScrollShell>
  );
}

/** 시도 칩 + 시군구 칩. 시도 칩 = 그 시도 전체 화면. */
function RegionBar({ regions, sido, lawd, onSido, onPick }: {
  regions: ReRegion[] | null; sido: string; lawd: string; onSido: (sido: string) => void; onPick: (lawd: string) => void;
}) {
  const sidos = useMemo(() => {
    const m = new Map<string, string>();
    for (const r of regions ?? []) m.set(sidoCode(r.lawdCd), r.sido);
    return [...m.entries()].sort((a, b) => a[0].localeCompare(b[0]));
  }, [regions]);
  const list = (regions ?? []).filter((r) => sidoCode(r.lawdCd) === sido).sort((a, b) => a.name.localeCompare(b.name, "ko"));
  return (
    <Card className="mb-3 space-y-2 p-3">
      <div className="flex flex-wrap gap-1">
        <Chip on={!sido} onClick={() => onSido("")}>전국</Chip>
        {sidos.map(([code, name]) => (
          <Chip key={code} on={sido === code} onClick={() => onSido(code)}>{shortSido(name)}</Chip>
        ))}
      </div>
      {sido && (
        <div className="flex flex-wrap gap-1 border-t pt-2">
          <Chip on={!lawd} onClick={() => onSido(sido)} small>전체</Chip>
          {list.map((r) => (
            <Chip key={r.lawdCd} on={r.lawdCd === lawd} onClick={() => onPick(r.lawdCd)} small>{r.name}</Chip>
          ))}
        </div>
      )}
    </Card>
  );
}

function shortSido(name: string) {
  const map: Record<string, string> = { 충청북도: "충북", 충청남도: "충남", 전라북도: "전북", 전북특별자치도: "전북", 전라남도: "전남", 경상북도: "경북", 경상남도: "경남", 강원특별자치도: "강원", 강원도: "강원", 제주특별자치도: "제주", 세종특별자치시: "세종", 전남광주통합특별시: "전남광주" };
  return map[name] ?? name.replace(/특별시|광역시|도$/, "");
}

function Chip({ on, onClick, children, small }: { on: boolean; onClick: () => void; children: React.ReactNode; small?: boolean }) {
  return (
    <button
      type="button"
      aria-pressed={on}
      onClick={onClick}
      className={cn(
        "shrink-0 rounded-full border px-2.5",
        small ? "h-6 text-micro" : "h-7 text-cell",
        on ? "border-[var(--brand)] bg-[var(--brand)]/10 font-semibold text-[var(--brand)]" : "text-[var(--text-secondary)] hover:bg-muted",
      )}
    >
      {children}
    </button>
  );
}

// ── 전국 · 시도 머리 ─────────────────────────────────────────────────────────

type SubPair = [string, number];

/** 전국·시도 요약 — 지수 변화, AI 요약, 시군구 오름·내림 분포, 많이 오른·내린 곳. */
function AreaHeader({ area, title }: { area: AreaIntel | null | undefined; title: string }) {
  const f = (area?.facts ?? {}) as Record<string, unknown>;
  const n = Number(f.regions ?? 0);
  const up = Number(f.up ?? 0);
  const down = Number(f.down ?? 0);
  const flat = Math.max(0, n - up - down);
  const top = (f.top as SubPair[] | undefined) ?? [];
  const bottom = (f.bottom as SubPair[] | undefined) ?? [];
  const sidos = (f.sidos as SubPair[] | undefined) ?? [];
  const pct = (k: string) => (typeof f[k] === "number" ? (f[k] as number) : null);
  return (
    <Card className="mb-3 p-3">
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="text-lg font-bold">{title} 아파트 시장</h2>
        {area?.score && <ScoreBadge score={area.score} kind="sido" />}
        <span className="ml-auto flex flex-wrap gap-x-3 gap-y-0.5 text-cell text-[var(--text-muted)]">
          <span>주간 매매지수 1주 <Pct v={pct("w1Pct")} digits={2} /></span>
          <span>4주 <Pct v={pct("w4Pct")} digits={2} /></span>
          <span>12주 <Pct v={pct("w12Pct")} digits={2} /></span>
          <span>전세 4주 <Pct v={pct("jeonse4Pct")} digits={2} /></span>
          <span>거래량 <Pct v={pct("volTrendPct")} digits={0} /></span>
        </span>
      </div>
      {area === undefined ? (
        <p className="mt-2 text-cell text-[var(--text-muted)]">불러오는 중…</p>
      ) : !area ? (
        <p className="mt-2 text-cell text-[var(--text-muted)]">이 묶음의 요약을 아직 만들지 않았습니다(매일 09:50 점수 → 10:20 AI 요약).</p>
      ) : (
        <>
          <AINoteLine note={area.note} className="mt-2" />
          {n > 0 && (
            <div className="mt-2 grid gap-2 text-micro md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
              <div>
                <div className="flex justify-between text-[var(--text-muted)]">
                  <span>시군구 {n}곳 · 4주 매매지수</span>
                  <span><span className="text-[var(--profit)]">상승 {up}</span> · 보합 {flat} · <span className="text-[var(--loss)]">하락 {down}</span></span>
                </div>
                <div className="mt-1 flex h-2 overflow-hidden rounded-full bg-[var(--surface-elevated)]" role="img" aria-label={`상승 ${up}, 보합 ${flat}, 하락 ${down}`}>
                  <div className="bg-[var(--profit)]" style={{ width: `${(up / n) * 100}%` }} />
                  <div className="bg-[var(--text-muted)]/30" style={{ width: `${(flat / n) * 100}%` }} />
                  <div className="bg-[var(--loss)]" style={{ width: `${(down / n) * 100}%` }} />
                </div>
              </div>
              <div className="space-y-0.5">
                {sidos.length > 0 && <PairLine label="시도" pairs={sidos.slice(0, 4)} />}
                <PairLine label="많이 오른" pairs={top} />
                <PairLine label="약한" pairs={bottom} />
              </div>
            </div>
          )}
        </>
      )}
    </Card>
  );
}

function PairLine({ label, pairs }: { label: string; pairs: SubPair[] }) {
  if (pairs.length === 0) return null;
  return (
    <p className="truncate">
      <span className="text-[var(--text-muted)]">{label} </span>
      {pairs.map(([name, v], i) => (
        <span key={name}>{i > 0 && " · "}{name} <Pct v={v} digits={2} /></span>
      ))}
    </p>
  );
}

// ── 전국 ──────────────────────────────────────────────────────────────────────

function NationView({ onSido, onRegion }: { onSido: (sido: string) => void; onRegion: (lawd: string) => void }) {
  const [market, setMarket] = useState<RegionMarket | null>(null);
  const [data, setData] = useState<RealEstateOverview | null>(null);
  const [failed, setFailed] = useState(false);
  const [sort, setSort] = useState<AreaSort>("score");
  const [level, setLevel] = useState<"sido" | "region">("sido");
  useEffect(() => {
    let alive = true;
    fetchRegionMarket("").then((m) => alive && setMarket(m)).catch(() => {});
    fetchRealEstateOverview("").then((d) => alive && setData(d)).catch(() => alive && setFailed(true));
    return () => { alive = false; };
  }, []);
  const sidos = useMemo(() => sortByFacts(data?.sidos ?? [], sort), [data, sort]);
  const regions = useMemo(() => sortByFacts(data?.regions ?? [], sort), [data, sort]);
  return (
    <>
      <AreaHeader area={failed ? null : data?.area} title="전국" />
      <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_400px] [&>*]:min-w-0">
        <Card className="p-0">
          <div className="flex flex-wrap items-center gap-2 border-b px-3 py-2">
            <MiniToggle options={[{ id: "sido", label: "시도별" }, { id: "region", label: "시군구 전체" }]} value={level} onChange={setLevel} />
            <MiniToggle options={AREA_SORTS} value={sort} onChange={setSort} />
          </div>
          {data == null && !failed ? (
            <p className="p-3 text-cell text-[var(--text-muted)]">불러오는 중…</p>
          ) : level === "sido" ? (
            sidos.length === 0 ? <p className="p-3 text-cell text-[var(--text-muted)]">시도 점수를 아직 만들지 않았습니다(매일 09:50).</p>
              : <AreaTable rows={sidos.map((s) => ({ id: s.sido, name: s.name, score: s.score, note: s.note }))} kind="sido" onPick={onSido} />
          ) : regions.length === 0 ? (
            <p className="p-3 text-cell text-[var(--text-muted)]">지역 점수를 아직 만들지 않았습니다(매일 09:50).</p>
          ) : (
            <AreaTable rows={regions.map((r) => ({ id: r.lawdCd, name: r.name, score: r.score, note: r.note }))} kind="region" onPick={onRegion} />
          )}
        </Card>
        <div className="space-y-3">{market && <RegionMarketCard market={market} />}</div>
      </div>
    </>
  );
}

type AreaRow = { id: string; name: string; score: ScoreView; note?: AINote };

function AreaTable({ rows, kind, onPick }: { rows: AreaRow[]; kind: ScoreKind; onPick: (id: string) => void }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[560px] text-cell">
        <thead className="text-micro text-[var(--text-muted)]">
          <tr className="border-b">
            <th className="px-3 py-1.5 text-left">지역</th>
            <th className="text-right">1주</th>
            <th className="text-right">4주</th>
            <th className="text-right">12주</th>
            <th className="text-right">거래량 추세</th>
            <th className="px-3 text-right">점수</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => {
            const f = r.score.facts as Record<string, number>;
            return (
              <tr key={r.id} className="cursor-pointer border-b last:border-b-0 hover:bg-muted" onClick={() => onPick(r.id)}>
                <td className="px-3 py-1.5">
                  <b>{r.name}</b>
                  {r.note?.body?.note && <span className="block max-w-[420px] truncate text-micro text-[var(--text-muted)]">{r.note.body.note}</span>}
                </td>
                <td className="text-right"><Pct v={f.w1Pct} digits={2} /></td>
                <td className="text-right"><Pct v={f.w4Pct} digits={2} /></td>
                <td className="text-right"><Pct v={f.w12Pct} digits={2} /></td>
                <td className="text-right"><Pct v={f.volTrendPct} digits={0} /></td>
                <td className="px-3 text-right"><ScoreBadge score={r.score} kind={kind} /></td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ── 시도 ──────────────────────────────────────────────────────────────────────

function SidoView({ sido, name, regionName, onRegion, onComplex }: {
  sido: string; name: string; regionName: (lawd: string) => string; onRegion: (lawd: string) => void; onComplex: (key: string, lawd: string) => void;
}) {
  const [market, setMarket] = useState<RegionMarket | null>(null);
  const [data, setData] = useState<RealEstateOverview | null>(null);
  const [failed, setFailed] = useState(false);
  const [sort, setSort] = useState<AreaSort>("score");
  const [rankSort, setRankSort] = useState<RankingSort>("score");
  const [ranks, setRanks] = useState<{ sort: RankingSort; items: ComplexRankRow[] } | null>(null);
  useEffect(() => {
    let alive = true;
    fetchRegionMarket("", sido).then((m) => alive && setMarket(m)).catch(() => {});
    fetchRealEstateOverview(sido).then((d) => alive && setData(d)).catch(() => alive && setFailed(true));
    return () => { alive = false; };
  }, [sido]);
  useEffect(() => {
    let alive = true;
    fetchComplexRankings("", sido, rankSort, 40)
      .then((d) => alive && setRanks({ sort: rankSort, items: d.items }))
      .catch(() => alive && setRanks({ sort: rankSort, items: [] }));
    return () => { alive = false; };
  }, [sido, rankSort]);
  const regions = useMemo(() => sortByFacts(data?.regions ?? [], sort), [data, sort]);
  const items = ranks?.sort === rankSort ? ranks.items : null;
  const title = name || data?.area?.name || sido;
  return (
    <>
      <AreaHeader area={failed ? null : data?.area} title={title} />
      <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_400px] [&>*]:min-w-0">
        <div className="space-y-3">
          <Card className="p-0">
            <div className="flex flex-wrap items-center gap-2 border-b px-3 py-2">
              <h2 className="text-section font-semibold">시군구</h2>
              <MiniToggle options={AREA_SORTS} value={sort} onChange={setSort} />
            </div>
            {data == null && !failed ? (
              <p className="p-3 text-cell text-[var(--text-muted)]">불러오는 중…</p>
            ) : regions.length === 0 ? (
              <p className="p-3 text-cell text-[var(--text-muted)]">이 시도의 시군구 점수가 아직 없습니다.</p>
            ) : (
              <AreaTable rows={regions.map((r) => ({ id: r.lawdCd, name: r.name.replace(/^\S+\s/, ""), score: r.score, note: r.note }))} kind="region" onPick={onRegion} />
            )}
          </Card>
          <Card className="p-0">
            <div className="flex flex-wrap items-center gap-2 border-b px-3 py-2">
              <h2 className="text-section font-semibold">{shortSido(title)} 아파트 단지</h2>
              <MiniToggle options={SORTS} value={rankSort} onChange={setRankSort} />
            </div>
            {items == null ? (
              <p className="p-3 text-cell text-[var(--text-muted)]">불러오는 중…</p>
            ) : items.length === 0 ? (
              <p className="p-3 text-cell text-[var(--text-muted)]">점수를 매긴 단지가 아직 없습니다.</p>
            ) : (
              <ComplexTable rows={items} onPick={(k) => onComplex(k, items.find((i) => i.complexKey === k)?.lawdCd ?? "")} regionName={regionName} />
            )}
          </Card>
        </div>
        <div className="space-y-3">
          {data?.area?.score && <ScoreCard title="시장 점수" score={data.area.score} kind="sido" />}
          {market && <RegionMarketCard market={market} />}
        </div>
      </div>
    </>
  );
}

// ── 지역 ──────────────────────────────────────────────────────────────────────

function RegionView({ lawd, region, onPick }: { lawd: string; region: ReRegion | null; onPick: (key: string) => void }) {
  const [market, setMarket] = useState<RegionMarket | null>(null);
  const [overview, setOverview] = useState<RegionOverviewRow | null>(null);
  const [sort, setSort] = useState<RankingSort>("score");
  const [ranks, setRanks] = useState<{ sort: RankingSort; items: ComplexRankRow[] } | null>(null);
  const [q, setQ] = useState("");
  const [found, setFound] = useState<ReComplex[] | null>(null);

  useEffect(() => {
    let alive = true;
    fetchRegionMarket(lawd).then((m) => alive && setMarket(m)).catch(() => {});
    fetchRealEstateOverview(sidoCode(lawd))
      .then((d) => alive && setOverview(d.regions.find((r) => r.lawdCd === lawd) ?? null))
      .catch(() => {});
    return () => { alive = false; };
  }, [lawd]);
  useEffect(() => {
    let alive = true;
    fetchComplexRankings(lawd, "", sort, 40)
      .then((d) => alive && setRanks({ sort, items: d.items }))
      .catch(() => alive && setRanks({ sort, items: [] }));
    return () => { alive = false; };
  }, [lawd, sort]);

  const search = async () => {
    try {
      setFound(await fetchComplexes(lawd, "apt", q));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "검색하지 못했습니다");
    }
  };
  const f = (overview?.score.facts ?? {}) as Record<string, number>;
  const items = ranks?.sort === sort ? ranks.items : null;

  return (
    <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_400px] [&>*]:min-w-0">
      <div className="space-y-3">
        <Card className="p-3">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-lg font-bold">{region ? `${region.sido} ${region.name}` : lawd}</h2>
            <ScoreBadge score={overview?.score} kind="region" />
            <WatchStar kind="region" itemKey={lawd} />
            <span className="ml-auto flex flex-wrap gap-3 text-cell">
              <span className="text-[var(--text-muted)]">주간 매매지수 1주 <Pct v={f.w1Pct} digits={2} /></span>
              <span className="text-[var(--text-muted)]">4주 <Pct v={f.w4Pct} digits={2} /></span>
              <span className="text-[var(--text-muted)]">전세 4주 <Pct v={f.jeonse4Pct} digits={2} /></span>
            </span>
          </div>
          <AINoteLine note={overview?.note} className="mt-2" />
        </Card>

        <Card className="p-0">
          <div className="flex flex-wrap items-center gap-2 border-b px-3 py-2">
            <h2 className="text-section font-semibold">아파트 단지</h2>
            <MiniToggle options={SORTS} value={sort} onChange={(v) => { setSort(v); setFound(null); }} />
            <span className="ml-auto flex items-center gap-1">
              <input value={q} maxLength={40} onChange={(e) => setQ(e.target.value)} onKeyDown={(e) => e.key === "Enter" && void search()}
                placeholder="단지 이름" aria-label="단지 이름" className="h-7 w-32 rounded-md border bg-background px-2 text-cell" />
              <Button size="sm" variant="outline" onClick={() => void search()} aria-label="단지 찾기"><Search className="size-3.5" /></Button>
            </span>
          </div>
          {found ? (
            <ul>
              {found.length === 0 && <li className="p-3 text-cell text-[var(--text-muted)]">거래가 있는 단지가 없습니다.</li>}
              {found.map((c) => (
                <li key={c.complexKey}>
                  <button type="button" onClick={() => onPick(c.complexKey)} className="flex w-full items-baseline gap-2 border-b px-3 py-2 text-left text-cell hover:bg-muted">
                    <b>{c.complexName}</b>
                    <span className="text-micro text-[var(--text-muted)]">{c.umdNm} {c.jibun}{c.buildYear ? ` · ${c.buildYear}년` : ""}</span>
                    <span className="ml-auto text-micro text-[var(--text-muted)]">거래 {c.trades} · 최근 {c.lastDeal}</span>
                  </button>
                </li>
              ))}
            </ul>
          ) : items == null ? (
            <p className="p-3 text-cell text-[var(--text-muted)]">불러오는 중…</p>
          ) : items.length === 0 ? (
            <p className="p-3 text-cell text-[var(--text-muted)]">점수를 매긴 단지가 아직 없습니다(최근 12개월 매매 5건 이상인 단지만). 이름으로 찾아보세요.</p>
          ) : (
            <ComplexTable rows={items} onPick={onPick} />
          )}
        </Card>
      </div>
      <div className="space-y-3">
        {overview?.score && <ScoreCard title="시장 점수" score={overview.score} kind="region" />}
        {market && <RegionMarketCard market={market} />}
        <AssetPostsCard kind="region" assetKey={lawd} />
      </div>
    </div>
  );
}

function ComplexTable({ rows, onPick, regionName }: { rows: ComplexRankRow[]; onPick: (key: string) => void; regionName?: (lawd: string) => string }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[620px] text-cell">
        <thead className="text-micro text-[var(--text-muted)]">
          <tr className="border-b">
            <th className="px-3 py-1.5 text-left">단지</th>
            <th className="text-right">평당가</th>
            <th className="text-right" title="최근 3개월 ㎡당 중앙가 vs 그 전 9개월">3개월 변화</th>
            <th className="text-right">6개월 거래</th>
            <th className="text-right">전세가율</th>
            <th className="text-right" title="최근 3개월 ÷ 12개월 월별 최고">고점 대비</th>
            <th className="px-3 text-right">점수</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => {
            const f = r.score.facts as Record<string, number | string>;
            return (
              <tr key={r.complexKey} className="cursor-pointer border-b last:border-b-0 hover:bg-muted" onClick={() => onPick(r.complexKey)}>
                <td className="px-3 py-1.5">
                  <b>{r.name}</b>
                  <span className="ml-1 text-micro text-[var(--text-muted)]">
                    {regionName ? `${regionName(r.lawdCd)} ` : ""}{String(f.umdNm ?? "")}{f.buildYear ? ` · ${f.buildYear}년` : ""}
                  </span>
                  {r.note?.body?.note && <span className="block max-w-[380px] truncate text-micro text-[var(--text-muted)]">{r.note.body.note}</span>}
                </td>
                <td className="text-right font-num tabular-nums">{money(Number(f.pyeongPrice))}</td>
                <td className="text-right"><Pct v={Number(f.momentumPct)} /></td>
                <td className="text-right font-num tabular-nums">{String(f.trades6m)}건</td>
                <td className="text-right font-num tabular-nums">{Number(f.jeonseRatio) > 0 ? `${Number(f.jeonseRatio).toFixed(0)}%` : "-"}</td>
                <td className="text-right font-num tabular-nums">{Number(f.peakPct).toFixed(0)}%</td>
                <td className="px-3 text-right"><ScoreBadge score={r.score} kind="complex" /></td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ── 단지 ──────────────────────────────────────────────────────────────────────

function ComplexView({ complexKey, lawd, region, onBack }: { complexKey: string; lawd: string; region: ReRegion | null; onBack: () => void }) {
  const { session } = useAuth();
  const [base, setBase] = useState<ComplexData | null>(null);
  const [areaKey, setAreaKey] = useState<number | null>(null);
  const [withEst, setWithEst] = useState<{ key: number; data: ComplexData } | null>(null);
  const [period, setPeriod] = useState<"1" | "all">("all");
  const [register, setRegister] = useState(false);
  const [now] = useState(() => Date.now());

  useEffect(() => {
    let alive = true;
    fetchComplex(complexKey, lawd).then((d) => alive && setBase(d)).catch(() => alive && setBase(null));
    return () => { alive = false; };
  }, [complexKey, lawd]);

  // 평형 = 전용면적 정수 부분(84.39·84.6·84.89 → 84㎡형, lib/re-trades).
  const groups = useMemo(() => (base ? groupAreas([...base.sales, ...base.rents]) : []), [base]);
  const busiest = groups.reduce<{ key: number; n: number } | null>((b, g) => (!b || g.n > b.n ? g : b), null);
  const key = areaKey ?? busiest?.key ?? null;
  const groupArea = groups.find((g) => g.key === key)?.area ?? null;

  useEffect(() => {
    if (key == null || groupArea == null) return;
    let alive = true;
    fetchComplex(complexKey, lawd, groupArea).then((d) => alive && setWithEst({ key, data: d })).catch(() => {});
    return () => { alive = false; };
  }, [complexKey, lawd, key, groupArea]);

  const est = withEst?.key === key ? withEst.data.estimate : undefined;
  const jeonse = withEst?.key === key ? withEst.data.jeonse : undefined;
  const inGroup = (t: ReTrade) => key != null && areaType(t.area) === key;
  const sales = (base?.sales ?? []).filter(inGroup);
  const rents = (base?.rents ?? []).filter(inGroup);
  const valid = sales.filter((t) => !t.cancelled && t.price);
  const maxPrice = valid.reduce((m, t) => Math.max(m, t.price ?? 0), 0);
  const name = base?.sales[0]?.complexName || base?.rents[0]?.complexName || "단지";
  const meta = base?.sales[0] ?? base?.rents[0];

  const cut = period === "1" ? new Date(now - 365 * 864e5).toISOString().slice(0, 10) : "";
  const points: TradePoint[] = [];
  for (const t of valid) if (t.dealDate >= cut) points.push({ t: Date.parse(t.dealDate), price: t.price!, label: t.dealDate, kind: "sale", direct: t.direct });
  for (const t of rents) if (t.deposit && !t.monthlyRent && t.dealDate >= cut) points.push({ t: Date.parse(t.dealDate), price: t.deposit, label: t.dealDate, kind: "jeonse" });
  if (est) points.push({ t: now, price: est.value, label: "추정가", kind: "estimate" });

  return (
    <div className="space-y-3">
      <Card className="p-3">
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" onClick={onBack} className="inline-flex items-center gap-1 text-cell text-[var(--text-muted)] hover:text-foreground">
            <ArrowLeft className="size-3.5" /> {region?.name ?? "지역"}
          </button>
          <h2 className="text-lg font-bold">{name}</h2>
          <span className="text-micro text-[var(--text-muted)]">{meta?.umdNm} {meta?.jibun}{meta?.buildYear ? ` · ${meta.buildYear}년 준공` : ""}</span>
          <ScoreBadge score={base?.score} kind="complex" />
          <span className="ml-auto flex flex-wrap items-center gap-1.5">
            <CompareButton lawd={lawd} complexKey={complexKey} />
            <WatchStar kind="complex" itemKey={complexKey} />
            {session && est && groupArea != null && (
              <Button size="sm" variant="outline" onClick={() => setRegister(true)}>내 포트폴리오에 등록</Button>
            )}
          </span>
        </div>
        <AINoteLine note={base?.note} className="mt-2" />
        {groups.length > 0 && (
          <div className="mt-3 flex flex-wrap gap-1">
            {groups.map((g) => (
              <Chip key={g.key} on={g.key === key} onClick={() => setAreaKey(g.key)} small>
                <span title="공급면적 기준 근사(전용 × 1.33)">{g.key}㎡ · 약 {supplyPyeong(g.area)}평</span> <span className="opacity-60">({g.n})</span>
              </Chip>
            ))}
          </div>
        )}
      </Card>

      {!base ? (
        <Card className="p-3 text-cell text-[var(--text-muted)]">불러오는 중…</Card>
      ) : (
        <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_360px] [&>*]:min-w-0">
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
              <Box label="추정가" strong value={est ? money(est.value) : "-"} sub={est ? `${money(est.low)}~${money(est.high)} · ${CONFIDENCE_LABEL[est.confidence]}` : "최근 1년 비교 거래 없음"} />
              <Box label="최근 실거래" value={valid[0] ? money(valid[0].price) : "-"} sub={valid[0] ? `${valid[0].dealDate} · ${valid[0].floor ?? "-"}층${valid[0].price === maxPrice ? " · 기간 최고가" : ""}` : ""} />
              <Box label="기간 최고가" value={maxPrice ? money(maxPrice) : "-"} sub={maxPrice && valid[0]?.price ? `최근 거래는 최고가의 ${((valid[0].price / maxPrice) * 100).toFixed(0)}%` : ""} />
              <Box label="전세 시세" value={jeonse ? money(jeonse.median) : "-"}
                sub={jeonse && est ? `전세가율 ${((jeonse.median / est.value) * 100).toFixed(0)}% · 갭 ${money(est.value - jeonse.median)}` : ""} />
            </div>
            <Card className="p-3">
              <div className="flex items-center gap-2">
                <h3 className="text-section font-semibold">실거래 추이</h3>
                <MiniToggle options={[{ id: "1", label: "1년" }, { id: "all", label: "전체" }]} value={period} onChange={setPeriod} />
              </div>
              <TradeScatter points={points} height={240} />
            </Card>
            <TradeTable sales={sales} maxPrice={maxPrice} />
            <RentTable rents={rents} />
          </div>
          <div className="space-y-3">
            {base.score && <ScoreCard title="시장 점수" score={base.score} kind="complex" />}
            {est && (
              <Card className="p-3 text-micro leading-relaxed text-[var(--text-muted)]">
                <b className="text-cell text-[var(--text-secondary)]">추정가는 어떻게?</b>
                <p className="mt-1">같은 단지·평형의 실거래 {est.n}건(최근 {est.windowMonths}개월{est.indexAdjusted ? ", 주간 지수로 시점 보정" : ""}) 중앙값입니다.
                  직거래 {est.excludedDirect}건 · 해제 {est.excludedCancelled}건은 뺐습니다. 층·향·수리 상태는 반영하지 않았습니다.</p>
              </Card>
            )}
            <AssetPostsCard kind="apt" assetKey={complexKey} />
          </div>
        </div>
      )}
      {register && est && groupArea != null && (
        <AssetFormDialog open onClose={() => setRegister(false)} onSaved={() => toast.success("내 포트폴리오에 추가했습니다")}
          prefill={{ assetType: "real_estate", name, propertyType: "apt", lawdCd: lawd, regionName: region ? `${region.sido} ${region.name}` : "",
            complex: { key: complexKey, name, umdNm: meta?.umdNm ?? "" }, area: groupArea, method: "comparable" }} />
      )}
    </div>
  );
}

function TradeTable({ sales, maxPrice }: { sales: ReTrade[]; maxPrice: number }) {
  const [all, setAll] = useState(false);
  const shown = all ? sales : sales.slice(0, 12);
  return (
    <Card className="p-0">
      <h3 className="border-b px-3 py-2 text-section font-semibold">매매 실거래 <span className="text-micro font-normal text-[var(--text-muted)]">{sales.length}건</span></h3>
      {sales.length === 0 ? <p className="p-3 text-cell text-[var(--text-muted)]">이 평형의 매매가 없습니다.</p> : (
        <table className="w-full text-cell">
          <thead className="text-micro text-[var(--text-muted)]">
            <tr className="border-b"><th className="px-3 py-1 text-left">계약일</th><th className="text-right">층</th><th className="text-right">거래가</th><th className="text-right">평당가</th><th className="px-3 text-right">비고</th></tr>
          </thead>
          <tbody>
            {shown.map((t, i) => (
              <tr key={i} className={cn("border-b last:border-b-0", t.cancelled && "text-[var(--text-muted)] line-through")}>
                <td className="px-3 py-1">{t.dealDate}</td>
                <td className="text-right font-num tabular-nums">{t.floor ?? "-"}</td>
                <td className="text-right font-num font-semibold tabular-nums">{money(t.price)}</td>
                <td className="text-right font-num tabular-nums">{t.price && t.area ? money((t.price / t.area) * 3.3058) : "-"}</td>
                <td className="px-3 text-right text-micro">
                  {t.cancelled && <span className="rounded bg-muted px-1">해제</span>}
                  {!t.cancelled && t.price === maxPrice && <span className="rounded bg-[var(--profit)]/10 px-1 text-[var(--profit)]">최고가</span>}
                  {t.direct && <span className="ml-1 rounded bg-amber-500/10 px-1 text-amber-600">직거래</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {sales.length > 12 && (
        <button type="button" onClick={() => setAll((v) => !v)} className="w-full border-t py-1.5 text-micro text-[var(--brand)] hover:bg-muted">{all ? "접기" : `${sales.length - 12}건 더 보기`}</button>
      )}
    </Card>
  );
}

function RentTable({ rents }: { rents: ReTrade[] }) {
  const [all, setAll] = useState(false);
  const shown = all ? rents : rents.slice(0, 10);
  if (rents.length === 0) return null;
  return (
    <Card className="p-0">
      <h3 className="border-b px-3 py-2 text-section font-semibold">전월세 <span className="text-micro font-normal text-[var(--text-muted)]">최근 1년 {rents.length}건</span></h3>
      <table className="w-full text-cell">
        <thead className="text-micro text-[var(--text-muted)]">
          <tr className="border-b"><th className="px-3 py-1 text-left">계약일</th><th className="text-right">층</th><th className="text-right">보증금</th><th className="text-right">월세</th><th className="px-3 text-right">계약</th></tr>
        </thead>
        <tbody>
          {shown.map((t, i) => (
            <tr key={i} className="border-b last:border-b-0">
              <td className="px-3 py-1">{t.dealDate}</td>
              <td className="text-right font-num tabular-nums">{t.floor ?? "-"}</td>
              <td className="text-right font-num tabular-nums">{money(t.deposit)}</td>
              <td className="text-right font-num tabular-nums">{t.monthlyRent ? money(t.monthlyRent) : "전세"}</td>
              <td className="px-3 text-right text-micro text-[var(--text-muted)]">{t.contractType || "-"}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {rents.length > 10 && (
        <button type="button" onClick={() => setAll((v) => !v)} className="w-full border-t py-1.5 text-micro text-[var(--brand)] hover:bg-muted">{all ? "접기" : `${rents.length - 10}건 더 보기`}</button>
      )}
    </Card>
  );
}

function Box({ label, value, sub, strong }: { label: string; value: string; sub?: string; strong?: boolean }) {
  return (
    <div className={cn("rounded-lg border bg-card px-2.5 py-2", strong && "border-[var(--brand)]/40 bg-[var(--brand)]/5")}>
      <div className="text-micro text-[var(--text-muted)]">{label}</div>
      <div className={cn("font-num tabular-nums", strong ? "text-lg font-bold" : "text-cell font-semibold")}>{value}</div>
      {sub ? <div className="text-micro text-[var(--text-muted)]">{sub}</div> : null}
    </div>
  );
}
