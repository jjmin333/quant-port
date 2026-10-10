"use client";

import { useEffect, useState } from "react";
import { ASSET_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { MoodBadge } from "@/components/news/news-briefing-card";
import { useQuickLook } from "@/components/providers/quick-look-provider";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { SectionCard } from "@/components/ui/section-card";
import { Pct } from "@/components/wealth/asset-intel-ui";
import { FIN_GROUP_LABEL } from "@/lib/fin-products-api";
import {
  fetchHomeAssets,
  type AssetBrief,
  type HomeAssets,
  type HomeProductGroup,
} from "@/lib/wealth-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 홈의 「주식 밖」 — 부동산·금리·금융상품. 전부 배치가 만든 저장분 하나(`/home/assets`)에서 온다.
 *
 * 2026-10-09 사용자 요청 6·7로 자리가 셋으로 갈렸다:
 *   · 부동산·금리 브리핑 → 「오늘의 시장 브리핑」 카드의 탭(`AssetBriefPanel`). 홈 맨 아래라 처음 온 사람이
 *     있는 줄도 몰랐다.
 *   · 정기예금 TOP → 「오늘의 키워드」 자리의 「금리 TOP」(`RateTopCard`) — 예금·적금·주담대 전환.
 *   · 관심 단지 새 거래는 홈 아래 줄(`ReWatchHomeCard`). 공매 마감 임박은 10-09 요청 5로 내렸다.
 * 한 번 받아 셋이 나눠 쓴다(`useHomeAssets`) — 요청은 그대로 하나다.
 */
export function useHomeAssets(): HomeAssets | null {
  const [d, setD] = useState<HomeAssets | null>(null);
  useEffect(() => {
    // 부동산·금융상품 비노출(lib/hidden-features) — null이면 브리핑 탭·금리 TOP이 스스로 빠진다.
    if (ASSET_FEATURES_HIDDEN) return;
    let alive = true;
    fetchHomeAssets()
      .then((x) => alive && setD(x))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);
  return d;
}

/* ── 금리 TOP ─────────────────────────────────────────────────────────────── */

const PRODUCT_TABS: { id: HomeProductGroup; label: string }[] = [
  { id: "deposit", label: "예금" },
  { id: "saving", label: "적금" },
  { id: "mortgage", label: "주담대" },
];

/**
 * 「금리 TOP」 (2026-10-09 요청 7 — 「오늘의 키워드」 자리). 예금·적금은 기본금리 높은 순, 주담대는 낮은 순.
 * 줄마다 권역(은행·저축은행)·기간·최고금리와 한은 평균 대비 차이 — 「이게 높은 편인가」를 숫자 하나로.
 * 누르면 상품 미리보기 창(페이지를 떠나지 않는다, 요청 5).
 */
export function RateTopCard({ d, className }: { d: HomeAssets | null; className?: string }) {
  const quickLook = useQuickLook();
  const groups = PRODUCT_TABS.filter((t) => d?.products?.[t.id]?.items.length);
  const [picked, setPicked] = useState<HomeProductGroup | null>(null);
  const tab = picked && groups.some((g) => g.id === picked) ? picked : (groups[0]?.id ?? null);
  if (!d || !tab) return null;
  const block = d.products![tab]!;
  const loan = tab === "mortgage";
  const href = loan ? "/finance/loans" : "/finance/savings";

  return (
    <SectionCard
      title="금리 TOP"
      href={href}
      className={className}
      action={groups.length > 1 ? <MiniToggle options={groups} value={tab} onChange={setPicked} /> : undefined}
    >
      {block.avg && (
        <p className="mb-1.5 text-micro text-[var(--text-muted)]">
          {block.avg.label} <b className="font-num text-[var(--text-secondary)]">{block.avg.value.toFixed(2)}%</b>
          {" · "}
          {block.avg.date.slice(0, 7).replace("-", ".")} 한국은행
        </p>
      )}
      <ol className="space-y-1">
        {block.items.map((p, i) => {
          const diff = block.avg ? p.facts.base - block.avg.value : null;
          // 예금·적금은 평균보다 높을수록, 대출은 낮을수록 좋다.
          const better = diff != null && (loan ? diff < 0 : diff > 0);
          const sub = [
            p.facts.company,
            p.facts.finGroup ? FIN_GROUP_LABEL[p.facts.finGroup] : null,
            p.facts.term > 0 ? `${p.facts.term}개월` : null,
            p.facts.joinDeny === 1 ? "누구나" : null,
          ].filter(Boolean);
          const body = (
            <>
              <span className="w-4 shrink-0 font-num text-micro text-[var(--text-muted)]">{i + 1}</span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-cell font-medium">{p.name}</span>
                <span className="block truncate text-micro text-[var(--text-muted)]">{sub.join(" · ")}</span>
              </span>
              <span className="shrink-0 text-right">
                <b className="block font-num text-cell tabular-nums text-[var(--text-primary)]">{p.facts.base.toFixed(2)}%</b>
                <span className="block font-num text-micro tabular-nums text-[var(--text-muted)]">
                  {diff != null && (
                    <span className={cn(better && "font-semibold text-[var(--brand)]")}>
                      {diff > 0 ? "+" : ""}
                      {diff.toFixed(2)}%p
                    </span>
                  )}
                  {!loan && p.facts.max > p.facts.base && ` · 최고 ${p.facts.max.toFixed(2)}%`}
                </span>
              </span>
            </>
          );
          return (
            <li key={p.id}>
              {quickLook ? (
                <button
                  type="button"
                  onClick={() => quickLook.openAsset({ kind: "product", key: p.id, name: p.name })}
                  className="-mx-1 flex w-[calc(100%+0.5rem)] items-center gap-2 rounded-md px-1 py-1 text-left hover:bg-muted/60"
                >
                  {body}
                </button>
              ) : (
                <a href={`/finance/products/${p.id}`} className="flex items-center gap-2 py-1">
                  {body}
                </a>
              )}
            </li>
          );
        })}
      </ol>
    </SectionCard>
  );
}

/* ── 브리핑 카드의 부동산·금리 탭 ──────────────────────────────────────────── */

export type AssetBriefTab = "re" | "rates";

/**
 * 「오늘의 시장 브리핑」 카드의 부동산·금리 탭 본문 (2026-10-09 요청 6). 오늘의 시장 브리핑과 같은 순서
 * (분위기 · 제목 · 문단 · 지켜볼 것), 아래에 숫자 칩. 글은 asset_ai 배치가 쓴 것만 읽는다.
 * 칩(시군구·금리)은 미리보기 창을 연다(요청 5).
 */
export function AssetBriefPanel({ d, tab }: { d: HomeAssets; tab: AssetBriefTab }) {
  const brief = d.briefing?.[tab];
  if (!brief) return null;
  return (
    <>
      <AssetBriefBody brief={brief} stamp={briefStamp(tab, d)} />
      {tab === "re" ? <RealEstateChips d={d} /> : <RateChips d={d} />}
      <p className="mt-2 text-micro text-[var(--text-muted)]">
        {tab === "re" ? "한국부동산원 R-ONE 주간 지수 · 국토부 실거래" : "한국은행 ECOS"}
      </p>
    </>
  );
}

/** 브리핑 본문(분위기 · 제목 · 문단 · 지켜볼 것) — 홈 카드와 금리 화면(F4)이 같이 쓴다. */
export function AssetBriefBody({ brief, stamp }: { brief: AssetBrief; stamp: string }) {
  return (
    <>
      <div className="flex flex-wrap items-center gap-1.5">
        {brief.mood && <MoodBadge mood={brief.mood} />}
        {brief.headline && <b className="text-section">{brief.headline}</b>}
        <span className="text-micro text-[var(--text-muted)]">
          {stamp} · {brief.source === "llm" ? "AI 요약" : "지표 요약"}
        </span>
      </div>
      <div className="mt-2 space-y-1.5 text-read leading-relaxed">
        {brief.body.map((p, i) => <p key={i}>{p}</p>)}
      </div>
      {brief.watch.length > 0 && (
        <div className="mt-3 rounded-lg border px-2.5 py-2">
          <p className="mb-1 text-cell font-semibold text-[var(--text-muted)]">{brief.headline ? "지켜볼 것" : "관점"}</p>
          <ul className="list-disc space-y-0.5 pl-3.5 text-cell leading-relaxed">
            {brief.watch.map((w, i) => <li key={i}>{w}</li>)}
          </ul>
        </div>
      )}
    </>
  );
}

/** 글 옆 날짜 — 부동산은 지수 기준 주, 금리는 갱신 시각. */
function briefStamp(tab: AssetBriefTab, d: HomeAssets): string {
  if (tab === "re") {
    const date = d.realestate?.nation?.date;
    return date ? `${date.slice(5).replace("-", "/")} 주간` : "주간";
  }
  const at = d.briefing?.rates?.updatedAt;
  return at ? `${at.slice(5, 10).replace("-", "/")} 기준` : "금리";
}

const CHIP = "flex items-baseline gap-1 rounded-full px-2 py-0.5 text-micro";

function RealEstateChips({ d }: { d: HomeAssets }) {
  const quickLook = useQuickLook();
  const re = d.realestate;
  if (!re) return null;
  const idx = ([["전국", re.nation], ["서울", re.seoul]] as const).filter(([, v]) => v);
  return (
    <div className="mt-3 flex flex-wrap gap-1.5">
      {idx.map(([label, v]) => (
        <span key={label} className={cn(CHIP, "bg-[var(--surface-elevated)]")}>
          <span className="text-[var(--text-muted)]">{label} 4주</span>
          <Pct v={v!.w4} digits={2} />
        </span>
      ))}
      {(re.risers ?? []).slice(0, 4).map((r) => {
        const name = r.name.split(" ").slice(1).join(" ") || r.name;
        return (
          <button
            key={r.lawdCd}
            type="button"
            disabled={!quickLook}
            onClick={() => quickLook?.openAsset({ kind: "region", key: r.lawdCd, name: r.name })}
            className={cn(CHIP, "border hover:bg-muted")}
          >
            <span className="max-w-[8rem] truncate" title={r.name}>{name}</span>
            <Pct v={r.facts.w4Pct} digits={2} />
          </button>
        );
      })}
    </div>
  );
}

function RateChips({ d }: { d: HomeAssets }) {
  const quickLook = useQuickLook();
  const items = d.rates?.items ?? [];
  if (items.length === 0) return null;
  return (
    <div className="mt-3 flex flex-wrap gap-1.5">
      {items.map((r) => {
        const bp = r.prev != null ? Math.round((r.value - r.prev) * 100) : 0;
        return (
          <button
            key={r.key}
            type="button"
            disabled={!quickLook}
            onClick={() => quickLook?.openAsset({ kind: "rate", key: r.key, name: r.label })}
            className={cn(CHIP, "bg-[var(--surface-elevated)] hover:bg-muted")}
          >
            <span className="text-[var(--text-muted)]">{r.label.replace(" 평균", "")}</span>
            <b className="font-num tabular-nums">{r.value.toFixed(2)}%</b>
            {bp !== 0 && <span className={cn("font-num tabular-nums", trendTextClass(bp))}>{bp > 0 ? "+" : ""}{bp}bp</span>}
          </button>
        );
      })}
    </div>
  );
}
