import Link from "next/link";
import type { Metadata } from "next";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { getApiBaseUrl } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import { fetchWithDeadline } from "@/lib/fetch-timeout";
import { sectorLabel } from "@/lib/score-explain-api";
import { pageMetadata } from "@/lib/seo";
import { squarify, type Rect } from "@/lib/treemap";
import { cn } from "@/lib/utils";

/*
시장 히트맵 (2026-10-21 티켓 QP-10, 참고: Finviz 맵). 시가총액 상위 200 보통주를 업종별로 묶고,
색은 그날 등락률 또는 **점수 변화**(직전 점수일 대비)다. 점수 변화 색은 경쟁 앱에 없는 화면이다.

시장·색 전환은 주소(`?market=us&color=score`)로 한다 — 서버가 다 그리므로 크롤러가 읽고, 링크로
공유되며, 화면에 클라이언트 코드가 없다. 넓은 화면과 좁은 화면은 가로세로비가 달라 두 벌을 그리고
CSS로 하나만 보인다(같은 viewBox를 좁은 폰에 줄이면 칸이 너무 작다).
*/

type Tile = {
  symbol: string;
  name: string;
  sector: string;
  marketCap: number;
  changeRate: number;
  score?: number;
  scoreDelta?: number;
};
type Heatmap = { market: string; tradeDate: string; scoreDate?: string; tiles: Tile[] };
type Props = { searchParams: Promise<Record<string, string | string[] | undefined>> };

async function fetchHeatmap(market: string): Promise<Heatmap | null> {
  try {
    const res = await fetchWithDeadline(`${getApiBaseUrl()}/api/v1/market/heatmap?market=${market}`, {
      next: { revalidate: 600 },
    });
    if (!res.ok) return null;
    return ((await res.json()) as ApiResponse<Heatmap>).data ?? null;
  } catch {
    return null;
  }
}

export function generateMetadata(): Metadata {
  return pageMetadata({
    title: "시장 히트맵 — 국내·미국 업종별 등락·AI 점수 변화",
    description: "시가총액 상위 200종목을 업종별로 묶어 오늘 등락률과 AI 점수 변화를 색으로 보여 줍니다. 투자 권유 아님.",
    path: "/market/heatmap",
  });
}

const PAD = 2;
const HEADER = 16;

function layout(tiles: Tile[], W: number, H: number) {
  const groups = new Map<string, Tile[]>();
  for (const t of tiles) {
    const k = t.sector || "other";
    groups.set(k, [...(groups.get(k) ?? []), t]);
  }
  const sectors = [...groups.entries()]
    .map(([sector, list]) => ({ sector, list, cap: list.reduce((s, t) => s + t.marketCap, 0) }))
    .sort((a, b) => b.cap - a.cap);
  const sRects = squarify(
    sectors.map((s) => s.cap),
    { x: 0, y: 0, w: W, h: H },
  );
  return sectors.map((s, i) => {
    const r = sRects[i];
    const inner: Rect = { x: r.x + PAD, y: r.y + HEADER, w: Math.max(0, r.w - PAD * 2), h: Math.max(0, r.h - HEADER - PAD) };
    const tRects = squarify(
      s.list.map((t) => t.marketCap),
      inner,
    );
    return { ...s, rect: r, tiles: s.list.map((t, j) => ({ t, r: tRects[j] })) };
  });
}

/** 등락은 ±3%, 점수 변화는 ±5점에서 가장 진하다. 한국식: 상승 빨강(--profit) · 하락 파랑(--loss). */
function fill(v: number | undefined, full: number): string {
  if (v == null || v === 0) return "var(--surface-elevated)";
  const pct = Math.round(Math.min(1, Math.abs(v) / full) * 75 + 15);
  return `color-mix(in srgb, ${v > 0 ? "var(--profit)" : "var(--loss)"} ${pct}%, var(--surface))`;
}

function MapSvg({ data, mode, W, H, className }: { data: Heatmap; mode: "change" | "score"; W: number; H: number; className?: string }) {
  const sectors = layout(data.tiles, W, H);
  return (
    <svg viewBox={`0 0 ${W} ${H}`} className={cn("h-auto w-full", className)} role="img" aria-label="시장 히트맵">
      {sectors.map((s) => (
        <g key={s.sector}>
          <rect x={s.rect.x} y={s.rect.y} width={s.rect.w} height={s.rect.h} fill="var(--surface)" stroke="var(--border)" />
          {s.rect.w > 40 && (
            <text x={s.rect.x + 4} y={s.rect.y + 12} fontSize={11} fill="var(--text-secondary)" fontWeight={600}>
              {sectorLabel(s.sector === "other" ? "" : s.sector) || "기타"}
            </text>
          )}
          {s.tiles.map(({ t, r }) => {
            const v = mode === "score" ? t.scoreDelta : t.changeRate;
            const fs = Math.max(8, Math.min(16, Math.min(r.w / 6, r.h / 3)));
            const label =
              mode === "score"
                ? t.scoreDelta == null
                  ? "—"
                  : `${t.scoreDelta > 0 ? "+" : ""}${t.scoreDelta}점`
                : `${t.changeRate > 0 ? "+" : ""}${t.changeRate.toFixed(1)}%`;
            return (
              <a key={t.symbol} href={`/stocks/${encodeURIComponent(t.symbol)}`}>
                <title>{`${t.name} · ${label}${t.score != null ? ` · 점수 ${t.score}` : ""}`}</title>
                <rect x={r.x} y={r.y} width={Math.max(0, r.w - 1)} height={Math.max(0, r.h - 1)} style={{ fill: fill(v, mode === "score" ? 5 : 3) }} />
                {r.w > 34 && r.h > fs * 1.6 && (
                  <text x={r.x + r.w / 2} y={r.y + r.h / 2 - (r.h > fs * 3 ? fs * 0.35 : -fs * 0.35)} fontSize={fs} textAnchor="middle" fill="var(--text-primary)" fontWeight={600}>
                    {t.name.length * fs * 0.95 > r.w ? t.name.slice(0, Math.max(1, Math.floor(r.w / (fs * 0.95)))) : t.name}
                  </text>
                )}
                {r.w > 34 && r.h > fs * 3 && (
                  <text x={r.x + r.w / 2} y={r.y + r.h / 2 + fs * 0.9} fontSize={fs * 0.8} textAnchor="middle" fill="var(--text-primary)">
                    {label}
                  </text>
                )}
              </a>
            );
          })}
        </g>
      ))}
    </svg>
  );
}

function Chip({ href, on, children }: { href: string; on: boolean; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      prefetch={false}
      className={cn(
        "rounded-full border px-2.5 py-0.5 text-cell",
        on ? "border-[var(--brand)] text-[var(--brand)]" : "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
      )}
    >
      {children}
    </Link>
  );
}

export default async function HeatmapPage({ searchParams }: Props) {
  const sp = await searchParams;
  const market = sp.market === "us" ? "us" : "domestic";
  const mode = sp.color === "score" ? "score" : "change";
  const data = await fetchHeatmap(market);
  const href = (m: string, c: string) => `/market/heatmap?market=${m}&color=${c}`;

  return (
    <PageScrollShell contentClassName="space-y-3">
      <header className="space-y-1">
        <h1 className="text-page font-semibold text-[var(--text-primary)]">시장 히트맵</h1>
        <p className="text-cell text-[var(--text-muted)]">
          시가총액 상위 200종목 · 칸 크기 = 시가총액 · 색 = {mode === "score" ? "AI 점수 변화(직전 점수일 대비)" : "등락률"}.
          {data?.tradeDate ? ` ${data.tradeDate} 종가 기준.` : ""}
        </p>
      </header>
      <div className="flex flex-wrap items-center gap-1.5">
        <Chip href={href("domestic", mode)} on={market === "domestic"}>
          국내
        </Chip>
        <Chip href={href("us", mode)} on={market === "us"}>
          미국
        </Chip>
        <span className="mx-1 h-4 w-px bg-[var(--border)]" aria-hidden />
        <Chip href={href(market, "change")} on={mode === "change"}>
          등락률
        </Chip>
        <Chip href={href(market, "score")} on={mode === "score"}>
          점수 변화
        </Chip>
      </div>
      {data && data.tiles.length > 0 ? (
        <>
          <MapSvg data={data} mode={mode} W={1200} H={680} className="hidden md:block" />
          <MapSvg data={data} mode={mode} W={600} H={980} className="md:hidden" />
          <p className="text-micro text-[var(--text-muted)]">
            칸을 누르면 종목 화면으로 갑니다. 점수는 같은 시장·업종 안의 상대 위치이며 매수·매도 권유가 아닙니다.
          </p>
        </>
      ) : (
        <p className="py-10 text-center text-cell text-[var(--text-muted)]">히트맵 자료를 불러오지 못했습니다.</p>
      )}
    </PageScrollShell>
  );
}
